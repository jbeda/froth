# Win32 window relations: what is actually a hierarchy, and what isn't

Research note for froth. Every substantive claim below is sourced to a Microsoft Learn page that was fetched and read; see [Sources](#sources). Claims that the docs do not settle are marked **AMBIGUOUS**, **DOC VS REALITY**, or **[UNVERIFIED]**.

**Core finding up front:** Win32 maintains exactly *two* window-to-window tree/graph relations (parent and owner) and *three* pieces of near-global scalar state (active, focus, capture). The parent tree is heavily overloaded — coordinates, clipping, z-order grouping, destruction/visibility cascade, and notification routing all ride on it. The owner relation is a thin second edge set governing only z-order and show/destroy cascade for top-level windows. Focus, activation, and capture are *not* trees at all; they are single HWND values per input queue.

---

## 1. Parent vs Owner

### The two relations are formally distinct and mutually exclusive

The definitive statement is in the `SetWindowLongPtr` remarks:

> **GWLP_HWNDPARENT** is used to change the owner of a top-level window, not the parent of a child window.
>
> A window can have either a parent or an owner, or neither, but never both simultaneously.
>
> — [SetWindowLongPtr](https://learn.microsoft.com/en-us/windows/win32/api/winuser/nf-winuser-setwindowlongptra)

So the answer to "can a window have both?" is **no, by documented design**. `CreateWindowEx`'s single `hWndParent` parameter is interpreted as *parent* if `WS_CHILD` is set and as *owner* if the window is `WS_OVERLAPPED`/`WS_POPUP` ([Window Features → Owned Windows](https://learn.microsoft.com/en-us/windows/win32/winmsg/window-features)). One slot, two meanings, selected by style.

**DOC VS REALITY / doc inconsistency:** the `GetWindowLongPtr` page documents `GWLP_HWNDPARENT` (-8) as *"Retrieves a handle to the parent window, if there is one"* with no mention of owners ([GetWindowLongPtr](https://learn.microsoft.com/en-us/windows/win32/api/winuser/nf-winuser-getwindowlongptra)), while the setter page for the same index says it sets *"a new owner for a top-level window."* The getter description is incomplete; the same storage slot holds whichever relation applies.

### What PARENT governs

From [About Windows](https://learn.microsoft.com/en-us/windows/win32/winmsg/about-windows) and [Window Features](https://learn.microsoft.com/en-us/windows/win32/winmsg/window-features):

- **Coordinate space.** *"The system always positions a child window relative to the upper left corner of its parent window's client area."*
- **Clipping (hard).** *"No part of a child window ever appears outside the borders of its parent window... the portion outside the parent window's client area is not displayed."*
- **Clipping (opt-in, the other direction).** *"The system does not automatically clip a child window from the parent window's client area"* — the parent draws over children unless it has `WS_CLIPCHILDREN`; siblings draw over each other unless `WS_CLIPSIBLINGS`.
- **Lifetime and visibility cascade.** Parent destroyed → child *"Destroyed before the parent window is destroyed"*; parent hidden → child hidden; parent moved → child moved; *"A child window is visible only when the parent window is visible."*
- **Existence requirement.** *"If you specify the WS_CHILD style in CreateWindowEx but do not specify a parent window, the system does not create the window."*
- **Z-order grouping** — see §2.
- **Notification routing** — see §5.
- **Disabled-child input redirection.** *"When a child window is disabled, the system passes the child's mouse input messages to the parent window."*

### What OWNER governs

Exactly three documented constraints, no more ([Window Features → Owned Windows](https://learn.microsoft.com/en-us/windows/win32/winmsg/window-features)):

> - An owned window is always above its owner in the z-order.
> - The system automatically destroys an owned window when its owner is destroyed.
> - An owned window is hidden when its owner is minimized.

Plus these documented facts:

- **Only top-level windows participate.** *"Only an overlapped or pop-up window can be an owner window; a child window cannot be an owner window."* If `hwndParent` names a child window, *"the system assigns ownership to the top-level parent window of the child window."*
- **Ownership is immutable after creation.** *"After creating an owned window, an application cannot transfer ownership of the window to another window."* **DOC VS REALITY:** this flatly contradicts `SetWindowLongPtr(GWLP_HWNDPARENT)` being documented as *"Sets a new owner for a top-level window"* on the same docs site. Both statements are primary. Treat the Window Features sentence as advisory-but-stale and the `SetWindowLongPtr` mechanism as the real one; the conflict is unresolved in the documentation.
- **Owner does NOT govern coordinates or clipping.** Nothing in any fetched page attributes coordinate mapping or clipping to the owner relation; owned windows are top-level and positioned in screen coordinates (a dialog template's coordinates are relative to the owner's client area only as a one-time creation-time convenience, per [About Dialog Boxes](https://learn.microsoft.com/en-us/windows/win32/dlgbox/about-dialog-boxes)).
- **Owner does not govern activation directly** — but modal dialogs disable the owner, which indirectly prevents its activation (§4).
- **Owner cascade for topmost status.** *"When a non-topmost window is made topmost, its owned windows are also made topmost. Its owners, however, are not changed."* and *"Any window (for example, a dialog box) owned by a topmost window is itself made a topmost window, to ensure that all owned windows stay above their owner."* ([SetWindowPos](https://learn.microsoft.com/en-us/windows/win32/api/winuser/nf-winuser-setwindowpos))

### Retrieval APIs conflate the two

`GetParent` is the trap: *"If the window is a child window, the return value is a handle to the parent window. If the window is a top-level window with the WS_POPUP style, the return value is a handle to the owner window."* Its own remarks tell you not to use it: *"To obtain a window's owner window, instead of using GetParent, use GetWindow with the GW_OWNER flag. To obtain the parent window and not the owner, instead of using GetParent, use GetAncestor with the GA_PARENT flag."* ([GetParent](https://learn.microsoft.com/en-us/windows/win32/api/winuser/nf-winuser-getparent))

`GetAncestor` gives the clean split ([GetAncestor](https://learn.microsoft.com/en-us/windows/win32/api/winuser/nf-winuser-getancestor)):

| Flag | Meaning |
| --- | --- |
| `GA_PARENT` | *"Retrieves the parent window. This does not include the owner, as it does with the GetParent function."* |
| `GA_ROOT` | *"Retrieves the root window by walking the chain of parent windows."* |
| `GA_ROOTOWNER` | *"Retrieves the owned root window by walking the chain of parent and owner windows returned by GetParent."* |

`GA_ROOTOWNER` is the only API that walks the *combined* parent-then-owner chain — evidence that the two relations are separate edge sets that a caller must explicitly ask to have concatenated.

### SetParent and the owner relation

[SetParent](https://learn.microsoft.com/en-us/windows/win32/api/winuser/nf-winuser-setparent) documents that it can retarget *"a pop-up, overlapped, or child window"*, that it *"does not modify the WS_CHILD or WS_POPUP window styles"* for compatibility (you must fix styles yourself), and that `NULL` means the desktop, `HWND_MESSAGE` means a message-only window.

**AMBIGUOUS:** the `SetParent` page says **nothing** about what happens to an existing *owner* relationship. Given the mutual-exclusivity rule, calling `SetParent` on an owned top-level window must resolve the conflict somehow, but the documentation does not state whether the owner is cleared, retained, or reinterpreted. **[UNVERIFIED]** — do not assume a behavior here.

---

## 2. Z-order

### One global list, but grouped

> *"The system maintains the z-order in a single list. It adds windows to the z-order based on whether they are topmost windows, top-level windows, or child windows... All topmost windows appear in the z-order before any non-topmost windows. **A child window is grouped with its parent in z-order.**"*
> — [Window Features → Z-Order](https://learn.microsoft.com/en-us/windows/win32/winmsg/window-features)

So the honest answer is *both*: there is one linear list, but membership in it is structured so that a parent's children form a contiguous run adjacent to the parent. *"When a window comes to the top of z-order, so do its child windows."* Bringing a top-level window forward moves its whole subtree with it — a child can never be interleaved between two unrelated top-level windows.

### The traversal API is scoped, not global

`GetWindow`'s z-order commands are explicitly per-cohort ([GetWindow](https://learn.microsoft.com/en-us/windows/win32/api/winuser/nf-winuser-getwindow)):

> `GW_HWNDNEXT` — *"The retrieved handle identifies the window below the specified window in the Z order. If the specified window is a topmost window, the handle identifies a topmost window. If the specified window is a top-level window, the handle identifies a top-level window. **If the specified window is a child window, the handle identifies a sibling window.**"*

Same qualification for `GW_HWNDPREV`, `GW_HWNDFIRST`, `GW_HWNDLAST`. `GW_CHILD` returns *"the child window at the top of the Z order... The function examines only child windows of the specified window. It does not examine descendant windows."*

Practically: **for an application author, z-order behaves as a per-parent ordering among siblings, plus a separate ordering among top-level windows, plus a topmost/non-topmost band split.** The "single list" is an implementation statement about how the window manager stores it, not an API surface you can traverse across levels.

Consistently, `BringWindowToTop` and the creation rule operate within the cohort: *"When an application creates a window, the system puts it at the top of the z-order for windows of the same type."*

### Owner's role in z-order

The owner relation is an ordering *constraint* layered on the list, not a separate list. *"An owned window is always above its owner in the z-order."* `SetWindowPos` enforces this across the topmost/non-topmost band boundary: *"A non-topmost window can own a topmost window, but the reverse cannot occur. Any window (for example, a dialog box) owned by a topmost window is itself made a topmost window, to ensure that all owned windows stay above their owner."* `SWP_NOOWNERZORDER` exists precisely to opt out of the owner-position side effects: *"Does not change the owner window's position in the Z order."* ([SetWindowPos](https://learn.microsoft.com/en-us/windows/win32/api/winuser/nf-winuser-setwindowpos))

### Activation moves z-order

*"The system positions the active window at the top of the z-order for windows of the same type."* ([Window Features → Active Window](https://learn.microsoft.com/en-us/windows/win32/winmsg/window-features)). And `SetWindowPos` documents the coupling in the other direction: *"An application cannot activate an inactive window without also bringing it to the top of the Z order."*

---

## 3. Focus / activation / capture — three distinct things

### Activation: per-input-queue, top-level only

> *"An active window is the top-level window of the application with which the user is currently working... **Only a top-level window can be an active window.** When the user is working with a child window, the system activates the top-level parent window associated with the child window."*
> — [Window Features → Active Window](https://learn.microsoft.com/en-us/windows/win32/winmsg/window-features)

`GetActiveWindow` *"Retrieves the window handle to the active window attached to the calling thread's message queue"* ([GetActiveWindow](https://learn.microsoft.com/en-us/windows/win32/api/winuser/nf-winuser-getactivewindow)). So activation is scoped to the input queue, not global — the system-wide notion is the *foreground* window (`GetForegroundWindow`), a separate concept.

`WM_ACTIVATE` is *"Sent to both the window being activated and the window being deactivated... first to the window procedure of the top-level window being deactivated, then to the window procedure of the top-level window being activated"* with `wParam` low word ∈ {`WA_INACTIVE`=0, `WA_ACTIVE`=1, `WA_CLICKACTIVE`=2} ([WM_ACTIVATE](https://learn.microsoft.com/en-us/windows/win32/inputdev/wm-activate)). Cross-application transitions instead produce `WM_ACTIVATEAPP`.

**Activation is not a tree.** It is one HWND per input queue, constrained to be a top-level window.

### Focus: one HWND per input queue, any depth

> *"The keyboard focus is a temporary property of a window... The window that has the keyboard focus receives (from the message queue of the thread that created it) all keyboard messages until the focus changes to a different window."*
>
> *"The window with the keyboard focus is either the active window, or a child window of the active window."*
> — [Keyboard Input Overview](https://learn.microsoft.com/en-us/windows/win32/inputdev/about-keyboard-input)

`SetFocus` *"Sets the keyboard focus to the specified window. The window must be attached to the calling thread's message queue."* It *"sends a WM_KILLFOCUS message to the window that loses the keyboard focus and a WM_SETFOCUS message to the window that receives the keyboard focus. **It also activates either the window that receives the focus or the parent of the window that receives the focus.**"* Passing `NULL` means *"keystrokes are ignored."* ([SetFocus](https://learn.microsoft.com/en-us/windows/win32/api/winuser/nf-winuser-setfocus))

So focus is a **single HWND value with no tree structure of its own** — but it is coupled to activation by an invariant (focus ⊆ active window's subtree) that `SetFocus` maintains by *changing activation as a side effect*. That coupling is the only place the parent tree enters the picture.

Focus interacts with the enabled state: *"If an application uses the EnableWindow function to disable a keyboard-focus window, the window loses the keyboard focus... EnableWindow then sets the keyboard focus to NULL... If a child window, or other descendant window, has the keyboard focus, the descendant window loses the focus when the parent window is disabled."* ([Window Features → Disabled Windows](https://learn.microsoft.com/en-us/windows/win32/winmsg/window-features))

`DefWindowProc` maintains the coupling on activation: *"If the window is being activated and is not minimized, the DefWindowProc function sets the keyboard focus to the window."* ([WM_ACTIVATE](https://learn.microsoft.com/en-us/windows/win32/inputdev/wm-activate))

### Capture: one HWND, overrides hit-testing

`GetCapture` *"Retrieves a handle to the window (if any) that has captured the mouse. **Only one window at a time can capture the mouse; this window receives mouse input whether or not the cursor is within its borders.**"* The handle is *"the capture window associated with the current thread"*; another thread may hold capture invisibly to you ([GetCapture](https://learn.microsoft.com/en-us/windows/win32/api/winuser/nf-winuser-getcapture)).

**Does capture bypass hit-testing? Yes, explicitly:**

> *"If the mouse is not captured, the message is sent to the window beneath the cursor. **Otherwise, the message is sent to the window that has captured the mouse.**"*
> — [WM_NCHITTEST](https://learn.microsoft.com/en-us/windows/win32/inputdev/wm-nchittest)

and

> *"The system typically posts a mouse message to the window that contains the cursor hot spot when a mouse event occurs. An application can change this behavior by using the SetCapture function to route mouse messages to a specific window. The window receives all mouse messages until the application calls ReleaseCapture or specifies another capture window, or until the user clicks a window created by another thread."*
> — [Mouse Input Overview → Mouse Capture](https://learn.microsoft.com/en-us/windows/win32/inputdev/about-mouse-input)

Note the hit-test message is still *sent* — it goes to the capture holder instead of the geometric target. Hit-testing is not skipped; its *destination* is redirected.

**Nuance / DOC VS REALITY:** the `SetCapture` page states a narrower rule than the conceptual page: *"SetCapture captures mouse input either when the mouse is over the capturing window, **or when the mouse button was pressed while the mouse was over the capturing window and the button is still down.**"* and *"If the mouse cursor is over a window created by another thread, the system will direct mouse input to the specified window only if a mouse button is down."* ([SetCapture](https://learn.microsoft.com/en-us/windows/win32/api/winuser/nf-winuser-setcapture)). The blanket "all mouse input regardless of position" is only fully true within your own thread's windows and, cross-thread, only while a button is held. Design to the narrower rule.

Additional constraints from `SetCapture`: *"Only the foreground window can capture the mouse... even if the foreground window has captured the mouse, the user can still click another window, bringing it to the foreground."* / *"This function cannot be used to capture mouse input meant for another process."* / *"When the mouse is captured, menu hotkeys and other keyboard accelerators do not work."*

### Capture-loss semantics

`WM_CAPTURECHANGED` (0x0215) is *"Sent to the window that is losing the mouse capture."* `lParam` is *"A handle to the window gaining the mouse capture."* `wParam` unused. Remarks:

> *"A window receives this message even if it calls ReleaseCapture itself. **An application should not attempt to set the mouse capture in response to this message.** When it receives this message, a window should redraw itself, if necessary, to reflect the new mouse-capture state."*
> — [WM_CAPTURECHANGED](https://learn.microsoft.com/en-us/windows/win32/inputdev/wm-capturechanged)

Losing capture means: any drag/tracking interaction in progress must be abandoned, not completed — you will not get the matching button-up. The message is a unilateral notification, not a veto point. `EnableWindow(hWnd, FALSE)` also sends `WM_CANCELMODE` (which is the "abandon modal-ish interaction" signal) before `WM_ENABLE`; and *"As the system creates the modal dialog box, it sends the WM_CANCELMODE message to the window (if any) currently capturing mouse input. An application that receives this message should release the mouse capture"* ([About Dialog Boxes](https://learn.microsoft.com/en-us/windows/win32/dlgbox/about-dialog-boxes)).

### Summary of §3

| Concept | Shape | Scope | Tree-structured? |
| --- | --- | --- | --- |
| Active | single HWND, must be top-level | per input queue | No |
| Focus | single HWND, any depth | per input queue | No (but invariant ties it into active window's subtree) |
| Capture | single HWND | one per thread; foreground-gated | No |

---

## 4. Modality

### Modality is not a first-class window property

There is no `WS_MODAL` style, no `IsWindowModal`, no modal flag in any fetched reference page. What exists is a *composition of three independent mechanisms*, spelled out verbatim in the `DialogBoxParam` remarks:

> *"The function displays the dialog box (regardless of whether the template specifies the WS_VISIBLE style), **disables the owner window**, and **starts its own message loop** to retrieve and dispatch messages for the dialog box.*
>
> *When the dialog box procedure calls the EndDialog function, DialogBoxParam destroys the dialog box, ends the message loop, enables the owner window (if previously enabled), and returns the nResult parameter..."*
> — [DialogBoxParam](https://learn.microsoft.com/en-us/windows/win32/api/winuser/nf-winuser-dialogboxparama)

The three ingredients:

1. **The owner relation** — supplies z-order-above-owner and the hide/destroy cascade (§1).
2. **The disabled state** (`WS_DISABLED` / `EnableWindow`) applied to the owner — supplies input blocking.
3. **A nested message loop** — supplies the *call-stack-blocking control flow* and the `INT_PTR` return value.

[About Dialog Boxes](https://learn.microsoft.com/en-us/windows/win32/dlgbox/about-dialog-boxes) confirms and adds detail:

> *"When the owner window is not already disabled, the system automatically disables the window and any child windows belonging to it when it creates the modal dialog box. The owner window remains disabled until the dialog box is destroyed. Although a dialog box procedure could potentially enable the owner window at any time, enabling the owner defeats the purpose of the modal dialog box and is not recommended. When the dialog box procedure is destroyed, the system enables the owner window again, but only if the modal dialog box caused the owner to be disabled."*

and

> *"To process messages for the modal dialog box, the system starts its own message loop, taking temporary control of the message queue for the entire application. **When the system retrieves a message that is not explicitly for the dialog box, it dispatches the message to the appropriate window.** If it retrieves a WM_QUIT message, it posts the message back to the application message queue so that the application's main message loop can eventually retrieve the message."*

Note the correction to a common misconception: the modal loop is **not** a filter that only pumps the dialog's messages. It pumps *everything* and dispatches normally. Other windows keep painting, keep receiving timers, keep receiving posted messages. What stops them receiving *user input* is the disabled state, not the loop. **This is the crux: the loop provides control flow, the disable provides input blocking. They are orthogonal.**

Two further pieces of evidence that the loop is not the blocking mechanism:

- *"When a modal dialog box has no owner, the system does not disable any of the application's other windows and allows the user to continue to carry out work in the other windows, defeating the purpose of the modal dialog box."* — an ownerless `DialogBox` still runs its nested loop, and is still *not modal in effect*. The loop alone buys nothing.
- `MB_TASKMODAL` exists so `MessageBox` can *"direct the system to disable all top-level windows belonging to the current thread"* — an extension of the disable set, not of the loop.

### Is the disable mechanism separable from the nested loop? Yes.

`EnableWindow` is a plain, first-class, independently callable window-state API:

> *"Enables or disables mouse and keyboard input to the specified window or control. When input is disabled, the window does not receive input such as mouse clicks and key presses."*
>
> *"If the window is being disabled, the system sends a WM_CANCELMODE message. If the enabled state of a window is changing, the system sends a WM_ENABLE message after the WM_CANCELMODE message... If a window is already disabled, its child windows are implicitly disabled, although they are not sent a WM_ENABLE message."*
>
> *"A window must be enabled before it can be activated. **For example, if an application is displaying a modeless dialog box and has disabled its main window, the application must enable the main window before destroying the dialog box.** Otherwise, another window will receive the keyboard focus and be activated. If a child window is disabled, it is ignored when the system tries to determine which window should receive mouse messages."*
> — [EnableWindow](https://learn.microsoft.com/en-us/windows/win32/api/winuser/nf-winuser-enablewindow)

That highlighted sentence is a **primary-source acknowledgment that the modeless-dialog + disabled-owner pattern is a real, supported thing** — Microsoft documents the ordering hazard for exactly that construction. It is not a hack; it is the documented composition minus the loop.

The disabled state's input-blocking is enforced by the window manager itself, independent of any message loop ([Window Features → Disabled Windows](https://learn.microsoft.com/en-us/windows/win32/winmsg/window-features)):

> *"A disabled window receives no keyboard or mouse input from the user, but it can receive messages from other windows, from other applications, and from the system."*

and note it is a *routing* rule, not a discard rule, for children: *"When a child window is disabled, the system passes the child's mouse input messages to the parent window."*

### Verdict for froth

**A library that has rejected nested message pumps can implement Win32-style modality faithfully.** The recipe, entirely from documented primitives:

1. Create the dialog as an ordinary top-level surface, owned by the blocked surface (owner relation → always above owner, cascaded hide/destroy).
2. `EnableWindow(owner, FALSE)` — or the froth equivalent: a per-surface `disabled` bit that the input router consults *before* hit-testing and *before* keyboard delivery.
3. Pump everything from the single flat loop as normal.
4. On dismissal: re-enable the owner **before** destroying the dialog, then destroy, then deliver the result via callback/continuation rather than a return value.

What you lose by dropping the nested loop — all of it control-flow ergonomics, none of it semantics:

- **The synchronous return value.** `DialogBoxParam` returns `EndDialog`'s `nResult`; *"The system does not return control from the function that creates the dialog box until the dialog box procedure has called the EndDialog function."* Without a nested loop the caller must be continuation-passing. This is the entire real cost.
- **`WM_ENTERIDLE`.** *"The system sends the WM_ENTERIDLE message to the owner window whenever the application message queue is empty"* during a modal dialog. Suppressible with `DS_NOIDLEMSG`. An idle-callback equivalent is trivial in a flat loop.
- **The automatic dialog keyboard interface.** *"The system automatically provides the keyboard interface for all modal dialog boxes. It does not provide the interface for modeless dialog boxes unless the application calls the IsDialogMessage function to filter messages in its main message loop."* ([Dialog Box Programming Considerations](https://learn.microsoft.com/en-us/windows/win32/dlgbox/dlgbox-programming-considerations)) — i.e. the modal loop's *only* other privilege is that it calls the `IsDialogMessage` equivalent for you. **[UNVERIFIED]** that the internal loop literally calls `IsDialogMessage`; the docs describe the effect, not the implementation. Either way, a flat loop reproduces it by calling `IsDialogMessage` (or froth's equivalent) itself — which is exactly what modeless dialogs already do.
- **The disable/enable bookkeeping.** `DialogBoxParam` re-enables the owner *"if previously enabled"*, i.e. it tracks whether it was the one that disabled it, so nested dialogs don't prematurely re-enable. A library must do that refcount/ownership tracking itself.

The one genuinely load-bearing caveat: **ordering**. Re-enable before destroy, per the `EnableWindow` remarks, or activation lands somewhere unintended.

---

## 5. Message routing

### There is no general bubbling

The dispatch model is flat: `DispatchMessage` delivers to the `hwnd` in the `MSG` and calls that window's `WndProc`. Nothing walks a chain looking for a handler. Confirmed structurally by [About Windows](https://learn.microsoft.com/en-us/windows/win32/winmsg/about-windows) — *"The chief component of a window class is the window procedure, a function that receives and processes all input and requests sent to the window"* — and by the explicit statement in [Window Features → Messages](https://learn.microsoft.com/en-us/windows/win32/winmsg/window-features):

> *"The system passes a child window's input messages **directly to the child window; the messages are not passed through the parent window**. The only exception is if the child window has been disabled by the EnableWindow function. In this case, the system passes any input messages that would have gone to the child window to the parent window instead."*

So the *one* input-routing rule involving the parent is the disabled-child redirect — and it is a single hop to the immediate parent, not a walk.

### `WM_COMMAND` / `WM_NOTIFY`: one hop to the immediate parent

`WM_COMMAND` is *"Sent when the user invokes a command item from a menu, when a control sends a notification message to its parent window, or when an accelerator keystroke is translated."* Three unrelated sources multiplexed onto one message, disambiguated by `wParam`'s high word ([WM_COMMAND](https://learn.microsoft.com/en-us/windows/win32/menurc/wm-command)):

| Source | wParam (high word) | wParam (low word) | lParam |
| --- | --- | --- | --- |
| Menu | 0 | Menu identifier | 0 |
| Accelerator | 1 | Accelerator identifier | 0 |
| Control | Control-defined notification code | Control identifier | Handle to the control window |

`WM_NOTIFY` is the newer, structured replacement, and its destination rule is unambiguous ([WM_NOTIFY](https://learn.microsoft.com/en-us/windows/win32/controls/wm-notify)):

> *"Sent by a common control to its parent window when an event has occurred or the control requires some information."*
>
> *"**The destination of the message must be the HWND of the parent of the control.** This value can be obtained by using GetParent."*

The documented sample literally does `SendMessage(GetParent(m_controlHwnd), WM_NOTIFY, ...)`. **This is a single hop, performed by the control, to `GetParent` of the control.** It is not a framework-provided bubble and there is no continuation past the first parent. If a grandparent needs the notification, the parent must forward it explicitly. (For Vista+, *"the WM_NOTIFY message cannot be sent between processes."*)

Why it looks like bubbling in dialogs: controls in a dialog are direct children of the dialog, so one hop reaches the dialog procedure. The apparent "bubble to the form" is just a flat tree.

### The narrow set of messages `DefWindowProc` genuinely propagates upward

Real multi-level propagation exists, but only for a handful of messages and only because `DefWindowProc` implements it — not because the dispatcher does:

- **`WM_MOUSEWHEEL`:** *"Rotating the mouse sends the WM_MOUSEWHEEL message to the focus window. The DefWindowProc function propagates the message to the window's parent. There should be no internal forwarding of the message, since **DefWindowProc propagates it up the parent chain until a window that processes it is found**."* ([Mouse Input Overview](https://learn.microsoft.com/en-us/windows/win32/inputdev/about-mouse-input)) — this is the one true multi-level bubble in the fetched docs.
- **`WM_APPCOMMAND`:** *"DefWindowProc sends a WM_APPCOMMAND message to a window when either XBUTTON1 or XBUTTON2 is clicked. DefWindowProc also sends the WM_APPCOMMAND message to its parent window."* ([Mouse Input Overview](https://learn.microsoft.com/en-us/windows/win32/inputdev/about-mouse-input); also [Keyboard Input Overview](https://learn.microsoft.com/en-us/windows/win32/inputdev/about-keyboard-input), which uses the word *"bubbles"*).
- **`WM_CONTEXTMENU`:** same pattern — *"DefWindowProc sends a WM_CONTEXTMENU message on a right button click, and bubbles it to its parent."*
- **`WM_CHANGEUISTATE`:** *"Each window will continue to send the message to its parent until it reaches the top-level window."* ([About Keyboard Accelerators](https://learn.microsoft.com/en-us/windows/win32/menurc/about-keyboard-accelerators))

Design point for froth: bubbling in Win32 is **opt-in default behavior implemented in one function**, applied to a small, deliberately chosen set of messages. It is not a property of the message system.

### `DefWindowProc`'s role

> *"Calls the default window procedure to provide default processing for any window messages that an application does not process. **This function ensures that every message is processed.** DefWindowProc is called with the same parameters received by the window procedure."*
> — [DefWindowProc](https://learn.microsoft.com/en-us/windows/win32/api/winuser/nf-winuser-defwindowproca)

The dialog manager substitutes its own layer here. A dialog procedure *"never calls the DefWindowProc function. Instead, it returns TRUE if it processes a message or FALSE if it does not"*; the predefined dialog window procedure then applies a documented default table (`WM_SETFOCUS`, `WM_NEXTDLGCTL`, `WM_ACTIVATE`, `WM_CLOSE`→`IDCANCEL`, `DM_GETDEFID`/`DM_SETDEFID`, …) and *"passes all other messages to DefWindowProc for default processing."* Custom dialog classes call `DefDlgProc` instead of `DefWindowProc`. ([Dialog Box Programming Considerations](https://learn.microsoft.com/en-us/windows/win32/dlgbox/dlgbox-programming-considerations))

So there is a *default-handling chain* (WndProc → DefWindowProc, or DlgProc → DefDlgProc → DefWindowProc), but it is a chain **within one window**, not up the tree.

### `WM_NCHITTEST` is the mouse-target routing mechanism

> *"Sent to a window in order to determine what part of the window corresponds to a particular screen coordinate. This can happen, for example, when the cursor moves, when a mouse button is pressed or released, or in response to a call to a function such as WindowFromPoint. **If the mouse is not captured, the message is sent to the window beneath the cursor. Otherwise, the message is sent to the window that has captured the mouse.**"*
> — [WM_NCHITTEST](https://learn.microsoft.com/en-us/windows/win32/inputdev/wm-nchittest)

The return value both selects client-vs-nonclient delivery and identifies the nonclient part:

> *"If the cursor is in the client area of a window, DefWindowProc returns the HTCLIENT hit-test value... the system converts the screen coordinates of the cursor hot spot to client coordinates, and then posts the appropriate client area mouse message. The DefWindowProc function returns one of the other hit-test values when the cursor hot spot is in a window's nonclient area... the system posts a nonclient area mouse message, placing the hit-test value in the message's wParam parameter."*
> — [Mouse Input Overview](https://learn.microsoft.com/en-us/windows/win32/inputdev/about-mouse-input)

Values include `HTCLIENT` (1), `HTCAPTION` (2), `HTSYSMENU` (3), `HTGROWBOX`/`HTSIZE` (4), `HTMENU` (5), `HTHSCROLL` (6), `HTVSCROLL` (7), `HTMINBUTTON`/`HTREDUCE` (8), `HTMAXBUTTON`/`HTZOOM` (9), the border/corner set `HTLEFT`(10)…`HTBOTTOMRIGHT`(17), `HTBORDER` (18), `HTCLOSE` (20), `HTHELP` (21), `HTNOWHERE` (0), `HTTRANSPARENT` (-1), `HTERROR` (-2).

Two routing-relevant returns:

- `HTTRANSPARENT`: *"In a window currently covered by another window in the same thread (**the message will be sent to underlying windows in the same thread until one of them returns a code that is not HTTRANSPARENT**)."* — a documented *sideways/downward* re-routing loop, thread-scoped.
- `HTERROR`: same as `HTNOWHERE` but `DefWindowProc` beeps.

Also in the hit-test/routing path: `WM_MOUSEACTIVATE`, sent *"after posting the WM_NCHITTEST message to the window, but before posting the button-down message"*, whose return (`MA_ACTIVATE`, `MA_NOACTIVATE`, `MA_ACTIVATEANDEAT`, `MA_NOACTIVATEANDEAT`) lets a window suppress activation and/or swallow the click ([Mouse Input Overview → Window Activation](https://learn.microsoft.com/en-us/windows/win32/inputdev/about-mouse-input)).

Layered windows change hit-testing geometrically: *"Hit testing of a layered window is based on the shape and transparency of the window... areas of the window that are color-keyed or whose alpha value is zero will let the mouse messages through"*, and `WS_EX_TRANSPARENT` bypasses shape entirely ([Window Features](https://learn.microsoft.com/en-us/windows/win32/winmsg/window-features)).

---

## 6. Keyboard navigation

### It is a convention layered on by the dialog manager, not a windowing primitive

Unequivocal:

> *"The system automatically provides the keyboard interface for all modal dialog boxes. **It does not provide the interface for modeless dialog boxes unless the application calls the IsDialogMessage function to filter messages in its main message loop.** This means that the application must pass the message to IsDialogMessage immediately after retrieving the message from the message queue."*
> — [Dialog Box Programming Considerations](https://learn.microsoft.com/en-us/windows/win32/dlgbox/dlgbox-programming-considerations)

And `IsDialogMessage` is explicitly a general-purpose bolt-on, not dialog-specific:

> *"Although the IsDialogMessage function is intended for modeless dialog boxes, **you can use it with any window that contains controls**, enabling the windows to provide the same keyboard selection as is used in a dialog box."*
>
> *"Because the IsDialogMessage function performs all necessary translating and dispatching of messages, a message processed by IsDialogMessage must not be passed to the TranslateMessage or DispatchMessage function."*
>
> *"IsDialogMessage sends WM_GETDLGCODE messages to the dialog box procedure to determine which keys should be processed."*
> — [IsDialogMessage](https://learn.microsoft.com/en-us/windows/win32/api/winuser/nf-winuser-isdialogmessagea)

A plain window loop gets **none** of this unless it calls `IsDialogMessage`. There is no tab order, no arrow navigation, no mnemonic handling, no default-button Enter handling at the raw windowing layer.

### The key table

| Key | Action |
| --- | --- |
| TAB | Moves the input focus to the next control that has the `WS_TABSTOP` style. |
| SHIFT+TAB | Moves the input focus to the previous control that has `WS_TABSTOP`. |
| RIGHT / DOWN | Moves the input focus to the next control **in the group**. |
| LEFT / UP | Moves the input focus to the previous control **in the group**. |
| ENTER | Sends `WM_COMMAND` with `wParam` = `IDOK` or the default push button's identifier. |
| ESC | Sends `WM_COMMAND` with `wParam` = `IDCANCEL`. |
| *mnemonic* / ALT+*mnemonic* | Moves focus to the first `WS_TABSTOP` control after the static control containing the mnemonic. |

(verbatim from [Dialog Box Programming Considerations](https://learn.microsoft.com/en-us/windows/win32/dlgbox/dlgbox-programming-considerations))

### `WM_GETDLGCODE` is the per-control veto

Before the dialog manager consumes a navigation key, it asks the focused control whether it wants it:

> *"By default, the system handles all keyboard input to the control; the system interprets certain types of keyboard input as dialog box navigation keys. To override this default behavior, the control can respond to the WM_GETDLGCODE message to indicate the types of input it wants to process itself."*
> — [WM_GETDLGCODE](https://learn.microsoft.com/en-us/windows/win32/dlgbox/wm-getdlgcode)

Return flags: `DLGC_WANTARROWS` (0x0001), `DLGC_WANTTAB` (0x0002), `DLGC_WANTALLKEYS` (0x0004), `DLGC_WANTMESSAGE` (0x0004 — same value), `DLGC_HASSETSEL` (0x0008), `DLGC_DEFPUSHBUTTON` (0x0010), `DLGC_UNDEFPUSHBUTTON` (0x0020), `DLGC_RADIOBUTTON` (0x0040), `DLGC_WANTCHARS` (0x0080), `DLGC_BUTTON` (0x2000), `DLGC_STATIC` (0x0100).

> *"Although the DefWindowProc function always returns zero in response to the WM_GETDLGCODE message, the window procedure for the predefined control classes return a code appropriate for each class."*

The negotiation order per key class (from the same page):

- **TAB/SHIFT+TAB:** *"the system first determines whether these keys are processed by the control that currently has the input focus. It sends the control a WM_GETDLGCODE message, and if the control returns DLGC_WANTTAB, the system passes the keys to the control. Otherwise, the system uses the GetNextDlgTabItem function to locate the next control that is visible, not disabled, and that has the WS_TABSTOP style."*
- **Arrows:** *"The system sends a WM_GETDLGCODE message to the control and if the control returns the DLGC_WANTARROWS value, passes the key to the control. Otherwise, the system uses the GetNextDlgGroupItem function to determine the next control in the group."*
- **Mnemonics:** *"if the control returns the DLGC_WANTALLKEYS or DLG_WANTMESSAGE value, the system passes the key to the control."* (note the docs' typo `DLG_WANTMESSAGE`; the constant is `DLGC_WANTMESSAGE`)

### Navigation is strictly linear in creation order — never geometric

This is stated twice, once for each traversal:

> `GetNextDlgTabItem` — *"The search starts with the control currently having the input focus and proceeds **in the order in which the controls were created** — that is, the order in which they are defined in the dialog box template."*

> `GetNextDlgGroupItem` — *"searches controls **in the order (or reverse order) they were created.** If the user presses the RIGHT or DOWN key, GetNextDlgGroupItem returns the next control if that control does not have the WS_GROUP style. Otherwise, the function reverses the order of the search and returns the first control that has the WS_GROUP style. If the user presses the LEFT or UP key, the function returns the previous control unless the current control already has the WS_GROUP style..."*

**Win32 performs no spatial/directional focus navigation whatsoever.** Arrow keys do not move focus toward the geometrically-adjacent control; they move to the next/previous control *in the template's declaration order*, wrapping within the group. `WS_GROUP` changes only where that linear walk wraps:

> *"The style marks the beginning of a group of controls. If a control in the group has the input focus when the user begins pressing direction keys, the focus remains in the group. In general, the first control in a group must have the WS_GROUP style and all other controls in the group must not have this style. **All controls in the group must be contiguous — that is, they must have been created consecutively with no intervening controls.**"*

Nothing in any fetched page mentions rectangles, distances, or directional projection. Geometry enters focus handling only via the mouse.

Nesting: *"If the search for the next control with the WS_TABSTOP style encounters a window with the `WS_EX_CONTROLPARENT` style, the system recursively searches the window's children."* — so the walk is a depth-first traversal of the parent tree in creation order, gated by an opt-in extended style. Still order-based, not geometry-based.

### Related bits

- **Focus persistence across activation** is handled by the dialog default proc, not the window manager: on `WM_ACTIVATE` it *"Restores the input focus to the control identified by the previously saved handle if the dialog box is activated. Otherwise, the procedure saves the handle to the control having the input focus."* Same save/restore on `WM_SHOWWINDOW` (hide) and `WM_SYSCOMMAND` (minimize).
- **Initial focus:** *"The control receiving the default input focus is always the first control specified in the template that is visible, not disabled, and has the WS_TABSTOP style. If no such control exists, the system sets the default input focus to the first control in the template."*
- **`WM_NEXTDLGCTL`** is the programmatic entry point: *"Sets the input focus to the next or previous control in the dialog box, to the control identified by the handle in the wParam parameter, or to the first control in the dialog box that is visible, not disabled, and has the WS_TABSTOP style. The procedure ignores this message if the current window with the input focus is not a control."* Recommended over raw `SetFocus` for custom dialogs because it also *"updates the appearance of the controls, such as moving the default push button border or setting an automatic radio button."*
- **Accelerators are a third, independent layer.** `TranslateAccelerator` is called in the message loop; it *"monitors keyboard input to the message queue, checking for key combinations that match an entry in the accelerator table... it translates the keyboard input... into a WM_COMMAND or WM_SYSCOMMAND message and then sends the message to the window procedure of the specified window."* Note *sends to the specified window* — the HWND passed to `TranslateAccelerator`, not the focus window. *"only one accelerator table can be active at a time for a specified thread."* ([About Keyboard Accelerators](https://learn.microsoft.com/en-us/windows/win32/menurc/about-keyboard-accelerators))
- **Disabled controls are skipped** by all of the above: *"A disabled control cannot receive the keyboard focus, nor can a user gain access to it."* ([EnableWindow](https://learn.microsoft.com/en-us/windows/win32/api/winuser/nf-winuser-enablewindow))
- **Capture suppresses accelerators:** *"When the mouse is captured, menu hotkeys and other keyboard accelerators do not work."* ([SetCapture](https://learn.microsoft.com/en-us/windows/win32/api/winuser/nf-winuser-setcapture))

---

## 7. Verdict

### Genuine relations Win32 maintains between windows

There are **two**, and only two:

**1. The parent tree** (`WS_CHILD` + `hWndParent`; read with `GetAncestor(GA_PARENT)`). Massively overloaded. It governs:
- Coordinate space (child coords are relative to parent's client-area origin)
- Hard clipping of the child to the parent's client area
- Opt-in clipping the other way (`WS_CLIPCHILDREN`) and between siblings (`WS_CLIPSIBLINGS`)
- Z-order *grouping* — a child's ordering is scoped to its siblings and the whole subtree moves with the parent
- Destroy / hide / show / move cascade
- `WM_COMMAND` and `WM_NOTIFY` notification delivery (**one hop, to the immediate parent**)
- Disabled-child mouse-input redirect to the parent (one hop)
- Implicit disabling of descendants when an ancestor is disabled
- Tab/group navigation traversal order (creation order within the tree, recursing only through `WS_EX_CONTROLPARENT`)
- `DefWindowProc`'s upward propagation of a short whitelist: `WM_MOUSEWHEEL`, `WM_APPCOMMAND`, `WM_CONTEXTMENU`, `WM_CHANGEUISTATE`

**2. The owner relation** (top-level windows only; `GWLP_HWNDPARENT`, read with `GetWindow(GW_OWNER)`). Thin. It governs:
- Z-order: owned window always above its owner; topmost status propagates owner→owned
- Destroy cascade: owner destroyed → owned destroyed
- Visibility: owned window hidden when owner minimized
- Which window a modal dialog disables (by convention of the dialog manager, not by the relation itself)

It does **not** govern coordinates, clipping, message routing, focus, or activation. Mutually exclusive with parent: *"A window can have either a parent or an owner, or neither, but never both simultaneously."*

### State that is NOT a hierarchy

- **Active window** — one HWND per input queue, constrained to be top-level. Children never "activate"; clicking a child activates its top-level ancestor. Not a tree.
- **Focus window** — one HWND per input queue, any depth. Not a tree. Coupled to activation only by the invariant that focus is the active window or a descendant of it, an invariant `SetFocus` maintains by *changing activation as a side effect*.
- **Mouse capture** — one HWND per thread, foreground-gated. Not a tree. **Overrides hit-testing entirely**: `WM_NCHITTEST` and all mouse messages go to the capture holder rather than the geometric target. Lost unilaterally (`WM_CAPTURECHANGED`, whose remarks forbid re-taking capture in response); the loser must abandon, not complete, the in-flight interaction.
- **Enabled/disabled** — a per-window boolean (`WS_DISABLED`), with one inheritance rule (disabling a window implicitly disables its children, without sending them `WM_ENABLE`). Not a hierarchy, but tree-aware.
- **Z-order** — one global list, but with structural constraints (topmost band before non-topmost band; children contiguous with their parent; owned above owner) that make it behave, from the API's point of view, as per-cohort orderings. The traversal API (`GW_HWNDNEXT`/`PREV`/`FIRST`/`LAST`) never crosses cohorts.
- **Modality** — **not state at all.** No flag, no style, no query. It is an emergent composition of: owner relation + `EnableWindow(owner, FALSE)` + a nested message loop that provides only control flow and (implicitly) the `IsDialogMessage` call. The nested loop dispatches *all* messages normally; it does not filter. Remove the loop and modality still works — you only lose the synchronous return value.

### The one-line answer to the core question

**The input hierarchy and the render hierarchy in Win32 are the same tree used for different purposes, plus a second thin edge set (owner) that is render/lifetime-only, plus three flat HWND registers (active, focus, capture) that are not hierarchies at all and that routinely override the tree** — capture overrides hit-testing outright; focus is the sole keyboard destination regardless of depth; activation collapses any depth to the top-level ancestor.

### Direct implications for froth

- Overloading one parent tree with coordinates + clipping + z-grouping + lifetime is the proven, load-bearing design. It is also the source of most Win32 API awkwardness (`GetParent` returning an owner, `SetParent` not fixing styles, `GWLP_HWNDPARENT`'s two meanings). If you want a second relation, give it its own field and its own accessor from day one.
- Focus and capture must be *single values*, not tree positions. Anything that walks a tree to find "the focused thing" diverges from the model.
- Capture must sit *in front of* hit-testing in the input pipeline, not inside it.
- Modality is implementable without nested pumps. The mechanism to build is a **disabled bit consulted by the input router before routing**, plus a disable-refcount so nested dialogs re-enable correctly, plus a strict enable-before-destroy ordering rule. Everything else `DialogBox` does is control-flow sugar you have already decided not to buy.
- Keyboard navigation is convention, not primitive, and it is *linear in declaration order* — never geometric. If froth wants directional navigation it is inventing something Win32 does not have; do not expect Win32 sources for it.
- Notification routing is one hop to the parent, not a bubble. General bubbling exists only for four specific messages and is implemented in `DefWindowProc`, i.e. it is opt-in default behavior, not a dispatcher feature.

---

## Sources

All URLs below were fetched and read directly during this research.

| URL | Used for |
| --- | --- |
| [About Windows](https://learn.microsoft.com/en-us/windows/win32/winmsg/about-windows) | Parent-vs-owner attribute definition, child coordinate space, top-level definition, owned-window summary, window creation messages, `WM_PARENTNOTIFY` on child creation |
| [Window Features](https://learn.microsoft.com/en-us/windows/win32/winmsg/window-features) | Child window positioning/clipping/parent-relationship/messages, Owned Windows (the three constraints), Z-Order (single list, child grouping), Active Window, Disabled Windows, Foreground/Background, layered-window hit testing, message-only windows |
| [GetParent](https://learn.microsoft.com/en-us/windows/win32/api/winuser/nf-winuser-getparent) | `GetParent` returns owner for `WS_POPUP` top-level windows; steer to `GetWindow(GW_OWNER)` / `GetAncestor(GA_PARENT)` |
| [GetAncestor](https://learn.microsoft.com/en-us/windows/win32/api/winuser/nf-winuser-getancestor) | `GA_PARENT` / `GA_ROOT` / `GA_ROOTOWNER` — the clean parent-vs-owner split |
| [GetWindow](https://learn.microsoft.com/en-us/windows/win32/api/winuser/nf-winuser-getwindow) | `GW_OWNER`, `GW_CHILD`, `GW_ENABLEDPOPUP`, and the per-cohort scoping of `GW_HWNDNEXT`/`PREV`/`FIRST`/`LAST` |
| [SetParent](https://learn.microsoft.com/en-us/windows/win32/api/winuser/nf-winuser-setparent) | What `SetParent` does and does not do (styles unchanged); silence on the owner relation |
| [GetWindowLongPtr](https://learn.microsoft.com/en-us/windows/win32/api/winuser/nf-winuser-getwindowlongptra) | `GWLP_HWNDPARENT` getter description (documented as "parent", inconsistently with the setter) |
| [SetWindowLongPtr](https://learn.microsoft.com/en-us/windows/win32/api/winuser/nf-winuser-setwindowlongptra) | The definitive parent-XOR-owner statement; `GWLP_HWNDPARENT` sets the *owner* of a top-level window; use `SetParent` for parents |
| [SetWindowPos](https://learn.microsoft.com/en-us/windows/win32/api/winuser/nf-winuser-setwindowpos) | `HWND_TOP`/`BOTTOM`/`TOPMOST`/`NOTOPMOST`, `SWP_NOOWNERZORDER`, topmost propagation to owned windows, activation↔z-order coupling |
| [Mouse Input Overview](https://learn.microsoft.com/en-us/windows/win32/inputdev/about-mouse-input) | Mouse Capture semantics, `WM_CAPTURECHANGED` on capture change, `WM_NCHITTEST` role and hit-test value table, client vs nonclient posting, `WM_MOUSEWHEEL` parent-chain propagation, `WM_APPCOMMAND`/`WM_CONTEXTMENU` bubbling, `WM_MOUSEACTIVATE` |
| [SetCapture](https://learn.microsoft.com/en-us/windows/win32/api/winuser/nf-winuser-setcapture) | Capture preconditions (foreground-only), cross-thread button-down qualification, accelerators disabled during capture, one window at a time |
| [GetCapture](https://learn.microsoft.com/en-us/windows/win32/api/winuser/nf-winuser-getcapture) | Capture is per-thread; receives input regardless of cursor position |
| [WM_CAPTURECHANGED](https://learn.microsoft.com/en-us/windows/win32/inputdev/wm-capturechanged) | Sent to the loser, `lParam` = winner, sent even on self-`ReleaseCapture`, must not re-take capture in the handler |
| [WM_NCHITTEST](https://learn.microsoft.com/en-us/windows/win32/inputdev/wm-nchittest) | Capture redirects the hit-test destination; full `HT*` value table with numeric codes; `HTTRANSPARENT` re-routing |
| [Keyboard Input Overview](https://learn.microsoft.com/en-us/windows/win32/inputdev/about-keyboard-input) | Keyboard Focus and Activation section: focus ⊆ active window subtree, `WM_KILLFOCUS`/`WM_SETFOCUS`, `DefWindowProc` sets focus on `WM_ACTIVATE`, `WM_APPCOMMAND` bubbling |
| [SetFocus](https://learn.microsoft.com/en-us/windows/win32/api/winuser/nf-winuser-setfocus) | Focus requires same input queue; `SetFocus` activates the window or its parent as a side effect; `NULL` = ignore keystrokes |
| [GetActiveWindow](https://learn.microsoft.com/en-us/windows/win32/api/winuser/nf-winuser-getactivewindow) | Active window is per calling thread's message queue; contrast with `GetForegroundWindow` |
| [WM_ACTIVATE](https://learn.microsoft.com/en-us/windows/win32/inputdev/wm-activate) | Sent to top-level windows only, `WA_*` values, `DefWindowProc` sets focus on activation |
| [EnableWindow](https://learn.microsoft.com/en-us/windows/win32/api/winuser/nf-winuser-enablewindow) | Disabled windows receive no keyboard/mouse input; `WM_CANCELMODE` then `WM_ENABLE`; implicit child disabling; **the documented modeless-dialog + disabled-owner pattern and its enable-before-destroy ordering rule**; disabled controls unfocusable |
| [WM_ENABLE](https://learn.microsoft.com/en-us/windows/win32/winmsg/wm-enable) | Sent to the window whose enabled state changed, after the `WS_DISABLED` bit changes, before `EnableWindow` returns |
| [About Dialog Boxes](https://learn.microsoft.com/en-us/windows/win32/dlgbox/about-dialog-boxes) | Modal vs modeless; dialog owner window role; automatic owner disabling and re-enabling; **the nested loop dispatches non-dialog messages normally**; `WM_QUIT` reposting; `WM_ENTERIDLE`/`DS_NOIDLEMSG`; `WM_CANCELMODE` to the capture holder; ownerless modal defeats modality; `MB_TASKMODAL`; modeless requires `IsDialogMessage`; `EndDialog` vs `DestroyWindow` |
| [DialogBoxParam](https://learn.microsoft.com/en-us/windows/win32/api/winuser/nf-winuser-dialogboxparama) | The verbatim three-step implementation: creates via `CreateWindowEx`, **disables the owner window**, **starts its own message loop**; re-enables and returns `nResult` on `EndDialog` |
| [Dialog Box Programming Considerations](https://learn.microsoft.com/en-us/windows/win32/dlgbox/dlgbox-programming-considerations) | Dialog keyboard interface key table; `WS_TABSTOP` / `WS_GROUP` semantics; `GetNextDlgTabItem` / `GetNextDlgGroupItem` **creation-order** search; `WS_EX_CONTROLPARENT` recursion; mnemonics; the automatic-for-modal / manual-for-modeless statement; dialog default message-processing table; `DlgProc` never calls `DefWindowProc`; `DefDlgProc` for custom classes; `WM_NEXTDLGCTL`; initial focus rule |
| [IsDialogMessage](https://learn.microsoft.com/en-us/windows/win32/api/winuser/nf-winuser-isdialogmessagea) | Usable with any window containing controls; must not also `TranslateMessage`/`DispatchMessage`; sends `WM_GETDLGCODE`; may send `DM_GETDEFID`/`DM_SETDEFID` |
| [WM_GETDLGCODE](https://learn.microsoft.com/en-us/windows/win32/dlgbox/wm-getdlgcode) | Full `DLGC_*` value table with numeric codes; `DefWindowProc` always returns zero; controls override navigation-key handling |
| [WM_COMMAND](https://learn.microsoft.com/en-us/windows/win32/menurc/wm-command) | Three sources (menu / accelerator / control) multiplexed; `wParam`/`lParam` decoding table |
| [WM_NOTIFY](https://learn.microsoft.com/en-us/windows/win32/controls/wm-notify) | Sent by a control **to its parent**; *"The destination of the message must be the HWND of the parent of the control"*; `GetParent`-based sample; no cross-process from Vista on |
| [DefWindowProc](https://learn.microsoft.com/en-us/windows/win32/api/winuser/nf-winuser-defwindowproca) | Default processing for unhandled messages; "ensures that every message is processed" |
| [About Keyboard Accelerators](https://learn.microsoft.com/en-us/windows/win32/menurc/about-keyboard-accelerators) | `TranslateAccelerator` in the message loop, translates to `WM_COMMAND`/`WM_SYSCOMMAND` sent to the *specified* window; one active table per thread; `WM_CHANGEUISTATE` parent-chain propagation |
