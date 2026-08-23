# One parent relation, carrying five properties

A surface's parent determines its coordinate origin, its lifetime, its stacking
context, its modality scope, and its routing chain — one link carrying all five.
Clipping is the deliberate exception, a per-surface flag: a layout container wants
to clip its children, while a surface that is a parent purely for ownership or
modality does not.

Win32 splits this into two relations, parent and owner, which are mutually exclusive
and govern different things. That split exists because a top-level window
structurally cannot have a parent — a constraint froth does not inherit. Parenting a
modal to the root is precisely the case Win32 needs `owner` for, and here one
relation covers it. See `docs/research/win32-hierarchies.md`.

## Amendment: the five are not equal

Pressure-tested after M1, against modals, dropdowns, tab strips and reconcile. The one
relation survives; the framing above needed correcting in four places.

**Coordinate origin is the weak one.** froth never translates coordinates. Placement
passes screen rects down, a layout takes and returns screen rects, and a surface reads a
screen rect while rendering. So the parent does not establish an origin — it decides
which rect is handed down as available space. Win32 genuinely translates, which is why
its parent link is heavier than froth's. This is what makes a floating surface cheap:
handing it the screen rect instead of its parent's costs no machinery at all.

**Stacking is authority, not presentation.** A surface may not escape its parent's
stacking context, even to draw over a neighbour. Hit-testing resolves in draw order, so
stacking decides what a click reaches — letting it escape would let a surface take input
on behalf of a subtree it does not belong to. This is where the pressure that produced
Win32's `owner` shows up in froth, and it is refused here. The accepted cost is that a
dropdown parented inside a pane is covered by a later sibling wherever it overflows.

**Parenthood is immutable.** `Reparent` is cut (see the scope note on issue #1), so a
surface's parent is fixed for its lifetime and all five properties are decided once, at
creation. Win32's documentation contradicts itself about whether ownership can be
transferred after creation; froth does not have the question. If reparenting returns it
should be an offer-and-accept handshake, so nothing can have a child forced on it.

**There are now three carve-outs, and a rule about them.** Clipping, layout participation
and placement space are metadata on a handle rather than properties of the link.
[ADR 0010](./0010-presentation-is-metadata-authority-is-the-link.md) states the test that
decides which way the next one goes.

### What the one relation is not asked to carry

Two things route by their own means rather than by parenthood, and neither is a second
relation:

- **A modal's answer** goes to its opener through `Reply`, not up the parent chain. A
  root-parented modal's intents reach root and never pass through the surface that opened
  it, and that asymmetry is deliberate: intents are for ancestors, replies are for the
  opener. Routing intents along the reply address would be exactly the second graph this
  ADR declined.
- **A modal's lifetime** is its parent's, so an opener that dies leaves its modal
  standing. That is accepted rather than fixed by making the reply address a lifetime
  edge, which is the same second relation arriving through a different door.
