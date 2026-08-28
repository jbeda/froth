# Prior art: focus, tab order and hit testing in Win32

A survey of how the Windows API separates *walkable*, *focusable* and *clickable*. Every
claim below carries a primary-source URL. Where froth's existing notes were recall rather
than citation, the verdict is stated explicitly.

Sources are learn.microsoft.com and the MicrosoftDocs source repos (`MicrosoftDocs/win32`,
`MicrosoftDocs/sdk-api`), which are the same text. Raymond Chen is marked **commentary**.

---

## A. The three claims

### A1. `SetFocus` has no focusable style bit — **CONFIRMED, with a documentation caveat**

`SetFocus` documents no style requirement of any kind. Its contract is only:

> Sets the keyboard focus to the specified window. The window must be attached to the
> calling thread's message queue. […] If the function succeeds, the return value is the
> handle to the window that previously had the keyboard focus. If the *hWnd* parameter is
> invalid or the window is not attached to the calling thread's message queue, the return
> value is NULL. […] Extended error ERROR_INVALID_PARAMETER (0x57) means that window is in
> disabled state.
> — <https://learn.microsoft.com/en-us/windows/win32/api/winuser/nf-winuser-setfocus>

So the documented gates are: valid HWND, same input queue, **not disabled**. No
`WS_TABSTOP`, no class check, no "is this a control" check. `EnableWindow` corroborates
the disabled gate from the other side: *"A disabled control cannot receive the keyboard
focus, nor can a user gain access to it."*
— <https://learn.microsoft.com/en-us/windows/win32/api/winuser/nf-winuser-enablewindow>

Chen adds a fourth, undocumented-in-the-reference failure mode — a `WH_CBT` hook can veto
a focus change and `SetFocus` then returns NULL with `GetLastError() == 0` (**commentary**):
<https://devblogs.microsoft.com/oldnewthing/20190614-00/?p=102593>

**Caveat, and it is a real one.** The static-control overview asserts the opposite as
prose: *"Although static controls are child windows, they cannot be selected. Therefore,
they cannot receive the keyboard focus and cannot have a keyboard interface."*
— <https://learn.microsoft.com/en-us/windows/win32/controls/about-static-controls>

That sentence describes the *dialog manager's* behaviour and the control's lack of a
keyboard interface, not a gate in `SetFocus`. Nothing in the `SetFocus` reference, the
window-styles list, or the dialog-manager text describes a mechanism that would make
`SetFocus(hwndStatic)` fail.

**UNVERIFIED:** the specific empirical claim that `SetFocus` on a static text control
*succeeds* (returns the previous focus HWND, sends `WM_SETFOCUS`, and focus then sits
there doing nothing useful). That is consistent with everything documented and is widely
repeated, but I have no Windows machine and found no primary source stating it. Treat
"no focusable style bit exists" as confirmed; treat "focus visibly lands on a static" as
strongly implied but untested.

### A2. The dialog manager is a second, separate layer — **CONFIRMED, but froth's notes attach the quote to the wrong walk**

Both quotes froth's notes cite are still present on the page, verbatim. But they are in
the **`WS_GROUP` / arrow-key** section, not the **`WS_TABSTOP` / tab** section. This
matters, because the two walks do not work the same way.

**The tab walk (`WS_TABSTOP`) sends `WM_GETDLGCODE` to the control that currently has
focus, not to the candidate.**

> When the user presses TAB or SHIFT+TAB, the system first determines whether these keys
> are processed by the control that currently has the input focus. It sends the control a
> **WM_GETDLGCODE** message, and if the control returns DLGC_WANTTAB, the system passes
> the keys to the control. Otherwise, the system uses the **GetNextDlgTabItem** function
> to locate the next control that is visible, not disabled, and that has the
> **WS_TABSTOP** style.
> — <https://learn.microsoft.com/en-us/windows/win32/dlgbox/dlgbox-programming-considerations>

`GetNextDlgTabItem`'s own contract confirms the filter is exactly three flags, with no
`WM_GETDLGCODE` and no static check:

> The function returns the first control it locates that is visible, not disabled, and has
> the WS_TABSTOP style. If no such control exists, the function returns *hCtl*.
> — <https://learn.microsoft.com/en-us/windows/win32/api/winuser/nf-winuser-getnextdlgtabitem>

Statics are excluded from the tab walk only because they are not given `WS_TABSTOP`. There
is no veto step in the tab path.

**The arrow/group walk does query the candidate and does skip statics.** This is where
froth's two quoted sentences live:

