# Bubble Tea v2 integration contract (for froth)

Sources read locally (module cache):

- `charm.land/bubbletea/v2@v2.0.6` — `$BT`
- `charm.land/lipgloss/v2@v2.0.6` — `$LG`
- `github.com/charmbracelet/ultraviolet@v0.0.0-20260811164956-006e29f97886` — `$UV`

One exception to "local only": the `examples/clickable` directory is **not present in the module cache** (Go's module system does not download nested-module example trees on `go mod download`). Fetched via `gh api repos/charmbracelet/bubbletea/contents/examples/clickable/main.go` from the `charmbracelet/bubbletea` GitHub repo (default branch, current HEAD at fetch time) — see §9. Also ran `gh api repos/charmbracelet/bubbletea/compare/v2.0.6...v2.0.9` to diff against the newer release (§10).

---

## 1. Model contract

`$BT/tea.go:53-65`:

```go
// Msg contain data from the result of a IO operation.
type Msg = uv.Event

// Model contains the program's state as well as its core functions.
type Model interface {
	// Init is the first function that will be called. It returns an optional
	// initial command. To not perform an initial command return nil.
	Init() Cmd

	// Update is called when a message is received. Use it to inspect messages
	// and, in response, update the model and/or send a command.
	Update(Msg) (Model, Cmd)

	// View renders the program's UI, which can be a string or a [Layer]. The
	// view is rendered after every Update.
	View() View
}
```

- `Msg` is a **type alias** (`=`) for `uv.Event`, not a distinct interface — any `uv.Event` satisfies `tea.Msg` directly, and vice versa. This means froth's own message types just need to be any Go value; there's no `tea.Msg` marker method to implement.
- `Update` returns `(Model, Cmd)` — the model is still **value semantics by convention**: nothing in the interface enforces value receivers, but every example in the module (`cursed_renderer.go`, the fetched `clickable/main.go`) uses `func (m model) Update(...) (tea.Model, tea.Cmd)` on a value receiver and returns `m` (a copy) each time. The interface itself would equally accept a pointer receiver; Bubble Tea does not care, but the ecosystem convention is value receivers + immutable-style updates.
- `View()` returns a `View` **struct** (not a string, not an interface) — full field list in §1a below.
- The doc comment claims View "can be a string or a `Layer`" (`tea.go:62`) but **no `Layer` type or field exists anywhere in this package** — grepped `$BT/*.go` for `type Layer`, `Layer interface`, `func.*Layer`: **[NOT FOUND]**. `Layer` only exists in `lipgloss/v2` (`$LG/layer.go`) as an independent compositing helper that produces a plain string via `Compositor.Render()`, which is then assigned to `View.Content`. The doc comment is aspirational/stale relative to what's shipped in v2.0.6 — there is no first-class layer type in `tea.View` itself.

### 1a. `View` struct — every field (`$BT/tea.go:84-190`)

```go
type View struct {
	Content                   string
	OnMouse                   func(msg MouseMsg) Cmd
	Cursor                    *Cursor
	BackgroundColor           color.Color
	ForegroundColor           color.Color
	WindowTitle               string
	ProgressBar               *ProgressBar
	AltScreen                 bool
	ReportFocus               bool
	DisableBracketedPasteMode bool
	MouseMode                 MouseMode
	KeyboardEnhancements      KeyboardEnhancements
}
```

| Field | Type | Purpose |
|---|---|---|
| `Content` | `string` | Styled-string screen content (ANSI-encoded). The only way to hand content to the renderer — see §8. |
| `OnMouse` | `func(MouseMsg) Cmd` | Per-frame mouse interception hook — see §3/§9. |
| `Cursor` | `*Cursor` | Position/shape/blink/color of the real terminal cursor — see §2. `nil` = no cursor shown. |
| `BackgroundColor` / `ForegroundColor` | `color.Color` | Sets terminal default bg/fg (OSC 10/11). `nil` resets to terminal default. |
| `WindowTitle` | `string` | Terminal window title. |
| `ProgressBar` | `*ProgressBar` | OSC progress-bar (taskbar) indicator. |
| `AltScreen` | `bool` | Enter/exit alt-screen buffer (full-window mode). |
| `ReportFocus` | `bool` | Enables `FocusMsg`/`BlurMsg` reporting. |
| `DisableBracketedPasteMode` | `bool` | Opts out of bracketed paste. |
| `MouseMode` | `MouseMode` | `MouseModeNone` / `MouseModeCellMotion` / `MouseModeAllMotion` — see §3. |
| `KeyboardEnhancements` | `KeyboardEnhancements` | Requests Kitty keyboard protocol features — see §4. |

`NewView(s string) View` (`tea.go:76`) and `(*View).SetContent(s string)` (`tea.go:279`) are the two constructors; both just set `Content`.

**Every one of these fields is re-declared on every `View()` call** — froth, composing N nested surfaces, must merge all of these into a single top-level `View` per frame (one `Cursor`, one `MouseMode`, one `OnMouse`, one `AltScreen`, etc.), since only the root model's `View()` return value reaches the renderer (`tea.go:886-890`, see §8). There is no per-layer/per-surface `View` — a nested surface cannot independently set its own `AltScreen` or `MouseMode`; froth must reconcile these program-global settings itself.

---

## 2. Cursor

`Cursor` type — `$BT/tea.go:357-381`:

```go
// Position represents a position in the terminal.
type Position struct{ X, Y int }

// Cursor represents a cursor on the terminal screen.
type Cursor struct {
	Position         // embedded: X, Y
	Color color.Color
	Shape CursorShape
	Blink bool
}

func NewCursor(x, y int) *Cursor {
	return &Cursor{Position: Position{X: x, Y: y}, Color: nil, Shape: CursorBlock, Blink: true}
}
```

`CursorShape` (`$BT/cursor.go:11-19`): `CursorBlock`, `CursorUnderline`, `CursorBar` (iota-ordered, block=0).

There is exactly **one** `*Cursor` slot per frame, on `tea.View.Cursor` (§1a). It is a pointer — `nil` means "no cursor shown at all" (renderer hides it; `cursed_renderer.go:172` checks `lv.Cursor == nil`).

**What a nested component must propagate upward:** since only the root's `View()` is rendered, a focused text input nested N levels deep inside froth surfaces must have its local cursor position (relative to its own content) translated by each ancestor surface's offset (x/y position within its parent, scroll offset if any, border/padding insets) until it reaches absolute terminal coordinates for the single root `View.Cursor`. Bubble Tea provides **no mechanism** for this translation — no "focused widget" registry, no relative-cursor composition helper. Froth owns 100% of that responsibility: each surface's `View()`-equivalent must report (a) whether it has cursor focus, (b) its local cursor `Position`/`Shape`/`Blink`/`Color`, and froth's compositor must walk the surface tree, sum offsets, and set exactly one `tea.Cursor` on the merged root `View`. If two nested surfaces both think they own the cursor, froth (not Bubble Tea) must arbitrate — nothing here prevents a bug where two surfaces both set a cursor and the last-merged one silently wins.

`CursorPositionMsg` (`$BT/cursor.go:7-9`) and `RequestCursorPosition() Msg` (`cursor.go:23-27`) exist for querying the terminal's actual reported cursor position (a round-trip DSR query) — unrelated to setting the display cursor; this is about reading back where the terminal thinks the cursor is, useful mostly for terminal capability probing, not for widget focus.

---

## 3. Mouse

All types in `$BT/mouse.go`:

```go
type MouseButton = uv.MouseButton

const (
	MouseNone, MouseLeft, MouseMiddle, MouseRight,
	MouseWheelUp, MouseWheelDown, MouseWheelLeft, MouseWheelRight,
	MouseBackward, MouseForward, MouseButton10, MouseButton11 MouseButton = ...
)

type MouseMsg interface {
	fmt.Stringer
	Mouse() Mouse
}

type Mouse struct {
	X, Y   int
	Button MouseButton
	Mod    KeyMod
}

type MouseClickMsg   Mouse   // button press
type MouseReleaseMsg Mouse   // button release
type MouseWheelMsg   Mouse   // wheel scroll
type MouseMotionMsg  Mouse   // movement (with or without a button held)
```

Each concrete type is a **defined type over `Mouse`** (not a wrapper struct), and each implements `MouseMsg` via a `.Mouse()` method that just casts back to `Mouse`, plus `.String()`.

- **No separate `MouseDragMsg` type — [NOT FOUND].** Drag is inferred, not a distinct message: `MouseMotionMsg` doc (`mouse.go` doc on `MouseModeCellMotion`, `tea.go:290-297`) says cell-motion mode "captur[es] movement events... if a mouse button is pressed (i.e., drag events)". Concretely: a drag is a `MouseMotionMsg` whose `.Mouse().Button != MouseNone` (or, in practice, matches the button that was down at the preceding `MouseClickMsg`). The `clickable` example (§9) implements dragging exactly this way: it tracks `mouseDown`/`dragID`/`dragOffsetX/Y` fields on its own model and switches on `case tea.MouseMotionMsg` inside its custom `LayerHitMsg`, checking `if m.mouseDown && m.dragID != ""`. **Froth must implement drag-start/drag-continue/drag-end itself** from a raw stream of Click → Motion(with button) → Release; Bubble Tea gives no drag lifecycle events.
- Enabling mouse tracking is **not a `ProgramOption` in v2** (unlike v1's `tea.WithMouseCellMotion()`). Grepped `$BT/options.go` for every `func With*` (`options.go:22-163`): `WithContext, WithOutput, WithInput, WithEnvironment, WithoutSignalHandler, WithoutCatchPanics, WithoutSignals, WithoutRenderer, WithFilter, WithFPS, WithColorProfile, WithWindowSize`. **No mouse-related option exists.** Mouse tracking is instead a **per-frame `View` field**: `View.MouseMode` (`tea.go:177`), one of `MouseModeNone` (0), `MouseModeCellMotion`, `MouseModeAllMotion` (`tea.go:284-305`). The renderer diffs this field frame-over-frame and emits the corresponding SGR mouse-mode escape sequences on change (`cursed_renderer.go:350-368`).
- Mouse events reach `Update` as ordinary broadcast `Msg`s (see §6) but are **also** special-cased in the event loop: `tea.go:800-808` — if the incoming msg is one of `MouseClickMsg, MouseReleaseMsg, MouseWheelMsg, MouseMotionMsg`, the renderer's `onMouse` hook is invoked first (`p.renderer.onMouse(msg)`, which forwards to `View.OnMouse` — `cursed_renderer.go:765-771`), and if that returns a non-nil `Cmd`, it's run synchronously (`go p.Send(cmd())`) to synthesize an **additional** message alongside the raw mouse message. This is how `OnMouse` + `lipgloss.Compositor.Hit()` produces layer-aware hit messages (see §9) without Bubble Tea itself knowing about layers/z-order.

`KeyMod` — bitmask type shared with keyboard, described in §4.

---

## 4. Keyboard

### `KeyPressMsg` / `KeyReleaseMsg`

`$BT/key.go:190-255`:

```go
type KeyPressMsg   Key   // defined type over Key
type KeyReleaseMsg Key

type KeyMsg interface {
	fmt.Stringer
	Key() Key
}
```

Both have `.String()`, `.Keystroke()`, `.Key() Key` methods (methods just cast to `Key` and delegate).

### `Key` struct — `$BT/key.go:302-343`

```go
type Key struct {
	Text        string   // populated only for printable characters
	Mod         KeyMod   // modifier bitmask
	Code        rune     // e.g. KeyEnter, KeyTab, or 'a'
	ShiftedCode rune     // shifted variant (Kitty/Windows Console only)
	BaseCode    rune     // PC-101 layout base key (Kitty/Windows Console only)
	IsRepeat    bool     // key-repeat flag (Kitty/Windows Console only)
}
```

`(Key).String()` (`key.go:351-353`) delegates to `uv.Key(k).String()` — textual form ("?" not "shift+/" on US ANSI). `(Key).Keystroke()` (`key.go:369-371`) gives the modifier-ordered form (`ctrl+alt+shift+meta+hyper+super` order, per doc comment `key.go:359-367`) — e.g. always `"ctrl+shift+alt+a"`, never `"shift+ctrl+alt+a"`.

Key name constants (`KeyUp`, `KeyF1`...`KeyF63`, keypad keys, etc.) are all re-exported aliases of `uv.Key*` constants (`key.go:9-188`).

### Kitty keyboard protocol / enhanced keys

`KeyboardEnhancements` struct — `$BT/tea.go:241-266` (fields: `ReportEventTypes`, `ReportAlternateKeys`, `ReportAllKeysAsEscapeCodes`, `ReportAssociatedText`). Enabled by setting `View.KeyboardEnhancements` fields (e.g. `v.KeyboardEnhancements.ReportEventTypes = true`, doc example at `tea.go:237`) — **again a per-`View` field, not a `ProgramOption`**, consistent with the mouse-mode pattern.

If the terminal supports it, the program receives a `KeyboardEnhancementsMsg` (`$BT/keyboard.go:8-27`) with a `Flags int` bitmask and helper predicates `SupportsKeyDisambiguation()`, `SupportsEventTypes()`, `SupportsAlternateKeys()`, `SupportsAllKeysAsEscapeCodes()`, `SupportsAssociatedText()` (`keyboard.go:30-56`), each testing `Flags` against `ansi.KittyReportEventTypes` etc. Only with these enhancements enabled do you get: `KeyReleaseMsg` at all, `Key.IsRepeat`, `Key.ShiftedCode`, `Key.BaseCode` (each doc-commented "only available with the Kitty Keyboard Protocol or the Windows Console API").

Same per-`View` reconciliation problem as `MouseMode`/`AltScreen`: froth must merge every nested surface's desired `KeyboardEnhancements` into one program-wide request.

---

## 5. Focus/blur, resize, paste

### Focus (`$BT/focus.go`, entire file)

```go
type FocusMsg struct{}
type BlurMsg struct{}
```

Enabled via `View.ReportFocus = true` (`tea.go:163-170`); the renderer toggles `ansi.SetModeFocusEvent`/`ResetModeFocusEvent` on change (`cursed_renderer.go:341-347`). Doc caveat: not all terminals/multiplexers support it; tmux needs explicit configuration.

### Resize (`$BT/screen.go:5-12`)

```go
type WindowSizeMsg struct {
	Width  int
	Height int
}
```

"sent to Update once initially and then on every terminal resize." Delivered via `tty.go:126` (`p.Send(WindowSizeMsg{Width: w, Height: h})`) on SIGWINCH-driven resize checks, and once at startup (`tea.go:1054`: `resizeMsg := WindowSizeMsg{Width: p.width, Height: p.height}`). Doc note: Windows has no SIGWINCH, so resize isn't reported there. There is also a **distinct unexported `windowSizeMsg{}`** (`commands.go:166-175`, lowercase) used internally as the payload of `RequestWindowSize() Msg`, which the event loop turns into an async re-check (`tea.go:852-853`: `case windowSizeMsg: go p.checkResize()`) — don't confuse the public `WindowSizeMsg` (the actual size report) with the private `windowSizeMsg` (a "please recheck" trigger).

### Paste (`$BT/paste.go`, entire file)

```go
type PasteMsg struct{ Content string }     // bracketed-paste payload; .String() returns Content
type PasteStartMsg struct{}
type PasteEndMsg struct{}
```

Governed by `View.DisableBracketedPasteMode` (default enabled unless set `true`); renderer toggles `ansi.SetModeBracketedPaste`/`ResetModeBracketedPaste` on change (`cursed_renderer.go:332-338`).

---

## 6. Commands — routing is the central finding

### `Cmd` signature

`$BT/tea.go:390`:

```go
type Cmd func() Msg
```

Doc comment right above it (`tea.go:383-389`) is explicit and important: *"Note that there's almost never a reason to use a command to send a message to another part of your program. That can almost always be done in the update function."* — i.e. the maintainers' mental model has **no concept of "another part of your program"** as an addressable target at all.

### `Batch` vs `Sequence` — `$BT/commands.go:7-54`

```go
func Batch(cmds ...Cmd) Cmd      // -> BatchMsg   (unordered/concurrent)
func Sequence(cmds ...Cmd) Cmd   // -> sequenceMsg (ordered, one at a time)

type BatchMsg    []Cmd
type sequenceMsg []Cmd   // unexported

func compactCmds[T ~[]Cmd](cmds []Cmd) Cmd {
	// drops nils; 0 valid -> nil Cmd; 1 valid -> that Cmd directly (unwrapped!);
	// 2+ -> func() Msg { return T(validCmds) }
}
```

- **`Batch`**: "performs a bunch of commands concurrently with **no ordering guarantees** about the results" (doc, `commands.go:7-8`). Confirmed in the executor (`tea.go:918-955`, `execBatchMsg`): each cmd in the `BatchMsg` slice is run in its own goroutine under a `sync.WaitGroup`, and each result is sent independently via `p.Send(msg)` as soon as it's ready — no ordering, no barrier on completion (the `WaitGroup.Wait()` isn't even used to gate anything visible; it exists for panic/goroutine bookkeeping, not sequencing semantics).
- **`Sequence`**: "runs the given commands one at a time, **in order**" (doc, `commands.go:23-24`). Confirmed (`tea.go:892-916`, `execSequenceMsg`): a plain `for` loop, calling `cmd()` synchronously and sending its result before moving to the next — genuinely blocking/ordered within that sequence's own execution (but this whole loop runs in a spawned goroutine relative to the main event loop: `go p.execSequenceMsg(msg)` at `tea.go:846`, so it doesn't block `Update`/render itself).
- **Important asymmetry for froth**: `compactCmds` means `Batch(oneCmd)` or `Sequence(oneCmd)` do **not** wrap at all — they return `oneCmd` directly, bypassing `BatchMsg`/`sequenceMsg` entirely. Any wrapping logic froth writes around `Batch`/`Sequence` must not assume it will always see a `BatchMsg`/`sequenceMsg` — a single-command batch is indistinguishable from a bare `Cmd`.
- Nested `BatchMsg`/`sequenceMsg` inside a `BatchMsg`/`sequenceMsg` are recursively expanded by the executors themselves (`tea.go:907-911`, `946-950`) rather than being sent to `Update` as literal `BatchMsg` values — so `Update` never actually sees a `BatchMsg`/`sequenceMsg` as an incoming `Msg` in practice (the event loop does special-case them at `tea.go:841-847`, spawning the executor and `continue`-ing before ever reaching `model.Update`).

