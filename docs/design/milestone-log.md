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
  [ADR 0007](../adr/0007-froth-owns-the-stacking-math.md). *Tested by M1.* **Proved by
  #3** — nesting three deep with siblings at each level composites in depth-first order,
  and an earlier sibling's whole subtree stays below a later sibling. One correction:
  ordinals start at 1, because `NewCompositor` wraps its layers in an implicit root
  layer at Z 0 and a tie there is exactly the undefined order the ordinals exist to
  avoid. Retire this line when M1 lands.
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

_In progress._ Sliced into #2 through #7, blockers-first: bootstrap and the compositor,
the registry and tree, then layout and the drain loop in parallel, then focus, then this
entry. #3 is where ADR 0007 is proved or broken; #5 chooses the iteration cap.

### Carried forward from #3, for the entry to absorb

- **`Create` takes no options yet.** The sketch's `Create(Surface, ...Option) Handle`
  shipped as `Create(Surface) Handle`; no option in the sketch's list has anything to act
  on until layout, modality and focus exist. Adding the variadic later is source
  compatible for every caller.
- **The harness is `Froth.Composite(Handle) Frame`, on froth itself rather than in a
  test-only package.** A subtree frame needs the whole tree rendered — rects resolve from
  the root down and Z is global — so it is froth's own render path, and `View` is
  `Composite` of the root. A separate package would have been a second path free to drift
  from the first.
- **Mounting drains a queue; it does not recurse over the tree.** An `ApplyCtx` stays
  valid for the whole Apply pass, so a surface can be handed one by another surface and
  create a child under a node the walk has already passed. A recursive mount rendered
  that child without ever mounting it. Draining the queue of surfaces that have entered
  the tree has no such hole, and it is the shape the Deliver/Apply drain wants anyway.
- **Mounting has its own budget, not the drain loop's cap.** `mountBudget` in `froth.go`
  stops a surface that mounts a child of its own type by panicking. #5 owns the real cap
  and the diagnostic hook to report it through, and should fold this into them.
- **A handle names its instance as well as its surface.** Per-registry ordinals starting
  at 1 made two instances issue equal handles, so a handle passed to the wrong one named
  an unrelated surface instead of being the defined drop ADR 0001 promises.
- **Lip Gloss v2 counts border and padding inside `Width`/`Height`.** #2's example
  subtracted two cells for the border and so drew every box two cells short. Fixed in the
  example; worth knowing before writing chrome.

### Carried forward from #4, for the entry to absorb

- **`Layout` shipped as the sketch drew it**, `Arrange(space Rect, children []Child) []Rect`,
  attached self-relatively by `ApplyCtx.SetLayout`. `Child` carries only a handle today. It
  is a struct rather than a bare handle so weight and measured size can be added as fields
  later: froth constructs every `Child`, so a layout only reads them and a new field breaks
  nobody.
- **A layout is handed screen rects, not container-relative ones.** Placement resolves from
  the root down, so `space` is already absolute and a layout returns absolute rects. The
  frame-relative offset the compositor wants is `Composite`'s business, not a layout's.
- **A layout returning the wrong number of rects panics.** A child with no rect has nowhere
  to render and a rect with no child answers about a surface that does not exist; neither
  has a sane reading. Worth revisiting only if a layout ever legitimately wants to hide a
  child, which would be a rect of zero size rather than a missing one.
- **Clipping happens on the string, before it becomes a layer**, with `ansi.Truncate` per
  line and a line count cut — so `github.com/charmbracelet/x/ansi` is now a direct
  dependency. It has to be froth's: a Lip Gloss layer takes its bounds from the string it
  was handed, so an oversized string simply makes an oversized layer.
- **`Composite` of a subtree with no cells returns the zero `Frame`**, alongside the two
  cases already documented — a dead handle and a frame before the first size. A layout can
  hand a child an empty rect (a row starves the children it cannot fit) and a buggy one a
  negative extent; the subtree's rect sizes the canvas, and a negative one is a `makeslice`
  panic from inside Lip Gloss rather than a diagnostic.
- **Lip Gloss's `Height` pads but does not truncate.** A box styled to its rect still
  overflows when its content is taller, which is what makes clipping load-bearing rather
  than a nicety — the example loses its bottom border in a short terminal, and that is the
  clip working.
- **The example lost its key echo.** Only the root sees messages, and the root is now
  covered by the children the layout placed, so there is nowhere for it to draw one. The
  panes go back to being interactive when routing arrives with focus (#6).

<!--
When it lands, replace with:

**Shipped.** …

**Changed against the design.** … (and update the ADR or spec in the same change)

**Next milestone should know.** …

**Assumptions retired.** … (move them out of the list above, saying proved or broken)
-->
