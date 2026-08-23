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

- **The iteration cap of 10000 is the right number for real work.** M1 proved the drain
  loop terminates and picked the value; nothing yet builds a tree big enough to press on
  it from either side — a legitimate cascade that trips it, or a cycle that takes
  visibly long to catch. Supports [ADR 0002](../adr/0002-three-phase-update.md).
  *Tested by M3, when reconcile cascades run through it.*
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

Shipped as #2 through #7: the registry and handles, the surface tree with children
created at mount, the three phases and the Deliver/Apply drain loop, rendering through
Lip Gloss's compositor with froth-assigned Z ordinals, a layout placing children and
clipping them to their rects, focus as one handle with raw keys delivered to it, and the
headless harness. `examples/panes` is two panes side by side, tab moving focus, each pane
drawing itself differently for it.

**Shipped.** `froth.New(ctx, root, ...Option)` with `Init`/`Update`/`View` forwarded from
an application model; `Handle`; `Surface` and the `Mounter` capability; `RenderCtx`,
`UpdateCtx` and `ApplyCtx`; `Layout` with `layout.Row`; `Composite(Handle) Frame`;
`OnDiagnostic` with `DiagIterationCap` and `DiagInvalidFocus`.

**Changed against the design.**

- **`Create` takes no options yet.** The sketch's `Create(Surface, ...Option) Handle`
  shipped as `Create(Surface) Handle`; no option in the sketch's list has anything to act
  on until layout metadata, modality and tab order exist. Adding the variadic later is
  source compatible for every caller.
- **A handle names its issuing instance as well as its surface.** Per-registry ordinals
  starting at 1 made two instances issue equal handles, so a handle passed to the wrong
  one named an unrelated surface instead of being the defined drop
  [ADR 0001](../adr/0001-surfaces-addressed-by-handle.md) promises. The ADR was corrected
  in this change.
- **Z ordinals start at 1, not 0.** `NewCompositor` wraps its layers in an implicit root
  layer at Z 0, and a tie there is exactly the undefined order the ordinals exist to
  avoid. [ADR 0007](../adr/0007-froth-owns-the-stacking-math.md) was corrected in this
  change.
- **The drain loop leads with Apply rather than Deliver.** The tree has to exist before
  anything can be delivered into it — the root's own mount is the first Apply any
  instance runs — and the Apply a delivery asks for is the one at the top of the next
  turn. It is the alternation [ADR 0002](../adr/0002-three-phase-update.md) describes,
  entered one step earlier; the ADR was corrected in this change.
- **`Focus` landed on `UpdateCtx` as well as `ApplyCtx`, with a `FocusSelf` on each.**
  The sketch put `Focus(Handle)` on `ApplyCtx` alone. M1's demo needs the surface holding
  a key to move focus, and `NeedsReconcile` — the sketch's route from Deliver into Apply
  — is M3, so `UpdateCtx.Focus` records the move and `apply` commits it, which is the
  sketch's own rule that structural calls on an `*UpdateCtx` commit at Apply. `FocusSelf`
  names no target, so it cannot fail.
- **`RenderCtx` gained `Self()` and `Focused()`.** A surface reading its own focus while
  drawing is what a focus indicator is made of. `Focused()` answers for that surface
  alone — an ancestor on the chain reports false, because ancestor indication is chrome,
  drawn by froth from handle metadata in M2.
- **`CONTEXT.md`'s focus chain entry was corrected.** It described the chain as formed by
  each parent remembering which child is active, which is the restoration state the spec
  puts underneath focus rather than the chain itself. The chain is derived from the one
  authoritative handle; remembered active children are M2's.
- **The harness is `Froth.Composite(Handle) Frame`, on froth itself rather than in a
  test-only package.** A subtree frame needs the whole tree rendered — rects resolve from
  the root down and Z is global — so it is froth's own render path, and `View` is
  `Composite` of the root. A separate package would have been a second path free to drift
  from the first.

**Next milestone should know.**

- **Structure.** Mounting drains a queue of surfaces that have entered the tree; it does
  not recurse over it. An `ApplyCtx` stays valid for the whole Apply pass, so a surface
  can be handed one and create a child under a node the walk has already passed — which a
  recursive mount rendered without ever mounting. `apply` has two levels, the phase and
  the one step in it (`mountEntered`); M3's intents become a second step ahead of it
  rather than an edit to the drain loop, and Apply's fixed order — intents first, then
  structural changes — is a comment until there are two things to order.
- **The cap.** 10000 iterations, and deliveries and mounts spend one budget — there is no
  separate mount budget any more — because a cascade that will not settle can be made of
  either and the two interleave. It is set
  well above anything legitimate rather than close to it: tripping late costs
  microseconds, while tripping early breaks an application that was merely large. It
  reports once per settle pass and abandons what it was draining — the message queue, or
  the pending mounts. A delivery cascade cut short leaves surfaces to mount on the next
  pass with a fresh budget, which `Composite` runs before it draws. A mount cascade cut
  short drops its surfaces permanently and they render unmounted: degraded, but not
  silent, because the diagnostic already fired.
- **Diagnostics are recorded during a phase and handed to the hook after it.** A hook is
  application code, and calling it mid-phase would hand it a froth instance that panics
  on the first thing it looked at — a hook that renders the tree from inside itself is
  the case that forced this, and it is a test rather than the example, whose hook only
  writes down what it was told. Flushing is bounded and non-re-entrant for the same
  reason: a hook that drives froth can provoke the diagnostic it is handling, and did,
  recursively, until `flushRounds` stopped it.