### Routing — **there is no addressing/targeting mechanism at all**

Traced the full path: `tea.go:743-883` (`eventLoop`) and `tea.go:698-739` (`handleCommands`).

1. `Program.Run` calls `model.Init()` once, gets an initial `Cmd`, feeds it into a single `cmds chan Cmd` (`tea.go:1140` area).
2. `handleCommands` (`tea.go:700-739`) reads that channel in a loop; for every non-nil `Cmd` it spawns a **new, unlabelled goroutine** that runs `msg := cmd()` and then `p.Send(msg)` — pushing the resulting `Msg` onto a single shared `p.msgs` channel. There is nothing on the `Msg` that says which `Cmd`, which subtree, or which "surface" produced it.
3. `eventLoop` (`tea.go:743-883`) reads `p.msgs` one at a time and, after some built-in special-casing (mouse/clipboard/window-size/etc.), calls `model, cmd = model.Update(msg)` — **the single root model's `Update`, once, for every message**, always. (`tea.go:871-872`.)
4. Whatever `Cmd` that `Update` call returns gets pushed back into the same `cmds` channel and the cycle repeats.

**There is exactly one `Update` entry point in the whole program: the root model's.** Bubble Tea has zero concept of message routing, addressing, subtree dispatch, or command provenance — it is a flat broadcast bus from Cmd-goroutines to one `Update` call. Any "this Cmd's result should come back to surface #7's Update" semantics **must be built entirely by froth**, because Bubble Tea provides nothing — not an ID field, not a context, not a typed wrapper, nothing. This directly and fully answers the brief's central concern.

