# vtui `FrameManager`: Z-ordered frame stack, one dispatcher, callback-based modality

## Bottom line

vtui's `FrameManager` is a real, load-bearing subsystem (~3,100 lines in
`framemanager.go` alone, in a ~67k-line library) and it does exactly what
froth's README claims: one `Frame` stack per workspace, one `dispatchEvent`
function, no nested pump anywhere in the codebase. Modality is enforced
inside the single dispatcher (keys go only to the top frame; mouse does a
Z-order hit-test walk that a modal frame terminates) — there is no
`RunModal`/blocking call. The single most useful idea to steal is the
**`OnResult func(int)` callback stored on the frame itself, invoked
synchronously from inside `SetExitCode` at the moment a control decides to
close the dialog** — this callback runs on the same goroutine, inside the
same call stack as the input dispatch that triggered it (a button's
`OnClick` calling `dlg.SetExitCode(code)`), so it is a genuine continuation
with zero scheduling machinery, and it composes with froth's rule of "push a
surface with a continuation; closing pops and delivers the result" almost
verbatim. The one place vtui's design leans on state Bubble Tea's model
discourages is that `Done`-marked frames are physically removed from the
stack in a separate sweep pass (`cleanupDoneFrames`) run *after* dispatch,
not by the dispatcher itself — froth will need an equivalent decoupling
point given Bubble Tea's `Update`/`View` split.

## Sources

- Repo: https://github.com/unxed/vtui
- Cloned commit: `6c903e7d72ca214c8027ea08575d0f93d87b73f7` (cloned 2026-08-22 into scratchpad)
- Files read in full or substantially: `framemanager.go`, `frame.go`,
  `baseframe.go`, `basewindow.go`, `events.go`, `desktop.go`,
  `common_dialogs.go`, `screenobject.go` (HitTest), `cmd/vtui-dialog/main.go`
  (call-site usage of `OnResult`).

---

## 1. Core types

### `Frame` interface — `framemanager.go:52-80`

```go
type Frame interface {
	ProcessKey(e *vtinput.InputEvent) bool
	ProcessMouse(e *vtinput.InputEvent) bool
	Show(scr *ScreenBuf)
	ResizeConsole(w, h int)
	GetType() FrameType
	SetExitCode(code int)
	IsDone() bool
	GetHelp() string
	IsBusy() bool // If true, FrameManager may skip the rendering phase
	HasShadow() bool
	GetKeyLabels() *KeySet
	HandleCommand(cmd int, args any) bool // Turbo Vision style command routing
	HandleBroadcast(cmd int, args any) bool
	Valid(cmd int) bool
	HitTest(x, y int) bool

	// MDI Methods
	GetMenuBar() *MenuBar
	SetPosition(x1, y1, x2, y2 int)
	GetPosition() (x1, y1, x2, y2 int)
	IsModal() bool
	GetWindowNumber() int
	SetWindowNumber(n int)
	RequestFocus() bool
	Close()
	GetTitle() string
	GetProgress() int // Returns 0-100, or -1 if no progress
}
```

There is no separate "window" type at the interface level — a "frame" is
anything satisfying `Frame`: `Desktop`, `Window` (dialogs/panels), `VMenu`
(dropdown menus are frames too, pushed onto the same stack), etc.

### `BaseFrame` — `baseframe.go:5-45`

Default embeddable implementation of the non-rendering parts of `Frame`:

```go
type BaseFrame struct {
	ScreenObject
	Done                bool
	ExitCode            int
	Modal               bool
	Number              int
	OnResult            func(int)
	Busy                bool
	AttentionSuppressed bool
}
```

`SetExitCode` (the only mutator for `Done`/`ExitCode`) is the modal-result
mechanism — see §4.

### `BorderedFrame` — `frame.go:7-15`

A presentation mixin (title, box-drawing, close button), embeds
`ScreenObject`. `Window` (in `window.go`, not fully quoted here) embeds both
`BaseFrame`-style state and `BorderedFrame` to become a concrete dialog/panel
type; the message-box/input-box helpers in `common_dialogs.go` build and
return `*Window`.

### `AppScreen` — `framemanager.go:90-95`

