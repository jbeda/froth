# Measure may be called repeatedly; render is called once

`Measure` answers "how big would you be at this size" and must change nothing,
because a layout may ask at sizes it never uses. `Render` is called exactly once per
surface per frame, at final geometry, and *may* commit view state — clamping a
scroll offset to a new size being the motivating case. A surface that does not
implement `Measure` is assumed to accept whatever it is offered, so froth never
speculatively renders anything.

This keeps multi-pass layout reachable — flexbox and similar need to probe children
at several constraints — without demanding purity in the one call where resize
adjustments naturally belong. Declaring the contract now costs nothing. Retrofitting
it later would break every surface written against a once-per-frame assumption,
which is why it is recorded before any exist.

## Parked: layout properties will face the Environment's question

A layout reads handles and metadata. The moment any of that metadata is something *either*
a surface or its parent could reasonably set — a stretch weight, an alignment, a minimum
size — it faces the question [ADR 0017](./0017-a-surface-and-its-parent-each-get-a-say.md)
answers for ambient values: who writes it, and who wins when both do.

Measure and `Arrange` do not have this problem and are not what this is about. They are a
protocol with two distinct roles — the child answers how big it would be, the parent decides
where it goes — so there is nothing to contend. The prior art puts declarative layout
properties on the parent's side of that line: CSS declares `flex-grow` on the child but
writes it from a stylesheet the parent controls, Flutter wraps in `Expanded`, and Qt puts
stretch factors on the layout object outright.

Nothing is parked here yet because froth has no such metadata. Recorded so that whichever
change introduces the first piece of it asks the question deliberately, with ADR 0017's
two-slot answer already on the table, rather than picking a side by accident.

## Amendment: Render is offered a size, not a rect

The contract above says `Render` is called *"at final geometry"*, and left open how much of
that geometry a surface is told. It is told the **size of its client rect** and nothing
else. `Render` takes a `Size`, and `RenderCtx.Rect` is gone with it.

**A surface cannot position itself.** Where it sits is its parent's layout's decision, and
`Arrange` has already made it by the time `Render` runs. Handing a surface coordinates it
has no way to act on invites drawing in screen space — the one thing that would break
froth's property of never translating coordinates, which ADR 0006's first amendment records
as what makes a floating surface cheap. Passing a size makes that property structural rather
than conventional.

**Nothing that draws was using the position.** `RenderCtx.Rect` had one call site in the
repo, a test asserting it panics out of phase. The rect parameter was used only for its
width and height, with one exception: `examples/dir-nav` stashed it so a banner could print
`"%s at %d,%d %d×%d"` while judging directional movement. That is a diagnostic, and
`froth/inspect` is where it belongs if it is wanted again.

**Rejected: letting `Render` return an offset within the rect it was offered.** It would let
a child place itself inside its parent's allocation, which is the parent's decision under the
Measure and `Arrange` protocol this ADR describes, and it would be the second mechanism after
`Float` by which placement escapes a layout. A surface that wants to sit in the middle of
what it was given draws itself in the middle of what it was given.

Where a surface *ends up* is still answerable, and by the party that should answer it:
`Frame` reports one surface rect per handle. See
[ADR 0018](./0018-a-surface-occupies-the-rectangle-its-output-makes.md) for the geometry
this leaves froth with, and for what the string a surface returns is then taken to occupy.
