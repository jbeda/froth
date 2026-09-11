# Bubble Tea v2: commands, the event loop, and Send

Primary source: `charm.land/bubbletea/v2@v2.0.9`, read from the module cache at
`/home/jbeda/src/go/pkg/mod/charm.land/bubbletea/v2@v2.0.9`, abbreviated `$BT` below.
That is the version pinned in froth's `go.mod`. Every claim below cites `file:line` in
that tree. A handful of behavioural claims were checked by running a probe test against
the real module rather than reading alone; those are marked **(probed)**.

This is a facts document. It contains no recommendation for what froth should do.

---

## 1. The event loop: what it inspects by concrete type

`eventLoop` is `$BT/tea.go:751-891`. Per iteration it selects on three channels
(`$BT/tea.go:753-760`):

- `<-p.ctx.Done()` → return, no error (`:754-755`)
- `<-p.errs` → return that error (`:757-758`)
- `msg := <-p.msgs` → the message path (`:760`)

On the message path, in order:

1. `msg = p.translateInputEvent(msg)` (`$BT/tea.go:761`, implementation
   `$BT/input.go:8-54`) — converts `ultraviolet` input events into `tea.*Msg` types.
   Anything it does not recognise is returned unchanged (`$BT/input.go:53`). A
   froth-defined message arriving via `Send` passes through untouched.
2. The program filter, if `WithFilter` was used (`$BT/tea.go:764-766`,
   `$BT/options.go:133-137`). A filter that returns `nil` drops the message entirely —
   `continue`, no `Update`, no render (`$BT/tea.go:767-769`).
3. The internal type switch, `$BT/tea.go:772-877`.
4. `model, cmd = model.Update(msg)` (`$BT/tea.go:880`).
5. The returned `cmd` is sent on the `cmds` channel, unless the context is done
   (`$BT/tea.go:882-886`).
6. `p.render(model)` → `p.renderer.render(model.View())` (`$BT/tea.go:888`, `:894-898`).

### 1.1 Every case in the switch

Three columns: the message type, what the loop does, and whether the message still
reaches `model.Update` afterwards.

| Case | Line | Exported? | Loop action | Reaches `Update`? |
|---|---|---|---|---|
| `QuitMsg` | `tea.go:773-774` | yes | `return model, nil` — the program exits | **No** |
| `InterruptMsg` | `tea.go:776-777` | yes | `return model, ErrInterrupted` | **No** |
| `SuspendMsg` | `tea.go:779-782` | yes | `p.suspend()` if `suspendSupported` | Yes |
| `CapabilityMsg` | `tea.go:784-792` | yes | on `"RGB"`/`"Tc"`, upgrades profile and `go p.Send(ColorProfileMsg{…})` | Yes |
| `ModeReportMsg` | `tea.go:794-806` | yes | configures renderer sync-updates / width method | Yes |
| `MouseMsg` (click/release/wheel/motion) | `tea.go:808-816` | yes (interface, `mouse.go:46`) | `p.renderer.onMouse(msg)`; if it returns a `Cmd`, runs it as `go p.Send(cmd())` | Yes |
| `readClipboardMsg` | `tea.go:818-819` | **no** | writes `ansi.RequestSystemClipboard` to output | Yes |
| `setClipboardMsg` | `tea.go:821-822` | **no** | writes OSC 52 set | Yes |
| `readPrimaryClipboardMsg` | `tea.go:824-825` | **no** | writes primary-clipboard request | Yes |
| `setPrimaryClipboardMsg` | `tea.go:827-828` | **no** | writes primary-clipboard set | Yes |
| `backgroundColorMsg` | `tea.go:830-831` | **no** | writes `ansi.RequestBackgroundColor` | Yes |
| `foregroundColorMsg` | `tea.go:833-834` | **no** | writes `ansi.RequestForegroundColor` | Yes |
| `cursorColorMsg` | `tea.go:836-837` | **no** | writes `ansi.RequestCursorColor` | Yes |
| `execMsg` | `tea.go:839-841` | **no** | `p.exec(msg.cmd, msg.fn)` — **blocks the event loop** (comment `// NB: this blocks.`) | Yes |
| `terminalVersion` | `tea.go:843-844` | **no** | writes `ansi.RequestNameVersion` | Yes |
| `requestCapabilityMsg` | `tea.go:846-847` | **no** | writes `ansi.RequestTermcap(…)` | Yes |
| `BatchMsg` | `tea.go:849-851` | yes (`commands.go:21`) | `go p.execBatchMsg(msg)` then **`continue`** | **No** |
| `sequenceMsg` | `tea.go:853-855` | **no** (`commands.go:30`) | `go p.execSequenceMsg(msg)` then **`continue`** | **No** |
| `WindowSizeMsg` | `tea.go:857-858` | yes | `p.renderer.resize(w, h)` | Yes |
| `windowSizeMsg` (the *request*) | `tea.go:860-861` | **no** (`commands.go:166`) | `go p.checkResize()` | Yes |
| `requestCursorPosMsg` | `tea.go:863-864` | **no** (`cursor.go:22`) | writes cursor-position request | Yes |
| `RawMsg` | `tea.go:866-867` | yes (`raw.go:5`) | `p.execute(fmt.Sprint(msg.Msg))` — raw write to output | Yes |
| `printLineMessage` | `tea.go:869-870` | **no** (`renderer.go:59`) | `p.renderer.insertAbove(msg.messageBody)` | Yes |
| `clearScreenMsg` | `tea.go:872-873` | **no** (`screen.go:24`) | `p.renderer.clearScreen()` | Yes |
| `ColorProfileMsg` | `tea.go:875-876` | yes (`profile.go:13`) | `p.renderer.setColorProfile(msg.Profile)` | Yes |

