# Direction is resolved geometrically, against the frame already drawn

Moving focus left, right, up or down resolves against **where surfaces actually ended up
on screen**, in the frame most recently drawn. No layout implements directional movement
and none is consulted about it: a layout returns rects, and froth reads them.

The geometry is the **rendered frame** — the same rects the test harness resolves against
and the same ones a click will be hit-tested against. There is no other geometry
available. Nothing renders during Update, and asking a layout to speculate is multi-pass
layout, which the spec puts out of scope.

## Considered options

**A layout implements directional movement.** The obvious shape, and it makes every new
layout carry a navigation implementation before it works properly — including the ones an
application writes. A `Row` that forgot to implement it would silently have no arrow keys,
and two layouts would disagree about what "up" means for reasons a user could never see.
Geometry is common to all of them and belongs to whoever already owns the frame.

**Speculative layout: arrange the tree during Update to answer the move.** Removes the
one-frame lag and buys a second layout pass on every keystroke, an `Arrange` called
outside Render with no rect that anything will draw into, and a phase that can observe
geometry the user never saw. Multi-pass layout is out of scope for v1 for its own reasons;
this would import it through a side door for a keystroke.

**Store a navigation graph as surfaces are created.** Explicit and wrong the moment a
layout moves anything: the graph would describe child order, which is not where surfaces
are. It is the same disagreement froth avoids for focus by deriving the chain rather than
storing it.

## Consequences

**A move resolves against a frame that may be stale.** After a resize, the retained
geometry is pre-resize. That is correct rather than tolerated: it is the frame the user
was looking at when they pressed the key.

**Before the first frame there is nothing to resolve against, so nothing moves.** So does
a move from a surface with no cells in that frame. Both are silent — indistinguishable to
a user from an ordinary "nothing that way", and neither is a mistake worth reporting.

**Directional movement is best-effort; traversal is the completeness guarantee.** A
geometric rule can leave a surface that no direction from any other surface resolves to,
and this cannot be fixed geometrically — a surface with no area has no distance to
anything. Tab reaches every stop in its scope, in order, wrapping. Falling back to
traversal order when no candidate exists was rejected: it converts "unreachable" into
"reachable by a move that lies about the direction pressed", which is harder to debug
rather than easier.

**Resolution reads rects and not visibility.** A candidate hidden behind a higher-Z
surface still has a rect and can still win. Computing occlusion is real work for a case
that cannot arise by accident, and the case that matters — a modal over the application —
is handled by truncating the chain rather than by geometry. Paint order is read for one
thing only: deciding which of two candidates *in the same place* is on top, which is the
last of the ordered tests and the one that makes them total.

**The retained frame is froth's, and it is the ordered placement list rather than a map.**
Order is paint order, so the same value answers "which surface is under this point"
topmost-first without a second structure. It can name surfaces destroyed since it was
drawn, so a reader filters against the live tree.

The resolution rule itself — which candidate wins among several — is deliberately not
here. It is a set of tie-breaks applied in order and tuned by use, recorded in the milestone log
and in `direction.go`, and changing it is an ordinary change rather than one that needs a
decision superseded.
