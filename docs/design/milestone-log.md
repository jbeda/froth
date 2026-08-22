# Milestone log

Append-only. One entry per milestone, written when it lands. This is the only artifact
carrying **build-time** learning forward — everything else in the repo is design-time.
A session starting a later milestone reads this to find out what the design got wrong.

Keep entries short. What shipped, what changed against the design and why, and what the
next milestone needs to know.

## Assumptions still untested

Load-bearing claims that have never been run. Retire each one as its milestone proves or
breaks it, and say which happened. If one is broken, the ADR it supports needs revisiting
rather than quietly working around.

- **Lip Gloss's compositor plus froth-assigned unique global Z ordinals renders nested
  surfaces correctly.** Read from source, never executed. Supports
  [ADR 0007](../adr/0007-froth-owns-the-stacking-math.md). *Tested by M1.*
- **The Deliver/Apply drain loop terminates under real cascades**, and the iteration cap
  is set somewhere useful — high enough not to trip on legitimate work, low enough to
  catch a cycle quickly. No value has been chosen. Supports
  [ADR 0002](../adr/0002-three-phase-update.md). *Tested by M1, refined through M3.*
- **The headless seam is pleasant to write tests against.** If asserting on subtree
  output is awkward, every later milestone inherits it. *Tested by M1.*
- **Immediate-mode rendering is fast enough at TUI scale.** Every surface renders every
  frame and the compositor flattens and sorts the whole layer set. A few hundred surfaces
  is believed comfortable; nobody has measured. Supports
  [ADR 0003](../adr/0003-controls-are-surfaces.md). *Tested by M7, when a real layout
  produces real surface counts.*
- **Controls-as-surfaces does not explode the surface count in practice.** Related to the
  above but distinct: the question is whether ordinary screens produce tens or thousands.
  *Tested by M2 onward as the example grows.*
- **Adapting bubbles holds up per component.** Sizing, focus and cursor are known to be
  non-uniform; the assumption is that the glue absorbs it without per-component
  surprises. Supports the adapter strategy in
  [finding 4](./kitchen-sink-sketch.md). *Tested by M8.*
- **Bindings declared on the focus chain are sufficient to express real shortcut
  schemes.** No remapping layer exists yet, and context-sensitivity is structural rather
  than predicate-based. Supports
  [ADR 0008](../adr/0008-shortcuts-resolve-to-actions.md). *Tested by M2.*

## M1 — Skeleton

_Not started._

<!--
When it lands, replace with:

**Shipped.** …

**Changed against the design.** … (and update the ADR or spec in the same change)

**Next milestone should know.** …

**Assumptions retired.** … (move them out of the list above, saying proved or broken)
-->
