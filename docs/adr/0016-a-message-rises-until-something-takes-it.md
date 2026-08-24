# A message rises until something takes it

There is one kind of delivery in froth. `Post` names a surface, `Update` receives, and
whether a message keeps travelling after it arrives is a property of **the message type**:
one that embeds `froth.Rising` is offered to the surface it was posted to, then to that
surface's parent, and on up to the root and finally the instance. A surface ends the walk
with `uc.StopRising()`. Silence means it kept going.

`Intent` is gone. It was a parent-ward walk with a claim, landing in Apply, reachable
through a second verb and a second interface. The walk is worth keeping and is now
ordinary delivery. The rest was machinery for a guarantee nothing turned out to need.

## What travels upward is the type, not the call

A rising message is declared where its type is:

```go
type ConfirmDelete struct {
    froth.Rising
    Row row
}
```

The alternative is a second verb — `Post` addresses, something else rises — and it was
rejected because `Post` must never rise. If a message posted to a handle continued past
it, addressing would mean nothing: an RPC result would land on an ancestor because the
target's type switch happened to lack a case, and the ancestor might have a case for that
same type. Silent misdelivery, against [ADR 0015](./0015-results-come-back-to-whoever-asked.md),
whose whole premise is that results come back to whoever asked. So the property cannot
live on the call, because one of the two calls cannot carry it.

Putting it on the type is also the mainstream of the prior art. Textual gates bubbling
with a class variable on the message class, the DOM with `bubbles` on the event, WPF with
a routing strategy fixed when the event is registered, Qt by event type. Only Flutter
decides at the call site, and Flutter has three unrelated propagation systems with three
conventions.

A type rises or it does not, permanently. That is Textual's constraint too, and it is a
feature: a reader of the type knows how far it travels without finding the send site.

`Rising` rather than `Bubbling`, which is the industry's word: froth is a Bubble Tea
library, and `froth.Bubbling` beside an import of `bubbletea` reads as though it concerned
the one thing it does not.

## The walk

It starts at the surface the message was posted to, visits that surface first, and climbs
the parent links to the root, then the instance. Every surface on the path sees it, because
there is one receiving method and no interface gating who takes part.

**Default is pass.** A surface that does not recognise a message does nothing and the
message keeps rising. This is unanimous in the prior art, and it is what makes framework
behaviour fall out of nobody else taking it rather than being a special case in front of
delivery.

**`StopRising` is named for the mechanism, not for the intent**, and that is what keeps a
third state from ever being needed. A surface that wants to act on a message *and* let it
continue simply does not call it. WPF had to invent `handledEventsToo` for exactly this
case because its flag was named `Handled`, which asserts something about the handler rather
than about the message. Every surveyed system that shipped a two-state handled signal later
needed a third; naming the signal after its effect is how froth avoids the pressure instead
of absorbing it.

**A message that reaches the tail with nobody taking it is a `Diagnostic`.** It is almost
always a missing case or a surface parented somewhere unexpected, and it is not froth's to
escalate. This is also the only thing catching a forgotten `StopRising`, since Go cannot
force the call.

## It lands in Deliver, and nothing needed Apply

Intent landed in Apply so an ancestor could restructure — create the modal, own the policy —
while the surface that raised it was still alive. Under this ADR a rising message is
delivered like any other, and a surface that must restructure records what it wants and
reconciles.

The evidence that nothing needs the old guarantee comes from a real Bubble Tea codebase
that had independently invented an intent mechanism: of sixteen raise sites, exactly two
could not have been messages, both because the handler reached back into a surface that was
about to close — cancelling its in-flight request, and mirroring a value into a picker.
[ADR 0015](./0015-results-come-back-to-whoever-asked.md) dissolved both after the fact. A
surface's context is cancelled on destroy with no teardown written, so the first is
automatic; a surface holding a service by reference has no copy to mirror, so the second
has nothing to update.

**Recorded so it is not re-proposed on general principle:** if a case appears where an
ancestor must act on a surface that is about to die, and cannot get what it needs from the
message payload, from a reference, or from a cancelled context, bring that case — not the
mechanism.

## The queue drains before Apply runs again

Apply used to run between every pair of deliveries, so a message posted during one delivery
was handled after the next Apply. That is now: Apply once so the root exists, then drain the
queue completely, then Apply, and repeat while anything structural is pending.

A walk cannot have the tree change under it mid-climb, which is what the old interleaving
allowed — a surface calling `uc.Close()` and posting a rising message would have had its
destroy committed before the message reached its parent. This is the ordering the old
"intents apply before queued destroys" rule was protecting, obtained from the loop rather
than from a separate mechanism.

