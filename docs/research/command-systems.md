# Commands that exist without keys: eight systems

Gathered to inform whether a froth `Action` should be invocable with no key press — a
command palette being the motivating case. froth today resolves a key along the focus
chain, nearest declaration wins, and delivers an `ActionMsg` **to the surface that
declared the winning binding**. Keyless invocation breaks that, because resolution is
currently what decides the destination.

**Core finding up front:** every system surveyed separates command identity from keys, and
every one of them also separates **identity from destination**. In no system does the site
that *declared* a binding become the recipient by virtue of having declared it. The
destination is decided at invocation time — by a named scope (`app.`, `win.`), by a walk
from focus, or by a flat registry — and the binding site is forgotten. froth is the outlier
here, and that is precisely why keyless invocation is awkward for it. Secondarily: the two
closest-domain systems with real palettes (VS Code, Textual) both ended up with **two
tables**, and their help and their keys disagree.

## 1. Cocoa / AppKit — the strongest precedent for "keys are one entry point"

**Identity is a bare selector.** There is no registry at all. `copy:` is a method name;
presence of an implementation *is* registration. Apple states the framework/app line
plainly: *"Event messages form a well-known set, so NSResponder provides declarations and
default implementations for all of them. Most action messages, however, are defined by
custom classes and can't be predicted"*
([Cocoa Event Handling Guide](https://developer.apple.com/library/archive/documentation/Cocoa/Conceptual/EventOverview/EventArchitecture/EventArchitecture.html)).

**Destination is found by walking, and the binder cannot name it.** A menu item stores an
`action` selector and a `target`; setting the target to `nil` — Interface Builder's "First
Responder" — is the normal idiom. `sendAction:to:from:` then searches: *"If `aTarget` is
`nil`, shared looks for an object that can respond to the message… It begins with the first
responder of the key window… continues following next responder links up the responder
chain… attempts to send the message to the key window's delegate"*, and if nothing answers,
*"shared tries to handle the message itself. If shared can't respond, it attempts to send
the message to its own delegate"*
([NSApplication.sendAction](https://developer.apple.com/documentation/appkit/nsapplication/sendaction(_:to:from:))).
There is always a backstop: *"The application object and its delegate are the receivers of
last resort"*
([Target-Action](https://developer.apple.com/library/archive/documentation/General/Conceptual/CocoaEncyclopedia/Target-Action/Target-Action.html)).

**Availability is a predicate on whoever is found, recomputed constantly.** *"NSMenu updates
every menu item whenever a user event occurs. A menu item is enabled if NSMenu can find an
appropriate object that responds to the menu item's action"*
([Enabling Menu Items](https://developer.apple.com/library/archive/documentation/Cocoa/Conceptual/MenuList/Articles/EnablingMenuItems.html)).
For a `nil` target that means a full responder-chain walk **per menu item, per user event**,
then `validateMenuItem:`/`validateUserInterfaceItem:` on whatever it found. No cost model is
documented. Not implementing the method is itself the disable: *"If a class never permits
copying, then you simply omit an implementation of `copy:`… and the Copy menu item is
disabled automatically."*

**But keys are emphatically not just another entry point.** Key equivalents run in a
separate, *earlier* pass with the opposite traversal direction — down the view hierarchy,
menu bar last: *"The application object handles key equivalents by going down the view
hierarchy in the key window, sending each object a `performKeyEquivalent:` message until an
object returns YES. If the message isn't handled by an object in the view hierarchy, NSApp
then sends it to the menus in the menu bar"* — and only afterwards does the ordinary
`keyDown:`/responder-chain path run
([Event Handling Guide, Path of Key Events](https://developer.apple.com/library/archive/documentation/Cocoa/Conceptual/EventOverview/EventArchitecture/EventArchitecture.html)).

**Help agrees with reality by sharing one field.** The key equivalent lives on the
`NSMenuItem`, and that is what both the drawing and `NSMenu.performKeyEquivalent(with:)`
read. Consistency is enforced downward too: *"If a menu item is disabled, its keyboard
equivalent is also disabled"*
([isEnabled](https://developer.apple.com/documentation/appkit/nsmenuitem/isenabled)). The
one divergence — a view claiming a key equivalent before the menu bar is consulted, so a
displayed shortcut never fires — follows from the documented order but *[is not itself
stated by Apple; inferred]*.

## 2. Emacs — the real bubbling analogue, and it splits the two paths

**Identity is a flat global symbol; keys are a separate, genuinely scoped lookup.** The
manual makes the split itself: *"the term 'command'… refers to an interactively callable
function… **It does not refer to the key sequence used to invoke a command**"*
([Interactive Call](https://www.gnu.org/software/emacs/manual/html_node/elisp/Interactive-Call.html)).
The keymap chain is the precedence structure froth's focus chain resembles — char-property
map, minor-mode maps, buffer-local map, global map, first match wins
([Searching Keymaps](https://www.gnu.org/software/emacs/manual/html_node/elisp/Searching-Keymaps.html)).
Note it is finer-grained than froth's: an active keymap can differ character by character
within one buffer.

**`M-x` does not use that chain at all.** It completes over the obarray — the global symbol
table — filtered by `commandp`; the source is literally
`(complete-with-action action obarray string pred)`
([simple.el](https://raw.githubusercontent.com/emacs-mirror/emacs/master/lisp/simple.el)).
Emacs then *shipped both designs side by side*: `M-S-x`
(`execute-extended-command-for-buffer`) *"limits the commands offered for completion to
those… of particular relevance to the current major mode… and also commands that are bound
to locally active keymaps."* Two adjacent keys, one flat and one chain-scoped, and the user
cycles between them mid-prompt.

**Bound by one node, handled by another — this is command remapping, and it is
first-class.** *"A special kind of key binding can be used to remap one command to another,
**without having to refer to the key sequence(s) bound to the original command**"*; a
minor-mode map contributes `[remap kill-line] → my-kill-line` while `C-k` stays bound
globally to `kill-line`
([Remapping Commands](https://raw.githubusercontent.com/emacs-mirror/emacs/master/doc/lispref/keymaps.texi)).
The elegance is that a remap is just a key lookup on the synthetic key `[remap COMMAND]`, so
"who gets to remap" reuses the keymap precedence order for free. Both identities stay
observable as `this-command` and `this-original-command`.

**The key path is privileged, and Emacs knows it.** Remapping happens in
`read_key_sequence` (`Fcommand_remapping` in
[keyboard.c](https://raw.githubusercontent.com/emacs-mirror/emacs/master/src/keyboard.c));
`execute-extended-command` calls `command-execute` directly and never remaps. So `C-k` runs
`my-kill-line` while `M-x kill-line RET` runs `kill-line`. Emacs detects this one
disagreement and papers over it rather than lie — from `simple.el`: *"If we're executing a
command that's remapped, we can't actually execute that command with the keymapping we've
found with `where-is-internal`"*, and it suppresses the "you can run this with…" hint.

**Availability is not a central predicate, and never gates invocation.** Being absent from
the active maps makes a command unreachable *by key* while leaving it fully reachable by
name. `read-extended-command-predicate` is the nearest thing to a `when` clause and it is
off by default and gates **completion only** — typing the full name still runs it.
Disabling is a symbol property with a substitute handler, and *"Disabling a command has no
effect on calling it as a function from Lisp programs."*

**Help reads the same maps dispatch does.** `where-is-internal` searches *"all the currently
active keymaps"* and follows remapping in both directions, with a `no-remap` argument to
pick which truth you want
([keymap.c](https://raw.githubusercontent.com/emacs-mirror/emacs/master/src/keymap.c)).
`substitute-command-keys` renders `\[command]` as its live binding *"or `M-x command` if
command has no key bindings"* — and an author's `:advertised-binding` preference *"is ignored
if it specifies a key binding that the command does not actually have"*
([help.texi](https://raw.githubusercontent.com/emacs-mirror/emacs/master/doc/lispref/help.texi)).
Preference is allowed; fabrication is not.

## 3. VS Code — the palette that motivated the question, and it has two tables

**Identity is flat and global**: one `CommandsRegistry` keyed by string. There *is* a
per-id `LinkedList`, but registration `unshift`s and lookup takes `Iterable.first` — a
shadowing stack where only the head is ever consulted, and disposal restores the previous
handler
([commands.ts](https://raw.githubusercontent.com/microsoft/vscode/main/src/vs/platform/commands/common/commands.ts)).
Nothing falls through; there is no "not handled".

**Which implementation runs for `editor.action.formatDocument` is decided imperatively by
the handler**, not by dispatch: `getFocusedCodeEditor() || getActiveCodeEditor()`, and if
neither exists it silently returns — the source comment is *"well, at least we tried..."*
([editorExtensions.ts](https://raw.githubusercontent.com/microsoft/vscode/main/src/vs/editor/browser/editorExtensions.ts)).
The one genuine chain is `MultiCommand` (used by `undo`, `redo`, `selectAll`): several
implementations for one id, sorted by an explicit priority integer, each filtered by a
`when` evaluated against `contextKeyService.getContext(getActiveElement())`, first truthy
return wins. It is opt-in, internal, and unreachable from extensions.

**`when` clauses relate to focus by DOM ancestry.** Context key services form a tree;
`getContext` walks *up* from the event target looking for a `data-keybinding-context`
attribute, and value lookup then inherits up the context-parent chain
([contextKeyService.ts](https://raw.githubusercontent.com/microsoft/vscode/main/src/vs/platform/contextkey/browser/contextKeyService.ts)).
Two orthogonal walks: one picks the scope, one resolves the key. Menus subscribe with
key-set filtering and the source is candid about cost — *"these events are firing often"*
([menuService.ts](https://raw.githubusercontent.com/microsoft/vscode/main/src/vs/platform/actions/common/menuService.ts)).
Keybindings do not cache at all; `when` is re-evaluated per keypress.

**Availability never gates programmatic invocation**, and the schema says so outright:
`enablement` is *"Condition which must be true to enable the command in the UI (menu and
keybindings). Does not prevent executing the command by other means, like the
`executeCommand`-api"*
([menusExtensionPoint.ts](https://raw.githubusercontent.com/microsoft/vscode/main/src/vs/workbench/services/actions/common/menusExtensionPoint.ts)).

**Keyless invocation starts at the registry.** `executeCommand` is one map lookup plus lazy
activation — no chain, no focus walk.

**Keys are slightly privileged.** Both the palette and a keypress converge on
`ICommandService.executeCommand`, but keybindings can carry `args` from `keybindings.json`
(*"You can invoke a command with arguments"* —
[keybindings.md](https://raw.githubusercontent.com/microsoft/vscode-docs/main/docs/configure/keybindings.md)),
while an ordinary palette entry never sets them. Menus inject arguments no other path
supplies — *"a menu item inside the Explorer is passed the URI of the selected resource"* —
and an `alt` property gives one menu item a second command.

**Help reads a different table from execution, and disagrees.** The palette builds from
`MenuRegistry` (`MenuId.CommandPalette` plus implicitly every `contributes.commands` entry),
filtered by `when` and `precondition` evaluated in the *editor's* scope — because the palette
itself holds focus while the list is built
([commandsQuickAccess.ts](https://raw.githubusercontent.com/microsoft/vscode/main/src/vs/workbench/contrib/quickaccess/browser/commandsQuickAccess.ts)).
Consequences: a command in `CommandsRegistry` with no `MenuRegistry` entry is fully
executable and wholly invisible (every internal command); the displayed keybinding is
`lookupKeybinding(id)` and is shown regardless of whether its `when` currently holds; and the
same expressions can return different verdicts depending on which scope asked. An extension
*can* take over handling of a built-in id — the API layer throws on duplicates but the
main-thread bridge does not, and the command service anticipates it in a comment: *"in case
an extension wants to overwrite it."*

## 4. WPF — identity with no implementation, and enablement as a global requery

**A `RoutedCommand` is an object identity with deliberately no behaviour.** The library
classes *"consist only of the `RoutedCommand` objects and not the implementation logic…
The implementation logic is the responsibility of the object on which the command is being
executed on"*
([Commanding Overview](https://learn.microsoft.com/en-us/dotnet/desktop/wpf/advanced/commanding-overview)).
A `CommandBinding` on a `UIElement` supplies it, and `Execute`/`CanExecute` merely raise
tunnel/bubble routed event pairs that *"tunnel and bubble through the element tree until
they encounter an object with a `CommandBinding`."*

**Scope is tree position, and the direction is asymmetric:** *"A `CommandBinding` attached to
an ancestor of the command target can be reached by the `Executed` event, but a
`CommandBinding` attached to a descendant of the command target cannot be reached."* So
bind-on-child/handle-on-window works; bind-on-window/handle-on-child does not, unless
`CommandTarget` is set explicitly.

**Keyless invocation falls back to focus, with no backstop.** `RoutedCommand.Execute(param,
null)` resolves `target = FilterInputElement(Keyboard.FocusedElement)`
([RoutedCommand.cs](https://github.com/dotnet/wpf/blob/main/src/Microsoft.DotNet.Wpf/src/PresentationCore/System/Windows/Input/Command/RoutedCommand.cs)).
If nothing has focus, `target` stays null and the command silently does nothing — there is
no application-object last resort of the kind AppKit guarantees.

**The documented "keyboard focus is the target" is delivered by a second hop, not by the
route.** When a bubbling `Executed` reaches an unhandled focus scope (`Menu` and `ToolBar`
both are), `CommandManager` abandons the route and re-runs the whole command targeting the
focused element of the *parent* focus scope
([CommandManager.cs](https://github.com/dotnet/wpf/blob/main/src/Microsoft.DotNet.Wpf/src/PresentationCore/System/Windows/Input/Command/CommandManager.cs)).
A retry from a second origin, not one walk.

**Enablement is a global requery.** `CommandManager.InvalidateRequerySuggested` fires on
every `KeyUp`, `MouseUp`, `GotKeyboardFocus` and `LostKeyboardFocus`
([CommandDevice.cs](https://github.com/dotnet/wpf/blob/main/src/Microsoft.DotNet.Wpf/src/PresentationCore/System/Windows/Input/Command/CommandDevice.cs)),
coalesced onto one background-priority dispatcher operation; each listening command source
then re-runs `CanExecute`, itself a full tunnel+bubble pass. Microsoft documents no
performance caveat *[the reputation is real but community-sourced — UNVERIFIED against a
primary source]*.

**Keys are privileged three ways.** Per-element precedence puts input bindings first —
*"input bindings associated with the targetElement instance… class… command bindings
associated with the targetElement instance… class"*; an unset `CommandTarget` on the key path
resolves to the element being walked, not to focus; and a matched gesture marks the input
`Handled` **even when `CanExecute` returned false**, with the `NotACommand` sentinel existing
solely to opt out.

**Help can lie.** `MenuItem.InputGestureText` defaults from the command's own `InputGestures`
— the same collection the key path matches — but only when unset, and it is otherwise plain
free text nothing validates
([MenuItem.cs](https://github.com/dotnet/wpf/blob/main/src/Microsoft.DotNet.Wpf/src/PresentationFramework/System/Windows/Controls/MenuItem.cs)).
It also shows only the first gesture, and shortcuts registered as class input bindings (how
`TextBox` registers Ctrl+C and Ctrl+Insert) are invisible to it entirely.

## 5. GTK 4 — namespaced identity, stored enablement, per-name shadowing

**Identity is scoped by prefix**, not by a tree address: *"the convention is to add a prefix
to the action name to indicate the scope of the actions, such as `app.` for the actions with
application scope or `win.` for those with window scope"*, and the prefixed form is used
*"when referring to actions from places like a `GMenu` or `GtkActionable` widget where the
scope of the action is not already known"*
([docs.gtk.org/gtk4/actions.html](https://docs.gtk.org/gtk4/actions.html)).

**Enablement is a stored boolean with a change signal**, not a predicate — `g_action_get_enabled`,
`g_simple_action_set_enabled`, and `action-enabled-changed`
([Gio.Action](https://docs.gtk.org/gio/iface.Action.html)). This is the one push model in the
survey: cost is paid by the author at the moment state changes, and every consumer gets an
O(1) read plus a signal, instead of the constant re-derivation AppKit and WPF do.

**Keys are genuinely one entry point among equals.** Accelerators, `GtkActionable` buttons and
menu items, and `GtkNamedAction` inside a shortcut controller all name the same detailed
action string (`"app.quit"`, `"win.open('file')"`) via
[`set_accels_for_action`](https://docs.gtk.org/gtk4/method.Application.set_accels_for_action.html).
None is privileged.

**Keyless invocation walks up.** *"When a widget with a connected action is activated, GTK
finds the action to activate by walking up the widget hierarchy, looking for a matching
action, ending up at the `GtkApplication`."*

**Bound-here/handled-there is the whole design**, and shadowing is documented per *name*, not
per prefix: *"even if you insert a group with prefix `prefix`, actions with the same prefix
will still be inherited from the parent, unless the group contains an action with the same
name"*
([insert_action_group](https://docs.gtk.org/gtk4/method.Widget.insert_action_group.html)).

**Framework actions share the namespace scheme** — `clipboard.copy`, `selection.select-all`,
`default.activate`, `color.select` are ordinary prefixed actions. *[`text.undo` was named in
the brief but not found on the fetched page — UNVERIFIED.]* Discoverability reads the live
table both directions (`get_accels_for_action`, `get_actions_for_accel`); whether GTK's own
menu rendering reads that same table is *[UNVERIFIED]*.

## 6. Textual — closest domain, has a palette, and kept two systems

**Identity is a method name resolved per node**: an action string names `action_<name>` on a
DOM node, optionally namespaced. The namespaces are a *closed set of three* — source:
`self._action_targets = {"app", "screen", "focused"}`
([app.py](https://raw.githubusercontent.com/Textualize/textual/main/src/textual/app.py)) —
not a general node path.

**Bound by one node, handled by another is production code.** `Screen.BINDINGS` contains
`Binding("tab", "app.focus_next", …)` and `Binding("shift+tab", "app.focus_previous", …)`
([screen.py](https://raw.githubusercontent.com/Textualize/textual/main/src/textual/screen.py)):
the key is bound on the Screen, the action runs on the App. Exactly froth's question 6, and
it works because the namespace names a *role*, not the declaring node.

**The walk exists only in key matching, not in dispatch.** Key resolution runs
`[focused_widget, …, screen, app]` via `ancestors_with_self`, and the node whose `BINDINGS`
matched becomes `default_namespace`. `_parse_action` then returns that namespace *unchanged*
for a bare action — no second walk. So `Widget.run_action("save")` with no namespace looks up
`action_save` on that one widget and, failing, logs *"has no target"* and returns `False`. It
does not fall back to ancestors.

**Availability is a three-way predicate.** `check_action` returns *"`True` if the action is
enabled+visible, `False` if the action is disabled+hidden, `None` if the action is
disabled+visible (grayed out in footer)"*
([dom.py](https://raw.githubusercontent.com/Textualize/textual/main/src/textual/dom.py)) —
but `run_action` gates on plain truthiness, so `False` and `None` block identically and the
distinction is purely for display. The Footer calls it per binding on every refresh.

**The decisive finding: the command palette is a wholly separate, hand-maintained system.**
Palette entries come from `Provider` classes; the built-in `SystemCommandsProvider` yields
whatever `App.get_system_commands` returns, which is a hand-written generator of bound
methods and lambdas — `self.action_change_theme`, `screen.action_minimize`, an inline
screenshot lambda
([system_commands.py](https://raw.githubusercontent.com/Textualize/textual/main/src/textual/system_commands.py),
[app.py](https://raw.githubusercontent.com/Textualize/textual/main/src/textual/app.py)).
Those callbacks are invoked as ordinary Python callables, bypassing `run_action`,
`_parse_action`, `_dispatch_action` and `check_action` entirely. No code path surfaces bound
keys in the palette or palette commands in the Footer. The command-palette guide never
mentions `BINDINGS`, and `get_system_commands`'s own docstring tells the author to re-list by
hand. **The framework closest to froth's problem, facing exactly this question, declined to
unify the two.**

## 7. Sublime Text — flat names, pull predicates, three identical entry points

Command names are flat strings derived from the class name; the official API reference says
only *"By default this is derived from the name of the class"*
([api_reference.html](https://www.sublimetext.com/docs/api_reference.html)) — the
`ExampleCommand → example` transform is stated only in community docs *[not primary]*.

Availability is a pull predicate on the class: `is_enabled()` — *"Return whether the command
is able to be run at this time"* — and `is_visible()` — *"Return whether the command should be
shown in the menu at this time"*. Exact call timing is not documented beyond "at this time".

Keys are one of three identical entry points: `.sublime-keymap`, `.sublime-menu` and
`.sublime-commands` all carry the same `"command"` + `"args"` pair *(the first two verbatim
from the official docs; the palette file format is community-sourced — UNVERIFIED)*.
Keyless invocation has documented precedence: `Window.run_command` *"is able to run any sort
of command, **dispatching the command via input focus**"*, while `sublime.run_command` and
`View.run_command` are type-scoped.

What happens on a name collision between a plugin command and a built-in is **[UNVERIFIED]** —
nothing in the official docs states it. Whether the palette displays keybindings, and from
where, is likewise **[UNVERIFIED]**.

## 8. Neovim — two unrelated mechanisms, glued by convention

There is no unified dispatch at all. User commands are a flat global namespace with opt-in
`-buffer` scoping; mappings are per-mode with buffer-local beating global (*"Buffer-local
mappings… take precedence over global mappings"*,
[map.txt](https://raw.githubusercontent.com/neovim/neovim/master/runtime/doc/map.txt)).

`<Plug>` is the identity/binding split done purely by convention: *"`<Plug>` is visible
outside of the script. It is used for mappings which the user might want to map a key
sequence to. `<Plug>` is a special code that a typed key will never produce"*
([usr_41.txt](https://raw.githubusercontent.com/neovim/neovim/master/runtime/doc/usr_41.txt)).
The plugin declares behaviour under a never-typed name; the user separately maps a key to it.
No command-identity object exists in the API.

Reverse discovery is not first-class: finding which keys invoke `<Plug>(FooDoThing)` means
scanning `maplist()`/`nvim_get_keymap` output and matching on `rhs` yourself, and Neovim
ships no built-in surface showing active keys at all.

## Cross-cutting findings

**1. Identity separated from keys is universal.** All eight. froth already has this and it is
uncontroversial.

**2. Identity separated from *destination* is also universal, and froth is the outlier.**
AppKit's binder holds only a selector and cannot name a handler. GTK's button names
`win.save` and does not know who implements it. Textual's binding names `app.focus_next`.
Emacs' `[remap]` rewrites the identity while the key stays bound elsewhere. WPF's binding
site and handling site are different tree nodes by design. In every case the destination is
derived at invocation from a scope name or a walk. froth deriving it from the declaration
site is unusual — not wrong, but it is the specific property that makes keyless invocation
hard, because there is no declaration site when nobody pressed a key.

**3. Nobody replays the key-resolution walk to service a keyless invocation.** The systems
split cleanly. *Flat registry*: VS Code (`executeCommand`), Emacs `M-x` (the obarray),
Sublime. *Walk from focus*: AppKit (first responder of the key window), WPF
(`Keyboard.FocusedElement`), GTK (up from the widget), Textual's namespaces. Not one system
answers "who receives this" by asking "who would have received the key".

**4. The walk-from-focus family needs a backstop, and the one without it fails silently.**
AppKit guarantees `NSApp` and its delegate as *"receivers of last resort"*. WPF has none, and
a command with nothing focused is silently dead. froth's chain ends at the root surface plus
froth's stock bindings, and a froth-serviced action is delivered nowhere at all — so it has
neither AppKit's backstop nor a defined outcome for an unclaimed action. ADR 0008's amendment
already names that as the cost of action bubbling: *"a defined outcome for an action nobody
claims"*.

**5. Availability is never allowed to block a programmatic invocation.** The two systems that
state it, state it flatly. VS Code: *"Does not prevent executing the command by other means,
like the `executeCommand`-api."* Emacs: *"Disabling a command has no effect on calling it as a
function from Lisp programs."* Textual's `check_action` does gate `run_action`, but the
palette bypasses `run_action` entirely and so bypasses the check too. Convergent, and it
bears directly on whether froth's `Disabled` would gate a palette invocation.

**6. Push vs pull enablement, and pull is expensive everywhere it is used.** GTK alone stores
the boolean and signals changes. AppKit recomputes on every user event, walking the responder
chain per menu item. WPF requeries on every key-up, mouse-up and focus change, tunnel+bubble
per source. Textual calls `check_action` per binding per Footer refresh. None of the pull
systems documents a cost model. froth's `Disabled`-recomputed-per-ask is the pull model, but
bounded by the focus chain rather than by a registry.

**7. "Keys are just one entry point" is more often aspiration than fact.** Genuinely equal in
GTK, Sublime and Textual. Privileged in AppKit (a separate earlier pass with the opposite
traversal), WPF (input bindings first, and handled-even-when-disabled), Emacs (remapping
applies to keys and not to `M-x`), and mildly in VS Code (`args` from `keybindings.json`,
menu-injected resource arguments).

**8. Help agrees with reality only where it reads the dispatch table itself.** Emacs
(`where-is-internal` over the active keymaps), AppKit (one field on the menu item) and GTK
(one accel table, readable both directions) agree by construction. VS Code reads
`MenuRegistry` while executing from `CommandsRegistry` and disagrees in four documented ways.
WPF's display defaults from the dispatch table but is overridable free text and hides a whole
registration channel. Textual maintains two entirely separate lists. **Two tables always
drift; the only question is how visibly.**

## What transfers to froth, and what does not

**Transfers — the framing.** froth's resolution currently answers two questions in one pass:
*which action*, and *who receives it*. No surveyed system fuses those. That is the real
content of the palette question, and it is sharper than "can an action fire without a key".
The three shapes on offer are: a **named scope** (GTK `win.`/`app.`, Textual
`app.`/`screen.`/`focused.`), a **walk from focus with a backstop** (AppKit), or a **flat
registry** (VS Code, Emacs `M-x`). froth has no registry and, per ADR 0008, deliberately no
second table — which leaves the first two as the shapes that fit what froth already is.

**Transfers — the honest warning from the closest neighbour.** Textual is the same domain,
has the same declarative `BINDINGS`, has the same per-node enablement predicate, and shipped
a command palette anyway as a hand-maintained parallel list that bypasses its own dispatch.
That is evidence about difficulty, not about correctness — but it is the most directly
comparable evidence available, and it argues that unifying the two is genuinely hard rather
than merely undone.

**Transfers — enablement should probably not gate a keyless call.** Two independent systems
say so explicitly and a third achieves it by accident. If froth lets a palette invoke an
action, `Disabled` blocking it would be the minority position and worth an explicit argument.

**Transfers — the shape of what froth is missing is small and specific.** `uc.Post(h,
ActionMsg{...})` already does everything the delivery half needs; `Query.Focus()` already
established that handing out a handle the caller did not create is acceptable
([ADR 0012](../adr/0012-dangerous-capability-is-named-and-imported.md)'s amendment). The gap
is that `Query.LiveBindings()` returns `[]Binding` and **drops `liveBinding.from`**, so froth's
own help walk cannot tell a caller where an action would go. That is a fact about the current
API, not a recommendation.

**Does not transfer — the mutable global registry.** VS Code's shadowing stack and Emacs'
redefinable function cell both depend on a process-wide mutable table. ADR 0008 already
rejected that shape (*"There is no second override table"*), and `Action` is a bare string
froth *"compares nowhere"*. There is consequently nothing in froth to enumerate: the only
list of live actions is the focus-chain walk, which by construction lists only what is
reachable from where focus is right now. A palette wants the opposite — everything, including
what is unreachable. No surveyed system derives that list from a focus walk, because none of
them has to.

**Does not transfer — synchronous chain search.** AppKit's `sendAction:to:nil` and WPF's
routed `Executed` are synchronous calls into unknown handlers, which
[ADR 0004](../adr/0004-no-synchronous-send.md) forbids outright. Their *precedence* transfers;
their *mechanism* cannot. froth's declarative resolution is already the non-reentrant version
of the same idea.

**Does not transfer — the backstop, cheaply.** AppKit's last resort works because there is
always an application object that may implement the selector. froth's equivalent would have to
be a defined outcome for an action no surface claims, which ADR 0008 already scoped and
costed as part of action bubbling. A palette does not get it for free.

**froth-specific hazards none of these systems have.** Two, both from `binding.go`:

- `claim` **drops any binding left with no keys** so that "nothing that cannot fire is ever
  offered". An action deliberately declared with no key — precisely what a palette-only
  command is — is therefore invisible to the query today. `ActionCancel` is already in this
  position.
- A palette's filter is a **text sink**, so while it holds focus `liveBindings` strips every
  printable key from every binding on the chain. A palette that listed commands from
  `LiveBindings()` would see a different set than the user saw a keystroke earlier, purely
  because the palette opened.

Neither is an argument for or against anything; both are consequences of rules adopted for the
key path that a keyless path would meet immediately.

---

*Sources are primary throughout: vendor documentation and, where docs were thin or
ambiguous, the shipping source (microsoft/vscode, dotnet/wpf, Textualize/textual,
emacs-mirror/emacs, neovim/neovim). Apple's modern reference pages are JS-rendered and were
read through Apple's own DocC JSON backend, which serves the same authored prose; the
archived Cocoa guides were fetched directly. Items that could not be confirmed from a primary
source are marked `[UNVERIFIED]` inline and should be re-fetched before any of them enters an
ADR.*