```go
type AppScreen struct {
	Number        int // Stable workspace number; never changes during its lifetime.
	Frames        []Frame
	CapturedFrame Frame
	Transparent   bool
}
```

This is the actual Z-order stack (`Frames []Frame`, index 0 = bottom,
last = top). vtui supports multiple `AppScreen`s ("workspaces", think
virtual desktops / tmux-like tabs) — `FrameManager` keeps a `[]*AppScreen`
and mirrors the active one into a flat `frames []Frame` field for fast
access during dispatch/render (kept in sync via `SyncCurrentScreen`,
`framemanager.go:472-477`).

### `frameManager` / `FrameManager` — `framemanager.go:212-283, 342`

```go
type frameManager struct {
	Screens           []*AppScreen
	ActiveIdx         int
	activationHistory []*AppScreen
	frames            []Frame // Points to the active screen's frame stack
	scr               *ScreenBuf
	...
	capturedFrame Frame // Points to the active screen's captured frame
	...
}
var FrameManager = &frameManager{}
type FrameManagerType = frameManager
```

`FrameManager` is a package-level singleton (there's also `NewFrameManager()`
for an independent instance, used by `vtui-host`). `capturedFrame` is the
frame that has an active mouse-button capture (drag in progress), separate
from Z-order/focus.

---

## 2. Frame stack representation and Z-order

- Z-order is literally slice order: `fm.frames` (mirrored per-`AppScreen` in
  `AppScreen.Frames`). Index `len(frames)-1` is always "top" / focused.
- **Push** (`framemanager.go:783-807`): assigns a window number 1-9 to
  non-modal frames (for Alt+N switching), sends the old top frame a
  `FocusEventType{SetFocus:false}`, appends the new frame, calls
  `SyncCurrentScreen()`, sends the new frame `FocusEventType{SetFocus:true}`.
  No return value — the frame is live in the stack synchronously after the
  call returns.
- **Pop** (`framemanager.go:914-925`): removes the last slice element,
  clears `capturedFrame` if it pointed at the popped frame, refocuses the
  new top.
- **RemoveFrame** (`framemanager.go:928-943`) and **PopFramesAbove**
  (`framemanager.go:947-958`) remove an arbitrary frame or truncate the
  stack above a given anchor (used to auto-close popups like combobox
  dropdowns when a drag starts on the window underneath).
- **RequestFocus** (`framemanager.go:876-911`, raise-to-top): walks the
  stack from the top down to the target frame; if it finds a modal frame
  *above* the target first, it refuses (`return false`) — this is one of
  the two places modality is actually enforced. Otherwise it splices the
  frame out and re-appends it at the end (top), sending focus-lost/-gained
  events to old/new top.
- **PushToFrameScreen** (`framemanager.go:810-834`) pushes onto whichever
  `AppScreen` owns a given anchor frame (not necessarily the active one),
  and auto-switches workspaces if the pushed frame is modal — so a modal
  raised on a background workspace still steals attention.
- Frames marked `Done` are **not** spliced out at click time; they are swept
  in a separate pass, `cleanupDoneFrames` (`framemanager.go:1768-1809+`),
  called at fixed points after dispatch (end of `dispatchEvent`, end of
  `Step`, after resize, etc. — 10 call sites). This avoids mutating the
  slice being iterated by the dispatcher mid-event.

---

## 3. Unified dispatcher: routing and modality enforcement

Single function, `dispatchEvent(ev *vtinput.InputEvent, is_injected bool)`,
`framemanager.go:2535-` (order below follows source order, elided sections
are input-method/translator/far2l housekeeping not relevant to modality):

- **Keys, paste, focus events** (`framemanager.go:2787-2790`): routed to the
  top frame *only*.
  ```go
  if ev.Type == vtinput.KeyEventType || ev.Type == vtinput.PasteEventType || ev.Type == vtinput.FocusEventType {
  	handled = topFrame.ProcessKey(ev)
  }
  ```
  There is no Z-order walk for keys — a background frame structurally
  cannot see a keystroke while anything is on top of it. That is the whole
  modality story for keyboard input: it falls out of "always address
  `frames[len-1]`", not from an explicit modal check.