Only four cases do not reach `Update`: `QuitMsg`, `InterruptMsg` (both return from the
loop) and `BatchMsg`, `sequenceMsg` (both `continue`). Every other case is
*additive* — the loop acts and then still calls `Update` and re-renders.

**There is no window-title case.** In v2 the window title is a `View` field
(`View.WindowTitle`, `$BT/tea.go:141-144`), not a message; `tea.SetWindowTitle` and
`p.SetWindowTitle` were removed (`$BT/UPGRADE_GUIDE_V2.md:317`, `:333`). Likewise
altscreen, mouse mode, bracketed paste, focus reporting, and cursor show/hide are all
`View` fields in v2, not messages (`$BT/tea.go:146-190`, `$BT/UPGRADE_GUIDE_V2.md:299-316`).
So the "mode enable/disable" family named in the task does not exist as messages in this
version.

### 1.2 Unexported message types, and detecting them from outside the package

Unexported, and therefore unnameable in an allowlist written outside `package tea`:

`sequenceMsg` (`commands.go:30`), `windowSizeMsg` (`commands.go:166`),
`clearScreenMsg` (`screen.go:24`), `requestCursorPosMsg` (`cursor.go:22`),
`readClipboardMsg` / `setClipboardMsg` / `readPrimaryClipboardMsg` /
`setPrimaryClipboardMsg` (`clipboard.go:26,38,48,62`), `backgroundColorMsg` /
`foregroundColorMsg` / `cursorColorMsg` (`color.go:10,18,26`), `execMsg`
(`exec.go:10`), `terminalVersion` (`xterm.go:15`), `requestCapabilityMsg`
(`termcap.go:5`), `printLineMessage` (`renderer.go:59`).

Two detection routes exist and both were verified:

- **Obtain a specimen value and compare `reflect.Type`.** Every unexported request
  message has an exported constructor that returns it: `tea.Sequence(...)` returns a
  `Cmd` producing a `sequenceMsg` (`commands.go:25-27`), `tea.ClearScreen()` returns a
  `clearScreenMsg` directly (`screen.go:18-20`), `tea.RequestWindowSize()` returns a
  `windowSizeMsg` (`commands.go:173-175`), `tea.RequestCursorPosition()` returns a
  `requestCursorPosMsg` (`cursor.go:162-164`), `tea.ReadClipboard()` /
  `tea.ReadPrimaryClipboard()` return theirs (`clipboard.go:108-110`, `:134-136`),
  `tea.RequestBackgroundColor()` / `ForegroundColor` / `CursorColor`
  (`color.go:13-15`, `:21-23`, `:29-31`), `tea.RequestTerminalVersion()`
  (`xterm.go:20-22`), `tea.SetClipboard(s)`, `tea.SetPrimaryClipboard(s)`,
  `tea.RequestCapability(s)`, `tea.Exec(...)`, `tea.Println(...)`/`tea.Printf(...)` all
  return `Cmd`s that produce theirs. So a froth allowlist can be built as a set of
  `reflect.Type` keys without ever naming the types.
- **Type-name string.** **(probed)** `reflect.TypeOf(tea.Sequence(a,b)()).String()` is
  `"tea.sequenceMsg"`, `PkgPath()` is `"charm.land/bubbletea/v2"`, `Name()` is
  `"sequenceMsg"`. This is stringly-typed and unversioned, but it works.

`printLineMessage` has no distinguishing exported accessor for its body, but a specimen
is obtainable from `tea.Println("")` (`renderer.go:70-76`).

---

## 2. `BatchMsg` and `sequenceMsg` execution

