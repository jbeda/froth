# Focus is admitted on two axes, and a click must find you first

Any surface can be focused by name. On top of that universal base sit two independent
admissions a surface declares about itself — `TabStop`, meaning traversal may land here,
and `ClickFocus`, meaning a click may move focus here. Neither implies the other.

Finding the surface a click belongs to is a separate question, answered before admission
and by different means: the hit walk descends Z and skips any surface its creator marked
`MouseTransparent`, along with that surface's subtree.

## The count

froth had one flag, `TabStop`. A click cannot be admitted by it without deciding that
everything walkable is clickable and everything clickable is walkable, and neither holds:
a label with selectable text wants the click and not the walk, and tabbing through every
label on a screen is the reason `TabStop` defaults off.

Three systems have answered this by splitting rather than by choosing. Qt carries
`TabFocus = 0x1` and `ClickFocus = 0x2` as independent bits of one enum and ships `QLabel`
using `ClickFocus` alone. The HTML standard defines `focusable` as a base with *sequentially
focusable* and *click focusable* as independent subsets of it, and says so outright: *"an
element can be focusable, but neither sequentially focusable nor click focusable."*
Terminal.Gui **shipped v1 with the two welded** — its `TabStop` setter was literally
`tabStop = CanFocus && value` — and unwelded them in v2, documenting v1's model as
"poorly specified, hard-to-test, and fragile". That last one is the argument: same
constraints as froth, shipped the cheap version, met real use, paid to undo it.

The base stays universal and unflagged, which is where Qt and Win32 also land — Qt's
`setFocus()` never reads `focusPolicy` and gates only on `isEnabled()`; Win32's `SetFocus`
has no focusable style bit. Flutter, GTK4 and Android refuse instead, so this is a choice
rather than a consensus, and it is froth's existing one.

**So there is no trichotomy.** The web's three-layer shape earns its form because its base
can be false — `disabled` and `inert` make it so. froth has no disabled surface, so a subset
of a set that is always everything is a plain boolean, and the honest description is one
universal capability plus two independent admissions.

## The click pipeline

**Targeting.** Walk placements top-down in Z. A surface marked `MouseTransparent` is skipped
and the walk continues beneath it; its subtree is skipped with it. An unmarked surface is a
candidate, which is what every system surveyed defaults to.

**Admission.** The surface found takes focus if it declared `ClickFocus`, and otherwise
**focus does not move at all**. There is no walk up to an ancestor that would have accepted.
The click is still delivered as a message either way; only focus is withheld.

The no-ancestor-walk rule is the HTML standard's — "get the focusable area" returns null and
the focusing steps return, with no ancestor step anywhere. Textual walks ancestors and
Terminal.Gui takes the immediate parent, so practice diverges; the ancestor walk that
everyone assumes the web performs is unspecified there. froth takes the explicit answer
because `MouseTransparent` covers the case that motivates the walk: a surface that should
pass a click onward says so, rather than relying on an implicit rescue.

A consequence worth stating because it is the common case, not an edge: both admissions
default off, so an ordinary layout container is a hit candidate that admits no focus, and a
click on one does nothing. That is intended. froth draws children over their parent's rect,
so a click on a container's own visible area has only that container's ancestors beneath it,
and falling through to them is the ancestor walk arriving by another door.

**Modality.** A modal *"takes input for its parent's whole subtree while it is open"*
(`CONTEXT.md`), and a click is input. A click whose target lies inside a subtree an open
modal blocks is suppressed. This is not a new mechanism and not a flag; it is froth's
existing modality finishing its own sentence.

Win32 solves the same problem the other way, and the difference is instructive rather than
better: it disables the owner — *"the system automatically disables the window and any child
windows belonging to it when it creates the modal dialog box"* — and disabled windows are
*"ignored when the system tries to determine which window should receive mouse messages."*
That needs a disabled concept, which froth does not have. froth's modality already names the
blocked subtree, so it needs nothing added.

## Who owns a flag

A surface owns what it is. Its creator owns how it sits in the composition.

`Flags` — `TabStop`, `ClickFocus`, `TextSink` — are declared by the surface's **type** through
a context-free method froth calls during `Create` and stores, and restated by the surface
through `SetFlags`/`GetFlags`. `FocusScope` and `MouseTransparent` are the creator's, through
a `CreateOption`; `MouseTransparent` is also mutable through `ChildCtx`, and `FocusScope` is
not mutable at all.

