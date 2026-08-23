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
