# Streams into a UI loop: delivery discipline and deterministic async

A prior-art survey along two axes:

- **Axis A** — how a UI framework gets a stream of events from a background source into its
  loop, and what it collapses on the way.
- **Axis B** — how a UI framework makes that async machinery deterministic in tests.

froth's current design is the reason for the survey: a producer is a step function
`func(ctx) (T, error)` that blocks until its next message, froth owns the goroutine and the
loop, and delivery **defaults to neither batching nor coalescing**, with `Batched[T]`,
`BatchedFunc(merge)` and `Coalesced[T]` available explicitly; the goroutine runner is
injectable so tests drive step functions in lockstep, with a small set of `testing/synctest`
tests against the real runner
([kitchen-sink sketch](../design/kitchen-sink-sketch.md), finding 3; issue #1, "Producers are
step functions"). Nothing below recommends a change. It reports what other systems do, and
the closing sections say plainly where the sweep supports froth's choices and where it does
not reach them.

Sources are the systems' own documentation and source. Line numbers were read at these
revisions; they drift.

| System | Read at |
|---|---|
| Elm | `elm/core` @ `65cea00`, `elm/browser` @ `1d28cd6`, `elm/time` @ `dc3b75b` (kernel JS) |
| Textual | `Textualize/textual` @ `06dbeef` (v8.2.8) |
| Bubble Tea | `charmbracelet/bubbletea` tag `v2.0.9` |
| tcell / tview | `gdamore/tcell` tag `v2.8.1`; `rivo/tview` @ `c15b79f` |
| gpui | `zed-industries/zed` @ `2518540`, `crates/gpui/src`, `crates/scheduler/src` |
| Qt | `qt/qtbase` branch `6.8` + doc.qt.io Qt 6 |
| Win32 | learn.microsoft.com, current |
| Flutter | `flutter/flutter` `master` (`packages/flutter/lib/src/...`) + api.flutter.dev |
| Android / Kotlin | developer.android.com, kotlinlang.org |
| Apple | developer.apple.com (some archived conceptual docs) |
| Rx | reactivex.io, RxJava 3 javadoc (mirror; javadoc.io returned 403) |
| Go | `testing/synctest` and `runtime/synctest.go` from the local Go 1.27.1 tree |

---

# AXIS A — what gets collapsed on the way in

## The shape every system shares

Before the per-system detail, the finding that eleven systems agree on, and which no single
system's documentation states: **every one of them splits the seam between a payload-free
"something changed" signal, which is collapsed aggressively, and a payload-carrying message,
which is delivered one-for-one.**

- gpui: `notify()` is deduped by entity id; `emit()` is not deduped at all.
- Qt: `UpdateRequest` / `LayoutRequest` / `Timer` are compressed in `postEvent`; the
  `QMetaCallEvent` behind a queued signal is not.
- Win32: `WM_PAINT` and `WM_TIMER` are generated on demand rather than queued N times;
  everything else posted is FIFO.
- Flutter: `markNeedsBuild` is idempotent per element and `scheduleFrame` is one global flag;
  the Dart stream underneath delivers every event.
- Apple: `setNeedsDisplay` is a dirty flag; run-loop input sources are lossless.
- Textual: `Widget.refresh` sets a flag checked at idle; key and mouse events are not
  collapsible.
- Bubble Tea and Elm: the renderer coalesces frames; the message path never merges.

Where a *payload* is collapsed, it is collapsed in one of three ways, and the choice tracks
what the payload is:

- **idempotent drop** when the message carries nothing (Qt `UpdateRequest`, gpui
  `Effect::Notify`),
- **latest wins** when the payload is a current-value snapshot (Qt `Resize`/`Move`, Win32
  mouse messages, `LiveData`, `StateFlow`, Rx `sample`),
- **union / accumulate** when the payload is a set that merges (Win32 `WM_PAINT` update
  region, Qt `UpdateLater` region `+=`, gpui `dirty_views`, `NSView` invalid rect).

**No surveyed system collapses a message whose payload is a delta.** Those stay lossless
everywhere. That is the closest thing the sweep produces to an independent statement of
froth's finding 3.

## Elm

**No app-level message queue exists.** `sendToApp` calls `update` synchronously on the
caller's stack and re-evaluates `subscriptions(model)` after every message
([Platform.js#L45-L50](https://github.com/elm/core/blob/65cea00/src/Elm/Kernel/Platform.js#L45-L50)).
Buffering is per effect-manager *process*, an unbounded JS array mailbox
([Scheduler.js#L73](https://github.com/elm/core/blob/65cea00/src/Elm/Kernel/Scheduler.js#L73),
drained at
[#L131-L147](https://github.com/elm/core/blob/65cea00/src/Elm/Kernel/Scheduler.js#L131-L147)).

`Time.every` merges **subscriptions**, not messages: all subs fold into
`Dict interval (List tagger)` and one `setInterval` is spawned per distinct interval
([Time.elm#L486-L504](https://github.com/elm/time/blob/dc3b75b/src/Time.elm#L486-L493)); each
tick then does
`Task.sequence (List.map (\tagger -> Platform.sendToApp router (tagger time)) taggers)` — N
subscribers, N separate `update` calls
([#L514-L527](https://github.com/elm/time/blob/dc3b75b/src/Time.elm#L514-L527)). Ticks cannot
pile up only because the HTML spec re-arms a repeating timer after its callback task runs
([timers](https://html.spec.whatwg.org/multipage/timers-and-user-prompts.html)); Elm itself
skips nothing.

`onAnimationFrame` keeps exactly one rAF outstanding, requesting the next one from inside the
handler
([AnimationManager.elm#L88-L104](https://github.com/elm/browser/blob/1d28cd6/src/Browser/AnimationManager.elm#L88-L104)).
Rendering is the only coalescing: the animator marks the model dirty and draws at most once
per frame however many `update`s ran
([Browser.js#L110-L136](https://github.com/elm/browser/blob/1d28cd6/src/Elm/Kernel/Browser.js#L110-L136)).

The one loss: a value arriving on an incoming port with no current subscriber is dropped —
the `subs` list is empty and nothing else buffers it
([Platform.js#L464-L466](https://github.com/elm/core/blob/65cea00/src/Elm/Kernel/Platform.js#L464-L466)).

**Lossless, no back-pressure, collapse at the view layer only.**

## Textual

The closest-domain system, and the only one in the sweep with an explicit opt-in
latest-wins collapse on the ordinary message path.

Each `MessagePump` (App, Screen, every Widget) owns an unbounded, non-blocking queue
([message_pump.py#L156-L157](https://github.com/Textualize/textual/blob/06dbeef/src/textual/message_pump.py#L156-L157),
[_queue.py#L11-L48](https://github.com/Textualize/textual/blob/06dbeef/src/textual/_queue.py#L11-L48));
`post_message` is `put_nowait`, thread-safe off-loop via `loop.call_soon_threadsafe`
([#L860-L888](https://github.com/Textualize/textual/blob/06dbeef/src/textual/message_pump.py#L860-L888)).
Drops are explicit and narrow: the pump is closing, or `check_message_enabled` says no
(`disable_messages`, and `prevent()` for widgets,
[widget.py#L4663](https://github.com/Textualize/textual/blob/06dbeef/src/textual/widget.py#L4663)).

The collapse is a method on the message class, checked in the pump loop:

```python
# Combine any pending messages that may supersede this one
while not (self._closed or self._closing):
    pending = self._peek_message()
    if pending is None or not message.can_replace(pending):
        break
    message = await self._get_message()
```
([message_pump.py#L648-L660](https://github.com/Textualize/textual/blob/06dbeef/src/textual/message_pump.py#L648-L660))

`can_replace` defaults to `False`
([message.py#L120-L129](https://github.com/Textualize/textual/blob/06dbeef/src/textual/message.py#L120-L129))
and is overridden for exactly `Update` (same widget only), `Layout`, `UpdateScroll`, `Prompt`
([messages.py](https://github.com/Textualize/textual/blob/06dbeef/src/textual/messages.py#L46))
and `events.Resize`
([events.py#L146-L147](https://github.com/Textualize/textual/blob/06dbeef/src/textual/events.py#L146-L147)).
Key and mouse events are not in that list.

Repaint is a flag, not a message. `Widget.refresh`: *"This method sets an internal flag to
perform a refresh, which will be done on the next idle event. Only one refresh will be done
even if this method is called multiple times."*
([widget.py#L4324-L4375](https://github.com/Textualize/textual/blob/06dbeef/src/textual/widget.py#L4324-L4375)).
The Screen accumulates a `_dirty_widgets` **set** and repaints on a timer,
`UPDATE_PERIOD = 1 / constants.MAX_FPS` with `MAX_FPS` defaulting to 60
([screen.py#L78](https://github.com/Textualize/textual/blob/06dbeef/src/textual/screen.py#L78),
[constants.py#L146](https://github.com/Textualize/textual/blob/06dbeef/src/textual/constants.py#L146)).

Timers **skip** missed ticks by default (`skip: bool = True`,
[timer.py#L54](https://github.com/Textualize/textual/blob/06dbeef/src/textual/timer.py#L54);
the skip arithmetic at
[#L158-L164](https://github.com/Textualize/textual/blob/06dbeef/src/textual/timer.py#L158-L164)).

Workers hand results back as ordinary posted messages
([worker.py#L183](https://github.com/Textualize/textual/blob/06dbeef/src/textual/worker.py#L183));
`@work(thread=True)` is `loop.run_in_executor`
([worker.py#L284-L326](https://github.com/Textualize/textual/blob/06dbeef/src/textual/worker.py#L284-L326)).
`call_from_thread` is the one back-pressured path: `asyncio.run_coroutine_threadsafe(...)`
then `future.result()`, so the worker thread blocks until the callback has run on the UI loop
([app.py#L1788-L1838](https://github.com/Textualize/textual/blob/06dbeef/src/textual/app.py#L1788-L1838)).

Documented stance: *"Messages are picked off a queue and processed (cooked) by a handler
method. This guarantees messages and events are processed even if your code can not handle
them right away"* ([events guide](https://textual.textualize.io/guide/events/)) — a guarantee
the `can_replace` path qualifies.

**Lossless by default; opt-in latest-wins for five named message types; the knob lives on the
message class, not at the subscription site.**

## Bubble Tea, tcell, tview

froth's substrate, and the strictest system in the sweep.

**One unbuffered channel**: `msgs: make(chan Msg)`
([tea.go#L606](https://github.com/charmbracelet/bubbletea/blob/v2.0.9/tea.go#L606)), one
message per `Update` in the event loop
([tea.go#L750-L890](https://github.com/charmbracelet/bubbletea/blob/v2.0.9/tea.go#L750)).
Nothing is dropped or merged.

**Back-pressure is total and lands on the producer.** `Send` blocks until the loop takes the
message
([tea.go#L1192-L1197](https://github.com/charmbracelet/bubbletea/blob/v2.0.9/tea.go#L1192-L1197));
each `Cmd` runs in its own goroutine ending in `p.Send(msg)`
([tea.go#L729-L741](https://github.com/charmbracelet/bubbletea/blob/v2.0.9/tea.go#L729-L741)),
so a fast producer accumulates *blocked goroutines* rather than a growing queue. Terminal
input goes into the same channel
([tty.go#L84-L88](https://github.com/charmbracelet/bubbletea/blob/v2.0.9/tty.go#L84-L88)) — a
slow `Update` back-pressures onto the tty read.

**`Batch` merges commands, never messages.** It yields a `BatchMsg []Cmd`
([commands.go#L15-L52](https://github.com/charmbracelet/bubbletea/blob/v2.0.9/commands.go#L15));
the loop runs them concurrently and `Send`s each result separately
([tea.go#L926-L964](https://github.com/charmbracelet/bubbletea/blob/v2.0.9/tea.go#L926)).
N commands, N `Update` calls.

**Frames are coalesced and messages are not**: `defaultFPS = 60`, *"defaultFramerate specifies
the maximum interval at which we should update the view"*
([renderer.go#L10-L15](https://github.com/charmbracelet/bubbletea/blob/v2.0.9/renderer.go#L10-L15));
`render` stores only the latest view under a mutex
([cursed_renderer.go#L627-L633](https://github.com/charmbracelet/bubbletea/blob/v2.0.9/cursed_renderer.go#L627-L633))
and a ticker flushes
([tea.go#L1402-L1430](https://github.com/charmbracelet/bubbletea/blob/v2.0.9/tea.go#L1402)).
`tea.Every` drains any backlog in its timer channel before returning
([commands.go#L100-L110](https://github.com/charmbracelet/bubbletea/blob/v2.0.9/commands.go#L100)).

**tcell is the one system in the sweep that silently drops on the public producer API.** The
queue is `make(chan Event, 10)`
([tscreen.go#L232](https://github.com/gdamore/tcell/blob/v2.8.1/tscreen.go#L232)) and
`PostEvent` is documented as *"This can fail if the event queue is full. In that case, the
event is dropped, and ErrEventQFull is returned"*
([screen.go#L113-L116](https://github.com/gdamore/tcell/blob/v2.8.1/screen.go#L113-L116)).
Internally, input events block but resize events are dropped when full
([tscreen.go#L1209-L1213](https://github.com/gdamore/tcell/blob/v2.8.1/tscreen.go#L1209-L1213)).
tcell offers a hook for app-level coalescing and does none itself — `HasPendingEvent`: *"The
purpose of this function is to allow multiple events to be collected at once, to minimize
screen redraws"*
([screen.go#L106-L111](https://github.com/gdamore/tcell/blob/v2.8.1/screen.go#L106-L111)).

**tview** uses two bounded channels of 100 with blocking sends
([application.go#L11-L17](https://github.com/rivo/tview/blob/c15b79f/application.go#L11-L17)),
and `QueueUpdate` is a rendezvous — it blocks the caller until the function has run on the
main loop
([#L916-L921](https://github.com/rivo/tview/blob/c15b79f/application.go#L916-L921)). It draws
synchronously per event with no frame-rate limit, except a 50 ms resize debounce
([#L475-L492](https://github.com/rivo/tview/blob/c15b79f/application.go#L475-L492)).

## gpui

The clearest source-level statement of the dirty-bit / message split.

One global `pending_effects: VecDeque<Effect>` plus a `pending_notifications:
FxHashSet<EntityId>` (`crates/gpui/src/app.rs:702`, `:736`). Dedup is in `push_effect`
(`app.rs:1657-1672`):

```rust
Effect::Notify { emitter } => {
    if !self.pending_notifications.insert(*emitter) {
        return;                       // already pending — drop this one
    }
}
```

The set entry is cleared at *dispatch* time (`apply_notify_effect`, `app.rs:1788-1796`), so
the dedup window is enqueue-until-delivery and a notify raised inside an observer is
delivered again in the same flush. Notify carries no payload (`app.rs:1095-1108`), so nothing
is lost by collapsing.

`emit` has no dedup arm — plain `push_back` (`app/context.rs:765-780`), delivered one-for-one
in order (`app.rs:1798-1808`).

Flushing is driven by update nesting, not a tick: `finish_update` flushes only when
`pending_updates == 1` (`app.rs:1080-1093`), so a whole nested call tree produces one flush.
When the queue drains, `flush_effects` does not draw — it calls `schedule_frame()` on dirty
windows (`app.rs:1728-1740`); the actual draw happens in the platform frame callback
(`window.rs:1681-1815`). A window keeps one `dirty` bool plus a `dirty_views` set, and the
platform waker fires only on the false→true edge (`window.rs:118-129`). **A burst of a
thousand notifies across fifty entities in one window produces one draw.** Per-window
bookkeeping literally counts "how many invalidations were coalesced into it"
(`window.rs:128-136`).

Background work returns via the foreground executor rather than a post-to-UI-thread API:
`BackgroundExecutor::spawn` requires `Send` (`executor.rs:124`), the foreground one does not
and ignores priority — *"Priority is ignored for foreground tasks - they run in order on the
main thread"* (`executor.rs:384`). The idiom is foreground task → `cx.background_spawn(fut)
.await` → `cx.update(...)`, whose `App::update` flushes on exit (`async_context.rs:165-169`).

Browse: <https://github.com/zed-industries/zed/blob/2518540/crates/gpui/src/app.rs>

## Qt

`QCoreApplication::compressEvent` is `\internal` — it does not appear on doc.qt.io for Qt 5 or
Qt 6, so the following is from source (`qt/qtbase`, branch `6.8`).

The hook is inside `postEvent` (`src/corelib/kernel/qcoreapplication.cpp:1716-1720`), gated on
a fast-path atomic; its doc comment reads *"Returns `true` if `event` was compressed away
(possibly deleted) and should not be added to the list."* The base class compresses exactly
`QEvent::Timer` (matched on `timerId()`) and `QEvent::Quit`, by linear scan of the pending
queue (`:1742-1777`). `QGuiApplication::compressEvent` adds nothing
(`src/gui/kernel/qguiapplication.cpp:2097-2100`). `QApplication::compressEvent`
(`src/widgets/kernel/qapplication.cpp:785-835`) adds `UpdateRequest`, `UpdateLater`,
`LayoutRequest`, `Resize`, `Move`, `LanguageChange` — and its per-type bodies are the sharpest
statement of the three collapse semantics in one switch:

```cpp
case QEvent::Resize:  ... m_size = ...->size();     break;   // latest wins
case QEvent::Move:    ... m_pos  = ...->pos();      break;   // latest wins
case QEvent::UpdateLater: ... m_region += ...->region(); break; // union, N into one
case QEvent::UpdateRequest: case QEvent::LanguageChange: case QEvent::LayoutRequest:
                      break;                                 // idempotent, just drop
```

`MouseMove` and `Paint` are **not** in this list; Qt's mouse-move compression is at the QPA
layer (UNVERIFIED — the QPA plugins were not read).

Queued signals are not compressed: `QMetaCallEvent` appears in no `compressEvent` switch, and
the docs say only that *"the slot is invoked when control returns to the event loop of the
receiver's thread"* and that *"Qt needs to copy the arguments to store them in an event behind
the scenes"*
([ConnectionType](https://doc.qt.io/qt-6/qt.html#ConnectionType-enum)). Posting order:
*"Events are sorted in descending priority order… Events with equal priority will be processed
in the order posted"*
([postEvent](https://doc.qt.io/qt-6/qcoreapplication.html#postEvent)). No bound and no
back-pressure are documented, and the source appends unconditionally.

The repaint rule is stated outright — [QWidget::update()](https://doc.qt.io/qt-6/qwidget.html#update):
*"Calling update() several times normally results in just one paintEvent() call."* So is timer
overrun — [QTimer](https://doc.qt.io/qt-6/qtimer.html): *"In such a case of timeout overrun, Qt
will emit timeout() only once, even if multiple timeouts have expired, and then will resume
the original interval."*

## Win32

The oldest statement of the split, and the only system in the sweep with a hard queue bound.

[About Messages and Message Queues](https://learn.microsoft.com/en-us/windows/win32/winmsg/about-messages-and-message-queues):

> With the exception of the **WM_PAINT** message, the **WM_TIMER** message, and the
> **WM_QUIT** message, the system always posts messages at the end of a message queue. This
> ensures that a window receives its input messages in the proper first in, first out (FIFO)
> sequence. … In addition, multiple **WM_PAINT** messages for the same window are combined
> into a single **WM_PAINT** message, consolidating all invalid parts of the client area into
> a single area.

That final clause is the same union semantics as Qt's `UpdateLater` and gpui's `dirty_views`,
written twenty years earlier.

Retrieval order, from [GetMessage](https://learn.microsoft.com/en-us/windows/win32/api/winuser/nf-winuser-getmessage)
(sent messages appear **twice**): sent → posted → input (hardware) and system internal events
→ sent again → `WM_PAINT` → `WM_TIMER`.

[WM_PAINT](https://learn.microsoft.com/en-us/windows/win32/gdi/wm-paint): *"The system sends an
internal WM_PAINT message only once… the system does not post or send further WM_PAINT
messages until the window is invalidated."*
[WM_TIMER](https://learn.microsoft.com/en-us/windows/win32/winmsg/wm-timer): *"The WM_TIMER
message is a low-priority message. The GetMessage and PeekMessage functions post this message
only when no other higher-priority messages are in the thread's message queue."*

Mouse input is explicitly lossy —
[Mouse Input](https://learn.microsoft.com/en-us/windows/win32/inputdev/about-mouse-input):
*"When mouse messages are posted faster than a thread can process them, the system discards
all but the most recent mouse message."* Note **discards**, not merges: intermediate positions
are gone.

The queue bound, from [PostMessageW](https://learn.microsoft.com/en-us/windows/win32/api/winuser/nf-winuser-postmessagew):
*"There is a limit of 10,000 posted messages per message queue… To adjust this limit, modify
the following registry key… `USERPostMessageLimit`… The minimum acceptable value is 4000."*
`GetLastError` returns `ERROR_NOT_ENOUGH_QUOTA`. And About Messages: *"A common programming
error is to assume that the PostMessage function always posts a message. This is not true when
the message queue is full. An application should check the return value… and, if it has not
been, repost it."* This is the only producer-visible signal in the whole sweep, and it is a
failure return, not blocking — the caller owns the retry.

## Flutter

**Lossless transport, lossy render.** `StreamBuilder` calls `setState` once per stream event
([async.dart:133-138](https://raw.githubusercontent.com/flutter/flutter/master/packages/flutter/lib/src/widgets/async.dart)),
but its class doc is unusually candid that `build` sees a subsequence:

> Widget rebuilding is scheduled by each interaction, using `State.setState`, but is otherwise
> decoupled from the timing of the stream. The `builder` is called at the discretion of the
> Flutter pipeline, and will thus receive a timing-dependent sub-sequence of the snapshots
> that represent the interaction with the stream. (async.dart:347-351)

> the `builder` may be called with any ordered sub-sequence of the following snapshots that
> includes the last one (async.dart:353-356)

Crucially, every event is folded into `_summary` (async.dart:136) — **the accumulated state is
lossless; only the intermediate renders vanish.** That is the one hybrid in the sweep: fold
losslessly, render the fold at most once per frame.

Three nested dedups: per-element (`markNeedsBuild`: `if (dirty) { return; }`,
framework.dart:5403-5407, with `setState` documented as *"idempotent, there is no benefit to
calling it more than once per State per frame"*, framework.dart:1144-1145), per-build-scope
(framework.dart:2735-2742), and one global flag (`scheduleFrame`: `if (_hasScheduledFrame ||
!framesEnabled) { return; }`, binding.dart:946-949).

No back-pressure: `StreamBuilder` never calls `pause()`/`resume()`.

## Android and Kotlin — the "coalesce by default" data points

`Handler`/`Looper` is FIFO and lossless; the only documented loss is teardown (`postDelayed`:
*"if the looper is quit before the delivery time of the message occurs then the message will
be dropped"*, [Handler](https://developer.android.com/reference/android/os/Handler)).
Coalescing is an app-level idiom, opt-in per call site: `removeMessages(what)` (*"Remove any
pending posts of messages with code 'what' that are in the message queue"*) followed by
`sendEmptyMessage(what)`, or guarded by `hasMessages(what)`. No queue bound is stated either
way (UNVERIFIED).

`Choreographer` is per-Looper-thread — *"Each Looper thread has its own choreographer"* — and
`View.postOnAnimation` runs *"once at the beginning of the next display frame"*
([Choreographer](https://developer.android.com/reference/android/view/Choreographer)).
UNVERIFIED: the familiar "N `invalidate()` calls collapse to one `onDraw`" claim is **not** on
the [View](https://developer.android.com/reference/android/view/View) page, which says only
that *"onDraw will be called at some point in the future."*

**`LiveData` is lossy by default in two independent ways, with no opt-out.**
[LiveData](https://developer.android.com/reference/androidx/lifecycle/LiveData): *"When data
changes while the owner is not active, it will not receive any updates. If it becomes active
again, it will receive the last available data automatically."* And
[postValue](https://developer.android.com/reference/androidx/lifecycle/MutableLiveData): *"If
you called this method multiple times before a main thread executed a posted task, only the
last value would be dispatched."* Stated as contract, not caveat.

Kotlin Flow is the one system that puts the whole menu in the API, labelled:

| Operator | Loss | Back-pressure |
|---|---|---|
| [`buffer()`](https://kotlinlang.org/api/kotlinx.coroutines/kotlinx-coroutines-core/kotlinx.coroutines.flow/buffer.html) — *"the emitter is suspended when the buffer overflows, to let collector catch up"* | lossless (default `SUSPEND`) | **yes** |
| [`conflate()`](https://kotlinlang.org/api/kotlinx.coroutines/kotlinx-coroutines-core/kotlinx.coroutines.flow/conflate.html) — *"emitter is never suspended due to a slow collector, but collector always gets the most recent value emitted"*; = `buffer(0, DROP_OLDEST)` | lossy, latest wins | no |
| [`collectLatest`](https://kotlinlang.org/api/kotlinx.coroutines/kotlinx-coroutines-core/kotlinx.coroutines.flow/collect-latest.html) — *"when the original flow emits a new value then the action block for the previous value is cancelled"* | lossy on the consumer's *work* | n/a |
| [`StateFlow`](https://kotlinlang.org/api/kotlinx.coroutines/kotlinx-coroutines-core/kotlinx.coroutines.flow/-state-flow/index.html) — *"Updates to the value are always conflated. So a slow collector skips fast updates, but always collects the most recently emitted value."* | lossy, **baked into the type** | no |
| [`SharedFlow`](https://kotlinlang.org/api/kotlinx.coroutines/kotlinx-coroutines-core/kotlinx.coroutines.flow/-shared-flow/index.html) — `replay` + `extraBufferCapacity` + `onBufferOverflow` (SUSPEND / DROP_OLDEST / DROP_LATEST) | configurable, incl. batched-carrying-all via replay | yes if SUSPEND |

The cost of coalesce-by-default is visible in both Android cases: **neither `LiveData` nor
`StateFlow` can have conflation turned off.** The documented escape is to switch primitive —
`SharedFlow` with real buffer capacity — not to pass a flag. A default that cannot be locally
overridden forces a type change at every call site that needs the other semantics.

## Apple

[Notification Queues](https://developer.apple.com/library/archive/documentation/Cocoa/Conceptual/Notifications/Articles/NotificationQueues.html)
(archived conceptual doc; the modern class reference renders client-side and was not readable):

> Coalescing is a process that removes from a queue notifications that are similar in some way
> to a notification that was queued earlier. If the new item is similar to a notification
> already queued, the new one isn't queued, and all similar notifications (except the first
> one in the queue) are removed.

Opt-in and keyed: `NSNotificationCoalescingOnName` / `OnSender` (OR-able) /
`NSNotificationNoCoalescing`. Posting styles bind the flush to the run loop: `NSPostASAP`
(*"when the current iteration of the run loop completes"*), `NSPostWhenIdle` (*"only when the
run loop is in a wait state"*), `NSPostNow`.

[`setNeedsDisplay`](https://developer.apple.com/documentation/uikit/uiview/setneedsdisplay()):
*"This method makes a note of the request and returns immediately. The view is not actually
redrawn until the next drawing cycle, at which point all invalidated views are updated."*
`NSView.setNeedsDisplay(_:)` is the union case — *"increasing the view's existing invalid
region to include it"*
([needsDisplay](https://developer.apple.com/documentation/appkit/nsview/needsdisplay)).
[`CADisplayLink`](https://developer.apple.com/documentation/quartzcore/cadisplaylink) gives one
callback per refresh; *"actual frames per second may differ from the preferred value."*
Run-loop input sources are delivered, not polled — *"Input sources deliver asynchronous
events"*
([Run Loops](https://developer.apple.com/library/archive/documentation/Cocoa/Conceptual/Multithreading/RunLoopManagement/RunLoopManagement.html)).

## Rx / ReactiveX — the vocabulary

| Operator | Loss profile | Buffer |
|---|---|---|
| [`debounce`](https://reactivex.io/documentation/operators/debounce.html) — *"filters out items… that are rapidly followed by another emitted item"* | lossy, latest per quiet run | 1 slot |
| [`throttleFirst`](https://reactivex.io/documentation/operators/sample.html) — *"emits the first item… in each sampling period rather than the most recently emitted item"* | lossy, **first** wins | none |
| [`sample` / `throttleLast`](https://reactivex.io/documentation/operators/sample.html) — *"emit the most recent items emitted by an Observable within periodic time intervals"* | lossy, latest per tick | 1 slot |
| [`buffer`](https://reactivex.io/documentation/operators/buffer.html) — *"periodically gather items… into bundles and emit these bundles rather than emitting the items one at a time"* | **batched, N into one carrying all** | accumulating list |
| [`window`](https://reactivex.io/documentation/operators/window.html) — *"it emits Observables, each one of which emits a subset"* | lossless, chunks as substreams | inner subscriber |

`buffer` carries a sharp edge worth froth's attention because froth's `BatchedFunc` has the
same shape: *"if the source Observable issues an onError notification, Buffer will pass on
this notification immediately without first emitting the buffer it is in the process of
assembling, even if that buffer contains items."* **A partial batch is lost on error.**

Back-pressure ([page](https://reactivex.io/documentation/operators/backpressure.html)):
`Flowable` has it, `Observable` does not; default operator buffer is 128;
`MissingBackpressureException` *"signals that an asynchronous operator's internal buffer has
overflowed."* The three overflow strategies map exactly onto the three collapse semantics:
`onBackpressureBuffer` (lossless), `onBackpressureDrop` (drops newest),
`onBackpressureLatest` (*"holds on to the most-recently emitted item… It drops any other items
that it observes between requests"*).

## Axis A summary table

| System | Default on the message path | Who buffers | Back-pressure | Scope of collapse |
|---|---|---|---|---|
| Elm | lossless, synchronous `update` | per-effect-manager mailbox, unbounded | none | view only (rAF), global |
| Textual | lossless + opt-in latest-wins for 5 types | framework, per widget, unbounded | none (except `call_from_thread`) | per widget for messages, global for the 60 fps pass |
| Bubble Tea | lossless, 1 msg = 1 `Update` | nothing — unbuffered channel | **full**, onto every producer incl. tty | frames only, global |
| tcell | **lossy** — cap 10, `PostEvent` drops | framework, bounded | input yes; `PostEvent` no | none |
| tview | lossless, cap 100, blocking; `QueueUpdate` is a rendezvous | framework, bounded | yes | resize only (50 ms) |
| gpui | `emit` lossless; `notify` deduped | `pending_effects`, unbounded | none | per entity id; per window for draw |
| Qt | queued signals lossless; 8 event types compressed | per-thread `QPostEventList`, unbounded | none documented | per receiver |
| Win32 | posted lossless to 10k then **fails**; paint/timer/mouse collapsed | per-thread queue, bounded | **yes**, as a failure return | per window |
| Flutter | stream lossless, fold lossless, render coalesced | Dart stream; widget keeps only the fold | none | per element + global frame |
| Android Handler | lossless | framework queue | none | none (app-level idiom) |
| LiveData / StateFlow | **lossy, latest wins, not overridable** | single slot | none | per value holder |
| Kotlin Flow | configurable per operator | operator | yes with `SUSPEND` | per flow |
| Rx | configurable per operator | operator | `Flowable` only | per stream |

---

# AXIS B — making it deterministic in tests

## Go `testing/synctest`

Release history, from the release notes: Go 1.24 shipped it as *"the new experimental
testing/synctest package… must be enabled by setting `GOEXPERIMENT=synctest` at build time"*
with `Run` + `Wait` ([go1.24](https://go.dev/doc/go1.24)); Go 1.25: *"The experiment has now
graduated to general availability in Go 1.25. The old API is still present if
`GOEXPERIMENT=synctest` is set, but will be removed in Go 1.26"*, `Run` → `Test(t, f)`
([go1.25](https://go.dev/doc/go1.25)). froth requires 1.27, so only `Test` exists.

The model, from the package doc
([synctest.go](https://tip.golang.org/src/testing/synctest/synctest.go)):

> Within a bubble, the `time` package uses a fake clock. Each bubble has its own clock. The
> initial time is midnight UTC 2000-01-01. Time in a bubble only advances when every goroutine
> in the bubble is durably blocked.

> A goroutine in a bubble is "durably blocked" when it is blocked and can only be unblocked by
> another goroutine in the same bubble.

And the three-way rule when the bubble goes quiet: `Wait` returns if called; otherwise time
advances to the next unblocking time if the root goroutine has not exited; otherwise it is a
deadlock and `Test` panics.

Durably blocking, verbatim: a blocking send or receive on a channel **created within the
bubble**; a select where every case is such a channel; `sync.Cond.Wait`;
`sync.WaitGroup.Wait` when `Add` was called in the bubble; `time.Sleep`. Explicitly **not**
durably blocking: locking a `sync.Mutex` or `sync.RWMutex`; blocking on I/O such as reading
from a network socket; system calls. The blog gives the reasons: *"Mutexes are often used to
guard global state, so a bubbled goroutine will often need to acquire a mutex held outside its
bubble"*, and *"Syscalls and cgo calls are not durably blocking. We can only reason about the
state of goroutines executing Go code"* ([Testing time](https://go.dev/blog/testing-time)).

Affinity is enforced by panics: *"A channel, `time.Timer`, or `time.Ticker` created within a
bubble is associated with it. Operating on a bubbled channel, timer, or ticker from outside
the bubble panics."* A `sync.WaitGroup` declared as a package variable *"cannot be associated
with a bubble and operations on it may not be durably blocking."*

**Real network reads are the killer, and the package's own example says so** — the HTTP
example carries this comment verbatim: *"We cannot use a loopback network connection for this
test, because goroutines blocked on network I/O prevent a synctest bubble from becoming
idle."* The blog: *"Tests of networked programs using synctest usually cannot use real network
connections. Instead, they should use an in-memory fake."*

`Test` waits for all bubble goroutines to exit; a goroutine still *running* is waited for, a
goroutine still *blocked* panics, with two distinct runtime messages
(`$GOROOT/src/runtime/synctest.go:238-247`):

```go
if bubble.done {
    reason = "deadlock: main bubble goroutine has exited but blocked goroutines remain"
} else {
    reason = "deadlock: all goroutines in bubble are blocked"
}
```

Other guardrails in the same file: `panic("synctest.Run called from within a synctest
bubble")` (`:174`), `panic("goroutine is not in a bubble")` (`:284`), `panic("wait already in
progress")` — `Wait` is not concurrently callable (`:292`). *"Time stops advancing when the
root goroutine of the bubble exits"* (`:225-228`).

The lockstep idiom is act → `synctest.Wait()` → assert. `synctest.Sleep` exists because
sleeping alone is racy: *"If the test itself and another goroutine running the system under
test sleeps for the exact same amount of time, it's unpredictable which of the two goroutines
will run first."*

## gpui `TestAppContext`

The deterministic core now lives in `crates/scheduler/src/test_scheduler.rs`, with
`crates/gpui/src/platform/test/dispatcher.rs` a thin delegate. `TestAppContext::build`
constructs both executors over one `TestDispatcher` (`test_context.rs:126-149`).

`run_until_parked` is `while self.tick(false) {}` (`dispatcher.rs:76-78`) → `while self.step()
{}` (`test_scheduler.rs:259-263`). Its doc comment is itself the interesting artifact:

> Under the scheduler-backed test dispatcher, `tick()` will not advance the clock, so a
> pending timer can keep `has_pending_tasks()` true even after all currently-runnable tasks
> have been drained. To preserve the historical semantics that tests relied on (drain all work
> that can make progress), we advance the clock to the next timer when no runnable tasks
> remain.

Ordering is a single queue with session affinity — *"For foreground tasks (with session_id),
only the first task from each session is a candidate (to preserve intra-session ordering); for
background tasks (no session_id), all are candidates"* (`test_scheduler.rs:337-364`).

**gpui is the only system in the sweep that adds seeded randomized interleaving on top of
determinism.** `rng: Arc<Mutex<StdRng>>` seeded `StdRng::seed_from_u64(config.seed)`
(`:35-37`, `:163`); with `randomize_order` the next runnable is picked by priority-weighted
RNG (`:376-399`) *and* newly scheduled tasks are inserted at a random queue index rather than
appended (`:606-624`, `:634-651`). `#[gpui::test]` takes `seed = N`, `seeds(1,2,3)`,
`iterations`, `retries`, `on_failure` (`gpui_macros/src/test.rs:13-18`), with `SEED` and
`ITERATIONS` env overrides; a failure prints `failing seed: {seed}` and how to rerun. Same
seed, identical interleaving; many seeds, race-shaking; reproduction by one env var.

Leak and hang detection fall out of owning the executor: `forbid_parking` is the default
(`dispatcher.rs:24-31`) and parking panics with *"Parking forbidden. Re-run with … to show
pending traces"* (`test_scheduler.rs:548-559`); the generated test teardown runs
`run_until_parked()`, then `forbid_parking()`, then `cx.quit()`. When parking is allowed there
is a 15 s wall-clock deadline: *"Test timed out after 15 seconds while parking. This may
indicate a deadlock or missing waker"* (`:488-546`).

Real I/O is quarantined rather than modelled: `dispatch_after` panics with *"dispatch_after
should not be called in tests. Use BackgroundExecutor::timer() which uses the scheduler's
native timer"* (`dispatcher.rs:132-137`), and `spawn_realtime` (`:139-143`) is the single
documented escape to a real OS thread.

## Flutter

[`pump`](https://api.flutter.dev/flutter/flutter_test/WidgetTester/pump.html): *"Triggers a
frame after `duration` amount of time. This makes the framework act as if the application had
janked (missed frames) for `duration` amount of time, and then received a 'Vsync' signal."*
One call, one frame, fake clock advanced.

[`pumpAndSettle`](https://api.flutter.dev/flutter/flutter_test/WidgetTester/pumpAndSettle.html):
*"Repeatedly calls pump with the given duration until there are no longer any frames
scheduled"* — and the documented failure mode, on the same page: *"If there is an infinite
animation in progress (for example, if there is an indeterminate progress indicator spinning),
this method will throw."* The doc's own advice is to *"figure out exactly why each frame is
needed, and then to pump exactly as many frames as necessary"* — quiescence-waiting is
structurally impossible against a repeating timer, and explicit stepping is the answer.

The clock is installed by the binding: `AutomatedTestWidgetsFlutterBinding.runTest()` *"Call[s]
the testBody inside a `FakeAsync` scope on which pump can advance time"*.
[`FakeAsync`](https://pub.dev/documentation/fake_async/latest/fake_async/FakeAsync-class.html)
is *"a class that mocks out the passage of time within a Zone"*, with `elapse`,
`flushMicrotasks` (*"Runs all pending microtasks… until there are no more"*), `flushTimers`,
`pendingTimers`.

Timer-leak detection is free because every timer is a `FakeTimer` in the zone —
`_verifyInvariants()` asserts `!timersPending` with *"A Timer is still pending even after the
widget tree was disposed"* and dumps each pending timer's duration and creation stack (exact
line UNVERIFIED).

The escape hatch names its own trap:
[`runAsync`](https://api.flutter.dev/flutter/flutter_test/WidgetTester/runAsync.html) is *"for
callers who need to call asynchronous methods where the methods spawn isolates or OS threads
and thus cannot be executed synchronously"*, and *"if your widget test hangs and you are using
runAsync, chances are your code depends on the result of a task that did not complete. Fake
async environment is unable to resolve a future that was created in runAsync."*

## Textual `Pilot`

**No fake clock.** `_time.py:1-51` hardcodes `time = monotonic` and wraps real
`asyncio.sleep`; `run_test()` runs the real app as a real task on the real loop
(`app.py:2201-2204`), headless.

Two settle mechanisms, and only one is deterministic:

- **Queue-drain (deterministic).** `Pilot._wait_for_screen` (`pilot.py:473-533`) posts a
  `call_later` callback onto every pump's queue and awaits all of them; because each pump
  drains in order, the callback firing proves everything queued before it has been dispatched.
  The 30 s timeout raises `WaitForScreenTimeout`, documented as *"Exception raised if messages
  aren't being processed quickly enough. If this occurs, the most likely explanation is some
  kind of deadlock in the app code."*
- **CPU-idle polling (heuristic).** `wait_for_idle`, verbatim docstring: *"This will compare
  wall clock time with process time. If the process time is not advancing at the same rate as
  wall clock time it means the process is idle… When the process is idle it suggests that
  input has been processed and the state is predictable enough to test."* Implemented as a
  20 ms real-time sleep loop over `process_time()` deltas.

`Pilot.pause()` is drain-then-`wait_for_idle(0)` (or a literal `asyncio.sleep(delay)`).
`App.workers.wait_for_complete()` is a plain `asyncio.gather` over `Worker.wait()`
(`worker_manager.py:172-181`) and `Pilot` never calls it — the test must. The
[testing guide](https://textual.textualize.io/guide/testing/) concedes the residual
nondeterminism: *"Some actions in a Textual app won't change the state immediately. For
instance, messages may take a moment to bubble from the widget that sent them"*, recommending
`pause(delay=...)`.

**Textual is the one system here that does not own the clock, and it is also the one whose own
docs tell test authors to insert manual delays.**

## Elm

Elm's testability comes from effects being inert data, stated in the
[`Platform.Cmd`](https://package.elm-lang.org/packages/elm/core/latest/Platform-Cmd) doc:

> Elm has **managed effects**, meaning that things like HTTP requests or writing to disk are
> all treated as *data* in Elm… Perhaps unexpectedly, this managed effects idea is the heart
> of why Elm is so nice for testing, reuse, reproducibility, etc.

The limitation that shapes elm-program-test, verbatim from its guidebook: *"Elm does not
currently provide a way to inspect `Cmd` values to see which commands they represent"*
(<https://elm-program-test.netlify.app/cmds.html>). So the library cannot intercept real
`Cmd`s; it requires the **application** to return a user-defined, inspectable effect type plus
a `withSimulatedEffects` mapping into `SimulatedEffect`, and `withSimulatedSubscriptions`
returning `SimulatedSub`s. Time is a manual logical clock: `advanceTime` *"simulates the
passing of time… will cause any pending `Task.sleep`s to trigger if their delay has elapsed."*

Correction to a commonly cited name: there is no `simulateSubscription` in the current API
(v4.0.1); it is `withSimulatedSubscriptions` plus `simulateIncomingPort`. That Elm's runtime
is "not injectable" is an inference from the two quotes above, not a documented statement
(UNVERIFIED).

## Who owns the loop: the two sides, and where Bubble Tea sits

**Framework owns the loop, the test steps it:** Flutter `pump(duration)`; Go
`synctest.Wait()`; gpui `run_until_parked()` / `advance_clock()`; React `act()` — *"React
provides a helper called `act()` that makes sure all updates related to these 'units' have
been processed and applied to the DOM before you make any assertions… Any updates triggered
within the `actFn` are added to an internal act queue, which are then flushed together"*
(<https://react.dev/reference/react/act>), with a step-driven mock scheduler
(`packages/scheduler/src/forks/SchedulerMock.js`: `unstable_flushAll`,
`unstable_flushUntilNextPaint`, `unstable_advanceTime`); Kotlin `runCurrent()` /
`advanceUntilIdle()`; elm-program-test calling the pure `update`.

**The component owns the loop and the test can only start, send, and poll:** this is
`teatest`, the Bubble Tea ecosystem's own answer, and it matters most here.
`NewTestModel` spawns the real program in a goroutine; `Send` forwards to `program.Send` with
no acknowledgement that it was processed; `WaitFor` is documented as *"keeps reading from `r`
until the condition matches. Default duration is 1s, default check interval is 50ms"* and
implemented as a sleep-poll; `WaitFinished` *"only returns once the program has finished
running or when it times out"*, with `tb.Fatalf("timeout after %s")`
(<https://raw.githubusercontent.com/charmbracelet/x/main/exp/teatest/teatest.go>,
<https://pkg.go.dev/github.com/charmbracelet/x/exp/teatest>). **There is no step primitive
anywhere in that file** — no synchronous "process one message and return", no direct `Update`
call. The entire exported API is start / send / read-output-with-timeout / golden-compare.

Swing sits on the quiescence side: `SwingUtilities.invokeAndWait` *"causes doRun.run() to be
executed synchronously on the AWT event dispatching thread. This call blocks until all pending
AWT events have been processed"*
(<https://docs.oracle.com/javase/8/docs/api/javax/swing/SwingUtilities.html>) — drain and
wait, never step.

**Injectable scheduler/dispatcher is an orthogonal third thing**, and it is common. Kotlin:
`runTest` *"runs the test code, automatically skipping delays"*, `TestCoroutineScheduler` is
*"the shared source of virtual time"*, `StandardTestDispatcher` queues-and-pumps while
`UnconfinedTestDispatcher` runs eagerly, `Dispatchers.setMain` swaps the main dispatcher, and
Android's official guidance is to **inject** a `CoroutineDispatcher` rather than hardcode
`Dispatchers.IO` (<https://developer.android.com/kotlin/coroutines/test>). RxJava
`TestScheduler` has `advanceTimeBy`, `advanceTimeTo`, `triggerActions` (*"triggers any actions
that have not yet been triggered and that are scheduled to be triggered at or before this
Scheduler's present time"*) with no auto-advance, plus global injection via
`RxJavaPlugins.setIoSchedulerHandler` and friends. Espresso `IdlingResource` is the inverse
shape — the *application* opts in to being observable, per async operation
(<https://developer.android.com/training/testing/espresso/idling-resource>).

## Four cross-cutting facts

1. **Every step-driven framework had to own or replace the clock.** synctest virtualizes
   `time` at the runtime level; Flutter installs `FakeAsync` via a Zone; gpui has a
   `TestClock`; Kotlin has `TestCoroutineScheduler`; elm-program-test has `advanceTime`.
   Textual is the one that does not, and the one that tells authors to sleep.
2. **Real I/O is the universal boundary, and every framework quarantines rather than models
   it** — synctest's `net.Pipe`, gpui's `spawn_realtime` (with `dispatch_after` panicking),
   Flutter's `runAsync` and its documented hang, Kotlin's unskipped real blocking calls.
3. **Two quiescence signals appear and they are not equivalent.** Queue-drain (Textual's
   `_wait_for_screen`, gpui's `run_until_parked`, React's act queue) proves *everything
   enqueued before now has been processed*. Durable-block detection (`synctest.Wait`) proves
   *nothing can make progress without external input* — strictly stronger, and the only one
   that also yields leak and deadlock detection.
4. **Leak detection falls out of owning the primitive, in every case.** Flutter's pending-timer
   assert works because every timer is a `FakeTimer`; synctest's deadlock panic works because
   the runtime knows bubble membership; gpui's `forbid_parking` works because the executor sees
   every park. None is bolt-on.

---

# Verdict — Axis A

**What transfers.**

- **"Default to neither batching nor coalescing" is the majority position on the message
  path, by a wide margin.** Elm, Textual, Bubble Tea, tview, Android `Handler`, Qt queued
  signals, Win32 posted messages and gpui `emit` all deliver every payload-carrying message.
  The collapsing in these systems is confined to payload-free dirty bits and to snapshot-style
  payloads. froth's finding 3 — coalescing drops content, batching does not, so default to
  neither — is an independent rediscovery of a line every one of these systems draws.
- **The delta/snapshot/nothing taxonomy is real and better-attested than "batch vs coalesce"
  alone.** Qt's `compressEvent` switch contains all three semantics side by side: latest-wins
  for `Resize`/`Move`, union for `UpdateLater`, idempotent drop for `UpdateRequest`. Win32's
  WM_PAINT region union and gpui's `dirty_views` are the same union case. froth's `Batched`
  (carry all) versus `Coalesced` (latest wins) covers two of the three; the third — an
  idempotent, payload-free "something happened" — has no froth spelling, and every system in
  the sweep has one.
- **froth's type change under `BatchedFunc` is normal, not exotic.** Rx `buffer` turns
  `Observable<T>` into `Observable<List<T>>`; `window` turns it into a stream of streams.
  What is froth-specific is that Go's `switch msg.(type)` makes the mismatch silent, which is
  why the sketch reaches for a diagnostic. Rx's sharp edge is worth carrying over too: a
  partial buffer is **lost on error**, which is a question froth's `BatchedFunc` plus
  `EndOfStream`/error termination has to answer one way or the other.
- **Back-pressure by construction is the Bubble Tea house style, and froth inherits it.**
  `tea.Send` blocks on an unbuffered channel, so a producer goroutine that emits faster than
  `Update` consumes simply parks. froth's step-function shape makes that even stricter — the
  framework does not call `step` again until it is ready, so a producer cannot outrun the loop
  at all. That is a stronger guarantee than any other system here except tview's
  `QueueUpdate` rendezvous, and it is bought without a queue.

**What does not transfer, or is not reached by the sweep.**

- **No surveyed UI framework puts the collapse choice at the subscription site.** Everywhere
  it is keyed by **receiver identity** and selected by **message type**, chosen by the
  framework: per window (Win32), per QObject and per `QEvent::Type` (Qt), per `EntityId`
  (gpui), per message class via `can_replace` (Textual). Textual is the nearest miss and the
  most instructive: the knob is a method on the *message*, so every sender of an `Update` gets
  the same collapse, and a widget cannot ask for a different discipline on its own stream.
  The precedent for froth's per-producer, caller-chosen knob is in **stream libraries**, not
  UI frameworks: Kotlin's `buffer()`/`conflate()` and Rx's `onBackpressureBuffer`/`Drop`/
  `Latest` are exactly that shape. froth is importing a reactive-streams idea into a UI loop.
  That is not an argument against it; it is a note that the UI prior art neither supports nor
  refutes it, and the stream prior art supports it.
- **The strongest counter-evidence to "default to neither" is Android, twice.** `LiveData` and
  `StateFlow` both coalesce by default and are heavily used. But the documented consequence
  cuts froth's way rather than against it: in **both** cases conflation cannot be turned off,
  and the escape is switching to a different primitive (`SharedFlow` with real capacity), not
  passing a flag. A default that cannot be locally overridden forces a type change at every
  call site that wanted the other semantics. froth is proposing the inverse — an overridable
  default — so the LiveData evidence argues against *unconditional* coalescing, not against
  coalescing as an option.
- **Batching reintroduces the buffer that the step function eliminated, and prior art has no
  good answer for bounding it.** `Batched`/`Coalesced` require pending emissions to
  accumulate somewhere between the producer and the next `Update`. gpui's `pending_effects` is
  an unbounded `VecDeque`; Qt's `QPostEventList` appends unconditionally; Textual's queue is
  unbounded; Elm's mailboxes are unbounded JS arrays. **Win32 is the only system with a bound
  (10,000, `ERROR_NOT_ENOUGH_QUOTA`) and it surfaces as a producer-visible failure return, not
  as blocking.** tcell is the only one that drops silently, and it is the system in the sweep
  whose behaviour is least defensible. The general answer across thirty years is: collapse
  aggressively enough that back-pressure never becomes the mechanism. froth's default does the
  opposite — no collapse — and relies on the step function's natural back-pressure instead,
  which works precisely because froth owns the call and nobody else can push.
- **Nobody splits "coalesce the render" from "coalesce the message" the way froth's constraint
  forces.** In every system, frame coalescing is global and the message path is separate:
  Bubble Tea's 60 fps `standard`/cursed renderer stores only the latest view, Textual's Screen
  repaints a dirty *set* at `TEXTUAL_FPS`, Elm draws once per rAF, gpui draws once per frame
  regardless of notify count, Flutter has one global `_hasScheduledFrame`. This is already
  true for froth by inheritance from Bubble Tea, and it means the delivery-discipline question
  is genuinely only about *message* volume — a per-`Update` cost — never about redraw cost.
  Any argument for coalescing framed as "otherwise we redraw too much" is wrong on this
  substrate.
- **`Coalesced[T]` has thinner UI precedent than `Batched[T]` for *application* messages.**
  Latest-wins in the prior art is nearly always applied to values the framework itself owns
  and understands — a mouse position, a widget size, a view's dirty flag. The one place it is
  applied to arbitrary application values is `LiveData`/`StateFlow`/`conflate`, i.e. the
  state-holder world, not the event world. That does not make froth's progress-percentage case
  wrong; it does mean `Coalesced[T]` is closer to "this stream is a value, not events" than to
  anything Win32 or Qt does, and it is the option most likely to be misused on a stream of
  deltas.

# Verdict — Axis B

**What transfers.**

- **"Framework owns the loop, test steps it" is the dominant pattern and the one with the
  strongest evidence.** Flutter `pump`, gpui `run_until_parked`, React `act`, Kotlin
  `advanceUntilIdle`, synctest `Wait`. Every one is a framework that owns its scheduler. The
  contrast case is directly in froth's neighbourhood: **`teatest`'s entire vocabulary is
  polling and timeouts** — `Send` with no acknowledgement, `WaitFor` at a 50 ms poll interval,
  `Fatalf("timeout after %s")` on every terminal assertion — because Bubble Tea's loop belongs
  to `Program` and no step primitive is exposed. froth already avoids that by driving its own
  update entry point at the single seam ([testing.md](../testing.md)); inverting the producer
  loop extends the same property to async.
- **The injectable-executor pattern is well attested and gpui is the closest match.** gpui
  ships a `TestDispatcher` that *is* the scheduler under test, forbids parking by default,
  panics on `dispatch_after`, and confines real threads to one named escape hatch. Kotlin's
  `TestDispatcher`/`Dispatchers.setMain` and RxJava's `TestScheduler` plus
  `RxJavaPlugins` handlers are the same idea at library scale. froth's injectable goroutine
  runner is a narrower version of this — a seam around the loop rather than around the whole
  scheduler — which is consistent with froth owning only one loop.
- **synctest's guarantees match what froth's fidelity tests claim to check, and its limits are
  benign for froth.** Cancellation on destroy, end-of-stream termination and goroutine-leak
  detection are exactly what bubble exit enforces: `Test` waits for every bubble goroutine and
  panics with `deadlock: main bubble goroutine has exited but blocked goroutines remain` if one
  is stuck. Channels created inside the bubble durably block, which is the normal shape of a
  producer test (`make(chan T)` in the test, fed by the test). froth's own step functions
  select on `ctx.Done()` and a channel — both bubble-internal.
- **Owning the primitive is what buys leak detection, in every system.** froth owns the
  goroutine and the cancellation, which is the precondition Flutter, gpui and Go all rely on.

**What does not transfer, or is a live risk.**

- **froth has two runner implementations; gpui and Flutter have one.** gpui's test dispatcher
  is not a double alongside a real scheduler in tests — it *is* the scheduler, and its answer
  to "does this match production?" is seeded randomized interleaving across many seeds with a
  printed reproduction seed, not a fidelity suite. Flutter's `FakeAsync` likewise replaces the
  clock wholesale. froth's design — a test runner plus a small `synctest` suite against the
  real one — is the classic injected-double arrangement, and the sketch names the classic risk
  ("every test passing against a fake that behaves differently from the real thing"). The
  sweep supports the *mitigation* being necessary; it offers no example of a framework that
  solved it this way, because the others avoided having two implementations at all. That is a
  real disagreement between froth's plan and its closest prior art, and worth stating in those
  terms rather than as a minor caveat.
- **synctest's non-durable-blocking list constrains what the fidelity tests can cover.**
  Mutexes, syscalls and real I/O never durably block. A producer whose step function reads a
  real file or socket cannot be exercised under synctest at all — the bubble never goes idle,
  the clock never advances, and the test hangs or panics as deadlocked. The package's own HTTP
  example says so outright. Whatever the fidelity tests drive must be an in-memory fake, which
  is fine for "cancels on destroy / terminates on EOS / leaks no goroutines" and useless for
  "this tailer works".
- **Channel affinity is a footgun in exactly froth's shape.** *"Operating on a bubbled channel,
  timer, or ticker from outside the bubble panics"*, and a package-level `sync.WaitGroup`
  cannot join a bubble. A test helper that constructs a producer's channel at package scope,
  or a runner that holds a shared `WaitGroup`, will fail confusingly under synctest.
- **`synctest.Wait` is a stronger signal than every queue-drain in the sweep, and froth should
  not confuse the two.** `run_until_parked` and Textual's `_wait_for_screen` prove "everything
  enqueued so far is processed"; `Wait` proves "nothing can progress without external input".
  A lockstep test runner gives froth the first; only the synctest suite gives it the second.
- **Quiescence-waiting is the pattern with documented failure modes, and froth's design avoids
  it — which the sweep endorses.** Flutter's `pumpAndSettle` *"will throw"* against an infinite
  animation and the doc's own advice is to pump an exact number of frames; Textual has no fake
  clock and tells authors to add delays; `teatest` polls. froth's "test says when the step
  runs" is on the right side of that line, and a producer that emits forever is exactly the
  case that breaks the other side.

---

# UNVERIFIED / not established

- Win32: no page states verbatim that multiple `WM_TIMER` messages are not queued. The
  collapse is implied by the generated-on-demand wording in About Messages and corroborated by
  Qt's independent implementation of the same policy, but the literal sentence was not found.
- Win32: the peek-to-drain idiom for coalescing a message class is an inference from
  `PeekMessage`'s filtering parameters, not documented doctrine. `GetMouseMovePointsEx` as the
  recovery path for discarded moves was not fetched.
- Qt: mouse-move compression at the QPA layer — asserted by absence from `compressEvent`, not
  by reading the platform plugins. Qt's posted-event queue being unbounded is inferred from the
  unconditional append, not documented.
- Android: no queue bound is stated for `MessageQueue` either way. The "N `invalidate()` calls
  collapse to one `onDraw`" claim is **not** on the `View` page.
- Apple: `NSNotificationQueue` quotes come from the archived conceptual article; the modern
  class reference renders client-side and was not readable. The "lossless input sources versus
  lossy dirty flag" framing is a synthesis of two quote sets, not a single Apple sentence.
- Rx: `throttleLatest` has no reactivex.io operator page. The claim that Android lowers the
  `Flowable` default buffer to 16 is unverified. `Flowable` quotes came from the reactivex.io
  javadoc mirror (javadoc.io returned 403).
- Flutter: exact line numbers in `binding.dart`, the `HttpClient`-faked-to-400 wording, and
  `FakeAsync` intercepting `Future.delayed`/`DateTime.now()` specifically.
- Textual: `Worker.wait()` implementation not read; "no fake clock" is based on seven files,
  not a repo-wide grep. Textual line numbers are from `main` as of 2026-09-10 and will drift.
- Elm: "the runtime is not injectable" is an inference from the managed-effects doc plus
  elm-program-test's statement that `Cmd` values cannot be inspected; no direct quote exists.
- gpui: repo-wide call sites of `simulate_random_delay` and `spawn_realtime` were not traced.
- No primary source was found for a framework publicly moving from a component-owned loop to a
  step-driven one, nor a Bubble Tea maintainer statement about `teatest`'s polling design.