> When the user presses a direction key, the system first determines whether the current
> control having the input focus processes the direction keys. The system sends a
> **WM_GETDLGCODE** message to the control and if the control returns the
> **DLGC_WANTARROWS** value, passes the key to the control. Otherwise, the system uses the
> **GetNextDlgGroupItem** function to determine the next control in the group. […]
> **After the system has the next or previous control, it sends a WM_GETDLGCODE message to
> the control to determine the control type. The system then moves the input focus to
> control if it is not a static control.** If the control is an automatic radio button, the
> system sends a **BM_CLICK** message to it.
> — same page

(froth's notes render the second sentence as *"The system moves the input focus to a
control if it is not a static control"*; the live wording is *"The system then moves the
input focus to control if it is not a static control"* — the doc's own typo included. Same
meaning; quote the live text.)

So the shape froth is interested in — *the creator sets a flag, and the control itself may
veto or amend* — is **real but confined to the arrow-key walk**. In the tab walk the
control can only claim the keystroke for itself (`DLGC_WANTTAB`); it cannot veto being
landed on.

**Opt-in-ness.** The whole layer is opt-in for anything other than a modal dialog:

> The system automatically provides the keyboard interface for all modal dialog boxes. It
> does not provide the interface for modeless dialog boxes unless the application calls the
> **IsDialogMessage** function to filter messages in its main message loop.
> — same page; and
> <https://learn.microsoft.com/en-us/windows/win32/api/winuser/nf-winuser-isdialogmessagea>
> (*"Although the IsDialogMessage function is intended for modeless dialog boxes, you can
> use it with any window that contains controls"* … *"IsDialogMessage sends WM_GETDLGCODE
> messages to the dialog box procedure to determine which keys should be processed."*)

`WS_TABSTOP`'s own entry says the same: *"For user-created windows and modeless dialogs to
work with tab stops, alter the message loop to call the IsDialogMessage function."*
— <https://learn.microsoft.com/en-us/windows/win32/winmsg/window-styles>

**Focus movement in this layer is more than `SetFocus`.** `WM_NEXTDLGCTL`:

> This message performs additional dialog box management operations beyond those performed
> by the **SetFocus** function. **WM_NEXTDLGCTL** updates the default pushbutton border,
> sets the default control identifier, and automatically selects the text of an edit
> control (if the target window is an edit control).
> — <https://learn.microsoft.com/en-us/windows/win32/dlgbox/wm-nextdlgctl>

Note it keys the select-all on *"if the target window is an edit control"*, not on
`DLGC_HASSETSEL` (see B).

### A3. Click-to-focus is decided by a third mechanism — **CONFIRMED**

The static control's own default window procedure decides it, per style, in
`WM_NCHITTEST`:

> | **WM_NCHITTEST** | Returns HTCLIENT if the control style is SS_NOTIFY; otherwise, returns HTTRANSPARENT. |
> — <https://learn.microsoft.com/en-us/windows/win32/controls/about-static-controls>

Corroborated from the caller's side by `WindowFromPoint`:

> If the point is over a static text control, the return value is a handle to the window
> under the static text control.
> — <https://learn.microsoft.com/en-us/windows/win32/api/winuser/nf-winuser-windowfrompoint>

And the same mechanism, same answer, in a different control class — the group box:

> | **WM_NCHITTEST** | Returns HTTRANSPARENT, if the button control is a group box. |
> — <https://github.com/MicrosoftDocs/win32/blob/docs/desktop-src/Controls/button-messages.md>
> (published as <https://learn.microsoft.com/en-us/windows/win32/controls/button-messages>,
> "Button Default Message Processing")

This is decided by the control class at runtime, per style, and is completely independent
of `WS_TABSTOP`, `WS_GROUP` and anything the dialog manager does. Three mechanisms, three
layers. Confirmed.

---

## B. `WM_GETDLGCODE` return values, and what the dialog manager does with each

The message and its full value list:
<https://learn.microsoft.com/en-us/windows/win32/dlgbox/wm-getdlgcode>

> Sent to the window procedure associated with a control. By default, the system handles
> all keyboard input to the control; the system interprets certain types of keyboard input
> as dialog box navigation keys. To override this default behavior, the control can respond
> to the **WM_GETDLGCODE** message to indicate the types of input it wants to process
> itself.

| Value | Hex | Documented meaning | What the dialog manager does with it — and where that is documented |
| --- | --- | --- | --- |
| `DLGC_WANTARROWS` | 0x0001 | "Direction keys." | Sent to the **focused** control before an arrow-key group walk; if returned, the key is passed to the control and no navigation happens. [dlgbox-programming-considerations, WS_GROUP section] |
| `DLGC_WANTTAB` | 0x0002 | "TAB key." | Sent to the **focused** control before a tab walk; if returned, TAB/SHIFT+TAB go to the control instead of moving focus. [same page, WS_TABSTOP section] |
| `DLGC_WANTALLKEYS` | 0x0004 | "All keyboard input." | Sent to the **focused** control on a letter/digit press; if returned, the key goes to the control instead of triggering a mnemonic search. [same page, Mnemonics section] |
| `DLGC_WANTMESSAGE` | 0x0004 | "All keyboard input (the application passes this message in the MSG structure to the control)." | Same numeric value as `DLGC_WANTALLKEYS`; named alongside it in the mnemonic rule (the page spells it `DLG_WANTMESSAGE` there — a typo in the live doc). |
| `DLGC_HASSETSEL` | 0x0008 | "EM_SETSEL messages." | **UNVERIFIED.** The reference defines the flag but no MS page I found states that the dialog manager acts on it. `WM_NEXTDLGCTL` documents the select-all-on-focus behaviour as keyed on *"if the target window is an edit control"*, not on this flag. The "dialog manager sends `EM_SETSEL 0,-1` to any control returning `DLGC_HASSETSEL`" story is widely repeated folklore; I could not source it. |
| `DLGC_DEFPUSHBUTTON` | 0x0010 | "Default push button." | **PARTIALLY VERIFIED.** `WM_NEXTDLGCTL` documents that focus movement *"updates the default pushbutton border, sets the default control identifier"*, and `DM_GETDEFID`/`DM_SETDEFID` exist, but no page states the manager reads this flag to decide. |
| `DLGC_UNDEFPUSHBUTTON` | 0x0020 | "Non-default push button." | Same as above — declared, mechanism not documented. |
| `DLGC_RADIOBUTTON` | 0x0040 | "Radio button." | **UNVERIFIED as a trigger.** The group walk's documented amendment is *"If the control is an automatic radio button, the system sends a BM_CLICK message to it"* — worded on the control's style, not on this returned flag. |
| `DLGC_WANTCHARS` | 0x0080 | "WM_CHAR messages." | **UNVERIFIED.** Declared; no navigation consequence documented. Buttons that accept the spacebar return it. |
| `DLGC_STATIC` | 0x0100 | "Static control." | **The one clearly documented veto.** After the group walk picks a candidate, the manager sends `WM_GETDLGCODE` *"to determine the control type"* and *"moves the input focus to control if it is not a static control."* Statics return `DLGC_STATIC` ([about-static-controls]); so does `BS_GROUPBOX` ([button-messages]). |
| `DLGC_BUTTON` | 0x2000 | "Button." | **UNVERIFIED.** Declared; no navigation consequence documented. Returned OR'd with the specific button codes. |

Per-class returns (both primary):
- Static: `WM_GETDLGCODE` → `DLGC_STATIC`.
  <https://learn.microsoft.com/en-us/windows/win32/controls/about-static-controls>
- Buttons, by style:
  `BS_AUTOCHECKBOX` → `DLGC_WANTCHARS | DLGC_BUTTON`;
  `BS_AUTORADIOBUTTON` → `DLGC_RADIOBUTTON | DLGC_BUTTON`;
  `BS_CHECKBOX` → `DLGC_WANTCHARS | DLGC_BUTTON`;
  `BS_DEFPUSHBUTTON` → `DLGC_DEFPUSHBUTTON | DLGC_BUTTON`;
  **`BS_GROUPBOX` → `DLGC_STATIC`**;
  `BS_PUSHBUTTON` → `DLGC_UNDEFPUSHBUTTON | DLGC_BUTTON`;
  `BS_RADIOBUTTON` → `DLGC_RADIOBUTTON | DLGC_BUTTON`.
  <https://learn.microsoft.com/en-us/windows/win32/controls/button-messages>

And the scoping note that shapes how much of this is load-bearing:

> Although the **DefWindowProc** function always returns zero in response to the
> **WM_GETDLGCODE** message, the window procedure for the predefined control classes return
> a code appropriate for each class. The **WM_GETDLGCODE** message and the returned values
> are useful only with user-defined dialog box controls or standard controls modified by
> subclassing.
> — <https://learn.microsoft.com/en-us/windows/win32/dlgbox/wm-getdlgcode>

**Honest reading of B.** The "creator sets a flag, control may veto or amend" shape is
real, but it is much narrower than the value table suggests. Exactly one veto is
documented (`DLGC_STATIC`, in the group walk only) and three claims-of-keystroke are
documented (`WANTTAB`, `WANTARROWS`, `WANTALLKEYS`/`WANTMESSAGE`, all queried on the
*outgoing* control, not the candidate). The remaining six values are type labels whose
consumption by the dialog manager is not stated in the documentation. Anyone citing them as
"the control amends the decision" is extrapolating.

---

## C. Hit-test transparency

### C1. What `HTTRANSPARENT` does — it walks **Z order within the thread**, not up to the parent

> | **HTTRANSPARENT** | −1 | In a window currently covered by another window in the same thread (the message will be
> sent to underlying windows in the same thread until one of them returns a code that is not
> **HTTRANSPARENT**). |
> — <https://learn.microsoft.com/en-us/windows/win32/inputdev/wm-nchittest>

Precisely: the search continues to **windows underneath in Z order that belong to the same
thread**, repeating `WM_NCHITTEST` on each, until one answers something other than
`HTTRANSPARENT`. It is not "go to the parent". In the ordinary case the parent *is* what
lies beneath a child control in Z, which is why "falls through to the parent" is the
description everyone uses and why it is usually right in practice — but the rule is Z
order, and it can land on a **sibling** beneath. The same-thread restriction is part of the
documented rule: a window on another thread terminates the walk.

`WM_NCHITTEST` also drives `WindowFromPoint`, not only mouse clicks:

> Sent to a window in order to determine what part of the window corresponds to a
> particular screen coordinate. This can happen, for example, when the cursor moves, when a
> mouse button is pressed or released, or in response to a call to a function such as
> **WindowFromPoint**. If the mouse is not captured, the message is sent to the window
> beneath the cursor. Otherwise, the message is sent to the window that has captured the
> mouse.
> — same page

Chen (**commentary**), on the same point — hit testing is a question, not a mouse event:
<https://devblogs.microsoft.com/oldnewthing/20110218-00/?p=11453>

### C2. `WS_EX_TRANSPARENT` is a *painting* style — the conflation, sorted out

The reference defines it purely as paint ordering, with **no mention of hit testing**:

> **WS_EX_TRANSPARENT** 0x00000020L — The window should not be painted until siblings
> beneath the window (that were created by the same thread) have been painted. The window
> appears transparent because the bits of underlying sibling windows have already been
> painted. To achieve transparency without these restrictions, use the **SetWindowRgn**
> function.
> — <https://learn.microsoft.com/en-us/windows/win32/winmsg/extended-window-styles>

Chen restates it as *"it doesn't mean transparent; it means paint over siblings"*
(**commentary**):
<https://devblogs.microsoft.com/oldnewthing/20121217-00/?p=5823>

The connection to hit testing is real but indirect, and it lives in the
`ChildWindowFromPoint*` family rather than in the message loop. `ChildWindowFromPointEx`
takes `CWP_SKIPTRANSPARENT` (0x0004) — *"Skips transparent child windows"* — but **the
reference never defines what "transparent" means for that flag**:
<https://learn.microsoft.com/en-us/windows/win32/api/winuser/nf-winuser-childwindowfrompointex>

Chen supplies the definition (**commentary**, and the only source I found for it):

> The `...ChildWindowFromPoint...` functions define *transparent* as *has the
> `WS_EX_TRANSPARENT` extended window style*.

whereas `WindowFromPoint` defines transparent as *returns `HTTRANSPARENT` to
`WM_NCHITTEST`* — and skips the `WM_NCHITTEST` query entirely for cross-thread windows,
treating them as opaque.
<https://devblogs.microsoft.com/oldnewthing/20101230-00?p=11873>

**So there are two different senses of "transparent" in the same API, keyed off two
different bits of state, and which one applies depends on which function you called.**
`WS_EX_TRANSPARENT` is a creator-set flag consulted by the `ChildWindowFromPoint*` helpers;
`HTTRANSPARENT` is a runtime answer consulted by real mouse routing and by
`WindowFromPoint`. Real mouse input follows the `HTTRANSPARENT` path. Where docs and
folklore disagree: folklore says "set `WS_EX_TRANSPARENT` to make clicks pass through" —
the documentation does not support that for actual mouse message routing, and returning
`HTTRANSPARENT` is the mechanism that does.

### C3. Can a window opt out of hit testing entirely?

Yes — by returning `HTTRANSPARENT` from `WM_NCHITTEST`. That is the opt-out, and it is a
per-message runtime answer, not a creation-time flag. There is no `WS_*` style that means
"not hit-testable" for mouse routing.

"Fall to the parent" is the wrong description of the mechanism. The correct description is
**"the hit test continues to whatever lies beneath in Z order within the same thread, and
asks it the same question"**. That happens to be the parent in the common child-control
case, which is why the folk description survives.

`HTNOWHERE` (0) is *not* the same thing — it means "screen background or a dividing line
between windows" and terminates the search; `HTERROR` (−2) is the same as `HTNOWHERE` plus
a system beep from `DefWindowProc`.
— <https://learn.microsoft.com/en-us/windows/win32/inputdev/wm-nchittest>

### C4. Disabled windows (`WS_DISABLED`) — skipped during routing, not swallowed

> **WS_DISABLED** 0x08000000L — The window is initially disabled. A disabled window cannot
> receive input from the user.
> — <https://learn.microsoft.com/en-us/windows/win32/winmsg/window-styles>

> If a child window is disabled, it is ignored when the system tries to determine which
> window should receive mouse messages. […] A disabled control cannot receive the keyboard
> focus, nor can a user gain access to it. […] If a window is already disabled, its child
> windows are implicitly disabled, although they are not sent a **WM_ENABLE** message.
> — <https://learn.microsoft.com/en-us/windows/win32/api/winuser/nf-winuser-enablewindow>

> The **WindowFromPoint** function does not retrieve a handle to a hidden or disabled
> window, even if the point is within the window.
> — <https://learn.microsoft.com/en-us/windows/win32/api/winuser/nf-winuser-windowfrompoint>

So disabled is skipped during the routing search — a *third* mechanism again, distinct
from both `HTTRANSPARENT` and `WS_EX_TRANSPARENT`, and one that gates keyboard focus at
the same time (it is the only documented `SetFocus` gate). Note the asymmetry: disabled
gates all three axes at once; hit-transparency gates only clicking; `WS_TABSTOP` gates only
walking.

**UNVERIFIED:** whether the click on a disabled child lands on the parent or is dropped
entirely. "Ignored when the system tries to determine which window should receive mouse
messages" is compatible with either (the routing search continues past it and finds the
parent, which is beneath in Z). I found no primary source stating the outcome.

