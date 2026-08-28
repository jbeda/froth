# Focus, walkability and hit-testing in character-grid TUI toolkits

A prior-art survey of how five terminal UI toolkits declare *what can be focused*, *what
tab can reach*, and *what a click can reach* — and how each finds the widget under a click.

Sources are the toolkits' own repositories, read at these revisions:

| System | Revision surveyed |
|---|---|
| Terminal.Gui v2 (C#) | `gui-cs/Terminal.Gui` (redirects to `tui-cs/Terminal.Gui`), branch `develop` @ `59e6b4d`, 2026-08-24; latest release `v2.4.17` (2026-07-07) |
| Terminal.Gui v1 (C#) | tag `v1.17.1`, files fetched from raw.githubusercontent |
| Textual (Python) | `Textualize/textual` @ `06dbeef`, 2026-07-11; `pyproject.toml` version `8.2.8` |
| tview (Go) | `rivo/tview` @ `c15b79f`, 2026-08-11 |
| prompt_toolkit (Python) | `prompt-toolkit/python-prompt-toolkit` @ `583b341`, 2026-07-26 (≈ 3.0.53) |
| Ratatui (Rust) | issue tracker checked live; no source clone needed (see §A5) |

---

## A. The focusability declaration in each system

### A1. Terminal.Gui v2 — three properties, deliberately decoupled

`View` carries three separate things (`Terminal.Gui/ViewBase/View.Navigation.cs`):

- **`public bool CanFocus`** (line 275) — may this view hold focus at all.
- **`public TabBehavior? TabStop`** (line 1063) — keyboard-walk filter only.
- **`public bool HasFocus`** — current state, backed by `_hasFocus`.

`TabBehavior` (`Terminal.Gui/ViewBase/Navigation/TabBehavior.cs`) has exactly three members
plus a meaningful `null`:

```csharp
public enum TabBehavior { NoStop = 0, TabStop = 1, TabGroup = 2 }
```

`null` means "not yet set"; setting `CanFocus = true` promotes it to `TabStop` as a
convenience, and the docs note the reverse does not happen (`View.Navigation.cs:270-273`,
`1045-1053`).

**This is the one system in the survey that separates walkable from click-focusable as an
explicit, documented design goal.** The `NoStop` doc comment states it outright:

> This flag has no impact on whether the view can be focused via means other than the
> keyboard. Use `View.CanFocus` to control whether a View can focus or not.

and `docfx/docs/navigation.md` ("Decouple Concepts" tenet) says:

> In v1 `CanFocus` is tightly coupled with `HasFocus`, `TabIndex`, `TabIndexes`, and
> `TabStop` and vice-versa. There was a bunch of "magic" logic that automatically attempted
> to keep these concepts aligned. This resulted in a poorly specified, hard-to-test, and
> fragile API. In v2 we strive to keep the related navigation concepts decoupled. […] A view
> with `CanFocus == true` can have `TabStop == NoStop` and still be focusable with the mouse.

The same doc's "Focus Requirements Summary" enumerates: `Visible`, `Enabled`, `CanFocus` —
and `TabStop != NoStop` **for keyboard navigation only**.

**What v1 actually did**, verified in `v1.17.1/Terminal.Gui/Core/View.cs`:

- `TabStop` was a plain `bool` whose setter forced coupling — `tabStop = CanFocus && value;`
  (line 328-335). You could not have a click-focusable non-tab-stop.
- `TabIndex` / `TabIndexes` (lines 269-321) maintained an explicit per-superview index list;
  both are gone in v2.
- The `CanFocus` setter (lines 342-400+) threw if the SuperView had `CanFocus == false`,
  auto-set `SuperView.CanFocus = true` when adding, forced `TabStop = value`, and cascaded
  `CanFocus` down to every subview (caching `oldCanFocus`/`oldTabIndex` for restoration).
  v2's `Add` keeps only the `TabStop` convenience and no longer touches `CanFocus`
  (`navigation.md`, "How Does `View.Add/Remove` Work?").
- `View.MostFocused` (v1 line 1415) walked the tree; v2 replaced it with
  `Application.Navigation.GetFocused()`.

So v2's split of `CanFocus` from `TabStop` is a **deliberate reversal of a shipped v1
decision**, taken because the coupled version was untestable — not an accident.

Nav keys are application-scoped bindings (`Terminal.Gui/App/Application.cs:322`):
`Command.NextTabGroup` = `Key.F6`. (The `TabBehavior.TabGroup` XML comment still says
`Ctrl+PageDown` — stale comment; the binding and `navigation.md` both say F6.)

### A2. Textual — a class attribute *plus* an overridable method

`textual/widget.py`:

```python
can_focus: bool = False            # line 337 — "Widget may receive focus."
can_focus_children: bool = True    # line 339 — "Widget's children may receive focus."
FOCUS_ON_CLICK: ClassVar[bool] = True   # line 330
```

All three are also settable via the class-creation keyword hook
(`widget.py:3879-3900`: `class Focusable(Static, can_focus=True)`).

On top of the attributes sit three *methods* — the asked-at-the-time layer:

- `Widget.allow_focus() -> bool` (line 872-881): "The base class returns `can_focus`. **This
  method may be overridden if additional logic is required.**"
- `Widget.allow_focus_children() -> bool` (line 883-892): same shape over `can_focus_children`.
- `Widget.focus_on_click() -> bool` (line 720-729): "Implement this if you want to change the
  default click to focus behavior. The default will return the classvar `FOCUS_ON_CLICK`."

And the composite, evaluated fresh on every query (`widget.py:2367-2375`):

```python
@property
def focusable(self) -> bool:
    return (not self.loading and self.allow_focus()
            and self.visible and not self._self_or_ancestors_disabled)
```

**Does Textual separate tab-reachable from click-focusable?** Two answers, and both matter:

- `FOCUS_ON_CLICK` / `focus_on_click()` is an explicit, *separate* per-widget axis for
  "a click focuses me", checked only on `MouseDown` (`screen.py:1934-1939`). It is the one
  place Textual names click-focus as its own thing.
- `can_focus_children` produces a *second, accidental* separation. `Screen.focus_chain`
  (`screen.py:790-833`) refuses to descend into a container whose `allow_focus_children()`
  is false, so its descendants never enter the tab ring. But `Screen.set_focus`
  (`screen.py:1126`) gates only on `widget.focusable`, and `get_focusable_widget_at`
  (`screen.py:700-713`) walks `widget.ancestors_with_self` checking only `node.focusable` —
  **neither consults any ancestor's `can_focus_children`**. A focusable child inside a
  `can_focus_children = False` container is therefore *unwalkable but still both
  click-focusable and name-focusable*. This falls out of where the check is placed rather
  than from a stated design intent; I found no doc asserting it.

`AUTO_FOCUS` is a CSS-selector string, not a boolean: `App.AUTO_FOCUS = "*"`
(`app.py:396`), `Screen.AUTO_FOCUS: ClassVar[str | None] = None` (`screen.py:151`) meaning
"inherit the app's". `Screen._update_auto_focus` (`screen.py:1489-1500`) runs on compose and
on screen resume, and focuses the first queried widget whose `.focusable` is true — only
`if self.focused is None`.

`Widget.focus()` (`widget.py:4579-4598`) is deferred: it schedules
`widget.screen.set_focus(self)` via `app.call_later`.

### A3. tview — no focusability declaration at all

The full `Primitive` interface (`primitive.go`), 11 methods, 3 unexported:

```go
type Primitive interface {
	Draw(screen tcell.Screen)
	GetRect() (int, int, int, int)
	SetRect(x, y, width, height int)
	InputHandler() func(event *tcell.EventKey, setFocus func(p Primitive))
	Focus(delegate func(p Primitive))
	HasFocus() bool
	Blur()
	MouseHandler() func(action MouseAction, event *tcell.EventMouse, setFocus func(p Primitive)) (consumed bool, capture Primitive)
	PasteHandler() func(text string, setFocus func(p Primitive))
	focusChain(chain *[]Primitive) bool
	focused()
	blurred()
}
```

The unexported methods seal the interface: outside code cannot implement `Primitive`
directly, only embed `Box`.

There is **no `focusable` boolean anywhere**. Focusability is inferred structurally: a
primitive is focusable iff `InputHandler()` returns non-nil (documented at
`primitive.go:16-22`); `Box.InputHandler` returns `nil` (`box.go:212`). `Box.hasFocus`
(`box.go:57`) is state, not capability. `HasFocus()` for a `Box` is derived by walking
(`box.go:550-562`).

`Application.SetFocus` (`application.go:839-887`) blurs the old chain leaf, then calls
`p.Focus(delegate)` where `delegate = func(p) { a.SetFocus(p) }` — so a container can
recursively redirect focus down to a descendant. There is **no application-level tab
machinery**: key dispatch is `if root.HasFocus() { root.InputHandler()(event, setFocus) }`
(`application.go:436-443`), and every container's `InputHandler` just forwards to whichever
child reports `HasFocus()` (`flex.go:256-269`, `grid.go:664-677`, `pages.go:338-349`).
`Tab`/`Backtab` are handled **only inside `Form`** (`form.go:801-861`), cycling modulo over
`f.items` + `f.buttons`. `Flex` and `Grid` contain no `KeyTab` references at all.

`SetFocusFunc` / `SetBlurFunc` (`box.go:503-521`) are notification hooks fired from
`focused()`/`blurred()`; they do not participate in the decision.

### A4. prompt_toolkit — focusability is a *filter*, on the control

Focusability lives on `UIControl`, not on `Window` or `Container`. `Window` has no
`focusable` attribute.

- `UIControl.is_focusable()` — base returns `False` (`layout/controls.py:85-89`).
- `FormattedTextControl.__init__(..., focusable: FilterOrBool = False, ...)`
  (`controls.py:298`), stored as `self.focusable = to_filter(focusable)` (`controls.py:306`),
  and `is_focusable()` returns `self.focusable()` — **a fresh call every time**
  (`controls.py:327-328`).
- `BufferControl.__init__(..., focusable: FilterOrBool = True, ...)` (`controls.py:521`),
  `self.focusable = to_filter(focusable)` (`controls.py:540`), `is_focusable()` returns
  `self.focusable()` (`controls.py:596-597`).

Every call site invokes the method live and caches nothing: `layout/layout.py:124, 149, 246`;
`application/application.py:433, 435`.

Walk order is document order — `Layout.walk()` (`layout.py:396-410`) is a plain pre-order
DFS over `container.get_children()`. `focus_next` / `focus_previous` (`layout.py:306, 322`)
index into `get_visible_focusable_windows()` (`layout.py:249`), which is
`get_focusable_windows()` (`layout.py:240`, filtering by `walk_through_modal_area()` and
`w.content.is_focusable()`) intersected with `self.visible_windows`, order preserved.
`current_window` / `current_control` are backed by `self._stack[-1]` (`layout.py:193-219`).

There is **no separation of walkable from click-focusable** — but click-focus is not
automatic either (§C4), so the axes never collide.

### A5. Ratatui — nothing to say

Ratatui's core crate has no focus model and no hit-testing; it is immediate mode and widgets
persist no identity. Maintainers state this directly on the mouse-support tracking issue
(`ratatui/ratatui#273`): widgets have no identifier and rendering records nothing about
location; apps are expected to keep the `Rect`s they passed to their own draw calls and hit-test
themselves. Adding `hover`/`clicked` helpers on `Rect` is proposed in `#1050`/`#1051` but is
unmerged discussion, not shipped API. Focus and click registries exist only as third-party
crates (`rat-focus`, `ratatui-interact`).

---

## B. Static flag, or a question asked at the time?

| System | Shape |
|---|---|
| Terminal.Gui v2 | **Static properties, plus a cancellable event.** `CanFocus`/`TabStop` are plain settable properties. The "ask" hook is refusal, not permission: `OnHasFocusChanging` / the `HasFocusChanging` event can set `e.Cancel = true` to veto an incoming focus change (`docfx/docs/navigation.md`, "Flexible Overrides"); `ApplicationNavigation.FocusedChanging` does the same app-wide. `SetFocus()` returns `bool` in v2 precisely so a caller can see the veto — v1's returned nothing. |
| Textual | **Both, layered.** Class attribute `can_focus` is the declaration; `allow_focus()` / `allow_focus_children()` / `focus_on_click()` are documented override points for "additional logic"; `focusable` is a property recomputed on each access, folding in `loading`, `visible` and ancestor-disabled. This is the closest thing in the survey to the `WM_GETDLGCODE` shape: a default that a widget may answer differently at the moment it is asked. |
| tview | **Neither — it is structural.** "Can this be focused" is not stored or asked; it is whether `InputHandler()` happens to return a non-nil func. That *is* an asked-at-the-time question, but the answer is a function value rather than a bool, and nothing in the framework consults it before focusing (see §C4 — `Box` focuses on click despite a nil `InputHandler`). |
| prompt_toolkit | **Asked at the time, by construction.** `focusable` is a `FilterOrBool` normalised through `to_filter()` and invoked on every query. A control can be focusable only while a buffer is non-empty, only in a given app mode, etc., with no event and no invalidation step. |
| Ratatui | N/A |

**Did any of them change from one to the other?** Only Terminal.Gui, and not along this axis:
v1→v2 moved from *one coupled flag with implicit cascade* to *two decoupled flags plus a
cancellable event*. Textual's `allow_focus()` method layer sits over a class attribute that
has been there since early versions; I did not establish when the method hook was added —
**UNVERIFIED**. prompt_toolkit's filter shape is idiomatic to the whole library (filters are
used for key bindings, visibility, style conditions) and shows no sign of having been a bool
first — **UNVERIFIED** as to history.

---

## C. Hit-test transparency

### C1. How the click target is found

**Terminal.Gui v2** — a tree walk producing a *depth-ordered list*, then take the last.
`View.GetViewsUnderLocation` (`ViewBase/View.Layout.cs:1390`) tries, in order: the active
popover; then every runnable in `App.SessionStack` topmost-first, returning the first
non-empty hit; then a fallback to `TopRunnableView`. The per-root helper
`GetViewsAtLocation` (`View.Layout.cs:1559-1634`) is an explicit stack-based DFS that pushes
adornments (Padding, Border, Margin) before subviews, and pushes subviews in reverse index
order, guarded by `subview is { Visible: true, Enabled: true } && subview.FrameToScreen().Contains(location)`.
The result list is "ordered by depth. The deepest View is at the end of the list."
`ApplicationMouse.RaiseMouseEvent` then does
`View? deepestViewUnderMouse = currentViewsUnderMouse?.LastOrDefault();`
(`App/Mouse/ApplicationMouse.cs:106-108`). So: depth-first, deepest-wins, with sibling
reverse-order as the stand-in for Z.

**Textual** — a per-row, z-ordered list, first match wins. `Compositor.layers_visible`
(`_compositor.py:768-792`) builds `list[list[tuple[Widget, cropped_region, region]]]`
indexed by screen row, appending each visible widget to every row it covers, in layer order.
`get_widget_at` (`_compositor.py:829-848`) is then:

```python
for widget, cropped_region, region in self.layers_visible[int(y)]:
    if contains(cropped_region, x, y) and widget.visible:
        return widget, region
raise errors.NoWidget(...)
```

That is a **z-descending scan over a precomputed per-row candidate list** — closer to a
per-cell ownership map than to a tree walk, but it is not a map: it is a linear scan of the
row's widgets. `get_widgets_at` is the generator form, yielding all of them in the same order.

**tview** — recursive descent, first consumer wins, no separate hit-test phase.
`Application.fireMouseActions` (`application.go:522-563`) hands the event to exactly one
primitive (the captured one, else `a.root`), nil-checking `primitive.MouseHandler()`. Every
container walks its own children inside its `MouseHandler`. `Box.InRect`
(`box.go:309-312`) is the geometric gate each widget applies first. `Pages.MouseHandler`
(`pages.go:316-335`) iterates `len(p.pages)-1` down to `0` — genuine z-descending. `Flex` and
`Grid` iterate document order, which for tiled layouts is equivalent.

**prompt_toolkit** — a real per-cell ownership map. `MouseHandlers`
(`layout/mouse_handlers.py`) is `defaultdict[int, defaultdict[int, MouseHandler]]`, row → col
→ callback, defaulting to a `dummy_callback` that returns `NotImplemented`.
`set_mouse_handler_for_range` (lines 42-53) blindly overwrites a rectangle of cells. **Only
`Window.write_to_screen` ever registers handlers** (`layout/containers.py:1876-1882`);
`HSplit`, `VSplit` and `FloatContainer` never do. Last writer for a cell wins, and floats
draw after body content (`FloatContainer.write_to_screen` defers via
`screen.draw_with_z_index`, `containers.py:802-843`), so overlap is resolved by *draw order*,
not by any comparison at click time. Dispatch is a direct index —
`handler = event.app.renderer.mouse_handlers.mouse_handlers[y][x]` (in
`key_binding/bindings/mouse.py`) — with no traversal at all.

### C2. Can a widget declare itself not a mouse target?

**Terminal.Gui v2: yes, explicitly.** `ViewportSettingsFlags.TransparentMouse`
(`ViewBase/ViewportSettingsFlags.cs:156-165`):

> If set the View will be transparent to mouse events: Specifically, any mouse event that
> occurs over the View that is NOT occupied by a SubView will not be captured by the View.

It pairs with `ViewportSettingsFlags.Transparent` (line 141-154), which is the *visual*
version — the two are separate flags and the docs tell you to combine them if you want both.
`ApplicationMouse.RaiseMouseEvent` passes `ViewportSettingsFlags.TransparentMouse` into
`GetViewsUnderLocation` (`ApplicationMouse.cs:106`, and again at line 464 for enter/leave),
so the filtering happens **during hit-testing, before dispatch**. The filter also does a
per-cell refinement for adornments: a view is removed only if the point is not inside the
adornment's `CachedDrawnRegion` (`View.Layout.cs:1480-1520`), i.e. clicks on *drawn* border
cells still land while clicks on undrawn cells fall through.

**Textual: no.** `grep -rn "pointer_events\|pointer-events" src/textual/` returns nothing.
The only ways out of the hit test are `visible` (CSS `visibility: hidden`) and `display: none`
— both of which also stop the widget being drawn. There is no "drawn but not a mouse target".
There is a narrow special case: `get_focusable_widget_at` returns `None` early for widgets
with the `-textual-system` class or `loading` set (`screen.py:705-708`), so those do not take
focus on click — but they still receive the event.

**tview: documented but effectively fictional.** `primitive.go:58-65` says returning a nil
func value from `MouseHandler()` "stop[s] the downward propagation of mouse events". No stock
widget does this — every one wraps via `WrapMouseHandler`, which never returns nil. And the
containers do not guard: `Flex.MouseHandler` (`flex.go:234-253`), `Grid.MouseHandler`
(`grid.go:642-661`) and `Pages.MouseHandler` (`pages.go:316-335`) all call
`item.Item.MouseHandler()(...)` with no nil check. Only `Application.fireMouseActions`
(`application.go:544`) nil-checks, and only for the root. A nested primitive returning nil
would panic. This reads as an inconsistency between the interface contract and the
implementation, not a feature.

**prompt_toolkit: yes, but scoped to modality, not to the widget.** See §C5. A window
outside the active modal area refuses every mouse event (`containers.py:1822-1825`). There is
no per-window "I am not a target" flag independent of that.

### C3. Skip-to-beneath versus bubble-to-parent

This distinction separates cleanly and the systems land in different places.

- **Terminal.Gui v2 implements *both*, as separate mechanisms.**
  *Skip-to-beneath* is `TransparentMouse`, applied inside the hit test: the view is removed
  from the candidate list, so the deepest remaining view — which may be an entirely unrelated
  view underneath — becomes the target.
  *Bubble-to-parent* is the dispatch loop (`ApplicationMouse.cs:244-272`):

  ```csharp
  while (deepestViewUnderMouse.NewMouseEvent (viewMouseEvent) is not true && _mouseGrabViewRef is null)
  {
      if (deepestViewUnderMouse is AdornmentView adornmentView)
          deepestViewUnderMouse = adornmentView.Adornment?.Parent?.SuperView;
      else
          deepestViewUnderMouse = deepestViewUnderMouse.SuperView;
      if (deepestViewUnderMouse is null) break;
      // rebuild viewMouseEvent with the new view's viewport-relative point …
  }
  ```

  A view that is hit but returns `false` sends the event to its **SuperView**, not to
  whatever is beneath it in Z. The two behaviours are named separately and do different
  things — this is the only system that gets both.

- **Textual: bubble-to-parent only.** `get_widget_at` returns exactly one widget; the event
  is forwarded to it (`screen.py:1944-1949`), and `MouseEvent` and all its subclasses are
  declared `bubble=True` (`events.py:331, 572-620`), so an unhandled click walks up the DOM
  ancestors. Nothing ever reconsiders the widget beneath.

- **tview: neither, exactly — it is one recursive search with backtracking.** Returning
  `consumed == false` does not bubble; it means "my caller's loop tries its next child". A
  container that fails to consume returns `false` to *its* parent's loop, which resumes
  iterating its own list. Because `Pages` iterates z-descending, the practical effect for
  overlapping pages *is* skip-to-beneath; for `Flex`/`Grid` it is document-order fallthrough.
  There is no separate propagation phase and no ancestor ever gets a second look at an event
  a descendant declined — it is depth-first search, first consumer wins.

- **prompt_toolkit: neither, structurally.** The `Window.write_to_screen` closure
  (`containers.py:1816-1874`) wraps the control's handler; on `NotImplemented` it falls back
  to `Window._mouse_handler` (`containers.py:2543-2558`), which handles scroll and otherwise
  returns `NotImplemented` again. That is the entire chain. Ancestors never registered a
  handler over those cells, so there is nothing to bubble to, and the cell map holds one
  handler so there is nothing beneath to skip to.

### C4. Does a click on a non-focusable widget move focus?

- **Terminal.Gui v2 — to the immediate SuperView, one level.** `View.Mouse.cs:734-749`:

  ```csharp
  if (!HasFocus)
  {
      if (CanFocus)
          SetFocus ();                                   // don't invoke Accept
      else if (SuperView is { CanFocus: true, HasFocus: false, Enabled: true } superView)
          superView.SetFocus ();
  }
  ```

  Note this checks the *immediate* SuperView only, not the nearest focusable ancestor. If the
  SuperView is also non-focusable, focus does not move.

- **Textual — to the nearest focusable ancestor, walking all the way up.**
  `get_focusable_widget_at` (`screen.py:687-713`) walks `widget.ancestors_with_self` and
  returns the first `node.focusable`; the caller focuses it only if
  `focusable_widget.focus_on_click()` is also true (`screen.py:1934-1939`), and only on
  `MouseDown`. A click that lands on nothing at all does `self.set_focus(None)`
  (`screen.py:1931-1932`) — clicking empty space *removes* focus.

- **tview — yes, and it is arguably a bug.** `Box.MouseHandler` (`box.go:276-284`) calls
  `setFocus(b)` on any `MouseLeftDown` inside its rect, even though `Box.InputHandler()`
  returns nil. A plain `Box` becomes "focused" on click while having no keyboard behaviour
  whatsoever. Nothing consults focusability before focusing.

- **prompt_toolkit — no, click-to-focus is opt-in per control.** There is no generic rule.
  `BufferControl` has `focus_on_click` (default `False`, `controls.py:526, 541`) and its
  `mouse_handler` sets `get_app().layout.current_control = self` only when unfocused, the
  filter is true, and the event is `MOUSE_UP`. `FormattedTextControl` has no such concept.
  Widgets that want it call `layout.focus(...)` explicitly from their own handlers
  (`widgets/toolbars.py:171, 177`; `widgets/menus.py:250`).

### C5. Modals and overlays that swallow clicks

- **Terminal.Gui v2 — popovers swallow-and-dismiss; there is no `Modal` flag.**
  `grep -rn "bool Modal" Terminal.Gui/` returns nothing; v1's `Toplevel.Modal`
  (`v1.17.1/Terminal.Gui/Core/Toplevel.cs:365`) was **removed** in v2. Modality now comes
  from two places. (a) The runnable stack: `GetViewsUnderLocation` walks `App.SessionStack`
  topmost-first and returns the first runnable that produces a hit — so a click *outside* a
  pushed dialog falls through to the runnable beneath it, i.e. runnables do **not** form a
  click-blocking scrim. (b) Popovers: a press outside the active popover hides it and then
  **re-dispatches the same event** (`ApplicationMouse.cs:152-177`) — the popover consumes the
  press as a dismissal, and the click reaches the view underneath on the recursive pass.
  Popover hit-testing also short-circuits everything else (`View.Layout.cs:1392-1404`), with
  an in-source `// BUGBUG` noting that other visible runnables are not traversed while a
  popover is up.

- **Textual — `ModalScreen` (`screen.py:2158-2173`) is about key bindings and a dimmed
  background**, not about clicks. Its docstring: "A screen with bindings that take precedence
  over the App's key bindings." Click containment is automatic and geometric: only the top
  screen is composited and hit-tested, so nothing beneath it is reachable regardless.

- **tview — stock `Modal` does not block outside clicks.** `Modal.Draw`
  (`modal.go:162-195`) overrides its rect to a small centered box; `Modal.MouseHandler`
  (`modal.go:198-207`) forwards to its internal form and consumes only if the click is inside
  that small `InRect`. A click outside falls through to whatever `Pages` item is beneath it
  (`pages.go:322-330`). Building a scrim is left to the caller.

- **prompt_toolkit — modality is a focus-scoping filter that also gates the mouse.**
  `Container.is_modal()` defaults `False` (`containers.py:131-134`); `HSplit`, `VSplit` and
  `FloatContainer` take a plain `modal: bool` ctor param (e.g. `containers.py:221, 225`) —
  note it is a bool, not a `Filter`, unlike `focusable`. `Layout.walk_through_modal_area()`
  (`layout.py:344-352`) walks *up* from `current_window` to the nearest modal ancestor and
  then DFS-walks down from there. It is used twice: to restrict Tab candidates
  (`layout.py:245`), and in the window mouse closure (`containers.py:1822-1825`):

  ```python
  if self not in get_app().layout.walk_through_modal_area():
      return NotImplemented
  ```

  So windows outside the modal region refuse all mouse events. Crucially this is keyed off
  **where focus currently is**, not off z-order or geometry.

---

## D. Containers focus treats as one thing, and place-memory

### D1. Terminal.Gui v2 — a real two-level model, and universal automatic memory

`TabBehavior.TabGroup` is exactly the "sidebar is one thing you are inside of" construct.
Per `docfx/docs/navigation.md`:

- `TabStop` — "a focusable view with no focusable subviews. `NextTabStop`/`PreviousTabStop`
  keys will advance ONLY through the peer-Views (`SuperView.SubViews`)."
- `TabGroup` — "a focusable container for other focusable views and enables keyboard
  navigation across these containers. This applies to both tiled and overlapped views."
  `FrameView` ships as `TabGroup` + `Arrangement.Fixed`; `Window` ships as `TabGroup` +
  `Movable | Resizable | Overlapped`.
- Tab / Shift+Tab walk tab stops; **F6 / Shift+F6** walk tab groups
  (`Application.cs:322`). Arrow keys are aliased to next/previous tab stop.
- A `NoStop` SuperView blocks group traversal into its subtree.

**Focus memory is automatic, universal, and lives on every `View`.**
`public View? PreviouslyFocused { get; private set; }` (`View.Navigation.cs:569`), documented
as "When this view loses focus, this property is set to the subview that had focus before the
view lost focus… This is set to `null` when the view gains focus." It is written
unconditionally in the focus-gaining path — `superViewOrParent?.PreviouslyFocused = this;`
(`View.Navigation.cs:895`) — so *every* container gets it whether or not it is a `TabGroup`.
It is consumed in three places:

- `RestoreFocus()` (`View.Navigation.cs:433-451`) — "Ignore TabStop": it builds the focus
  chain with `behavior: null`, so restoration is **not filtered by `TabStop`**. A `NoStop`
  child that was last focused by mouse will be restored by a later focus of its container.
- `SetHasFocusTrue` (`:654`) — on gaining focus, try `RestoreFocus()` first, and only if that
  fails fall back to `AdvanceFocus(Forward, null)`.
- The tab-group advance path (`View.Navigation.cs:157-163`) — entering a group restores that
  group's `PreviouslyFocused` if it is still in the group's `TabStop` chain, else its first
  tab stop:

  ```csharp
  if (focusChain[i].PreviouslyFocused is null || subViews.All (v => v != focusChain[i].PreviouslyFocused))
      return subViews[0].SetFocus ();
  return focusChain[i].PreviouslyFocused!.SetFocus () || subViews[0].SetFocus ();
  ```

`public void ClearFocus() => PreviouslyFocused = null;` (`:456`) is the escape hatch, and
`navigation.md` flags the known hazard: "there must be logic that removes the focus-cache used
by `RestoreFocus()` if something changes that makes the previously-focusable view not
focusable (e.g. if `Visible` has changed)."

Mouse focus goes through the same machinery — `navigation.md`: "If the View was previously
focused, the system keeps a record of the SubView that was previously most-focused and
restores focus to that SubView (`RestoreFocus()`). If the View was not previously focused,
`AdvanceFocus()` is called."

### D2. Textual — no tab groups, no container memory

`can_focus_children = False` on a container removes its whole subtree from `focus_chain`
(`screen.py:816`), which is the "container is one thing" gesture — but there is no second key
that walks between such groups, and no arrow-within convention at the framework level.
Widgets that behave like groups (`Tabs`, `OptionList`, `DataTable`) implement their own arrow
keys internally.

The only focus memory is `Screen.focused` — `Reactive[Widget | None]`
(`screen.py:229`). Because it is per-`Screen`, pushing and popping screens preserves each
screen's own focus for free, and `_update_auto_focus` on resume only fires `if self.focused
is None` (`screen.py:1495`). There is also `App._last_focused_on_app_blur`
(`app.py:844, 4427-4446`) for restoring focus when the terminal loses and regains focus. I
found **no per-container last-focused-child memory** — `grep -rn "_last_focused\|_previously_focused"`
over `src/textual/*.py` returns only those app-level entries. Focusing a container in Textual
means focusing that widget itself, not delegating into it.

### D3. tview — one container remembers, and only via static config elsewhere

- `Flex.Focus` (`flex.go:207-215`) delegates to whichever item had `focus bool` passed to
  `AddItem(item, fixedSize, proportion, focus)` (`flex.go:96-99`) — **static configuration,
  not memory**. Refocusing always lands on the same configured item.
- `Grid.Focus` (`grid.go:249-257`) — identical static `item.Focus` flag.
- `Pages.Focus` (`pages.go:283-298`) — always the current topmost visible page.
- `Form.Focus` (`form.go:702-724`) — the one genuine exception: a persistent
  `f.requestedFocus` int field reused across blur/refocus cycles, skipping items that have
  since become `Disabled`.

No tab-group notion anywhere; `Flex`/`Grid`/`Pages` contain no `KeyTab` handling at all.

### D4. prompt_toolkit — an append-only focus history, not per-container slots

`Layout._stack: list[Window]` (`layout.py:46`) is appended to on **every** `current_window`
assignment (setter at `layout.py:217-219`) — a full history, never replaced in place.
`Layout.focus(container)` on a non-`Window` container (`layout.py:141-159`) collects the
container's focusable descendant windows, scans `reversed(self._stack)` for the most recently
focused among them, and restores it; falling back to the first focusable window if none was
ever focused. So place-memory is **emergent from a global back-stack**, not stored on the
container. `Layout.focus_last()` (`layout.py:299-304`) pops the stack.

Modal containers scope the tab ring (§C5), which is the closest thing to a tab group — but
there is no second key that moves between modal regions; modality is entered by focusing into
it, not walked to.

---

## E. What the terminal forces

Short version: the mouse protocol delivers **cell coordinates, a button/modifier bitfield and
press/release**, and that is nearly all. Textual's Linux driver
(`drivers/linux_driver.py:128-141`) enables `?1000h` (VT200 button events), `?1003h`
(any-event / motion reporting), `?1015h` (urxvt extended) and `?1006h` (SGR extended) —
SGR extended being what lifts the 223-column coordinate ceiling of the legacy encoding.
Textual can additionally opt into `?1016h` (SGR-Pixels) via `_enable_mouse_pixels`
(`linux_driver.py:144-150`), which reports pixel rather than cell coordinates — the only
sub-cell information available anywhere in this survey, and Textual uses it for smoother
drag/resize rather than for hit testing.

Consequences visible in the toolkits:

- **Motion and hover are available** (`?1003h`), so all four non-Ratatui toolkits implement
  enter/leave: Textual's `MouseMove`/`Enter`/`Leave`, Terminal.Gui's
  `RaiseMouseEnterLeaveEvents` (`ApplicationMouse.cs:242-244`, which re-runs the same
  `GetViewsUnderLocation` with the same `TransparentMouse` filter at line 464 — hover and
  click therefore share one hit-test definition).
- **The event carries no target**, so every toolkit must derive one geometrically. That is why
  all four hit-test designs are pure geometry over the layout, and why prompt_toolkit can get
  away with a write-during-render cell map: there is no source of truth to reconcile against.
- **Drag needs an explicit capture**, since the protocol will happily report motion over a
  different widget. Every toolkit has one: Terminal.Gui `App.Mouse.GrabMouse` /
  `HandleMouseGrab` (`ApplicationMouse.cs:528-560`), Textual `app.mouse_captured`
  (`screen.py:1930`), tview's `capture Primitive` return value becoming
  `a.mouseCapturingPrimitive` (`application.go:562`), prompt_toolkit's per-cell map plus
  application-level drag state.
- Terminal.Gui v2 **removed** v1's `WantMousePositionReports` and
  `WantContinuousButtonPressed` (`v1.17.1/Core/View.cs:429, 434`) — `grep` finds neither in
  the v2 tree — replacing them with `MouseState` and `IMouseHoldRepeater`
  (`ViewBase/Mouse/`). The v1 names exposed a protocol-level toggle as a per-view property;
  v2 does not.

---

## F. Summary table

| | focus declaration | walkable vs click-focusable separated? | static flag or asked-at-the-time | hit-test opt-out | skip-to-beneath vs bubble-to-parent | remembers focused child | tab-group notion |
|---|---|---|---|---|---|---|---|
| **Terminal.Gui v2** | `CanFocus` (bool) + `TabStop` (`TabBehavior?`: `NoStop`/`TabStop`/`TabGroup`/null) + `HasFocus` (state) | **Yes, explicitly and by design.** `CanFocus=true, TabStop=NoStop` = mouse/code-focusable, not tab-reachable. Programmatic and click focus are not separated from each other | Static properties + cancellable `HasFocusChanging` / `FocusedChanging` events (veto, not permission); `SetFocus()` returns bool | **Yes** — `ViewportSettingsFlags.TransparentMouse`, applied during hit test, with per-cell refinement for adornments | **Both, as distinct mechanisms.** `TransparentMouse` = skip-to-beneath; unhandled event = bubble to `SuperView` (`ApplicationMouse.cs:244`) | **Yes, automatic on every View** — `PreviouslyFocused` + `RestoreFocus()`; restoration ignores `TabStop` | **Yes** — `TabGroup` + F6/Shift+F6; group entry restores that group's `PreviouslyFocused` |
| **Terminal.Gui v1** | `CanFocus` + `bool TabStop` (setter forced `CanFocus && value`) + `TabIndex`/`TabIndexes` | **No** — coupled by construction | Static, with implicit cascade through `Add` and the `CanFocus` setter | UNVERIFIED (not surveyed) | UNVERIFIED (not surveyed) | UNVERIFIED | `Toplevel.Modal` bool (removed in v2); no tab groups |
| **Textual 8.2.8** | `can_focus` / `can_focus_children` class attrs; `FOCUS_ON_CLICK`; methods `allow_focus()`, `allow_focus_children()`, `focus_on_click()`; property `focusable` | **Partly, two ways.** `focus_on_click()` is a deliberate click-focus axis. `can_focus_children=False` makes children unwalkable while still click- and name-focusable — an *accident of where the check sits*, not a stated design | **Both** — class attribute as declaration, overridable method as the live answer; `focusable` recomputed per access | **No** — no `pointer-events` equivalent; only `visible`/`display`, which also stop drawing | **Bubble-to-parent only** — one widget from `get_widget_at`, then DOM bubbling (`MouseEvent(bubble=True)`) | **No per-container memory.** Only `Screen.focused` (per screen) and `App._last_focused_on_app_blur` | **No.** `can_focus_children=False` groups a subtree out of the ring, but nothing walks between groups |
| **tview** (`c15b79f`) | **None.** Focusable iff `InputHandler()` returns non-nil | **No** — and the two disagree: `Box` focuses on click yet has a nil `InputHandler` | Structural — a function value, asked at the time, but never consulted before focusing | Documented (nil `MouseHandler` func, `primitive.go:58-65`) but unused by every stock widget and unguarded in `Flex`/`Grid`/`Pages` — would panic. Effectively **no** | **Neither.** One recursive DFS, first consumer wins; `false` resumes the *caller's* loop over its own children. `Pages` iterates z-descending so overlap behaves like skip-to-beneath | **Only `Form`** (`requestedFocus`). `Flex`/`Grid` use a static `focus bool` set at `AddItem`; `Pages` uses top page | **No.** Tab exists only inside `Form` |
| **prompt_toolkit 3.0.53** | `UIControl.is_focusable()`; `focusable: FilterOrBool` on `BufferControl` (default True) and `FormattedTextControl` (default False). Lives on the *control*, not the window | **No** — but click-to-focus is opt-in per control (`focus_on_click`, default False), so the axes rarely collide | **Asked at the time**, idiomatically: `to_filter()` normalises to a callable invoked on every query; no cached bool anywhere | Effectively yes, but scoped to modality: a window outside `walk_through_modal_area()` returns `NotImplemented` for all mouse events. No per-window flag | **Neither.** One handler per cell (control → same-window fallback). Ancestors never register handlers, so nothing to bubble to; the map holds one entry, so nothing to skip to | **Yes, emergent** — the append-only `Layout._stack` history; `focus(container)` restores the most recent stack entry inside it. Not a per-container slot | **No.** `modal=True` scopes the tab ring but is entered by focusing in, not walked to |
| **Ratatui** | None — immediate mode | N/A | N/A | N/A | N/A | N/A | N/A |

---

## Verdict: what transfers

**The three-axis hypothesis is confirmed by exactly one system, and that system arrived there
by reversing itself.** Terminal.Gui v2's `CanFocus` × `TabBehavior` is a deliberate,
documented split of "can hold focus" from "keyboard walks to it", made because v1's coupled
version (`tabStop = CanFocus && value`) was, in the maintainers' own words, "poorly specified,
hard-to-test, and fragile". It is the strongest evidence in the survey that at least axes 1
and 2/3 want separating. But note what it does *not* separate: `CanFocus` covers both
programmatic focus and click focus with one flag. **No system in this survey separates
"focusable by name" from "clickable".** Terminal.Gui folds them into `CanFocus`; Textual folds
them into `focusable` and then adds `focus_on_click()` on top as a click-only *suppressor*;
prompt_toolkit's `focus_on_click` is likewise a per-control opt-in over a single focusability
answer. The recurring shape is **one capability flag plus a click-specific modifier**, not
three peers.

**Hit-test opt-out is rare and was added late where it exists.** Only Terminal.Gui v2 has a
working, first-class one, and it is careful about two things worth copying: it is a *separate
flag* from visual transparency (`TransparentMouse` vs `Transparent`, with the docs telling you
to combine them), and it is applied **during hit testing**, so hover, enter/leave and click all
agree on what is transparent. Textual, the most CSS-shaped system here, deliberately has no
`pointer-events` — that absence in a toolkit whose whole model is CSS is a meaningful negative
data point, not an oversight of an unsophisticated system. tview's version is documented and
broken, which is an accident of history, not a design.

**Skip-to-beneath and bubble-to-parent are genuinely different and Terminal.Gui v2 is the only
one that ships both.** The distinction is worth naming because the fallbacks answer different
questions: bubbling asks "who else in my ownership chain wants this?", skipping asks "what is
actually underneath?". Textual chose bubbling only, which is coherent with a DOM/CSS model and
means an unrelated widget under an overlay is unreachable by design. tview's design collapses
the question — a declined event resumes the *searcher's* loop rather than starting a new
phase, which is simpler and arguably the cleanest of the four, but it gives the application no
way to express either intent explicitly.

**Container place-memory is settled prior art, and it is universal, not opt-in, in the one
system that takes it seriously.** Terminal.Gui v2 puts `PreviouslyFocused` on *every* `View`
and writes it unconditionally on focus loss; `TabGroup` is only the thing that makes it
*visible* to the user, not the thing that enables it. Two details are load-bearing: (1)
`RestoreFocus()` deliberately ignores `TabStop`, so a child reached only by mouse is still
restorable — which only makes sense in a world where walkable and clickable are separate; (2)
the documentation flags the cache-invalidation hazard (a remembered child that later becomes
invisible or non-focusable) as a known requirement, which is the real cost of the feature.
prompt_toolkit gets equivalent behaviour for free from an append-only focus history, at the
price of unbounded growth and no per-container control. Textual has nothing here, and neither
does tview outside `Form` — in both cases because their containers are not focus delegates at
all, which is a structural choice upstream of the memory question.

**Modality does not come from a per-window "swallow clicks" flag in any current system.**
Terminal.Gui v2 *removed* v1's `Toplevel.Modal` bool. What replaced it: geometry plus a
runnable stack (which does *not* block outside clicks) and popovers (which consume the outside
press as a dismissal, then **re-dispatch the same event** so the click still lands underneath —
a specific, copyable behaviour). Textual gets containment for free because only the top screen
is composited. prompt_toolkit's modality is the most interesting outlier: it is a *focus-scoping*
construct that gates the mouse as a side effect, keyed off where focus currently is rather than
z-order — meaning a click is refused because of where the keyboard is, which is a coupling the
other three avoid.

**Static-vs-asked is not settled, and the two answers correlate with what else the toolkit
believes.** prompt_toolkit's filters are asked-at-the-time everywhere (key bindings, visibility,
styles), so `focusable` being a filter is consistency, not a focus-specific insight — and
notably `modal` in the *same* constructors is a plain bool, which suggests the filter treatment
was applied where it was cheap rather than where it was reasoned about. Textual's layering
(class attribute → overridable method → derived property) is the closest match to the
`WM_GETDLGCODE` shape and is the version that costs a caller nothing until they need it.
Terminal.Gui v2 chose static properties and put the dynamism in a *veto* event instead — which
is a different answer to the same problem: rather than asking permission, it lets the widget
refuse after the fact. That is worth weighing: a veto composes with programmatic focus,
click focus and tab walking uniformly, because all three funnel through `SetFocus`.

---

## Footnote: Textual issue #877

froth's notes describe `Textualize/textual#877` as a filed bug about focus-border reflow. What
it actually is: **"Applying border on focus CSS removes content"**, opened 2022-10-12, **closed**
the next day with no labels and no maintainer explanation in the thread — only a bot comment.
The report is that a widget with `padding: 3 6` and a `:focus` border shows no content once
focused. The posted CSS uses a descendant selector, `Focusable :focus`, not `Focusable:focus`,
which would not match the widget at all — the quick close without discussion is consistent with
that being the resolution, but **this is inference: the issue thread contains no statement of
cause and the outcome is UNVERIFIED.** It is not usable as evidence about Textual's focus model.
