# Shortcuts resolve to actions before input is delivered

A key press is first matched against bindings declared by surfaces on the focus
chain, **deepest first**. The surface whose binding matches receives a typed
**Action** — not the key. Only when nothing matches does the raw key reach the
focused surface. A surface marked a **text sink** suppresses matching for bare
printable keys, so ordinary typing is never stolen, while modified chords still
resolve.

## Considered options

**Deliver to the focused surface, then bubble root-ward when unhandled.** This is
what most Bubble Tea code does and it was the original decision here. It requires
every surface to report truthfully whether it consumed a key, and adapted
third-party components cannot: bubbles' `Update(tea.Msg) (Model, tea.Cmd)` returns
no such signal. A focused text input therefore swallows `ctrl-c`.

**Match root-ward first, then deliver.** Fixes the swallowing, but inverts
precedence: a global binding beats a dialog-local one for the same chord, which is
backwards. Escape inside a dialog belongs to the dialog.

Matching declaratively, deepest first, before delivery gets the precedence of the
first and the timing of the second, and requires nobody to report anything.

## Consequences

Bindings need **stable action identity** so keys can be remapped without reaching
inside surfaces. A surface declares `(ActionID, default keys, help text)`; froth
owns the key-to-action table and the override layer on top of it. Any sophisticated
TUI will want user remapping, and retrofitting it onto bindings with no identity is
unpleasant, so identity is required from the start even though the override layer
is not built yet.

The same declarations feed the help query, so help can never drift from what will
actually fire — including greying out bindings whose `Enabled()` is currently false.

Adapted components are necessarily **leafs**: nothing can sit beneath a surface that
cannot say what it consumed, and nothing above can trust its answer. They keep their
own internal keymaps, which froth cannot see or remap; rebinding those means
mutating the component's keymap directly.

Chorded sequences need pending state in the resolver. Not built, not foreclosed.

## Amendment: modality truncates the chain

Settled before M2 was sliced. A modal takes input for its parent's whole subtree, and
that has to mean bindings as well as raw keys: with focus inside a modal the chain still
runs modal → … → root, so on the rule above every ancestor's bindings would keep firing
underneath it. **Resolution stops at the modal.** Nothing above it matches.

One exception, declared rather than built in: a binding carries a `SurvivesModal` flag,
and those still resolve from above. It survives modality only — not focus scopes, not
anything else — and any surface may declare one.

froth hardcodes no quit key. The consequence is deliberate and worth stating plainly: an
application that marks nothing as surviving can write a modal it cannot quit past, and
froth will not save it. The alternative is froth deciding what quit means, which is worse
— a double-tap `ctrl-c` is a common and perfectly reasonable policy, and it belongs to the
application. Such a policy needs no new mechanism either: the surface receives its action
twice and compares timestamps.

## Amendment: froth binds the keys for the actions it services itself