### 2.1 Where the inner commands run

`execBatchMsg` (`$BT/tea.go:926-965`): one goroutine per non-nil inner cmd
(`:936-960`), `sync.WaitGroup` waited at `:964`. `execSequenceMsg`
(`$BT/tea.go:900-924`): a plain `for` loop, one cmd at a time, in order (`:912-923`).

Both are launched with `go` from the event loop (`$BT/tea.go:850`, `:854`), so neither
blocks the loop.

Both recurse on the inner result's concrete type:

```go
msg := cmd()
switch msg := msg.(type) {
case BatchMsg:
    p.execBatchMsg(msg)
case sequenceMsg:
    p.execSequenceMsg(msg)
default:
    p.Send(msg)
}
```

(`$BT/tea.go:914-922` for sequence, `$BT/tea.go:952-959` for batch.)

So inner results **do not re-enter the event loop as `BatchMsg`/`sequenceMsg`**; they are
unwrapped inside these helpers and only the leaf messages reach `p.Send` and hence the
loop. Nesting is arbitrarily deep and is flattened by recursion, not by the loop.

### 2.2 Consequence for a wrapper that wraps only the outer `Cmd`

The `BatchMsg`/`sequenceMsg` cases match by **concrete type** (`$BT/tea.go:849`, `:853`).
A wrapper that returns, say, `frothEnvelope{handle, BatchMsg{...}}` does not match either
case, so:

- `go p.execBatchMsg` / `go p.execSequenceMsg` never run — the inner commands are
  **never executed at all**.
- The envelope falls through to `model.Update` as an ordinary message
  (`$BT/tea.go:880`).

That is: wrapping the outer result does not "tag the batch and lose the inner tags" — it
**breaks the batch entirely**, unless froth itself recognises the envelope-wrapped
`BatchMsg`/`sequenceMsg` and re-drives the inner commands.

### 2.3 Rewriting the inner commands from outside the package

`BatchMsg` is `[]Cmd` and exported (`$BT/commands.go:21`), so its elements can be
replaced directly.

`sequenceMsg` is `[]Cmd` but unexported (`$BT/commands.go:30`). **(probed)** A
`sequenceMsg` value is `reflect`-convertible to `[]tea.Cmd` and back:

```
sequence convertible to []tea.Cmd? true len=2
converted inner cmd result: a
round-trip type: "tea.sequenceMsg"
```

So `reflect.ValueOf(msg).Convert(reflect.TypeOf([]tea.Cmd{}))` yields the slice, and
`reflect.ValueOf(newSlice).Convert(origType)` produces a fresh `sequenceMsg` that the
event loop's `case sequenceMsg:` will match. Inner-command rewriting from outside the
package is therefore possible for both, with reflection needed only for `Sequence`.

Note the aliasing hazard: converting a slice does not copy it, so mutating the converted
`[]tea.Cmd` in place mutates the original backing array.

### 2.4 `Batch`/`Sequence` with nil and single commands

Both go through `compactCmds[T]` (`$BT/commands.go:36-54`), which drops nil entries
(`:38-43`) and then:

- **zero** non-nil cmds → returns `nil` (`:45-46`)
- **exactly one** → returns *that command directly*, with no `BatchMsg`/`sequenceMsg`
  wrapper at all (`:47-48`)
- **two or more** → returns a `Cmd` producing `T(validCmds)` (`:49-52`)

**(probed)** confirms all four: `tea.Batch()` and `tea.Batch(nil, nil)` are `nil`;
`tea.Batch(a)`, `tea.Batch(nil, a)` and `tea.Sequence(a)` all return the identical
function pointer as `a`; `tea.Batch(a, a)()` is a `tea.BatchMsg` of length 2.

Separately, nil cmds are skipped again at execution time (`$BT/tea.go:912-914`,
`:934-936`) and by the command pump (`$BT/tea.go:721-723`).

---

## 3. Command execution

`handleCommands` (`$BT/tea.go:708-748`) is the pump. It runs in one goroutine
(`:712`), reads from the `cmds` channel (`:719`), skips nil cmds (`:721-723`), and
then **spawns a fresh goroutine per command** (`:729-746`):

```go
go func() {
    if !p.disableCatchPanics {
        defer func() {
            if r := recover(); r != nil { p.recoverFromPanic(r) }
        }()
    }
    msg := cmd() // this can be long.
    p.Send(msg)
}()
```

So: **one goroutine per command**, the command's return value goes straight to
`p.Send`, and a panicking command is recovered into `recoverFromPanic`
(`$BT/tea.go:1278-1301`), which pushes `ErrProgramPanic` onto `p.errs` and calls
`p.shutdown(true)`.

### 3.1 Cancellation

