# Surfaces are addressed by handle, not by pointer

froth routes messages to surfaces by name, and a name gets stored — in the message
queue, in a modal's record of its opener, in the focus field, in an outstanding
async request. A pointer held across time can name a surface that has since been
destroyed, and calling into it is undefined. A handle can be validated at delivery
and dropped. Handles also carry froth-owned metadata — parent, sibling order,
chrome, focus and tab flags — which keeps that bookkeeping out of every surface.

## Considered options

Holding surfaces as values inside a parent's model is the idiomatic Bubble Tea
shape, but it forces copy-back ceremony on every mutation. An interface value over
a pointer escapes that entirely — Bubble Tea copies the interface header, not the
pointee — and is sufficient when there is exactly one surface. It stops being
sufficient the moment a message can name a target, which is what forces this
decision.

## Consequences

Delivery to a dead handle is a defined, counted drop rather than an error. Surfaces
need no per-instance staleness guards — request epochs, owner checks — because
identity is validated once, centrally.