- **A retained context still panics rather than reporting through the hook.** The hook is
  for what froth detects but cannot decide about; a surface using a context in the wrong
  phase is that surface's own bug, caught at the call site where the stack still names it.
- **Layout.** `Arrange(space Rect, children []Child) []Rect`, attached self-relatively by
  `ApplyCtx.SetLayout`. A layout is handed **screen** rects and returns screen rects —
  placement resolves from the root down, and the frame-relative offset the compositor
  wants is `Composite`'s business. `Child` carries only a handle today; it is a struct so
  weight and measured size can be added as fields later, and froth constructs every one,
  so a new field breaks nobody. Returning the wrong number of rects panics: a child with
  no rect has nowhere to render and a rect with no child answers about a surface that does
  not exist. A layout that wants to hide a child gives it a rect of zero size.
- **Clipping is froth's, and happens on the string before it becomes a layer** —
  `ansi.Truncate` per line and a line count cut, so `github.com/charmbracelet/x/ansi` is a
  direct dependency. A Lip Gloss layer takes its bounds from the string it was handed, so
  an oversized string simply makes an oversized layer. Lip Gloss's `Height` pads but does
  not truncate, which is what makes clipping load-bearing: the example loses its bottom
  border in a short terminal, and that is the clip working. Lip Gloss v2 also counts
  border and padding inside `Width`/`Height` — worth knowing before writing chrome in M2.
- **`Composite` returns the zero `Frame` in three cases**: a handle that names nothing, a
  frame before the first terminal size, and a subtree with no cells. A layout can hand a
  child an empty rect, and a buggy one a negative extent — the subtree's rect sizes the
  canvas, and a negative one is a `makeslice` panic from inside Lip Gloss rather than a
  diagnostic.
- **Focus is one handle, and the chain is derived from it.** `chainTo` walks parents to
  the root; reaching the root is also what validates a focus target, which is deliberate —
  a handle froth cannot build a chain to is one it could not route to either. Focus
  commits at the top of Apply, before mounting, so a surface that mounts a child and
  focuses it there overwrites a move requested in the delivery before, which is the right
  way round. A pending move carries its own flag rather than being detected by a
  non-zero handle, since "nothing pending" and "pending a move to nothing" are the same
  handle value. An invalid target is `DiagInvalidFocus` with focus unchanged, not a panic:
  the handle is very often one the application stored earlier, and surviving that is the
  whole of ADR 0001.
- **Routing is one line: a `tea.KeyMsg` goes to the focused surface, everything else to
  the root.** `tea.KeyMsg` is an interface in Bubble Tea v2 covering press and release, so
  both route. M2's binding resolution sits in front of this rather than replacing it —
  [ADR 0008](../adr/0008-shortcuts-resolve-to-actions.md)'s third step is exactly what
  happens here. Delivery reads the deep end of the chain rather than the focus field, so
  focus falls back to the root if the focused surface is not in the tree; unreachable
  until M3 can destroy one, but input going nowhere is the one outcome an application
  cannot recover from.
- **A post to a dead handle is a silent drop today.** The spec's *counted* drop, reported
  through the hook, is M5's along with the command envelopes it counts.
- **Nothing renders during `Update`, and nothing can create a surface during Deliver.**
  `View` and `Composite` are the only render path, and `Composite` applies pending
  structure first, so a test that composites without ever calling `Update` gets a mounted
  tree. Because nothing creates a surface during Deliver yet, the Apply between two
  deliveries has nothing to do and cannot be tested; `Reconcile` (M3) is what first puts
  something in it.
- **The example's quit key is on the panes, not the root.** A raw key reaches only the
  focused surface, so there is nowhere else for it to live until bindings resolve along
  the chain in M2. The example says so rather than hiding it. Its status line sits above
  the body, so the diagnostic the `d` key produces survives a short terminal — the clip
  takes the body first.

**Assumptions retired.**

- **Lip Gloss's compositor plus froth-assigned unique global Z ordinals renders nested
  surfaces correctly** — **proved** (#3). Nesting three deep with siblings at each level
  composites in depth-first order, and an earlier sibling's whole subtree stays below a
  later sibling. [ADR 0007](../adr/0007-froth-owns-the-stacking-math.md) stands, with the
  one correction above: ordinals start at 1.
- **The Deliver/Apply drain loop terminates under real cascades, with a cap value
  chosen** — **proved** (#5). A chain of posts settles inside one update, and a deliberate
  cycle trips the cap in well under a millisecond. The cap is 10000. Whether that is the
  right number for real work is a narrower question, left standing above for M3.
- **The headless seam is pleasant to write tests against** — **proved** (#2–#6), at the
  cost of two conventions now written down in [testing.md](../testing.md). A test must
  post a `tea.WindowSizeMsg` before anything renders, and a fixture surface that a test
  wants to assert about has to record `ac.Self()` in its `Mount`, because `Create` returns
  a handle only to the creating parent and froth has no lookup by name — which is
  [ADR 0001](../adr/0001-surfaces-addressed-by-handle.md) and the no-global-namespace
  non-goal working as intended, not friction to remove. What made it pleasant is
  `Composite` returning rects alongside the string: "is the child where the layout put
  it" is an assertion on a map, not on the shape of a rendered box. Nothing in the suite
  reaches past the seam except `layout/row_test.go`, which tests a published interface as
  the pure function it is — the one second attachment point `testing.md` sanctions.
