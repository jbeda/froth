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
froth's bindings and handling their messages would make one path out of two, and the
binding walk would lose the special case at its tail.

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

## Amendment: froth ships no grouping surface of its own

Settled while designing the Environment. The consequences above set the bar — grouping
surfaces are fine so long as they are **explicitly created**, because what this ADR forbids
is froth *inserting* nodes nobody asked for. That stands, and a three-line container in
application code clears it.

froth ships no stock one. A stock wrapper gets reached for reflexively in a way a
hand-written one does not, and reflexive wrapping is how *"the alternative arrives one
convenience at a time"* actually happens. It also costs more now than it used to: every
grouping surface is one more hop on every rising message through that part of the tree
([ADR 0016](./0016-a-message-rises-until-something-takes-it.md)), and froth trees are
already deep because controls are surfaces.

The case that would have made one load-bearing has gone away.
[ADR 0017](./0017-a-surface-and-its-parent-each-get-a-say.md) needed a way for a parent to
give one child different ambient values, which every declarative system does by wrapping;
two Environment slots provide it with no node at all. Revive this if some *other* capability
turns out to be expressible only by interposing a surface — and note that adding a stock
container later breaks nothing, while removing one would.

## Amendment: what the chrome survey confirmed, and what chrome was narrowed to

Ten systems surveyed on chrome (`docs/research/chrome.md`), which moves both halves of the
consequence above from assertion to evidence.

**Chrome-as-a-sibling-view is a real design, and this ADR forecloses it.** Turbo Vision's
`TFrame` is an ordinary `TView` inserted as a child of `TWindow` at the window's full extent.
It consumes no layout space and there is no client-rect concept anywhere; application authors
inset content by hand, and `TRect r(getExtent()); r.grow(-1,-1);` appears in every framed
window in the tree. That is the cheapest chrome model anyone has shipped, and froth cannot
take it, because it is exactly the inner/outer duality this ADR exists to prevent.

**The system that tried the cheap model reversed it.** Terminal.Gui v1 had no general chrome
and smuggled a private `ContentView` into `Window` to fake an inner rect. Its own comments are
the evidence: *"The 'client area' … is a rectangle deflated by one or more rows/columns from
`View.Bounds`. A this time there is no API to determine this rectangle."*, and
`// TODO: Merge these classes` over the duplication. v2 replaced `Bounds` with `Viewport` and
promoted the adornments to first-class. (The rationale is inferred from those comments; no
maintainer statement was found.)

So chrome is drawn by froth from handle metadata, consumes space from the outside, and splits
a **surface rect** into chrome and a **client rect** — the vocabulary is in `CONTEXT.md`.

**Chrome was narrowed at the same time.** Default chromeless and opt-in per surface, matching
tview, whose border is off by default and which no widget turns on internally. And its size
never changes with focus: both mature systems solve focus indication without resizing, and
deliberately — Turbo Vision switches glyphs single-to-double, Textual carries `$border` and
`$border-blurred` for exactly this. Chrome that appeared on focus would shift the client rect
under a surface that had already laid out against it, which is the failure those conventions
avoid.

This paragraph originally cited Textual #877 as a filed bug about focus-border reflow. It is
not one — see the correction in `docs/research/chrome.md`. The narrowing does not rest on it,
and the two conventions above are cited independently.

Chrome has no ADR of its own because it has no code. When it gets one, this amendment and
`CONTEXT.md`'s `Chrome` entry are what it is built from.