---

## 7. Cmd wrapping — idiomatic tagging pattern

**[NOT FOUND]** as a built-in: no `tag(id, cmd)`-style helper anywhere in `$BT/*.go` (grepped `commands.go`, `tea.go` for `tag`, `wrap`, `Envelope`, `Addressed` — none). The doc comment at `tea.go:387-389` ("there's almost never a reason to use a command to send a message to another part of your program") signals the maintainers do not consider this a supported use case — it is explicitly *not* idiomatic Bubble Tea, but it is exactly what a component library like froth (or `bubbles/list`, `bubbles/table` in single-model programs, or Bubble Tea's own multi-model examples like `examples/split-editors` or `examples/pipe`, none of which are locally available to verify) has to do when composing multiple independently-updating sub-models under one root `Update`.

The general shape any such wrapper takes (froth will need to write this itself; not present in the library):

```go
func tag(id SurfaceID, cmd tea.Cmd) tea.Cmd {
	if cmd == nil {
		return nil // MUST short-circuit: don't manufacture an envelope for a no-op
	}
	return func() tea.Msg {
		msg := cmd()
		if msg == nil {
			return nil // a Cmd legitimately producing "nothing" must stay "nothing"
		}
		return envelopeMsg{ID: id, Inner: msg}
	}
}
```

Hazards to get right, all confirmed against the actual source:

- **`nil` Cmd.** `handleCommands` already skips nil Cmds before ever calling them (`tea.go:712-714`, `if cmd == nil { continue }`) — but if froth's `tag()` wraps a nil `Cmd` into a non-nil closure, it defeats that skip and turns a no-op into real goroutine + channel traffic. `tag()` must check `cmd == nil` and return `nil`, not a closure.
- **`nil` Msg result.** A `Cmd` returning `nil` is a valid, common pattern (`tea.go:759-761`, `if msg == nil { continue }` in the event loop — confirms nil messages are silently and safely dropped). If `tag()` blindly wraps every result in `envelopeMsg{...}`, a legitimate "no message" result becomes a non-nil envelope carrying a nil `Inner`, which will incorrectly reach `Update` and must be specially unwrapped-and-discarded there instead of being invisible by construction. Safer: check `msg == nil` inside the wrapper and propagate `nil` outward untouched.
- **`BatchMsg` / `sequenceMsg`.** These are special-cased by the event loop itself (`tea.go:841-847`) and drive their own recursive executors (`execBatchMsg`/`execSequenceMsg`) — they are never passed to `model.Update` as literal values. If froth's `tag()` wraps a `Cmd` that itself returns a `BatchMsg` (e.g. because the wrapped inner `Cmd` was actually `tea.Batch(a, b)` collapsed by `compactCmds` into a single closure — see §6), envelope-wrapping it would produce `envelopeMsg{Inner: BatchMsg{...}}`, and the event loop's `case BatchMsg:` type-switch (`tea.go:841`) will **never match** because the outer type is `envelopeMsg`, not `BatchMsg` — silently breaking Bubble Tea's batch/sequence execution machinery. **Any `tag()` wrapper must detect `BatchMsg`/`sequenceMsg` results and re-batch/re-sequence the *tagging* over the inner commands, not tag the aggregate result.** Concretely: if `cmd()` returns a `BatchMsg` (a slice of `Cmd`), the wrapper should return `BatchMsg` of `tag(id, innerCmd)` for each, not `envelopeMsg{Inner: thatBatchMsg}`.
- **`QuitMsg` / other internal control messages.** `QuitMsg` (`tea.go:559-561`, returned by `tea.Quit()`/`Quit() Msg` at `tea.go:555-557`) is checked by direct type-switch in the event loop (`tea.go:765-766`, `case QuitMsg: return model, nil`) to actually terminate `Run()`. If `tag()` wraps a `Cmd` that returns `tea.Quit()`'s `QuitMsg{}`, the result becomes `envelopeMsg{Inner: QuitMsg{}}` and the event loop's `case QuitMsg:` **will not fire** — the program will never quit through that path. Same class of hazard applies to `InterruptMsg`, `SuspendMsg`, `CapabilityMsg`, `ModeReportMsg`, `readClipboardMsg`/`setClipboardMsg` and siblings, `execMsg`, `requestCursorPosMsg`, `RawMsg`, `printLineMessage`, `clearScreenMsg`, `ColorProfileMsg`, `windowSizeMsg`/`WindowSizeMsg` — the entire list handled by the big type-switch at `tea.go:764-869`. **Any of these returned by a wrapped sub-surface Cmd must pass through un-enveloped**, or the corresponding built-in behavior silently stops working. Practically: `tag()` needs an allowlist/denylist of "pass through raw" message types (at minimum everything switched on in that block) versus "safe to envelope" (everything else, i.e. the sub-surface's own domain messages).

**Net: wrapping is necessary and entirely froth's responsibility, but naive wrapping breaks Batch/Sequence/Quit/mouse/clipboard/etc. the moment a wrapped Cmd happens to produce one of Bubble Tea's own internal message types.** This is the single sharpest trap in the whole contract.

---

## 8. Renderer — string in, cell buffer out; no pre-built-buffer hook

Traced `render`/`flush` path in `$BT/cursed_renderer.go`:

- `renderer` interface (`$BT/renderer.go:18-38`, unexported): `start()`, `close() error`, `render(View)`, `flush(closing bool) error`, `reset()`, `insertAbove(string) error`, `setSyncdUpdates(bool)`, plus more not shown. **`render` takes a `View` (whose only content is `Content string`)** — there is no overload or alternate entry point taking a pre-built cell buffer.
- `flush` (`$BT/cursed_renderer.go:257` onward) is where the string actually becomes cells:
  - `cursed_renderer.go:268`: `content := uv.NewStyledString(view.Content)` — parses the ANSI-encoded `Content` string into a `uv.StyledString`.
  - `cursed_renderer.go:276-279`: measures `content.Height()` against the current frame area to decide whether to resize the frame (non-altscreen "grow to fit content" behavior).
  - `cursed_renderer.go:287-306`: diffing gate — `if !s.starting && !closing && s.lastView != nil && viewEquals(s.lastView, &view) && frameArea == s.cellbuf.Bounds() { return nil }` — **this is the actual frame-to-frame diff**: if the new `View` is deep-equal (per `viewEquals`, `cursed_renderer.go:801` onward, comparing `Content`, `MouseMode`, and other fields) to the previous one and the frame area hasn't changed, **nothing is redrawn at all**. Bubble Tea's diffing operates at the whole-`View` level (identical `Content` string ⇒ skip), not at a cell/line level pre-render; once it decides a redraw is needed, the diffing of *which cells actually changed on screen* happens later, inside `s.cellbuf`/`s.scr` (ultraviolet's screen/cellbuf machinery), not shown in this excerpt but implied by the "touched lines" (`s.cellbuf.Touched`) bookkeeping at `cursed_renderer.go:299`.
  - `cursed_renderer.go:310-311`: `s.cellbuf.Clear(); content.Draw(s.cellbuf, s.cellbuf.Bounds())` — this is the **only place** the string becomes a cell buffer: `StyledString.Draw` parses the ANSI stream and writes into `s.cellbuf` (a `uv` cell buffer type), cell by cell.
- **No way to hand the renderer a pre-built buffer in v2.0.6 as shipped.** The public surface is entirely string-based (`View.Content string`, `SetContent(string)`, `NewView(string)`); `uv.StyledString`/`uv.NewStyledString` and the cell buffer types are reached only internally by the renderer, not exposed as a pluggable input on `Program`/`View`. Froth (or any layering library) must render to a string — via `lipgloss`'s `Layer`/`Compositor.Render() string` (§9) or equivalent — there is no lower-level "give me your `uv.Buffer` directly" path in this version.

---

## 9. `examples/clickable` — maintainer-sanctioned overlapping-layers + hit-testing pattern

(Fetched from GitHub since not present in the module cache — see header note. File: `charmbracelet/bubbletea` repo, `examples/clickable/main.go`.)

Confirmed the supporting `lipgloss.Layer`/`Compositor` API exists locally at `$LG/layer.go` (`NewLayer`, `.ID/.X/.Y/.Z/.AddLayers`, `NewCompositor`, `.Hit(x,y) LayerHit`, `.Render() string`).

Pseudocode of the pattern (~10 lines, faithful to the real code):

```
type LayerHitMsg struct { ID string; Mouse tea.MouseMsg }   // app-defined, NOT a tea type

func (m model) View() tea.View {
    root := lipgloss.NewLayer(background).ID("bg")
    for i, d := range m.dialogs {
        root.AddLayers(d.view().Z(i+1))          // each dialog is its own *Layer, with its own child button Layer
    }
    comp := lipgloss.NewCompositor(root)

    var v tea.View
    v.MouseMode = tea.MouseModeAllMotion          // opt into motion so hover/drag work
    v.OnMouse = func(msg tea.MouseMsg) tea.Cmd {  // runs BEFORE Update, per §3
        return func() tea.Msg {
            x, y := msg.Mouse().X, msg.Mouse().Y
            if id := comp.Hit(x, y).ID(); id != "" {
                return LayerHitMsg{ID: id, Mouse: msg}   // re-injected as an ordinary Msg
            }
            return nil
        }
    }
    v.SetContent(comp.Render())                  // Compositor -> flat string -> View.Content
    return v
}
```

`Update` then switches on `LayerHitMsg` and, inside it, on the concrete `msg.Mouse.(type)` (`MouseClickMsg`/`MouseMotionMsg`/`MouseReleaseMsg`) to implement press/drag/release/hover — all state (`mouseDown`, `pressID`, `dragID`, `dragOffsetX/Y`) is hand-rolled on the model, confirming §3's point that Bubble Tea itself has no drag concept.

---

## 10. v2.0.6 → v2.0.9 diff (upstream currently newer)

Via `gh api repos/charmbracelet/bubbletea/compare/v2.0.6...v2.0.9`. Relevant commits between the two tags:

- `fix: prevent data race with cursedRenderer.onMouse (#1691)` — **the exact `OnMouse` mechanism froth's hit-testing depends on (§3/§9) had a data race in v2.0.6**, fixed in v2.0.7. Since froth is being built against the locally-vendored v2.0.6, this is worth tracking upstream if froth leans hard on concurrent `OnMouse` calls.
- `fix: assign MouseButton11 = uv.MouseButton11 (#1754)` — v2.0.6 has `MouseButton11` declared without an explicit value in the `const` block (`mouse.go` — relies on iota-continuation from `MouseButton10`); confirmed by reading `mouse.go` locally that `MouseButton11` has no `= uv.MouseButton11` assignment, unlike its siblings. Fixed by explicit assignment upstream.
- `fix(key): map media record to ultraviolet code (#1757)` — a key-mapping bug fix, unlikely to affect froth's contract.
- `fix: pendingErase in cursedRenderer (#1755)` and `fix: restore kitty keyboard stack on exit (#1750)` — renderer/terminal-state correctness fixes, not contract changes.
- No commits between the two tags touch `Model`, `View` struct shape, `Cmd`/`Batch`/`Sequence` semantics, or add any routing/addressing mechanism — **the integration contract described above is unchanged between v2.0.6 and v2.0.9.**

---

## Traps for froth

1. **No Cmd-result routing exists, at all (§6).** Every `Cmd` result is broadcast to one root `Update`. Froth must invent and maintain its own addressing scheme (envelope + ID) for every Cmd issued by every nested surface — Bubble Tea offers nothing to build on here, not even a convention.
2. **Naive envelope-wrapping breaks Bubble Tea's own internals (§7).** `BatchMsg`/`sequenceMsg`/`QuitMsg`/`InterruptMsg`/clipboard & cursor-position request messages/etc. are all recognized by exact type in the event loop's type-switch (`tea.go:764-869`). Wrap one of these inside froth's envelope type and its built-in handling silently stops firing — including `Quit` no longer quitting.
3. **`compactCmds` unwraps single-element Batch/Sequence (§6).** `Batch(oneCmd)`/`Sequence(oneCmd)` return `oneCmd` directly — froth cannot assume a `Batch`/`Sequence` call always yields a `BatchMsg`/`sequenceMsg` wrapper to detect and re-tag.
4. **Cursor is a single global slot, and Bubble Tea does nothing to help composite it (§2).** Only one `*tea.Cursor` exists per frame, in absolute terminal coordinates. Every nested surface's local cursor state must be translated through every ancestor's offset by froth itself; Bubble Tea provides no relative-cursor/focus-chain concept.
5. **Per-View global settings can't be scoped to a subtree (§1a, §3, §4).** `MouseMode`, `AltScreen`, `ReportFocus`, `KeyboardEnhancements`, `DisableBracketedPasteMode` are single program-wide values set once per frame on the merged root `View`. A nested surface cannot locally opt into `MouseModeAllMotion` without it applying to the entire program; froth must reconcile (e.g. take the "loosest" mode requested by any surface) every frame.
6. **No drag message — must be inferred from raw Motion+Button state (§3, §9), and there's no hit-testing built into Bubble Tea itself** — layering/z-order/hit-testing is entirely a `lipgloss.Layer`/`Compositor` + `View.OnMouse` convention (maintainer-sanctioned via the `clickable` example) sitting *outside* Bubble Tea proper. Froth reimplements or adopts this same pattern; nothing here is enforced or provided by `tea` itself.
7. **Renderer diffs whole-`View` equality, not per-surface (§8).** A single unchanged nested surface's content still forces a full `viewEquals` comparison and, on any difference anywhere in the merged tree, a full `Content` string reparse (`uv.NewStyledString` + `Draw`) — there's no cell-level or subtree-level incremental update hook exposed publicly; froth's own compositor is the only place partial-redraw optimization could live.
8. **v2.0.6 has a known `OnMouse` data race (§10),** fixed in v2.0.7 (three patch versions ahead of what's vendored) — worth a version bump before froth leans hard on concurrent mouse hit-testing.