Two consequences worth stating because they are decisions, not accidents:

- **A message posted to a surface destroyed later in the same turn is delivered.** The
  destroy is recorded during Deliver and commits at the next Apply, which is now after the
  drain.
- **A delivery asking for Apply waits for the whole drain** rather than getting it on the
  next turn of the loop. `Create` is unaffected — no handle comes back either way — but a
  focus request and the messages queued behind it see the old focus.

The iteration cap is unchanged and already covers the cascade this makes easier to build:
two surfaces posting a rising message to each other spend from the same budget as any other
delivery, and exhausting it reports and abandons rather than hanging.

## Delivery order is guaranteed, and a walk is not interrupted

Two statements, both promises rather than accidents of the data structure.

**Messages are delivered in the order they were posted.** Post A and then B, and A is
delivered before B. This has always been true of the queue and was never written down.

**A rising message reaches every ancestor in order, with nothing delivered in between.**
The walk is *one* queue entry carrying its current position, and one dequeue climbs it
until a surface calls `StopRising` or it reaches the tail. Had each hop been its own
entry, a message posted after a rising one would be delivered between two of its hops,
and the first guarantee would be true in letter and useless in practice.

Nothing about this nests: each hop calls a different surface's `Update`, sequentially, and
a path through a tree cannot visit the same surface twice, so
[ADR 0004](./0004-no-synchronous-send.md)'s exclusion of re-entrant delivery is untouched.
Every hop still spends from the budget, so a pathological chain trips the cap as any other
cascade would. A surface that posts while the walk is passing through it gets ordinary
tail-of-queue behaviour: a walk can cause later traffic, but cannot be split by it.

## Modality stops keys, not messages

Delivery never truncates at a modal. Modality is settled entirely while resolving a key
press ([ADR 0008](./0008-shortcuts-resolve-to-actions.md)), and once something is a message
it travels the tree normally.

The alternative breaks `SurvivesModal` by construction. That flag exists so a binding
declared above a modal still resolves; if delivery then truncated at the modal, the message
would never reach the surface that declared the binding, and every surviving binding would
end at the tail as a diagnostic. It would also make rising delivery behave differently from
`uc.Post`, which ignores modality entirely today — nothing in `deliver` checks ownership or
modality, and posting into or out of an open modal already works.

**A consequence for application structure, not a rule froth enforces:** a modal is parented
to the root, so its chain is exactly `[root, modal]` and root sees every rising message from
inside every modal. Root should stay thin and hold only what is genuinely global; anything
else belongs in a child that a modal's chain does not pass through.

## The instance is the tail

froth's own actions are handled after every surface has passed, not before any of them has
looked. Root gets its turn first, so an application pre-empts a froth action by taking it,
with no override table — the same shape as froth's own bindings being matched last during
resolution.

**froth has no out-of-band hatch, deliberately.** Every surveyed system that put framework
defaults in the ordinary chain kept one anyway: Flutter's early/late `FocusManager` registry,
Textual's `priority=True` pass and a hardcoded `escape`, GTK4's use of the capture phase for
global accelerators, AppKit's `noResponderFor:`, tview's hardcoded Ctrl-C. Those exist for
input froth does not handle — IME, forced quit, accessibility. The first of those to arrive
is what creates froth's first case, and it should be argued then rather than added quietly.

Tail position was chosen over head position knowingly. WPF puts framework class handlers at
the head of every node and documents the result: *"Class handling might give the outward
appearance that the routed event isn't ever raised."* Tail-position defaults fail by being
unreachable; head-position defaults fail by being invisible. Unreachable is the better
failure, because it is visible.

## What this retires

`Intent`, `IntentSink`, `Raise`, `ApplyCtx.Sender()`, `uc.Reply()` and `froth.RepliesTo()`.

A message that needs a reply address carries the handle in a field, and a surface that
depends on knowing who opened it takes that handle as a constructor parameter. froth records
no sender. Two arguments: [ADR 0015](./0015-results-come-back-to-whoever-asked.md) holds that
addressing and lifetime are froth's while the currency is the caller's, and who asked is
currency; and a message arriving from outside through a `PostFunc` has no sender to record,
so froth could not carry one uniformly even if it wanted to.

This costs a constructor parameter where a create option used to hide one, which is the
intended direction — the dependency is real and now says so where a reader is already
looking.

**Redirection needs no mechanism.** A surface may take a message and also post it onward. If
the type rises, posting it to another handle starts a fresh walk there, which is how a modal
hands something to its opener. Textual guards this case specifically — *if the message's
sender is the parent, stop after the parent* — and froth does not: it is a subtle rule that
would surprise someone, and the iteration cap is the honest backstop.