**Every flag has exactly one initial author**, and that is the property the design is built
around rather than a pleasant accident. Two authors plus whole-set replacement is a silent
clobber whichever order froth picks: a type declaring a zero value would wipe a creator's
option, and fixing the order at create would not help, because the surface's next `SetFlags`
wipes it again. Removing the overlap dissolves the ordering question instead of answering it,
and lets `SetFlags` keep the whole-set shape `SetBindings` already has — *"the last thing said
declares again"* — safely, because each set has one writer.

`GetFlags` exists although `SetBindings` has no counterpart, and
[ADR 0008](./0008-shortcuts-resolve-to-actions.md) parks reading declarations back on the
grounds that read-modify-write makes froth's stored set the memory. That reasoning does not
reach flags, and the difference is worth stating because the two look alike.

A surface **computes** its bindings from its own state, so recomputing the whole set
self-heals: a missed declaration is corrected by the next one. Flags are not computed from
anything — they *are* the state, and nothing recomputes them, so there is no self-healing
path for a read to spoil. And a flag's authoritative initial value lives in froth, put there
by the type's declaration; without a read, a surface wanting to flip one bit would have to
restate that declaration by hand from a second copy, which is the drift ADR 0008 is trying to
avoid rather than an instance of it.

The same asymmetry in one line: bindings have a source of truth outside froth and flags do
not.

**Why the type declares rather than the parent.** A component knows whether it takes keys; a
caller who forgets `TabStop()` produces a text field the keyboard cannot reach, and the
component's author cannot prevent it. Win32 puts these defaults on the window class, and
froth's analogue of a class is the Go type — reached through the optional-interface pattern
`surface.go` already describes. It is asked **once, at `Create`**, so the flags are right from
the instant the handle exists, and it takes no context, so there is nothing it could
re-enter. That is what separates it from `WM_GETDLGCODE`, which ADR 0004 forbids: a question
asked per keystroke is a synchronous call into a surface, and a value read once and stored is
not.

**Why the parent has no say over a surface's own flags.** No case was found. The two
candidates — suppressing a whole subtree, and moving the tab stop between members of a group
the way Win32 physically moves `WS_TABSTOP` — both belong to features froth has not built. A
parent wanting a child out of the walk posts it a message and the child restates its own
flag, which is ADR 0004's shape and needs nothing new. Granting authority later is easy;
withdrawing it once applications depend on it is not.

## Automatic landing consults two different predicates

`enterFocusScope` and `rehomeFocus` pick targets nobody named, and they now ask different
questions on purpose.

A **remembered** link is checked against *admits focus by any route* — `TabStop` or
`ClickFocus`. Restoring means putting focus back where it was, and clicking is a legitimate
way to have got there. Terminal.Gui v2's `RestoreFocus()` deliberately ignores `TabStop` for
this reason, and the survey notes that only makes sense because its two axes are separate.

The **fallback** scan keeps `TabStop` alone. Nothing is remembered, so froth is choosing a
sensible start, and a keyboard-driven entry into a dialog should not land on a click-only
label that happens to be first in child order.

## `Focus(scope)` redirects only when nothing would admit it

Superseding ADR 0011's outside-enters/inside-is-literal rule: **focus moved to a surface that
admits focus by no route redirects into it; a surface that admits focus by any route takes it
literally.** One predicate, the same union as above, and no dependence on where focus already
was.

This was proposed in review while building #12 and rejected then, because keyed on `TabStop`
alone it made an unwalkable container permanently unfocusable — `Focus(container)` landed on a
child, tab moved on, and the one call naming the container could never have it. `ClickFocus`
dissolves that: a container may now admit clicks and not the walk, which is the web's
`tabindex="-1"` and, in the survey's words, the platform's central composite-widget primitive.
So "focus the container itself" became expressible, and the rule stopped being a trap.

A scope declaring no admissions still redirects, which is what a dialog wants and what froth
did before. What changes is `Focus(scope)` from *inside* a scope, which no longer lands
literally on a scope surface that admits nothing. That is a fix: focus on a surface admitting
nothing can be tabbed away from and never returned to, which is the trap the review objected
to, arriving through the other door.

## Considered options

**One boolean, with clicks following `TabStop`.** What Terminal.Gui v1 shipped and v2 undid.

**A flag plus a capability the surface answers**, the `WM_GETDLGCODE` shape, recorded on #1 as
deserving weight because it survived thirty years. It survived as a *keystroke-routing*
mechanism: of eleven `DLGC_*` values exactly one is a documented focusability veto, it applies
to the arrow walk and not the tab walk, and the quotes froth had recorded were attached to the
wrong walk. ADR 0004 forbids the synchronous ask regardless. Terminal.Gui's variant — refuse
after the fact, `SetFocus()` returning bool — composes better and collides with ADR 0004 the
same way.

