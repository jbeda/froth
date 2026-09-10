# Capture is a claim on every mouse message, and it is lost rather than kept

While a surface holds capture, every mouse message goes to it, wherever the message
landed. Nothing else is consulted — not the hit walk, not focus admission, not what is
drawn on top. Otherwise a mouse message is hit-tested, which is
[ADR 0019](./0019-focus-is-admitted-on-two-axes-and-a-click-must-find-you-first.md)'s
pipeline. Capture is checked first because it is the override: a palette that dismisses on
a click outside itself needs to see a click that hit-testing would have sent elsewhere.

The spec's contract is the starting point: *"Capture is held by at most one surface and
releases on destroy, on modal open, or explicitly, each delivering a capture-lost message
first whose contract is to abandon rather than complete."* Building it against
subtree-scoped modality and an Apply-committed destroy changed three of its details, and
this record is where those are argued.

## Taking it is immediate

`Capture()` and `ReleaseCapture()` live on `SelfUpdateCtx` and `SelfApplyCtx`, and only
there: a surface captures for itself, never for a stranger, which is the ownership rule
rather than a check. Both take effect at once in either phase.

Focus is committed at Apply because deliveries later in the same drain must see the old
focus, and because a move has to be validated against the tree that will exist. Capture
has neither constraint. Its only reader is routing, which runs once per `Update` before
anything is delivered, so no delivery in a drain can observe the change whichever way it is
done. A pending slot would be bookkeeping with no observable effect. The one ordering
immediacy fixes is transfer: `CaptureLostMsg` to the old holder is queued at the moment
the new holder takes over, so it is delivered after the release has taken effect, and the
old holder cannot act on a capture it no longer has.

The shape this has to serve is the common one: capture on the press, release on the
release. A press that captures during its own delivery must be holding capture when the
release arrives as the next `Update`, and it is.

## The new holder wins

Taking capture while another surface holds it posts that surface `CaptureLostMsg` and
replaces it. Win32's `SetCapture` is this shape. The alternatives were refusing with a
diagnostic, or refusing through a bool return; both make the second caller's correctness
depend on a global it cannot see, which is the shape froth avoids everywhere else. Losing
capture to another surface is one more reason to abandon, no different from a modal
opening, and the message already means exactly that.

A surface re-taking capture it already holds keeps it and hears nothing. A surface
releasing capture it does not hold releases nothing and hears nothing, with no diagnostic:
a press that captured and a release that lets go will routinely find a modal took it away
in between, and that is the ordinary case rather than a mistake.

## Destroy releases without a message

The spec says all three releases deliver the message first. Destroy cannot honour that
consistently. A destroy asked for during Apply commits at once through `removeSubtree`, so
a message posted to the holder there addresses a handle that is dead before the drain
runs, and froth's dead-handle rule drops it — while the same destroy asked for during
Deliver would deliver it. Delivering synchronously inside `removeSubtree` would be the only
place froth runs `Update` on a surface outside a drain, and `Unmount` already exists for
the moment a surface leaves.

So the holder leaving the tree clears capture, on every route out including quarantine and
shutdown, and posts nothing. The message's contract is *abandon rather than complete*, and
a surface that has left the tree has no way to complete anything; the abandon is guaranteed
by the destroy itself. Recorded as a divergence from the spec's wording, not its intent.

## Modal open releases only when the modal blocks the holder

*"Releases on modal open"* was written when a modal was root-parented and blocked
everything. [ADR 0008](./0008-shortcuts-resolve-to-actions.md)'s amendment scoped modality
to a subtree, and under that scoping an unconditional release would abandon a drag in one
pane because a modal opened inside another, for no reason a user could see.

The rule is the predicate modality already has: capture is released, with the message, when
the modal that just entered the tree blocks the holder — `blockedByModal`, the same
question a raw key is asked. A modal parented where the holder is not blocked leaves it
alone. Because a fresh modal has no children yet, a holder can never be inside the modal
that just opened, so the two cases are exhaustive.

The same predicate refuses `Capture()` from a surface an open modal has already taken
input away from, with a diagnostic. A blocked holder would be the one state in which
*"every mouse message goes to the holder"* and *"a modal takes input for the subtree"*
contradict each other, and there is no reading under which both hold. It is a diagnostic
rather than a silent no-op because it names the caller doing something froth cannot
honour, which is what every other froth diagnostic does.

## A captured press moves focus nowhere

Focus admission is part of the hit walk, and under capture there is no hit: position was
never resolved, so there is no target to admit. A holder that wants focus asks for it,
which it can already do. Routing the press to the holder and then focusing the surface
under the cursor would have the two halves of one click disagree about where it went.

## Considered and not built

**Delivering `CaptureLostMsg` synchronously on destroy.** Above.

**Refusing a second captor.** Above.

**Capture as a `CreateOption`.** A palette could be created capturing, which is what the
sketch's `ac.Capture()` in `Mount` approximates. Rejected because capture is not a fact
about how a child sits in the composition: it is a claim the surface makes about input,
changes over the surface's life, and has exactly one author — the surface itself — which is
the property ADR 0019 builds the flag ownership around.

## Costs

**A holder learns it lost capture one delivery late.** The message is queued, so between
the transfer and its delivery the old holder can still be handling a message it believes
it captured. Nothing it does in that window reaches the mouse — routing has already moved
on — but state it commits there is state it will be told to abandon a moment later.

**The spec's wording is now wrong in two places** and its intent is kept in both. This
record is the correction; the spec body on #1 is not edited, per the working agreements.

**Nothing in the examples holds capture.** The case it was built for is the palette that
dismisses on a click outside itself, and that is #55's. Until then capture is exercised by
the suite alone.