### C5. Which classes are hit-transparent by default — documented, not folklore

Both cases I found are documented in Microsoft's own default-message-processing tables:

| Class / style | `WM_NCHITTEST` default | Source |
| --- | --- | --- |
| `STATIC` without `SS_NOTIFY` | `HTTRANSPARENT` | [about-static-controls](https://learn.microsoft.com/en-us/windows/win32/controls/about-static-controls) |
| `STATIC` with `SS_NOTIFY` | `HTCLIENT` | same |
| `BUTTON` with `BS_GROUPBOX` | `HTTRANSPARENT` | [button-messages](https://learn.microsoft.com/en-us/windows/win32/controls/button-messages) |
| all other button styles | not overridden (i.e. `DefWindowProc` → `HTCLIENT` in the client area) | same table lists no other case |

`SS_NOTIFY` is exactly the switch: *"Sends the parent window STN_CLICKED, STN_DBLCLK,
STN_DISABLE, and STN_ENABLE notification codes when the user clicks or double-clicks the
control."*
— <https://learn.microsoft.com/en-us/windows/win32/controls/static-control-styles>

Note what this means: **a static with `SS_NOTIFY` is clickable but still not walkable and
still vetoed by the group walk.** Its `WM_GETDLGCODE` still returns `DLGC_STATIC`. That is
a live, shipping example of clickability and walkability disagreeing in a single control,
decided by two unrelated bits of state. The comctl32 classes (list view, tree view, tab)
have no documented `WM_NCHITTEST` override — **UNVERIFIED** whether any are transparent.

---

## D. `WS_GROUP` and arrow-key navigation

### D1. What it marks

> **WS_GROUP** 0x00020000L — The window is the first control of a group of controls. The
> group consists of this first control and all controls defined after it, up to the next
> control with the **WS_GROUP** style. The first control in each group usually has the
> **WS_TABSTOP** style so that the user can move from group to group. The user can
> subsequently change the keyboard focus from one control in the group to the next control
> in the group by using the direction keys. You can turn this style on and off to change
> dialog box navigation. To change this style after a window has been created, use the
> **SetWindowLong** function.
> — <https://learn.microsoft.com/en-us/windows/win32/winmsg/window-styles>

> The style marks the beginning of a group of controls. […] In general, the first control
> in a group must have the **WS_GROUP** style and all other controls in the group must not
> have this style. **All controls in the group must be contiguous — that is, they must have
> been created consecutively with no intervening controls.**
> — <https://learn.microsoft.com/en-us/windows/win32/dlgbox/dlgbox-programming-considerations>

### D2. Flat and order-based, not a containment relationship — **CONFIRMED**

`WS_GROUP` is a bit on the *first sibling* of a run, and group membership is "everything
from here until the next window carrying the bit", in **creation order** (template order).
There is no group object, no parent, no handle for "the group". `GetNextDlgGroupItem`
navigates entirely by walking that order:

> The **GetNextDlgGroupItem** function searches controls in the order (or reverse order)
> they were created in the dialog box template. The first control in the group must have
> the **WS_GROUP** style; all other controls in the group must have been consecutively
> created and must not have the **WS_GROUP** style.
> — <https://learn.microsoft.com/en-us/windows/win32/api/winuser/nf-winuser-getnextdlggroupitem>

The wrap behaviour is derived, not stored — the function reverses direction to find the
run's start and comes back:

> When searching for the next control, the function returns the first control it locates
> that is visible, not disabled, and does not have the **WS_GROUP** style. If it encounters
> a control having the **WS_GROUP** style, the function reverses the search, locates the
> first control having the **WS_GROUP** style, and returns this control if it is visible and
> not disabled.
> — same page

Note also that a **group box** (`BS_GROUPBOX`) is *not* the group. It is a sibling control
that draws a rectangle: *"The sole purpose of a group box is to organize controls related
by a common purpose. […] Because a group box cannot be selected, it has no check state,
focus state, or push state."*
— <https://learn.microsoft.com/en-us/windows/win32/controls/button-types-and-styles>
The visual container and the navigational group are unrelated constructs that happen to be
drawn on top of each other.

### D3. The two levels, and the one recursion escape hatch

- **TAB / SHIFT+TAB** walk `WS_TABSTOP` across the whole dialog in creation order.
- **Arrow keys** walk within the `WS_GROUP` run.
- Convention wires them together: *"Generally, the first control in the group combines the
  WS_GROUP and WS_TABSTOP styles so that the user can move from group to group by using the
  TAB key."* — dlgbox-programming-considerations

Hierarchy enters through exactly one flag, and it is opt-in per container:

> **WS_EX_CONTROLPARENT** 0x00010000L — The window itself contains child windows that
> should take part in dialog box navigation. If this style is specified, the dialog manager
> recurses into children of this window when performing navigation operations such as
> handling the TAB key, an arrow key, or a keyboard mnemonic.
> — <https://learn.microsoft.com/en-us/windows/win32/winmsg/extended-window-styles>

Both `GetNextDlgTabItem` and `GetNextDlgGroupItem` document the same recursion. So Win32's
tree-awareness in navigation is a single per-window "descend into me" bit, not a
containment model.

### D4. Does Win32 remember your place in a group?

**Yes for radio-button groups — and it does it by mutating the style bit.**

> If the group contains radio buttons, the application should apply the **WS_TABSTOP** style
> only to the first control in the group. **The system automatically moves the style when the
> user moves between controls in the group. This ensures that the input focus will always be
> on the most recently selected control when the user moves to the group using the TAB key.**
> — <https://learn.microsoft.com/en-us/windows/win32/dlgbox/dlgbox-programming-considerations>

That is the whole memory mechanism: there is no separate "last active member" field. The
`WS_TABSTOP` bit *is* the remembered position, moved from sibling to sibling. Tab into the
group and you land where you left, because the flag physically moved. It also means the
memory and the tab-eligibility are the same piece of state — froth should notice that
Win32 did not build a second thing.

**Yes at the dialog level, for whole-dialog activation** — one saved HWND in the dialog's
default window procedure:

> | **WM_ACTIVATE** | Restores the input focus to the control identified by the previously saved handle if the dialog box is activated. Otherwise, the procedure saves the handle to the control having the input focus. |
> | **WM_SETFOCUS** | Sets the input focus to the control identified by a previously saved control window handle. If no such handle exists, the procedure sets the input focus to the first control in the dialog box template that is visible, not disabled, and has the **WS_TABSTOP** style. If no such control exists, the procedure sets the input focus to the first control in the template. |
> | **WM_SHOWWINDOW** | Saves a handle to the control having the input focus if the dialog box is being hidden […] |
> | **WM_SYSCOMMAND** | Saves a handle to the control having the input focus if the dialog box is being minimized […] |
> — same page, "Dialog Box Default Message Processing"

**UNVERIFIED:** where that handle physically lives. `DLGWINDOWEXTRA` reserves per-dialog
extra bytes (*"it must set the cbWndExtra member to the DLGWINDOWEXTRA, which is the number
of extra bytes the system requires for each dialog box"* — same page) and the saved focus
HWND plausibly lives there, but no doc says so.

**No for the ordinary non-radio group.** Nothing in `GetNextDlgGroupItem` or the
`WS_GROUP` prose stores a last-visited member; the walk is re-derived from creation order
each time. The group-memory behaviour users perceive is the radio-button `WS_TABSTOP` trick.

**Controls remember a selection independently of focus — a distinct, per-control concept.**

- List box: `LB_GETCARETINDEX` — *"Retrieves the index of the item that has the focus in a
  multiple-selection list box. **The item may or may not be selected.**"*
  <https://learn.microsoft.com/en-us/windows/win32/controls/lb-getcaretindex>
  So a list box carries an internal focus cursor separate from selection, and separate
  again from whether the HWND has keyboard focus.
- List view: `LVIS_FOCUSED` — *"The item has the focus, so it is surrounded by a standard
  focus rectangle. Although more than one item may be selected, only one item can have the
  focus."* `LVIS_SELECTED` — *"The item is selected. The appearance of a selected item
  depends on whether it has the focus […]"*
  <https://learn.microsoft.com/en-us/windows/win32/controls/list-view-item-states>

This is Win32's real answer to "a container that remembers your place": **the container is
a single HWND with internal item state**, not a set of sibling HWNDs the navigator has to
remember. `WS_GROUP` is for the *other* case — genuinely separate controls that should
arrow-navigate together — and it deliberately stores almost nothing.

---

## E. Which axes are actually independent?

| Axis | Mechanism | Who decides | When | Independent of the others? |
| --- | --- | --- | --- | --- |
| **walkable** (TAB) | `WS_TABSTOP` style bit + `GetNextDlgTabItem`'s visible/enabled filter; `WS_EX_CONTROLPARENT` for recursion; `DLGC_WANTTAB` from the *focused* control to suppress the walk | Creator sets the bit; the outgoing control can claim the key; the system itself moves the bit between radio siblings | Creation, mutable via `SetWindowLong`; queried per keystroke | Yes. Only overlap: disabled/invisible also excludes. |
| **walkable** (arrows) | `WS_GROUP` run + `GetNextDlgGroupItem`; `DLGC_WANTARROWS` from the focused control; **`DLGC_STATIC` from the candidate vetoes the landing** | Creator sets `WS_GROUP`; candidate control class vetoes via `DLGC_STATIC` | Creation, mutable; both codes queried per keystroke | Yes — a separate axis from TAB, with its own flag *and* its own veto. |
| **focusable by name** (`SetFocus`) | No style bit. Gated only by: valid HWND, same input queue, not `WS_DISABLED`, no CBT-hook veto | The system, on universal rules | Call time | Yes — and it is the *odd one out*: it is universal-by-default, the opposite of the other two. |
| **clickable / hit-testable** | `WM_NCHITTEST` return value; `HTTRANSPARENT` continues the search into Z-order-beneath, same-thread. Disabled windows skipped by the router | The control class, at runtime, per style (`SS_NOTIFY`, `BS_GROUPBOX`) | Every hit test | Yes. `WS_EX_TRANSPARENT` is a *fourth*, different thing consulted only by `ChildWindowFromPoint*`. |

**Verdict on independence: four axes, not three** — TAB-walkable and arrow-walkable are
separately flagged and separately vetoed in Win32, and there is a cross-cutting
`WS_DISABLED` that shuts off all of them at once. Each axis is controlled by a different
kind of authority:

- `WS_TABSTOP` / `WS_GROUP` / `WS_EX_CONTROLPARENT` — **creator-set, per window, mutable.**
- `WM_GETDLGCODE` / `WM_NCHITTEST` — **class-decided, answered at runtime**, which is what
  lets `SS_NOTIFY` flip clickability without touching tabbability.
- `SetFocus` — **no per-window authority at all**, only universal gates.
- `WS_DISABLED` — **creator-set, cross-cutting all three**, the single mechanism that gates
  focus, keyboard walk and mouse routing together.

The Win32 answer to froth's question, stated plainly: *walkable* is a flag, *focusable* is
not a flag (it is the default), and *clickable* is not a flag either — it is a question the
thing answers each time it is asked.

---

## Verdict: what transfers

**Transfers well:**

1. **Programmatic focus with no gate at all.** Win32 shipped for thirty years with
   `SetFocus` accepting essentially any enabled window, and the consequence — focus can
   land somewhere useless — was never considered severe enough to add a bit. That is a
   real data point for "focusable by name should stay implicit and universal".
2. **Walkability as a creator-set flag, separate from everything else.** `WS_TABSTOP` never
   grew into a general focus policy, and it was worth keeping distinct.
3. **Clickability decided by the thing being clicked, per its configuration, at hit time.**
   `SS_NOTIFY` is the crisp existence proof: one control class, one style bit, clickable or
   not — while tabbability is unchanged and focusability is unchanged. If froth wants one
   fact to justify a third axis, this is it.
4. **The one-bit veto in the arrow walk.** "Creator flags the candidate set, the candidate
   itself can decline" is real, works, and is narrow — one flag, one walk. Worth noting
   that Win32 did *not* generalise it: the tab walk has no veto at all.
5. **Remembering your place by moving the eligibility marker.** The radio-group trick is
   frugal: no second data structure, no "last focused child" map — the tab-stop bit itself
   is the memory. Whether that is elegant or a hack is arguable, but it is one fewer piece
   of state.
6. **Disabled as the cross-cutting off switch.** One flag that gates all axes at once,
   documented consistently across `SetFocus`, `GetNextDlgTabItem`, `GetNextDlgGroupItem` and
   mouse routing. That symmetry is cheap and worth copying regardless of how many axes
   froth lands on.

**Does not transfer, and why:**

1. **Hit-transparency is Z-order fallthrough between OS windows on one thread.** froth has
   no HWND-level Z order, no cross-thread hit-test boundary, and no window manager
   re-asking each window beneath. In a terminal compositor "the click passes through to
   whatever is under it" is a property of froth's own tree walk, not of a message the
   surface answers. The *shape* (the thing declines, the search continues) transfers; the
   *mechanism* (Z-order sibling walk, same-thread only) does not, and the folk description
   "falls through to the parent" is wrong even in Win32.
2. **Two meanings of "transparent" in one API.** `HTTRANSPARENT` versus
   `WS_EX_TRANSPARENT` is a naming accident that has confused people for decades, made
   worse by `ChildWindowFromPointEx` documenting `CWP_SKIPTRANSPARENT` without defining
   "transparent". This is a cautionary tale, not a design to copy.
3. **The dialog manager being opt-in.** `IsDialogMessage` in your message loop or you get
   no keyboard navigation at all. That is an artifact of Win32 needing to layer a
   navigation model onto a window system that predated it. froth is designing the
   navigation model and the surface tree together and has no reason to make one optional.
4. **`WS_GROUP`'s flat, order-based grouping.** Confirmed flat: a bit on the first sibling
   of a contiguous creation-order run, with `WS_EX_CONTROLPARENT` bolted on later to get any
   recursion at all. froth has a real tree, so the whole "contiguous, no intervening
   controls" constraint — and the group-box-is-not-the-group split — is a workaround for an
   absence froth does not have.
5. **Non-client areas.** More than half of `WM_NCHITTEST`'s return values are captions,
   borders, resize corners and minimise buttons. That vocabulary has no froth analogue and
   should not shape how froth thinks about the message.
6. **Two walks, two flags, two vetoes.** Win32 has *four* axes partly because arrow-key
   group navigation was retrofitted alongside tab navigation with its own flag and its own
   veto rule. froth should read that as evidence that axes multiply when navigation modes
   are added later — not as an endorsement of four.

**The honest caveat.** Two things in this survey are UNVERIFIED and matter: (a) whether
`SetFocus` on a static text control actually succeeds, which is the crux of A1 and which
the static-control overview verbally contradicts; and (b) what most `DLGC_*` values
actually cause the dialog manager to do, which is the crux of B. Six of eleven `DLGC_*`
values are documented as type labels with no stated consumer. If froth's decision record
wants to lean on "the control amends the creator's flag", it can lean on `DLGC_STATIC` in
the arrow walk and on `SS_NOTIFY`/`WM_NCHITTEST`. It should not lean on the rest of the
table.
