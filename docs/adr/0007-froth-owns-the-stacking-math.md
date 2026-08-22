# froth assigns global Z ordinals from per-parent stacking contexts

A surface's Z-order positions it among its siblings and never against a surface
under a different parent. Lip Gloss's compositor cannot express that: it accumulates
*position* through the layer tree but not Z, sorting on each layer's raw value
globally, with an unstable sort. Nested layers therefore give no stacking contexts —
a child of a low-Z parent sorts against everything in the tree — and equal values
have undefined order.

froth walks its own tree depth-first and assigns monotonically increasing unique
ordinals, so stacking contexts are real and no two layers ever tie. This also keeps
the compositor a replaceable back-end rather than the source of truth for ordering.
See `docs/research/bubbletea-v2-compositing.md`.