There is none, and the source says so explicitly (`$BT/tea.go:725-728`):

> Don't wait on these goroutines, otherwise the shutdown latency would get too large as
> a Cmd can run for some time (e.g. tick commands that sleep for half a second). It's not
> possible to cancel them so we'll have to leak the goroutine until Cmd returns.

`tea.Cmd` is `func() Msg` (`$BT/tea.go:398`) — **no context parameter, no cancellation
channel, nothing passed in at all.** A command in flight when the program quits keeps
running to completion; its `p.Send` then no-ops because `p.ctx` is done
(`$BT/tea.go:1192-1196`). The pump goroutine itself exits on `<-p.ctx.Done()`
(`$BT/tea.go:715-716`), which only stops *new* commands from being dispatched.

`p.ctx` is derived from `p.externalCtx` in `NewProgram` (`$BT/tea.go:614`), and
`WithContext` sets the external one (`$BT/options.go:22-26`). Nothing hands that
context to a command.

### 3.2 Nil returns

- A nil `Cmd` returned from `Update` is skipped by the pump (`$BT/tea.go:721-723`).
  It is still sent on the `cmds` channel first (`$BT/tea.go:885`).
- A non-nil `Cmd` returning a nil `Msg` is **not** skipped — `p.Send(nil)` is called
  (`$BT/tea.go:744`), the nil lands on `p.msgs`, `translateInputEvent` returns it
  unchanged (`$BT/input.go:53`), and — if no filter is set — it falls through the type
  switch and reaches `model.Update(nil)`. Only the filter path has an explicit nil
  check (`$BT/tea.go:767-769`), and that check fires on the filter's *output*.
  UNVERIFIED by execution; this is a reading of the control flow, not a probe.

---

## 4. `Program.Send`, `Quit`, `Kill`, `Wait`

`Send` is `$BT/tea.go:1191-1196`:

```go
func (p *Program) Send(msg Msg) {
	select {
	case <-p.ctx.Done():
	case p.msgs <- msg:
	}
}
```

`p.msgs` is created **unbuffered** in `NewProgram` (`$BT/tea.go:606`:
`msgs: make(chan Msg)`).

Facts that follow:

- **Blocking.** `Send` blocks until the event loop reads the message or the program's
  context is cancelled. There is no buffering and no timeout. The doc comment
  (`$BT/tea.go:1186-1190`) states: "If the program hasn't started yet this will be a
  blocking operation. If the program has already been terminated this will be a no-op."
- **Before `Run`.** `p.ctx` exists from `NewProgram` (`$BT/tea.go:614`) and is not
  cancelled, and nothing reads `p.msgs` until `eventLoop` starts (`$BT/tea.go:1153`).
  A `Send` in that window **blocks** — it does not buffer, and it does not drop. It
  unblocks when the loop starts reading (or never, if `Run` is never called).
- **After exit / kill.** `Run` defers `p.cancel()` (`$BT/tea.go:1013`) and
  `shutdown` calls `p.cancel()` (`$BT/tea.go:1252`), so `p.ctx.Done()` is closed and
  `Send` returns immediately without delivering. Safe, silent, lossy.
- **From inside `Update`.** The event loop calls `model.Update` synchronously
  (`$BT/tea.go:880`) and is the sole reader of `p.msgs`. A **synchronous** `p.Send`
  from inside `Update` therefore **deadlocks** until the program's context is cancelled
  — which, absent an external cancel, is never. Bubble Tea's own internal call sites
  consistently use `go p.Send(...)` for exactly this reason (`$BT/tea.go:789`, `:814`,
  `:1098`, `:1101`, `:1105`; `$BT/tty.go:21`, `:128`; `$BT/exec.go:110`, `:118`, `:123`).
  A `go p.Send(...)` from inside `Update` is fine and is the documented shape.
- **Channel closure.** `p.msgs` is **never closed** anywhere in the package (only
  `p.finished`, `$BT/tea.go:1010`, and the various handler `ch`s are). A late `Send`
  therefore cannot panic on "send on closed channel"; it takes the `ctx.Done()` branch.

Related methods:

- `Quit()` (`$BT/tea.go:1206-1208`) is exactly `p.Send(Quit())`, i.e. `p.Send(QuitMsg{})`
  (`Quit()` the command function is `$BT/tea.go:564-566`). It inherits every property
  above, including "blocks if the program hasn't started" and "no-op after exit".
- `Kill()` (`$BT/tea.go:1213-1215`) is `p.shutdown(true)` — immediate, skips the final
  render, guarded by `shutdownOnce` (`$BT/tea.go:1251`). `Run` then reports
  `ErrProgramKilled` (`$BT/tea.go:1160-1173`).
