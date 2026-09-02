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
- **`shift+arrow` is the right default for directional movement.** Driven on
  `examples/dir-nav` in M2. Nothing about it felt wrong and nothing about it is settled:
  the board carries a key that swaps it for the bare arrows, and swapping produced a
  usable argument against them — a text sink can never have a cursor, and the swap key
  cannot swap back while the field holds focus. That is an argument, not a conclusion.
  Supports [ADR 0013](../adr/0013-direction-is-resolved-geometrically.md). *Tested by
  continued use of the board through M3 and M4.*
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

## M2 — Input (in progress)

Written per ticket while the milestone is open, and folded into one entry when it lands.
Everything here is subject to a later ticket in the same milestone contradicting it.

### #9 — Create options and handle metadata

**Shipped.** `ApplyCtx.Create(Surface, ...CreateOption) Handle`; `CreateOption` with
`TabStop`, `TextSink` and `FocusScope`; the metadata stored on the node next to the
parent link, the child list and the layout; keyboard traversal reading `TabStop`, with
`UpdateCtx.FocusNext` and `FocusPrev`. The example's two panes are created with
`froth.TabStop()` and a third is not, and its tab key asks froth to walk rather than
naming a sibling.

**Two calls the ticket left open, and the reasoning.**

- **`CreateOption` is a function over an unexported struct**, mirroring the existing
  `Option func(*options)` that `New` takes. Opacity is structural rather than checked: an
  application can name the type but cannot write a value of it, because the only thing an
  option may touch is `*meta`, which it cannot name — so there is no list of legal options
  to validate against, because an illegal one cannot be spelled. The alternatives were a
  struct with an unexported field and an exported constructor returning an unexported
  interface. Both give the same guarantee; neither is better here, and both would have put
  a second options idiom in a package that already had one. All three share one hole — the
  zero value, a nil function — and froth skips a nil option rather than dereferencing it,
  since panicking inside `Create` would blame the wrong line.
- **The name is `CreateOption`, not the sketch's `Option`.** `Option` was taken by the
  instance options in M1, and sharing one type would let a caller pass `OnDiagnostic` to
  `Create` and have it compile. The sketch was corrected rather than the code.

**Traversal is opt-in, against the spec.** Story 45 says a surface is opted *out* of the
tab order, and the sketch listed `NoTabStop`. Both were written before controls-as-surfaces
was fully absorbed: under [ADR 0003](../adr/0003-controls-are-surfaces.md) a tree is mostly
structural containers, so opt-out means marking the majority of surfaces, and it gets worse
as trees deepen. HTML, Win32 and GTK are all opt-in for the same reason — a `div` is not in
the tab order, `WS_TABSTOP` is a per-control style. froth was inverting the precedent
ADR 0008 otherwise cites approvingly. The flag is `TabStop` and the default is off; the
scope change is recorded on the spec.

**Which flags, and which of them does anything.** `TabStop`, `TextSink` and
`FocusScope` are M2's three; `Weight`, `Float`, `Z`, `Clip`, `Modal`, `RepliesTo` and
`AutoRaise` are not shipped, because each needs a consumer that does not exist and a flag
stored against guessed semantics is a guess later code has to honour. Of the three, only
`TabStop` acts today — the other two are read by nobody until the tickets that give
them behaviour, on the same terms #10 ships `SurvivesModal` inert.