## Costs

**Every ancestor's `Update` sees a rising message.** froth trees are deep, because controls
are surfaces, so a key press at depth twenty is twenty deliveries and twenty type switches
where there was one. It is far inside the iteration budget, but it is also twenty chances
for an ancestor to hold a case for a type it did not expect. Textual accepts exactly this,
and it is the price of one uniform path.

**The stop signal can be forgotten.** No surveyed system can force its equivalent either;
the Go precedent for a must-call is an external analyzer, `go vet`'s `lostcancel` and
`errcheck`. The tail diagnostic is what makes forgetting visible.

## Considered options

**A bool returned from `Update`.** Rejected before, in [ADR 0002](./0002-three-phase-update.md),
and still rejected: it taxes every addressed message for a signal only rising ones need.
Being wrong about *where* the bool went is what kept it plausible — every one-path system
concluded independently that the signal should not be a return value at all.

**A separate method on a separate interface**, returning bool, implemented only by surfaces
that take rising messages. It answers the objection above precisely, cannot be forgotten,
and lets the walk test before delivering. Rejected on the evidence: almost no surveyed system
has two receive paths, Flutter is the only one and it has three, and it makes the naming
uninferable — `Post`/`Update` against `Raise`/`Handle` tells a reader nothing about which is
which. Textual, the closest system in domain, delivers everything through one path.

**A mutable field on the message.** WPF's `Handled`. Rejected: a shared mutable flag cannot
express "I want to see this even though someone dealt with it", which is what forced
`handledEventsToo`. A call on a live context has no such problem, which is what the DOM and
Qt do.

**Default stop, with an explicit continue.** Rejected: its failure mode is silent — the walk
dies at the first surface and nothing above ever runs, with no way for froth to tell a
deliberate stop from a forgotten continue. Default pass fails loudly, at the tail.

## Amendment: two things building it settled

Built as #38. Both are refinements of "a message that reaches the tail with nobody taking
it is a `Diagnostic`" rather than departures from it.

**The tail diagnostic is reported once per message type per update.** A cascade of untaken
rising messages otherwise reports one diagnostic per message: two surfaces bouncing one
between them produced 5,000 before the cap tripped, each of them a call into application
code, and a hook that itself drives froth turns that into a hang. Coalescing on the type is
what the finding actually is — what is missing is a *case*, and a case belongs to a type
rather than to any one message, so two hundred rows posting the same untaken message is one
thing to fix. Two different untaken types are two findings and both are reported. The handle
in the detail is the first one seen, as an example of where to look rather than an
enumeration. The precedent is in the same file: `trip` reports the cap once per settle pass
for the same reason.

**There is a way to ask, in two places.** `froth.Rises(msg)` reports whether a type
travels, and `uc.Rises()` answers the same for the delivery in progress. The ADR above
argues that a reader of a type knows its reach without asking — which is true of code that
handles types it declared, and not true of a handler generic over the message type, or of a
surface handling a type froth or another package declared. The pair costs nothing: the
package function is the `riser` assertion the walk already makes, and the method returns the
same field `StopRising` checks, so the two can never disagree with each other or with the
panic. `Rises` follows froth's bare-predicate house style — `Rect.Empty`, `RenderCtx.Focused` —
and matches the verb this ADR and `CONTEXT.md` already use for what a message does.

**`StopRising` panics when the message does not rise.** It is the one misuse that no data
can produce: whether a message travels is fixed by its type, so a surface calling it while
handling an addressed message is wrong about the type it is holding, not handling a
condition. This is [ADR 0014](./0014-a-handle-confers-nothing.md)'s split applied to a
second case — a stranger's handle where a child's belongs is a bug and panics, a surface
that has gone is a condition and reports.

## Parked: a check for a forgotten `StopRising`

A `go vet`-style analyzer could find a surface that acts on a rising message and never ends
the walk. Not built: it is real work for one call, and the tail diagnostic already makes the
mistake visible at runtime. Revive it if the diagnostic turns out to fire often enough in
practice that people want it caught at build time — bring the frequency, not the idea.

## Parked: a root-first pass

Nothing in froth needs a message or a key to be offered to ancestors before descendants.
Should something ever need it, the shape with evidence behind it is Textual's: a per-binding
flag and one `reversed()`, scoped to bindings rather than to all delivery, about four lines.
The general capture phase is not the thing to reach for — the DOM has one nobody defends in
print, WPF's is documented with repeated cautions and a workarounds article, and GTK4 uses
its capture phase only for globally scoped accelerators. Revive on a concrete case.