- `Wait()` (`$BT/tea.go:1218-1220`) is `<-p.finished`. `p.finished` is allocated **in
  `Run`** (`$BT/tea.go:1008`), not in `NewProgram`. Calling `Wait()` before `Run` reads
  from a nil channel and blocks forever. UNVERIFIED by execution; read from the two
  cited lines.
- **`Program.Println` / `Program.Printf` are not `Send`.** They write directly to
  `p.msgs` with no `ctx` guard (`$BT/tea.go:1382-1384`, `:1396-1398`). After the program
  exits those calls block forever.

There is no `Program.Context()` method; `grep "func (p \*Program)"` over the package
lists every method and none exposes the context (`$BT/tea.go`, `$BT/tty.go`,
`$BT/exec.go`, `$BT/input.go`, the `signals_*`/`termios_*`/`tty_*` files).

---

## 5. Can a model obtain the `*Program` or a send function from inside the framework?

**No.** Checked exhaustively for v2.0.9:

- `Model` is `Init() Cmd`, `Update(Msg) (Model, Cmd)`, `View() View`
  (`$BT/tea.go:52-66`). No argument carries a program, a context, or a send function.
- `Cmd` is `func() Msg` (`$BT/tea.go:398`) — no parameters.
- **There is no `tea.Context` type** in the package. The only context usage is
  `Program.ctx`/`externalCtx` (`$BT/tea.go:498-505`) and `WithContext`
  (`$BT/options.go:22-26`); neither is reachable from a model.
- `tea.View` (`$BT/tea.go:82-190`) is a plain struct of render settings plus an
  `OnMouse func(MouseMsg) Cmd` hook. It carries no program reference. `OnMouse` is
  invoked by the renderer (`$BT/cursed_renderer.go:816-827`) and its `Cmd` is run by the
  event loop as `go p.Send(cmd())` (`$BT/tea.go:813-815`) — note this bypasses the
  `cmds` channel and the `handleCommands` pump entirely.
- No `ProgramOption` injects anything into the model. The full list is `WithContext`,
  `WithOutput`, `WithInput`, `WithEnvironment`, `WithoutSignalHandler`,
  `WithoutCatchPanics`, `WithoutSignals`, `WithoutRenderer`, `WithFilter`, `WithFPS`,
  `WithColorProfile`, `WithWindowSize` (`$BT/options.go:22-168`).
- The messages the program sends at startup carry data, not capability:
  `ColorProfileMsg` (`$BT/tea.go:1097`), `WindowSizeMsg` (`$BT/tea.go:1101`),
  `EnvMsg` (`$BT/tea.go:1105`). None holds a program or a func.
- `WithFilter`'s callback does receive the `Model` and the `Msg`
  (`$BT/tea.go:765`, `$BT/options.go:133`) — that is a hook the *program owner* installs,
  not something the model can reach out through, and it does not supply a send function
  either.

So the send function must be handed to the model from outside. Ordering facts:

- `NewProgram(model, opts...)` (`$BT/tea.go:596-648`) stores the model at `:604` and
  fully constructs `p.ctx`/`p.msgs` before returning (`:606`, `:614`). It does not
  run anything.
- `Run()` (`$BT/tea.go:999`) is a separate, blocking call. `model.Init()` is invoked at
  `$BT/tea.go:1126`, well inside `Run`.
- So `p := tea.NewProgram(m)` → hand `p.Send` to `m` (or to something `m` reaches) →
  `p.Run()` is a valid ordering, and `Init` will not have run yet when `p.Send` is
  handed over.
- A `Send` actually *called* between `NewProgram` and `Run` **blocks** (unbuffered
  channel, no reader yet — §4). It does not buffer and it is not dropped. Handing over
  the function value is free; invoking it early is not.

---

## 6. Message mapping and request/response identity

**There is no `Cmd.map` analogue.** No `Map`, no `Tag`, no address, no routing helper
exists in the package. The complete exported command surface is `Batch`, `Sequence`,
`Every`, `Tick`, `RequestWindowSize` (`$BT/commands.go`), `Quit`, `Suspend`, `Interrupt`
(`$BT/tea.go:564-600`), `ClearScreen` (`$BT/screen.go:18`), the clipboard four
(`$BT/clipboard.go`), the colour three (`$BT/color.go`), `RequestCursorPosition`
(`$BT/cursor.go:162`), `Exec`/`ExecProcess` (`$BT/exec.go:26,54`), `Raw`
(`$BT/raw.go:33`), `RequestCapability` (`$BT/termcap.go:30`), `RequestTerminalVersion`
(`$BT/xterm.go:20`), `Println`/`Printf` (`$BT/renderer.go:70,86`).