**Two slots per flag, resolved by position**, as ADR 0017 does for Environment. Not built.
Environment values *substitute*, so position decides; booleans want *suppression*, which is a
different mechanism wearing the same clothes, and no case needs it.

**A `ParentFlags` struct, mirroring `Flags`.** Rejected for now only because the creator's side
has two flags of which one is immutable, so a single value would make an illegal state
representable and then need policing at runtime — which `phase.go` explicitly avoids by giving
each phase its own context type. `CreateOption` is also `func(*meta)` and so can carry
something that is not a boolean, which the modal work is the likeliest source of. **Revisit
during #53**: if the creator's side reaches three or four flags, or any of them needs
read-modify-write, the symmetric version wins.

## Amendment: the root's type is asked too, and a panic there escapes

Written while building, and it corrects an omission rather than a decision. Flags are declared
as a surface enters the tree, and `New` is the root's entry — so `New` asks the root's type and
stores the answer, exactly as `Create` does for everyone else. Without it a component would be
a text sink everywhere except at the root, silently.

**A panic in the root's declaration is left to escape `New`.** Nothing is in progress to unwind
and there is no tree to keep consistent, so the quarantine route that protects a `Create` would
here queue the root for destroy and leave an instance whose root names nothing — worse than the
panic. `New` already panics through `bug()` for a nil root; this is the same kind of failure at
the same moment.

Reporting a diagnostic instead was tried and reverted. It fires for any reusable surface that
declares flags the moment someone uses it as a root — four tests in the existing suite trip it,
because an ordinary component is used both as a child and as a root — and the application cannot
act on it without deleting a method the component needs elsewhere. froth's other diagnostics all
name something the caller did wrong.

One consequence taken deliberately: a root may now declare `TabStop` and be a stop within its own
scope. The previous rule that the root can never be one was a consequence of `New` taking no
create options, not a decision.

**`rehomeFocus` needed no edit.** It delegates to `enterFocusScope`, so the predicate change
above reaches it for free. This ADR named both, which reads as two changes.

**The cost note asking the examples to carry one surface of each admission is not met**, and
cannot be until a click can reach one: a declared flag nothing reads is invisible in a running
program. It belongs with hit-testing.

## Parked: a surface that is the hit target for its whole subtree

A composite control — a button assembled from an icon surface and a label surface — wants a
click anywhere inside it to land on the button. A flag saying *the hit walk stops descending
here* would say that in one declaration on the button itself.

Not built. It is redundant with what already exists: `MouseTransparent` is subtree-scoped, and
the button creates its own parts, so it marks each one transparent at the create site — which
is where a reader is already looking, and is not an outside caller who might forget. Building
the flag first would be building the sugar before the mechanism. It also has almost no prior
art: of the systems surveyed only Android has anything like it, in `onInterceptTouchEvent`,
and Textual and Terminal.Gui avoid the problem entirely by making a button one widget with
rendered content rather than child widgets — which ADR 0003 and ADR 0009 deny froth.

Decisively, froth has no composite control. Every `TabStop` in both examples is on a leaf.

**Do not call this capture.** `CONTEXT.md` already defines Capture as a claim on all mouse
input regardless of what lies under the cursor, which is nearly the opposite. The session that
designed this used the word for an hour before the identifier check caught it.

Revive it on a composite control whose parts cannot be marked transparent at their create
site.

## Parked: a surface suppressing its own targetability

`MouseTransparent` is the creator's alone. Should a surface need to make itself untargetable
without its parent's cooperation, the shape is a second flag on the surface OR-ed with the
creator's, so either sets it. That is *suppression* rather than substitution, and it is the
form booleans want when they have two authors — as against ADR 0017's positional resolution,
which is right for values that replace one another.

Not built: no case, and it reintroduces two authors for one effect, which the ownership rule
above exists to avoid. Revive on a surface that must refuse the mouse and whose creator will
not do it for it.

## Costs

**A surface's participation is spread across two places.** Its type declares, its creator
composes, and a reader wanting the full picture reads both. The alternative — everything at
the create site — is what produced the forgotten-`TabStop` defect.

**Two admissions is more to get right than one**, and the failure is quiet: a surface that is
walkable and not clickable is reachable by keyboard and dead to the mouse, which reads as a
bug in the mouse code. The examples should carry at least one of each so the difference is
visible in something that runs.

**`enterFocusScope`'s two loops test different predicates**, which looks like an inconsistency
and needs a comment saying it is not.

**The flag set is larger than the ergonomics have been examined against.** Five flags across
two authors, with a declaration method, a `CreateOption`, a `ChildCtx` setter and a
`SetFlags`/`GetFlags` pair. The model is settled; the ergonomics are deliberately a later
pass.