The amendment above once read "froth owns no bindings of its own and hardcodes no quit
key". The first half was overreach: it was written about quit and generalised to every
binding, and it contradicts both the sketch's `froth.ActionCancel, // esc, per froth's
stock bindings` and the user story that wants tab to move focus without an application
implementing it.

**The line is what froth services.** Moving focus is froth's job — no surface implements
traversal — so the action that asks for it is froth's, and so is the key. Quit is not:
froth would only forward it to a surface, so it stays the application's to declare.
`ActionFocusNext` and `ActionFocusPrev` are froth's with `tab` and `shift+tab` bound to
them; `ActionCancel` is a froth identity with no key, because froth services nothing for
it until modals exist.

**An application takes one over through this same machinery.** froth's stock bindings are
matched after every surface on the focus chain, so a surface that declares `tab` for
something else simply wins. Declaring a `Binding` for a froth action anywhere on the chain
replaces froth's keys for it, and a declaration marked `Disabled` leaves the action with
no keys at all — as inert as any other disabled binding. There is no second override
table, and there is nothing an application can do to a binding of its own that it cannot
do to one of froth's.

**A serviced action is delivered nowhere.** The surface that declared the binding asked
froth to move focus, not to be told that it did.

## Amendment: the case against bubbling is about raw keys, not about actions

The rejected option above — deliver to the focused surface, then bubble root-ward when
unhandled — was rejected because it *requires every surface to report truthfully whether
it consumed a key*, and adapted third-party components cannot: bubbles' `Update` returns
no such signal.

That argument does not reach actions. A surface receiving an `ActionMsg` declared the
binding that produced it, so it is froth-native by construction — there is no adapted
component in the path that would have to report anything. **Action bubbling is unbuilt,
not foreclosed.**

What it would take is a way to decline during Deliver — recorded like `Focus` and `Raise`,
acted on after the handler returns — with resolution resuming from the next surface up the
chain and froth's own bindings as the final rung. It costs a resumable walk and a defined
outcome for an action nobody claims.

Most of it is already reachable without any of that: declare an action of your own on the
key and call `uc.FocusNext()` in the branch where you do not want to override. So
declining buys ergonomics and the froth-owned-action case rather than new power.

**Deferred to M3**, which is where intents ship. The two are one question: an intent
already walks parent-ward and reports through a claim, and an action that can be declined
is the same walk anchored at the focused surface instead of at the raiser. Deciding them
apart would land froth with two chain-of-responsibility mechanisms that differ for no
reason. Deferring further, once intents exist and the walk has been felt, is a legitimate
outcome; designing declining without looking at intents is not.

## Amendment: a key press resolves to an identity, and nothing more

Settled while slicing M3, and it corrects the rule at the top of this ADR: *"The surface
whose binding matches receives a typed Action."* It does not. Resolution answers **which
action this key means here**, and delivery answers **who acts on it** — two questions,
settled separately.

The chain walk is unchanged: deepest first, nearest declaration wins, shadowing settled
statically, text sinks suppressing bare printable keys. What changes is that its output is
an `Action` and nothing else. Delivery starts at the active surface and rises
([ADR 0016](./0016-a-message-rises-until-something-takes-it.md)).

**froth was the outlier here.** Every system surveyed separates identity from destination:
none of them makes the site that *declared* a binding the recipient by virtue of having
declared it. Destination is decided at invocation from a named scope (GTK's `win.`/`app.`,
Textual's `app.`/`screen.`/`focused.`), from a walk anchored at focus (AppKit's nil-target
`sendAction:`, WPF), or from a flat registry (VS Code, Emacs). Delivering to `lb.from` was
froth's alone.

Three things follow, and the third is what makes the whole change worth it.

**A surface can take a froth action.** It could not before: `frothOwned` removed froth's
default keys for a declared action but `service` still intercepted the identity, so a
surface could rebind `focus.next` and never receive it. The declaration site gave no hint,
which is the *"property invisible at the declaration site"* [ADR 0009](./0009-one-tree-no-implicit-surfaces.md)
parked the superroot over.

**"A serviced action is delivered nowhere" is retired.** froth's own handling is the last
stop of the walk, after root. An application pre-empts a froth action by taking it, through
the machinery it already uses, with no override table — the same shape as froth's bindings
being matched last during resolution. Framework behaviour falls out of nobody else taking
it.

**An action can be invoked with no key press.** The walk is the same minus resolution, so a
command palette needs no parallel path. Textual is the warning here: it has the same
declarative bindings and the same per-node enablement predicate, and its palette is a
hand-maintained parallel list that bypasses `run_action`, `_parse_action`, `_dispatch_action`
and `check_action` entirely. Its palette guide never mentions bindings. That is what not
unifying the two entry points costs.

**Declining is answered, and needed no mechanism.** The amendment above deferred it to M3 as
one question with intents, and it was right that they were one question. Delivery is
default-pass, so declining is silence and the message continues up the chain. The resumable
walk and the recorded decline that amendment sketched are both unnecessary.

**Nothing gates a keyless invocation.** `Disabled` affects key resolution and nothing else,
which matches the prior art exactly — VS Code's own schema says a disabled command *"does
not prevent executing the command by other means, like the executeCommand-api"*, and Emacs
that *"disabling a command has no effect on calling it as a function from Lisp programs"*. An
application may show a disabled action greyed in a palette and decline to invoke it; that is
its call.

## Amendment: the binding table has two readers, and one of them is not about keys

The binding walk is *"the only place the chain is decided"*, and both resolution and help
read it — which is what makes it impossible for help to describe something other than what
a key press does. That property is kept. What changes is that keyless invocation adds a
third reader, and two behaviours correct for a key map are wrong for a catalog:

- **`claim` drops any binding left with no keys.** Correct when everything fires by key.
  But a deliberately keyless action is now a legitimate declaration, and it is invisible.
  `ActionCancel` already sits in this hole: froth publishes the identity, binds no key, and
  nothing can see it.
- **Text-sink stripping.** While a text sink holds focus, bare printable keys are stripped
  from every binding on the chain. A command palette's filter field *is* a text sink, so a
  listing built from the binding table shows a different set than the user saw one keystroke
  earlier, because it opened.

So `Bindings()` stays the single source and **two views are derived from it**: the
key-shaped view resolution uses — shadowed, stripped, keyless entries dropped — and a
**catalog** of every enabled action on the chain, keyless entries included, carrying the
handle that declared each one.

Two views over one declaration is not the two-table drift the survey warns about; that
warning is about two *sources*. Help and dispatch still cannot describe different worlds,
only ask different questions about the same one. Textual derives its footer and its
dispatcher from one `active_bindings` for the same reason.

The catalog is also what makes user-visible rebinding and discovery work, which is why
`Action` stays a `string`. A Go type would be stronger against collisions — Flutter maps
shortcut to Intent *type* — but a type has no stable spelling in a settings file or a
palette row, which is what an action identity is for. VS Code, Emacs, GTK and Sublime all
use string ids, and for this reason.

**An action carries no parameters.** A binding has nowhere to put arguments, and an
application that wants one defines its own rising message type and posts it — needing
nothing from this machinery. A string-keyed argument bag would be strictly worse than a
typed struct. The catalog lists actions; it is not the only way to invoke behaviour.

## Amendment: what modality settles, and what froth still refuses

Modality is settled here and nowhere else. Delivery does not truncate at a modal, because
if it did, a `SurvivesModal` binding would resolve from above and then never reach the
surface that declared it — every surviving binding would end as a diagnostic. See
[ADR 0016](./0016-a-message-rises-until-something-takes-it.md).

**`SurvivesModal` must be honoured inside the one binding walk and nowhere else** —
`chainBindings()` in `binding.go`. Textual has shipped the bug that comes from doing
otherwise: its escape mechanism is a second walk over
the untruncated chain, while its footer reads the truncated one, so `ctrl+p` fires under a
modal screen and is absent from the help. One walk is what prevents this, and the flag has
to live inside it.

The flag keeps its default. Six of eight surveyed systems default to not-surviving, and the
two that do not simply lack a mechanism — no system built one and defaulted it open. Worth
recording that froth's own examples are two of two the other way, and both are quit; that
matches the three systems which curate an explicit survivor list, all of which start it with
quit and none of which needed more than about six entries. The reading is that the flag is
**degenerate in the common case** rather than wrongly defaulted: a modal is parented to the
root, so truncation blocks the bindings of exactly one surface. It stays general because
`Modal()` is an option on any create, and a modal parented deeper truncates more.

**froth is also the only surveyed system that guarantees no way out**, and the amendment
above should say so rather than read as uncontested. Every system except Win32 keeps
something alive under a modal, and Textual's framework declares the surviving quit itself.
froth's refusal is *better* justified than when written — under
[ADR 0016](./0016-a-message-rises-until-something-takes-it.md) a froth key bound to an action
nobody takes would fire the tail diagnostic on every press — but it is a departure from
otherwise unanimous precedent, and it is recorded as one.

Whether froth services `ActionCancel` by closing a modal, once modals exist, is open. It is
worth noting that this was [ADR 0009](./0009-one-tree-no-implicit-surfaces.md)'s stated
condition for reviving the superroot — *"if modality gives froth something to service that
has no sensible home"* — and that it no longer would, because the instance at the tail of
the walk is a sensible home.

## Amendment: four things building it settled

Built as #39. None of them changes the decisions above; three are refinements and one is
a correction to this ADR's own wording.

**The catalog is called `Query.Actions`, and the entry type is shared with help.**
`LiveBindings` and `Actions` both return `[]ChainBinding` — the declaration plus the
handle that declared it — and differ only in how `Keys` is narrowed. One type is the
honest encoding of *"two views over one declaration"*: a second struct would have made
them look like two sources. `From` also closes a hole that predates the catalog, since
`LiveBindings` had been dropping the declaring handle entirely.

**`From` is where a declaration came from and says nothing about where the action goes.**
That distinction is the whole point of this ticket, so the field's doc says it outright.
It is the zero handle for a stock binding, which no surface declared.

**A disabled declaration is listed, and "every enabled action on the chain" above is
wrong.** The amendment that coined the phrase also says an application *"may show a
disabled action greyed in a palette and decline to invoke it; that is its call"*, which
needs the entry present. Listing it also matches what help already does — a disabled
binding is reported with its keys so help can grey it rather than lose it — and keeps the
catalog free of a key-path rule. Read "every enabled action" as "every action".

**Shadowing is computed twice, because it is not the same question twice.** A key a text
sink takes as typing is claimed by nobody, so what shadows what genuinely differs between
the two views whenever a sink holds focus. One walk, one call to each surface's
`Bindings()`, two sets of claims — which is still one walk in the sense the amendment
above insists on, since nothing re-reads a declaration or re-decides the chain. A
declaration every one of whose keys was shadowed is listed with none, which reads the same
as a keyless declaration and should: neither has a key that reaches it here.

**"Catalog" is this ADR's word and not froth's.** The thing is `Query.Actions` in code and
an **action listing** in `CONTEXT.md`. The amendments above are left as written rather than
retitled, but nothing should carry "catalog" forward into code, comments or vocabulary.

**Keyless invocation makes the `ActionCancel` gap reachable without a key.** The amendment
above notes that a froth key bound to an action nobody takes fires the tail diagnostic on
every press, and that froth binds no key here for exactly that reason. `Invoke` gets past
that: the listing offers `froth.cancel`, an application may invoke it, and until something
takes it the walk ends untaken and reports. That is the designed behaviour of a walk nobody
ends rather than a defect, and it sharpens the open question below rather than answering it.

## Amendment: a surface declares its bindings, and froth stores them

Settled as #30, and it inverts the mechanism this ADR has assumed throughout. froth no
longer asks a surface for its bindings. A surface **pushes** its whole set with
`SetBindings`, froth stores it against the handle, and `chainBindings` reads stored state.
`Binder` is gone.

Everything above about *what* the walk does is unchanged: deepest first, nearest
declaration wins, shadowing settled statically, text sinks suppressing bare printable keys,
`SurvivesModal` honoured inside the one walk, froth's own bindings matched last, two views
derived from one set. Read `Bindings()` in the amendments above as "the surface's stored
declarations". What changes is where those declarations come from.

**froth now makes no synchronous call into a surface at all.** Resolution and both queries
were the last two, and they read the same declarations, so inverting one inverted both.
Four things went with the call: the re-entrancy guard in `chainBindings`,
`DiagReentrantQuery`, `Binder`, and the quarantine site that had to read the phase because
it was the one callback froth made from whatever phase it happened to be in — the site
whose zero token caused the bug recorded in the #40 log entry.

**It also made a latent bug visible, which is worth recording as a consequence.** Pull let
an *unmounted* surface answer `Bindings()`, which was asking a surface to declare before it
had been given its chance to. Push makes "a surface has declared nothing until it has
mounted" true, and that exposed `Update` routing a message against a tree it had not applied
yet. `Update` now applies before it routes. See the #30 milestone-log entry.

**The trade is a drift for a drift, and it is not a wash.** Pulling made drift impossible:
a `Disabled` field was recomputed at the moment froth needed the answer. Pushing makes a
different drift possible: a surface changes state and does not declare again, so it goes on
answering to what it last said. Nothing in froth detects that, and nothing was built to.

Three things make it a trade worth taking rather than a regression:

- **The property this ADR is actually about survives untouched.** *"Help can never describe
  something other than what fires"* is a claim about two readers of one source. Both are
  now reads of froth's stored set, which is a stronger version of the same guarantee, not a
  weaker one — they cannot disagree even in principle, where before they merely asked the
  same surface twice in one walk.
- **A stale declaration is stale in one direction that matters and one that does not.** A
  wrongly-enabled binding fires an action the surface then declines to service, which is
  what an action nobody takes already does. A wrongly-disabled one loses a keystroke. Both
  are visible, neither corrupts anything.
- **The push point is where the state changed.** `SetBindings` is on the self tier in both
  Apply and Update, so the surface declares three lines from the field it just wrote.
  Routing every re-declaration through `Self().Reconcile()` was considered and rejected for
  exactly this: it puts a request and a phase boundary between the state change and the
  declaration that describes it, for a set of bindings that is not structure.

**Considered and rejected: froth requests a reconcile on a surface's behalf.** After any
delivery to a surface that has declared bindings, froth could queue a reconcile, which
makes forgetting impossible. Rejected because it breaks the invariant #41 shipped — froth
never calls `Reconcile` on its own — and because it makes a structural callback run per
keystroke for every binding-declaring surface, spending settle budget on the common case to
insure against the rare one. The honest answer is that push accepts a drift, and this ADR
records it as accepted rather than mitigated.

**Considered and rejected: `Binder` survives as a one-time seed read at Mount.** Every
deletion above still happens, and a surface with static bindings keeps a declarative
`Bindings()`. Rejected because a method called exactly once, whose later return values are
silently ignored, is the *"property invisible at the declaration site"*
[ADR 0009](./0009-one-tree-no-implicit-surfaces.md) parked the superroot over. A filter
field recomputing `Disabled` per keystroke would have gone on compiling and quietly stopped
working. A leaf that only declares bindings grows a three-line `Mount` instead, which says
what it does.

**Considered and rejected: dropping `Disabled`.** It is the field that drifts; static
declarations would barely drift at all. Rejected because greying a shortcut is what this
ADR wanted the shared declaration set for, and the action listing publishes `Disabled` for
a palette to act on. The churn stays, and it is what decided that Update gets `SetBindings`
rather than Apply alone.

## Parked: reading a surface's declarations back

`SetBindings` writes and nothing reads, which is an odd shape for an API and was noticed
immediately. It is not quite write-only: `Query.LiveBindings` and `Query.Actions` both
report declarations with the handle that declared each, so a surface can already see its
own — but only while it is on the focus chain, and only with `Keys` narrowed to what the
question being asked cares about. What is missing is the unnarrowed set, for any handle.

Not built, because the shape it would enable is the one froth argues against everywhere
else. A read-modify-write makes froth's stored set the memory, which is the journal
`Reconciler` refuses for children: *"the state a surface already keeps is the memory, so
there is no journal of what has been asked for and no way for one to drift."* It also
compounds the drift the amendment above accepts — recomputing the whole set self-heals,
because the next declaration corrects a missed one, while a read-modify-write turns a
missed write into a permanently wrong base for every later read. A surface computes its
declarations from its own state, which both examples that declare dynamically now do.

Revive it on a case where a surface genuinely **cannot** compute its whole set: a wrapper
around a third-party component that declares bindings froth stored and the wrapper never
saw. Go embedding is not that case — `append(b.base.bindings(), extra...)` composes without
froth. Debugging and demos are not that case either; that want is the inspection hatch,
which reads a *stranger's* declarations rather than modifying one's own, and belongs to
whatever ships `froth/inspect`.

If it is built, note the trap: the obvious spelling is `Bindings()` on `HandleReader`,
which is the exact name this amendment deleted from `Surface`. Different receiver, so it
compiles; a reader would have to be told why.

**Flags went the other way, and it is not precedent for bindings.**
[ADR 0019](./0019-focus-is-admitted-on-two-axes-and-a-click-must-find-you-first.md) gives
`Flags` a `GetFlags` alongside `SetFlags`. The argument above turns on bindings being
*computed* from a surface's own state, which makes recomputation self-healing and a stored
base a journal. Flags are computed from nothing — they are the state, froth holds the
authoritative copy because the surface's type declared it there, and no recomputation exists
for a read to spoil. Reading a declaration back is safe exactly when there is no source of
truth outside froth, which is true of flags and false of bindings.

## Parked: registering an action

Help text lives on `Binding`, so the same identity declared in two places can carry two
different help strings with nothing to reconcile them. A registry — identity, help text,
and whatever else describes an action rather than a key — would fix that and give the
catalog a place to read from.

Not built, because the bug is not reachable yet: it needs the catalog to exist and something
to declare a duplicate. Revive it when the catalog actually has to choose between
contradictory entries for one identity — bring the collision, not the tidiness.

## Parked: namespacing action identities

froth already has namespacing as convention with no enforcement: identities are dotted
(`froth.focus.next`), and `Action`'s own doc says identity is *"a caller's to keep unique"*
and that froth compares actions nowhere. GTK enforces namespaces (`app.`, `win.`) and Textual
scopes them the same way.

Enforcement buys nothing until independently authored surface libraries can collide, which
needs a third-party ecosystem that does not exist. Revive it on a real collision.
