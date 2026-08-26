# A surface occupies the rectangle its own output makes

froth composites to a **string**. A surface returns one, froth clips it to the client rect
it was offered, and Lip Gloss flattens the result. What a surface *occupies* on screen is
not its surface rect: it is its **drawn rect** — line count by longest printable line — the
rectangle its own output makes.

The two are the same for a surface that fills what it was offered and differ for one that
underfills. When they differ the drawn rect is the truth, because it is what was painted,
and a click resolves against it.

## The information is not lost, which is the thing to get right

It is natural to assume the cells → string → cells round trip destroys the answer: that a
space a surface painted and a cell it never touched arrive as the same character. Under the
back end froth drives, they do not.

A layer's bounds come from its **content**, never from anything froth passes:

```go
// lipgloss/v2@v2.0.6 layer.go:137
width, height := Width(l.content), Height(l.content)
```

and `uv.StyledString.Draw` (`ultraviolet/styled.go:50-61`) clears that box to empty cells
before painting into it. So inside the drawn rect every cell is opaque — interior spaces,
ragged short-line tails, all of it — and outside it nothing was drawn at all. The boundary
is exactly `lipgloss.Width` by `lipgloss.Height` of the string, which is the same
measurement Lip Gloss itself performs.

froth pays nothing to know it. `clip` already splits the string and calls `ansi.StringWidth`
on every line, unconditionally, so the drawn rect falls out of a walk that exists rather
than needing a second one.

## What froth guarantees about the string

`clip` truncates to the client rect and **pads ragged lines out to the longest one**, so the
string froth hands the compositor is always a solid rectangle no larger than the client
rect.

The padding is froth's, not the back end's. Leaving it to the clear-before-draw above would
work today and would be a second unstated dependency on compositor behaviour — the same
trade `clip.go` already warns about for clipping. A drawn rect should be something froth
computed, not something it observed.

## Considered options

**Composite in cells, above the Bubble Tea boundary.** The escape hatch is real:
`lipgloss.Canvas` implements `uv.Screen`, so froth could implement `uv.Drawable` for its own
surface type and get per-cell control while reusing Charm's buffer and grapheme logic.
Rejected on two counts. It puts `uv.Cell` and `uv.Screen` in `Surface`, the one interface
every user of froth implements, and `ultraviolet` is untagged at `v0.0.0-2026…` with Lip
Gloss and Bubble Tea pinning *different* pseudo-versions of it — froth would be the third.
And it does not remove the string parse it appears to remove: `bubbleadapt` wraps components
whose `View()` returns a string, so every adapted surface would parse its own output into
cells. One parse froth controls becomes N parses it does not.

**Ask the compositor.** `Compositor.Hit(x, y)` exists and walks the z-sorted list high to
low. It works on **bounding rectangles only**, with no per-cell masks, so it answers nothing
froth cannot answer from its own placements — and taking it would hand stacking truth to the
back end, which is what ADR 0007 exists to prevent.

**Retain the composited cell buffer on `Frame`.** froth already builds one and discards it.
Keeping it answers no hit-test question: a cell knows its glyph and style, not which surface
painted it. Ownership needs a parallel structure either way, and the placement list already
is one. It would also put a Lip Gloss type on froth's public API, which is the coupling the
one-file back-end confinement exists to avoid.

**Resolve a click against the surface rect.** Simpler, and wrong wherever surfaces overlap.
A container with no layout stacks every child at its own whole rect; if each child claimed
that rect, only the topmost would ever be reachable by mouse. Floats and modals are the same
shape and are not an edge case.

**Pad every surface out to fill its client rect**, making the drawn rect and the client rect
identical and the question disappear. It breaks the same overlap cases visually rather than
in the hit test: each stacked child would opaquely cover the one before it.

## Consequences

**Hit-testing is exact, not approximate.** The drawn rect is the rectangle the compositor
drew, derived by the same measurement, so the two cannot disagree.

**The space between a drawn rect and its client rect belongs to nobody.** This is not
transparency and does not imply it: the surface is not there, so whatever was composited
earlier stands. In a tiling layout nothing was, and the cells are blank.

**A `Frame` reports surface rects.** That is the assertion a layout test needs — *did the
row give the sidebar twenty columns* — and it is stable under chrome, which splits a surface
rect into chrome and a client rect without changing what the surface rect means. A `Frame`
gains client rects when chrome makes the two differ and a test can tell them apart.

**Directional movement resolves against surface rects.** Under ADR 0013 a move navigates a
structure, and the structure is what the layout decided; a click aims at a glyph. Where
chrome exists the two converge anyway, because a border paints the whole perimeter.

**`Render` is offered a size, not a rect.** A surface cannot position itself — its parent's
layout decides that — so handing it coordinates it cannot act on invites drawing in screen
space. This is an amendment to ADR 0005, which owns the `Render` contract.

## Parked: compositing in cells, with froth's own loop

Not the `uv.Drawable` variant above but the deeper one — froth writes its own compose loop
and keeps a per-cell ownership grid beside the cells. It would make `clip` unnecessary, since
it could write only the cells inside a rect, and it would give a scroller the viewport
clipping Lip Gloss has no primitive for: `Compositor.Draw` ignores the `area` it is handed in
favour of each layer's own bounds, so there is no scissor rect at any level.

Revive it when a surface must clip a subtree's composited output to a sub-rectangle — a
scroller with content taller than its frame is the case to bring — not on the general
argument that cells are a cleaner representation than strings.

## Parked: transparency and non-rectangular surfaces

Not expressible with `Layer` as shipped, at any price short of the loop above:
`StyledString.Draw` clears before painting, so every layer is opaque over its whole box.
Probed rather than reasoned: content `"AB\n  \nCD"` over a field of `X` rendered
`XXABXXXXXX / XX··XXXXXX / XXCDXXXXXX`, the interior spaces punching through.

Revive it for a surface that must show what is beneath it — brought as a screen someone
wants, not as a property someone wants.

## Parked: a surface that draws its own cells

A surface could optionally implement `uv.Drawable` while `Render` keeps returning a string,
so the `uv` types appear only for the surface that asks for them and the base case is
untouched. This is the survivable half of the first rejected option and costs nothing while
nobody implements it.

Revive it for a surface that cannot express what it draws as a string — per-cell metadata is
the likely case. Not for one that would merely prefer to write cells.