`WithFilter` (`$BT/options.go:133`) is the only interception point, and it sits on the
*inbound* path in the loop (`$BT/tea.go:764-766`), not on commands.

### 6.1 Request → response pairs

Every one of these responses arrives at `Update` as a plain message carrying **no
correlation identity** — no request id, no sender, nothing to distinguish two concurrent
requesters.

| Request (a `Cmd` result or `View` field) | Loop action | Response at `Update` |
|---|---|---|
| `RequestWindowSize()` → `windowSizeMsg` (`commands.go:173`) | `go p.checkResize()` (`tea.go:861`) | `WindowSizeMsg` via `p.Send` (`tty.go:127`) |
| `RequestCursorPosition()` → `requestCursorPosMsg` (`cursor.go:162`) | writes CPR request (`tea.go:864`) | `CursorPositionMsg` (`cursor.go:7`) from input decode (`input.go:18-19`) |
| `RequestBackgroundColor()` (`color.go:13`) | `tea.go:831` | `BackgroundColorMsg` (`color.go:67`, `input.go:14-15`) |
| `RequestForegroundColor()` (`color.go:21`) | `tea.go:834` | `ForegroundColorMsg` (`color.go:36`, `input.go:12-13`) |
| `RequestCursorColor()` (`color.go:29`) | `tea.go:837` | `CursorColorMsg` (`color.go:81`, `input.go:16-17`) |
| `ReadClipboard()` (`clipboard.go:108`) | `tea.go:819` | `ClipboardMsg` (`clipboard.go:5`, `input.go:10-11`) |
| `ReadPrimaryClipboard()` (`clipboard.go:134`) | `tea.go:825` | `ClipboardMsg` — same type, `Selection` byte distinguishes (`clipboard.go:5-9`) |
| `RequestTerminalVersion()` (`xterm.go:20`) | `tea.go:844` | `TerminalVersionMsg` (`xterm.go:4`, `input.go:46-47`) |
| `RequestCapability(s)` (`termcap.go:30`) | `tea.go:847` | `CapabilityMsg` (`termcap.go:41`, `input.go:44-45`); also intercepted at `tea.go:784-792` |
| `Raw(seq)` (`raw.go:33`) | `tea.go:867` | whatever the terminal replies with, e.g. `ModeReportMsg` (`screen.go:60`, `input.go:50-51`) |
| `View.KeyboardEnhancements` (`tea.go:186`) | renderer emits the request | `KeyboardEnhancementsMsg` (`keyboard.go:9`, `input.go:48-49`) |
| `SetClipboard(s)`/`SetPrimaryClipboard(s)` (`clipboard.go:96,120`) | `tea.go:822`, `:828` | none — write-only |
| `Exec`/`ExecProcess` (`exec.go:26,54`) | `p.exec` blocks the loop (`tea.go:841`) | the `ExecCallback`'s message via `go p.Send(fn(err))` (`exec.go:110,118,123`) |
| `SuspendMsg` (`tea.go:582`) | `p.suspend()` (`tea.go:781`) | `ResumeMsg` via `go p.Send(ResumeMsg{})` (`tty.go:21`) |

Structural fact worth separating out: with one exception, **the request message is a
`Cmd` result** (so it would pass through any froth command wrapper) while **the response
is not** — it comes from the input reader goroutine, which streams decoded terminal
events straight into `p.msgs` (`$BT/tty.go:87`: `p.inputScanner.StreamEvents(p.ctx, p.msgs)`)
and through `translateInputEvent` (`$BT/tea.go:761`). Responses therefore arrive
untagged no matter what the wrapper does. The exception is `RequestWindowSize`, whose
response is produced by `p.checkResize()` calling `p.Send` (`$BT/tty.go:127`) — also
outside any wrapper.

Unprompted messages that arrive the same way (input reader or program internals) and
are likewise never command results: all key messages (`KeyPressMsg`, `KeyReleaseMsg`,
`key.go:191,224`), all mouse messages (`mouse.go:83-128`), `FocusMsg`/`BlurMsg`
(`focus.go:5,9`), `PasteMsg`/`PasteStartMsg`/`PasteEndMsg` (`paste.go:5,16,20`),
`WindowSizeMsg` on resize (`signals_unix.go:15`), and the three startup messages
`ColorProfileMsg`/`WindowSizeMsg`/`EnvMsg` (`$BT/tea.go:1097,1101,1105`).

---

## 7. Rendering cadence

**One message is one `Update` call, with four exceptions.** The loop calls
`model.Update(msg)` exactly once per message it dequeues (`$BT/tea.go:880`). It does
*not* for: a message the filter nils out (`:767-769`), `QuitMsg` (`:773`),
`InterruptMsg` (`:776`), `BatchMsg` (`:851`), `sequenceMsg` (`:855`). There is no
message coalescing, no de-duplication, and no batching of `p.msgs` reads anywhere.