- **Menus have priority over the top frame** (`framemanager.go:2738-2762`):
  if a `VMenu`/`MenuBar` is active, it gets first refusal on keys even over
  a modal top frame, provided the top frame *is* the menu or the menu's
  owner (`isMenuRelated`) — an explicit carve-out so an open dropdown from a
  modal dialog still works.
- **Mouse**: (`framemanager.go:2861-2925`) full Z-order hit-test walk,
  top-to-bottom:
  ```go
  for i := len(fm.frames) - 1; i >= 0; i-- {
  	f := fm.frames[i]
  	if f.GetType() == TypeDesktop { handled = f.ProcessMouse(ev); ...; break }
  	x1, y1, x2, y2 := f.GetPosition()
  	hitShadow := f.HasShadow() && ...
  	if f.HitTest(mx, my) || hitShadow {
  		if i != len(fm.frames)-1 {
  			if fm.RequestFocus(f) { handled = f.ProcessMouse(ev) }  // click-to-raise
  		} else {
  			handled = f.ProcessMouse(ev)
  		}
  		if handled && ev.ButtonState != 0 { fm.capturedFrame = f }
  		if f.IsModal() || handled { break }
  	}
  	if f.IsModal() {
  		// click landed OUTSIDE this modal frame's bounds: convert to
  		// synthetic Escape (LMB) or Enter/Escape (RMB), then break —
  		// nothing below this modal frame ever sees the raw click.
  		...
  		break
  	}
  }
  ```
  This is the second, more important place modality is enforced: the loop
  `break`s the instant it reaches a modal frame, whether or not that frame's
  bounds were hit. A background frame below a modal one can never receive a
  mouse event, full stop — regardless of whether the modal frame consumed
  it.
- `HitTest` itself (`screenobject.go:103`) is a plain bounding-box test:
  `x >= X1 && x <= X2 && y >= Y1 && y <= Y2`. No arbitrary-shape regions.
- `fm.capturedFrame` (mouse-button-down capture) bypasses the whole
  hit-test walk on subsequent mouse-move/-up events while a button is held
  (`framemanager.go:2814-2818`) — drag/resize doesn't re-hit-test every
  move.
- Global chrome (menu bar, key bar, workspace tab strip) is hit-tested
  *before* the frame walk and is allowed to intercept even over a modal
  frame in specific cases (`canActivateMenu`), which is a second explicit
  modality exception alongside the menu-priority one for keys.

---

## 4. How a modal returns a result without a nested loop — THE key mechanism

There is no blocking call anywhere. `ShowMessageEx`/`SelectDirDialog`/etc.
(`common_dialogs.go`) **push the dialog and return immediately**, handing
back the live `*Window`:

```go
// common_dialogs.go:175-177
func ShowMessageEx(title string, text string, buttons []string, kind MessageKind) *Window {
	dlg := createMessageDialog(title, text, buttons, kind)
	FrameManager.Push(dlg)
	return dlg
}
```

The caller attaches a **continuation closure** to the returned frame's
`OnResult func(int)` field (declared on `BaseFrame`, `baseframe.go:11`)
before or after `Push` — order doesn't matter since delivery only happens on
a later input-driven call:

```go
// cmd/vtui-dialog/main.go:218-226 (real call site)
dlg := vtui.ShowMessage(cfg.title, cfg.msgbox, buttons)
dlg.OnResult = func(code int) {
	if code == 0 {
		res.ExitCode = 0
	} else if code == 1 && cfg.extraLabel != "" {
		res.ExitCode = 2
	} else {
		res.ExitCode = 1
	}
}
```

The continuation fires when a button handler (itself invoked synchronously
from `dispatchEvent` → `topFrame.ProcessMouse`/`ProcessKey`) calls
`dlg.SetExitCode(code)`:

```go
// baseframe.go:16-29
func (bf *BaseFrame) SetExitCode(code int) {
	bf.Done = true
	bf.ExitCode = code
	if bf.OnResult != nil {
		bf.OnResult(code)
	}
	if FrameManager != nil {
		FrameManager.emitEventSink(UIEvent{Kind: "closed", SrcID: bf.ID(), Index: code})
	}
}
```

