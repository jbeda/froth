# Addressing an async result back to the component that asked: thirteen systems

Gathered to test [ADR 0015](../adr/0015-results-come-back-to-whoever-asked.md) against how real
systems behave. Four questions run through every section, because they are four questions and
every system answers them separately:

1. **Address** — how does the result find the component that asked?
2. **Dead recipient** — what happens when it arrives after the component is gone?
3. **Cancellation** — framework on destroy, or the component in teardown?
4. **Stale result** — anything for a component that is *alive and has moved on*?

Plus a fifth that ADR 0015 makes load-bearing: **can an outside object hold a component's
address forever without deregistering, and what happens when it posts to a dead one?**

Sources are primary throughout — vendor documentation and shipping source, read at these
revisions. Anything that could not be confirmed from a primary source is marked `UNVERIFIED`
inline.

| System | Revision read |
|---|---|
| Elm | elm/core 1.0.5, elm/http 2.0.0, `evancz/guide.elm-lang.org@master` |
| Iced (Rust) | tag `0.14.0` (released 2025-12-07) |
| Textual (Python) | tag `v8.2.8` |
| tview / tcell (Go) | tview `v0.42.0`, tcell `v2` / `v3.4.2` |
| Bubble Tea (Go) | bubbletea `v2.0.9`, bubbles `v2.2.1` |
| gpui (Rust) | zed-industries/zed `251854020a9dbea1a388bf392c9bd04fd136a557` (main, 2026-09-11) |
| Qt (C++) | qtbase tag `v6.11.2` (qdoc comments in source; doc.qt.io renders the same text) |
| Flutter / Dart | Flutter 3.47.3 stable (2026-09-09), Dart 3.13.3 |
| SwiftUI | developer.apple.com DocC data API, fetched 2026-09-10 |
| React | facebook/react `main` (CHANGELOG top entry 19.3.0, 2026-09-09); reactjs/react.dev `main` |
| Win32 | learn.microsoft.com reference pages, fetched 2026-09-10 |
| Android | developer.android.com; androidx `androidx-main` @ `91cb564fb9025bee076dab18b61f15b855c068d9` |

---

## The sweep at a glance

| | Address | Post to dead | Cancel on destroy | Stale-result facility |
|---|---|---|---|---|
| **Elm** | pure tagger `(a -> msg)` | delivered to root; app's `_ -> ignore` branch | nobody | none (`Http.cancel` is manual) |
| **Iced** | closure `Task::map` to one `Message` | delivered to root; `else` branch | component, via dropped `Handle` | none (`abortable` is a primitive) |
| **Bubble Tea** | none — root `Update`; `bubbles` self-tags with `id`+`tag` | reaches root, matches nothing | nobody (Cmd goroutines leak by design) | `tag` generation counter in `bubbles` |
| **tview** | closure capture; no address exists | runs against an orphan | no lifecycle exists | none |
| **Textual** | object ref (`DOMNode`), `bubble=False` | `post_message` returns `False` | **framework** (`cancel_node` on unmount) | **`exclusive=True` + `group=`** |
| **gpui** | `WeakEntity<T>` passed into `spawn` | `Err("entity released")` | component, via dropped `Task` | none |
| **Qt** | `QObject*` receiver + queued metacall | **destructor purges every route to itself** | framework for routes/timers; caller for work | `QFutureWatcher::setFuture` purges queued callouts |
| **Flutter** | closure over `State` | throws (debug `FlutterError`, release `TypeError`) | component, in `dispose()` | `FutureBuilder` holds a generation token |
| **SwiftUI** | view identity; `@State` storage | undocumented (`UNVERIFIED`) | **framework** (`.task`) | **`task(id:)`** — cancel and recreate on id change |
| **React** | bound `setState` (fiber + queue) | silent no-op; **warning removed in 18.0** | component, in effect cleanup | none; docs prescribe an `ignore` flag |
| **Win32** | `HWND` + message number | `PostMessage` returns `FALSE` at post time | framework for timers only | none (`SetTimer` replaces by id) |
| **Android (Handler)** | closure capture | **delivered anyway; keeps the dead object alive** | caller, `removeCallbacksAndMessages(null)` | none |
| **Android (scopes/LiveData)** | scope owned by the component | scope already cancelled; observer already removed | **framework** | `collectLatest`, `repeatOnLifecycle` |

