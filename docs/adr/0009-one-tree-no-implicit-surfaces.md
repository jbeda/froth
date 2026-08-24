# One tree, and every surface in it is one somebody asked for

froth never inserts surfaces of its own. There is a single tree, every node was
explicitly created by some surface or by the app, and encapsulation comes from the
**ownership rule** — a surface owns itself and its immediate children — rather than
from a second tree or a visibility boundary. A parent can see its grandchildren; it
may not touch them.

## Why this is worth a decision

The alternative arrives one convenience at a time. Chrome could be a surface wrapping
the surface it decorates. A layout could insert grouping containers to hold a row. A
transition could wrap a surface while it animates. Each is locally reasonable, and
together they produce the shadow-DOM problem: an *outer* surface that the parent
configures and addresses, and an *inner* structure that actually renders. Once that
duality exists, every question about the tree needs an answer to "which one do you
mean" — who is my parent, what is my rect, which surface does this click hit, what does
tab order traverse. The web has slots, `::part`, and piercing selectors because it
could not avoid this. froth can.

## Consequences

This is already load-bearing in a decision made for other reasons. Chrome is drawn by
froth from handle metadata rather than by a wrapping surface — the stated reason was
surface count and keeping non-surfaces off the focus chain, but the real one is that
chrome-as-a-wrapper is exactly inner/outer.

It also sets the bar for grouping surfaces — layout-only containers, the equivalent of
a `div`. The question is not whether froth should have them; it is whether they can be
**explicitly created**, so they remain ordinary surfaces somebody asked for. Explicit
grouping costs a line of code. Implicit grouping costs the single-tree property, and
with it the meaning of "immediate children" that ownership is defined in terms of.

The cost paid for this is real: froth cannot silently optimise or restructure the tree,
and features that would be natural as wrappers have to be expressed as metadata on a
handle instead.

## Parked: a node that only ever exists at the root

Raised while slicing M3, and not taken. The case for it is real and worth stating so it is
judged on its merits rather than rediscovered: **froth's own actions are inconsistent with
every other action.** An action a surface declares arrives as an `ActionMsg`; an action
froth services arrives nowhere, and which you get turns on whether the identity is
froth-owned — a property invisible at the declaration site. A root-most surface declaring
froth's bindings and handling their messages would make one path out of two, and
`liveBindings` would lose the special case at its tail.

Against it: this ADR, whose argument is that the duality arrives one convenience at a time.
And the placement precedent is unanimous — no windowing system surveyed puts application
scope in a node of the window tree; `GtkApplication`, `QApplication` and `NSApp` are all
application-level objects, and none is a window. froth already has that tier in the
instance, `Query` and `Composite`, and gates it more tightly than any of them.

Revive it if the inconsistency above produces a real bug, or if modality gives froth
something to service that has no sensible home — not on uniformity alone.

## Amendment: the superroot's case has dissolved, and so has part of the case against

Settled while slicing M3, and it moves both sides of the park above.

**The case for it is gone.** It rested on one thing: *"froth's own actions are inconsistent
with every other action. An action a surface declares arrives as an `ActionMsg`; an action
froth services arrives nowhere, and which you get turns on whether the identity is
froth-owned — a property invisible at the declaration site."* That inconsistency no longer
exists. A key press now resolves to an identity only, delivery is a walk from the active
surface up through the root, and froth's own handling is the last stop on it — so a surface
that declares a froth action receives it like any other, and an application pre-empts froth
by taking it. See [ADR 0016](./0016-a-message-rises-until-something-takes-it.md) and
[ADR 0008](./0008-shortcuts-resolve-to-actions.md)'s amendments.

**The case against is weaker than this ADR claims**, and that is worth recording rather than
only the convenient half. The placement precedent above is called unanimous — *"no windowing
system surveyed puts application scope in a node of the window tree"* — and it is unanimous
among windowing systems. It is reversed in the two systems closest to froth in kind: Textual's
`App` is the top of its DOM and its binding chain walks into it, and Flutter's framework
defaults live in ordinary `Shortcuts` and `Actions` widgets in the tree. The unanimity was an
artifact of which family got surveyed.

**Still parked**, on the remaining argument: this ADR's own case that the duality arrives one
convenience at a time, and the fact that the instance is now a demonstrably sufficient home
for the one thing that wanted a node. The revival condition changes with the case. It is no
longer the inconsistency, which is gone, nor modality giving froth something to service with
nowhere to put it — the tail of the walk is somewhere to put it. Revive it if froth acquires
behaviour that must run **before** any surface sees a message and cannot be expressed as a
binding, since that is the one thing a tail position cannot do and a root-most node could.