**`View()` is also called once per `Update`** — `p.render(model)` runs unconditionally
after every `Update` (`$BT/tea.go:888`) and calls `model.View()`
(`$BT/tea.go:894-898`).

**Frames are coalesced by a ticker, not by messages.** `render` on the real renderer
only stores the view under a mutex (`$BT/cursed_renderer.go:627-632`):

```go
func (s *cursedRenderer) render(v View) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.view = v
}
```

The actual terminal write happens on a separate goroutine driven by
`time.Ticker` at the program's fps (`$BT/tea.go:1403-1432`):

```go
framerate := time.Second / time.Duration(p.fps)
...
case <-p.ticker.C:
    _ = p.flush()
    _ = p.renderer.flush(false)
```

Defaults and clamps: `defaultFPS = 60`, `maxFPS = 120`
(`$BT/renderer.go:10-15`); `NewProgram` clamps `p.fps` into `[60 default, 120 max]`
(`$BT/tea.go:619-623`), and `WithFPS` sets it (`$BT/options.go:142-146`).

`cursedRenderer.flush` short-circuits when nothing changed
(`$BT/cursed_renderer.go:320-323`): if not starting, not closing, no pending erase, and
`viewEquals(s.lastView, &view)` with an unchanged frame area, it returns without writing.

Net: N producer messages cost N `Update` calls and N `View()` calls, but at most
`fps` terminal writes per second regardless of N, and fewer if the view is unchanged.

---

## 8. v1 → v2 differences relevant to the above

From `$BT/UPGRADE_GUIDE_V2.md` (shipped inside the module) plus the code:

- **`View() string` → `View() tea.View`** (`UPGRADE_GUIDE_V2.md:474`,
  `$BT/tea.go:65`). Terminal state that was previously driven by commands or program
  methods is now declarative `View` fields.
- **A whole family of commands was deleted**, each replaced by a `View` field:
  `EnterAltScreen`/`ExitAltScreen`, `EnableMouseCellMotion`/`EnableMouseAllMotion`/
  `DisableMouse`, `HideCursor`/`ShowCursor`, `EnableBracketedPaste`/
  `DisableBracketedPaste`, `EnableReportFocus`/`DisableReportFocus`, `SetWindowTitle`
  (`UPGRADE_GUIDE_V2.md:299-316`). This directly shrinks the set of internal messages
  the event loop must intercept — none of these exist as messages in v2.
- **Program methods deleted**: `Start`, `StartReturningModel`, `EnterAltScreen`,
  `ExitAltScreen`, the four mouse toggles, `SetWindowTitle`
  (`UPGRADE_GUIDE_V2.md:318-332`). What remains on `*Program` for outside-in control is
  `Run`, `Send`, `Quit`, `Kill`, `Wait`, `Println`, `Printf`, `ReleaseTerminal`,
  `RestoreTerminal` (`$BT/tea.go`, method list in §4).
- **`tea.Sequentially` → `tea.Sequence`**; **`tea.WindowSize()` → `tea.RequestWindowSize`,
  which now returns a `Msg` rather than a `Cmd`** (`UPGRADE_GUIDE_V2.md:339-340`,
  `$BT/commands.go:173`). Several other request helpers share that shape — `ClearScreen`,
  `ReadClipboard`, `RequestCursorPosition`, `RequestBackgroundColor` etc. are
  `func() Msg` (which *is* a `tea.Cmd`, so they may be passed as one, but calling them
  yields the message directly).
- **`Msg` is now an alias for `ultraviolet.Event`** (`$BT/tea.go:50`), and
  `uv.Event` is `interface{}`
  (`/home/jbeda/src/go/pkg/mod/github.com/charmbracelet/ultraviolet@v0.0.0-20260811164956-006e29f97886/event.go:15`).
  So `Msg` remains effectively `any` and any froth envelope type satisfies it.
- **Input decoding moved to ultraviolet** — `translateInputEvent` (`$BT/input.go`) is a
  v2 layer converting `uv.*Event` to `tea.*Msg`, and the read loop streams events
  directly into `p.msgs` (`$BT/tty.go:87`).
- **Key and mouse messages became interfaces / split types** — `tea.KeyMsg` and
  `tea.MouseMsg` are interfaces (`$BT/key.go:259`, `$BT/mouse.go:46`) with concrete
  `KeyPressMsg`/`KeyReleaseMsg` and `MouseClickMsg`/`MouseReleaseMsg`/`MouseWheelMsg`/
  `MouseMotionMsg` (`UPGRADE_GUIDE_V2.md:108-139`, `:231-254`). An allowlist keyed on
  concrete type must enumerate the concretes; one keyed on interface satisfaction can
  use the interfaces.
