# Focus and hit-test axes in retained-mode GUI toolkits: Qt, GTK4, Android, Flutter

Research note for froth. Every substantive claim is sourced to a primary doc page or a source
file that was fetched and read in this session; see [Sources](#sources). Claims the sources do
not settle are marked **UNVERIFIED**.

**Versions described.** Qt 6.11 (6.11.2 released; source line numbers from the `qt/qtbase` `dev`
branch). GTK 4 (`GNOME/gtk` `main`, fetched 2026-08-27). Android — AOSP `frameworks/base`
`refs/heads/main`; current API level is Android 17 / API 37, but see the UNVERIFIED note in
§Version caveats. Flutter 3.47 stable per `docs.flutter.dev/release/release-notes`, with source
read from `flutter/flutter` `main` rather than the 3.47.0 tag.

---

## Core findings up front

**1. The three-axis hypothesis is confirmed, but no system factors all three the same way, and
two of them contradict each other on axis 2.**

- **Qt factors walkable from click-focusable inside one enum, as literal independent bits**
  (`TabFocus = 0x1`, `ClickFocus = 0x2`). This is the sharpest confirmation in the survey.
- **Flutter factors walkable from focusable-by-name** (`skipTraversal` vs `canRequestFocus`) —
  and these are exactly froth's axes 1 and 2, checked in different code paths.
- **Qt and Flutter disagree on whether programmatic focus respects the declaration.** Qt's
  `setFocus()` never reads `focusPolicy` at all; Flutter's `requestFocus()` returns early when
  `canRequestFocus` is false. GTK4 and Android side with Flutter and refuse.
- **Nobody factors "clickable" as a per-widget focus bit except Qt.** GTK4 has
  `focus-on-click`, but that is a *behavior* flag on an already-focusable widget, not a
  permission. Android derives focusability *from* clickability rather than the reverse.

**2. Hit-test opt-out is a real, named, per-widget property in three of four systems** —
Qt `WA_TransparentForMouseEvents`, GTK4 `can-target`, Flutter `IgnorePointer` — and all three
mean *"skipped, so the search continues to what is beneath."* **Android has no such
mechanism at all.**

**3. "Hit-tested but forwards to the parent" and "skipped so the search continues beneath" are
genuinely different mechanisms, and Qt is the only system that has both.** Qt's
`event->ignore()` re-dispatches the same event to `parentWidget()`, walking the ancestor chain.
GTK4 and Flutter do not have this at all — they deliver along the *pick path*, so ancestors are
already participants and nothing needs re-routing. Android has neither: a refusing child causes
the sibling search to continue, and the parent handles the event itself only when the whole
subtree refused.

**4. froth's note that the prior art is unanimous on "geometry picks the container, memory picks
the member" is WRONG as stated.** All four systems *track* a per-container focused child, and in
all four the tracking nests up every ancestor. But only **two of four actually restore from it**:

| | tracks focused child | nests | restores on focus return |
|---|---|---|---|
| Qt | yes (`focus_child`) | yes | **no** — only at top-level window activation |
| GTK4 | yes (`focus-child`) | yes | **yes** — the anchor for Tab and directional nav |
| Android | yes (`mFocused`) | yes | **no** — `requestFocus` re-scans by child index |
| Flutter | yes (`focusedChild`) | yes | **yes** — `FocusScopeNode` recurses into it |

The split is not accidental: GTK4 and Flutter both restore, and both are the systems where
traversal is a tree walk. Qt's traversal is a flat linked list and Android's restoration is an
opt-in side feature. This is a live disagreement in the prior art, not a settled convention.

---

## A. The focus-policy axis

### Qt 6 — one enum, factored into bits

From `qnamespace.h` (corroborated by
[doc.qt.io/qt-6/qt.html#FocusPolicy-enum](https://doc.qt.io/qt-6/qt.html#FocusPolicy-enum)):

```cpp
enum FocusPolicy {
    NoFocus     = 0,
    TabFocus    = 0x1,
    ClickFocus  = 0x2,
    StrongFocus = TabFocus | ClickFocus | 0x8,   // 0xB
    WheelFocus  = StrongFocus | 0x4              // 0xF
};
```

Five names, but **two primitive bits plus two composites**. `TabFocus` admits tab traversal only;
`ClickFocus` admits mouse-click focus only; `StrongFocus` is both (plus a reserved 0x8);
`WheelFocus` adds focus-on-wheel. A widget declaring `TabFocus` is walkable and *not*
click-focusable — the exact separation froth is asking about, expressible in Qt today.

Defaults, from source: `QWidget` base is `NoFocus` (`data.focus_policy = 0`, `qwidget.cpp:907`);
`QPushButton` takes `StrongFocus` from the style hint `SH_Button_FocusPolicy`
(`qcommonstyle.cpp:5184`); `QLabel` is `NoFocus` unless `setTextInteractionFlags` upgrades it —
`LinksAccessibleByKeyboard` → `StrongFocus`, text-selectable → `ClickFocus`, else `NoFocus`
(`qlabel.cpp:678-688`); `QLineEdit` sets `StrongFocus` explicitly (`qlineedit_p.cpp:220`).

That `QLabel` ladder is worth noting on its own: Qt uses `ClickFocus` for the case where a
widget should be reachable by pointing at it but should not appear in the Tab order. The
selectable label is exactly a "clickable but not walkable" surface.

### GTK 4 — four independent properties, re-split from GTK3

From `gtk_widget_class_init` in `gtk/gtkwidget.c` and
[migrating-3to4](https://docs.gtk.org/gtk4/migrating-3to4.html):

| property | default | meaning (GTK4) |
|---|---|---|
| `focusable` | `FALSE` | "Whether this widget itself will accept the input focus." |
| `can-focus` | `TRUE` | "Whether the widget **or any of its descendents** can accept the input focus." |
| `can-target` | `TRUE` | "Whether the widget can receive pointer events." |
| `focus-on-click` | `TRUE` | "Whether the widget should grab focus when it is clicked with the mouse." |

**The GTK3→GTK4 re-split is the interesting part.** In GTK3, `can-focus` meant "this widget
itself accepts focus" and defaulted `FALSE`. GTK4 kept the *name* and changed both the *meaning*
and the *default*: `can-focus` is now a **subtree gate** defaulting `TRUE`, and the per-widget
declaration moved to the new `focusable`. Source doc comments confirm:
`gtk_widget_set_can_focus` — "Sets whether the input focus can enter the widget or any of its
children"; `gtk_widget_set_focusable` — "Sets whether the widget can own the input focus."

This means GTK4 spends one of its axes on **scoping** (block a whole subtree) rather than on
splitting walkable from clickable. GTK4 has **no walkable/clickable split at all**: a `focusable`
widget is both tab-reachable and click-focusable, and `focus-on-click` only suppresses the
*side effect* of a click. Its own doc: "This property is only relevant for widgets that can take
focus" — it is a behavior modifier on an already-focusable widget, not a permission.

`focus-on-click` lives on `GtkWidget` in GTK4, not `GtkButton`
([class.Button](https://docs.gtk.org/gtk4/class.Button.html) lists it under "Properties inherited
from GtkWidget").

### Android — the split is by input *modality*, not input *mechanism*

Constants from `View.java`:

```java
public static final int NOT_FOCUSABLE  = 0x00000000;
public static final int FOCUSABLE      = 0x00000001;
public static final int FOCUSABLE_AUTO = 0x00000010;  // "This is the default."
```

`FOCUSABLE_AUTO` (API 26 / O) resolves at flag-set time, and its **only input is `CLICKABLE`**
(`View.setFlags`, ~18474):

```java
if (((mViewFlags & FOCUSABLE_AUTO) != 0)
        && (changed & (FOCUSABLE_MASK | CLICKABLE)) != 0) {
    // Heuristic only takes into account whether view is clickable.
    final int newFocus = ((mViewFlags & CLICKABLE) != 0) ? FOCUSABLE : NOT_FOCUSABLE;
```

Android therefore runs froth's axis relationship **backwards**: rather than declaring
focusability and deriving clickability, it declares clickability and derives focusability.
`setOnClickListener` forces `setClickable(true)` (`View.java:7868`), which re-runs the heuristic.

**`focusableInTouchMode` is a genuinely different reason for a second axis.** Touch mode is
**per-window state tracked by the framework**, not a per-view property:

```java
public boolean isInTouchMode() {
    if (mAttachInfo != null) return mAttachInfo.mInTouchMode;
    return mResources.getBoolean(com.android.internal.R.bool.config_defaultInTouchMode);
}
```

`ViewRootImpl.processPointerEvent` calls `ensureTouchMode(true)` on `ACTION_DOWN`/`ACTION_SCROLL`
from any pointer-class device; `processMotionEvent` calls `ensureTouchMode(false)` when a
D-pad/trackball-class device generates one. `ensureTouchMode` round-trips through
`WindowManagerGlobal.setInTouchMode(...)`, so it is display-global, not per-Activity.

So the second bit does not answer "can a click focus this?" — it answers **"may this be focused
while the device is currently in touch mode?"** A view that is `focusable` but not
`focusableInTouchMode` is fully keyboard/D-pad reachable and completely unfocusable under
touch. This is a modality axis; froth has no equivalent state (a terminal has no touch mode) and
should not read it as an analogue of clickability.

`descendantFocusability` on `ViewGroup` (`ViewGroup.java:372-390`), default
`FOCUS_BEFORE_DESCENDANTS` (constructor, `ViewGroup.java:733`):

- `FOCUS_BEFORE_DESCENDANTS` (`0x20000`) — "This view will get focus before any of its descendants."
- `FOCUS_AFTER_DESCENDANTS` (`0x40000`) — "This view will get focus only if none of its descendants want it."
- `FOCUS_BLOCK_DESCENDANTS` (`0x60000`) — "This view will block any of its descendants from getting focus, even if they are focusable."

### Flutter — the cleanest walkable/nameable split in the survey

From `FocusNode`
([api.flutter.dev](https://api.flutter.dev/flutter/widgets/FocusNode-class.html)):

- `canRequestFocus` (default `true`) — "this focus node may request the primary focus… Set to
  false if you want this node to do nothing when `requestFocus` is called on it."
- `skipTraversal` (default `false`) — "tells the focus traversal policy to skip over this node
  for purposes of the traversal algorithm."

The traversal filter in `focus_traversal.dart` is literally the conjunction:

```dart
return node.canRequestFocus && !node.skipTraversal;
```

**`canRequestFocus: true, skipTraversal: true` is the meaningful and coherent combination:
focusable by name and by pointer, invisible to Tab.** That is froth's axis 2 without axis 1. The
inverse (`canRequestFocus: false, skipTraversal: false`) is coherent but degenerate — traversal
excludes it anyway via the conjunction, so `skipTraversal` carries no information there.

Container-level mirrors, both on `FocusNode`: `descendantsAreFocusable` ("If false, will disable
focus for all of this node's descendants… Does not affect focusability of this node: for that,
use `canRequestFocus`") and `descendantsAreTraversable`. Note the deliberate symmetry — Flutter
factors *both* axes at *both* levels, four flags where GTK4 has two-and-a-scope.

`ExcludeFocus` and `ExcludeFocusTraversal` are scoped tree overrides layered on top: their docs
state explicitly that they do **not** mutate the descendants' own `canRequestFocus` /
`skipTraversal` values. So Flutter distinguishes "this node declares itself unwalkable" from
"an ancestor is currently suppressing traversal here" — a widget can be restored to walkable by
removing the ancestor, without its own declaration having been touched.

`FocusTraversalGroup` carries a `FocusTraversalPolicy` — `WidgetOrderTraversalPolicy` (creation
order), `ReadingOrderTraversalPolicy` (reading-order bands respecting `Directionality`),
`OrderedTraversalPolicy` (explicit order from a `FocusTraversalOrder` ancestor), and
`DirectionalFocusTraversalPolicyMixin`. Grouping does not itself gate focusability except through
its own `descendantsAreFocusable`/`descendantsAreTraversable` parameters.

---

## B. Does programmatic focus respect the policy?

**This is where the prior art openly disagrees.**

### Qt — no. `setFocus()` bypasses `focusPolicy` entirely.

`QWidget::setFocus(Qt::FocusReason)`, `qwidget.cpp:6547`:

```cpp
void QWidget::setFocus(Qt::FocusReason reason)
{
    if (!isEnabled())
        return;
    QWidget *f = d_func()->deepestFocusProxy();
    if (!f) f = this;
    ...
}
```

The only guard is `isEnabled()`. `focusPolicy` is never read. Qt documents this deliberately, in
the `focusInEvent` doc comment (`qwidget.cpp:9700`):

> A widget normally must setFocusPolicy() to something other than Qt::NoFocus to receive focus
> events. (Note that the application programmer can call setFocus() on any widget, even those
> that do not normally accept focus.)

So in Qt, **froth's axis 2 is not a declaration at all — it is universal and unconditional**,
gated only by enabled-ness. `focusPolicy` governs the two *input-driven* paths (tab, click,
wheel) and nothing else. That is a direct match for froth's current state, where focusability by
name is implicit and universal.

`setFocusProxy` is a **static redirect, not memory**: `setFocus()` calls `deepestFocusProxy()`
and follows the (possibly chained) pointer on every call. It always targets whatever
`focusProxy()` currently returns, never a last-focused child.

### GTK4 — yes, refuses; delegation is opt-in per widget class.

`gtk_widget_grab_focus`, `gtk/gtkwidget.c` ~5218:

```c
gboolean gtk_widget_grab_focus (GtkWidget *widget)
{
  if (!gtk_widget_is_sensitive (widget) ||
      !get_effective_can_focus (widget) ||
      widget->priv->root == NULL)
    return FALSE;
  return GTK_WIDGET_GET_CLASS (widget)->grab_focus (widget);
}
```

`get_effective_can_focus` walks the parent chain: a single ancestor with `can-focus: FALSE`
blocks focus regardless of the target's own `focusable`. The base vfunc implementation
`gtk_widget_grab_focus_self` then fails outright for a non-`focusable` widget:

```c
gtk_widget_grab_focus_self (GtkWidget *widget) {
  if (!priv->focusable) return FALSE;
  gtk_root_set_focus (priv->root, widget);
  return TRUE;
}
```

**Delegation to a descendant is not automatic — it is a vfunc override.** GTK ships a second
canned implementation, `gtk_widget_grab_focus_child`, which walks children front-to-back calling
`gtk_widget_grab_focus` recursively; it is installed by `GtkColumnView`, `GtkFileChooserWidget`,
`GtkPopoverBin`, `GtkColorChooserWidget`, `GtkFontDialogButton`, `GtkPasswordEntry`,
`GtkColorDialogButton`. Others explicitly install `gtk_widget_grab_focus_self` to *refuse*
delegation (`GtkExpander`, `GtkListBox`, `GtkScale`). `GtkEntry` writes a custom one that
forwards to its internal `GtkText` (`gtkentry.c:427`). The public doc states the contract:

> If @widget is not focusable, or its `grab_focus` implementation cannot transfer the focus to a
> descendant of @widget that is focusable, it will not take focus and false will be returned.

So GTK4's answer is a **three-valued** one: refuse, or succeed, or delegate — and which of the
three you get is a per-class decision, not a per-instance flag.

### Android — yes, refuses; unconditionally.

`View.requestFocusNoSearch` (`View.java:15372`) is the real guard behind `requestFocus()`:

```java
private boolean requestFocusNoSearch(int direction, Rect previouslyFocusedRect) {
    // need to be focusable
    if (!canTakeFocus()) return false;

    // need to be focusable in touch mode if in touch mode
    if (isInTouchMode() &&
        (FOCUSABLE_IN_TOUCH_MODE != (mViewFlags & FOCUSABLE_IN_TOUCH_MODE))) return false;

    // need to not have any parents blocking us
    if (hasAncestorThatBlocksDescendantFocus()) return false;
    ...
}
```

`canTakeFocus()` (`View.java:18443`) requires visible, `FOCUSABLE`, `ENABLED`, and nonzero size
(unless `sCanFocusZeroSized`). There is **no path that forces focus onto a non-focusable view.**

`ViewGroup.requestFocus` delegates according to `descendantFocusability`
(`ViewGroup.java:3337`):

```java
case FOCUS_BLOCK_DESCENDANTS:
    result = super.requestFocus(direction, previouslyFocusedRect); break;
case FOCUS_BEFORE_DESCENDANTS: {
    final boolean took = super.requestFocus(direction, previouslyFocusedRect);
    result = took ? took : onRequestFocusInDescendants(direction, previouslyFocusedRect); break;
}
case FOCUS_AFTER_DESCENDANTS: {
    final boolean took = onRequestFocusInDescendants(direction, previouslyFocusedRect);
    result = took ? took : super.requestFocus(direction, previouslyFocusedRect); break;
}
```

So delegation direction is a per-container declaration — but see §D: the descendant search does
**not** consult remembered focus.

### Flutter — yes, refuses silently; no redirection.

`FocusNode._doRequestFocus` in `focus_manager.dart`:

```dart
void _doRequestFocus({required bool findFirstFocus}) {
  if (!canRequestFocus) {
    return;
  }
  ...
}
```

Refused, not redirected — there is no "nearest focusable ancestor takes it instead" fallback.
`enclosingScope` walks up to the nearest `FocusScopeNode`, but it is used to decide *where to
record* `focusedChild`, not to reroute a refused request.

**`skipTraversal` has zero effect on `requestFocus()`.** `_doRequestFocus` never reads it;
traversal filtering lives entirely in `focus_traversal.dart`. Confirmed by inspection of both
call paths — axes 1 and 2 are checked in separate code, which is what makes them genuinely
independent rather than nominally so.

**Verdict on axis 2.** Only Qt treats focusable-by-name as unconditional. Flutter treats it as
the *primary* declaration with walkability as a modifier on top. GTK4 and Android gate it behind
the same declaration that governs tab. So froth's axis 2 is genuinely independent in exactly one
system (Qt, where it is independent by being universal) and is a distinct *declaration* in
exactly one other (Flutter, where `canRequestFocus` is precisely that declaration).

---

## C. Hit-test transparency

### The distinction froth asked for, named

There are two mechanisms that look similar and are not:

- **Skipped** — the widget is excluded from the hit test, so the search continues past it to
  whatever is beneath it in z-order. The widget never sees the event and never had a claim.
- **Forwarded to parent** — the widget *was* hit, took the event, declined it, and the toolkit
  re-dispatches the same event to its ancestor. Siblings beneath are never reconsidered.

**Qt is the only system in this survey with both, as separate named mechanisms.** GTK4 and
Flutter have only "skipped," because their delivery model already puts ancestors on the path.
Android has neither as a per-widget opt-out.

### Qt 6

**`Qt::WA_TransparentForMouseEvents`** (value 51,
[doc.qt.io/qt-6/qt.html#WidgetAttribute-enum](https://doc.qt.io/qt-6/qt.html#WidgetAttribute-enum)):

> When enabled, this attribute disables the delivery of mouse events to the widget and its
> children. Mouse events are delivered to other widgets **as if the widget and its children were
> not present in the widget hierarchy**; mouse clicks and other events effectively "pass through"
> them.

The implementation confirms this is a hit-test skip, not a redirect.
`QWidgetPrivate::childAtRecursiveHelper` (`qwidget.cpp:10483`) excludes such children from the
walk outright:

```cpp
if (!child || child->isWindow() || child->isHidden() ||
    child->testAttribute(Qt::WA_TransparentForMouseEvents) ...)
    continue;
```

→ **skipped; the search continues beneath.** `QWidget::childAt()` is Qt's hit test and respects
the attribute.

**The ignore/propagate path goes to the parent.** `QApplication::notify()`, loop at
`qapplication.cpp:2727-2751`:

```cpp
QPointer<QWidget> pw = w;
while (w) {
    ... res = d->notify_helper(w, ...);
    eventAccepted = (...)->isAccepted();
    if (res && eventAccepted) break;
    if (w->isWindow() || w->testAttribute(Qt::WA_NoMousePropagation)) break;
    relpos += w->pos();
    w = w->parentWidget();
}
```

An ignored mouse event is **re-dispatched, translated, to `parentWidget()`**, walking the
ancestor chain. This is the "forwarded to parent" mechanism, and it is structurally distinct
from the z-order skip above.

**`Qt::WA_NoMousePropagation`** (value 73): "Prohibits mouse events from being propagated to the
widget's parent. This attribute is disabled by default." It is the `break` in that loop — a stop
flag on the parent walk, not a hit-test property.

**`setEnabled(false)`**: "In general an enabled widget handles keyboard and mouse events; a
disabled widget does not"
([qwidget.html#enabled-prop](https://doc.qt.io/qt-6/qwidget.html)). Disabling cascades to
children implicitly. On focus, `setFocus()` returns immediately for a disabled widget
(`qwidget.cpp:6548`) — a disabled widget cannot be focused by any route including programmatic.

### GTK 4

`GtkPickFlags` from `gtk/gtkenums.h`:

```c
GTK_PICK_DEFAULT        = 0        // widgets receiving events
GTK_PICK_INSENSITIVE    = 1 << 0   // ALSO include insensitive widgets
GTK_PICK_NON_TARGETABLE = 1 << 1   // ALSO include can-target:FALSE widgets
```

The flags **widen** the search; the default excludes both insensitive and non-targetable widgets.

The picking walk (`gtk_widget_pick` → `gtk_widget_can_be_picked` → `gtk_widget_do_pick`):

1. `gtk_widget_can_be_picked` requires mapped, `can-target` (unless `GTK_PICK_NON_TARGETABLE`),
   and sensitive (unless `GTK_PICK_INSENSITIVE`). Failing it excludes the **entire subtree** —
   pick does not recurse in at all.
2. `gtk_widget_do_pick` iterates children **last-to-first** (`_gtk_widget_get_last_child` →
   `_gtk_widget_get_prev_sibling`), i.e. topmost-drawn first, z-descending. `GTK_IS_NATIVE`
   children are skipped (separate surfaces); inverse transforms are applied for transformed
   children.
3. Recurses into the first child yielding a non-NULL pick, depth-first, returning the **deepest**
   hit.
4. If no child matches, falls back to the widget's own `contains` vfunc.

Doc comment: "Usually widgets will return NULL if the given coordinate is not contained in
@widget checked via `gtk_widget_contains`. Otherwise they will recursively try to find a child
that does not return NULL. Widgets are however free to customize their picking algorithm."

**`contains` is the only overridable geometry hook — `pick` itself is not a vfunc.** The
`GtkWidgetClass` vtable carries `contains`, `grab_focus`, `focus`, and `set_focus_child`, but not
`pick`. The default `gtk_widget_real_contains` is a CSS border-box **rounded-rect** point test
(`gsk_rounded_rect_contains_point` against `gtk_css_boxes_get_border_box`), not a bounds check —
a rounded corner is genuinely not hit.

**`can-target: FALSE` → skipped, continues beneath.** The pick loop `continue`s to the next
(lower z-order) sibling at the same level. It does not go to the parent.

`gtk_widget_set_sensitive(FALSE)` excludes from picking the same way, and blocks focus:
`gtk_widget_grab_focus` checks `gtk_widget_is_sensitive` up front; `gtk_widget_child_focus`
likewise.

**Delivery is along the path, not to one winner.**
[PropagationPhase](https://docs.gtk.org/gtk4/enum.PropagationPhase.html): `GTK_PHASE_NONE`,
`GTK_PHASE_CAPTURE` (root→target, before bubble), `GTK_PHASE_TARGET` (the target's own default
handlers), `GTK_PHASE_BUBBLE` (target→root). Every event controller on every widget in the
root-to-target chain gets a shot, in a defined order. There is no "forward to parent" because the
parent was already a participant.

### Android — no hit-test opt-out exists

`ViewGroup.dispatchTouchEvent`'s down-event search loop (~`ViewGroup.java:2733`):

```java
final ArrayList<View> preorderedList = buildTouchDispatchChildList();
...
for (int i = childrenCount - 1; i >= 0; i--) {
    final int childIndex = getAndVerifyPreorderedIndex(childrenCount, i, customOrder);
    final View child = getAndVerifyPreorderedView(preorderedList, children, childIndex);
    ...
    if (!child.canReceivePointerEvents() || !isTransformedTouchPointInView(x, y, child, null)) {
        continue;
    }
    ...
    if (dispatchTransformedTouchEvent(ev, false, child, idBitsToAssign)) {
        newTouchTarget = addTouchTarget(child, idBitsToAssign);
        break;
    }
    // else: loop continues to the NEXT candidate child
}
```

Iteration runs `childrenCount-1` down to `0` — the source comment says "Scan children from front
to back," i.e. reverse draw order, topmost first. `buildTouchDispatchChildList()` /
`getAndVerifyPreorderedView` exist so a custom `getChildDrawingOrder` or Z-translation reorder
still yields topmost-first.

**Definitive answer to the sibling-vs-parent question — Android does both, at different points.**
On the initial `ACTION_DOWN`, a candidate child returning `false` from `dispatchTouchEvent` does
**not** end the search and does **not** hand the event to the parent: the loop `continue`s to the
next candidate underneath in z-order, within the same dispatch. Only when **no child at all**
accepts does the ViewGroup fall back to acting as a leaf:

```java
if (mFirstTouchTarget == null) {
    // No touch targets so treat this as an ordinary view.
    handled = dispatchTransformedTouchEvent(ev, canceled, null, TouchTarget.ALL_POINTER_IDS);
}
```

with `child == null` routing to `super.dispatchTouchEvent` — the group's own `onTouchEvent`
(`ViewGroup.java:3106-3170`). Once a target is locked into `mFirstTouchTarget` for a gesture,
subsequent MOVE/UP events go straight to it; there is no per-move re-search and no bubbling if
that target later returns `false` mid-gesture.

**`setEnabled(false)` does not affect hit testing.** `canReceivePointerEvents()`
(`View.java:16638`) checks visibility only:

```java
protected boolean canReceivePointerEvents() {
    return (mViewFlags & VISIBILITY_MASK) == VISIBLE || getAnimation() != null;
}
```

A disabled-but-visible view is still hit-tested and still receives the dispatch. What changes is
inside `View.onTouchEvent` (`View.java:18059`), with an explicit comment:

```java
if ((viewFlags & ENABLED_MASK) == DISABLED
        && (mPrivateFlags4 & PFLAG4_ALLOW_CLICK_WHEN_DISABLED) == 0) {
    ...
    // A disabled view that is clickable still consumes the touch
    // events, it just doesn't respond to them.
    return clickable;
}
```

So a disabled clickable view **swallows** the touch — Android's disabled state behaves like
Flutter's `AbsorbPointer`, not like `IgnorePointer`. For focus, `canTakeFocus()` requires
`ENABLED`, so a disabled view is unfocusable.

**No `IgnorePointer` equivalent.** `INVISIBLE` (`0x4`) and `GONE` (`0x8`) both fail the
`VISIBILITY_MASK == VISIBLE` test and are equally excluded from hit testing — but both also stop
the view being drawn. There is no flag that keeps a view drawn while removing it from hit
testing. `setFilterTouchesWhenObscured` is a *security* feature (reject touches when another
window overlaps) and is not this. `TouchDelegate` forwards hits from an *expanded* region to
another view and is also not this. **Confirmed: no built-in equivalent exists.**

### Flutter — an entry *list*, not a winner

**`IgnorePointer`** ([api.flutter.dev](https://api.flutter.dev/flutter/widgets/IgnorePointer-class.html)):

> When ignoring is true, this widget (and its subtree) is invisible to hit testing. It still
> consumes space during layout and paints its child as usual. It just cannot be the target of
> located events, because it returns false from RenderBox.hitTest.

```dart
bool hitTest(BoxHitTestResult result, {required Offset position}) {
  return !ignoring && super.hitTest(result, position: position);
}
```

Returns `false` without calling `super.hitTest` — subtree skipped entirely, and because the
return is `false`, the ancestor's child loop keeps going to what is beneath. **Skipped;
continues beneath.**

**`AbsorbPointer`**:

> When absorbing is true, this widget prevents its subtree from receiving pointer events by
> terminating hit testing at itself.

```dart
bool hitTest(BoxHitTestResult result, {required Offset position}) {
  return absorbing ? size.contains(position) : super.hitTest(result, position: position);
}
```

When absorbing and in bounds it returns `true` **without calling `hitTestChildren` and without
calling `result.add`**. So: the subtree is excluded, the ancestor's child loop stops (a `true`
return short-circuits the standard reverse-paint-order iteration), and therefore nothing behind
it is hit either — but `RenderAbsorbPointer` **does not add itself to the path**, so it is not a
dispatch target. It is a pure void: swallows the hit, and nobody receives the event. This is a
sharper behavior than the usual informal description of it; note it did not come from a doc
sentence but from reading the two `hitTest` overrides against the default child-iteration
contract, so treat the "nothing behind it is hit" half as a source-derived inference rather than
a documented guarantee.

**`HitTestBehavior`**, from `RenderProxyBoxWithHitTestBehavior` (`proxy_box.dart:179-216`):

```dart
@override
bool hitTest(BoxHitTestResult result, {required Offset position}) {
  var hitTarget = false;
  if (size.contains(position)) {
    hitTarget = hitTestChildren(result, position: position) || hitTestSelf(position);
    if (hitTarget || behavior == HitTestBehavior.translucent) {
      result.add(BoxHitTestEntry(this, position));
    }
  }
  return hitTarget;
}

@override
bool hitTestSelf(Offset position) => behavior == HitTestBehavior.opaque;
```

- **`deferToChild`** (default) — `hitTestSelf` is false, so a hit only registers if a child was
  hit. No child → adds no entry, returns false, search continues beneath.
- **`opaque`** — `hitTestSelf` always true, so within bounds it always registers and always
  returns true, blocking anything behind.
- **`translucent`** — the entry is added unconditionally when in bounds, **but the return value
  is still the real `hitTarget`**, not forced true. That asymmetry is the whole mechanism: the
  translucent widget joins the path *and* the parent's iteration continues past it, so what is
  beneath joins the same path too. Both end up in one `result.path`.

**The path model.** `RenderBox.hitTest` (`rendering/box.dart` ~2921):

```dart
if (_size!.contains(position)) {
  if (hitTestChildren(result, position: position) || hitTestSelf(position)) {
    result.add(BoxHitTestEntry(this, position));
    return true;
  }
}
```

Children recurse and add themselves *before* the parent appends itself, so `result.path` comes
out ordered **leaf → root** as a natural consequence of the call stack unwinding.
`HitTestResult`'s doc: "The first entry in the path is the most specific, typically the one at
the leaf of tree being hit tested. Event propagation starts with the most specific (i.e., first)
entry and proceeds in order through the path."

Dispatch walks the whole thing — `GestureBinding.dispatchEvent` (`gestures/binding.dart:477`):

```dart
for (final HitTestEntry entry in hitTestResult.path) {
  entry.target.handleEvent(event.transformed(entry.transform), entry);
}
```

**Every** entry receives the event. For gesture-recognizer targets, `handleEvent` means "enter
this pointer into my recognizers," which is how several nested `GestureDetector`s all compete for
one pointer. The **gesture arena** then resolves the single winner —
[GestureArenaManager](https://api.flutter.dev/flutter/gestures/GestureArenaManager-class.html):
"The first member to accept or the last member to not reject wins."

So Flutter's model is three layers, not one: (1) the hit test builds an ordered multi-target
path; (2) every target on the path is notified and may enter the arena; (3) the arena, *not* the
hit test, picks which recognizer claims the gesture. "One winner" is a decision made **after**
and **separately from** hit testing — which is exactly why Flutter needs no "forward to parent"
mechanism.

---

## D. Containers that focus treats as one thing

### The claim under test

froth's note holds that the prior art is unanimous: *geometry picks the container, memory picks
the member.* **It is not unanimous.** All four track; two restore.

### Flutter — yes, automatic, nested, self-pruning

`FocusScopeNode.focusedChild` is backed by a **stack**, not a slot:

```dart
FocusNode? get focusedChild => _focusedChildren.lastOrNull;
```

`FocusScopeNode._doRequestFocus` prunes then recurses:

```dart
void _doRequestFocus({required bool findFirstFocus}) {
  while (_focusedChildren.isNotEmpty &&
      (!_focusedChildren.last.canRequestFocus ||
       _focusedChildren.last.enclosingScope == null)) {
    _focusedChildren.removeLast();
  }
  final FocusNode? focusedChild = this.focusedChild;
  if (!findFirstFocus || focusedChild == null) {
    if (canRequestFocus) { _setAsFocusedChildForScope(); _markNextFocus(this); }
    return;
  }
  focusedChild._doRequestFocus(findFirstFocus: true);
}
```

Note the ordering: **stale entries are dropped before the memory is read**, and the recursion
continues into nested scopes. A remembered child that was disposed, removed, or whose
`canRequestFocus` went false falls off the stack and the *next-most-recent* entry is used — the
scope focuses itself only if the stack empties. That is a graceful degradation ladder, not a
null check.

**The memory nests, in one pass, on every focus change** — `_setAsFocusedChildForScope`:

```dart
void _setAsFocusedChildForScope() {
  var scopeFocus = this;
  for (final FocusScopeNode ancestor in ancestors.whereType<FocusScopeNode>()) {
    ancestor._focusedChildren.remove(scopeFocus);
    ancestor._focusedChildren.add(scopeFocus);
    scopeFocus = ancestor;
  }
}
```

Every `FocusScopeNode` ancestor records its own child on the path. **Automatic, not opt-in** —
this runs unconditionally inside `_doRequestFocus` with no flag to disable it. Removal is also
handled at the source, in `_removeChild`, which strips the node *and all its descendants* from
the enclosing scope's stack.

**`FocusTraversalGroup` carries a second, separate memory.** `_FocusTraversalGroupNode` holds a
single-slot `FocusNode? lastRequestedFocus`, set when traversal focuses a node in the group and
consumed-and-nulled when the group would otherwise take focus itself. It is mechanically parallel
to but implementation-wise independent of `FocusScopeNode._focusedChildren`, serving the
traversal-re-entry path rather than scope-level restoration. **A group and a scope are different
things with different memories** — worth keeping straight.

**Directional traversal keeps its own per-scope history.**
`DirectionalFocusTraversalPolicyMixin` stores `Map<FocusScopeNode, _DirectionalPolicyData>
_policyData`, each holding a `List<_DirectionalPolicyDataEntry>` of (direction, node) pairs.
`_popPolicyDataIfNeeded` checks whether the incoming direction reverses the last recorded one and,
if so, **pops back to the previously-visited node instead of computing a fresh spatial target**.
The doc comment states the motive outright: "Since hysteresis in the navigation order is
undesirable, this implementation maintains a stack of previous locations that have been visited
on the policy data for the affected FocusScopeNode." If the remembered node has been unmounted
(`history.last.node.parent == null`), the scope data resets — with an explicit acknowledgement in
the source that "hysteresis might not be avoided when items that go off screen get unmounted."

This is a **third** memory, distinct from the other two: not "where was I inside this container"
but "where did I come from when moving this direction." froth's directional-return problem is
arguably closer to this one than to `focusedChild`.

### GTK4 — yes, automatic, nested, and actually consulted by traversal

The GTK3 `gtk_container_get/set_focus_child` moved to `gtk_widget_get_focus_child` /
`gtk_widget_set_focus_child` on `GtkWidget` (`gtkwidget.c` ~12661). `set_focus_child` is a vfunc;
the default just does `g_set_object(&priv->focus_child, child)`. The public doc warns it is "only
suitable for widget implementations."

**Maintained centrally, not by each container.** `GtkBox` contains no calls to `set_focus_child`
at all. The bookkeeping lives in `synthesize_focus_change_events` in `gtkwindow.c` (~5209-5330),
called every time `gtk_window_root_set_focus` changes the window's focus widget: on focus-out it
walks from the old focus up to the common ancestor clearing each `focus_child`; on focus-in it
builds a stack from the new focus to the root and, walking down, sets each ancestor's
`focus_child` to its child on the chain. **Nests fully, automatically, for every container.**

**And GTK actually uses it for navigation.** `gtk_widget_focus_move` (`gtk/gtkwidgetfocus.c`
~440-511) reads `_gtk_widget_get_focus_child(widget)` as the anchor — both to resume Tab after
the current child, and (via `focus_sort_left_right` / `focus_sort_up_down` / `find_old_focus`) as
the **spatial reference point** for `GTK_DIR_LEFT/RIGHT/UP/DOWN`. `find_old_focus` validates the
chain as it walks up, checking `focus_child` matches at every level, so a stale or mismatched
chain is not used as an anchor.

This is the closest thing in the survey to froth's stated model: the container's remembered child
is what directional navigation resolves against.

Cleanup on removal: in the unparent path (`gtkwidget.c` ~2710),
`if (gtk_widget_get_focus_child(priv->parent) == widget) gtk_widget_set_focus_child(priv->parent,
NULL)`. **UNVERIFIED:** the exact destroy-time (as opposed to unparent-time) path for clearing a
dangling reference was not traced end to end; normal GTK4 lifecycle unparents before destroying,
so the verified path is very likely sufficient, but this was not confirmed.

`GtkWindow` remembers `focus-widget` across hide/show. `gtk_window_hide` only unmaps and does not
clear `priv->focus_widget`; `gtk_window_show` contains:

```c
if (!priv->focus_widget)
  gtk_window_move_focus (widget, GTK_DIR_TAB_FORWARD);
```

— a fresh first-focusable is picked **only if** nothing was remembered.

### Android — tracks it, nests it, and then does not use it for restoration

`ViewGroup.mFocused` is maintained and propagated. `requestChildFocus(child, focused)`
(`ViewGroup.java:858-878`) sets `mFocused = child` then calls `mParent.requestChildFocus(this,
focused)` — **nested up every ancestor**, same shape as GTK4 and Flutter.

**But `onRequestFocusInDescendants` never reads it** (`ViewGroup.java:3382-3403`):

```java
protected boolean onRequestFocusInDescendants(int direction, Rect previouslyFocusedRect) {
    ...
    for (int i = index; i != end; i += increment) {
        View child = children[i];
        if ((child.mViewFlags & VISIBILITY_MASK) == VISIBLE) {
            if (child.requestFocus(direction, previouslyFocusedRect)) return true;
        }
    }
    return false;
}
```

No reference to `mFocused` anywhere in the method. **`requestFocus()` on a group re-derives a
target by child index order every time.** `mFocused`'s actual job is routing and querying:
`getFocusedChild()` returns it, `findFocus()` recurses through it, and `dispatchKeyEvent` /
`dispatchKeyEventPreIme` / `dispatchTrackballEvent` / `dispatchGenericMotionEvent` all route
directly to it rather than searching.

Restoration in Android is a **separate, opt-in feature** using different fields:
`restoreDefaultFocus()` / `mDefaultFocus`, and `restoreFocusInCluster()` / `mFocusedInCluster`
for keyboard navigation clusters. Not `mFocused`.

Removing the focused child (`removeViewInternal`, `ViewGroup.java:5595-5636`) unfocuses it,
clears `mFocused` up the chain via `clearChildFocus` (which itself propagates to
`mParent.clearChildFocus(this)`), then calls `rootViewRequestFocus()` — **a fresh search from the
root, not a restore** — and only fires `notifyGlobalFocusCleared` if that fails. A view that
loses its `FOCUSABLE` bit while focused triggers `clearFocus()` directly from `View.setFlags`.

**Geometry and delegation are two sequential, non-overlapping steps.** `View.focusSearch` walks
up (`return mParent.focusSearch(this, direction)`); `ViewGroup.focusSearch` keeps walking until a
root namespace, then invokes the geometric search (`ViewGroup.java:1082`):

```java
public View focusSearch(View focused, int direction) {
    if (isRootNamespace()) {
        return FocusFinder.getInstance().findNextFocus(this, focused, direction);
    } else if (mParent != null) {
        return mParent.focusSearch(focused, direction);
    }
    return null;
}
```

`FocusFinder.findNextFocusInAbsoluteDirection` does the beam-based geometric scoring
(`beamBeats`, `beamsOverlap`, `FocusFinder.java:486-600+`). The two meet in `ViewRootImpl`
(~7928-7954):

```java
View focused = mView.findFocus();
if (focused != null) {
    View v = focused.focusSearch(direction);
    if (v != null && v != focused) {
        ...
        if (v.requestFocus(direction, mTempRect)) {
```

FocusFinder's geometric result is handed straight into `requestFocus`, which re-enters the
`descendantFocusability` machinery. **FocusFinder never calls `requestFocus`, and `requestFocus`
never uses beam geometry.** They are cleanly separated: a stateless geometric query, then a
stateful transfer subject to all the guards.

So Android confirms froth's *"geometry picks the container, then delegation picks the member"*
pipeline shape exactly — but the member is picked by **child index order**, not by memory.

### Qt — negative result, stated plainly

Qt's tab order is a **flat doubly-linked list**, not a tree walk. Each `QWidget` carries
`focus_next` / `focus_prev` pointers (`QWidgetPrivate::focus_next`, `qwidget.cpp:108`), walked by
`nextPrevElementInFocusChain()` (`qwidget.cpp:13288`) and exposed by `nextInFocusChain()`
(`qwidget.cpp:6890`). There is no container in the traversal to remember anything.

Qt **does** have the tracking field. `QWidgetPrivate::focus_child` is updated by
`updateFocusChild()` (`qwidget.cpp:6688-6708`) on every `setFocus()`/`clearFocus()`, walking
every ancestor up to and including the top level. `QWidget::focusWidget()` (`qwidget.cpp:6879`)
reads it, documented as: *"Returns the last child of this widget that setFocus had been called
on. For top level widgets this is the widget that will get focus in case this window gets
activated."*

**Qt consumes it in exactly one place: top-level window activation** (`qapplication.cpp:1877`):

```cpp
QWidget *w = QApplicationPrivate::active_window->focusWidget();
if (w && w->isVisible()) w->setFocus(Qt::ActiveWindowFocusReason);
```

**No sub-container in qtbase restores from it.** Checked, all negative:

- `QStackedWidget` — **zero** focus-related code in `qstackedwidget.cpp`. Switching pages does
  not remember or restore per-page focus.
- `QTabWidget` — sets `TabFocus` and `setFocusProxy(tabs)` on the tab bar
  (`qtabwidget.cpp:209-210`); the only other focus code (`qtabwidget.cpp:1127`) is a Ctrl+Tab
  fallback that focuses the tab bar *only when nothing else has focus*. Not a per-tab restore.
- `QMdiArea` — `NoFocus` on itself (`qmdiarea.cpp:1687`); `addSubWindow` (`qmdiarea.cpp:1955`)
  saves and re-applies `focusWidget()` only *around a reparent*, because `setParent` clears it.
  A one-time repair, not a feature.
- `QDockWidget` — `NoFocus` on itself (`qdockwidget.cpp:118`); no child-focus-restore code.

And hide/show is **actively hostile** to restoration. `QWidgetPrivate::hide_helper`
(`qwidget.cpp:8226-8237`) moves focus *away* when hiding a widget that has it:

```cpp
QWidget *fw = QApplication::focusWidget();
while (fw && !fw->isWindow()) {
    if (fw == q) { q->focusNextPrevChild(true); break; }
    fw = fw->parentWidget();
}
```

`show_helper` has no corresponding restore. **Qt's only built-in remember-and-restore is
window-level.** For a stacked page, tab page, dock widget, or MDI subwindow, the application
must implement it.

### Cross-cutting answers

**Does the memory nest?** Yes in all four — Qt `focus_child`, GTK4 `focus-child`, Android
`mFocused`, Flutter `_focusedChildren`. Every one sets the pointer on *every* ancestor along the
path to the focused leaf, not just the immediate parent. That much genuinely is unanimous.

**Opt-in or automatic?** The *tracking* is automatic and unconditional in all four. The
*restoration* is automatic in GTK4 and Flutter, absent for sub-containers in Qt, and available in
Android only through separate opt-in fields (`mDefaultFocus`, `mFocusedInCluster`).

**Destroyed or unfocusable remembered child?** Three approaches:
- **Flutter** prunes lazily *at read time*, walking back down a stack until a valid entry is
  found — the only system with a fallback ladder rather than a single fallback.
- **GTK4** clears eagerly at unparent time, and additionally validates the whole chain in
  `find_old_focus` before trusting it as an anchor.
- **Android** clears eagerly, then does a **fresh search from the root** rather than falling back
  to a previous member.
- **Qt** clears `focus_child` implicitly and moves focus to the next widget in the flat chain.

---

## E. Summary table

| | walkable declaration | click-focusable declaration | programmatic focus | hit-test opt-out | remembers focused child |
|---|---|---|---|---|---|
| **Qt 6** | `Qt::TabFocus` bit (`0x1`) in `focusPolicy` | `Qt::ClickFocus` bit (`0x2`) — a genuinely separate bit | **bypasses `focusPolicy` entirely**; only `isEnabled()` gates it | `WA_TransparentForMouseEvents` (skipped → beneath). Separately, `event->ignore()` → re-dispatch to **parent**; `WA_NoMousePropagation` stops that walk | tracks `focus_child`, nested — **restores only at top-level window activation**; no sub-container restore |
| **GTK 4** | `focusable` (default `FALSE`), gated by `can-focus` on every ancestor (default `TRUE`, subtree gate) | none — same `focusable`. `focus-on-click` (default `TRUE`) suppresses the *side effect*, not the permission | `gtk_widget_grab_focus` **refuses** if insensitive or not effectively-can-focus; base vfunc refuses if not `focusable`; **delegation is a per-class vfunc override**, not automatic | `can-target: FALSE` (default `TRUE`) — subtree skipped → beneath. `GtkPickFlags` widen the search. Insensitive also skipped | tracks `focus-child` on `GtkWidget`, nested, automatic, maintained centrally — **and used as the anchor for Tab and directional nav**. `GtkWindow` keeps `focus-widget` across hide/show |
| **Android (API 37)** | `focusable` (default `FOCUSABLE_AUTO`, resolved from `CLICKABLE`), plus `focusableInTouchMode` — a **modality** axis, not a mechanism axis | none. Relationship runs backwards: `CLICKABLE` **derives** focusability under `FOCUSABLE_AUTO` | `requestFocus` **refuses** — `canTakeFocus()` (visible/focusable/enabled/sized), touch-mode check, `hasAncestorThatBlocksDescendantFocus()`. Group delegation per `descendantFocusability` | **none.** No `IgnorePointer` equivalent; `INVISIBLE`/`GONE` also stop drawing. `setEnabled(false)` still hit-tests and still **consumes** the touch | tracks `mFocused`, nested, automatic — **but `onRequestFocusInDescendants` re-scans by child index and never reads it.** Restoration is separate opt-in (`mDefaultFocus`, `mFocusedInCluster`) |
| **Flutter 3.47** | `skipTraversal` (default `false`); container form `descendantsAreTraversable`; scoped override `ExcludeFocusTraversal` | none as a permission; `canRequestFocus` governs both pointer and programmatic focus | `requestFocus` **refuses silently** when `canRequestFocus` is false — no redirect. `skipTraversal` has **zero** effect here. `FocusScopeNode.requestFocus` delegates into `focusedChild` | `IgnorePointer` (skipped → beneath) vs `AbsorbPointer` (swallowed, nothing behind hit, and it does not enter the path itself). `HitTestBehavior.deferToChild` / `opaque` / `translucent` | `FocusScopeNode._focusedChildren` — a **stack**, nested, automatic, pruned at read time. `FocusTraversalGroup.lastRequestedFocus` is a second, separate slot. `_DirectionalPolicyData` is a third, per-scope directional history |

---

## Verdict: what transfers

**Transfers cleanly.**

- **Factoring walkability out of focusability is right, and two systems do it independently.** Qt
  splits Tab from Click as literal bits; Flutter splits `skipTraversal` from `canRequestFocus`
  and checks them in different code paths. Neither treats the second axis as a rename of the
  first. froth's hypothesis that these are two axes is not speculative — it has been reached
  twice, from different directions.
- **The "skipped vs. forwarded to parent" distinction is real and worth naming.** froth is right
  that these are different behaviors. Flutter names them (`IgnorePointer` / `AbsorbPointer`), Qt
  has both as separate mechanisms, GTK4 has only the skip, Android has neither. A survey that
  collapsed them would have been wrong.
- **z-descending, depth-first, deepest-hit-wins is the universal picking walk.** GTK4's
  `gtk_widget_do_pick` (last child to first), Android's `dispatchTouchEvent` loop
  (`childrenCount-1` down to `0`), Flutter's `hitTestChildren` in reverse paint order, Qt's
  `childAtRecursiveHelper`. froth's z-descending walk is in unanimous company.
- **Container focus memory nests, in all four systems, automatically.** If froth builds it, the
  pointer belongs on every ancestor, set in one pass at focus-change time. All four agree, and
  the three that write it write it the same way.
- **Geometry and delegation as two separate steps.** Android's pipeline —
  `FocusFinder.findNextFocus` returns a candidate, then `requestFocus` on that candidate runs the
  policy machinery — is a clean, stateless-then-stateful separation that would survive
  translation to a terminal unchanged.

**Transfers with a correction to froth's notes.**

- **The prior art is not unanimous on restoring from memory.** Two of four do it (GTK4, Flutter);
  Qt does not below the window level and Android does not through the primary path. If froth
  wants memory-based restoration it is choosing a side in a live disagreement, not following a
  convention. The correlation worth noticing: the two that restore are the two whose traversal is
  a tree walk with real container nodes. Qt's flat focus chain has no container to hang memory
  on, and Android's index re-scan is cheap enough that memory buys little. froth's traversal
  shape should probably decide this, not the head count.
- **Flutter suggests froth may want more than one kind of memory.** Flutter runs three, for three
  different questions: `FocusScopeNode.focusedChild` (where was I inside this scope),
  `_FocusTraversalGroupNode.lastRequestedFocus` (traversal re-entry), and `_DirectionalPolicyData`
  (where did I come from moving this direction, popped on reversal to avoid hysteresis). froth's
  stated problem — *"leaving directionally and coming back should return you to the row you
  left"* — is closer to the **third** than the first. A single "container remembers its member"
  slot may not answer it.
- **Degradation when the remembered member goes away is a design decision, not a null check.**
  Flutter walks back down a stack; GTK4 validates the whole ancestor chain before trusting the
  anchor; Android abandons memory and searches from the root. Three systems, three different
  answers, all deliberate.

**Shaped by things froth does not have — discount accordingly.**

- **Sub-widget hit areas and pixel geometry.** GTK4's default `contains` is a rounded-rect test
  against the CSS border box; Flutter applies inverse transforms per child; Android carries
  `isTransformedTouchPointInView` and Z-translation reordering. All of that machinery exists
  because a widget's hit region is not its layout rectangle. In a terminal a surface's hit region
  *is* its cell rectangle, so the `contains` vfunc, the transform plumbing, and most of the
  pick-customization surface have no counterpart. What survives is the *walk*, not the geometry.
- **Hover, wheel, and multi-pointer.** `Qt::WheelFocus` exists because a wheel over an unfocused
  spinbox is a real interaction. Flutter's entire gesture arena exists to arbitrate between
  recognizers competing for one pointer over time — taps vs. drags vs. long-presses. Android's
  `mFirstTouchTarget` locking and `onInterceptTouchEvent` exist because a gesture is a *stream*
  with an ownership question. froth has clicks, not gestures. **The arena, the interception
  protocol, and the target-locking are solving a problem froth does not have** — do not import
  their vocabulary for a single-event click.
- **Android's `focusableInTouchMode` should not be read as an analogue.** It is a global modality
  bit flipped by the last input device's class, not a per-widget mechanism split. It answers "is
  there a visible focus ring right now," which a terminal does not ask. It is the survey's most
  tempting false friend.
- **A retained widget tree the toolkit owns.** GTK4's central `synthesize_focus_change_events`,
  Flutter's `_removeChild` pruning, Android's `removeViewInternal` teardown, and Qt's
  `hide_helper` focus-shedding all exist because the toolkit owns object lifetimes and gets
  destruction callbacks. froth's surfaces are named and the application holds them. The *policy*
  questions transfer (what happens when the remembered member disappears); the *mechanisms* do
  not.
- **Accessibility trees.** `QLabel`'s focus-policy ladder keys off
  `LinksAccessibleByKeyboard` vs `LinksAccessibleByMouse`; Android's whole `FOCUSABLE_AUTO`
  heuristic is entangled with TalkBack and D-pad TV navigation; GTK4's `can-focus` subtree gate
  serves a11y traversal as much as Tab. Some of the multi-axis complexity in these systems is
  paying for an accessibility tree froth does not maintain. That is a reason to expect froth to
  need *fewer* axes than Qt or Android, not the same number.
- **Qt's `setFocus()` bypass is the single closest match to froth today** — focusability by name
  as universal and unconditional, with the declared policy governing only input-driven paths.
  That is froth's current shape, reached independently by a mature toolkit, and it is evidence
  that "focusable by name" being implicit is a defensible position rather than an omission. It is
  also the position Flutter, GTK4, and Android all declined to take.

---

## Version caveats

- **Qt**: 6.11.2 released, 6.11.3 planned ~Sept 2026 as the last of the 6.11 series
  ([Qt 6.11.2 Released](https://www.qt.io/blog/qt-6.11.2-released),
  [Qt 6.11 Release wiki](https://wiki.qt.io/Qt_6.11_Release)). Source line numbers are from the
  `qt/qtbase` `dev` branch and will drift.
- **GTK**: GTK 4 is current stable. **UNVERIFIED:** that GTK 5 has not been released was not
  independently confirmed live; it is not load-bearing for anything above. Source is
  `GNOME/gtk` `main`.
- **Android**: **UNVERIFIED as to the exact number.** Android 17 / API 37 (stable since
  2026-06-16) is corroborated by three secondary sources
  ([Wikipedia](https://en.wikipedia.org/wiki/Android_17), [apilevels.com](https://apilevels.com/),
  [itechguides.com](https://www.itechguides.com/android-17-release-schedule-is-officially-set/))
  but **no primary Google page stating the current API level was successfully fetched**. All
  behavioral claims are read from AOSP `refs/heads/main` and do not depend on the number.
- **Flutter**: 3.47 is top of the version list on
  [docs.flutter.dev/release/release-notes](https://docs.flutter.dev/release/release-notes)
  (fetched 2026-08-27). **Source excerpts are from `main`, not the 3.47.0 tag** — line numbers and
  exact wording could differ from that tagged release.
- **`HitTestBehavior` prose**: the enum's own api.flutter.dev page 404'd on direct fetch; the
  value descriptions above are paraphrase, but the **mechanism** is quoted directly from
  `RenderProxyBoxWithHitTestBehavior`, which is authoritative.
- **developer.android.com `View` reference**: WebFetch returned only navigation chrome, not method
  prose. Constant-level and method-level claims rest on AOSP source javadoc instead — the more
  authoritative artifact regardless.

---

## Sources

| Source | Used for |
| --- | --- |
| [Qt::FocusPolicy enum](https://doc.qt.io/qt-6/qt.html#FocusPolicy-enum) | The five policy values and their documented meanings |
| `qt/qtbase` `src/corelib/global/qnamespace.h` | Actual bit values: `TabFocus=0x1`, `ClickFocus=0x2`, `StrongFocus=0xB`, `WheelFocus=0xF` |
| [Qt::WidgetAttribute enum](https://doc.qt.io/qt-6/qt.html#WidgetAttribute-enum) | `WA_TransparentForMouseEvents` (51) "as if… not present in the widget hierarchy"; `WA_NoMousePropagation` (73) |
| [QWidget](https://doc.qt.io/qt-6/qwidget.html) | `focusPolicy` default `NoFocus`; `enabled` property; `focusWidget()` doc; `setFocusProxy` doc |
| `qt/qtbase` `qwidget.cpp` | `setFocus` bypassing `focusPolicy` (6547); `updateFocusChild` (6688); `focusWidget` (6879); `nextInFocusChain` (6890); `hide_helper` focus-shedding (8226); `focusInEvent` doc (9700); `childAtRecursiveHelper` (10483); `focus_next` (108, 13288) |
| `qt/qtbase` `qapplication.cpp` | Window-activation focus restore (1877); the ignore→parent propagation loop (2727-2751) |
| `qt/qtbase` `qlabel.cpp`, `qlineedit_p.cpp`, `qabstractbutton.cpp`, `qcommonstyle.cpp` | Per-class focus-policy defaults, including QLabel's ClickFocus-for-selectable ladder |
| `qt/qtbase` `qstackedwidget.cpp`, `qtabwidget.cpp`, `qmdiarea.cpp`, `qdockwidget.cpp` | The negative result: no sub-container focus restore anywhere in qtbase |
| [GTK4 migrating-3to4](https://docs.gtk.org/gtk4/migrating-3to4.html) | The `can-focus` / `focusable` re-split and the default flip |
| [GtkButton](https://docs.gtk.org/gtk4/class.Button.html) | `focus-on-click` inherited from GtkWidget, not a Button property |
| [GtkPropagationPhase](https://docs.gtk.org/gtk4/enum.PropagationPhase.html) | CAPTURE / TARGET / BUBBLE / NONE — delivery along the pick path |
| `GNOME/gtk` `gtk/gtkwidget.c` | Property defaults and doc comments (~1330-1385); `gtk_widget_grab_focus` + `get_effective_can_focus` (~5218); `gtk_widget_pick` / `can_be_picked` / `do_pick` (~10488-10660); `set/get_focus_child` (~12661); focus-child clear on unparent (~2710) |
| `GNOME/gtk` `gtk/gtkenums.h` | `GtkPickFlags` values |
| `GNOME/gtk` `gtk/gtkwindow.c` | `synthesize_focus_change_events` maintaining focus-child up the chain (~5209-5330); hide/show focus-widget retention |
| `GNOME/gtk` `gtk/gtkwidgetfocus.c` | `gtk_widget_focus_move` using focus-child as the Tab and directional anchor (~440-511); `find_old_focus` chain validation |
| `GNOME/gtk` `gtk/gtkentry.c` | `grab_focus` delegating to the internal GtkText (427) |
| [AOSP View.java](https://android.googlesource.com/platform/frameworks/base/+/refs/heads/main/core/java/android/view/View.java) | `FOCUSABLE_AUTO` and the CLICKABLE heuristic (1104-1131, 18474); `isInTouchMode` (17282); `requestFocusNoSearch` (15372); `canTakeFocus` (18443); `setOnClickListener` forcing clickable (7868); `canReceivePointerEvents` (16638); disabled-but-clickable consuming touches (18059) |
| [AOSP ViewGroup.java](https://android.googlesource.com/platform/frameworks/base/+/refs/heads/main/core/java/android/view/ViewGroup.java) | `descendantFocusability` constants (372-390) and default (733); `focusSearch` (1082); `getFocusedChild` (1226); `requestChildFocus` nesting (858-878); `requestFocus` dispatch (3337); `onRequestFocusInDescendants` index scan (3382); `dispatchTouchEvent` child loop (~2733); `dispatchTransformedTouchEvent` (3106-3170); `removeViewInternal` (5595-5636) |
| [AOSP FocusFinder.java](https://android.googlesource.com/platform/frameworks/base/+/refs/heads/main/core/java/android/view/FocusFinder.java) | `findNextFocus` (79), `findNextFocusInAbsoluteDirection` (345), beam scoring (486-600+) |
| [AOSP ViewRootImpl.java](https://android.googlesource.com/platform/frameworks/base/+/refs/heads/main/core/java/android/view/ViewRootImpl.java) | `ensureTouchMode` entry/exit and WindowManagerGlobal round-trip (~7100-7185, 7773-7796); the focusSearch→requestFocus junction (~7928-7954) |
| [FocusNode](https://api.flutter.dev/flutter/widgets/FocusNode-class.html) | `canRequestFocus`, `skipTraversal`, `descendantsAreFocusable`, `descendantsAreTraversable`, `focusedChild` |
| [IgnorePointer](https://api.flutter.dev/flutter/widgets/IgnorePointer-class.html) / [AbsorbPointer](https://api.flutter.dev/flutter/widgets/AbsorbPointer-class.html) | The two documented behaviours, verbatim |
| [ExcludeFocus](https://api.flutter.dev/flutter/widgets/ExcludeFocus-class.html) / [ExcludeFocusTraversal](https://api.flutter.dev/flutter/widgets/ExcludeFocusTraversal-class.html) | Scoped overrides that do not mutate the underlying node flags |
| [GestureArenaManager](https://api.flutter.dev/flutter/gestures/GestureArenaManager-class.html) | "The first member to accept or the last member to not reject wins" |
| `flutter/flutter` `widgets/focus_manager.dart` | `_doRequestFocus` guard; `FocusScopeNode._doRequestFocus` pruning + recursion; `_setAsFocusedChildForScope` nesting; `_removeChild` |
| `flutter/flutter` `widgets/focus_traversal.dart` | The `canRequestFocus && !skipTraversal` filter; `_FocusTraversalGroupNode.lastRequestedFocus`; `_DirectionalPolicyData` history and `_popPolicyDataIfNeeded` |
| `flutter/flutter` `rendering/proxy_box.dart` | `RenderIgnorePointer`/`RenderAbsorbPointer` `hitTest` overrides; `RenderProxyBoxWithHitTestBehavior` (179-216) |
| `flutter/flutter` `rendering/box.dart` | `RenderBox.hitTest` building the leaf→root path (~2921) |
| `flutter/flutter` `gestures/hit_test.dart`, `gestures/binding.dart` | `HitTestResult` path ordering doc; `dispatchEvent` iterating every entry (477) |
| [Flutter release notes](https://docs.flutter.dev/release/release-notes) | Current stable version |
| [Qt 6.11.2 Released](https://www.qt.io/blog/qt-6.11.2-released) / [Qt 6.11 wiki](https://wiki.qt.io/Qt_6.11_Release) | Current Qt version |