So the "no nested pump" story is: **the callback executes in the same
goroutine and the same call stack as the input event that caused the
close** — it is not deferred to "next tick", not delivered via a channel,
not polled. `SetExitCode` only *marks* `Done = true`; the frame is not
spliced out of the stack inside this call (that would mutate the slice
`dispatchEvent`'s mouse-walk is iterating). Removal happens afterward in
`cleanupDoneFrames`, called at the tail of `dispatchEvent`
(`framemanager.go:2695`, `3056`) and at several other fixed points
(`framemanager.go:1100,1110,1126,1136,1151,1164,1179,1249`). Net effect: the
opener's continuation runs to completion, mutating whatever state it
captured (a `res.ExitCode` field, an app-level struct, etc.), and only after
the whole dispatch returns does the stack forget the dialog ever existed.

Button wiring that triggers this, for context (`common_dialogs.go:97-98`):
```go
btnOk.OnClick = func() { dlg.SetExitCode(1) }
btnCancel.OnClick = func() { dlg.SetExitCode(-1) }
```

`IsDone()`/`ExitCode` are also directly readable (poll-style) if a caller
prefers not to use the callback — `BaseFrame.IsDone()` (`baseframe.go:31`)
— but every real call site found (`cmd/vtui-dialog/main.go`) uses `OnResult`,
not polling.

`Desktop` (`desktop.go:65-68`) implements the same `SetExitCode` contract
without embedding `BaseFrame` (it predates/bypasses the shared struct, has
no `OnResult`, since the desktop is never "opened by" anything).

---

## 5. Rendering / compositing

Single shared cell buffer (`*ScreenBuf`, `fm.scr`), one paint pass per
render, driven from `renderPhase()` (`framemanager.go:2335-2513`):

- Frames are painted **bottom-to-top**, screen-by-screen from the first
  non-transparent `AppScreen` up through the active one:
  ```go
  for sIdx := baseIdx; sIdx <= fm.ActiveIdx; sIdx++ {
  	frames := fm.GetActiveFrames(sIdx)
  	for _, frame := range frames {
  		if frame.HasShadow() { ... fm.scr.ApplyShadow(...) ... }
  		frame.Show(fm.scr)
  		if !isTopFrame(sIdx, frame) { fm.scr.SetCursorVisible(false) }
  	}
  }
  ```
- Occlusion is purely **paint-order overwrite** — there is no clip-region
  or dirty-rect exclusion of frames underneath. A frame later in the list
  paints its full rect over whatever a lower frame already wrote into
  `fm.scr`. `BorderedFrame.Show`/`DisplayObject` (`frame.go:68-91`) fills its
  background rect (`p.Fill(...)`) before drawing box/title — that fill is
  what erases whatever was under it.
  ```go
  func (f *BorderedFrame) Show(scr *ScreenBuf) {
  	...
  	f.ScreenObject.Show(scr)
  	f.DisplayObject(scr) // Fill() + DrawBox() + DrawTitle()
  }
  ```
- Shadows (`HasShadow()`) are drawn as a manual 2-cell/1-cell overlay
  offset from the frame's bounds, applied to the shared buffer before
  `frame.Show`, so a frame's shadow always lands on whatever is already
  painted from lower frames/screens.
- Cursor ownership: only the actual top frame of the whole composited
  stack keeps `SetCursorVisible(true)`; every lower frame's `Show()` call is
  followed by an explicit `SetCursorVisible(false)` — an interesting patch
  called out in a comment (`framemanager.go:2419-2431`, referencing f4 issue
  #518) for the case where the top frame's focused control has no caret of
  its own (a checkbox/button) and would otherwise leave the *previous*
  frame's cursor bleeding through.
- Rendering is gated by `fm.needsRender` and a buffered `RedrawChan` — any
  mutation calls `fm.Redraw()` (`framemanager.go:969-976`), which just sets a
  flag/pings a channel; the actual `renderPhase()` call happens once per
  `Step()` (`framemanager.go:1067-` area), not per mutation. So multiple
  state changes during one dispatch coalesce into a single repaint.

---

## What ports / what doesn't, against froth's no-reentrancy constraint

**Ports directly:**
- Flat `[]Frame` Z-order stack with push/pop/raise as the *only* structural
  operations — froth's Bubble Tea `Model` can hold the equivalent
  `[]Surface` and mirror `Push`/`Pop`/`RequestFocus` almost 1:1.
- Single dispatcher, keys-to-top-only + mouse Z-walk-with-modal-break, is a
  clean match for a single Bubble Tea `Update(msg tea.Msg)` — no nested
  `tea.Program` needed, which is exactly the constraint froth has committed
  to.
- **The `OnResult` continuation is the load-bearing idea to port.** It maps
  onto "push a surface with a continuation; closing pops and delivers the
  result" essentially verbatim: `Push(surface, func(result T) tea.Cmd)` or
  equivalent, invoked from inside whatever plays the role of
  `SetExitCode`, itself called from inside `Update` while handling the
  close-triggering message. No channel, no goroutine, no second event loop.
- Deferred removal (`cleanupDoneFrames` after dispatch, not during) is a
  pattern froth needs too, and translates well: Bubble Tea's `Update`
  already returns a new immutable-ish model each call, so "mark closed
  during handling, sweep at the top of the next `Update`/before `View`" is a
  natural fit — arguably *more* natural in Bubble Tea's re-render-every-tick
  model than in vtui's mutate-in-place one.
- Bottom-to-top paint-into-shared-buffer with plain overwrite-based
  occlusion maps directly onto composing `lipgloss`/manual cell buffers in
  a single `View()` call.

**Doesn't port cleanly / friction points:**
- vtui's `OnResult` and the whole `Frame` set are **mutable, pointer-shared,
  imperative objects** (`*Window`, `func(int)` closures mutating captured
  outer variables) — natural in Go-as-Turbo-Vision but foreign to Bubble
  Tea's message-passing, value-returning idiom. froth will need to translate
  "closure that mutates captured state" into "closure that returns a
  `tea.Msg`/`tea.Cmd` carrying the result", since Bubble Tea's `Update`
  can't safely reach into arbitrary outer-scope mutable state the way an
  imperative callback can (concurrency/re-render timing differs).
- vtui's exit code is a bare `int` — no generic result type. froth will
  presumably want the continuation typed on the specific modal's result
  (`func(T) tea.Cmd`), which vtui doesn't need because every dialog reduces
  to a button-index/exit-code convention.
- The `AppScreen`/multi-workspace layer (an entire second stack-of-stacks
  for virtual desktops, workspace tab bar, drag-to-reorder tabs, Alt+N
  switching) is orthogonal to froth's stated scope and adds ~1,500 lines of
  the 3,100 in `framemanager.go`. Not worth studying further for froth;
  it's a distraction from the core mechanism.
- Global chrome carve-outs (menu-active exception, key-bar/menu-bar
  hit-test-before-frames) show real interaction complexity once you have a
  persistent top-of-screen menu bar competing with modal enforcement — two
  separate special-cased "is this allowed to bypass the modal frame"
  checks (`isMenuRelated` for keys, `canActivateMenu` for mouse). If froth
  ever adds a persistent menu/status bar, expect the same kind of
  special-casing rather than a clean orthogonal layer.
- Mouse-capture (`capturedFrame`) is stored as global `FrameManager` state
  outside the stack, set/cleared based on `ev.ButtonState == 0`/`!= 0`
  transitions scattered through `dispatchEvent`. It works but is manual
  bookkeeping vtui didn't factor into a reusable primitive — froth should
  decide deliberately whether "captured input owner" is stack-adjacent
  state or something better modeled as an explicit mode.
- `Show(scr *ScreenBuf)` painting into one mutable shared buffer, with
  occlusion solely by paint order and manual shadow rectangles, is
  fundamentally an imperative-immediate-mode renderer. Bubble Tea/Lip Gloss
  favor composing strings/layouts declaratively per frame; froth will need
  to decide whether "paint bottom-up into a shared buffer" survives as an
  internal implementation detail behind a more declarative `View()` façade,
  or whether occlusion is instead solved by layout composition (each
  surface renders independently and results are stacked/cropped) — vtui
  doesn't need to answer this because it owns the whole terminal write
  path, whereas froth sits on top of Bubble Tea's own render cycle.