- **New options**: `WithColorProfile`, `WithWindowSize` (`UPGRADE_GUIDE_V2.md:341-348`,
  `$BT/options.go:153,163`).
- `Run()` in v2.0.9 takes **no arguments** (`$BT/tea.go:999`). Two doc comments in the
  source show `p.Run(context.Background())` (`$BT/tea.go:91`) — those comments are stale
  relative to the code.

I did not consult an online changelog; everything in this section comes from the
`UPGRADE_GUIDE_V2.md` shipped in the module and from the v2.0.9 source. I did not read
the v1 source, so "v1 did X" claims here are the upgrade guide's word, not mine.

---

## What is settled fact vs what I could not verify

**Settled — read directly from v2.0.9 source, cited above:**

- The complete event-loop switch, which four cases skip `Update` (`QuitMsg`,
  `InterruptMsg`, `BatchMsg`, `sequenceMsg`), and that every other case is additive.
- No window-title / altscreen / mouse-mode / cursor-visibility messages exist in v2;
  they are `View` fields.
- Which internal message types are unexported, and that each has an exported
  constructor yielding a specimen value.
- `execBatchMsg` / `execSequenceMsg` internals: goroutine-per-cmd vs sequential loop,
  recursion on nested `BatchMsg`/`sequenceMsg`, leaf results going to `p.Send`.
- That an envelope-wrapped `BatchMsg`/`sequenceMsg` fails the concrete-type match and so
  the inner commands are never run.
- `compactCmds` semantics for nil / single / multiple commands.
- One goroutine per command; no context, no cancellation; the source comment stating
  commands are deliberately leaked until they return.
- `p.msgs` is unbuffered and never closed; `Send`'s two-way select; `Quit` = `Send(QuitMsg{})`;
  `Kill` = `shutdown(true)`; `Wait` = `<-p.finished` with `p.finished` allocated in `Run`;
  `Println`/`Printf` bypass the `ctx` guard.
- No route from a model to the `*Program`, a send function, or a context. No `tea.Context`
  type. No `Program.Context` method. No option injects one.
- No `Cmd.map` analogue; the request/response table and the fact that responses carry no
  correlation identity and arrive outside the command path.
- Renderer cadence: `render` stores, a ticker at `fps` flushes, `defaultFPS = 60`,
  `maxFPS = 120`, unchanged-view short-circuit in `flush`.

**Verified by running a probe against the real module (probed):**

- `reflect.TypeOf(sequenceMsg).String() == "tea.sequenceMsg"`, `PkgPath ==
  "charm.land/bubbletea/v2"`.
- `sequenceMsg` is `reflect`-convertible to `[]tea.Cmd` and back, so inner commands of a
  `Sequence` can be rewritten from outside the package.
- `Batch()` / `Batch(nil, nil)` return nil; `Batch(a)`, `Batch(nil, a)`, `Sequence(a)`
  return the same function pointer as `a`; `Batch(a, a)()` is a length-2 `BatchMsg`.

**Not verified:**

- **`Update` receives a nil `Msg`** when a command returns nil. This is a reading of the
  control flow (`tea.go:744` → `:760` → `input.go:53` → `:880`), not an executed test.
- **`Wait()` before `Run()` blocks forever on a nil channel.** Read from
  `tea.go:1008` and `:1219`; not executed.
- **`Kill()` before `Run()`.** `shutdown` is `shutdownOnce`-guarded and cancels `p.ctx`,
  so a later `Run` would presumably return near-immediately with `ErrProgramKilled`, but
  I did not trace or test the interaction between `shutdownOnce` having already fired and
  `Run`'s own teardown. Treat as unknown.
- **Behaviour under `WithoutRenderer`** (`nilRenderer`, `$BT/nil_renderer.go`) for any
  of the renderer-touching cases — not read.
- **Whether `p.Send` called synchronously from `Update` can ever unblock** other than by
  context cancellation. The reasoning (sole reader is the loop, which is inside `Update`)
  is sound but I did not construct the deadlock to confirm it.
- **Any v1 claim.** I did not read the v1 source; §8's v1 side is the shipped upgrade
  guide's assertion.
- **Online release notes / changelog** for v2.0.x point releases — not consulted. The
  only v2 migration text I read is the `UPGRADE_GUIDE_V2.md` inside the module.
- **`sequenceMsg` conversion stability across versions.** The reflect conversion works
  today because the underlying type is `[]Cmd`; nothing in the package promises that.