---

## A. The two families

The systems split cleanly, and the split is not about quality — it is about whether a
component has an identity the runtime knows.

**Family 1 — no component address exists.** Elm, Iced, Bubble Tea, tview. The result goes to
one global entry point and the application routes it by hand.

Elm is the purest: `Platform.Cmd.map : (a -> msg) -> Cmd a -> Cmd msg`, with the tagger baked
into the effect data rather than into any registry (`elm/core@1.0.5` `src/Task.elm:320-322`:
`cmdMap tagger (Perform task) = Perform (map tagger task)`). Delivery is to the app's single
`update` — `Platform.sendToApp` is documented as *"a message for the main loop of your app…
handled by the overall `update` function"*. Elm is explicit that this is not an oversight: the
guide's structure page says *"Actively trying to make components is a recipe for disaster in
Elm. The root issue is that components are objects … There are no objects in Elm!"*
([structure.md](https://github.com/evancz/guide.elm-lang.org/blob/master/book/webapps/structure.md)).

Iced is the same shape in Rust. `Task::map` is `self.then(move |output| Task::done(f(output)))`
([`runtime/src/task.rs:111-121`](https://github.com/iced-rs/iced/blob/0.14.0/runtime/src/task.rs)),
and the pocket guide's nesting idiom is `task.map(Message::Contacts)`
([`src/lib.rs:424-452`](https://github.com/iced-rs/iced/blob/0.14.0/src/lib.rs)). Iced *does*
address windows by `window::Id`, but only for runtime actions, never for results. Its
`widget::Component` was deprecated in 0.13 with the note *"components introduce encapsulated
state and hamper the use of a single source of truth"* — and it could never return a `Task` at
all, so a deprecated Iced component could not start addressed async work in the first place.

**Family 2 — the component has a runtime identity.** Textual, gpui, Qt, Flutter, Android,
Win32. The address is an object reference, a weak handle, or an OS handle, and the result is
delivered by calling into it.

Textual constructs a worker with the node that started it — `Worker.__init__(self, node: DOMNode, ...)`,
docstring *"node: The widget, screen, or App that initiated the work"* (`worker.py:141-160`) —
and posts lifecycle messages straight at it, `bubble=False` (`worker.py:123`, `203-208`), so
they land on exactly that node's queue and travel nowhere.

gpui hands the future a weak handle. `Context<T>::spawn` takes
`AsyncFnOnce(WeakEntity<T>, &mut AsyncApp) -> R`
([`crates/gpui/src/app/context.rs:237-245`](https://github.com/zed-industries/zed/blob/main/crates/gpui/src/app/context.rs)),
documented as *"The function is provided a weak handle to the entity owned by this context and
a context that can be held across await points"*, and `spawn_in`'s doc says the weak handle is
*"to avoid leaking the entity for a long-running process."*

**froth sits in Family 1's ecosystem and is building Family 2's answer.** That is not a
contradiction the prior art condemns — but it is worth naming that Bubble Tea's own
maintainers wanted it and never shipped it. In
[discussion #751](https://github.com/charmbracelet/bubbletea/discussions/751) (2023-05), a user
asks *"What I want to do is deliver the message to the same model that sent the message"*, and
maintainer meowgorithm replies that they should *"implement the (very simple) pattern in which
models only respond to their own messages."*
[PR #936](https://github.com/charmbracelet/bubbletea/pull/936), which wraps a `Cmd`'s result in
an int-keyed envelope so a parent can route it back, was opened 2024-03-02 and is **still open
and unmerged as of 2026-09-10**; no `Wrap`/routing symbol exists in v2.0.9.

---

## B. Dead recipient — what actually happens

**Silently dropped, everywhere it is defined at all.** Nobody crashes on purpose. The
interesting variation is in *who notices*.

- **Textual** returns a bool. `MessagePump.post_message(message) -> bool`, docstring
  *"Returns: `True` if the message was queued for processing, otherwise `False`"*, guarded by
  `if self._closing or self._closed: return False` (`message_pump.py:860-888`). `Widget` overrides
  it with *"True if the message was posted, False if this widget was closed / closing"*
  (`widget.py:4485-4507`). **This is froth's `PostFunc func(tea.Msg) bool`, already shipping in
  a TUI framework of comparable scope.**
- **gpui** returns a `Result`. `WeakEntity::update` — *"Updates the entity referenced by this
  handle with the given function if the referenced entity still exists. Returns an error if the
  entity has been released"* — implemented as `self.upgrade().context("entity released")?`
  (`app/entity_map.rs:777-787`). The error string is literally `"entity released"`. Failure is
  silent unless the caller propagates it; the idiom is
  `Task<Result<_>>::detach_and_log_err` (`executor.rs:35-64`).
- **Qt** makes it a non-event from the *receiver's* side. `~QObject`: *"All signals to and from
  the object are automatically disconnected, and any pending posted events for the object are
  removed from the event queue"* (`qobject.cpp:1022-1028`), implemented by walking every
  connection (`:1085-1145`) and calling `QCoreApplication::removePostedEvents(q_ptr, 0)`
  (`:206-207`), whose doc is *"The events are not dispatched, instead they are removed from the
  queue."* The sender learns nothing and needs to know nothing. The price is that any route Qt
  was *not* told about — a lambda capturing `this` with no context object — is undefined
  behavior, which is why `QT_NO_CONTEXTLESS_CONNECT` exists.
- **React** made it a deliberate no-op and *removed the warning*. `dispatchSetStateInternal`
  ends with `enqueueConcurrentHookUpdate(...)`, which returns `getRootForUpdatedFiber(fiber)`;
  a detached fiber never reaches a `HostRoot`, so `root` is `null`, nothing is scheduled, and
  the call returns `false`. React 18.0.0's CHANGELOG: *"**No warning about `setState` on
  unmounted components:** Previously, React warned about memory leaks… This warning was added
  for subscriptions, but people primarily run into it in scenarios where setting state is fine,
  and workarounds make the code worse. We've removed this warning."* The PR
  ([#22114](https://github.com/facebook/react/pull/22114)) is blunter: *"there is no actual
  problem here. There's no 'memory leak' here: the POST comes back, we try to set state, the
  component is unmounted (so nothing happens), and then we're done."*
- **Flutter** is the loud outlier and only in debug. `setState() called after dispose(): $this`
  with *"This error happens if you call setState() on a State object for a widget that no longer
  appears in the widget tree… This error can occur when code calls setState() from a timer, from
  an animation callback, or after an asynchronous operation (such as an awaited network request
  or other Future) completes after the widget has been removed from the tree"*
  (`framework.dart:1161-1188`). The whole block is inside `assert(() { ... }())`; in release the
  code falls through to `_element!.markNeedsBuild()` with a null `_element`, so it still throws,
  as a `TypeError` rather than the explanatory error. (`UNVERIFIED`: the release-build message
  was read from the code path, not observed.)
- **Android's Handler is the one genuinely bad answer**, and is instructive precisely because
  it fails the other way: the message is delivered *and the queue's strong reference keeps the
  dead component alive to receive it*. That is what the `HandlerLeak` lint check exists for —
  *"Since this Handler is declared as an inner class, it may prevent the outer class from being
  garbage collected"* (`HandlerDetector.java:51-68`). The only `false` return is looper exit:
  *"a result of true does not mean the message will be processed — if the looper is quit before
  the delivery time of the message occurs then the message will be dropped."*
- **Win32** fails at post time, not delivery time: `PostMessage` *"If the function fails, the
  return value is zero."* `UNVERIFIED`: I could find **no** learn.microsoft.com sentence saying
  that posting to a *destroyed* HWND specifically sets `ERROR_INVALID_WINDOW_HANDLE`. The docs
  do say the check is racy: *"A thread should not use IsWindow for a window that it did not
  create because the window could be destroyed after this function was called. Further, because
  window handles are recycled the handle could even point to a different window"*
  ([IsWindow](https://learn.microsoft.com/en-us/windows/win32/api/winuser/nf-winuser-iswindow)).
- **tview** has no answer because it has no lifecycle. The `Primitive` interface
  (`primitive.go:6-69`) has no `Destroy`, `Close`, `Unmount` or `Detach`; `Pages.RemovePage` is
  slice surgery that notifies nothing. A `QueueUpdate` closure holding a removed primitive
  simply mutates an orphan that is never drawn.

---

## C. Cancellation on destroy — genuinely split

**Framework cancels automatically:** Textual, SwiftUI, Android's lifecycle scopes. Qt is a
partial: it automatically kills *routes* (connections, posted events, timers) but not *work*.

Textual: `Widget._on_unmount` is `self._uncover(); self.workers.cancel_node(self)`
(`widget.py:4849-4851`), reached from `remove()` → `App._prune` → `Prune` → `_close_messages` →
`Unmount`. Docs: *"Workers are tied to the DOM node… if you remove the widget or pop the screen
where they are created, then the tasks will be cleaned up automatically."*
Note the ordering: `_closing = True` is set **before** `Unmount` fires, so a still-running
worker's `post_message` already returns `False` in the window between prune and cancellation.

SwiftUI: *"Use this modifier to perform an asynchronous task with a lifetime that matches that
of the modified view. If the task doesn't finish before SwiftUI removes the view or the view
changes identity, SwiftUI cancels the task."* (`task(name:priority:file:line:_:)`). Note: the
"before this view disappears" wording named in the brief is **not** in current docs — that
phrasing is `UNVERIFIED`.

Android: `viewModelScope` is *"automatically canceled when the [ViewModel] is cleared"*
(`ViewModel.kt:197-198`), via an `AutoCloseable` whose `close()` is `coroutineContext.cancel()`
(`CloseableCoroutineScope.kt:78-84`), run by `ViewModelImpl.clear()` **before** `onCleared()`.
`lifecycleScope` is *"Canceled when the [Lifecycle] is destroyed"* (`Lifecycle.kt:419-422`), and
`LifecycleCoroutineScopeImpl.onStateChanged` removes the observer and cancels at `DESTROYED`
(`:471-477`).

**Component cancels in teardown:** Flutter (`dispose()` — the framework's own `State.dispose`
body does nothing but flip a debug flag, `framework.dart:1334-1341`), React (effect cleanup),
gpui, Iced, Win32 (for anything but timers), Android's Handler era.

gpui and Iced are the same trick: *ownership is the teardown*. gpui's `Task` doc — *"If you drop
a task it will be cancelled immediately. Calling `Task::detach` allows the task to continue
running, but with no way to return a value"* (`scheduler/src/executor.rs:375-382`), and `Task`
is `#[must_use]`. Nothing in `App::release_dropped_entities` (`app.rs:1748-1765`) cancels tasks;
a task stored in the entity's own struct dies when the struct does, and a **detached** one keeps
running and gets `Err("entity released")` on its next upgrade. Iced's equivalent is
`Task::abortable` + `Handle::abort_on_drop` — *"Returns a new `Handle` that will call
`Handle::abort` whenever all of its instances are dropped… really useful if you do not want to
worry about calling `Handle::abort` yourself"* (`task.rs:322-337`) — parked in the state enum:
`Downloading { progress: f32, _task: task::Handle }`.

**Nobody cancels:** Elm, Bubble Tea, tview.

Elm's `Task` effect manager spawns and throws the process id away
(`src/Task.elm:346-351`); its state type is `()`. Cancellation exists only where you captured a
`Process.Id` yourself, or through `elm/http`'s `tracker` + `Http.cancel`. Bubble Tea says so in
a source comment (`tea.go:723-729`):

```go
// Don't wait on these goroutines, otherwise the shutdown
// latency would get too large as a Cmd can run for some time
// (e.g. tick commands that sleep for half a second). It's not
// possible to cancel them so we'll have to leak the goroutine
// until Cmd returns.
```

The only context in bubbletea is program-wide (`p.ctx, p.cancel = context.WithCancel(...)`,
`tea.go:622`); it gates the loops, never an individual `Cmd`.

**The finding that matters more than the tally: cancellation is cooperative everywhere it
exists.** Textual thread workers cannot be interrupted and must poll
`get_current_worker().is_cancelled`, whose docstring says *"Note that cancelled work may still
be running."* Swift's `Task.cancel()` — *"Task cancellation is cooperative and idempotent…
Cancelling a task does not automatically cause arbitrary functions on the task to stop running
or throw errors… a function that doesn't specifically check for cancellation will run to
completion normally, even if the task it is running on is canceled."* SE-0304: *"cancellation
has no effect at all unless something checks for cancellation."* Qt: *"Note that the
cancellation is asynchronous"*, and *"not all running asynchronous computations can be
canceled"* (`qfuturewatcher.cpp:79-92`).

**So "the framework cancels on destroy" never implies "no result arrives after destroy."** Every
system that cancels on destroy *also* needs the dead-recipient path, and has one.

---

## D. Stale results — the split is real and the ADR's framing is wrong about it

The ADR says a framework answer *"would be a guess"*, because froth would need request groups
and a rule for what is superseded, *"all of which depends on what the requests mean, which only
the caller knows."* The second half is right. The conclusion does not follow, because the
systems that ship an answer **do not guess — they make the caller name the group.**

**Textual.** `@work(exclusive=True, group="search")`. `WorkerManager.add_worker`
(`worker_manager.py:65-79`):

```python
if exclusive and worker.group:
    self.cancel_group(worker.node, worker.group)
self._workers.add(worker)
```

and `cancel_group` is scoped to *both* group name and node — `if (worker.group == group and
worker.node == node)` (`:149-153`) — so two widgets on the default group do not cancel each
other. Docs: *"The `exclusive` flag tells Textual to cancel all previous workers before
starting the new one."* It is cancel-previous, not a counter, and since cancellation is
cooperative, a worker that ignores `is_cancelled` still delivers to a live widget. The framework
supplies the *vocabulary*; the caller supplies the meaning.

**SwiftUI.** `task(id:)` is the same idea with `Equatable` instead of a group name:
*"it also cancels and recreates the task when a specified value changes. To detect a change, the
modifier tests whether a new value for the `id` parameter equals the previous value"*, with the
worked example *"When the user chooses a different server, SwiftUI cancels the task and creates
a new one."*

**Android.** `Flow.collectLatest` — *"when the original flow emits a new value then the `action`
block for the previous value is cancelled"*. `repeatOnLifecycle` — *"The [block] will cancel and
re-launch as the lifecycle moves in and out of the target state"* (`RepeatOnLifecycle.kt:30-58`).
Both are library-level (kotlinx.coroutines, androidx), not in the Handler/View layer, which has
nothing.

**Flutter ships a generation counter in framework code.** `_FutureBuilderState`
(`async.dart:595-599`):

```dart
/// An object that identifies the currently active callbacks. Used to avoid
/// calling setState from stale callbacks, e.g. after disposal of this state,
/// or after widget reconfiguration to a new Future.
Object? _activeCallbackIdentity;
```

`_subscribe()` mints `final callbackIdentity = Object()`, and both the `then` and `onError`
callbacks are gated on `if (_activeCallbackIdentity == callbackIdentity)` (`:638-656`).
`_unsubscribe()` is just `_activeCallbackIdentity = null` (`:669-671`) — the old future keeps
running and its result is ignored. This covers both froth's cases at once: dead (dispose) and
alive-but-moved-on (`didUpdateWidget` with a new future).

**Bubble Tea's own standard component library does it too, by hand, three times.**
`spinner/spinner.go:12-18`:

```go
// Internal ID management. Used during animating to ensure that frame messages
// are received only by spinner components that sent them.
var lastID int64
```

and the guard (`:131-153`):

```go
case TickMsg:
    // If an ID is set, and the ID doesn't belong to this spinner, reject
    // the message.
    if msg.ID > 0 && msg.ID != m.id {
        return m, nil
    }
    // If a tag is set, and it's not the one we expect, reject the message.
    if msg.tag > 0 && msg.tag != m.tag {
        return m, nil
    }
    ...
    m.tag++
    return m, m.tick(m.id, m.tag)
```

`id` is the address; `tag` is the generation counter, incremented immediately before issuing
the next tick. It is latest-wins **filtering at the receiver** — the stale goroutine still runs
and still delivers. `timer.TickMsg` documents the model outright: *"a timer will reject ticks
from other timers, so it's safe to flow all TickMsgs through all timers and have them still
behave appropriately."* Two sharp edges worth froth's attention: **`timer` declares `tag` but
never increments it**, so `msg.tag > 0` is always false and its guard is dead code in v2.2.1
(and in v0.21.0); and `spinner`/`timer` use `msg.ID > 0 && msg.ID != m.id` — id `0` is a
wildcard every instance accepts — while `stopwatch` uses the strict `msg.ID != m.id`.

**Left entirely to the caller:** Elm, Iced, gpui, tview, Win32, React, Bubble Tea core. React is
the most explicit, and its prescription is exactly what the ADR describes as the thing to avoid —
a check at every call site:

```js
useEffect(() => {
  let ignore = false;
  async function startFetching() {
    const json = await fetchTodos(userId);
    if (!ignore) { setTodos(json); }
  }
  startFetching();
  return () => { ignore = true; };
}, [userId]);
```

*"If the `userId` changes from `'Alice'` to `'Bob'`, cleanup ensures that the `'Alice'` response
is ignored even if it arrives after `'Bob'`."* And on `AbortController`: *"by itself this is not
enough to protect against race conditions. More asynchronous steps could be chained after the
fetch, so using an explicit flag like `ignore` is the most reliable way to fix this type of
problem."* React chose caller-side flags over a framework primitive, knowing it is verbose, and
made StrictMode double-run effects so the bug shows up in development.

**Two systems put the guard in an adapter object rather than at the call site** — which is close
to the ADR's own "let the service decide" answer. Flutter's `FutureBuilder` (above), and Qt's
`QFutureWatcher::setFuture`, whose `disconnectOutputInterface(true)` reaches
`QCoreApplication::removePostedEvents(q_func(), QEvent::FutureCallOut)`
(`qfuturewatcher.cpp:458-466`, `:480-483`) — already-queued callouts from the old future are
purged, while the old computation keeps running unless `cancel()` was called. Latest-wins for
*delivery*, not for *work*.

---

## E. Holding an address forever

Three distinct answers, and the ADR treats one of them as the only one.

**1. Sender checks (the `PostFunc` family).** Textual's `post_message -> bool`; gpui's
`WeakEntity::update -> Result`; Qt's `QPointer` — *"behaves like a normal C++ pointer `T *`,
except that it is automatically cleared when the referenced object is destroyed (unlike normal
C++ pointers, which become 'dangling pointers' in such cases)"* (`qpointer.qdoc:17-22`). No
registration, no deregistration, address safe to keep forever, posting to a dead one is a
defined negative answer. **froth's decision, with three independent precedents.**

gpui adds a refinement froth should note: the weak handle *self-prunes registries*.
`Context::observe`/`subscribe` wrap the callback so it upgrades the weak handle and returns
`false` when the observer is gone (`app/context.rs:63-115`), and `SubscriberSet::retain` drops
any callback that returned `false` (`subscription.rs:110-133`). The bool is not just an answer
to the sender — it is the garbage-collection signal for the registry.

**2. Receiver purges (Qt's connections).** The dead object's destructor removes every route to
itself, so the sender never needs to check and never needs to be told. Cheap for senders,
and undefined behavior for any route the framework was not told about.

**3. Framework deregisters the holder (Android LiveData).** `observe(LifecycleOwner, Observer)`
— *"If the owner moves to the DESTROYED state, the observer will automatically be removed… If
the given owner is already in DESTROYED state, LiveData ignores the call."* Source confirms both
halves (`LiveData.java:193-196`, `:431-435`). The escape hatch is documented as the caller's
problem: `observeForever` *"is considered as always active… you should manually call
removeObserver."* This is a fourth-party answer the ADR does not mention: the framework, not the
sender and not the receiver, owns the subscription's lifetime.

**Hazards to watch, from the sweep:**

- **Handle recycling makes the bool lie.** Win32's `IsWindow` doc concedes the recycled handle
  could *"point to a different window"*. Raymond Chen's account (devblogs.microsoft.com — a
  Microsoft-hosted developer blog by a Windows shell developer, semi-official, not normative)
  is that NT's `HWND` is a 16-bit table index plus an upper-bit uniquifier bumped on reuse,
  added *"merely to cover for buggy programs"*
  ([2007-07-17](https://devblogs.microsoft.com/oldnewthing/20070717-00/?p=25983)). A `PostFunc`
  returning `true` for a *recycled* handle is worse than one returning `false`.
- **Blocking cross-thread entry points deadlock.** tview's `QueueUpdate` does
  `a.updates <- queuedUpdate{f: f, done: ch}; <-ch`. After `Stop()`, the event loop breaks out
  and there is **no drain of `a.updates`** (`application.go:509-521`) — a late `QueueUpdate`
  blocks forever on `<-ch`, and calling it from *inside* the loop self-deadlocks. Qt's
  `SendMessage` analogue is the same shape: *"The sending thread is blocked until the receiving
  thread processes the message."* Bubble Tea's `Program.Send` is the good version — *"If the
  program has already been terminated this will be a no-op, so it's safe to send messages after
  the program has exited"*, implemented as `select { case <-p.ctx.Done(): case p.msgs <- msg: }`
  (`tea.go:1185-1197`). froth wrapping `Program.Send` inherits the safe shape.
- **Textual's ordering gap.** `_closing` is set at prune time, *before* `Unmount` fires and
  therefore before `cancel_node` runs. There is a real window in which the surface is
  un-postable-to but its workers are still running.

---

## Verdict

### What transfers

**The `PostFunc func(tea.Msg) bool` shape is well-precedented and is the right family.** Textual
ships literally this signature in a TUI framework of comparable ambition; gpui ships the `Result`
form for exactly the case the ADR names (a long-running task holding a weak address so it cannot
leak the component). The ADR's claim that *"without the bool, anything long-lived slowly fills up
with addresses for surfaces that no longer exist"* is not just plausible — gpui builds its entire
subscription GC on that return value. **Agree, and the ADR understates its own case.**

**Cancel-on-destroy transfers, but not as the guarantee the ADR implies.** froth's per-surface
`context.Context` is the same primitive Textual, SwiftUI and Android use, and it is cooperative
in exactly the same way. Every system with framework cancellation also has a defined
dead-recipient path, because cancellation does not stop a goroutine that never checks `ctx.Done()`.
froth has both, so this is **agreement** — but the ADR's sentence *"When the surface is destroyed,
the work is cancelled"* should not be read by any later document as "so results cannot arrive
afterwards." Textual's own ordering gap (post fails before workers are cancelled) shows the
window is real.

**Delivery to the asker, not the root, transfers.** Every Family-2 system does it, and Family 1
only avoids it by having no components. Notably, froth's host ecosystem's own maintainers said
they wanted it and the PR is still unmerged — froth is not going against Bubble Tea's grain, it
is doing the thing Bubble Tea left undone.

### What does not

**Single-threaded message loop removes most of the sweep's hard problems.** Qt's destructor
purge, `QPointer`, `deleteLater`, Flutter's `mounted`, Android's `HandlerLeak`, Win32's HWND
recycling — all exist because a callback can execute against a partially-destroyed object, or
because a queue holds a strong reference that keeps a corpse alive. froth's goroutines cannot
touch surface state at all, and delivery happens inside `Update`. The entire "use-after-free"
half of the prior art is inapplicable, which is why froth can afford the simplest answer.

**Ownership-as-teardown (gpui, Iced) does not transfer.** Both get cancellation for free from
`Drop` on a task handle stored in the component's own struct. Go has no destructor, and froth's
surfaces are not RAII values. froth's explicit `context` cancellation on the unmount walk is the
right substitute, and the ADR's own note that *"cancel has to actually run, every time, or the
context stays registered with its parent forever"* is the cost Rust does not pay.

**Elm's "the compiler makes you write the discard branch" does not transfer.** Elm's stale-result
safety comes from exhaustive matching forcing a `( _, _ ) -> ( model, Cmd.none )` branch — *"Disregard
messages that arrived for the wrong page"* in elm-spa-example. Go's type switch has no such
pressure, so froth's drop must be a runtime behavior, not a compile-time one.

### Where the sweep disagrees with ADR 0015

**1. The diagnostic on a dropped result is the weakest of the four decisions.** The sweep is
unanimous that the *drop* is right. It is not unanimous that the drop should be *noisy*, and the
one framework that had a warning there **removed it after field experience**, with a rationale
that reads as a direct rebuttal: *"people primarily run into it in scenarios where setting state
is fine, and workarounds make the code worse."* Textual warns only under `constants.DEBUG`;
Qt, gpui and Bubble Tea say nothing at all; only Flutter is loud, and only under `assert`.

The specific worry, stated plainly: froth cancels on destroy, cancellation is cooperative, and
some work will not check the context. So "result arrives for a destroyed surface" is not an
anomaly — it is the *expected* steady-state outcome of destroying a surface with work in flight.
A diagnostic that fires on the expected path is the thing React walked back. This does not say
the diagnostic is wrong; froth's `DiagUntakenMessage` precedent (once per type per update) shows
froth knows how to make a diagnostic that does not cry wolf. It says the ADR should decide
whether this diagnostic reports a *bug* or a *routine event*, because React's history says
getting that wrong costs more than the diagnostic is worth.

**2. "Any framework answer to staleness would be a guess" is falsified by the sweep.** Textual's
`exclusive=True, group="search"` guesses nothing: the caller names the group, and the framework
supplies only the mechanism of cancelling the group's previous member. SwiftUI's `task(id:)`
guesses nothing: the caller supplies the `Equatable` id. Android's `collectLatest` guesses
nothing. Six of thirteen systems ship something here, and none of them infers intent — they give
the caller a *vocabulary* for expressing currency and then act on what the caller declared.

The ADR rejected "the framework tracks currency" (correctly — nobody does that) and concluded
"the framework offers nothing" (which is a different claim, and is the minority position by a
narrow margin). The middle option — a declared group or id that froth cancels on, with the
caller deciding what a group *means* — was not considered and is what the strongest-matching
system in the sweep actually ships.

Two things blunt this, and they belong in the same breath. First, every framework-level answer
is *cancel-previous*, and cancellation is cooperative, so none of them actually prevents a stale
result from being delivered — they reduce its likelihood and free the resource. Filtering at the
receiver (`bubbles`' `tag`, Flutter's `_activeCallbackIdentity`) is the only thing that
*prevents* it, and that is caller-side in every system that does it. Second, the ADR's
alternative — the service decides which fetch is current and simply does not send the older
answer — has real precedent in Flutter's `FutureBuilder` and Qt's `QFutureWatcher`, both of which
put the guard in an adapter object rather than at the call site. **The ADR's positive
recommendation is well-supported; its negative claim about what a framework answer would require
is not.**

**3. The ADR's supporting anecdote deserves a check it has not had.** ADR 0015 cites *"a real
Bubble Tea codebase … about twenty generation counters and request tokens scattered through its
UI, one of them with a four-part condition."* Nothing in this survey can confirm or deny that,
and it is uncited in the ADR. What the survey *can* say is that the pattern appears in
`charmbracelet/bubbles` itself in three components, that its guards are two-part rather than
four-part, and that one of the three (`timer`) has been dead code since at least v0.21.0 because
`m.tag++` was never written. That is a *better* argument for the ADR's conclusion than the
anecdote — hand-rolled generation counters are not just numerous, they are silently wrong — and
it is one anyone can check.

**4. One question the ADR does not raise.** Every "sender checks" system in the sweep depends on
the address never being reused, and the one system where it *is* reused (Win32) had to add a
uniquifier to cover for it. froth's `CONTEXT.md` says a handle *"outlives the surface it names,
so delivery to a dead surface is a defined drop rather than an error."* Whether a handle value
can ever name a *later* surface decides whether `PostFunc` returning `true` is trustworthy. That
is a fact about froth's registry, not a design question this survey should answer — but it is
load-bearing for decision four and is not written down anywhere the survey could find.

---

*Every claim above is read from the pinned revisions in the table or quoted from official
documentation. Items marked `UNVERIFIED` inline — Win32's `ERROR_INVALID_WINDOW_HANDLE` on a
destroyed HWND, the fate of already-queued Win32 messages after `DestroyWindow`, Flutter's
release-build error text, SwiftUI's historical `.task` wording and its behavior when a surviving
task writes `@State` for a removed view, and Dart's official position on uncancellable Futures —
should be re-fetched before any of them enters an ADR. Raymond Chen's blog is flagged at each
use as semi-official rather than normative.*
