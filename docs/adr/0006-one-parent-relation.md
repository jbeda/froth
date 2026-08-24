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
- **A modal's lifetime** was its parent's, so an opener that died left its modal standing.
  Superseded — see the amendment below, which makes the reply address a lifetime edge
  after all.

## Amendment: the opener edge carries lifetime, and this is a second edge

Settled while slicing M3, against prior art gathered for the purpose
(`docs/research/modal-parenting.md`). The bullet above is reversed: **a modal is destroyed
when its opener is destroyed.**

froth already had an opener edge. `Reply` addresses the surface that opened a modal, and a
root-parented modal's intents reach root without passing through it. What changes is that
the edge now also decides lifetime, which is precisely what this ADR declined as *"the same
second relation arriving through a different door."*

Calling it anything else would be dishonest. **This is Win32's parent/owner split, taken
narrowly.** The reasoning above — that Win32 needs `owner` only because a top-level window
structurally cannot have a parent, a constraint froth does not inherit — was incomplete.
The owner relation also carries the destroy cascade, and that has nothing to do with the
structural constraint. AppKit reaches the same shape by different means: `sheetParent` is
deliberately a separate relation from `childWindows`, so walking the window tree does not
find sheets.

What keeps it from becoming the general case:

- The opener edge is created only by `OpenModal`, never by a caller naming one.
- It cannot be transferred or reparented, like the parent link.
- It carries reply routing and lifetime, and nothing else. Position, modality scope,
  stacking and routing all stay on the parent link, which is why a modal is still parented
  to the root.

So the headline stands with an exception: one parent relation carrying four properties for
every surface, plus an opener edge carrying two for modals alone.

### Why the orphan was not worth keeping

The case for leaving the modal standing is real and is Flutter's documented guarantee — a
dialog's widget *"can be safely removed from the tree before the dialog is closed."* But
Flutter's motivating scenario is **framework churn**: a list item recycled out of the tree,
or an Activity destroyed by a configuration change, where the opener's disappearance is
bookkeeping unrelated to the user's question. froth has no churn. Surfaces are not
recycled and there is no rebuild, so a surface being destroyed is always a deliberate
application decision.

Without churn, the orphan buys a user answering a question nobody is listening to. The
reply had one destination and it is gone.

### What this costs and retires

- **The counted-drop-with-a-diagnostic for an orphaned modal's reply is retired.** The
  modal dies before it can answer, so there is nothing to drop.
- **`ApplyCtx.Post` loses its stated motivation** — it was added so an opener being torn
  down could ask its modal to close itself during `Unmount`. froth now does that. It needs
  a new justification or it does not ship.
- **Opener edges form chains.** A modal that opens a modal makes one, and a surface dying
  takes the whole chain. The cascade runs leaf-up alongside the parent cascade, and a
  pathological chain is the iteration cap's problem rather than a new invariant.