**The debatable call: traversal came with the flag.** An option nothing acts on is an
option nothing tests, and none of M2's three flags could be made observable without
building something. `TextSink` needs bindings to lose to (#10, #11) and `FocusScope` needs
a walk to bound (#12), so the cheapest honest option was to give `TabStop` a walk here.
What landed is deliberately the minimum: depth-first over the child list, skipping
excluded surfaces, wrapping at the ends. **#12 should treat this as its starting point
rather than as settled**, because two of its decisions were pre-empted by shipping it:

- **Whether a container is a stop.** Under opt-in this stopped being a rule and became a
  consequence: nothing is a stop unless its creator said so, the root included, since
  `New` takes instance options and cannot be handed a create option. The question #12
  inherits is narrower than it was — whether a focus scope's own surface can ever be a
  stop within itself.
- **A relative move counts from the move already pending**, not from committed focus, so
  two moves in one delivery advance two stops. That is not the "later request wins" rule
  absolute moves follow, and deliberately so: a relative move is a request about a
  starting point, not about a destination.

**Next tickets should know.**

- **Nothing reads a surface's own metadata, and no context exposes it.** froth reads
  `node.meta` while deciding something; there is no `uc.IsTextSink()` and no reason for
  one yet. #11 should read the focused node's `meta.textSink` from inside binding
  resolution rather than adding an accessor, unless a surface turns out to need to know.
- **Traversal has no notion of a scope, and `FocusScope` bounds nothing.** A dialog
  created with it today cycles into the rest of the tree exactly as if it had not been.
- **Restoration state does not exist.** Focus entering a subtree lands on the next stop in
  document order, not on the child that was last active there.
- **froth binds no key to `FocusNext`/`FocusPrev`.** The example's panes call them from
  their own key handler, which is the same workaround its quit key is: it comes out when
  froth services its own navigation actions.
- **`preorder` is a third walk over the tree**, alongside `place` in render.go and
  `chainTo` in focus.go, and it is the one that will need to see floats when they exist —
  a float is still a child, so it is still traversed, even though its parent's layout
  never sees it.
- **The metadata is on the node, not in the `Handle`.** A handle gets copied everywhere;
  a copy carrying flags would be a second answer to a question the registry already
  answers. `Child` in layout.go is still handle-only — when a layout needs weight, the
  field goes there and froth fills it from `node.meta`.

**Documents corrected in this change.** The sketch's status note and its Create-options
block (name, and which flags exist); its `UpdateCtx` reference row, which gains
`FocusNext`/`FocusPrev`. [ADR 0010](../adr/0010-presentation-is-metadata-authority-is-the-link.md)
gained an amendment: all three flags are about input, which is the word its test does not
use, so it now says why input flags that change only what happens to their own surface are
metadata — and where the line is for one that would redirect input away from where the
tree says it goes.

### #10 — Bindings resolve to actions along the focus chain

**Shipped.** `Action`, `Binding`, `ActionMsg` and the `Binder` capability; resolution in
`binding.go` matching a key press against the focus chain deepest first and addressing an
`ActionMsg` to the surface that declared the winning binding; `destination` in update.go
becomes `route`, which returns a whole envelope because a key press now changes its
message as well as its address. The example's quit key moves to the root as a binding and
fires while a pane holds focus; the panes declare their own binding for the cycle key, and
tab stays a raw key they match themselves.

**`Binding` carries froth's own fields, not bubbles' `key.Binding`.** The sketch wrote
`Keys key.Binding`, which cannot be built: the spec's module boundaries make
`froth/bubbleadapt` the only package depending on bubbles, and story 72 says so outright.
`charm.land/bubbles/v2/key` is itself a leaf package with no non-stdlib imports, so the
cost is not compiled code — it is that importing it drags the whole bubbles *module* and
its eight transitive modules into every consumer's `go.sum`, SBOM and dependency scan.
froth is a library, so that is a cost it imposes on people rather than pays.

The reframe that made the choice cheap: `Binding` was always a wrapper. `Action` identity
and `SurvivesModal` are froth concepts `key.Binding` has no room for, so the only question
was whether the keys/help/enabled part held bubbles' type or froth's. It holds froth's:
`Keys []string` spelled as Bubble Tea's `KeyMsg.String()` renders itself — which is what
bubbles matches on too, so the fidelity is identical — plus `Help` and `KeyHelp`.

`KeyHelp` exists for **lossless convertibility**. `key.Binding` exposes `Keys() []string`,
`Help() struct{ Key, Desc string }` and `Enabled() bool`, all four exported, so a
`key.Binding` converts into a froth `Binding` with nothing dropped — and an adapted
bubbles component's `ShortHelp`/`FullHelp` is exactly what the help query will need to
read. **The converter belongs in `bubbleadapt`, and lives there when that package
exists**; writing it now would mean creating an M8 package for one function.

**Enabled-ness is a `Disabled bool` field, not a method or a closure.** `Bindings()` is
answered fresh on every key press and every help walk, so a surface recomputes the field
from its own state at the moment it is asked; a `func() bool` would buy nothing that a
value recomputed just as often does not already give. ADR 0008 asks only that help can
tell a disabled binding from an enabled one at query time, and a field is queryable
without firing anything. It is phrased as `Disabled` rather than bubbles' `Enabled` so the
zero `Binding` is one that works.

**How resolution resolves ties.** Deepest surface on the chain first; within one surface,
the first binding in the returned slice that lists the key. Both are pinned by tests. A
surface off the focus chain is not consulted at all, and a disabled binding is skipped
entirely rather than counting as a match — the key keeps climbing, and reaches the focused
surface raw if nothing above claims it.

**Only a key *press* resolves.** `tea.KeyMsg` is an interface covering press and release,
and matching both would fire every action twice. A release goes to the focused surface as
a raw key, which is what M1 did with it.

**Next tickets should know.**

- **`Bindings()` is called with no phase in progress and no context.** A surface cannot
  reach froth from inside it, which is the constraint that keeps a synchronous call into
  application code safe. It may be called more than once for one key press. The help query
  (#14) is the second such call and inherits the same rule.
- **`resolve` walks `f.chainTo(f.focused())`**, not `f.focusChain()`, so it agrees with
  where a raw key would have gone when focus names nothing and falls back to the root.
- **#11's text sink goes inside `resolve`**, before the per-surface loop: the focused
  node's `meta.textSink` is readable there, so no accessor has to be added to any context.
- **#12's navigation bindings are froth's own**, and nothing in `resolve` knows about
  froth-owned bindings yet — the walk asks surfaces and nothing else. The example still
  calls `uc.FocusNext()`/`FocusPrev()` from a raw key handler, which is the workaround that
  ticket removes.
- **#14 reads the same `Bindings()` declarations**, so help cannot drift from what fires.
  `Help` is the description and `KeyHelp` the optional key spelling; how several keys are
  joined when `KeyHelp` is empty is the help query's presentation choice, not stored here.
- **`SurvivesModal` is stored and read by nobody.** There are no modals, so nothing
  truncates the chain for it to survive. M4 both truncates and honours it.
- **The remap layer is still unbuilt.** `Binding.Keys` is the default keys with no override
  table in front of it, which is what ADR 0008 says v1 ships; identity is what makes adding
  one later not a rewrite.

**Documents corrected in this change.** The sketch's status note, its `Binding` reference
block (which named `key.Binding`) and its worked `Bindings()` example, which called
`key.NewBinding`. `CONTEXT.md`'s **Query** entry said a query was the *only* synchronous
call into a surface; shortcut resolution is the other one, and both read the same
declarations.

### #11 — Text sink

**Shipped.** `TextSink()` acts: `resolve` asks the focused node whether it is one before
it walks the chain, and a key that counts as typing matches nothing and reaches that
surface raw. The example grows a filter field along the top, created with
`froth.TabStop()` and `froth.TextSink()`, and an application-local layout that gives the
first child a strip and hands the rest to `layout.Row`.

**What counts as "bare printable", exactly.** A press is typing when it produced a
character and no chord modifier was held:

- **Chord modifiers are ctrl, alt, meta, hyper and super.** Shift and the lock keys are
  deliberately not among them: they decide *which* character a key produces, not whether
  it produced one, so `A` and `?` are typing exactly as `a` is — and a `?` help binding
  therefore does not fire while a field has focus, which is the point. `shift+tab` is
  still not typing, because tab prints nothing whatever is held with it.
- **"Produced a character" is `Key.Text` non-empty, or a printable `Key.Code`.** Text is
  Bubble Tea's own answer — its decoder fills it for printable keys and clears it for
  anything carrying a modifier other than shift — and it is empty for enter, tab, escape
  and the arrows, which is what keeps a dialog's escape working while a field is focused.
  The `Code` half covers a press froth was handed rather than decoded from a terminal,
  which carries no text; without it a synthetic `KeyPressMsg{Code: 'q'}` would be a
  shortcut and a real one would be typing.
- Space is typing. `Key.String()` renders it `"space"` rather than `" "`, so a binding on
  it is spelled `"space"` and stops resolving while a sink has focus like any other
  character does.

Rejected: keying on `String()` alone, which cannot tell `?` from a named key without
re-deriving the classification the fields already carry; and treating shift as a chord,
which would make a capital letter unavailable to a text field.

**Space is the one key spelled unlike what it does, and it is the only one.** Ultraviolet's
`Key.String()` returns `Text` unless it is empty or exactly `" "`, so every other character
spells as itself — `shift+1` is `"!"`, `shift+a` is `"A"`, a non-breaking space is itself.
Space alone falls through to the keystroke form, because `" "` would be invisible in a
config file and in a help bar. The consequence to know: **`"space"` and `"shift+space"` are
the only bindings that read like names but behave like typing, so neither fires while a
text sink has focus.** For `"space"` that is plainly right. For `"shift+space"` the `shift+`
prefix suggests a chord, and it is not one — shift+space types a space. Sweeping every named
key constant confirms `Space` is the only one `isTyping` accepts: the arrows, function,
keypad, media and lock keys all carry Private Use Area codes, which `unicode.IsPrint`
rejects.

**Two conventions exist, and this follows the terminal one.** tmux has no `S-a`: shift on a
letter *is* the capital, and `S-` is reserved for keys with a single form (`S-Left`). VS Code
is the opposite — always `shift+a`, case never meaningful. The split is not arbitrary: VS
Code sits on a toolkit reporting physical keys plus modifier state, while a terminal delivers
a *character* and often cannot say whether shift was held. froth lands on tmux's rule exactly
— `"A"` for shift+letter, `"shift+up"` for a key with one form — because it is under the same
constraint, not because it copied it.

**`shift+enter` is expressible and is not typing**, so the newline-versus-submit case an
input box wants needs nothing new: enter prints nothing, so `Keys: []string{"shift+enter"}`
resolves normally even inside a sink. Whether it ever *arrives* is the terminal's business —
the same byte has meant both since 1978, and telling them apart needs the Kitty keyboard
protocol. Bubble Tea requests basic key disambiguation unconditionally, so this works on
terminals that speak it and cannot be made to work on those that do not.

**The check is on the focused surface, not the chain.** `resolve` reads
`f.reg.get(f.focused()).meta.textSink` before the per-surface loop, so a sink that is an
*ancestor* of the focused surface suppresses nothing — the flag is about where input
lands, and input lands at the deep end. Nothing is stored anywhere when focus moves: the
flag is read from the node on every press, so moving off a sink restores matching with no
state to unwind. No accessor was added to any context; nothing outside `resolve` needs to
know.

**Suppression is total, the sink's own bindings included.** A bare printable matches
nothing *anywhere* on the chain rather than skipping ancestors and stopping at the sink.
A surface that both consumes typing and wants a printable shortcut is asking for two
incompatible things; the modified chord is the way to have both.

**The example's filter is not a text input.** It appends `Key.Text`, drops the last rune
on backspace, and draws what it has. No cursor, no selection, no history — the bubbles
adapter is where a real one comes from. It handles tab and shift-tab itself, the same
workaround the panes use, because froth still binds no navigation key of its own. It also
matches only `tea.KeyPressMsg`: a release is the same key arriving a second time and
would put every character in twice.

**Next tickets should know.**

- **`isTyping` is the only place froth interprets a key beyond `String()`.** #12's
  navigation bindings are chords and named keys, so none of them is typing and none needs
  to reason about the sink; a navigation binding on a bare printable would be suppressed
  inside a field, which is correct.
- **#14's help query is unaffected by the flag.** A binding suppressed for a key press is
  still declared, and help lists what a surface declares. If help should say that typing
  wins here, that is a presentation decision the query makes from the metadata, not
  something resolution records.
- **A layout may be the application's.** The example implements `froth.Layout` itself and
  delegates the remaining space to `layout.Row`, which is the first thing in the repo to
  show that composing stock layouts needs no support from froth.

**Documents corrected in this change.** The sketch's status note and its Create-options
block, which said `TextSink` was stored and read by nobody.

### #12 — Tab navigation, focus scopes, and restoration

**Shipped.** `FocusScope()` bounds traversal; every surface records the child focus was
last on and moving focus to a scope lands there; `ActionFocusNext` and `ActionFocusPrev`
are froth's own, with `tab` and `shift+tab` bound to them and serviced by froth. The
example's panes and filter stop calling `FocusNext`/`FocusPrev` from a key handler, and it
grows a focus scope holding two controls.

**Decision 2, revisited and left standing: a relative move counts from the pending move.**
Two `FocusNext` calls in one delivery advance two stops. What changed under it is that
keys no longer take this path — `tab` arrives as one press per `Update`, so each move sees
a committed focus and the rule never fires for typing. What is left is application code
calling the method twice in one handler, and there "advance two" is the only reading of
the second call that does anything. The alternative — the later request wins, as it does
for absolute moves — would make the second call a no-op, which is a silent one. The
asymmetry with absolute moves stands too, and is the point: an absolute move names a
destination and the last one named is what the caller wants, while a relative move names
a distance from wherever the caller has got to.

[ADR 0011](../adr/0011-a-focus-scope-is-closed-to-traversal.md) is new and carries the
boundary rule with its rejected alternatives; the summary below is what building it
settled.

**A scope's own surface is a stop within itself, and in no other scope.** That is the
question #9 left open. The rule underneath it is one line — a surface's scope is the
nearest `FocusScope` at or above it, itself included — and everything else falls out:
`scopeOf(dialog)` is the dialog, so focus resting on a dialog and then tabbing stays
inside it, and the dialog is absent from the walk above because that walk covers only
surfaces whose scope is the one being walked. `TabStop` on a scope therefore admits it to
its own cycle and to nothing else. Had a scope belonged to the scope above it, a dialog
holding focus would tab straight out into the application behind it.

**The boundary holds in both directions, and traversal never crosses it.** The walk above
a scope steps over the scope surface *and everything under it*; the walk inside cycles at
its ends. Considered and rejected: letting `tab` enter a scope as one unit, which reads
well until you notice it makes every stop *after* the scope in child order unreachable —
focus goes in and traversal cannot bring it out, so a scope halfway down a tree eats the
rest of the tab order. Entering is worth an explicit move, and the two real cases already
are one: a dialog is focused when it opens and focus is put back when it closes.

**Moving focus to a scope means entering it — but only from outside.** `Focus(scope)`
resolves down to the scope's remembered child, or its first stop, and this is the only
place in froth where a focus move does not land on the handle it names. It earns that:
the sketch's `ac.Focus(dlg)` wants a control focused rather than the dialog frame, and
every caller would otherwise write the descent by hand. The exemption for a move that
starts *inside* the scope is what makes a scope marked `TabStop` reachable at all — the
walk reaching it would otherwise be sent straight back to the child it just came from.
A scope with no stop anywhere in it keeps focus on its own surface: a defined place to be,
not a rejected move, and not a diagnostic.

**Restoration is written on every node and read only on entering a scope.**
`node.lastActive` is written for every surface on the chain each time focus commits, so a
container's record is correct whether or not it is ever entered — and writing on commit
rather than on the way out means a surface that dies while focused leaves its ancestors'
records already right. Only `enterScope` reads it.

That narrows the acceptance criterion, which said *a container* remembers its last active
child and focus entering *that subtree* lands there. It does remember; what is deliberately
missing is any redirect for `Focus(plainContainer)`, which still lands on the container it
names. Only a scope has a defined notion of being entered, and a container that handles
keys itself has to be able to hold focus — a redirect on every container would take that
away with no way to opt out.

The chain of records is read deepest-first, because "where focus left" means the button
rather than the row the button sits in. Every link is checked against the tree and against
still being a `TabStop`, so a remembered surface its creator kept out of traversal falls
back to the scope's first stop; a remembered surface that is itself a scope is *entered*
rather than skipped, so restoration through a nested scope lands on that scope's stop
rather than on the outer scope's first one. The dead-handle half of the guard is written
but untestable through the seam until M3 ships `Destroy`.

**Focus is still one handle.** Nothing consults `lastActive` to answer where focus is;
`f.focus` and the chain derived from it are the only answer, and a test pins that a
remembered surface neither renders as focused nor receives a key.

**Every froth action is one entry in one list.** `frothBindings` holds the action, its
default keys and its help text together, and `frothOwned` reads that list rather than
repeating the set. An entry with no keys is an identity froth publishes and binds nothing
to, which is what `ActionCancel` is. Only `service` is separate, because it is behaviour
rather than data.

**froth's stock bindings are matched last and overridden by declaring them.** `resolve`
asks every surface on the chain first and only then its own two, skipping any whose action
a declaration on the chain named — enabled or disabled. So rebinding is declaring
`{Action: froth.ActionFocusNext, Keys: []string{"ctrl+n"}}`, disabling is declaring it
with `Disabled: true`, and a surface that wants `tab` for itself just declares `tab` and
wins on depth. No override table, and nothing an application can do to its own bindings
that it cannot do to froth's. A serviced action is delivered nowhere: the declarer asked
froth to move focus, not to be told it did. `route` therefore returns `(envelope, bool)` —
a key froth answered itself produces no message.

**`ActionCancel` is declared with no key.** froth services nothing for it until modals
exist, and a key bound to an action nobody services is a key taken from the application
for nothing. The identity ships now so surfaces and applications spell it the same way.
ADR 0008's amendment is narrowed to quit and a second amendment states the line: froth
binds keys to the actions it services itself.

**Next tickets should know.**

- **`scopeOf`, `scopeOrder` and `stepFocus` are where #13's directional movement plugs
  in.** Directional moves are bounded by the same scope and should read the same three
  functions rather than growing a second notion of "the surfaces you can reach".
- **#13's directional keys are froth-owned in the same family**, so they are entries in
  `frothBindings` and are overridden the same way. Nothing else has to learn about them.
- **#14's help query will want to list froth's own bindings**, which `frothBindings` holds
  with `Help` text already filled in. The query has to apply the same override rule
  resolution does, or help will list `tab` for an application that rebound it.
- **M4 gives `ActionCancel` behaviour**, and it is the first froth action serviced by
  something other than a focus move. `service` is the switch to extend.
- **The example's scope is entered and left by the same key**, `g`, declared on the root
  and on the scope. That is not a mechanism — it is ADR 0008's depth-first precedence,
  with the scope on the chain and deeper than the root while focus is inside it.

**Documents corrected in this change.** The sketch's status note, its Create-options block
(`FocusScope` was listed as stored and read by nobody) and its `UpdateCtx` reference row;
the sketch's `ActionCancel` comment, which claimed a stock key froth does not bind.
`CONTEXT.md`'s **Focus scope** entry, which described the boundary in one direction only
and said nothing about restoration.

### #8 — M1 Go-idiom follow-ups

**Shipped.** The dead `len(cmds) == 0` guard is gone from `settle` — `tea.Batch` already
answers `nil` for an empty set. `chainTo` reverses with `slices.Reverse` and `Composite`
finds its subtree start with `slices.IndexFunc`. The test split landed earlier, in 0fd92f5.

**`fmt.Sprintf` builds every message a human reads, and the reason is not readability.**
`go list -deps .` puts `fmt` in froth's graph already: Bubble Tea and lipgloss both import
it, so no consumer's binary loses a byte by froth hand-assembling its panic strings with
`strconv` and `+`. The one argument for the old style bought nothing, which left only the
cost — a sentence whose shape is invisible until you mentally concatenate it. The rule is
in `CLAUDE.md`; the conversion is uniform, including `Handle.String`, which is the layer ID
the compositor gets and so the only per-frame caller. Measured at ~50ns per call over the
concatenation, against a whole tree rendered every frame; if the immediate-mode rendering
assumption above ever comes under real measurement, this is not the line to look at first.

### #16 — `RenderCtx.FocusWithin()`

**A container can now ask whether focus is at or below it.** `Focused()` answers for one
surface and nothing else, so a container holding the focused surface reported false and
drew itself dormant while the thing inside it took every keystroke. The example's scope
pane was the case that surfaced it; a tab group, a list, and froth's own chrome all want
the same question.

**One focus, and a derived query — not a second focus.** The alternative was letting a
focus scope's surface *take* focus when something inside it is active. That would mean
the control does not have focus, contradicting **Active surface** and sending input
somewhere other than where the tree says it goes. Every toolkit that solves this keeps one
focus and gives ancestors a query: CSS `:focus-within`, Win32's `WM_NCACTIVATE` against a
child that keeps keyboard focus, macOS key window versus first responder.

`FocusWithin` is `slices.Contains(focusChain(), self)` — the chain already existed, so
nothing new is stored and nothing can disagree with the one focus handle. It is a superset
of `Focused` rather than a rival: a surface being rendered is in the tree, so the focused
surface always finds itself on the chain.

**Whether froth has a default opinion about which containers draw active is still open.**
This is the tool, not the policy. The example applies it to its scope pane's border.

### #13 — Directional focus movement

**Shipped.** `ActionFocusLeft`, `ActionFocusRight`, `ActionFocusUp` and `ActionFocusDown`,
froth-owned and serviced beside `ActionFocusNext`/`ActionFocusPrev`, bound to `alt+left`,
`alt+right`, `alt+up` and `alt+down`. Resolution is in `direction.go`; the premise is
[ADR 0013](../adr/0013-direction-is-resolved-geometrically.md) and the scope bound is a
second amendment to [ADR 0011](../adr/0011-a-focus-scope-is-closed-to-traversal.md).

**How a direction resolves to a surface.** A candidate is a tab stop in the source's scope
with both of its edges moved the way the key points. Among candidates, these tests are
applied in order and the first that separates two of them decides:

1. A candidate whose perpendicular extent overlaps the source's beats one whose does not,
   outright.
2. Least distance along the direction of travel, source's leading edge to the candidate's
   nearest one — clamped at zero, so a candidate that overlaps the source scores the same
   as one sitting flush against it rather than better for reaching further in.
3. Least distance between perpendicular centres.
4. Reading order: topmost, then leftmost.
5. Drawn last, and so on top.

**The fifth test exists because the order has to be total, and the first four are not.** A container
that never calls `SetLayout` gets its children its own whole rect, so two stops at
identical rects are ordinary rather than contrived — and they tie on every geometric test.
Without a final test the winner would be whichever `scopeOrder` yielded first, which is
declaration order: the tree structure deciding a geometric move, which is the one input
this rule refuses. Paint order ends it, because two surfaces cannot share a place in the
frame and the one drawn over the other is the one the user is looking at. This is not
occlusion — nothing computes whether a candidate is covered — it is only which of two
surfaces in the same place is on top.

**Reading order is the test that matters most, and it is why neither reference algorithm
was imported.** Android's `FocusFinder` ends on `13 * major² + minor²` and CSS's `css-nav-1`
on `euclidean + displacement − alignment − sqrt(overlap)`; both are tuned for pixel
geometry, where exact ties essentially never occur, and both therefore treat the final
tie-break as an afterthought. froth's rects are **integer cells produced by tiling
layouts**, so a row of equal panes ties on every geometric test. A formula borrowed from
pixels would tie constantly and then resolve by whatever order the placement walk happened
to produce — exactly the "fell out of the implementation" outcome the ticket forbade. The
order must bottom out in something total, and it does.

**Reading order rather than traversal order**, though "when geometry cannot decide, arrows
agree with tab" was the tempting property. Traversal order is *declaration* order: a layout
free to place children in any order it likes can already produce a tab sequence that
visibly jumps around, and tie-breaking to it would import that jump into arrows at the
moment the user is least able to explain what happened. Every input to the rule stays a
rect. This test is a two-way door and is built as one — it lives here and in `direction.go`
rather than in the ADR.

**The candidate predicate is "both edges moved that way",** which is Android's `isCandidate`
with a redundancy removed: its `LEFT` clause reads
`(src.right > dst.right || src.left >= dst.right) && src.left > dst.left`, and the second
disjunct cannot be true while the first is false for any source with width. The two forms
diverge only for a zero-width source, which resolves to "nothing moves" anyway. Requiring
no overlap at all was rejected — #12 made a scope a stop within itself, so a stop can
contain another one, and a no-overlap predicate would silently strand every nested stop.

**`shift` with the arrows, chosen from measurement rather than from reasoning.** Two
earlier answers were picked by argument and both were wrong, so the keys were tested
through a real chain — Ghostty on macOS, over mosh and ssh into tmux — with a program that
prints what Bubble Tea actually receives. That program is now `examples/keys`, because the
question recurs.

What arrives, and what does not:

| combination | result |
| --- | --- |
| `alt+left` / `alt+right` | **never arrives.** Ghostty binds them to `esc:b` / `esc:f` by default on macOS, [explicitly because Terminal.app does](https://ghostty.org/docs/config/keybind), and iTerm2's Natural Text Editing preset does the same. The program receives `alt+b` / `alt+f`. Unbinding it in the terminal is [reported not to reliably work](https://github.com/ghostty-org/ghostty/discussions/7740). |
| `alt+up` / `alt+down` | arrives — only left and right carry word movement |
| `ctrl+left` / `ctrl+right` | **never arrives.** macOS Mission Control takes them above the terminal, so no terminal setting can return them. |
| `shift` with any arrow | arrives |
| bare arrows | arrive |

So `alt` is broken on one axis on macOS by default, and `ctrl` is broken at the OS level.
`shift` is what is left. Its own collision is real — shift with an arrow is text selection
by GUI convention — but froth has no selection concept, and a surface that grows one
declares the keys and wins, which is the path that already exists.

**Bare arrows were reconsidered and rejected on a better argument than the first one.**
The obvious objection — that a list or text field wants them — is mostly answered by
resolution order already: `resolve` walks the focus chain before it reaches
`frothBindings`, so any surface declaring `up` beats froth's stock binding while it holds
focus. The real objection is what that adds up to. In a rich TUI most focusable things
want arrows, so bare-arrow focus movement would work only on the surfaces that do nothing
with them — the gesture would be unavailable exactly where there is most to navigate
between. A modifier keeps it meaning one thing everywhere. An application whose surfaces
do not want arrows opts in with four declarations.

**This is the decision most likely to change**, and #22 is where it gets felt out. That
`examples/keys` has to be a separate program to see keys froth would otherwise resolve is
its own gap, collected as #23.

**Geometry is retained as `Froth.renderedFrame`,** the ordered `[]placement` that `place`
already builds, written on every render. The name is `CONTEXT.md`'s — **Rendered frame** —
and "last frame" is on its `_Avoid_` list. Ordered rather than a map because order is paint
order, which answers both the last of those tests and, when mouse input arrives,
hit-testing topmost-first. It can name surfaces destroyed since the frame was drawn;
candidates come from `scopeOrder`, which walks the live tree, so the filtering is
structural rather than defensive.

**Reachability is best-effort and that is now written down** — in ADR 0013, and as
candidate checks on #21: a warning for a focusable surface with no area, and a mode that
asserts every stop is directionally reachable over an application's own layout.

### #22 — `examples/dir-nav`, a board for judging directional movement

A second example, and the first that exists to be *used* rather than read: a screen shaped
like an application — toolbar, sidebar, detail, two panels and a focus scope, filter bar —
laid out so nothing lines up across columns, plus keys that slide the dividers around under
a fixed focus. It ships no library code and changes nothing about the rule it is for.

**Scoped down from a stress rig to a board.** The first design was an adversarial grid: a
zero-area tab stop, deliberately symmetric ties, a cell reachable only by tab. That was
rejected in the design conversation. The question is whether directional movement *feels*
right, which is a judgement made against an intention, and an intention needs a screen you
recognise. Pathologies belong to #21, which already owns the zero-area warning; planting one
on a board about how navigation feels teaches the wrong lesson. Reachability holes were
dropped for the same reason — hunting for one is fuzzing, not UX.

**Nothing tells an application where focus went.** A directional move is serviced by froth
and delivered nowhere; `RenderCtx` answers `Focused` and `FocusWithin` about the asking
surface and nothing else. So the board learns where focus is the only way available: every
surface records itself as it draws, into shared state, and the banner — created last, so it
renders last in the depth-first walk — reports what they wrote. The same shared record is
what tells the root which divider the size keys should move. This is a tool's need rather
than an application's, and it is collected on #24 (`froth/inspect`) instead of being worked
around twice.

**Both arrow-key answers are in the board, switchable.** `m` swaps froth's `shift+arrow`
defaults for bare arrows by declaring the four actions on the root, which replaces froth's
keys outright — there is no second override table. The argument against bare arrows is on
screen rather than in prose: in that mode the filter bar can never have a cursor, and `m`
cannot swap back while the field holds focus, because `m` is a character there too. The keys
that move dividers are `-`/`=` and `[`/`]`, all unshifted, chosen so nothing depends on how a
terminal reports a shifted punctuation key.

**Two frictions in the layout API, both M7's to answer, both now filed.** `Child` carries a
`Handle` and nothing else, so a layout that places children by role has to know them by
position — `shell` reads its six children by index and documents the order (#25). And a
surface can learn its rect only during `Render`, so the root cannot clamp a divider against
the screen without the layout stashing the space it last divided into the shared state
(#26) — a layout mutating application state during Render, which works and should not be
copied.

**What the board is for is written into it.** The package comment carries a table naming, for
each of `better()`'s five ordered tests, a press where it is the first test that separates the
candidates — and says plainly that the fifth is unreachable, because it needs two surfaces
with the same rect and a tiling board never produces one. That gap closes with floats and
modals in M4.

**Focus changes how a surface looks and never how much room it has.** The focus marker's
column is reserved whether or not it is drawn — a title that grew by two cells when focused
wrapped inside a box that had not moved, which reads as a layout bug and is not one. Worth
remembering wherever chrome draws focus indication.

**Driving it produced two findings within the first session, and both are history problems
rather than tie-break problems.** #28: leaving the sidebar and coming back lands on the row
nearest straight ahead rather than the row you left. #29: an arrow and its opposite do not
undo each other. The second carries a result worth not rediscovering — reversibility cannot
be guaranteed by any purely geometric rule, because a many-to-one move has no single inverse,
and on the board Reply and Forward both move down to Archive. Tuning `better()` toward
reversibility is wasted effort; the answers are an explicit override or a memory.

**No tests.** Deliberate, and decided rather than skipped: a test pinning which surface a
press lands on would pin a board that is supposed to keep changing through M3 and M4, and
would turn every layout tweak into a failure. The rule itself is tested in
`direction_test.go`, where the board's geometry cannot reach it.

**The verdict, and it closes #22 rather than #13.** `better()`'s ordering stands: beam,
then nearest ahead, then straightest, then reading order, then drawn last — probably as
good as a purely geometric rule gets. What the board found wanting is not the order of the
tests but the absence of anything besides them, and that is filed rather than tuned: #29
(reversal, which no geometric rule can deliver) and #28 (a container that remembers your
place). The keys stay `shift+arrow` and stay provisional; the assumption is recorded at
the top of this file rather than retired.

The board stays on `main` and grows through M3 and M4, which is what it was built as a
second example for.

### #14 — The help query

**Shipped.** `Query`, a read-only value with `Focus` and `LiveBindings`, handed out by
`Froth.Query` and by all three contexts; `StockBindings` at package level; and the walk
underneath both, in `binding.go`. The example grows a help bar built entirely from the
query. M2 closes here.

**Resolution and help are one walk, not two that agree.** `liveBindings` collects the
chain deepest-first and `resolve` consumes it, so "help cannot describe something other
than what the key does" is a property of the code rather than of a test. `resolve` no
longer early-exits on the first match; the chain is short and this runs once per press.
**Modal truncation attaches in that one function**, and M4 should cut there rather than
anywhere else.

**Shadowing is per key, not per binding.** A binding whose key something closer to focus
answers to keeps its other keys and loses that one; left with none, it is dropped rather
than reported empty. That is what makes a `[]Binding` enough — no `Shadowed` flag, and a
bar printing `Keys` cannot misreport. The cost: a `KeyHelp` of "↑/k" can disagree with a
filtered `Keys`, and froth cannot fix that.

**A text sink filters the query, and this was nearly missed.** While one holds focus a
bare printable matches nothing anywhere, so a quit on `q` and `ctrl+c` must report only
`ctrl+c` — otherwise pressing the key the bar offers types a letter. The example shows it
without the bar knowing what a text sink is.

**The filter reads the key's spelling, which resolution does not.** Help is asked about
keys nobody has pressed, so `keyIsTyping` decides from the string where `isTyping` decided
from the press; `isTyping` is gone. The two agree for everything a declaration realistically
contains — they part only for a multi-rune `Text` press, which nothing can bind without
spelling it out.

**A disabled binding is reported marked, not dropped**, and keeps its keys so an
application can grey it. Story 39 in the spec says "neither fires nor shows in help"; ADR
0008 says "greying out". The ADR wins, because returning it marked is strictly more
capable and the application is the one that knows whether its bar has room. It also claims
no keys while shadowing, since it does not stop the key resolving further up either.

**`Query` is a struct with an unexported field, not an interface.** Unconstructible,
uncastable, and extensible without breaking implementors. It is what makes the threading
problem disappear: the help bar asks `rc.Query()` and no surface holds a `*Froth`.

**Next milestones should know.**

- **Bindings are still pulled, and M3 should invert them.** `Bindings()` is answered fresh
  on every press and every query, which is what makes drift impossible. Pushing them during
  `Reconcile` and storing them on the node would make resolution and help both reads of
  froth's own state — taking the count of synchronous calls into a surface from two to
  zero, and the re-entrancy guard with it. It cannot be built before M3: `Mount` runs once
  and an existing surface never sees another `ApplyCtx`. The cost to answer there is the
  drift pull makes impossible.
- **The re-entrancy guard is deliberately minimal** — one flag, one diagnostic, an empty
  answer — because it is expected to be deleted rather than grown.
- **Core now hands out a handle the caller did not create**, via `Query.Focus`. See ADR
  0012's amendment. Reading *about* a handle is deliberately absent: addressing is the same
  question for reads and writes, and the ownership table settles both.
- **A superroot surface is tabled, not rejected.** froth's own bindings come from nowhere
  in the tree, which is honest but uncomfortable; `StockBindings` gives them somewhere to
  be asked for without inventing a node. ADR 0009 is what any such change has to answer.
- **The query is hard-coded to `Binder`.** `CONTEXT.md` describes a genus with one member;
  the second capability earns the mechanism.

## M3 — Structure (in progress)

Written per ticket while the milestone is open, and folded into one entry when it lands.
Everything here is subject to a later ticket in the same milestone contradicting it.

### #37 — Handle-scoped contexts: three tiers, reads and enumeration

**Shipped.** `Self()`, `Child(h)` and `At(h)` on all three phase contexts, minting five
concrete types — `ReadCtx` from Render, `SelfUpdateCtx`, `SelfApplyCtx`, `ChildCtx` and
`AnyCtx` — all satisfying a sealed `HandleReader` of `Handle`, `Parent` and `Children`.
`Create` and `SetLayout` moved to `SelfApplyCtx`; `Post` and `Focus` onto the tiers.
`DiagDeadHandle` is new. `UpdateCtx.Post`, `UpdateCtx.Focus`, `FocusSelf`,
`ApplyCtx.Create`, `ApplyCtx.Focus` and `ApplyCtx.SetLayout` are gone, and every example
moved with them.

**The self tier carries the reach set, which ADR 0014's table does not show it carrying.**
`uc.Post(uc.Self(), msg)` is a real pattern — a surface continuing in a later delivery —
and routing it through `At(Self().Handle())` to reach a tier that already permits it would
be verbose for nothing. The tiers nest by capability: self ⊇ child ⊇ open. They still do
not nest by *target*, which is why self is a tier rather than a special case of child.

**Apply gained `Post`, which is what makes five types rather than seven.** The child and
open tiers are one type each across Deliver and Apply, and that only works if their method
sets agree. #17's session-B comment had already given `ApplyCtx.Post` its justification —
a surface being torn down telling something outside froth — so this was owed anyway.

**Minting on a dead handle reports, and that changes what a dead-handle drop looks like.**
Posting to a stored handle whose surface has gone was silent; it is now
`DiagDeadHandle` plus the same drop. ADR 0014 asks for the diagnostic and compares it to
"delivery to a dead handle", which does not report — so the simile was wrong about froth
as it stood, and the ADR's own instruction is what shipped.

**`DiagInvalidFocus` is now unreachable through the public API.** Every path to
`setFocus` mints first, and the mint rejects a handle that names nothing. It becomes
reachable again the moment a surface can leave the tree between a recorded move and the
Apply that commits it, which is #40. Nothing was deleted; the three tests that used to
provoke it now assert `DiagDeadHandle`.

**What stayed on the phase contexts, and why.** `FocusNext`/`FocusPrev` name no handle at
all — they walk from wherever focus is — so they are not handle-scoped. `Focused`,
`FocusWithin` and `Rect` answer about the surface being drawn and are Render's own.
`Query` stays on all three. The contexts are otherwise mint sites, as ADR 0014 intended.

**The examples read better with the mint hoisted.** `self := ac.Self()` at the top of a
`Mount` beats six `ac.Self().Create(...)`, and that is the shape to copy. `Focus` reads
worse than it did: `uc.Focus(h)` became `uc.At(h).Focus()`. Both are input to #32.

**Next tickets should know.**

- **Reads are on `reader`, which every tier embeds.** A read added later lands on all five
  types at once, and the seal means the interface can grow with them.
- **Enumeration is one list.** ADR 0012's amendment partitions froth-parented surfaces
  into a second one; nothing carries that mark until modals exist, so `Children` is the
  only list and the partition arrives with the surfaces subject to it.
- **`Destroy` has a place to go and no work to do here.** It belongs on `ChildCtx`, and on
  `SelfApplyCtx`/`SelfUpdateCtx` if a surface may destroy itself — the tier table answers
  that, but #40 has to decide the second half.

### #38 — Messages rise until something takes them, and the queue drains before Apply

**Shipped.** `froth.Rising` — an embeddable struct whose unexported method nothing outside
froth can satisfy — makes a message type travel: it is offered to the surface it was posted
to, then to each ancestor, ending where a surface calls `uc.StopRising()`. One nothing takes
reaches the root and is reported as `DiagUntakenMessage`. `settle` now applies, drains the
queue **completely**, and applies again only while something structural is pending. The
example gained a rising message: `n` inside the scope posts one from a control, the scope
counts it without ending the walk, and the root takes it.

**A walk is one queue entry, not one per hop.** The climb happens inside a single dequeue,
which is what makes ADR 0016's two ordering promises hold together: messages are delivered
in the order posted, *and* a message posted after a rising one cannot land between two of
its hops. Mutating this into one-entry-per-hop breaks two tests, which is the pair worth
having.

**The tail diagnostic coalesces on the message type, once per settle pass.** Not in the
ADR, found by measuring: two surfaces bouncing an untaken rising message reported 5,000
diagnostics before the cap tripped — 8ms, but 5,000 calls into application code, and a hook
that drives froth turns that into a hang. Coalescing on the type matches what the finding
is: a missing *case* belongs to a type. The reasoning and the alternatives are in ADR 0016's
amendment. `trip` had already made the same call for the cap, one report per pass.

**`StopRising` panics for a message that does not rise**, rather than being a silent no-op.
Whether a message travels is fixed by its type, so no data produces the call — it is ADR
0014's bug-versus-condition split applied again.

**The mutation pass paid, as it did on #37.** Six mutations, every one caught. Five failed
the test that should have caught them; the sixth — treating every message as rising — did
not fail so much as hang the suite for minutes, and chasing why is how the untaken storm
above was found. A mutation that hangs is a finding rather than a broken run.

**Asking whether a message rises is public, in two forms.** `froth.Rises(msg)` and
`uc.Rises()`. Not in the ticket, and **built on a judgement rather than on a case in this
repo** — recorded that way deliberately, since the house rule is to bring a case. The
argument is that a reader of a type they wrote already knows, while a handler generic over
the message type has no way to find out and no safe way to probe, because `StopRising`
panics on the wrong guess. No such handler exists in froth or its examples today; the
nearest candidate, #39's `ActionMsg` sites, knows statically and does not need it. The
method returns the same field the panic checks, so they cannot disagree.
`Rises` keeps froth's bare-predicate style (`Rect.Empty`, `RenderCtx.Focused`) and the verb
`CONTEXT.md` already uses. An `IsRising` spelling was tried first and reverted rather than
start a second naming convention for one call.

**Next tickets should know.**

- **#39 has to add `StopRising` to seven `case froth.ActionMsg:` handlers** across the two
  examples, all of which handle application actions. Ordinary call-site work the ticket
  already implies, listed on #39 so it is not found twice. A *froth* action is the opposite
  case and #39's body covers it: not taking one is supported, because the tail services it.
- **`deliverEntry` is where the tail lives.** #39 puts `service()` at the end of the climb,
  which is the `break` that currently falls straight into `reportUntaken` — an action nobody
  takes should be serviced there rather than reported.
- **`deliver` takes a `*node`** because `deliverEntry` has already resolved one to find the
  parent. A destroy that runs mid-drain (#40) has to invalidate that, or re-resolve per hop.
- **A dead handle ends a walk** rather than climbing from where the surface used to be. The
  parent is not recoverable from a handle that names nothing, and guessing is worse than
  dropping.

### #39 — An action resolves to an identity and rises, and the bindings gain a listing

**Shipped.** Resolving a key press answers which action it means here and nothing else;
`ActionMsg` embeds `froth.Rising` and is posted to the **active surface**, so who acts on it
is settled by the climb rather than by who declared the binding. `service()` moved from in
front of delivery to the tail of the walk, after root. `uc.Invoke(a)` asks for an action by
identity with no key press, through the same envelope a resolved press produces.
`Query.LiveBindings` and the new `Query.Actions` are two views over one walk, both
`[]ChainBinding`.

**The hole this closed is worth stating in one line.** A surface could rebind `focus.next`
and never receive it, because `service` intercepted the identity before delivery. Now it
receives it, and taking the message is the whole of pre-empting froth — no override table,
and framework behaviour falls out of nobody else taking it.

**Two claim maps, not one.** A key a text sink takes as typing is claimed by nobody, so
shadowing genuinely resolves differently for the two views while a sink holds focus. It is
still one walk and one `Bindings()` call per surface, which is what ADR 0008's amendment
insists on; only the claim bookkeeping is doubled. Merging them into one map fails two
tests.

**`route` lost its bool.** With `service()` gone from the front, there is no press froth
answers without delivering, so "nothing to deliver" stopped being expressible. `unparam`
found it rather than a human.

**The untaken diagnostic now coalesces per action.** Every action is one message type, so
#38's per-type coalescing would have reported the first untaken action of a pass and hidden
the rest. `untakenKind` is the one place that decides what the finding is; ADR 0016 carries
the reasoning.

**Disabled entries are listed, and ADR 0008 said otherwise.** Its amendment says "every
enabled action on the chain" two sentences before saying an application may grey a disabled
one in a palette. The second is what shipped, and the ADR is corrected rather than the code
bent to it — help already reports a disabled binding for exactly this reason.

**Nine mutations, nine caught**, failing between one and nineteen tests each. What the
harness itself taught is in `docs/agents/working-agreements.md` under Verifying, which is
where process lessons live.

**The test fixture grew a `takes` field** — the actions a `speaker` ends the walk for. It
had to: with actions rising, every ancestor `speaker` records what a key produced, so
several M2 tests that read "the declarer got it and the focused surface did not" now read
"both did", and the ones about focus navigation had to stop asserting on the root's row at
all. Those rewrites are the change being visible in the tests rather than tests being
loosened.

**The example gained a one-line command palette.** The filter field matches what it is typed
against `Query.Actions()` and enter invokes the first match. It is there because `Invoke`
and `Actions` would otherwise have shipped with no caller, and because it demonstrates the
two rules that separate the listing from help in the one place they matter: it keeps a
keyless action, and it is a text sink itself.

**The review found the constraint had drifted from its own home.** ADR 0008 said
`SurvivesModal` must be honoured "inside `liveBindings()`", and this ticket renamed that
function — so the one sentence M4 has to obey was pointing at nothing. Renaming a function
an ADR names by hand is a documentation change, and `grep` for the old name is the check.

**Two views, one walk, and now a test that says so.** Nothing pinned "one `Bindings()` call
per surface", which is the actual anti-drift invariant behind the whole two-views design —
the ⊆ test that existed would pass just as well if each view walked separately. A counting
`Binder` asked once per view and once per key press is what pins it.

**"Catalog" was never froth's word.** ADR 0008's amendments use it throughout, the code and
`CONTEXT.md` say listing and `Query.Actions`. The ADR now says so rather than being
retitled, because rewriting what an amendment said is worse than recording that its term
did not survive contact.

**`SurvivesModal` is still unread, which is the right amount of work.** #39's last
acceptance criterion says the flag must be honoured inside the one binding walk and nowhere
else. Modality does not exist until M4, so there is nothing to honour yet; what this ticket
owed was leaving exactly one place for it to go, and `chainBindings()` is it. The constraint is
recorded in ADR 0008 and will be met or broken there.

**Untouched and still open on #17's session-B comment:** nothing. Both catalog bugs and the
dropped `liveBinding.from` are fixed here.

### #40 — A surface leaves the tree: destroy, cascade, `Unmount`, shutdown and quarantine

**Shipped.** `Destroy` on all three tiers that may have it — `Self().Destroy()` during
Deliver or Apply, `Child(h).Destroy()`, nothing on `At(h)`. Destruction cascades to the
whole subtree. `Unmounter` runs leaf-up, once per surface, whichever of the four routes out
it leaves by. `Froth.Shutdown()` runs that same walk once from the root. A surface that
panics inside `Update`, `Render`, `Mount`, `Unmount` or `Bindings` is reported and removed
and the program continues.

**One walk, four routes.** Everything above goes through `removeSubtree`. That was the whole
argument for one ticket rather than four, and it held: nothing about shutdown is special
except that it starts at the root and that posting from it is reported as pointless.

**Nothing is deleted until every `Unmount` has run.** That is what lets a child reach its
parent while tearing down, which is the entire reason the walk is leaf-up rather than the
mirror of the mount order. Collecting the subtree, running teardown over it, and only then
deleting is three passes where one would have been cheaper and wrong.

**Apply gained a third step and the order is a decision.** Destroy, then focus, then mount —
recorded in ADR 0002's amendment. Committing destroys first is what gives `DiagInvalidFocus`
a reachable path again, which ADR 0014's amendment had flagged as owed to this ticket.

**Focus rehoming reuses `enterFocusScope` rather than inventing a rule.** When focus is inside
what is going, it lands on the nearest surviving ancestor and then descends the way entering
a focus scope does. It reports nothing: nobody asked for the move, froth is repairing a
chain a destroy broke. Note the consequence a test caught — if the surviving ancestor is
itself a tab stop, focus lands *on it* rather than descending past it, because that is what
`enterFocusScope` already means. The alternative would have been a second rule about where focus
belongs, and froth has one.

**froth's own panics are now an unexported error type.** Quarantine has to tell a surface
that failed from a program that violated froth's contract, and both arrive as a panic out of
the same five callbacks. Every deliberate panic inside froth goes through one `bug()`
constructor and quarantine re-panics what it recognises. Reasoning in ADR 0014's amendment.
The visible cost: a test recovering a froth panic gets an error rather than a string, so
`panicText` in `helpers_test.go` renders one for assertions.

**`Unmount` mirrors the mount step, so a surface that never mounted is never unmounted.** A
surface created and destroyed inside one Apply, before the mount queue reached it, has
nothing to undo. `node.mounted` records that the step ran, whether or not the surface
implements `Mounter`.

**A quarantined surface is marked before its destroy is recorded.** The destroy commits at
the next Apply, so without the mark the surface would be delivered to, drawn and asked for
bindings in between — and "not called again" would be false for exactly one turn. The mark
is what the criterion rests on; the removal is what it means afterwards.

**No quarantine site is handed a phase token; every one of them reads the phase.** The first
shape passed the caller's token through, and `Bindings` — the one callback froth makes from
whatever phase it happens to be in — had none to pass, so it passed a zero one. A query
asked from Apply then removed the panicking surface on the spot and handed that zero token
to a surviving descendant's `Unmount`, whose first context use raised a froth panic that
quarantine correctly re-raised: the program died. `f.current()` mints the token for the
phase froth is in, and destroy no longer threads one at all. Found by the spec review;
`TestASurfaceThatPanicsAnsweringAQueryDuringApplyIsRemoved` pins it.

**Next tickets should know.** Destroying an *already-mounted* surface immediately during
Apply is not reachable from anything M3 has shipped: a surface's `Mount` runs before its own
children's, so no mounting surface can name a child that has already mounted. #41's
`Reconcile` is the first callback that can, and it is the first real test of the immediate
path — what is pinned today is the record-and-commit path and the Apply-time path through a
`Mount` panic. Left for M4 with the rest of the modal work: whether froth services
`ActionCancel` by closing one, and destroy-then-create-a-replacement ordering in one Update
callback.

**Thirty-four mutations, thirty-three caught.** Three real gaps came out of the first sweep and
all three are now pinned: nothing destroyed anything from inside an `Unmount`, nothing
asserted that a surface panicking while drawing takes its subtree off the frame, and
nothing rehomed focus that a flat mutation could not survive. The surviving mutant is
equivalent — the idempotence guard at the top of `Shutdown` cannot be observed, because the
first call empties the registry and the second finds no root to walk. It stays as the
stated contract rather than as load-bearing code.

**Quarantine takes the root out, and nothing else can.** A surface asking to destroy the
root panics; quarantine is not a surface asking, and the alternative is to go on calling
something that has already failed. What is left is an instance with an empty tree, which
draws nothing and delivers nothing — the same end state `Shutdown` reaches. Marking it shut
down as well was written and then deleted: no test could observe the difference, because an
empty registry already makes every entry point inert.

**A defect the render mutation found, older than this ticket.** `Composite` flushed
diagnostics through `applyPending` and then drew, so anything *drawing* reported — a dead
handle minted from a `RenderCtx`, a reentrant query, and now a surface that panicked — waited
for whatever drove froth next. `Composite`'s own doc comment claimed otherwise. It flushes
after the frame is built.

**The guard against drawing a quarantined surface looked unreachable and is not.** It was
deleted on that reading — `Composite` applies pending structure before it draws — and the
`Bindings` case above put it back: a surface drawn early in a frame can ask a query, a query
walks the focus chain, and a surface further along the chain has not been drawn yet. All
four skips are pinned by tests now.

**`surface.go` now holds every interface a surface may satisfy**, and that is the file's
job rather than a place `Unmounter` happened to fit: `Surface`, `Mounter`, `Unmounter` and
`Binder`, which came over from `binding.go`. What a surface can be asked is one question,
whatever part of froth does the asking, and a reader looking for the extension points
should find them in one place. `Layout` stays where it is — it is implemented by a layout,
not by a surface.

**The focus helpers read from outside `focus.go` now say focus**: `rememberFocusChain`,
`rehomeFocus`, `enterFocusScope`, `focusScopeOf`, `focusScopeOrder`, with their two
file-local siblings renamed for consistency. `rehomeFocus` moved into `focus.go` with them,
which was half the problem — it was defined next to the walk that calls it rather than next
to the rule it reuses. ADR 0011 named `scopeOrder` by hand and was corrected; the log's
older entries keep the old spelling on purpose, because they are dated records.

**The example gained `x`.** The focused pane removes itself, its `Unmount` says so in the
notes pane, and focus moves to the nearest survivor rather than back to the root. `main`
calls `Shutdown` after `Run` returns, which is the demonstration that matters: the walk is
the guarantee, and an application that only calls it sometimes does not have one.

### #41 — `Reconcile`, and a list that adds and removes its own items

**Shipped.** `Reconciler` is a fourth optional interface on a surface, asked for with
`Self().Reconcile()` from Deliver or Apply and called during Apply with a full
`ApplyCtx`. Apply's three steps became destroy, focus, and reconcile-and-mount together.

**The name went to the callback, not to the phase.** Collapsing the two — `RequestApply()`
and an `Applier` interface, or renaming the phase to Reconcile — was considered and
rejected. `Mount` and `Unmount` already run during Apply and are named for what they do,
so `Reconcile` is their third sibling rather than the odd one out; and the phase is
froth's commit step, which is not reconciliation. The request is `Reconcile()` on the self
tier rather than the sketch's `NeedsReconcile()`, which reads as a getter — the same
correction `Handled()` got while M3 was being sliced. The sketch has been updated.

**Reconcile and mount share one drain.** They were nearly a fourth step and are not,
because each makes work for the other and a request from inside `Reconcile` has to be
serviced somewhere. One loop that takes a reconcile if there is one and otherwise a mount
settles all three cases, and both spend from the settle budget — which is what turns a
surface that never stops asking into a report rather than a hang. Reasoning in ADR 0002's
amendment.

**A request is never immediate, and that is the one place `Reconcile` differs from
`Destroy`.** Destroy executes on the spot during Apply; reconcile records in every phase,
because running it where it was asked would put a surface inside its own callback. What
this buys is the criterion about asking from inside `Reconcile`: the second call is an
ordinary queued request against whatever the tree is by the time it comes round.

**The bit found its caller.** ADR 0002 parked "the bit as well as the list" for destroys,
asking for a case rather than a preference. Reconcile is that case and it is a *separate*
list — asking twice must reconcile once, where destroying twice may find the second
subtree already gone. Two details are load-bearing and both are pinned: the bit is cleared
*before* the call, which is what makes a request from inside `Reconcile` a fresh one, and
it is cleared alongside the list when the budget abandons a pass, without which a surface
left marked could never ask again.

**A surface that asks and has no `Reconcile` method is reported.** The method is the
opt-in, so the request names a callback that does not exist. `DiagNoReconciler` is not
coalesced the way an untaken message is: one per request is one per keypress, not the
thousands a cascade can produce.

**This is the ticket that reached the immediate destroy path #40 flagged.** A surface's
`Mount` runs before its children's, so nothing before this could destroy an already-mounted
child during Apply. `Reconcile` can, and `TestAReconcileDestroysAnAlreadyMountedChildAtOnce`
pins it: the child is gone from `Children()` before the callback returns and its `Unmount`
has already run.

**Focus during Apply was immediate already, so create-and-focus needed nothing.** A
reconcile creates a child and focuses it in the same callback, because `setFocus` asks only
that the handle be in the tree and mounting is a later step — the same thing `root.Mount`
in the example has always done. Worth recording because it was the open question that
decided reconcile's position after focus rather than before it.

**The demo went into the existing focus scope rather than into a new column.** It was built
as a separate tab strip first and that was wrong twice over: a vertical list beside four
other panes is not a tab strip, and the scope already held a list, so the two were the same
demo drawn twice. The scope now keeps the labels it should be showing and which one should
hold focus, and `match` — shared by `Mount` and `Reconcile` — destroys what is present and
unwanted, creates what is wanted and absent, and puts focus where it says. `+` and `-` are
the scope's own bindings, so they resolve only while one of its items holds focus. The
items never remove themselves: a child that destroyed itself would leave the scope's idea
of the world wrong, which is the sharpest edge in the desired-state shape and worth showing
by avoiding rather than by hitting.

**Removing showed who owns the successor, and the answer is the application.** The first
version removed the *last* item and focused the new last, so removing anything looked like
focus jumping to the end. It now removes the item that holds focus — read with
`uc.Query().Focus()`, which is the only way a container learns where the keyboard is — and
names the neighbour above it, or below if the first went. froth's own rehoming is a repair
for what nobody decided, and a reconcile is where a surface overrides it: it runs after
Apply's focus step and `Focus()` during Apply is immediate, so the surface that knows the
list has the last word. Recorded in ADR 0002's amendment.

**One item sends two rising messages that stop in different places.** `t` posts a
`tallyMsg` the scope takes; `n` posts a `noteMsg` the scope counts without ending the walk,
and the root takes. The pair is the argument for placement made from one surface:
everything a modal contains is parented to the root, so the root sees every rising message
from inside every dialog, and behaviour that is not genuinely global belongs on a surface
that strangers' chains do not pass through.

**The scope claims no focus while mounting.** Its `Mount` builds two items and then clears
the field that says which should hold focus, because `match` is shared and would otherwise
focus one — overruling the root, which had already said focus starts on the left pane.
Caught by the spec review after the example silently started up focused in the wrong place.
Worth naming as the general shape: a surface that grabs focus in `Mount` beats whoever
built it, since children mount after their parent.

### #30 — A surface declares its bindings, and froth stores them

**Shipped.** `SetBindings` on the self tier in both Apply and Update. froth stores the set
against the handle and `chainBindings` reads it. `Binder` is gone, and with it the
re-entrancy guard, `DiagReentrantQuery`, and the quarantine site for `Bindings`.

**froth now makes no synchronous call into a surface.** That was the prize the M2 log
entry named — "taking the count of synchronous calls into a surface from two to zero" —
and the two turned out to be one inversion, because resolution and both queries had always
read the same declarations.

**The ticket asked the wrong question, and the answer was better than the one it wanted.**
Its title is "push bindings during Reconcile", and its stated blocker was that M2 had no
hook to push from. `Reconcile` was not the hook that mattered. #37's `SelfUpdateCtx` was:
`Disabled` is a function of fine-grained state, that state changes in `Update`, and the
declaration belongs three lines from the field it describes rather than a queued request
and a phase boundary away. `Reconcile` is now one push point among three, not the designed
one.

**The write is immediate in both phases, and the reasoning is about when resolution runs.**
A key resolves exactly once, in `route`, before `settle` starts; `Invoke` skips resolution
entirely. So a declaration made while delivering cannot change the meaning of the key being
handled, and `settle` always runs a final Apply before returning, so a recorded write would
have committed before the next key arrived anyway. The only observer that could tell them
apart is a query asked later in the same settle pass — which pulling answered from the
surface's current state, so immediate is the option that changes nothing.
`TestADeclarationMadeWhileDeliveringIsVisibleLaterInTheSamePass` is the only place that
seam is visible, and it exists because nothing else could see it.

**ADR 0014 predicted this would record and it does not.** Its phase rule listed binding
declaration among the things that "record cleanly and commit at Apply". The corrected rule
is in its amendment: Update's operations record only when something downstream needs the
tree to hold still, and a declaration is not structure.

**The drift is accepted, not mitigated, and it has a test that says so.**
`TestASurfaceThatChangesStateWithoutDeclaringKeepsWhatItLastSaid` pins the thing pulling
made impossible. Two mitigations were weighed and rejected: froth requesting a reconcile
after any delivery to a binding-declaring surface, which breaks #41's invariant and runs a
structural callback per keystroke; and `Binder` surviving as a one-time seed read at Mount,
which keeps every deletion but leaves a method whose later return values are silently
ignored. Reasoning in ADR 0008's amendment.

**`n.panicked` changed meaning and needed a test it did not have.** Pulling skipped a
quarantined surface by not making the call. Storing means skipping a *read of live data* —
the declarations sit on the node until the destroy commits at the next Apply — so the skip
is now load-bearing rather than incidental.
`TestAQuarantinedSurfaceStopsContributingItsBindings` pins it, and removing the guard fails
it.

**The stored set is copied.** Pulling handed froth a fresh answer each time and had nothing
to alias; storing means a surface can write through the slice it passed. `Keys` is cloned
too, which is what `StockBindings` already did for the same reason.

**Two tests died and one of them is a real loss.**
`TestASurfaceThatPanicsAnsweringAQueryDuringApplyIsRemoved` pinned the zero-token bug from
#40 — quarantine reading the phase rather than being handed one. `Bindings` was the only
callback froth made from an arbitrary phase, so the bug is now unreachable and the test
with it. The `f.current()` mechanism stays and is still used by the teardown walk, but it
no longer has a regression test of its own.
`TestASurfaceQuarantinedMidFrameIsNotDrawnInIt` went the same way: nothing calls into a
second surface during Render any more, so a surface can no longer be quarantined mid-frame
by somebody else.

**Push exposed a bug in `Update` that predates it, and Apply now runs before routing.** A
message is addressed against the tree, and `route` was reading one that had never been
applied: on a fresh instance the root is registered but not mounted, so it has declared
nothing and has not had its chance to move focus. Pull hid half of this, because an
unmounted surface still answered `Bindings()` — asking a surface to declare before `Mount`
was itself dubious, and push made the answer honest and the bug visible. The other half was
never hidden: a first-ever key press has always gone to the root rather than to what `Mount`
focused. `Update` now runs `f.apply` before `f.route`, sharing one budget with the settle
that follows, and `settle` gained a `settleWith` for that.

Blast radius is exactly the broken case. Every message after the first finds the new Apply a
no-op early-return, because `settle` always ends on one. Non-key messages were never
affected at all: `route` sends everything that is not a key to the root unconditionally, and
the root is a live handle from `New`, so an async result arriving first landed correctly and
was delivered after the root's `Mount` in any case. Found by the review;
`TestTheFirstMessageIsAddressedAgainstAMountedTree` pins both faces and both subtests fail
without the new call.

**`SetBindings` takes a slice, not a variadic, and the call sites decided it.** Variadic
was written first and was worse at every one of the seven: five are multi-element literals,
where variadic forces `froth.Binding{` to be spelled per element instead of once, and two
pass a computed set, which variadic makes `bindings()...`. The slice form is also what the
old `Bindings()` returned, so the surfaces that moved kept the literal they already had.
`Create`'s variadic option list is not the same case: there zero arguments is the common
call, and here a surface with no bindings simply does not call it.

**Both dynamic examples were verified by scaffolding, and nothing pins them in CI.** A
throwaway test drove each through the public API and asserted that `Disabled` follows the
filter's text and that the arrow-key set follows `bd.bare`; deleting the re-declare line
failed both. The tests were then deleted, per the working agreement. The examples remain
untested by construction, so a future change that drops a `SetBindings` call in one of them
will be caught by reading rather than by the suite.

**Every example surface that declared bindings changed, and two of them are the point.**
`panes/filter.go` recomputes `Disabled` from text that changes on almost every key, so its
`Update` calls the handler and then declares — one call after the fact rather than one per
branch, because the branches are what would drift. `dir-nav/root.go` swaps the arrow-key
set on `actionArrowMode` and declares in that case arm. The other four are static and
declare once in `Mount`, including two leaves that grew a `Mount` for nothing else — which
is the cost of killing `Binder`, paid where a reader can see it.

## M4 — Modals and mouse (in progress)

### #44 — `Render` is offered a size, and a drawn rect follows from what it returns

**Shipped.** `Surface.Render(rc *RenderCtx, s Size) string`, with `RenderCtx.Rect` gone.
`clip` now pads a surface's ragged lines out to its longest one and returns the size of
the rectangle that makes, which `placement` keeps as its drawn rect.

**The churn was the reason it went first, and it was almost all fixtures.** Every `Render`
in the suite and both dynamic examples changed signature; three tests changed meaning.
Two in `layout_test.go` asserted a child's position by having the surface stash the rect it
was handed, and they now read it from `Frame.Rects` — which is the answer ADR 0005's
amendment points at, and a better test besides, because it asserts on what froth publishes
rather than on what a fixture recorded. `registry_test.go`'s `block` stashes a size.

**No surface lost anything it was using.** The position had one real consumer,
`examples/dir-nav`'s banner, which printed `"%s at %d,%d %d×%d"` for the focused surface
and now prints `"%s %d×%d"`. The board's `spot` carries a size. Judging a directional move
still works, because the geometry a move surprises you about is the size of the boxes; where
they sit is the layout's arithmetic, which the board's own splits already say.

**Padding is invisible in the frame, which is why it needs a direct test.** `uv.StyledString.Draw`
clears a layer's box before painting it, so the composited string is byte-identical with the
padding and without it. The suite passing unchanged is *not* evidence for that — no fixture in
it was ragged — so `clip_test.go` is in package `froth`, a third entry on `docs/testing.md`'s
self-contained list, and it names Lip Gloss to assert that froth's drawn rect is what the
compositor measures from the same string. That is the one place outside `render.go` that knows
the back end, and it is there because the agreement is the assumption. Found by the review.

**The suite gained its first ragged surface, and it pins the rejected option rather than the
padding.** `TestASurfaceOccupiesWhatItDrewAndNotWhatItWasOffered` draws three columns of a
four-column offer over a parent's field and asserts the fourth column is still the parent's.
Removing the padding does not fail it — the compositor clears the box either way, which is the
point above — but padding out to the client rect, ADR 0018's rejected option, does.

**`geom.go` gained what the ticket did not ask for, and it is a gap the ticket exposed.**
`Size.Empty` first: every `Render` that guarded on `r.Empty()` needed the same guard on what it
is now offered, and `clip`'s early return is the same question, so `Rect.Empty` now delegates
to it. Then `Origin`, the counterpart of the `Size` accessor that already existed — without it
a `Rect` decomposed one way and not the other, and `Point` was public API nothing could reach.
Then `String` on all three, because `Handle`, `Diagnostic` and `phase` all have one and
geometry is what a layout diagnostic will be about: a rect prints `40×12 at 0,0`.

Deliberately absent: containment. A click needs `Contains`, and #53 is where it earns a name
next to the thing that calls it.

**A truncated line is measured again rather than assumed.** `ansi.Truncate` to a width can
return a narrower line when a wide grapheme straddles the edge, so taking the offered width
as the drawn width would have over-reported by a column for exactly the case `clip`'s
existing comment exists to handle.

**Nothing reads the drawn rect yet.** #53 does, and the wiring test — that froth clips every
surface's output and resolves a click against what it drew — belongs there.

### #47 — How many focus axes, and what a click can reach

A design session with four prior-art surveys, no code. It produced
[ADR 0019](../adr/0019-focus-is-admitted-on-two-axes-and-a-click-must-find-you-first.md),
an amendment to [ADR 0011](../adr/0011-a-focus-scope-is-closed-to-traversal.md), three
`CONTEXT.md` entries (Flags, Click focus, Mouse transparent), and four survey files in
`docs/research/focus-axes-*.md` covering Win32, the web platform, Qt/GTK4/Android/Flutter,
and Textual/Terminal.Gui/tview/prompt_toolkit/Ratatui.

**The answer is not three axes.** The ticket and the parked comment on #1 both proposed
walkable / focusable-by-name / clickable as a trichotomy. It is one universal capability and
two independent admissions, because the web's three-layer shape only earns its form when the
base can be false — and froth, having deferred any notion of a disabled surface, has a base
that is always true. A subset of a set that is always everything is a plain boolean.

**Three of froth's own recorded claims were recall dressed as citation, and all three were
wrong.** The `WM_GETDLGCODE` quotes on #1 describe the arrow walk, not the tab walk — the tab
walk queries only the outgoing control and filters on visible/enabled/`WS_TABSTOP`. "The prior
art is unanimous" on container focus memory is contradicted by two independent surveys: only
GTK4 and Flutter restore from it, and the ARIA practices name three re-entry conventions
rather than one. And Textual #877 is a one-day-old closed report about a CSS descendant
selector, not a focus-border reflow bug; it had reached ADR 0009, `chrome.md` and `HANDOFF.md`,
and was corrected in all three.

**The `WM_GETDLGCODE` shape died on evidence rather than on principle.** #47 asked for it to be
weighed rather than dismissed, since #9 had rejected the same idea and Win32's version survived
thirty years. It survived as keystroke routing: one documented focusability veto out of eleven
values, arrow walk only. ADR 0004 forbids the synchronous ask independently, so the two
arguments arrive at the same place from opposite directions.

**#28 was in scope and the answer is no.** #47 required the axes to answer it. Tested against
the composite-control case and they turn out to be near-inverses: a composite button wants the
mouse to treat it as one thing and has no focusable members, while every canonical #28 case —
sidebar, radio group, tab bar — needs clicks to reach the individual members. The distinguishing
question is whether the children are independently addressable. #28 is unblocked, since its
vocabulary dependency is now settled, and unanswered.

**The mouse "one thing" flag was designed, named, and then parked when the identifier check
found the name taken.** `CONTEXT.md` has defined Capture since M2 as a claim on all mouse input
regardless of what lies under the cursor — nearly the opposite. The session used the word for
an hour. The flag was parked in ADR 0019 for an unrelated and better reason: it is redundant
with subtree-scoped `MouseTransparent` set at the child's create site, and froth has no
composite control to justify it. Every `TabStop` in both examples is on a leaf.

**Flags moved from `CreateOption` to the type.** `TabStop()` and `TextSink()` as create options
put the declaration at every call site, where a caller who forgets produces a text field the
keyboard cannot reach and the component author cannot prevent it. They become fields of a
`Flags` value the surface's type declares through a context-free method froth calls during
`Create` and stores — Win32's window-class defaults, reached through the optional-interface
pattern `surface.go` already describes. Because it takes no context it can run inside `Create`,
which closes the window in which a child sat in the tree with zero-valued flags.

**The ownership rule was derived from a failure, not chosen.** Two authors plus whole-set
replacement silently clobber whichever order froth picks, and fixing the create-time order does
not help because the next `SetFlags` clobbers again. Removing the overlap dissolves the question:
`Flags` is the surface's, `FocusScope` and `MouseTransparent` are the creator's, and every flag
has exactly one initial author.

**Modal clicks needed nothing new.** `CONTEXT.md` already says a modal takes input for its
parent's whole subtree, and a click is input. Win32 solves the same problem by disabling the
owner subtree, verified against MSDN — which needs the disabled concept froth deferred, so froth
does not copy it.

**Two parks and two watches.** ADR 0019 parks the subtree hit-target flag and self-suppressed
targetability; ADR 0016 parks a sender on a delivered message, as the rejected half of a rule
`CLAUDE.md` already states; ADR 0002 parks an `IsMounted` accessor. ADR 0019 also carries an
explicit instruction to revisit `ParentFlags` during #53.

**Nothing was built.** The tickets that consume this are #48, #49, #51, #53 and #55, plus
a new one for the `CreateOption` migration, which touches 28 call sites across six example files
and five test files and is not in M4's original slicing.

### #56 — Flags move from `CreateOption` to the surface's type

Built by a subagent against ADR 0019, with three corrections after review.

**`Declarer` / `Declare() Flags`**, chosen against the call sites rather than in the design
session. Every existing optional interface is `<Verb>er` with the bare verb as its method, so
`Declares()` would have been the only third-person one in the set; the return type already says
what is declared. `ChildCtx.SetMouseTransparent(bool)` takes a bool rather than being a pair of
verbs, because a creator may put a child back into the hit walk.

**The type-level declaration kept leaves as leaves.** `dir-nav`'s `control`, `panel` and
`filter` gained `Declare` and no `Mount`, which was the shape the ticket was written to protect:
a leaf growing an `ApplyCtx` callback to carry one flag would have meant the mechanism was wrong.

**The root's declaration was missed, and the first fix for it was wrong.** `declareFlags` ran
only from `Create`, so a root's `Declare` was silently ignored — a component would be a text sink
everywhere except at the root. Reporting a diagnostic was tried first and reverted on evidence:
it broke four existing tests, because an ordinary component is used both as a child and as a
root, and an application cannot act on it without deleting a method the component needs
elsewhere. `New` now asks the root's type and lets a panic escape, since nothing is in progress
to unwind. ADR 0019 carries the amendment.

**`isTabStop`'s justification was stale in two successive versions.** It said the root is never a
stop because `New` takes no create options; the subagent updated it to mention the declaration,
and honouring the root's declaration then made the whole claim false. A root may now declare
`TabStop` and be a stop within its own scope.

**Every changed predicate was mutation-checked** — the remembered-link union, the redirect rule
and the root declaration each have a test that fails when the code is reverted to the old rule.
Two were checked independently at review rather than taken from the build report.

**`ClickFocus` and `MouseTransparent` are stored and read by nobody.** That is what lets the rest
of M4 slice independently, and it is why ADR 0019's request that the examples carry one surface
of each admission is deferred: a flag nothing reads is invisible in a running program.
