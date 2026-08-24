# When a modal is open, what happens to the shortcuts declared outside it

Eight systems, on one narrow question: the **key-press-to-command resolution step**, and how
modality changes it. Not parenting (`docs/research/modal-parenting.md`), not bubbling
([`event-bubbling.md`](./event-bubbling.md)), not command identity ([`command-systems.md`](./command-systems.md)).

**Core finding up front:** the mechanism differs wildly, but the *authority* barely does.
In six of eight systems the **framework** decides what survives a modal and the shortcut's
declarer has no say at all. Only two — Textual and Flutter — put the decision on the
declaring binding, which is where froth's `SurvivesModal` puts it. And of the three systems
that curate an explicit survivor list, all three start it with quit.

---

## 1. Win32 — loop replacement, total

The system modal loop is a different message pump, and your accelerator table is not in it.

> "To process messages for the modal dialog box, the system starts its own message loop,
> taking temporary control of the message queue for the entire application. When the system
> retrieves a message that is not explicitly for the dialog box, it dispatches the message to
> the appropriate window."
> — [About Dialog Boxes](https://learn.microsoft.com/en-us/windows/win32/dlgbox/about-dialog-boxes)

Your `TranslateAccelerator` call lives in *your* `GetMessage` loop, which is not running.
Microsoft never states the negative; the confirmation is Wine's `DIALOG_DoDialogBox`, the
function behind all four `DialogBox*` entry points, whose entire loop body is:

```c
if (!(dlgInfo->flags & DF_END) && !IsDialogMessageW( hwnd, &msg ))
{
    TranslateMessage( &msg );
    DispatchMessageW( &msg );
}
```
— [wine/dlls/user32/dialog.c](https://raw.githubusercontent.com/wine-mirror/wine/master/dlls/user32/dialog.c)

No `HACCEL` anywhere in the function. *[Wine is a clean-room reimplementation — authoritative
for observable behaviour, not Microsoft's source. Flagged as strong-but-not-first-party;
no Raymond Chen post was found that states the omission directly.]*

**What replaces accelerators** is a fixed, non-extensible table — the dialog keyboard
interface: TAB/SHIFT+TAB (tabstops), arrows (within a group), mnemonics, ENTER→IDOK,
ESC→IDCANCEL ([Dialog Box Programming
Considerations](https://learn.microsoft.com/en-us/windows/win32/dlgbox/dlgbox-programming-considerations)).
Note the vocabulary trap: Win32's dialog-manager "accelerator" is the `&`-mnemonic, unrelated
to `HACCEL` tables.

**Modality itself does not gate accelerators.** The owner is disabled (`EnableWindow`), which
blocks input routing, but `TranslateAccelerator` has no modality predicate: forced from a
`WH_MSGFILTER` hook it still delivers the `WM_COMMAND` to a disabled window. Only the
menu-init notifications are suppressed —
[TranslateAcceleratorW](https://learn.microsoft.com/en-us/windows/win32/api/winuser/nf-winuser-translateacceleratorw)
lists "The window is disabled" among conditions under which `WM_INITMENU` is *not* sent, and
says nothing about the command itself.

**MFC is the opposite answer in the same OS.** `CWnd::RunModalLoop` pumps through MFC's own
`PreTranslateMessage` chain, and `CWinApp::PreTranslateMessage`'s "default implementation
performs accelerator-key translation"
([CWinApp](https://learn.microsoft.com/en-us/cpp/mfc/reference/cwinapp-class?view=msvc-170)).
Under MFC, accelerators keep working during `DoModal`.

| | |
|---|---|
| Mechanism | Loop replacement. Coarse, all-or-nothing. |
| Escapes | Nothing from the accelerator table. Only the fixed dialog keyboard interface, or what an app rescues via `WH_MSGFILTER` / a modeless dialog with its own loop. |
| Decides | The *dialog's creator*, by picking `DialogBox` vs `CreateDialog`. The declarer has no say — an accelerator entry carries no scope field. |
| Help reflects it | **UNVERIFIED.** No MS doc found. Mechanically the frame's menu is unreachable anyway. |
| Resolve vs reach | Inverted: resolution never happens. Where it is forced, delivery is unconditional. No separation. |

---

## 2. Qt — per-shortcut scoping, and the scope enum is not the modality mechanism

The enum, from the qdoc source (the rendered `doc.qt.io` page emits the *focus-policy*
descriptions against these names — a doc-generation bug; do not trust it):

> **Qt::WidgetShortcut** — "The shortcut is active when its parent widget has focus."
> **Qt::WidgetWithChildrenShortcut** — "The shortcut is active when its parent widget, or any of
> its children has focus. Children which are top-level widgets, except pop-ups, are not
> affected by this shortcut context."
> **Qt::WindowShortcut** — "The shortcut is active when its parent widget is a logical subwidget
> of the active top-level window."
> **Qt::ApplicationShortcut** — "The shortcut is active when one of the applications windows are
> active."
> — [qnamespace.qdoc](https://raw.githubusercontent.com/qt/qtbase/dev/src/corelib/global/qnamespace.qdoc)

**The answer to the case that was to be nailed down: no. `Qt::ApplicationShortcut` does not
fire while an application-modal `QDialog` is open.** Modality is a separate predicate bolted
onto the context matcher, not a value of the scope enum:

```cpp
    if (context == Qt::ApplicationShortcut)
        return QApplicationPrivate::tryModalHelper(w, nullptr); // true, unless w is shadowed by a modal dialog
```
— [qshortcut_widgets.cpp](https://raw.githubusercontent.com/qt/qtbase/dev/src/widgets/kernel/qshortcut_widgets.cpp),
`correctWidgetContext`. The `WindowShortcut` branch ends with the identical call.
`WidgetShortcut`/`WidgetWithChildrenShortcut` skip it and compare against
`QApplication::focusWidget()`, which is already inside the dialog — modality enforced
transitively.

`tryModalHelper` is `!isBlockedByModal(widget->window())`, with one unconditional exemption:
"the active popup widget always gets the input event"
([qapplication.cpp](https://raw.githubusercontent.com/qt/qtbase/dev/src/widgets/kernel/qapplication.cpp)).
`Qt::Tool` windows escape too, by an explicit redirect commented *"if a floating tool window
is active, keep shortcuts on the parent working"* — which is the mechanism behind
[forum.qt.io/topic/43570](https://forum.qt.io/topic/43570/qaction-still-triggered-when-modal-dialog-executed),
a report of a MainWindow `QAction` firing under a modal, reproducible only with
`QDialog(this, Qt::Tool)`.

**So the docs' scope enum and the actual modality rule are two different things, and the enum
documentation has never been amended to mention modality.** That is the regret, such as it is.
The confusion is real but thinly sourced:
[forum.qt.io/topic/91429](https://forum.qt.io/topic/91429/possible-to-allow-qshortcut-to-work-with-modal-qdialogs)
asks exactly the ApplicationShortcut-under-modal question; the accepted workaround is to
**reparent the shortcut onto the dialog** before `exec()` — change the owner, because there is
no way to change the policy. *[UNVERIFIED: `bugreports.qt.io` now redirects to a JS-only Jira
shell. Search surfaced QTBUG-60983 ("menu actions are triggered despite a modal dialog being
open") and QTBUG-46287 as leads; no issue body was readable. Treat as leads, not citations.]*

`QDialog::exec()`'s nested loop is the *same* `QEventLoop` machinery — it suppresses nothing.
All the suppression is the predicate.

| | |
|---|---|
| Mechanism | Context predicate, per shortcut, evaluated at match time on every key press. |
| Escapes | Popups (unconditional), `Qt::Tool` windows via parent-redirect, anything owned inside the dialog. All framework-hardcoded. |
| Decides | The framework. `ShortcutContext` picks *which* predicate runs; every top-level-scoped one ends in `tryModalHelper`. No declarer opt-out exists. |
| Help reflects it | **UNVERIFIED** for shortcut hints. Qt does disable the whole *platform* menu on macOS while a modal is up ("only the QPA menu is disabled, not the QMenu"). |
| Resolve vs reach | **Split, and modality lands on the resolve side.** `QKeySequence::matches()` still returns `ExactMatch`; `correctContext()` drops the entry before `dispatchEvent`, so no `QShortcutEvent` is ever constructed and `tryShortcut` returns false — the key falls through as an ordinary press. |

---

## 3. Cocoa / AppKit — two mechanisms, and the split is exactly froth's

**Event delivery is loop replacement.** `runModal(for:)`:

> "While the app is in that loop, it does not respond to any other events (including mouse,
> keyboard, or window-close events) unless they are associated with the window."
> — [NSApplication.runModal(for:)](https://developer.apple.com/documentation/appkit/nsapplication/runmodal(for:))

**The escape from that is a framework mechanism, not a convention.** From the archived
[How Modal Windows Work](https://developer.apple.com/library/archive/documentation/Cocoa/Conceptual/WinPanel/Concepts/UsingModalWindows.html):

> "The normal behavior of a modal window or session is to exclude all other windows and panels
> from receiving events. However, windows and panels that serve as general auxiliary controls
> (such as menus and the Font panel) can override the `worksWhenModal` method to return `YES`.
> This allows the window to receive mouse and keyboard events even when a modal window is
> present, **enabling the user to use menu key equivalents (such as Cut and Paste)**."

And `worksWhenModal` is "By default … `false`. **Only subclasses of `NSPanel` should override
this default.**"
([NSWindow.worksWhenModal](https://developer.apple.com/documentation/appkit/nswindow/workswhenmodal)).
The menu bar keeps working because AppKit's *own* menu window is such a panel. The application
opts into nothing.

**Enablement is a second, independent gate, and this is the crux.** Resolution order —
[Handling Key Events](https://developer.apple.com/library/archive/documentation/Cocoa/Conceptual/EventOverview/HandlingKeyEvents/HandlingKeyEvents.html):

> "`NSApp` dispatches events it recognizes as potential key equivalents … in its `sendEvent:`
> method. It sends a `performKeyEquivalent:` message to the key `NSWindow` object … **If no
> object in the view hierarchy handles the key equivalent, `NSApp` then sends
> `performKeyEquivalent:` to the menus in the menu bar.**"

Then validation decides whether it fires:

> "When you use automatic menu enabling, `NSMenu` updates the status of every menu item whenever
> a user event occurs. To update the status of a menu item, `NSMenu` first determines the target
> of the item and then determines whether the target implements `validateMenuItem:` or
> `validateUserInterfaceItem:` … **If there is no object in the responder chain that implements
> the item's action, the item is disabled.**"
> — [Enabling Menu Items](https://developer.apple.com/library/archive/documentation/Cocoa/Conceptual/MenuList/Articles/EnablingMenuItems.html)

That page mentions modality **nowhere**. The modality rule appears only in a forum reply from
an Apple Frameworks Engineer
([developer.apple.com/forums/thread/746760](https://developer.apple.com/forums/thread/746760)):

> "if the app is in a modal state, AppKit skips over item validation and always disables the
> item. This is meant to provide automatic disabling of main menu items while a modal window is
> the main window."
> …
> "If you implement the method `-(BOOL)worksWhenModal` to return YES on the target object for
> your menu item, AppKit will allow the menu item to be validated, and enabled if the target
> implements the item's action."

So `worksWhenModal` does double duty: on a **window** it gates event delivery (documented), on a
**menu item's target object** it gates validation (undocumented outside that reply).

**Why Quit / Hide / Cut / Copy / Paste stay live: framework, in two layers, neither of them a
convention.** (a) the menu bar's window is a `worksWhenModal` panel, so keys reach it; (b) those
items are nil-target ("First Responder") items whose responder chain terminates at `NSApp`,
which implements `terminate:` and `hide:` — so they validate and enable.
*[UNVERIFIED: no Apple source reconciles "always disables the item" with the observed fact that
these stay live. The defensible reading is that the modal disable applies to items whose
validation would reach outside the modal window's chain. Treat the exact boundary as unverified;
the current HIG "Modality" page is JS-rendered and yielded no text.]*

| | |
|---|---|
| Mechanism | Loop replacement at the event layer, **plus** a context predicate at the validation layer. |
| Escapes | Windows returning `YES` from `worksWhenModal` — for the menu bar, AppKit's own. Individual items via `worksWhenModal` on their target. Framework-declared. |
| Decides | The framework. The modal declares only that it is modal; the shortcut declarer declares nothing about modality. |
| Help reflects it | **Yes, and by the same mechanism** — the menu *is* the resolution surface, automatic enabling greys out what will not work, and it re-runs "whenever a user event occurs". The strongest case in the survey. |
| Resolve vs reach | **Explicitly split, and modality lands on the reach side.** `performKeyEquivalent:` resolves the keystroke to an item and selector; `validateMenuItem:` then decides whether it fires. The exact inverse of Qt. |

---

## 4. GTK 4 — propagation-root replacement; `GtkShortcutScope` is not involved

`gtk_window_set_modal` is two layers, and the doc understates it:

> "Modal windows prevent interaction with other windows in the same application."
> — [set_modal](https://docs.gtk.org/gtk4/method.Window.set_modal.html)

Implementation: `gdk_toplevel_set_modal` (a compositor hint) **plus `gtk_grab_add (widget)`**.
`get_modal`'s own doc is more honest: "true if the window is set to be modal and **establishes a
grab when shown**."

The grab is enforced before any controller runs, in `gtk_main_do_event`:

```c
  /* If the grab widget is an ancestor of the event widget
   * then we send the event to the original event widget.
   * This is the key to implementing modality. This also applies
   * across windows that are directly or indirectly transient-for
   * the modal one.
   */
  ...
      handled_event = gtk_propagate_event (grab_widget, event);
```
— [gtk/gtkmain.c](https://gitlab.gnome.org/GNOME/gtk/-/raw/main/gtk/gtkmain.c)

**This is neither truncation nor GDK-level blocking: the propagation root is swapped.** The
event still arrives at GTK for the other window, but `gtk_propagate_event` is handed the modal
window instead, so the other window's shortcut controllers — capture phase included — never
execute. Scope is the window *group*, not the process ("limiting the effect of GTK grabs and
modality to windows in the same group" —
[gtkwindowgroup.c](https://gitlab.gnome.org/GNOME/gtk/-/raw/main/gtk/gtkwindowgroup.c)).

**`GtkShortcutScope.GLOBAL` means global to one window.** `gtk_shortcut_controller_root`
resolves GLOBAL to `gtk_widget_get_root(...)` cast to `GtkShortcutManager`, and it is
`GtkWindow` that implements that interface
([gtkshortcutcontroller.c](https://gitlab.gnome.org/GNOME/gtk/-/raw/main/gtk/gtkshortcutcontroller.c),
[gtkwindow.c](https://gitlab.gnome.org/GNOME/gtk/-/raw/main/gtk/gtkwindow.c)). So a GLOBAL-scope
shortcut on a different window does **not** fire under a modal — its manager is never reached.
The name is misleading. This extends [`event-bubbling.md`](./event-bubbling.md)'s GTK section: the scope enum is
orthogonal to modality and never consulted for it.

**`app.` accelerators survive by replication, not by exemption.** `gtk_window_set_application`
gives *every* window its own capture-phase, GLOBAL-scope controller backed by the shared accel
model:

```c
priv->application_shortcut_controller = gtk_shortcut_controller_new_for_model (gtk_application_accels_get_shortcuts (app_accels));
gtk_event_controller_set_propagation_phase (priv->application_shortcut_controller, GTK_PHASE_CAPTURE);
gtk_shortcut_controller_set_scope (..., GTK_SHORTCUT_SCOPE_GLOBAL);
```
— gtkwindow.c

The registry is shared; the dispatch site is replicated per window. So `app.quit` fires under a
modal **only if the modal was added to the `GtkApplication`** — a `GtkApplicationWindow` always
is; a plain `GtkDialog` given only a transient parent is not, and then app accels do not fire at
all while it grabs. *[Read from source; not verified at runtime. `gtk_window_set_transient_for`
does not propagate the application.]*

One place GTK does express modality as enablement: `update_window_actions`, called from
`set_modal`, disables `window.minimize` and `window.toggle-maximized` on a modal window.

| | |
|---|---|
| Mechanism | Propagation-root replacement — closest in shape to loop replacement, decided per event before any controller runs. |
| Escapes | Descendants of the grab widget, and windows transient-for the modal (`is_transient_for` branch). Framework-declared via ancestry; there is no modal-exempt flag anywhere in `GtkShortcut`. |
| Decides | The framework, keyed on the grab. The modal's only input is a boolean. |
| Help reflects it | **No.** `gtkshortcutswindow.c` contains zero occurrences of "modal"; it renders a static `GtkBuilder` description with no notion of what currently resolves. Sharpest contrast with Cocoa. |
| Resolve vs reach | Split exists (`gtk_action_muxer_query_action`'s `enabled` gate in `gtk_named_action_activate`), but modality is enforced earlier, so for cross-window shortcuts the distinction never arises. |

---

## 5. VS Code — context predicates for quick input, an explicit allowlist for real modals

There is one `keydown` listener on `window` for the whole workbench, and one flat globally
ordered rule list filtered by `when`:

> "Rules are evaluated from **bottom** to **top**. The first rule that matches both the `key` and
> `when` clause, is accepted." … "If a rule is found, no more rules are processed."
> — [Key Bindings](https://code.visualstudio.com/docs/configure/keybindings)

`inQuickOpen` is a plain global boolean toggled on show/hide:

```ts
this._register(this.onShow(() => this.inQuickInputContext.set(true)));
this._register(this.onHide(() => this.inQuickInputContext.set(false)));
```
— [quickInputService.ts](https://github.com/microsoft/vscode/blob/main/src/vs/workbench/services/quickinput/browser/quickInputService.ts);
documented as "The Quick Open dropdown has focus."
([when-clause contexts](https://code.visualstudio.com/api/references/when-clause-contexts)).

**Nothing more privileged is involved.** The quick input's only `window` keydown listeners are a
capture-phase one that records modifier state and an input-box one that updates ARIA — neither
calls `preventDefault` or `stopPropagation`. So the keybinding service sees every keystroke, and
**any binding with no `when`, or whose `when` still evaluates true, fires while quick open is
up**. The second half of the suppression is context *lookup*, not dispatch truncation:
`getContext(target)` walks up from the event target for a `data-keybinding-context` attribute,
so `editorTextFocus` goes false simply because the target is no longer under the editor's scoped
context node
([contextKeyService.ts](https://github.com/microsoft/vscode/blob/main/src/vs/platform/contextkey/browser/contextKeyService.ts)).

*[`quickInputFocus` is not on the documented context-key page — UNVERIFIED. The real source key
is `inQuickInput`.]*

**A true modal dialog needed a different mechanism, and that is the finding.** Custom in-window
dialogs use capture-phase interception plus resolve-then-veto against a hardcoded list:

```ts
keyEventProcessor: (event: StandardKeyboardEvent) => {
    const resolved = keybindingService.softDispatch(event, layoutService.activeContainer);
    if (resolved.kind === ResultKind.KbFound && resolved.commandId) {
        if (!allowableCommands.has(resolved.commandId) && !shouldPassThroughCommand?.(resolved.commandId, event)) {
            EventHelper.stop(event, true);
        }
    }
},

const defaultDialogAllowableCommands = new Set([
	'workbench.action.quit',
	'workbench.action.reloadWindow',
	'copy', 'cut',
	'editor.action.selectAll',
	'editor.action.clipboardCopyAction',
	'editor.action.clipboardCutAction',
	'editor.action.clipboardPasteAction'
]);
```
— [dialog.ts](https://github.com/microsoft/vscode/blob/main/src/vs/workbench/browser/parts/dialogs/dialog.ts)

Eight commands, and the list opens with quit. Native dialogs (`window.dialogStyle` default on
desktop) are Electron `showMessageBox` with a parent window — the renderer gets no keydown at
all, so the keybinding service is not involved.

| | |
|---|---|
| Mechanism | Context predicates over one flat ordered list (quick input); capture + resolve-then-veto against an allowlist (real modal); nothing at all (native dialog). |
| Escapes | Quick input: anything whose `when` still holds, or that has none — **declarer-decided**. Modal: eight named commands — **modal-decided**. |
| Decides | Different answer per case, which is the point: `when`-clauses were insufficient for a true modal. |
| Help reflects it | **No.** The Keyboard Shortcuts editor filters by text, `@source:` and `+when:` *string equality*; the When column is displayed, never evaluated. |
| Resolve vs reach | **Split twice over.** `softDispatch` resolves without executing — a public seam explicitly for this. And `isBubble` (`^command`) executes without `preventDefault`, so "ran" and "consumed the key" are separate outcomes. |

---

## 6. The web platform — no mechanism at all, and the spec is explicit about why

Inertness is enumerated facility by facility, and **event dispatch is not one of them**:

> A node … can be **inert**. When a node is inert:
> - Hit-testing must act as if the `'pointer-events'` CSS property were set to `'none'`.
> - Text selection functionality must act as if the `'user-select'` CSS property were set to `'none'`.
> - If it is editable, the node behaves as if it were non-editable.
> - The user agent should ignore the node for the purposes of find-in-page.
>
> Inert nodes generally cannot be focused, and user agents do not expose the inert nodes to
> accessibility APIs or assistive technologies.
> — [HTML §6.3](https://html.spec.whatwg.org/multipage/interaction.html#inert-subtrees)

That is the whole list. Two further steps make the answer airtight. Key routing checks
inertness on the **event target only**:

> 5. **If *target node* is not inert:** Let *canHandle* be the result of dispatching the key event
> at *target node*.
> — [focus processing model](https://html.spec.whatwg.org/multipage/interaction.html#focus-processing-model)

and with a modal open, focus is inside the dialog, which escapes inertness. Then DOM dispatch
walks parents with no inert concept whatsoever — **the string "inert" does not occur anywhere in
the DOM Standard** ([dom.spec.whatwg.org](https://dom.spec.whatwg.org/), verified 0 matches).

**So a `keydown` listener on `document` or `window` fires normally while a modal `<dialog>` is
open.** Inertness restricts what can *be* a target; it never restricts propagation *past* inert
ancestors. `element.click()` and `dispatchEvent` are likewise unaffected — the `click()` steps
contain no inert check, and MDN's "do not have `click` events fired" is about user clicks, a
consequence of the `pointer-events: none` rule.

**Even Esc is subordinate to a page listener.** The close-request steps fire the ordinary
`keydown` first, then: "**If *event* is not null, and its canceled flag is set, then return.**"
([close requests](https://html.spec.whatwg.org/multipage/interaction.html#close-requests)).
`preventDefault()` on your handler stops the dialog closing. Esc is the only key the platform
claims.

The one declarative shortcut registry, `accesskey`, has a fixed three-condition predicate —
visible, enabled, in a document with a browsing context, no `hidden` ancestor — and
**inertness is not in it** ([HTML
§6.7.3](https://html.spec.whatwg.org/multipage/interaction.html#assigning-keyboard-shortcuts)).
The only connection is the informative sentence "Inert nodes that are commands will become
inoperable to users." *[Genuine spec looseness. UNVERIFIED whether implementations block
accesskey inside an inert subtree.]* `navigator.keyboard` (Keyboard Lock, Keyboard Map) is about
which physical keys you can observe, not scoped commands, and both are WICG explainers.

| | |
|---|---|
| Mechanism | None. |
| Escapes | Everything. |
| Decides | Nobody. The app author hand-rolls the guard. |
| Help reflects it | N/A — no registry to display. |
| Resolve vs reach | Only via `preventDefault`/`stopPropagation` on the event. |

---

## 7. Flutter — no truncation, and the one modality knob is unused by routes

`ModalBarrier` is pointer and semantics only: "A widget that prevents the user from interacting
with widgets behind itself" — its `build` is `BlockSemantics` → `ExcludeSemantics` →
`_ModalBarrierGestureDetector`, with no key handling anywhere in the file
([modal_barrier.dart](https://raw.githubusercontent.com/flutter/flutter/master/packages/flutter/lib/src/widgets/modal_barrier.dart)).

Key events walk the focus chain to the root unconditionally:

```dart
// Walk the current focus from the leaf to the root, calling each node's
// onKeyEvent on the way up, and if one responds that they handled it or
// want to stop propagation, stop.
for (final node in <FocusNode>[
  FocusManager.instance.primaryFocus!,
  ...FocusManager.instance.primaryFocus!.ancestors,
]) {
```
— [focus_manager.dart](https://raw.githubusercontent.com/flutter/flutter/master/packages/flutter/lib/src/widgets/focus_manager.dart)

No scope check, no modal check. **`FocusScopeNode` does not truncate this walk** — its class doc
claims only traversal grouping ("Scopes form sub-trees of nodes that can be traversed as a
group"), and `showDialog`'s public doc scopes only `traversalEdgeBehavior`
([showDialog](https://api.flutter.dev/flutter/material/showDialog.html)). `Shortcuts` installs a
plain `Focus` node with `canRequestFocus: false`, and `_WidgetsAppState.build` wraps the whole
`Navigator` in one, so a dialog's focus nodes are strict descendants of the app-level
`Shortcuts`. **An app-level `Shortcuts` is still consulted for keys pressed inside a dialog.**

The proof is Escape closing a dialog: `WidgetsApp._defaultShortcuts` maps
`SingleActivator(LogicalKeyboardKey.escape)` to `DismissIntent()` — declared *outside* the
dialog — and `ModalRoute` supplies only a nearer `Actions` entry, `_DismissModalAction`
([routes.dart](https://raw.githubusercontent.com/flutter/flutter/master/packages/flutter/lib/src/widgets/routes.dart)).

**Flutter does have a truncation knob, and it is declarer-owned and unused by the framework.**
`ShortcutManager.modal`:

> "True if the [ShortcutManager] should not pass on keys that it doesn't handle to any
> key-handling widgets that are ancestors to this one." … "The net effect of setting [modal] to
> true is to return [KeyEventResult.skipRemainingHandlers] from [handleKeypress] if it does not
> exist in the shortcut map, instead of returning [KeyEventResult.ignored]."
> — [shortcuts.dart](https://raw.githubusercontent.com/flutter/flutter/master/packages/flutter/lib/src/widgets/shortcuts.dart)

Code search over `packages/flutter/lib` finds `ShortcutManager` in four files, none of them
route, dialog or overlay code, and `"modal: true"` returns zero hits. Nothing in Flutter's
routing layer sets it. Note also the direction: `modal` is a property of a **`Shortcuts` widget**
— the declarer of a *set* of shortcuts saying "stop above me" — not a property of a route.

| | |
|---|---|
| Mechanism | None, by default. An unused opt-in truncation flag exists on the shortcut installer. |
| Escapes | Everything declared outside. |
| Decides | The shortcut declarer, via `ShortcutManager.modal`. The modal's only lever is planting a nearer `Actions` entry that wins. |
| Help reflects it | N/A — Flutter ships no shortcut display surface. *[UNVERIFIED whether any shipped widget renders active shortcuts.]* |
| Resolve vs reach | **Split, sharply.** Key→Intent resolves in an ancestor `Shortcuts`; Intent→Action restarts from `primaryFocus` and can land on a different, nearer handler — exactly the Escape/dialog case. A found-but-disabled Action stops the Action search but *not* the key: `handleKeypress` returns `ignored` and the focus walk continues. |

---

## 8. Textual — extending the prior survey

`_modal_binding_chain` truncation and the `priority` reversal pass are already in
[`event-bubbling.md`](./event-bubbling.md). What is new:

**`App.BINDINGS` ships two bindings with opposite fates.**

```python
BINDINGS: ClassVar[list[BindingType]] = [
    Binding("ctrl+q", "quit", "Quit", ..., show=False, priority=True),
    Binding("ctrl+c", "help_quit", show=False, system=True),
]
```
— [app.py](https://raw.githubusercontent.com/Textualize/textual/main/src/textual/app.py)

`ctrl+q` **survives** a `ModalScreen`, because the priority pass runs over the untruncated
`_binding_chain`. `ctrl+c` **does not** — `priority` defaults False, so it goes through
`_modal_binding_chain`, which cuts the `App` namespace off. The command palette also survives:
`Binding(self.COMMAND_PALETTE_BINDING, "command_palette", ..., priority=True)`, `ctrl+p`.

**`system=True` is display-only. Nailed.**

```python
system: bool = False
"""Make this binding a system binding, which removes it from the key panel."""
```
— [binding.py](https://raw.githubusercontent.com/Textualize/textual/main/src/textual/binding.py)

Its only consumer is `_key_panel.py`'s `if not binding.system:` filter. Nothing in `screen.py`,
`app.py`, `dom.py` or `widget.py` reads it during resolution. `system` grants no modal escape.

**`ModalScreen` differs from `Screen` by one line plus CSS** — `self._modal = True` and a dimmed
background ([screen.py](https://raw.githubusercontent.com/Textualize/textual/main/src/textual/screen.py)).
`is_modal`'s only other consumer in `app.py` is a styling check.

**And Textual has shipped the divergence froth is trying to avoid.** `Screen.active_bindings`
walks `_modal_binding_chain` — the *truncated* chain — and the `Footer` and key panel both
consume it. Surviving priority bindings are not added back. Concretely, the footer's dedicated
command-palette slot silently disappears under a modal:

```python
if self.show_command_palette and self.app.ENABLE_COMMAND_PALETTE:
    try:
        _node, binding, enabled, tooltip = active_bindings[self.app.COMMAND_PALETTE_BINDING]
    except KeyError:
        pass
```
— [_footer.py](https://raw.githubusercontent.com/Textualize/textual/main/src/textual/widgets/_footer.py)

`ctrl+p` still works under a modal and stops being advertised. **`active_bindings` is not the set
of keys that resolve.** This is the direct cost of having a second, separate walk for the
escaping bindings.

| | |
|---|---|
| Mechanism | Truncation, plus a whole second untruncated loop for the escapers. |
| Escapes | `priority=True` bindings only. In stock Textual: `ctrl+q` and `ctrl+p`. Not `ctrl+c`, not `system=True`. |
| Decides | The shortcut **declarer**, via `priority`. The modal decides only the cut point. Docs: "Such bindings can not be disabled by binding the same key on a widget." ([guide/input](https://textual.textualize.io/guide/input/)) |
| Help reflects it | Yes — but **over-reflects**, per above. |
| Resolve vs reach | Not a separate stage. `run_action` gates on `check_action` at delivery, and a falsy result *resumes* the search — "resolved but refused" degrades into "keep looking", not into "swallowed". |

---

## Cross-cutting findings

**1. Four mechanism shapes, and the winner is not the obvious one.**

| Shape | Systems |
|---|---|
| Loop replacement / propagation-root swap | Win32, Cocoa (event layer), GTK 4 |
| Context predicate | Qt, Cocoa (validation layer), VS Code (quick input) |
| Truncation | Textual only |
| Nothing | Web, Flutter |
| Modal-owned allowlist | VS Code (custom dialog) |

**Textual is the only system in the survey that truncates a chain.** That is worth stating
plainly, because froth's ADR 0008 amendment is described as arriving at Textual's answer
independently, and the two of them are the entire population. The reason is structural: only a
system whose shortcut resolution is *already* a tree walk can truncate. Everything else has a
flat table (Win32, Qt, VS Code) or a per-window dispatch site (GTK, Cocoa), and reaches the same
outcome with a predicate or by not running the loop.

**2. The framework decides almost everywhere. Only two systems let the declarer decide.**

Framework-decided: Win32 (nothing survives), Qt (`tryModalHelper`, no opt-out — the workaround is
to reparent the shortcut), GTK (grab ancestry, no flag exists), Cocoa (`worksWhenModal` is
AppKit's own on the menu panel). Modal-decided: VS Code's dialog allowlist.
**Declarer-decided: Textual's `priority`, Flutter's `ShortcutManager.modal`.**

Note that the two declarer-decided systems are inverted with respect to each other. Textual's
flag is per-binding and says *"I survive truncation"* (default: no). Flutter's is per-`Shortcuts`
widget and says *"truncate above me"* (default: no truncation). froth's `SurvivesModal` is
Textual's, exactly.

**3. Three systems curate an explicit survivor list. All three start it with quit.**

- Cocoa: Quit, Hide, Cut/Copy/Paste — by nil-target items resolving to `NSApp`.
- VS Code: `workbench.action.quit`, `reloadWindow`, and six clipboard/select-all commands.
- Textual: `ctrl+q` quit, `ctrl+p` command palette.

The survivor set is small, named, and quit-shaped in every case. Nobody found they needed a
general facility; they found they needed about six commands.

**4. Help reflecting modality is rare, and the one system that gets it right does so by
identity.** Cocoa's menu *is* the resolution surface, so it cannot drift. GTK's shortcuts window
has no notion of modality. VS Code's shortcuts editor compares `when` clauses as strings and
never evaluates them. Textual reflects modality and gets it *wrong in the shipped product*,
because the escape mechanism is a second walk that the help surface does not consult.

**5. Yes — four systems distinguish "still resolves" from "still reaches its handler", and they
disagree about which side modality lands on.**

| System | The split | Where modality lands |
|---|---|---|
| Cocoa | `performKeyEquivalent:` → `validateMenuItem:` | **Reach.** The key equivalent resolves; validation disables it. |
| Qt | `QKeySequence::matches()` → `correctContext()` → `dispatchEvent` | **Resolve.** Still an `ExactMatch`; dropped before any event exists. |
| VS Code | `softDispatch` (resolve, don't run); `isBubble` (run, don't consume) | Both — `softDispatch` is a public seam built for exactly this veto. |
| Flutter | key→Intent (ancestor walk) → Intent→Action (restart at focus) | N/A — no modality, but the two-step *is* how a dialog overrides Escape. |

froth is not inventing the split. The interesting part is the disagreement: Qt drops the match so
the key falls through as raw input; Cocoa keeps the match and greys the item. Those produce
visibly different user experiences for the same press, and both ship.

---

## What transfers to froth, and what does not

### Transfers

**The survivor set is small, named, and quit-shaped.** Three systems curated one; none exceeded
eight entries; all three led with quit. This is the strongest empirical support for froth's
shape: a per-binding opt-in, sparsely used, is what the field converged on in practice even where
the mechanism was framework-owned.

**One walk, or help lies.** Textual has the exact bug froth's `liveBindings()` was designed to
prevent, and it is caused by the escape mechanism being a *second* walk (`_binding_chain` for
priority, `_modal_binding_chain` for everything else) that the help surface does not consult. If
froth implements truncation, `SurvivesModal` must be honoured inside `liveBindings()` and nowhere
else — the moment there is a second path, `Query` stops describing what a key press does. The
comment already in `binding.go` ("This is the only place the chain is decided") is the right
instinct, and Textual is the evidence for it.

**Cocoa's split is froth's split, and it is the better-argued side.** `performKeyEquivalent:`
resolves; `validateMenuItem:` decides whether it fires; modality enters at the *second* step. That
is froth's new identity/destination split with modality landing on delivery rather than on
resolution. Qt lands it on resolution instead and the observable consequence is that the key
becomes raw input inside the dialog — which for froth would mean a modal's text sink suddenly
receiving `ctrl+c`. Cocoa's ordering is the safer default and has the older pedigree.

**"Global" scope names are a documented trap.** GTK's `GLOBAL` means one window. Qt's
`ApplicationShortcut` means "one of the application's *unblocked* windows", and the enum
documentation has never been amended to say so — Qt's own forum threads are people discovering
this. Whatever froth calls this flag, the name must not imply a reach it does not have.
`SurvivesModal` is honest in a way `Global` would not be, and ADR 0008's "It survives modality
only — not focus scopes, not anything else" is the sentence Qt failed to write.

### Does not transfer

**Loop replacement and propagation-root swapping.** Win32, Cocoa and GTK all reach the answer by
having a different pump or a different propagation root. froth has one `Update` and one walk.
Their escape hatches (`WH_MSGFILTER`, `worksWhenModal` panels, transient-for grab exemption) are
artifacts of that architecture and have no froth analogue.

**Context predicates as a general mechanism.** Qt's and VS Code's answers are cheap *because they
already had* a flat table with a per-entry predicate. froth's chain walk already encodes
"where am I" positionally; adding a `when` expression would be a second, redundant authority over
the same question. Note also that VS Code found `when` clauses **insufficient** for a real modal
and reached for an allowlist anyway.

**Qt's "reparent the shortcut" workaround.** The accepted answer to "I want this to work under a
modal" is to move the declaration onto the dialog and move it back afterwards. In froth that is
already the ordinary thing — a modal declaring its own binding — and it is not a workaround for
anything.

### On the `SurvivesModal` default: defensible, and the flag is more likely wrong than the default

**The default (zero value = does not survive) matches the field almost unanimously.** Six of eight
systems default to not-surviving: Win32 (nothing survives at all), Qt (`tryModalHelper` blocks),
GTK (grab blocks), Cocoa (`worksWhenModal` is documented "By default … `false`"), VS Code's dialog
(closed allowlist), Textual (`priority` defaults False, and even stock `ctrl+c` does not escape).
The two that default to surviving — Flutter and the web — do so because they have no mechanism,
not because they chose it. There is no system in the survey that built a modality mechanism and
then defaulted it open. **Flipping the default would be unprecedented.**

**But froth's truncation is much narrower than any of theirs, and the evidence cannot judge
that.** In every surveyed system, "above the modal" is a large and varied population: an entire
other window, a whole editor's context, a deep widget subtree. In froth, modals are parented to
the root, so the chain is `[root, modal]` and truncation blocks the bindings of exactly one
surface. Combined with the discipline that root holds only genuinely global behaviour, *every
binding truncation can block is by construction the kind of binding that Cocoa, VS Code and
Textual all put on their survivor lists.* No system in the survey has this shape, so the survey
cannot tell you whether a flag is warranted — it can only tell you that where the population was
large, a flag was.

**The empirical hint from froth's own tree is that the flag is degenerate.** Both examples declare
exactly one root binding, and both set `SurvivesModal: true` (`examples/panes/root.go:95`,
`examples/dir-nav/root.go:72`). Two of two. A flag whose every use site sets it is not carrying a
decision. That is weak evidence — two examples, neither with a modal — but it is the only froth-
shaped evidence there is, and it points the opposite way from the framework default.

**The one place the evidence actively pushes back on ADR 0008.** Every system in the survey except
Win32 guarantees that *something* gets you out. Cocoa's Quit is framework-guaranteed via the
`NSApp` responder terminus. VS Code's allowlist opens with `workbench.action.quit`. Textual ships
`ctrl+q` with `priority=True` in `App.BINDINGS` — the framework declares it, not the application.
ADR 0008 deliberately refuses this ("an application that marks nothing as surviving can write a
modal it cannot quit past, and froth will not save it"), and the refusal is well-argued on the
grounds that quit policy is the application's. But it should be recorded as a departure from
unanimous precedent rather than as an unremarkable consequence — and the second amendment's own
line, "the line is what froth services", is what makes it survivable: froth cannot service quit,
so it cannot guarantee it.

**Honest verdict:** the default is right by precedent and unjudgeable by froth's actual shape. The
question the evidence reframes is not "which way should the default point" but "is a per-binding
flag the right granularity when it can only ever affect one surface" — and on that, the survey is
silent because nobody else has a one-surface population. The decision belongs to whether root
stays thin, which is a froth discipline question, not a prior-art one.
