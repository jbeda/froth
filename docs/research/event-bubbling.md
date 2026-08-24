# Event bubbling and the "was it handled" signal

Gathered to inform whether froth's own six actions should be serviced by falling out of
"nobody handled it as it bubbled to the root" rather than by the hardcoded switch in
`service()` that intercepts them after resolution and before delivery.

**Core finding up front.** Nine systems, and the useful split is not bubbling-vs-not. It is
**where the "handled" answer lives** and **whether declining is separable from disabling**.
Two systems answer froth's exact question — *may a node declare a binding and let it pass?* —
and they answer it in opposite directions: **WPF and Textual let a declined binding fall
through to the next node up; Flutter and Qt make a declared-but-disabled binding a hard
stop.** The systems that made their own defaults ordinary outermost bindings (Flutter,
Textual, GTK4) all still kept one or two out-of-band escape hatches, which is the most
directly load-bearing fact for the decision this feeds.

Terms below are froth's: *surface*, *handle*, *Post*, *Deliver/Apply/Render*.

---

## 1. DOM / browser events

**Path.** Strictly the ancestor chain, built by the dispatch algorithm — not a focus chain.
The algorithm walks upward via "get the parent" from the target, then plays the path *in
reverse* for capture and forward for bubble
([DOM §2.9](https://dom.spec.whatwg.org/#events)). Focus and the parent chain are separated
cleanly: the UI Events spec says *"The event target of a key event is the currently focused
element which is processing the keyboard activity"*
([UI Events §3.5.5](https://w3c.github.io/uievents/#events-keyboard-event-order)), and from
that target the walk is ordinary ancestry. **Focus picks the entry node; the parent chain
does the walking.** Every system in this survey that has both does exactly this.

**Three flags, not one, and the spec is explicit about why they are three.**

- `stopPropagation()` — *"set this's stop propagation flag"*, checked between nodes.
  MDN: it *"does not, however, prevent any default behaviors from occurring; for instance,
  clicks on links are still processed"*
  ([MDN](https://developer.mozilla.org/en-US/docs/Web/API/Event/stopPropagation)).
- `stopImmediatePropagation()` — sets *both* the stop propagation and stop immediate
  propagation flags; the latter is checked in the inner **per-listener** loop, so it also
  kills sibling listeners on the current node.
- `preventDefault()` — sets the **canceled flag**, orthogonal to both, and only *"if event's
  cancelable attribute value is true"*.

The three answer three genuinely independent questions: should ancestors see this; should
other listeners *on this same node* see it; should the browser's own behaviour still run. A
single "handled" bit cannot express "stop bubbling but keep the link navigation", which is a
routine need. Note the middle one exists only because a node can carry many
independently-authored listeners — a problem froth does not have, since a surface is one
`Update`.

**Unhandled.** Silence. No error, no diagnostic. `dispatchEvent` returns *"false if event's
canceled flag is set; otherwise true"* — it reports cancellation, not handling. The caller
cannot learn whether anyone listened.

**Framework at the tail: a bolted-on special case.** After both phases finish, dispatch runs
one more step — *"if event's canceled flag is unset, then run activationTarget's activation
behavior"* — a distinct algorithm slot on the target, not a listener. There is even a
second, `legacy-canceled-activation behavior`, for what happens when you *did* cancel, which
the spec notes exists only for checkbox and radio inputs. The browser's own handling is
neither a listener nor at the tail of the chain in any structural sense; it is a hook the
dispatch algorithm calls afterward.

**Retargeting** is why the path is not simply parentNode-until-null. `composedPath()` returns
the invocation targets *"except for any nodes in shadow trees of which the shadow root's mode
is 'closed'"* ([DOM](https://dom.spec.whatwg.org/#dom-event-composedpath)), and retargeting is
invoked per path item so a listener outside a shadow tree sees the host rather than the real
origin. The lesson that transfers is narrow: **a path may need to lie about the target
depending on who is looking**, which matters if froth ever encapsulates a subtree.

*[UNVERIFIED: no primary-source statement of regret about the capture phase was found.
whatwg/dom issues #685, #746, #888, #916 were checked and are conformance bugs, not
retrospectives. The "capture is rarely used" characterisation is secondary-source consensus.
The exact `retarget` algorithm text could not be retrieved — fetch truncation.]*

---

## 2. Win32

**Path: there is none.** Win32 has no bubbling primitive. `WM_COMMAND` and `WM_NOTIFY` are
each a **single hop to the parent**. `WM_NOTIFY`: *"The destination of the message must be the
HWND of the parent of the control"*
([Microsoft Learn](https://learn.microsoft.com/en-us/windows/win32/controls/wm-notify)).
Anything beyond one hop is application-built — MFC's command routing is exactly that layer
bolted on top.

**Handled is an LRESULT, and the convention is per-message.** `WM_COMMAND`: *"If an
application processes this message, it should return zero."* `WM_NOTIFY`: *"The return value
is ignored except for notification messages that specify otherwise"* — and inside a dialog
proc you must set the result via `SetWindowLong(DWL_MSGRESULT)` instead, because the dialog
proc's own return is a different BOOL. `WM_NCCREATE` uses TRUE/FALSE; the structurally
adjacent `WM_CREATE` uses 0/-1. The cost of encoding "handled" in an untyped return is that
correctness is per-message documentation discipline, not a learnable rule.

**`DefWindowProc` is the tail, and it is a special case an app must remember to call.**
*"provide[s] default processing for any window messages that an application does not process,
ensuring that every message is processed"*. Nothing enforces the call. Raymond Chen's post
makes the failure concrete: returning zero unconditionally for unrecognised messages is not
neutral — *"returning zero in response to the WM_QUERYENDSESSION message means, 'No, don't
shut down'"*, which silently blocked system shutdown
([Old New Thing, 2006-04-25](https://devblogs.microsoft.com/oldnewthing/20060425-16/)). There
are in fact two stacked tails: `DefSubclassProc` ends the subclass chain into the real
wndproc, which must then separately call `DefWindowProc`.

**Accelerators are a separate pre-dispatch pass — the closest structural match to froth
today.** *"The TranslateAccelerator function examines each window message, looking for
key-down messages… When TranslateAccelerator successfully translates a key stroke, the
function returns a non-zero value, which means you should skip the normal processing for the
message. Otherwise… pass the window message to TranslateMessage and DispatchMessage, as
normal"*
([Accelerator Tables](https://learn.microsoft.com/en-us/windows/win32/learnwin32/accelerator-tables)).
It runs in the message loop *ahead of* dispatch and is exclusive: either the keystroke becomes
a `WM_COMMAND`, or ordinary key delivery proceeds. Never both. This is froth's ADR 0008
timing, arrived at independently.

**Declare without handling.** Two: an accelerator maps a key to a command ID with no
guarantee anything handles it (unmatched IDs fall to `DefWindowProc` silently); and MFC's
`ON_UPDATE_COMMAND_UI`, which lets a class control a command's enabled/checked/text state via
a `CCmdUI` **without** being the object that handles `ON_COMMAND` for that ID
([TN021](https://learn.microsoft.com/en-us/cpp/mfc/tn021-command-and-message-routing)). The
UI-state declaration and the execution can live in different objects in the routing chain.
That is a real separation of "what does help say about this binding" from "who runs it".

*[MODERATELY VERIFIED: the WM_NCCREATE/WM_CREATE contrast came via search summary of
Microsoft's docs rather than a direct side-by-side fetch.]*

---

## 3. Cocoa / AppKit — the responder chain

The closest thing in the survey to what froth is considering, and the most instructive about
what a dynamic language buys you.

**Path.** *"a linked series of responder objects to which an event or action message is
applied. When a given responder object doesn't handle a particular message, the object passes
the message to its successor"*
([Event Architecture](https://developer.apple.com/library/archive/documentation/Cocoa/Conceptual/EventOverview/EventArchitecture/EventArchitecture.html)).
Same two-chain split as everyone: *"The default responder chain for a key event message begins
with the first responder in a window; the default responder chain for a mouse or tablet event
begins with the view on which the user event occurred."* One link does it: *"A view's next
responder is always its superview."*

**Action messages get a longer chain than event messages** — *"They are not restricted to a
single window, as are event messages"* — running first responder → superviews → window →
window controller → window delegate → document → `NSApp` → app delegate → document
controller. Note it contains objects that are not `NSResponder`s at all.

**How you say "handled": by implementing the method and not calling `super`.** *"The default
implementation simply passes this message to the next responder"*
([keyDown](https://developer.apple.com/documentation/appkit/nsresponder/keydown(with:))).
Propagation *is* the base-class implementation. Dropping an event on the floor and stopping
the chain are indistinguishable from the framework's side.

**Unhandled beeps.** *"If no object is found to handle the event, the last responder in the
chain invokes `noResponderFor:`, which for a key-down event simply beeps."* The reference
confirms: *"Handles the case where an event or action message falls off the end of the
responder chain… The default implementation beeps if `eventSelector` is `keyDown(with:)`."*
It is overridable. Mouse events fall off silently. **This is the only system in the survey
where unhandled is a user-visible signal**, and it is worth noting it is scoped to key-down
and is an overridable method at the tail, not an error.

**Declare without handling — three mechanisms, and the wording is the money quote.**

The key-binding dictionary is a global key→selector table, structurally froth's key→action
table. `interpretKeyEvents:` consults it and sends `doCommandBySelector:` back. Then:

> *"The default (`NSResponder`) implementation of `doCommandBySelector:` determines if the view
> responds to the keyboard-action selector and, if the view does respond, it invokes the
> action method; if the view doesn't respond, `doCommandBySelector:` is sent to the next
> responder (and so on up the responder chain). **Therefore, a view should only implement the
> action methods corresponding to the actions that it wants to handle.**"
> ([Handling Key Events](https://developer.apple.com/library/archive/documentation/Cocoa/Conceptual/EventOverview/HandlingKeyEvents/HandlingKeyEvents.html))

The binding table is central and shared; participation is declared by which selectors a node
implements. A node that doesn't implement `moveWordForward:` is transparent to it. That is
"bind the key centrally, let the action find its own owner" — froth's shape, with the
crucial difference noted below.

The table lives in `StandardKeyBinding.dict` (user-overridable via
`~/Library/KeyBindings/DefaultKeyBinding.dict`, which *"augment[s] or replace[s] the standard
bindings"*), maps key strings to selector strings, and supports **multi-keystroke sequences as
nested dictionaries** — an Escape entry can be a whole dictionary of what may follow it.
Relevant to froth's parked chord support. Cost: *"you must save the file and relaunch the
application for the bindings to take effect."*

Menu validation is the `CanExecute` analogue: when a menu item's target is nil, *"`NSMenu`
uses the responder chain to determine the target. If there is no object in the responder chain
that implements the item's action, the item is disabled"*, and if one does, its
`validateMenuItem:`/`validateUserInterfaceItem:` return decides
([Enabling Menu Items](https://developer.apple.com/library/archive/documentation/Cocoa/Conceptual/MenuList/Articles/EnablingMenuItems.html)).
`NSValidatedUserInterfaceItem` states the intent: enable or disable *"depending on whether any
responder in the responder chain can handle the control's action method."*

**But the decline does not forward.** Apple's own guidance for an action you are not
interested in is *"Return a default value of `YES`… even if you don't implement the action"* —
returning `NO` **disables the item outright**; it does not resume the chain walk looking for
another responder that might want it. The walk already ended at `respondsToSelector:`.
AppKit's validation is "the found node may decline the item", not "may decline and forward".
WPF is strictly more expressive here.

`supplementalTargetForAction:sender:` extends *who responds* (letting a non-chain object
claim an action) but fires only after `respondsToSelector:` says no. It does not let a
responder decline something it does respond to.

**Framework at the tail: yes, and by the same mechanism.** `NSApp` and its delegate are
ordinary chain members with the same `nextResponder` link and the same dispatch;
`NSApplication` is itself an `NSResponder`, so `terminate:` and `hide:` are found exactly as
application handlers are. The only genuine special case is what happens *after* the tail:
`noResponderFor:`.

**What it costs, honestly.** The whole thing rests on `respondsToSelector:` — a runtime query
of whether an arbitrary object implements an arbitrary named method. Go has no equivalent.
Three consequences: the chain is heterogeneous and untyped (`sendAction:to:from:` walks `id`,
and Apple warns *"You should never send an action message directly to a responder object of an
unknown class"*); declining is by **omission**, which cannot be enforced, tested at compile
time, or introspected — you cannot ask a responder what it handles except selector by
selector; and validation re-walks the chain on **every user event**, viable only because ObjC
method lookup is cached and menus are small.

froth already has the static answer to the first two: `Bindings()` is an explicit declaration
list, so "what does this surface handle" is a value, not a probe.

---

## 4. WPF / .NET — routed events and routed commands

**Path.** The visual/logical element tree. Focus picks the entry node only. Three
`RoutingStrategy` values: Bubbling (*"Most routed events use the bubbling routing strategy"*),
Tunneling (*"WPF input events are generally implemented as preview and bubbling pairs"*), and
Direct.

**Handled is a mutable bool on shared event args — and it does not stop propagation.**

> *"A routed event that's marked as handled will continue along its route, but only handlers
> that are configured to respond to handled events are invoked. Basically, marking a routed
> event as handled limits its visibility to listeners along the event route."*
> ([Marking routed events as handled](https://learn.microsoft.com/en-us/dotnet/desktop/wpf/events/marking-routed-events-as-handled-and-class-handling))

The docs call it *"a **simple protocol**"* and are explicit that it is a convention, not a
mechanism: *"How you use this protocol is up to you."* This forced the invention of
`AddHandler(…, handledEventsToo: true)`, described as *"available for the rare cases when it's
necessary to respond to handled events"* — an admission that "handled" is really "handled,
unless an ancestor specifically insists otherwise." And the flag is mutable in both
directions, legitimately: *"In very rare circumstances it is appropriate to handle events
where `Handled` is marked `true`, and modify the event arguments by changing `Handled` to
`false`"*, cited for `KeyDown` vs `TextInput` competing over the same key. A shared mutable
flag handlers can un-set is state, not a signal.

**Unhandled: silence.** No error, no diagnostic, no tail handler. *[UNVERIFIED as an
affirmative statement — established by source inspection and by absence in the docs.]*

**The tunnel direction, and its documented cost.** Preview and bubble **share one event-args
instance**, so *"if a preview routed event is marked as handled then its paired bubbling event
will also be handled"*. The docs carry unusually strong warnings: *"Be cautious when marking
preview events as handled… Marking a preview event as handled on an element other than the
element that raised it can prevent the element that raised it from handling the event"*
([Preview events](https://learn.microsoft.com/en-us/dotnet/desktop/wpf/events/preview-events)).
There is also a wart: *"Not all preview events are tunneling events"* — `PreviewMouseLeftButtonDown`
travels downward but is a **Direct** event reraised per element. The strongest tell that the
design has a cost is that the docs ship a section titled *"Working around event suppression by
controls"*: a documented workaround for the framework's own composition behaviour. No explicit
regret, but no other system in this survey needed a workarounds section.

**Declare without handling — the cleanest answer in the survey, and it is in the source, not
the docs.** From
[`CommandBinding.cs`](https://github.com/dotnet/wpf/blob/main/src/Microsoft.DotNet.Wpf/src/PresentationCore/System/Windows/Input/Command/CommandBinding.cs):

```csharp
CanExecute(sender, e);
if (e.CanExecute)
{
    e.Handled = true;
}
```

`e.Handled = true` is set **only inside** `if (e.CanExecute)`. A `CommandBinding` that matches
the command but whose `CanExecute` sets `e.CanExecute = false` leaves `Handled` false, so the
`CanExecute` event keeps bubbling to the next ancestor with a binding for the same command. **A
binding that declines is transparent.** No extra API; it falls out of a rule about when the
handled flag gets set. Contrast AppKit, where `NO` ends the question.

**WPF separates two declines that everyone else conflates.** `ContinueRouting` is the second.
Docs say only *"Determines whether the input routed event that invoked the command should
continue to route through the element tree… The default value is `false`."* The semantics are
in `CommandManager.TranslateInput`:

```csharp
// If we mapped an input event to a command, we should always
// handle the input event - regardless of whether the command
// was executed or not.  Unless the CanExecute handler told us
// to continue the route.
inputEventArgs.Handled = !continueRouting;
```

| | question asked | mechanism | default |
|---|---|---|---|
| `CanExecute = false` | should *this node* run it, or another node's binding? | `Handled` left false, CanExecute keeps bubbling | pass it on |
| `ContinueRouting = true` | nobody ran it — should the **keystroke** survive? | sets `inputEventArgs.Handled = false` | **swallow the key** |

A matched `KeyBinding` eats the key even when the command is disabled, unless a handler says
otherwise. Both defaults live in source comments, not documentation.

**`ApplicationCommands.NotACommand`** is a third form — a sentinel identity whose only purpose
is unbinding. Its source remark: *"This 'command' is always ignored, without handling the
input event that caused it. This provides a way to turn off an input binding built into an
existing control."* Direct prior art for turning off a stock binding without a removal API.

**Stock command identities with stock gestures**, exactly parallel to froth's stock bindings:
*"These classes consist only of the `RoutedCommand` objects and **not** the implementation
logic of the command"*, and *"if you specify that your application handles the copy command,
you automatically get the keyboard binding 'CTRL+C'."* Source: `Ctrl+C;Ctrl+Insert`. **A
command is an identity, not a handler** — froth's Action, arrived at independently.

**Framework position: the HEAD, at every node, by a different mechanism.** This is the
sharpest contrast with AppKit and the most consequential fact here. *"Class handlers for a
given class are invoked before any instance handler responding to the same event on any
instance of that class."* `ButtonBase` overrides `OnMouseLeftButtonDown`, sets `Handled = true`
and raises `Click` — *"The end result for most listeners is that the `MouseLeftButtonDown` and
`MouseRightButtonDown` events are hidden."* The motivation is stated as pre-emption: *"your
class handler can enforce event-driven class behaviors that can't be accidentally suppressed
by an instance handler."* And the consequence is acknowledged: *"Class handling might give the
outward appearance that the routed event isn't ever raised."* Registration is
`EventManager.RegisterClassHandler` or a virtual override — **not** the mechanism application
handlers use.

---

## 5. Qt

**Path.** Parent widget chain, entered at the focus widget for keys and the widget under the
cursor for mouse. *"Calling ignore() on a key event will propagate it to the parent widget. The
event is propagated up the parent widget chain until a widget accepts it or an event filter
consumes it."* Qt's `setTabOrder`/`nextInFocusChain` focus chain exists but is for *moving*
focus, not delivery.

**Two boolean signals that look alike and are not.** `QObject::event()` returns bool — *"should
return true if the event e was recognized and processed"*, and *"the return value indicates
whether an event was dealt with; a true value prevents the event from being sent on to other
objects"*
([The Event System](https://doc.qt.io/qt-6/eventsandfilters.html)). Separately
`QEvent::accept()`/`ignore()` set a flag on the event object, and *that* is what the
parent-chain propagation reads. The two govern different layers — object dispatch versus
widget propagation — and this is a known confusion source. Also: *"By default, isAccepted() is
set to true, but don't rely on this as subclasses may choose to clear it in their
constructor."*

**Unhandled: silence.** The event walks to the top of the chain and is dropped.

**Default handler at the tail, reached by superclass call.** *"If you only want to extend the
base class's functionality, then you implement what you want and call the base class to obtain
the default behavior for any cases you do not want to handle."* Calling
`QWidget::keyPressEvent(event)` is how you decline — the base implementation typically calls
`ignore()`. **This is an OO-inheritance mechanism, not a chain position.** froth has no
equivalent and should not try to grow one.

**Event filters are the tunnel.** *"An event filter gets to process events before the target
object does"*, and *"If all the event filters allow further processing of an event (by each
returning false), the event is sent to the target object itself. If one of them stops
processing (by returning true), the target and any later event filters do not get to see the
event at all."* App-wide filters run before per-object ones. `QCoreApplication::notify` sits
above all of it, and Qt is steering away from it: *"This function will not be called for
objects that live outside the main thread in Qt 7."*

**Declare without handling: `QEvent::ShortcutOverride`**, a genuine pre-pass claim. *"This flag
is set by default for QEvent::KeyPress and QEvent::KeyRelease, so there is no need to call
accept() when acting on a key event. **For QEvent::ShortcutOverride the receiver needs to
explicitly accept the event to trigger the override.**"* A widget accepting it makes Qt's
shortcut system back off and deliver an ordinary key press instead. Note the direction: this
is a *descendant pre-empting a shortcut*, not a node declining and forwarding. It is the
mirror of what froth is asking about — closer to Textual's `check_consume_key` than to WPF's
`CanExecute`.

**A disabled `QAction`'s shortcut is dead, not forwarded.** *"When an action is disabled, it is
not possible to trigger it through its shortcut."* No evidence of fall-through.

**Ambiguity is a first-class outcome.** When two shortcuts match, `QShortcutEvent::isAmbiguous()`
is true and `activatedAmbiguously()` fires instead: *"The activated() signal is not emitted in
this case."* Qt refuses to pick. froth picks statically by depth, which is a defensible
different answer, but note that a system with a flat shortcut namespace found it needed to
report the collision rather than resolve it.

*[UNVERIFIED: exact verbatim text for `Qt::WidgetShortcut` and `Qt::ApplicationShortcut`
(the enum page rendered without per-value descriptions); a reported Qt 5.11.0 regression where
disabled QActions still triggered is forum-sourced, not changelog-confirmed.]*

---

## 6. GTK 4

**Path: an explicit three-phase model over the widget tree**
([input-handling](https://docs.gtk.org/gtk4/input-handling.html)). Capture: *"The event is
delivered to each widget from the top-most… down to the target widget."* Target. Bubble: *"from
the target to the top-most."* `GtkPropagationLimit` additionally scopes which widgets
participate (`GTK_LIMIT_SAME_NATIVE`).

**Shortcuts are an event controller in the same propagation system** — the notable
architectural choice here. *"GTK handles accelerators and mnemonics in a global scope, during
the capture phase, and key bindings locally, during the target phase."* Global shortcuts get
capture *deliberately*, so they can pre-empt; local bindings ride the ordinary path.
`GtkShortcutScope` names the three: LOCAL (*"handled inside the widget the controller belongs
to"*), MANAGED (*"handled by the first ancestor that is a GtkShortcutManager"*), GLOBAL
(*"handled by the root widget"*).

**Handled is a return value — except where it isn't.** `GtkEventControllerKey::key-pressed`
returns *"TRUE if the key press was handled, FALSE otherwise."* But `GtkGestureClick::pressed`
is `void`; gestures instead claim the event sequence out of band
(`GTK_EVENT_SEQUENCE_CLAIMED`): *"A controller may indicate that a received event was consumed
and propagation should therefore be stopped. If gestures are used, this may happen when the
gesture claims the event touch sequence."* Two conventions, split by controller type rather
than by API surface.

**Declare without handling: yes, and the framework does the falling-through.** This required
reading source; the docs for `gtk_widget_action_set_enabled` do not state it.
`gtk_named_action_activate` (`gtk/gtkshortcutaction.c`) queries the muxer's enabled state and
returns `FALSE` if disabled. That `FALSE` reaches `gtk_shortcut_controller_run_controllers`
(`gtk/gtkshortcutcontroller.c`), whose loop only sets `retval = TRUE` on successful
activation — so it **continues to other matched shortcuts**, and if none succeed
`gtk_shortcut_controller_handle_event` returns `FALSE` and ordinary capture/target/bubble
propagation continues to other controllers and widgets, as if this controller had never
claimed the event. Meanwhile `gtk/gtkactionmuxer.c` shows a disabled action is still *found*
by lookup rather than skipped — so the muxer does not keep searching ancestors for a
same-named enabled action. **The fall-through happens at the shortcut layer, not the action
lookup layer.** Same split as Flutter's, resolved the other way.

`gtk_widget_activate_action` returns gboolean, *"True if the action was activated"*, and emits
no warning when the action is absent or disabled.

**Framework in the same mechanism.** `GtkShortcutController` *is* an ordinary
`GtkEventController`; `gtk_widget_class_add_binding_action` is how a widget class installs its
own defaults using the identical `GtkShortcut`/trigger/action vocabulary application code
uses. **This is the strongest unification in the survey** — stronger than AppKit's, which
unifies dispatch but reaches defaults through inheritance, and far stronger than WPF's, which
uses a separate registration API.

---

## 7. Flutter

Worth reading closely because it uses froth's word *Intent* and because it has **three
unrelated propagation systems with three different conventions**. Notifications bubble the
widget tree with a bool. Key events walk the focus chain with a three-valued enum. Pointer
events walk a hit-test path with *no* handled signal at all, disambiguated later by the
gesture arena (*"The first member to accept or the last member to not reject wins"*). They
share no machinery.

**Notification — the one bool bubble.** From `notification_listener.dart`: *"Return true to
cancel the notification bubbling. Return false to allow the notification to continue to be
dispatched to further ancestors."* Path is the parent chain, not focus. Typed dispatch:
a listener whose type parameter does not match returns `false` and is transparent. `dispatch`
returns `void` — **the sender cannot learn whether anyone listened**, and returning `false` is
indistinguishable from "I ignored it". If nobody returns true: silence.

**Shortcuts → Intent → Actions.** `Shortcuts` maps activator → Intent; `Actions` maps Intent
*type* → Action; lookup walks up the widget tree from the focused context
(`_visitActionsAncestors` in `actions.dart`). Docs: *"If the given `intent` doesn't map to an
action, then it will look to the next ancestor [Actions] widget in the hierarchy until it
reaches the root."*

**The decisive question, and Flutter answers it NO.** `Actions.invoke`:

```dart
final Action<Intent>? result = _getActionForIntent(actions, intent);
if (result != null && result._isEnabled(intent, context)) {
  returnValue = _findDispatcher(element).invokeAction(result, intent, context);
}
return result != null;          // stops on MAPPED, not on ENABLED
```

`maybeInvoke`'s own doc says it plainly: *"If a suitable [Action] is found but its
[Action.isEnabled] returns false, the search will stop and this method will return null."*
**A disabled Action shadows every ancestor Action for that Intent type.**

But this yields a **two-level fallthrough** worth naming precisely:

- **Key → Intent** *does* fall through up the focus chain, per `Shortcuts` widget.
- **Intent → Action** does *not*. First mapping wins, enabled or not.

The escape hatch is not a resumed walk but explicit delegation: `Action.overridable(defaultAction:, context:)`
— *"allows itself to be overridden by the closest ancestor [Action], and falls back to its own
`defaultAction` when no overrides can be found"*, with the override typically writing
`bool get isActionEnabled => callingAction?.isActionEnabled ?? false;`. Chaining is hand-wired
per Action, and the framework wires its own (`WidgetsApp` wraps `ScrollAction` this way).
**That is the cost of "first mapping wins".**

**`consumesKey` — invoke without claiming.** *"Indicates whether this action should treat key
events mapped to this action as being 'handled'… If the key event is not handled, it will be
passed back to the engine, and continue to be processed there, allowing text fields and
non-Flutter widgets to receive the key event."* Read carefully: `consumesKey == false` does
**not** mean keep bubbling in Flutter — `toKeyEventResult` maps it to `skipRemainingHandlers`.

**Why a third state was needed:** two downstreams, not one.

```dart
enum KeyEventResult { handled, ignored, skipRemainingHandlers }
```

`ignored` = *"continue to be propagated to other key event handlers, even non-Flutter ones"*;
`skipRemainingHandlers` = *"not been handled, but the key event should not be propagated to
other key event handlers. It will be returned to the platform embedding."* Three of the four
points in a 2×2 (Flutter's chain × the platform IME); the fourth is unrepresentable and
apparently unwanted. **A bool can only encode one axis.**

**No unbind primitive.** You shadow an ancestor binding by binding the key to
`DoNothingAndStopPropagationIntent` — *"one way to disable a keyboard shortcut defined by a
widget higher in the widget hierarchy"* — versus plain `DoNothingIntent`, *"a similar intent
that will handle the key event."*

**Framework defaults ARE ordinary outermost widgets — verified in `app.dart`:**

```dart
child: Shortcuts(
  debugLabel: '<Default WidgetsApp Shortcuts>',
  shortcuts: widget.shortcuts ?? WidgetsApp.defaultShortcuts,
  // DefaultTextEditingShortcuts is nested inside Shortcuts so that it can
  // fall through to the defaultShortcuts.
  child: DefaultTextEditingShortcuts(
    child: Actions(
      actions: widget.actions ?? <Type, Action<Intent>>{ ...WidgetsApp.defaultActions, ... },
```

No privileged path. That inline comment is the framework layering *its own* defaults against
itself using nothing but tree depth. And `defaultShortcuts` includes
`Tab → NextFocusIntent()`, `Shift+Tab → PreviousFocusIntent()`, and the four arrows →
`DirectionalFocusIntent(...)`, resolved by `defaultActions`'s `NextFocusAction()`,
`PreviousFocusAction()`, `DirectionalFocusAction()`. **Nine built-in Intent/Action pairs
including focus traversal, shadowable by any descendant, with no special status — the direct
parallel to froth's six.**

**But there is still an out-of-band pair.** `FocusManager.addEarlyKeyEventHandler` — *"called
before any key event handlers in the focus tree"* — and `addLateKeyEventHandler` — *"called if
none of the key event handlers in the focus tree handle the event"*. A separate registry, not
`Shortcuts`/`Actions`. "Same mechanism, just outermost" was necessary but not sufficient.

**Unhandled:** silence in release; an assert-guarded `debugPrint` trace behind
`debugFocusChanges` in debug.

*[UNVERIFIED: no primary source expressing regret for any of these designs was found. The one
self-acknowledged wart is an in-code `ErrorHint` on `Actions.maybeFind` pointing at
flutter/flutter#180871 — a generics-variance problem, not a propagation one.]*

---

## 8. Textual — the TUI-native case

**Message bubbling** walks the DOM parent chain, gated by a class var (`bubble: ClassVar[bool]`).
The whole engine is a few lines of `MessagePump._on_message`, including an anti-echo rule
worth noting: *if the message's sender is the parent, stop after the parent.*

**`stop()` and `prevent_default()` exist and do NOT mean what the DOM means.** Source:
`prevent_default` sets `_no_default_action`, which breaks out of the **Python MRO loop** on the
current node — it suppresses base-class `_on_*` implementations, i.e. the framework's own
per-widget default behaviour. `stop()` sets `_stop_propagation`, the tree-direction control.
And `_bubble_to` **resets** `_no_default_action = False` on every hop. So the two are
orthogonal axes — MRO depth versus tree depth — that happen to be named after the DOM's pair.
Textual borrowed the names and not the semantics.

**Handled is a mutable field set out of band.** Handlers return `None`. The one exception is
inconsistent: key *methods* use a return value, and `handled = (await invoke(...)) is not False`
— returning `None` counts as handled. Textual's docs discourage them: *"Consider key methods
to be a convenience for experimenting with Textual features. In nearly all cases, key bindings
and actions are preferable."*

**Bindings resolve along the focus chain, bottom-up.** *"Textual will first check for a
matching binding in the `BINDINGS` list of the currently focused widget. If no match is found,
it will search upwards through the DOM all the way up to the `App`."* `Screen._binding_chain`
is `[(node, node._bindings) for node in focused.ancestors_with_self]`, and
`_modal_binding_chain` **truncates the chain at the first `node.is_modal`** — Textual's
modality is chain truncation, exactly froth's ADR 0008 amendment, arrived at independently.

**There is a tunnel pass, and it cost four lines.** `_check_bindings` iterates
`reversed(self.screen._binding_chain) if priority else self.screen._modal_binding_chain`, and
`_process_messages` runs the priority pass first. So: (1) priority bindings, root→leaf; (2) the
key delivered to the focused widget, bubbling leaf→root as an ordinary message. Textual did not
build a general capture phase; it needed a root-first pass, and got it with a per-binding
`priority` flag and one `reversed()`. **That is the cheapest tunnel in the survey, and it is
scoped to bindings rather than to all events.**

**`check_action` — declare a binding, decline it, and the key falls through. Confirmed.**

```python
def check_action(self, action: str, parameters: tuple[object, ...]) -> bool | None:
    """Returns:
        `True` if the action is enabled+visible,
        `False` if the action is disabled+hidden,
        `None` if the action is disabled+visible (grayed out in footer)
    """
```

`run_action` returns `False` when `check_action` is falsey; `_check_bindings` only early-returns
on `if await self.run_action(...): return True`, so **the loop continues to the next binding for
that key and then to the next namespace up the chain.** A Textual widget can bind a key,
declare it currently unavailable, and let an ancestor's binding for the same key win.

**Two things to flag.** First: the docs never say this. `/guide/actions/` says only "prevent the
action running". It is a source-read fact, not a documented guarantee. Second: **Textual spends
its third state on the help dimension, not on propagation** — `False` hides the key from the
footer, `None` greys it. froth's `Binding.Disabled` currently occupies the `None` slot
(reported to help with keys intact, does not stop the key resolving further up); Textual found
it needed the `False` slot too.

**A second, opposite hook: `check_consume_key`.** *"Check if the widget may consume the given
key… Implementing this method will hide key bindings from the footer and key panel that would
be *consumed* by the focused widget."* Applied in `_binding_chain` with the comment *"Filter out
bindings that could be captures by widgets (such as Input, TextArea)"* — each descendant, in
leaf-to-root order, **deletes from every ancestor's binding map** any key it will swallow. This
is Qt's `ShortcutOverride` in another dress, and it is froth's text-sink rule generalised: a
text input has no binding for `a`, it just eats it. Textual needed both hooks because
`check_action` answers "is *my binding* live?" while `check_consume_key` answers "will my raw
`on_key` eat this before your binding ever runs?"

**Help and dispatch share one source of truth.** `Screen.active_bindings` walks
`self._modal_binding_chain` — the same chain the dispatcher walks — calling `check_action` per
binding and dropping `False` entries while keeping `None` ones with `enabled=False`. Same
guarantee froth's `liveBindings()` gives. Refresh is push-based (`refresh_bindings()`), or
automatic via `bindings=True` on a reactive.

**Framework defaults are ordinary bindings at the outermost namespaces.**
`Screen.BINDINGS` holds `Binding("tab", "app.focus_next", show=False)` and
`Binding("shift+tab", "app.focus_previous", show=False)`; `App.BINDINGS` holds quit. Any focused
widget can shadow them. **But**, as with Flutter, escape hatches survived: `priority=True`
(join the root-down pass), `system=True`, and one genuinely hardcoded key — `App` special-cases
`escape` for `screen.minimize()` **before** `_check_bindings` runs, guarded by
`escape_to_minimize`. The one framework behaviour that could not be expressed as a binding is a
key that jumps the queue.

**Unhandled key: silence**, plus a devtools-console line at verbose level that is off by
default. Nothing at the tail catches it.

---

## 9. The Go / Bubble Tea ecosystem — an honest negative

**No Go TUI library surveyed implements bubbling with a handled signal.** This is a finding,
not a gap in the search.

- **bubbletea**: `Update(Msg) (Model, Cmd)`. No bool, no enum, no hierarchy. A parent calls a
  child's `Update` and cannot learn whether the child did anything. This is ADR 0008's premise,
  confirmed at the source.
- **tview**: tunnel-only, and the return type is the tell —
  `InputHandler() func(event *tcell.EventKey, setFocus func(p Primitive))`, **no results**. The
  walk goes root → focused leaf, each container forwarding to whichever child `HasFocus()`.
  `SetInputCapture` is a transform-or-swallow filter: *"This function can then choose to
  forward that key event (or a different one) to the default handler by returning it. If nil is
  returned, the default handler will not be called."* The framework's own Ctrl-C sits hardcoded
  in the middle of that pass — *"It requires special handling"* — and the documented way to
  forward Ctrl-C without quitting is to return a **fresh** `EventKey` with the same key, an
  identity-comparison trick that exists purely because there is no handled signal. **Notably,
  tview's mouse path *does* have one**: `MouseHandler() func(...) (consumed bool, capture Primitive)`.
  The author reached for a handled bool exactly where hit-testing made ownership ambiguous, and
  skipped it where focus already answered.
- **charmbracelet/bubbles `key.Binding`**: the ecosystem's closest thing to a declaration with
  help text and an enabled flag — *"Disabled keybindings won't be activated and won't show up
  in help"* — plus a distinct `Unbind()`: *"This is a step beyond disabling it."* No chain;
  `Matches` is a predicate a model calls inside its own `Update`. But note it independently
  found two states insufficient, for the same reason Textual's `check_action` returns three.
- **gocui**: flat registry, no hierarchy, **no early exit** — every matching binding fires and
  `matched` is a summary, not a short-circuit.
- **bubblezone**: hit-testing only, and explicitly broadcast — *"Any time there is a mouse
  event, pass it down to all children"*. No handled signal, nothing stops.

*[UNVERIFIED: teacup was not examined.]*

froth adding chain propagation with a handled signal to a Bubble Tea program has **no Go
precedent**. The design pressure has to come from the systems above, not from the ecosystem it
sits in.

---

## Cross-cutting findings

**1. Focus picks the entry node; the parent chain does the walking. Universally.** DOM, AppKit,
WPF, Qt, Flutter and Textual all resolve the focus-chain-vs-parent-chain tension identically,
and none of them treats it as a real choice. froth's focus chain is already this — the path
from root to active surface derived from one handle — so there is nothing to reconcile.

**2. Everyone who shipped a two-state handled signal later needed a third, and the third state
is always about a *second consumer*.** Flutter's `skipRemainingHandlers` (the platform IME);
Textual's `check_action → None` (the footer); bubbles' `Unbind()` (help text); WPF's
`handledEventsToo` (ancestors who insist); the DOM's three flags (ancestors, sibling listeners,
the browser). The pattern is exact: **a bool encodes one axis, and every one of these systems
turned out to have two.** froth has two consumers already — the chain and the help query — and
they are wired to the same declarations by design. If a handled bool arrives, expect pressure
for the third value from the help side, which is precisely where Textual spent its.

**3. On froth's exact question the systems split, and the split is clean.**

| system | node declares a binding, then declines it | what happens |
|---|---|---|
| **WPF** | `CanExecute = false` | `Handled` left false → **keeps bubbling to the next binding** |
| **Textual** | `check_action` → `False`/`None` | `run_action` returns False → **loop continues up the chain** |
| **GTK4** | disabled action behind a shortcut | activate returns FALSE → **controller keeps matching, then propagation continues** |
| **Flutter** | disabled Action | lookup **stops dead**; ancestors shadowed; chain by hand via `Action.overridable` |
| **Qt** | disabled QAction | shortcut is **inert**; no fall-through |
| **AppKit** | `validateMenuItem:` → NO | item **disabled**; walk already ended; no forwarding |

Three-to-three, but the three that fall through are the three whose lookup happens to be a loop
over declarations, and the three that stop are the ones whose lookup is a *find* that commits at
the first match. **It is a consequence of how the walk is written, not a considered position** —
in WPF and GTK4 it lives in source, unmentioned in the docs; Textual's is undocumented too.
That is worth weighing: nobody argued for either behaviour in public, so the "precedent" is
thinner than the tally suggests.

**4. WPF separates two declines everyone else conflates**, and froth will need them apart:
declining the **action** (someone else's binding should run it) versus declining the **key**
(nobody ran it — should the keystroke survive as raw input?). WPF's defaults differ between
them: pass the action on, but swallow the key. froth's current design answers the second
question implicitly — a matched binding always consumes — and has no way to ask the first.

**5. Framework-defaults-as-outermost-ordinary-bindings is validated, and incomplete.** Flutter
and Textual both do it, both with focus traversal specifically (Tab/Shift-Tab/arrows), both
shadowable with no privileged status. GTK4 goes further and puts its own shortcut handling in
the same controller mechanism. AppKit puts `NSApp` in the chain as an ordinary member. **But
every single one of them kept an out-of-band hatch**: Flutter's early/late `FocusManager`
registry, Textual's `priority=True` pass *and* a hardcoded `escape`, AppKit's `noResponderFor:`
after the tail, GTK4's deliberate use of the capture phase for global accelerators, tview's
hardcoded Ctrl-C. The honest reading is that "the framework is just the outermost binding" is
achievable for the *ordinary* cases and has never been sufficient for **all** of them.

**6. Framework at head or tail is a real fork, and the two have different failure modes.**
AppKit, DOM (sort of), and Win32 put the default handler at the end; WPF puts framework class
handlers at the **head of every node** and documents the consequence — *"Class handling might
give the outward appearance that the routed event isn't ever raised."* GTK4 puts its global
shortcuts in the capture phase, which is head-like, deliberately. Tail-position defaults fail by
being unreachable; head-position defaults fail by being invisible.

**7. "Unbind" is not a primitive anywhere.** Flutter binds the key to
`DoNothingAndStopPropagationIntent`; WPF binds it to `ApplicationCommands.NotACommand`; Textual
uses `check_consume_key` to have a descendant delete ancestors' bindings; bubbles has `Unbind()`
but no chain. Three of these are sentinel values standing in for a missing operation. froth's
`Disabled` flag on a declaration of a froth-owned action already does this properly — a
declaration takes the action over, and a disabled one leaves it with no keys — which is a
cleaner answer than any of them.

**8. Unhandled is silence almost everywhere.** DOM, WPF, Qt, GTK4, Flutter (release), Textual,
Win32-after-`DefWindowProc`. The one exception is AppKit's beep, scoped to key-down, delivered
by an overridable method at the tail. Nobody treats an unhandled key as an error or a
diagnostic. froth's diagnostic hook already treats an *unclaimed intent* as "nearly always a
missing case" (ADR 0002) — that is a stronger position than any system here takes about keys,
and the survey neither supports nor undermines it, because no system has froth's distinction
between a raw key nobody wanted and a declared action nobody claimed.

**9. A command is an identity, not a handler — twice, independently.** WPF's `RoutedCommand`
classes *"consist only of the `RoutedCommand` objects and not the implementation logic"*;
AppKit's `NSResponder` declares the selector vocabulary that `StandardKeyBinding.dict` targets
while apps implement the subset they want. froth's Action is the same idea. This is settled
prior art, not an open question.

---

## What transfers to froth, and what does not

### Transfers

- **The two-decline distinction (WPF).** Declining an *action* so another surface's binding
  runs it, versus declining a *key* so it continues as raw input, are different questions with
  different sensible defaults. Any froth design here needs both answers explicitly. This is the
  single most useful thing in the survey and it is free of any framework's constraints.

- **Fall-through-on-decline needs no new API when lookup is already a loop (WPF, Textual,
  GTK4).** In all three the behaviour falls out of *"only mark it handled if it actually ran"*.
  froth's `liveBindings()` is already a loop over declarations with a claim step; the question is
  whether a decline arrives before the walk (a declaration is skipped) or after delivery (the
  walk resumes) — and the latter is what ADR 0008's amendment already describes as costing "a
  resumable walk and a defined outcome for an action nobody claims".

- **Framework defaults as ordinary outermost bindings is proven (Flutter, Textual, GTK4), and
  proven insufficient alone.** Flutter's `Tab → NextFocusIntent → NextFocusAction` at the root of
  `WidgetsApp`, and Textual's `Binding("tab", "app.focus_next")` on `Screen`, are froth's six
  actions with a different spelling. But every one of these systems kept an out-of-band hatch,
  so the realistic target is "the ordinary cases fall out of the chain" rather than "the switch
  disappears entirely". Deciding *which* case justifies the remaining hatch is froth's to make,
  not something the survey settles.

- **Help and dispatch reading one source of truth (Textual).** `Screen.active_bindings` walks the
  same `_modal_binding_chain` the dispatcher walks. froth already has this in `liveBindings()`.
  Confirmation, not news — but it is the reason Textual could afford a third `check_action` state
  in the help dimension without the two mechanisms drifting.

- **Modality as chain truncation (Textual).** `_modal_binding_chain` truncates at the first
  `is_modal` node. Identical to ADR 0008's amendment, arrived at independently. Corroboration
  for a decision already taken.

- **The pre-emption hook is a distinct thing from declining (Qt `ShortcutOverride`, Textual
  `check_consume_key`).** Both let a *descendant* pre-emptively claim a key so ancestors'
  bindings never match. froth's text-sink rule is the special case of this that froth chose to
  hardcode. If a general form is ever wanted, these two are the prior art — and note that both
  systems needed it *in addition to* a declining mechanism, not instead of one.

- **A sentinel for "turn off an inherited binding" is what you get when you lack a real
  operation (WPF `NotACommand`, Flutter `DoNothingAndStopPropagationIntent`).** froth's
  `Disabled` declaration is the real operation. Worth knowing this is the road not taken.

- **Multi-keystroke bindings as nested tables (AppKit's `.dict`).** Concrete shape for froth's
  parked chorded sequences: a key's value is either a leaf action or another table.

### Does not transfer

- **AppKit's `respondsToSelector:` dispatch.** The entire responder chain rests on runtime "does
  this object implement this named method". Go has no equivalent, the chain is untyped `id`, and
  declining is by **omission** — uninspectable, untestable, unenforceable. froth's `Bindings()`
  already replaces the probe with a value, which is strictly better; the mechanism AppKit uses to
  reach that answer is not available and is not wanted.

- **Qt's superclass-call default handler.** `QWidget::keyPressEvent(event)` reaching the default
  is OO inheritance, not a chain position. froth has no base class for surfaces and should not
  grow one to get this.

- **WPF's mutable shared `Handled` field.** It required inventing `handledEventsToo`, permits
  handlers to set it back to `false` (documented as legitimate), makes invocation order
  load-bearing across five handler categories and two directions, and needed a docs section on
  working around its own controls. froth's ADR 0002 records structural change during Deliver as
  recorded-then-committed for iterator-invalidation reasons; a decline recorded the same way —
  "like `Focus` and `Raise`", as ADR 0008 already puts it — is a return-shaped signal in a
  recorded coat, and shares none of WPF's mutable-shared-state cost.

- **The DOM's three flags.** `stopImmediatePropagation` exists because a node carries many
  independently-authored listeners. A froth surface is one `Update`. Two of the three DOM
  concepts collapse for froth; only "stop the walk" versus "let the default happen" survives,
  which is finding 4 restated.

- **A general capture/tunnel phase.** DOM has one nobody can point to a primary source
  celebrating; WPF's is documented with repeated "be cautious" warnings, a shared-args gotcha, a
  non-tunneling "preview" wart, and a workarounds article; GTK4 uses it deliberately but only for
  globally-scoped accelerators. Textual is the informative case: it needed a root-first pass, and
  bought it with a per-binding `priority` flag and one `reversed()` — **scoped to bindings, not
  to all events, and about four lines.** If froth ever needs a root-first pass, that is the shape
  with evidence behind it. Nothing here argues for a general second direction.

- **The Go ecosystem.** Nothing to copy. bubbletea has no handled signal; tview tunnels to focus
  with a void return and works around its own hardcoded Ctrl-C with an identity check; gocui is
  flat and fires every match. The one positive signal is small and worth taking seriously
  anyway: tview added a `consumed bool` for *mouse* input specifically, where focus could not
  answer who owned the event.

### The honest caveat on the tally

Six systems were asked whether a node may declare a binding and decline it; three say yes. But
in all three, the yes is an emergent property of how a loop was written, undocumented, and
discovered only by reading source. **No system in this survey argues for fall-through-on-decline
as a design position.** The three that stop dead do not argue for that either. Anyone reaching
for "the precedent supports X" in either direction is overreading the evidence — what the survey
genuinely establishes is the *shape* of the question (findings 2, 4, 5), not its answer.

---

## Sources

Every claim above is from a page or file fetched during this survey. DOM/UI Events:
dom.spec.whatwg.org, w3c.github.io/uievents, MDN. Win32: learn.microsoft.com (wm-command,
wm-notify, DefWindowProc, accelerator-tables, subclassing-overview, TN021), devblogs "Old New
Thing". AppKit: developer.apple.com archived Cocoa guides (Event Architecture, Handling Key
Events, Text Defaults and Key Bindings, Enabling Menu Items) plus the documentation JSON backing
store for NSResponder and the validation protocols. WPF: learn.microsoft.com routed-events and
commanding docs plus `dotnet/wpf` source (CommandManager.cs, CommandBinding.cs, RoutedCommand.cs,
ApplicationCommands.cs, Generated/UIElement.cs). Qt: doc.qt.io Qt 6. GTK4: docs.gtk.org plus
`gitlab.gnome.org/GNOME/gtk` source (gtkshortcutaction.c, gtkactionmuxer.c,
gtkshortcutcontroller.c). Flutter: api.flutter.dev plus `flutter/flutter` master source
(actions.dart, shortcuts.dart, focus_manager.dart, notification_listener.dart, app.dart).
Textual: textual.textualize.io guides plus `Textualize/textual` main source (message.py,
message_pump.py, app.py, screen.py, dom.py, _dispatch_key.py). Go: charmbracelet/bubbletea,
charmbracelet/bubbles, rivo/tview, jroimartin/gocui, lrstanley/bubblezone.

Unverified items are flagged inline at the end of each section.
