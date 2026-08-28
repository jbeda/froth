# Chrome (border / title / padding / focus indication): a prior-art survey

This merges four parallel prior-art surveys run on 2026-08-26, each conducted
independently against primary sources (source code, official docs, issue trackers).
Facts only — no recommendations for froth, no design synthesis. Organised by question
(axis) rather than by system, so each axis is comparable across all ten systems covered:

- **Immediate-mode / string-render:** Ratatui, Bubble Tea + Lip Gloss + Bubbles
- **Retained-widget TUI:** Textual, Turbo Vision, Terminal.Gui v2, tview, gocui
- **GUI toolkit / platform:** Win32, GTK4, CSS/the web box model

Two honesty caveats from the source surveys are preserved and repeated where relevant,
because they qualify how much weight the rest of that system's answers should carry:

1. **Ratatui has no widget tree, no event loop, no focus, and no mouse routing.** Its
   answers to "who declares chrome" and "who gets which rect" are consequences of
   immediate-mode rendering, not independent design choices about chrome specifically.
   Reading `Block` as a considered chrome design overstates it — `Block` is an ordinary
   widget that happens to draw a frame.
2. **The Terminal.Gui v1→v2 rationale below is inference from v1's own code comments**
   (the `ContentView` hack, the "there is no API to determine this rectangle" doc
   comment, a `TODO: Merge these classes`), not a maintainer statement. No design
   discussion or issue stating the "why" in the maintainers' own words was found.

## Versions checked (all 2026-08-26 local unless noted)

| System | Version / commit | How verified |
| --- | --- | --- |
| Textual (Python) | 8.2.8 | `pypi.org/pypi/textual/json` → `info.version` |
| Ratatui (Rust) | 0.30.2 | crates.io sparse index, last non-yanked line |
| Turbo Vision (magiblot fork) | `1f10395`, no version tags (rolling); frame code is Borland's 1994 TV 2.0 | Source clone |
| Terminal.Gui v2 | `develop` @ `59e6b4d` (2026-08-24); stable NuGet `2.4.17`, prerelease `2.4.18-develop.50` | Source clone + `api.nuget.org` |
| Terminal.Gui v1 | `v1_release` branch, last tag `v1.9.0` | Targeted raw fetches |
| tview (rivo/tview) | v0.42.0 | GitHub releases API |
| lipgloss | v2.0.6 (v1.1.0 also read for comparison) | GitHub releases/tags API |
| bubbletea | v2.0.9 | GitHub releases API |
| bubbles | v2.2.1 | GitHub releases API |
| gocui (awesome-gocui fork) | v1.1.0 | GitHub releases API |
| Win32, GTK4, CSS/web | n/a (living platform docs) | Official docs (`learn.microsoft.com`, `docs.gtk.org`, MDN, W3C), fetched 2026-08-26 |

---

## Capability matrix

| System | Chrome first-class? | Who declares | Consumes layout space | Draw gets | Focus indication | Hit test over chrome | Regret/redesign evidence |
| --- | --- | --- | --- | --- | --- | --- | --- |
| **Ratatui** | Yes, but as an ordinary widget (`Block`), not a framework layer | The construction call site (`.block(Block::bordered()...)`) | Yes — `Block::inner()` computes it | **Outer** rect; widget shrinks itself | None — no focus concept at all | None — no mouse routing at all | `block::Title` removed 0.30; measurement-vs-chrome bug class (#1233); `inner()`/`outer()` friction (#211, #1914) |
| **Textual** | Yes, genuinely — CSS, rendered by the framework outside the widget's draw | Widget class `DEFAULT_CSS`; instance/app CSS; occasionally the parent (`keyline`) | Yes — `gutter` = padding+border; `box-sizing: border-box` default | **Inner** (content-relative) | Framework-drawn, via CSS pseudo-classes and `$border`/`$border-blurred` | Outer rect wins; widget must convert via `get_content_offset` | Border/outline can't coexist (#1475); breaking min/max sizing change; `compact` opt-out (#5778) |
| **Turbo Vision** | Yes, but only for `TWindow` — chrome is a *sibling view*, not a view property | The view's own class hierarchy, via a virtual-base factory hook (`TWindowInit`) | **No** — no inner-rect concept; author manually insets (`r.grow(-1,-1)`) | **Outer**/full-extent; frame draws its own border rows/cols | Framework-drawn, by `TFrame` reading `sfActive` pushed down from `TWindow::setState` | Outer, ordinary z-order hit test — frame is just the sibling under those cells | No RFC found; frame carries interaction behavior, not just paint |
| **Terminal.Gui v2** | Yes, and universal — every `View` has Margin/Border/Padding adornments | The view itself (usually a themeable class default); any caller can set it on any view | Yes, explicit — `Frame` (outer) vs `Viewport` (inner), via `Thickness` | **Inner** (`Viewport`); clip narrowed before content draw | Framework-drawn by `BorderView`, keyed on `Adornment.Parent.HasFocus`; focus can change border geometry | Adornment's own `AdornmentView`, frame-relative coords; bubbling skips the framed view to its `SuperView` | v1→v2 total redesign of the adornment model (rationale [UNVERIFIED], inferred from v1 code comments) |
| **tview** | Yes, structurally — `Box` superclass every widget embeds, chrome opt-in via `SetBorder` | Caller/creator, via setters on the constructed widget; containers don't impose chrome | Yes — `GetInnerRect()`; 1 cell/side when `border == true` | **Outer** for `Box`'s own chrome pass, **inner** (`GetInnerRect`) for subclass content | Framework-drawn, centrally, in `Box.DrawForSubclass`, via `p.HasFocus()` | **Outer**-inclusive by default (`InRect`); `InInnerRect` exists but is opt-in | None found in source; not exhaustively searched in issues [UNVERIFIED] |
| **Bubble Tea / Lip Gloss / Bubbles** | No framework concept — chrome is a `Style` property on a string; no container abstraction exists | Whoever holds the `Style` at render time (often the caller; `textarea` owns its own) | No rect type; explicit `GetFrameSize()`/`GetHorizontalBorderSize()` getters, **not automatic** outside `.Render()` | N/A (string composition); a widget derives its own inner budget via `GetFrameSize()` before rendering | Widget-drawn, per-component, no shared mechanism (`textarea`'s `Focused`/`Blurred` styles) | N/A — no rect/hit-test primitive at all; raw `tea.MouseMsg` cell coordinates only | lipgloss #281 (frame-size/`BorderStyle` mismatch); bubbletea Discussion #240 (unaccounted border broke scrolling); `GetFrameSize` "Provisional" since v1 |
| **gocui** | Yes, more centralized than tview — `View` holds `Frame`/`Title`/`Padding` fields, but the `*Gui` draws them | The creator, via the `x0,y0,x1,y1` rect plus fields; no parent/container (flat) | Yes, but **unconditionally** — 1-cell margin reserved regardless of `Frame` (open bug) | Content only; offset always `+1` regardless of `Frame` | Framework-drawn, centrally, in `Gui.Layout`, gated by a separate `Highlight` opt-in bool | A third variant — strict open-interval test excludes the frame line itself | jroimartin/gocui #70, open since 2016 (`Frame=false` still consumes space); the fork itself is evidence of stagnation |
| **Win32** | Yes — the non-client area, a system-defined region distinct from the client area | Window styles (`dwStyle` bitmask) at creation; immutable after | Yes — two rects (`GetWindowRect`/`GetClientRect`); `AdjustWindowRect` computes the delta | **Client area only** (`WM_PAINT`); non-client painted by `DefWindowProc` unless overridden | System-drawn, via `WM_NCACTIVATE` → `DefWindowProc` | One message, `WM_NCHITTEST`, returns which named region (`HTCLIENT`, `HTCAPTION`, ...) | MS docs warn against custom non-client painting; DWM custom frames lose default move/resize; `microsoft/terminal#872` |
| **GTK4** | Yes — CSS box model on every widget; CSD (client-side decoration) for top-level windows | CSS selectors matching a widget's CSS node; top-level decoration toggled per-window | Yes — full CSS box model (margin/border/padding/content) | Content area (the widget's own local-coordinate origin) | [UNVERIFIED] — no primary source fetched successfully | [UNVERIFIED] — not covered by sources fetched | CSD flashpoint — `GTK-NoCSD` tool, cross-desktop-environment friction, GNOME Discourse proposal thread |
| **CSS / the web** | Yes — box model on every element, same paint pipeline as content; no system/app split | Author stylesheet, per-selector; UA stylesheet defaults `border-box` for some form controls | Yes — `content-box` (spec default) vs `border-box`; border area strictly larger than padding area | Content gets whatever's left after border+padding are subtracted, in either `box-sizing` mode | Historically UA-drawn (`:focus` outline); authors stripped it en masse → `:focus-visible` heuristic added | Implied: border area is part of the hit-test target (not directly quote-verified) | `content-box` default widely treated as an ergonomic mistake (near-universal `border-box` reset); `:focus-visible` born from `:focus` removal breaking accessibility |

---

## 1. Is chrome a first-class framework concept?

### Ratatui — yes, but as a widget, not a framework layer

Chrome is the `Block` widget (`ratatui-widgets/src/block.rs`). It is an ordinary
`Widget`; nothing in the framework knows it is "chrome".

> `//! In its simplest form, a `Block` is a [border](Borders) around another widget. It can have a`
> `//! [title](Block::title) and [padding](Block::padding).`
> — `ratatui-widgets/src/block.rs:5-6`

> `/// A `Block` is a foundational widget that creates visual containers by drawing borders around an`
> `/// area. It serves as a wrapper or frame for other widgets… Most built-in widgets in Ratatui use a`
> `/// pattern where they accept an optional `Block` parameter that wraps the widget's content.`
> — `block.rs:29-32`

The framework-level hook is the convention `block: Option<Block<'_>>` on a widget plus
a `BlockExt` trait:

> `/// An extension trait for [`Block`] that provides some convenience methods.`
> `/// This is implemented for [`Option<Block>`](Option) to simplify the common case of having a`
> `/// widget with an optional block.`
> `pub trait BlockExt { fn inner_if_some(&self, area: Rect) -> Rect; }`
> — `block.rs:1144-1152`

Ten built-in widgets carry that field (`sparkline`, `list`, `tabs`, `gauge`, `paragraph`,
`calendar`, `barchart`, `chart`, `table`, `canvas`; grep `block: Option<Block` in
`ratatui-widgets/src/`). Each re-implements the same three lines of glue. There is no
base class, no trait default, no framework enforcement — a third-party widget that
forgets to accept a `Block` simply cannot be framed except by the caller drawing a
`Block` and passing the inner rect down manually.

### Textual — yes, genuinely first-class and framework-drawn

Chrome is CSS (`border`, `padding`, `outline`, `border-title-*`, `keyline`, plus
scrollbars) and is rendered by the framework in `StylesCache`, outside the widget's own
draw:

> `class StylesCache:`
> `    """Responsible for rendering CSS Styles and keeping a cache of rendered lines.`
> `    The render method applies border, outline, and padding set in the Styles object to widget content.`
> — `src/textual/_styles_cache.py:36-39`

`StylesCache.render_widget(widget, crop)` (`:97`) pulls `widget._border_title`,
`widget._border_subtitle`, `widget.styles`, `widget.region.size`,
`widget.content_region.size`, `styles.padding` and passes `widget.render_line` in as a
*callback* (`:107-145`). The widget never sees the border.

Chrome pieces: `border` (`BorderProperty(layout=True)`, `styles.py:332`);
`border_title_align`/`border_subtitle_align`; `outline` (`BorderProperty(layout=False)`
— *"The outline is drawn \*on top\* of the widget, rather than around it like
border."*, `styles.py:349-352`); `padding`/`margin` (margin is outside the widget);
`keyline` (chrome drawn by a *container* around its children — see §2); and scrollbars,
which Textual itself calls chrome: *`"""Arrange the 'chrome' widgets (typically
scrollbars) for a layout element."""`* (`widget.py:3951-3952`).

### Turbo Vision — yes, but only for `TWindow`, and chrome is a sibling view, not a view property

`TFrame` is an ordinary view (`class TFrame : public TView`, `include/tvision/views.h:553`).
It is **not** owned by `TView`. It is created by `TWindow`'s constructor and **inserted
as a child of the window**:

```cpp
// source/tvision/twindow.cpp:52-56
    if( createFrame != 0 &&
        (frame = createFrame( getExtent() )) != 0
      )
        insert( frame );
```

`getExtent()` — the frame is sized to the window's **entire** rect
(`source/tvision/tview.cpp:521-524`), and it auto-tracks resizes via
`growMode = gfGrowHiX + gfGrowHiY` (`tframe.cpp:31`). `TWindow : public TGroup`;
`TFrame : public TView`; the frame is one of the group's subviews, at the back of the
z-order (inserted first), with a strong back-pointer on the window
(`TFrame *frame;`, `views.h:1022`). `TFrame` reaches *up* into its owner constantly,
casting `owner` to `TWindow*`:

```cpp
// source/tvision/tframe.cpp:65-66
    if( ( ((TWindow *)owner)->flags & (wfClose | wfZoom) ) != 0 )
        l -= 6;
```

The coupling is bidirectional and unchecked — `TFrame` is only valid as a direct child
of a `TWindow`. `TGroup`, `TDeskTop`, and plain `TView` have **no** chrome concept at
all. Chrome in Turbo Vision is a `TWindow` feature, not a view feature.

### Terminal.Gui v2 — yes, and universal: every `View` has three adornment layers

Three named layers, outermost-in: **Margin → Border → Padding**. From the lexicon
(`docfx/includes/layout-lexicon.md`):

> **Adornment** | Lightweight objects (`IAdornment` implementations) that define the
> `Thickness`es separating the `Frame` from the `Viewport`. There are three Adornments:
> `Margin`, `Border`, and `Padding`. Each can lazily create an `AdornmentView` for
> rendering and SubView support via `GetOrCreateView()`. Adornments are not part of the
> View's content and are not clipped by the View's `ClipArea`.

> **Border** | The `Adornment` that resides in the inside of the `Margin`. The Border is
> where a visual border (drawn using line-drawing glyphs) and the `View.Title` are
> drawn, and where the user can interact with the mouse/keyboard to adjust the Views'
> Arrangement.

Types (namespace `Terminal.Gui.ViewBase`): `IAdornment` (contract), `AdornmentImpl`
(base settings object), `Margin`/`Border`/`Padding` (subclasses), `IAdornmentView`/
`AdornmentView` (lazily-created backing `View` that renders), `MarginView`/`BorderView`/
`PaddingView`, `ShadowView`, `TitleView` (tab header, itself a real focusable `View`),
`Arranger`/`ArrangerButton`/`ArrangeButtons` (move/resize interaction).

`Border` is documented as a settings object, not a View:

```csharp
// Terminal.Gui/ViewBase/Adornment/Border.cs:3-16
/// The lightweight Border settings for a <see cref="View"/>. Accessed via <see cref="View.Border"/>.
/// Stores <see cref="Thickness"/>, <see cref="LineStyle"/>, and <see cref="Settings"/> without creating a full View
/// unless rendering, arrangement, or SubViews require it.
```

Allocated eagerly and parented in the constructor:

```csharp
// Terminal.Gui/ViewBase/View.Adornments.cs:112, 225, 334
public Margin Margin { get; } = new ();
public Border Border { get; } = new ();
public Padding Padding { get; } = new ();
```

**Caveat carried from the source survey:** this is `develop`, not the shipped 2.4.17
NuGet package. The lazily-created-`AdornmentView` detail is `develop`-only until
verified; the Frame/Viewport/Thickness model itself is stable across all of v2.

### tview — yes, structurally, and opt-in

`Box` is the shared superclass every widget embeds by value (`*Box` field), and it owns
all chrome state: position, inner-rect cache, padding, border flag/style,
title/title-color/title-align, background color, focus state.

> "Box implements the Primitive interface with an empty background and optional
> elements such as a border and a title. Box itself does not hold any content but
> serves as the superclass of all other primitives." — `box.go:7-11`

Confirmed every concrete widget embeds it (`list.go:43-44`, `table.go:464-465`,
`textview.go:154-155`). Border is **off by default** and **opt-in**: `Box.border` is a
plain `bool` field, zero value `false`; grepping the whole source for internal
`SetBorder(true)` calls inside any widget file returns nothing except the setter
definition itself (`box.go:337`). No widget ships pre-bordered.

### Bubble Tea + Lip Gloss + Bubbles — no runtime concept; chrome is a style property on a string

Architecturally different: Bubble Tea's runtime (v2.0.9) has **no concept of chrome at
all** — grepping its top-level `.go` files for border/style/chrome-named files returns
nothing. A `Model.View()` returns a plain `string`; any border, padding, or title is
baked into that string before it's returned. All chrome lives in Lip Gloss (styling a
string) and, per-component, in Bubbles.

Lip Gloss: chrome is a style property on a `Style` value, applied by whoever holds that
`Style` and calls `.Render(content)` on it — there is no separate "container"
abstraction that imposes chrome on content it did not create. `Style.Border(...)`,
`.BorderForeground(...)`, `.Padding(...)` are plain fluent setters on the same `Style`
struct used for color/bold/etc.

Bubbles: **mixed, per-component, no house rule.** `table.Model` and `list.Model` ship
with **no border at all** by default (`table.DefaultStyles()` has no border,
`table.go:112-118`); a border is the caller's job. `textarea.Model` is the
counterexample — it bakes a border into its own `Styles.Focused.Base`/
`Styles.Blurred.Base` and renders through it inside its own `View()`
(`textarea.go:1590-1598`).

### gocui — yes, more centralized than tview

A `View` has `Frame bool`, `FrameColor`, `FrameRunes`, `Title`, `TitleColor`,
`Subtitle`, `PaddingX`, `PaddingY` fields, but the actual frame/title/corner drawing
code lives not on `View` but on `*Gui` — `gui.go`'s `Layout()` loop calls
`g.drawFrameEdges`, `g.drawFrameCorners`, `g.drawTitle`, `g.drawSubtitle` for each view
with `Frame == true` (`gui.go:590-626`). The GUI owns the chrome render pass across all
views in one place, not each view drawing its own. `Frame` defaults to **true**
(opposite of tview's default-off `border`; `view.go:173-187`).

### Win32 — yes, the non-client area

The non-client area is a system-defined region distinct from the client area:

> "The system sends a WM_NCPAINT message to the window whenever a part of the
> nonclient area of the window, such as the title bar, menu bar, or window frame, must
> be updated... In general, processing these messages for standard windows is not
> recommended, because the application must be able to draw all the required parts of
> the nonclient area for the window. For this reason, most applications pass these
> messages to DefWindowProc for default processing."
> — [Nonclient Area](https://learn.microsoft.com/en-us/windows/win32/gdi/nonclient-area)

By default **the system (via `DefWindowProc`) paints the non-client area** — title bar,
menu bar, window frame/border, scroll bars, system buttons. `WM_NCPAINT`: *"An
application can intercept the WM_NCPAINT message and paint its own custom window
frame."* ([WM_NCPAINT](https://learn.microsoft.com/en-us/windows/win32/gdi/wm-ncpaint))
— the non-default, harder path.

### GTK4 — yes, both as widget CSS and (newly) as client-side window decoration

GTK4's CSS box model gives **every widget** margin/border/padding/content areas, styled
by CSS, rendered by the toolkit's own CSS machinery. `GtkFrame`:

> "Surrounds its child with a decorative frame and an optional label... GtkFrame has a
> main CSS node with name 'frame', which is used to draw the visible border."
> — [Gtk.Frame](https://docs.gtk.org/gtk4/class.Frame.html)

For **top-level windows**, GTK pioneered **client-side decorations (CSD)**: the
application (via `GtkHeaderBar`) draws its own title bar/border/buttons instead of
relying on the window manager to draw a server-side frame.

> "GTK was the first GUI toolkit on Linux that implemented client-side decoration using
> the GtkHeaderBar widget."
> — [Client-side decoration, Wikipedia](https://en.wikipedia.org/wiki/Client-side_decoration)
> (the direct GtkHeaderBar reference page failed to fetch — DNS error — so treat the
> GtkHeaderBar-specific claim as **[PARTIALLY UNVERIFIED]**; the CSD-pioneer claim is
> corroborated by multiple independent secondary sources.)

So: **yes for top-level windows**, GTK moved window-manager-drawn frames (server-side
decoration) to toolkit-drawn frames (CSD) as the GNOME/Wayland default. `GtkFrame` (the
child widget) was never system-drawn; it has always been an ordinary CSS-styled widget.

### CSS / the web — yes, unified with content in one paint pipeline

The box model defines four nested boxes (content, padding, border, margin); the element
itself is styled to have a border/background, drawn by the browser as part of laying
out and painting that element — no separate "system vs. app" split as in Win32.

> "The **content area**... often has a background color or background image." … "The
> **padding area**... extends the content area to include the element's padding." …
> "The **border area**... extends the padding area to include the element's borders."
> — [Introduction to the CSS box model, MDN](https://developer.mozilla.org/en-US/docs/Web/CSS/CSS_box_model/Introduction_to_the_CSS_box_model)

Background paints *under* the border by default (alterable via `background-clip`) —
chrome and content are the same paint pipeline, differentiated only by which box a given
property targets.

---

## 2. Who declares it?

### Ratatui — the code that constructs the widget

`Paragraph::new("Hello, world!").block(Block::bordered().title("My Paragraph"))`
(`block.rs:137`). The block is a builder argument. There is no parent, no container, no
stylesheet — ratatui has no widget tree at all.

> `/// This pattern allows widgets to focus on their content while blocks handle the visual framing.`
> — `block.rs:140`

A `Block` may also be used entirely standalone: render the block, then call
`block.inner(area)` and render whatever you like into the result (`block.rs:100-115`).

### Textual — the widget declares its own defaults; CSS from anywhere can override; occasionally the parent

Three declaring parties, in cascade order:

1. **The widget class**, via `DEFAULT_CSS`, scoped to the type by default
   (`SCOPED_CSS: ClassVar[bool] = True`, `dom.py:154`). Canonical example
   (`_input.py:186-227`):
   ```
   Input {
       padding: 0 2;
       border: tall $border-blurred;
       height: 3;
       &.-textual-compact { border: none !important; height: 1; padding: 0; }
       &:focus { border: tall $border; background-tint: $foreground 5%; }
       &.-invalid { border: tall $error 60%; }
   }
   ```
2. **The widget instance / creating code**, via `widget.styles.border = (...)` or
   `classes=`/`id=` plus app CSS. Title text is a widget attribute, not CSS:
   `border_title = _BorderTitle()` descriptor (`widget.py:384-387`), with class-level
   default `BORDER_TITLE: ClassVar[str] = ""` (`widget.py:313`). Only *alignment* of the
   title is CSS.
3. **The parent**, via ordinary descendant selectors, and via `keyline` — chrome the
   container draws around its children:
   > `def render_keyline(self, container: Widget) -> StripRenderable:`
   > `    """Render keylines around all widgets."""`
   > — `src/textual/layout.py:277-278`

   `Widget.render()` returns the keyline canvas when the widget is a container with a
   keyline set (`widget.py:4453-4456`), and the grid layout reserves the space for it
   (`src/textual/layouts/grid.py:89-101`).

### Turbo Vision — the class, via a virtual-base constructor hook

`TWindow` derives from `TWindowInit`, which carries a function pointer to a frame
factory:

```cpp
// include/tvision/views.h:969-980
class TWindowInit {
public:
    TWindowInit( TFrame *(*cFrame)( TRect ) ) noexcept;
protected:
    TFrame *(*createFrame)( TRect );
};
// include/tvision/views.h:996
class TWindow: public TGroup, public virtual TWindowInit
```

`TWindow`'s own ctor passes `&TWindow::initFrame` (`twindow.cpp:41`), whose body is
`return new TFrame(r);` (`:167-170`). A **subclass** substitutes its own frame class by
naming a different factory in its virtual-base initializer, e.g.
`TWindowInit( &TEditWindow::initFrame )` (`teditwnd.cpp:35`),
`TWindowInit( &TDialog::initFrame )` (`tdialog.cpp:26`). Passing `TWindowInit( 0 )`
yields a window with **no frame at all** — used by several demo windows
(`examples/tvdemo/puzzle.h:71` etc.). Declared by the view's own class hierarchy at
construction time; not by the parent, not per-instance by the creator.

### Terminal.Gui v2 — the view itself (usually its class default), settable by anyone at any time

Every `View` has the three adornments; whether they are visible is `Thickness`.
`Window` sets its own in its constructor:

```csharp
// Terminal.Gui/Views/Window.cs:22-30
public Window () {
    CanFocus = true; TabStop = TabBehavior.TabGroup; Arrangement = ViewArrangement.Overlapped;
    SchemeName = SchemeManager.SchemesToSchemeName (Schemes.Base);
    BorderStyle = DefaultBorderStyle; base.ShadowStyle = DefaultShadow;
}
```

`DefaultBorderStyle`/`DefaultShadow` are `[ConfigurationProperty (Scope = typeof
(ThemeScope))]` — chrome defaults are **themeable configuration**, changeable app-wide
without touching the view. Any caller can also just set it, on any view:
`view.BorderStyle = LineStyle.Single;`, `view.Border.Thickness = new Thickness(1);`
(`docfx/docs/migratingfromv1.md:298-308`). The parent never declares a child's chrome.

### tview — the widget's mechanism, the caller's decision

The widget's own `Box` embed carries the mechanism, but the **caller/creator** declares
whether it's used, by calling `SetBorder`, `SetTitle`, `SetBorderColor`, etc. on the
constructed widget. Containers (`Flex`, `Grid`, `Pages`) do not impose or add chrome to
their children — they only assign each child's **outer** rect and let the child's own
`Box.DrawForSubclass` draw whatever chrome that child has configured.

### Bubble Tea / Lip Gloss / Bubbles — whoever holds the `Style` at render time

For components that don't bake in a border (table, list, viewport), that's the
**application/caller**, wrapping the component's `View()` output. For `textarea`, the
**component itself** declares and owns it (its `Styles` struct, settable via
`SetStyles`), including switching it by focus state internally.

### gocui — the creator, via the rect passed to `SetView`

Declared by the creator, via the `x0,y0,x1,y1` rect passed to
`Gui.SetView(name, x0, y0, x1, y1, overlaps)` (`gui.go:204-227`) plus the
`Frame`/`Title`/etc. fields set on the returned `*View` afterward — there is no
parent/container primitive in gocui (no nesting); every view is a flat,
independently-positioned rectangle managed by the one `*Gui`, so "container" and
"framework" are the same actor here.

### Win32 — window styles, at creation time

Chrome is declared at window-creation time via **window styles** (`dwStyle` passed to
`CreateWindow`/`CreateWindowEx`), not a stylesheet or the widget itself:

| Style | Effect |
| --- | --- |
| `WS_BORDER` | "The window has a thin-line border" |
| `WS_CAPTION` | "The window has a title bar (includes the WS_BORDER style)." |
| `WS_DLGFRAME` | Dialog-style border; window cannot have a title bar |
| `WS_THICKFRAME`/`WS_SIZEBOX` | "The window has a sizing border." |
| `WS_SYSMENU` | Requires `WS_CAPTION` |
| `WS_OVERLAPPEDWINDOW` | Composite of the above plus min/max boxes |

— [Window Styles](https://learn.microsoft.com/en-us/windows/win32/winmsg/window-styles)

These are bit flags set once at creation ("after the window has been created, these
styles cannot be modified, except as noted"); chrome is declarative configuration
handed to the system, not drawn by the widget.

### GTK4 — CSS selectors; top-level decoration toggled per-window

A widget's chrome is declared through **CSS** (selectors matching the widget's CSS
node, e.g. `frame { border-style: ...; }`) — not a creation-time flag bitmask like
Win32's window styles. Top-level window decoration (CSD vs none) is controlled
programmatically per-window; the exact GTK4 call signature is **[UNVERIFIED]** — the
migration-guide fetch that would confirm it did not return usable primary text.

### CSS / the web — the author's stylesheet, per-selector

Declared per-element/per-selector via ordinary CSS properties (`border`,
`border-style`, etc.), not a separate creation-time flag set (Win32) or a toolkit-level
widget class default (GTK's `frame` CSS node) — although `box-sizing` itself has UA
defaults for certain form controls:

> "`box-sizing: border-box` is the default styling that browsers use for the `<table>`,
> `<select>`, and `<button>` elements, and for `<input>` elements whose type is
> `radio`, `checkbox`, `reset`, `button`, `submit`, `color`, or `search`."
> — [box-sizing, MDN](https://developer.mozilla.org/en-US/docs/Web/CSS/box-sizing)

---

## 3. Does chrome consume layout space? Outer vs inner rect

### Ratatui — yes, and the widget computes the inner rect itself

`Block::inner(area) -> Rect` (`block.rs:762-790`) subtracts, in order: 1 cell per
present border edge, 1 cell top/bottom for a title even where there is **no** border on
that edge, then padding:

```rust
if self.borders.intersects(Borders::TOP) || self.has_title_at_position(TitlePosition::Top) {
    inner.y = inner.y.saturating_add(1).min(inner.bottom());
    inner.height = inner.height.saturating_sub(1);
}
```
— `block.rs:768-787`

There is no outer/inner *type* distinction — both are `Rect`. The inverse operation
exists on the geometry type, not the block: `Rect::outer(self, margin: Margin) -> Self`
(`ratatui-core/src/layout/rect.rs:257-268`), added after issue #1914 asked for it.
There is **no** `Block::outer` in 0.30.2 (grep: no match). Measurement must be
duplicated: `Block::vertical_space(&self)` is `pub(crate)` (`block.rs:1133-1141`), used
by `Paragraph::line_count` to account for chrome (`paragraph.rs:337-341`) — not public,
so third-party widgets cannot use it.

### Textual — yes, explicitly, with a full border-box model

`Styles.gutter` = `self.padding + self.border.spacing` (`styles.py:1393-1404`).
`Widget.gutter` = `self.styles.gutter + self.scrollbar_gutter` (`widget.py:2189-2196`).
Outer: `Widget.region` (screen-absolute, includes chrome); `Widget.outer_size` — *"The
size of the widget (including padding and border)"* (`widget.py:2216-2223`). Inner:
`Widget.content_region` = `self.region.shrink(self.styles.gutter)`
(`widget.py:2234-2242`); `Widget.size` is aliased to the **content** size (`widget.py:2198-2205`).

`box-sizing` defaults to **`border-box`**, unlike web CSS:

> `box_sizing = StringEnumProperty(VALID_BOX_SIZING, "border-box", layout=True)`
> — `styles.py:365`

> "When you set padding or border it reduces the size of the widget's content area...
> setting padding or border won't change the width or height of the widget."
> — `docs/guide/styles.md:305-307`

`outline` is the deliberate exception: it does not consume space and paints over
content (`docs/styles/outline.md:3, 22`).

### Turbo Vision — no. There is no inner/content rect concept anywhere.

`TView` has exactly these rect accessors (`source/tvision/tview.cpp`):

```cpp
// :442-445
TRect TView::getBounds() const noexcept
{ return TRect( origin, origin+size ); }          // parent-relative outer rect
// :521-524
TRect TView::getExtent() const noexcept
{ return TRect( 0, 0, size.x, size.y ); }         // self-relative, FULL size
// :475-482
TRect TView::getClipRect() const noexcept          // damage/visibility rect, NOT chrome-aware
```

Because the frame is a full-extent sibling, **every window's content is manually inset
by the application author**:

```cpp
// source/tvision/teditwnd.cpp:56-59
    TRect r( getExtent() );
    r.grow(-1, -1);
    editor = new TFileEditor( r, hScrollBar, vScrollBar, indicator, fileName );
```

The same `r.grow(-1,-1)` idiom appears in every framed demo window. There is no
`getClientRect()`; the framework never subtracts the frame for you. Scrollbars are
placed *onto* the frame's cells by hand, overpainting it because they are nearer the
front of the z-order (`source/tvision/twindow.cpp:197-210`; `TGroup::redraw` draws
`first()` forward, `tgroup.cpp:437-440`).

### Terminal.Gui v2 — yes, explicitly. `Frame` is outer, `Viewport` is inner.

```csharp
// Terminal.Gui/ViewBase/View.Content.cs:505-515
public virtual Rectangle Viewport {
    get {
        Thickness thickness = GetAdornmentsThickness ();
        return new Rectangle (_viewportLocation,
            new Size (Math.Max (0, Frame.Size.Width - thickness.Horizontal),
                      Math.Max (0, Frame.Size.Height - thickness.Vertical)));
    }
    set => SetViewport (value);
}
```

`GetAdornmentsThickness()` = `Margin.Thickness + Border.Thickness + Padding.Thickness`.
Setting the chrome thicker **shrinks the content**, and setting `Viewport.Size` **grows
the Frame** (`View.Content.cs:522-526`, `SetViewport`). `BorderStyle` is sugar that
mutates `Thickness`, and hence layout (`View.Adornments.cs:300-317`). A fourth rect
exists — **Content Area**, origin always `(0,0)`, size from `GetContentSize()`, which
`Viewport` is a scrolling window onto. Scrollbars occupy `Padding`, and turning one on
**thickens the padding**, shrinking the viewport — the exact opposite of Turbo Vision,
where the scrollbar sits *on* the border and costs the content nothing.

### tview — yes, explicit and named

`Box.GetRect()` returns the outer rect; `Box.GetInnerRect()` computes and caches the
content rect:

```go
func (b *Box) GetInnerRect() (int, int, int, int) {
    x, y, width, height := b.GetRect()
    if b.border { x++; y++; width -= 2; height -= 2 }
    x, y, width, height = x+b.paddingLeft, y+b.paddingTop,
        width-b.paddingLeft-b.paddingRight, height-b.paddingTop-b.paddingBottom
    ...
}
```
(`box.go:111-136`) — border costs exactly 1 cell per side, only when `b.border` is
true; padding is additional and independent of the border flag. `InRect(x,y)` tests the
**outer** rect; `InInnerRect(x,y)` tests the inner rect (`box.go:307-319`) — both exist
as named, separate primitives.

### Bubble Tea / Lip Gloss — no rect type; accounting is opt-in

Lip Gloss operates on strings and cell counts, not coordinates. The equivalent of
"does chrome consume space" is answered by explicit size-accounting getters:

```go
// GetHorizontalFrameSize returns the sum of the style's horizontal margins, padding
// and border widths.
// Provisional: this method may be renamed.
func (s Style) GetHorizontalFrameSize() int {
    return s.GetHorizontalMargins() + s.GetHorizontalPadding() + s.GetHorizontalBorderSize()
}
```
(`get.go:446-465`, v2.0.6 — same signature and same "Provisional" doc comment as
v1.1.0's `get.go:389-409`, unchanged across the entire v1→v2 rewrite). Internally,
`Style.Render` subtracts these before laying out content (`lipgloss-style.go:302-303,
408-409`). But this accounting is **not automatic outside `Render`** — nothing forces
a caller doing their own layout math (e.g. sizing a Bubble Tea window in a
`WindowSizeMsg`) to call `GetFrameSize()`; they must remember to (see §7).

### gocui — yes, but unconditionally: a quirk not present in tview

`View.Size()` always reserves a 1-cell margin on each axis regardless of whether
`Frame` is actually true:

```go
func (v *View) Size() (x, y int) {
    return v.x1 - v.x0 - 1 - 2*v.PaddingX, v.y1 - v.y0 - 1 - 2*v.PaddingY
}
```
(`view.go:200-203`), and content is written with a fixed `+1` offset (`view.go:233`).
This `-1`/`+1` is applied whether or not `v.Frame` is true — setting `Frame = false`
stops the border glyphs from being drawn but does **not** give the content that cell of
space back (subject of an open upstream issue — see §7). There is no separate named
"outer rect" vs "inner rect" API; `Size()` is the only content-area accessor, folding
the border subtraction in silently.

### Win32 — yes, two distinct rectangles

- `GetWindowRect` — bounding rectangle including chrome. Vista+ wrinkle: "the Window
  Rect now may include invisible resize borders" — `DwmGetWindowAttribute` with
  `DWMWA_EXTENDED_FRAME_BOUNDS` is needed for the chrome-less visible bounds.
- `GetClientRect` — excludes frame, title bar, decorative elements.
- `AdjustWindowRect` — the inverse: given a desired **client** rect and a style,
  computes the **window** rect needed. *"The AdjustWindowRect function does not take
  the WS_VSCROLL or WS_HSCROLL styles into account."*
  — [AdjustWindowRect](https://learn.microsoft.com/en-us/windows/win32/api/winuser/nf-winuser-adjustwindowrect)

The size of the border/caption is not queryable as a simple constant per style — it
depends on system metrics.

### GTK4 — yes, full CSS box model

> "The CSS stylesheet that is in use determines the sizes (and appearance) of the
> margin, border and padding areas for each widget... the bounds of a widget correspond
> to the border area. The size of the content area is determined by GTK's layout
> algorithm using each widget's `Gtk.WidgetClass.measure` and
> `Gtk.WidgetClass.size_allocate` vfuncs."
> — [Coordinate systems in GTK](https://docs.gtk.org/gtk4/coordinates.html)

Margin can also be set directly per-widget via `GtkWidget` properties `margin-top`,
`margin-bottom`, `margin-start`, `margin-end`.

### CSS / the web — yes, by definition; `content-box` vs `border-box`

The border area is strictly larger than the padding area, which is strictly larger than
the content area. Under `content-box` (the CSS default), an authored `width`/`height`
sizes the **content box only**; border and padding are added on top. Under
`border-box`, the authored dimension bounds the **border box** instead, and the browser
subtracts border+padding to get content size (floored at 0):

> **content-box:** "`.box {width: 350px; border: 10px solid black;}` renders a box that
> is 370px wide." **border-box:** "...renders a box that is 350px wide, with the area
> for content being 330px wide."
> — [box-sizing, MDN](https://developer.mozilla.org/en-US/docs/Web/CSS/box-sizing)

`content-box` is CSS's spec initial value ([CSS Box Sizing Module Level 3,
W3C](https://www.w3.org/TR/css-sizing-3/)).

---

## 4. What area does the widget's own draw/render method get?

### Ratatui — the outer rect

`Widget::render(self, area: Rect, buf: &mut Buffer)` (`ratatui-core/src/widgets/widget.rs:70-76`).
The area passed is the full outer rect; each widget shrinks it itself:

```rust
impl Widget for &Paragraph<'_> {
    fn render(self, area: Rect, buf: &mut Buffer) {
        let area = area.intersection(buf.area);
        buf.set_style(area, self.style);
        self.block.as_ref().render(area, buf);
        let inner = self.block.inner_if_some(area);
        self.render_paragraph(inner, buf);
    }
}
```
— `paragraph.rs:408-416`

The buffer is not clipped to the area either — `Buffer` is screen-sized and any widget
can scribble anywhere; ratatui only relies on convention.

### Textual — the inner (content) rect

`render_content_line(y - gutter.top)` (`_styles_cache.py:455`) — `Widget.render_line(y)`
receives a **content-relative** y and is expected to return a strip of content width.
The higher-level `Widget.render() -> RenderResult` (`widget.py:4432-4458`) returns
content only. Note the asymmetry: **render is inner-relative but mouse events are
outer-relative** (see §6).

### Turbo Vision — the outer rect. `draw()` may paint every cell of its own bounds.

`draw()` takes no argument; the view writes via `writeLine(x, y, count, rows, buf)` in
**self-relative coordinates spanning `0..size.x`**. `TFrame::draw()` writes rows `0` and
`size.y-1` and columns `0`/`size.x-1` of the *window's* full rect (`tframe.cpp:107-119`)
— it can do this because the frame *is* a full-extent view. A framed window's content
view is a *different* view whose bounds were pre-shrunk by the author (§3). Clipping is
by owner rect only.

### Terminal.Gui v2 — the inner (Viewport) rect, with the clip set for you

Draw pipeline order (`docfx/docs/borders.md`):

```
DoDrawAdornments → DoClearViewport → DoDrawSubViews → DoDrawText → DoDrawContent
→ DoDrawAdornmentsSubViews → DoRenderLineCanvas → DoDrawComplete
```

Adornments are drawn first, then the clip is narrowed to the viewport before any
content work (`View.Drawing.cs:132-141`); content hooks receive the `Viewport`
(`View.Drawing.cs:519-527`). Lexicon: *"The View drawing primitives (e.g. `View.Move`)
take **Viewport-Relative** coordinates."* Adornments deliberately escape that clip:
*"Adornments are not part of the View's content and are not clipped by the View's
`ClipArea`."*

### tview — outer for `Box`'s own chrome pass, inner for the subclass's content

`Box.Draw` → `DrawForSubclass(screen, p)` runs on the **outer** rect (`box.go:419-489`):
clears background, draws border runes and title across the full outer rectangle. Every
subclass's own `Draw` calls `t.Box.DrawForSubclass(screen, t)` first, then calls its own
`GetInnerRect()` and draws content only inside that. `SetDrawFunc` is the one place
framework and app code visibly hand off outer→inner: the callback receives the box's
**outer** dimensions and must *return* the inner rect that `GetInnerRect()` will report
thereafter (`box.go:159-170, 481-488`) — a custom draw func can redefine what "inner"
means, and everything downstream uses that redefinition.

### Bubble Tea / Lip Gloss — N/A as coordinates; the widget author decides

The widget author decides, because they call `Style.Render(content)` themselves — the
border is drawn *around whatever string they hand in*, at the moment they choose to
apply it. `textarea` computes its viewport content at the size already reduced by the
border's frame size (`reservedOuter := m.activeStyle().Base.GetHorizontalFrameSize()`,
`textarea.go:1186`) *before* rendering, then wraps that content in the bordered `Base`
style — the "inner" budget is derived by the widget itself, not handed to it.

### gocui — content only, and there is only one kind of draw path

`View.draw()` writes runes through `setRune`, which always offsets into `x0+1, y0+1`
and clamps to `Size()`. Callers writing to a view only ever address content-relative
coordinates; there's no way for application code to draw into the frame cells at all
(the frame is drawn separately, by `Gui.Layout`).

### Win32 — client area only

`WM_PAINT` operates on the client rectangle; the app never receives paint messages for
the frame/title bar unless it explicitly opts into custom non-client painting.

### GTK4 — the content area

"The content area in the CSS box model is the region that the widget considers its
own," and it is the origin of the widget's local coordinate system. Border/padding/
margin are drawn by GTK's CSS rendering, outside what the widget itself considers "its
own" paint region.

### CSS / the web — the box the author's stylesheet targets; content always gets what's left

The element's content paint always gets whatever is left after border+padding are
subtracted, in either `box-sizing` mode; what changes is which box the author's
declared dimension refers to.

---

## 5. Is focus indication framework-drawn?

### Ratatui — neither. There is no focus.

Grep for `focus` across `ratatui-core/src`, `ratatui-widgets/src`, `ratatui/src` returns
only prose uses of the English word. There is no focus concept, no focus ring, no tab
order. Applications track focus in their own state and typically pass a different
`Block::border_style` for the focused pane. Consistent with the immediate-mode model:

> "Instead of creating a fixed set of UI widgets and updating their state, you 'draw'
> your UI from scratch in every frame based on the current application state. There's
> no permanent widget object in memory."
> — ratatui.rs, Concepts → Rendering

### Textual — framework-drawn, expressed through CSS pseudo-classes

Focus is framework state (`Screen.set_focus`), surfaced as pseudo-classes the
stylesheet matches on (`"focus": lambda widget: widget.has_focus`, ..., `widget.py:392-412`).
The *border repaint* is done by `StylesCache`, so the widget's own code is not
involved. Themes ship two dedicated variables so focus indication is a colour swap at
constant thickness:

| Variable | Purpose |
| --- | --- |
| `$border` | "The border color for focused widgets with a border" |
| `$border-blurred` | "The border color for unfocused widgets" |

`Input` declares `border: tall $border-blurred` blurred and `border: tall $border` on
`:focus` — same `tall` thickness both ways, so focusing does **not** reflow. Textual
learned this the hard way; see §7.

### Turbo Vision — framework-drawn, by `TFrame`, from state bits it is told about

`TFrame::draw()` branches on its own `sfDragging`/`sfActive` state to pick palette
entries:

```cpp
// source/tvision/tframe.cpp:39-57
    if( (state & sfDragging) != 0 ) { cFrame = 0x0505; cTitle = 0x0005; f = 0; }
    else if( (state & sfActive) == 0 ) { cFrame = 0x0101; cTitle = 0x0002; f = 0; }
    else { cFrame = 0x0503; cTitle = 0x0004; f = 9; }
```

`f` also selects the line-glyph set — inactive drawn single-line, active double-line.
Only the active frame draws its close/zoom icons. The window pushes the bit down and
the frame redraws itself (`twindow.cpp:172-181`, `tframe.cpp:202-207`). `TFrame` itself
never has focus — it does not set `ofSelectable` (`tframe.cpp:29-33`), and
`TView::handleEvent` only calls `focus()` for `ofSelectable` views (`tview.cpp:551-558`).

### Terminal.Gui v2 — framework-drawn by `BorderView`, keyed on the parent's `HasFocus`

```csharp
// Terminal.Gui/ViewBase/Adornment/BorderView.cs:294-304
bool hasFocus = Adornment.Parent.HasFocus;
if (Adornment.Parent.SuperView is null) { hasFocus = false; }
Adornment.Parent.TitleTextFormatter.Draw (Driver, titleRect,
    GetAttributeForRole (hasFocus ? VisualRole.Focus : VisualRole.Normal),
    GetAttributeForRole (hasFocus ? VisualRole.HotFocus : VisualRole.HotNormal));
```

Focus changes **geometry** too, for tab-style borders — the content-side edge of the
tab header is suppressed when focused (open gap, visually connecting header to
content); unfocused draws a closed separator. Implemented as a focus-dependent
`Thickness` on the `TitleView`'s own border. `BorderView` sets `CanFocus = false`
(`BorderView.cs:66`) — the chrome is not itself in the focus chain. `TitleView`, used
only for tab headers, *is* focusable.

### tview — framework-drawn, centrally, inside `Box.DrawForSubclass`

Every widget gets it for free by virtue of embedding `Box`, with no per-widget code:

```go
if b.border && b.width >= 2 && b.height >= 2 {
    if p.HasFocus() { horizontal = Borders.HorizontalFocus; ... }
    else { horizontal = Borders.Horizontal; ... }
}
```
(`box.go:436-452`) — note `p.HasFocus()`, not `b.HasFocus()`: `DrawForSubclass` takes
the concrete subclass primitive `p` specifically so focus querying dispatches through
the subclass's own (possibly overridden) focus chain, even though border-selection
logic lives in `Box`. `SetBorderColor`/`SetBorderAttributes` exist as a manual override
path independent of focus, but the automatic focus/unfocus glyph-and-style swap happens
regardless, every draw call.

### Bubble Tea / Lip Gloss / Bubbles — widget-drawn, per-component, no shared mechanism

`textarea` is the concrete example: two independent `lipgloss.Style` values
(`Styles.Focused`, `Styles.Blurred`), the component's own `activeStyle()` method picks
one based on its own internal `m.focus` flag, and `View()` renders through whichever is
active (`textarea.go:790-795, 1590-1598`). There is no Bubble Tea or Lip Gloss API that
swaps a border's color automatically when some global "focused model" pointer changes —
each component (or the app author styling a component) must wire that itself.
`table`/`list`'s default styles have no border to change at all.

### gocui — framework-drawn, centrally, gated by an opt-in flag

```go
if v.Frame {
    if g.Highlight && v == g.currentView {
        fgColor = g.SelFgColor; bgColor = g.SelBgColor; frameColor = g.SelFrameColor
    } else { ... }
    g.drawFrameEdges(v, frameColor, bgColor)
    g.drawFrameCorners(v, frameColor, bgColor)
}
```
(`gui.go:590-620`). Gated by a separate `Gui.Highlight bool` switch — focus
color-swapping is off entirely unless the app opts in, distinct from tview where the
swap always runs (only colors are overridable). "Current view" is a single `*Gui`-level
pointer, not a per-view "hasFocus" bool.

### Win32 — system-drawn, tied to the non-client area, via `WM_NCACTIVATE`

The *active* top-level window gets an active-colored title bar, inactive windows
inactive coloring, both drawn by `DefWindowProc`. *"The DefWindowProc function draws
the title bar or icon title in its active colors when the wParam parameter is TRUE and
in its inactive colors when wParam is FALSE."* There is no separate concept of a focus
ring for controls at the Win32 chrome level in the docs fetched here — Common Controls
draw their own focus rectangles inside their client area **[UNVERIFIED]** for
control-level specifics.

### GTK4 — [UNVERIFIED]

No GTK4 API-reference page for a focus-ring CSS node/pseudo-class (default `:focus`
selector behavior, default `outline`-equivalent styling) was fetched successfully.
GTK4's CSS supports `:focus` selectors, consistent with its CSS-alike theming engine,
but the specifics of what an unstyled/default GTK4 theme draws for keyboard focus were
not confirmed against a primary source.

### CSS / the web — historically UA-drawn, then made author-overridable, then re-gated by heuristic

> "Originally, user-agent CSS set focus styles based only on the :focus pseudo-class,
> styling most focused elements with a focus ring outline."
> — search synthesis of MDN's `:focus-visible` page

Authors widely disabled it (`outline: none` on `:focus`), which broke keyboard-
accessible focus indication for sighted users. The fix, `:focus-visible`, is a
**UA-heuristic-gated, author-styleable** pseudo-class:

> "The :focus-visible pseudo-class applies while an element matches the :focus
> pseudo-class and the UA determines via heuristics that the focus should be made
> evident on the element." Chrome and Firefox shipped it in UA stylesheets in February
> 2021, Safari in December 2021.
> — [`:focus-visible`, MDN](https://developer.mozilla.org/en-US/docs/Web/CSS/:focus-visible)

Default focus indication is UA-drawn chrome (an outline — explicitly *not* part of the
box model, does not affect layout, unlike border), author-overridable, and its removal
by authors became a recognized, named accessibility regression that motivated a new
selector.

---

## 6. Hit testing / mouse routing over chrome

### Ratatui — none at all

No mouse routing, no widget map, no event system (`grep MouseEvent
ratatui-core/src ratatui-widgets/src` → no matches). Mouse events come raw from the
backend crate with absolute column/row; the application does its own hit testing,
typically with `Rect::contains` (`ratatui-core/src/layout/rect.rs:355-360`). Whether a
click on the border counts as the widget is entirely the application's choice — nothing
in the library expresses a preference.

### Textual — outer rect wins; a click on the border resolves to the widget

The compositor stores each widget's **outer** `region` (the same region `StylesCache`
paints the border into) and tests point-in-rect against it, per-scanline
front-to-back (`_compositor.py:829-848`). Consequences, confirmed in source:

- **A click on the border focuses the widget.** `Screen._forward_event` calls
  `get_focusable_widget_at` on `MouseDown` (`screen.py:1934-1939`).
- **The forwarded event offset is relative to the OUTER origin** — a click on the
  top-left border corner arrives as `offset == (0, 0)` (`screen.py:1947`).
- **The widget must explicitly convert** to content coordinates and handle the
  "in the chrome" case:
  ```python
  def get_content_offset(self, widget: Widget) -> Offset | None:
      """Get offset within a widget's content area, or None if offset is not in content (i.e. padding or border)."""
      if self.screen_offset not in widget.content_region: return None
      return self.get_content_offset_capture(widget)
  ```
  (`events.py:528-552`)

So Textual routes on the outer rect but renders on the inner rect, and hands widgets an
explicit adapter to bridge the two. Border titles are clickable and can carry links
(see the bug in §7, Textual #4410).

### Turbo Vision — the click resolves to `TFrame`, a real sibling view, by ordinary z-order hit test

`TGroup` routes positional events to the frontmost child containing the point
(`tgroup.cpp:352-383`). There is no special case for chrome: the frame just happens to
be the only child under the border cells. `TFrame::handleEvent` (`tframe.cpp:143-199`)
is where all window manipulation lives — hard-coded column offsets for close/zoom hit
boxes, a nested mouse-capture loop for the close button, and `dragWindow` operating on
the **owner** (`tframe.cpp:126-140`). Window *selection* on click is separate and
happens on `TWindow` itself (`ofSelectable | ofTopSelect`, `twindow.cpp:47`).

### Terminal.Gui v2 — the click resolves to the adornment's own `AdornmentView`, which is not a SubView

Hit testing walks the tree and explicitly descends into adornments **before** subviews
(`View.Layout.cs:1582-1608`). The deepest hit gets a **Frame-relative** (not
viewport-relative) mouse position, the special case for adornments
(`ApplicationMouse.cs:202-213`). **Bubbling skips the framed view** — if the adornment
does not handle the event, the next candidate is the framed view's *SuperView*, not the
framed view (`ApplicationMouse.cs:244-252`). Border interaction is delegated to an
`Arranger`, and which edge does what is **declared by flags on the framed view**
(`ViewArrangement.Movable`, `.Resizable`, `.LeftResizable`, `.BottomResizable`). *"The
Border must be visible (non-zero `Thickness`) for mouse interaction."* Adornments can
be made mouse-transparent per-layer via `ViewportSettingsFlags.TransparentMouse`.

### tview — outer rect, by default

```go
func (b *Box) MouseHandler() ... {
    return b.WrapMouseHandler(func(action MouseAction, event *tcell.EventMouse, setFocus func(p Primitive)) (consumed bool, capture Primitive) {
        if action == MouseLeftDown && b.InRect(event.Position()) { setFocus(b); consumed = true }
        return
    })
}
```
(`box.go:276-284`) — `InRect` tests the full outer rectangle including the border
cells, so a click on the border itself focuses/consumes for that widget. `InInnerRect`
(content-only) exists as a separate method for widgets that want to distinguish
"clicked my border" from "clicked my content", but the default `Box.MouseHandler`
doesn't use it — opt-in for subclasses that override `MouseHandler`.

### Bubble Tea / Lip Gloss — not applicable in the tview/gocui sense

Bubble Tea delivers raw `tea.MouseMsg` with terminal cell coordinates to `Update`;
there is no built-in rect-vs-point primitive or automatic "which widget owns this cell"
resolution — entirely component/app code, using whatever size bookkeeping (including
`GetFrameSize()`) the app already did for layout. **[UNVERIFIED — not fetched]**
whether any Bubbles component (e.g. `list`) implements its own hit-testing against
`tea.MouseMsg`.

### gocui — a third variant: strict-open-interval bounds that exclude the frame line itself

```go
func (g *Gui) ViewByPosition(x, y int) (*View, error) {
    for i := len(g.views); i > 0; i-- {
        v := g.views[i-1]
        if x > v.x0 && x < v.x1 && y > v.y0 && y < v.y1 { return v, nil }
    }
    return nil, ErrUnknownView
}
```
(`gui.go:282-291`) — `x0..y1` are the view's outer bounding coordinates, but the strict
`>`/`<` comparisons mean a click landing exactly on the frame line does **not** resolve
to that view. Closer to "inner-of-outer" than either tview's outer-inclusive `InRect`
or an explicit inner-rect test — an artifact of using strict inequality on the outer
corners rather than a deliberate inner-rect calculation (no `PaddingX`/`PaddingY` or
`Frame`-awareness in this function at all).

### Win32 — one message, `WM_NCHITTEST`, resolves to a named region

> "Sent to a window in order to determine what part of the window corresponds to a
> particular screen coordinate."
> — [WM_NCHITTEST](https://learn.microsoft.com/en-us/windows/win32/inputdev/wm-nchittest)

Selected return codes: `HTCLIENT` (client area), `HTCAPTION` (title bar), `HTSYSMENU`,
`HTMINBUTTON`/`HTMAXBUTTON`, `HTLEFT`/`HTRIGHT`/`HTTOP`/`HTBOTTOM` + corners (resizable
border), `HTBORDER` (non-sizing border), `HTCLOSE`, `HTTRANSPARENT`, `HTNOWHERE`.
**`HTCLIENT` is just one hit-test result among many** — the client area is one region
alongside the chrome regions, resolved by the same single message/return-value
mechanism. Vista+: custom-frame apps should pass this to `DwmDefWindowProc` first so
DWM can hit-test caption buttons.

### GTK4 — [UNVERIFIED]

GTK4 widget hit-testing (`pick()`) and CSD hit-testing (e.g. resize-edge detection on a
CSD window border) were not verified against primary docs in this pass.

### CSS / the web — implied that the border area is part of the hit-test target

> "The pointer-events property controls whether an element can be the target of
> pointer interactions... and how the browser performs hit-testing for that element...
> Normally, the browser chooses the topmost element under the pointer."
> — search synthesis of MDN's `pointer-events` page (page not fetched directly for
> verbatim text in this pass)

This implies, but does not verbatim-confirm, that by default an element's **border
area is part of its hit-test target** (border is spatially part of the element's own
box per the box model), distinct from margin, which is not paintable.
**[UNVERIFIED]** whether margin participates in hit-testing — general web-platform
behavior is that it does not, but this was not confirmed against a primary source.

---

## 7. Evidence of regret, change, or friction

### Ratatui

- **`block::Title` removed in 0.30.0** — the dedicated title struct was deleted in
  favour of `Line` plus positional methods (`BREAKING-CHANGES.md:148-171`, issue #738,
  PR #1926):
  > "The title alignment is better expressed in the `Line` as this fits more coherently
  > with the rest of [the library]" — `BREAKING-CHANGES.md:152`

  Also in the `Block` doc comment: *"Note that prior to `v0.30.0`, the `block::Title`
  struct was used to create titles. This struct has been removed."* Earlier churn:
  `Block::title_on_bottom` deprecated in 0.22 and removed.
- **Chrome-aware measurement is a recurring bug class.** Issue #1233 (closed) —
  *"`Paragraph::line_count` and `line_width` don't take block borders into account"* —
  fixed by threading a `pub(crate) fn vertical_space` through `Paragraph`. Related:
  #658, bottom-positioned title + bottom border on a 2-row area preventing content
  rendering.
- **Users reach for the inverse operation.** Issue #211 ("`Block: block.outer()`
  function"), duplicated by #1914. Maintainer `joshka`, responding to a user asking
  whether `Block::inner` was even meant for application code:
  > **Boltzmachine:** "Oh I just realize maybe I should not use the `Block::inner`
  > function since it is not designed for users to use?"
  > **joshka:** "There's a few reasonable use cases that would make sense to use a
  > block as a user. Anytime you want borders around an area that contains more than
  > one widget a block is the right thing to use (or when some widget doesn't take the
  > block as an argument)."

  The parenthetical is the tell: the `Option<Block>` convention has holes, and the
  fallback is manual `inner()`. Resolved by adding `Rect::outer(Margin)`, not
  `Block::outer`.
- **Still-open requests against the single-`Block` model**: #1468 "Connecting blocks",
  #2448 "Support distinct border colors and side text in `Block` widget", #1975 "Add a
  `clear_first()` method to `Block`". 0.30 added `Block::merge_borders`/`MergeStrategy`
  and `Block::shadow` to relieve some of this.
- No maintainer statement found abandoning the `Option<Block>` pattern.
  **[UNVERIFIED]** whether it has been formally debated in an RFC — none found in the
  repo.

### Textual

- **A border applied only on `:focus` shrinks the content area on focus**, because
  `box-sizing: border-box` is the default and the border is charged to the widget's own
  box. The present-day convention is visible in shipped CSS: every focusable built-in
  reserves the border in the blurred state and changes only the colour, via
  `$border`/`$border-blurred`. Enforced by discipline, not by the framework — nothing
  in Textual prevents a widget from adding chrome on focus.

  **Correction.** This entry previously cited `Textualize/textual#877` as a filed bug
  about focus-border reflow. That is not what #877 is: its title is *"Applying border on
  focus CSS removes content"*, it was opened 2022-10-12 and closed the next day with no
  labels, one bot comment, and no maintainer explanation in the thread. The report's CSS
  uses a descendant selector (`Focusable :focus`, not `Focusable:focus`), which would not
  match the widget at all — consistent with the quick close, but the thread states no
  cause, so the outcome is **[UNVERIFIED]** and #877 is not usable as evidence about
  Textual's focus model. Verified against the GitHub API on 2026-08-27. The convention
  above stands on its own sources (`theme.py`, `docs/guide/design.md`), which were never
  #877.
- **`border` and `outline` cannot coexist on the same edge** — issue #1475 (closed,
  documented as a limitation). Maintainer `rodrigogiraoserrao`:
  > "This is a current (documented) limitation of Textual: `border` and `outline`
  > can't coexist in the same edge of a widget."
- **Breaking change to make sizing chrome-inclusive** — `CHANGELOG.md:2091`:
  *"Breaking change: max/min-width/height now includes padding and border."* (PR
  #3712, fixes #3687, a lost scrollbar in a horizontal layout) — the border-box model
  had to be extended to min/max constraints after shipping.
- **Same measurement bug class as ratatui #1233**: issue #4522, "Content height of
  markdown does not take border/padding into account" (closed).
- **Rendering bugs from the border/content split**: #5634 (log widget border tries to
  render a line -1); #2063 (CSS border-change watch misplaces content); #1175 (border
  of wrapping widget not rendered if background unset); #4410 (`KeyError` on border
  title link click — border titles are in the hit-test surface).
- **Chrome felt too expensive in rows, so it became opt-out.** PR #5778, "Compact
  widgets" — *"Adds `compact` switch for borderless controls"* — added a `compact`
  parameter to `Button`, `Input`, `ToggleButton`, `RadioSet`, `OptionList`, `TextArea`,
  later extended to `MaskedInput` (#5952) and `Binding.Group` (#6132). Implemented as
  a CSS class that zeroes the chrome. A 3-row `Input` for one row of text was the
  complaint being answered.
- **Chrome grew over time, in the CSS rather than in widget code** — evidence the
  CSS-owned model absorbed extensions cheaply: border-title properties (#1864, #2289),
  `BORDER_TITLE`/`BORDER_SUBTITLE` classvars (#3097), `keyline` rule (#3669),
  percentage alpha on borders (#1863), `!important` applying to border (#2420). No
  deprecation or reversal of the model itself was found.

### Turbo Vision & Terminal.Gui v1→v2

**Turbo Vision itself carries little churn evidence** beyond the structural facts in
§1–§6: chrome as a `TWindow`-only, sibling-view feature that carries real interaction
behavior (close/zoom/move/resize hit boxes hardcoded by column offset), not just paint.
No RFC or design document was found discussing whether to generalize `TFrame` beyond
`TWindow`.

**Terminal.Gui v1 had no general chrome concept.** `Window`/`FrameView` faked an inner
rect with a hidden child view. `Border` in v1 is a plain class, not a `View`:

```csharp
// v1_release Terminal.Gui/Core/Border.cs:95
public class Border {
```

`Bounds` in v1 was always the full rect, so `Window` smuggled in a private
`ContentView` subview. The class comment is unusually candid:

```csharp
// v1_release Terminal.Gui/Core/Window.cs:20-23
/// The 'client area' of a <see cref="Window"/> is a rectangle deflated by one or more rows/columns from <see cref="View.Bounds"/>. A this time there is no
/// API to determine this rectangle.
// Window.cs:80-84
/// ContentView is an internal implementation detail of Window. It is used to host Views added with <see cref="Add(View)"/>.
/// Its ONLY reason for being is to provide a simple way for Window to expose to those SubViews that the Window's Bounds
/// are actually deflated due to the border.
```

`FrameView` was a near-duplicate of `Window`, and the file header says so and asks for
the two to be merged:

```csharp
// v1_release Terminal.Gui/Core/Window.cs:5-10
// NOTE: Window is functionally identical to FrameView with the following exceptions.
//  ...
// Any udpates done here should probably be done in FrameView as well; TODO: Merge these classes
```

Mouse-on-chrome in v1 was a hard-coded row test on the Toplevel, not a border object
(`Core/Toplevel.cs:857-863`). Structurally this is close to Turbo Vision's model in
effect (an extra inner view carries the content), except the inner view was hidden from
the developer instead of being theirs to construct.

**What v2 changed, per the official migration guide** (`docfx/docs/migratingfromv1.md`,
"Frame vs Bounds", §227–248): v1's `Bounds` was always `{0,0,Width,Height}`; v2's
`Viewport` is "Visible area in content coordinates" and *"can now be non-zero for
scrolling"*. "Adornments" (§288–308): v2 adds `Border`, `Margin`, `Padding` as built-in
adornments, replacing custom `Border` drawing. `docfx/docs/newinv2.md:289-306` frames
this as a feature: *"v2 introduces a comprehensive Adornment system with lightweight
adornments that lazily create Views when needed."*

**The "why" in maintainer words: [UNVERIFIED].** No design issue or discussion thread
stating the rationale was located; the repo's own docs assert the *what*, not the
*why*. The narrative implied by v1's own source comments — "there is no API to
determine this rectangle", the `ContentView` hack, the `TODO: Merge these classes` — is
the strongest evidence available, and **it is inference from code comments, not a
maintainer statement.**

Secondary v2 changes visible in `develop` but not in the migration guide: `Padding`
became the home for built-in scrollbars, so showing one shrinks the viewport rather
than overpainting the border; `LineCanvas` + `SuperViewRendersLineCanvas` let adjacent
views' border lines auto-join into `┬`/`┴`/`├` junctions (no Turbo Vision analogue);
tab-style borders (`BorderSettings.Tab`, `TitleView`, `TabSide`) make the chrome itself
a tab strip — `develop`-only, not verified against 2.4.17.

### tview

None found in the fetched source (comments, doc strings) beyond ordinary API churn
markers. **[UNVERIFIED]** whether GitHub issues on `rivo/tview` discuss the border/
chrome model specifically — not searched exhaustively; the tview wiki page referenced
in `box.go`'s doc comment was not fetched.

### Bubble Tea / Lip Gloss / Bubbles

- **Lip Gloss issue #281** — "`Style.GetFrameSize()` does not account for border when
  using only `BorderStyle()`" (qualidafial, 2024-04-22): rendering with
  `BorderStyle(border)` (no explicit per-side toggle) visually drew a full border, but
  `GetHorizontalBorderSize()`/`GetVerticalBorderSize()`/`GetFrameSize()` returned 0 for
  that same style — the reported/rendered chrome and the size-accounting API
  disagreed. Fixed in PR #282. This is exactly the outer/inner accounting bug that
  tview and gocui avoid by computing size and draw from the *same* stored
  `border bool`/`Frame bool` field rather than two independently-set style toggles.
- **`charmbracelet/bubbletea` Discussion #240** — "Setting a border around a list
  results in 'breaks' when scrolling the list" (er4hn, 2022-02-22). Maintainer
  meowgorithm's diagnosis: the app sized its list using only its own margin math and
  not the border's cells, so total content exceeded the terminal height. Fix: call
  `docStyle.GetFrameSize()` and subtract it in the `WindowSizeMsg` handler instead of
  hand-rolled arithmetic. Root cause: there is no structural link between "I put a
  border on this style" and "my layout math accounts for it" — that link is one
  function call the app author must remember to make.
- **`GetFrameSize` "Provisional: this method may be renamed"** doc comment present
  unchanged since at least v1.1.0 through current v2.0.6 — flagged as provisional
  across a major version rewrite that did rename large parts of the API
  (`AdaptiveColor`→`LightDark`, etc.), suggesting the maintainers still consider the
  frame-size naming/shape unsettled even after keeping it stable in practice.
- Bubbles v2's `UPGRADE_GUIDE_V2.md` restructured `textarea`'s style API specifically
  around focus/blur border state: `FocusedStyle`/`BlurredStyle` (v1, flat fields)
  became `Styles.Focused`/`Styles.Blurred` (v2, a nested `Styles` struct of type
  `StyleState`) — a reorganization of exactly the focus-chrome mechanism in §5, though
  the underlying "two independent Style values, picked by an internal bool" design
  carried over unchanged.

### gocui

- **jroimartin/gocui issue #70** — "View with no frame -- consume zero character
  space" (kkellner, opened 2016-10-28, still open, labeled enhancement): setting
  `Frame = false` still leaves the border's blank cells taking up space rather than
  returning it to content; the reporter asked for "a view option that consume[s] no
  visual space for its border." Confirmed still true in the current awesome-gocui fork
  nearly a decade later — the `-1`/`+1` offset in `Size()`/`setRune` is unconditional,
  not gated on `v.Frame`.
- The existence of `awesome-gocui/gocui` itself, as a community fork of
  `jroimartin/gocui`, is evidence the original became slow to evolve.
  **[UNVERIFIED]** whether chrome/frame issues specifically motivated the fork — not
  established from primary sources, only inferred from general "more actively
  maintained" framing found via web search, not a fork-rationale document.

### Win32

- **Non-client-area custom painting is explicitly flagged as advanced/fragile by
  Microsoft's own docs:** *"processing these messages for standard windows is not
  recommended, because the application must be able to draw all the required parts of
  the nonclient area."* — Microsoft warning developers away from the very API it
  exposes for custom chrome.
- **Custom title bars carry real cost**: removing the standard frame means "the loss
  of the default resizing and moving behavior," requiring apps to reimplement caption-
  button hit testing and resize/move logic by hand; close/minimize/maximize buttons
  remain DWM/uxtheme-drawn unless told otherwise; "the entire window is your client
  region" once decorations are removed, so UI must be manually offset per frame width.
  — [Custom Window Frame Using DWM](https://learn.microsoft.com/en-us/windows/win32/dwm/customframe),
  corroborated by [microsoft/terminal#872](https://github.com/microsoft/terminal/issues/872)
  ("We need to draw the whole non-client area ourselves").
- **Since Vista, chrome rendering moved partly out of the app's `WndProc` and into the
  compositor:** "the appearance of the non-client areas of application windows... is
  controlled by the DWM" — a system-level acknowledgment that non-client painting
  needed a dedicated compositing layer.

### GTK4 — CSD is the flashpoint

From secondary but well-corroborated sources (GNOME's own Discourse, a KWin
developer's blog, cross-referenced community discussion):

> "GTK refuses to provide reasonable default values and adjust by end users, with GTK
> applications defaulting to client-side decorations. This has created friction across
> different Linux desktop environments... GTK only supports the older and now
> deprecated KDE specific server decoration protocol rather than the newer
> xdg-decoration standard."
> — search synthesis of
> [Server Side Decorations in GTK: A Proposal (GNOME Discourse)](https://discourse.gnome.org/t/server-side-decorations-in-gtk-a-proposal/16029)
> and [Server side decorations and Wayland (Martin Gräßlin/KWin)](https://blog.martin-graesslin.com/blog/2018/01/server-side-decorations-and-wayland/)

Community friction was concrete enough to produce a tool, `GTK-NoCSD`, an
`LD_PRELOAD` shim specifically to force server-side decorations back on. Complaints
cited include inconsistent close-button placement (Fitts's-law violations) and GTK4
forcing non-GNOME environments to reimplement titlebars that worked under server-side
decoration in GTK3. Treat the specific attributed quotes as secondary-source
synthesis, not verbatim primary citations.

### CSS / the web

- `content-box` as the CSS-default `box-sizing` value is widely treated by the
  frontend community as an ergonomic mistake serious enough to warrant a standard,
  near-universal `* { box-sizing: border-box }` reset:
  > "Ugh. If I say the width is 200px, gosh darn it, it's gonna be a 200px wide box
  > even if I have 20px of padding."
  > — [`* { box-sizing: border-box } FTW`, Paul Irish, 2012](https://www.paulirish.com/2012/box-sizing-border-box-ftw/)
  > (Note: the specific framing "the way boxes should have worked from the start"
  > could **not** be verified verbatim — the fetched text supports the sentiment but
  > does not contain that exact phrase; treat any such quotation as paraphrase.)

  CSS frameworks like Bootstrap adopted `border-box` globally; browsers independently
  default several native form controls to `border-box`. The W3C did not change the
  spec's initial value (`content-box` remains default per
  [CSS Box Sizing Module Level 3](https://www.w3.org/TR/css-sizing-3/)), but did add
  `box-sizing` as an escape hatch — tacit acknowledgment that `border-box` is more
  usable for UI-control-shaped boxes even though the general default stayed.
- `:focus-visible` (§5) is itself evidence of redesign: the original always-on
  `:focus` UA outline was widely stripped by authors, breaking accessibility, which
  drove the WICG/CSSWG to spec a heuristic-gated replacement now shipped in all major
  browsers' UA stylesheets.

---

## What transfers poorly to a terminal

Facts and observations only, collected from all four surveys — not a froth
recommendation.

- **Ratatui is not comparable on focus and hit-testing.** It has no widget tree, no
  event loop, no focus, and no mouse routing — those are the application's job or a
  third-party crate's. Its answers to "who declares chrome" and "who gets which rect"
  are consequences of immediate mode, not independent design choices about chrome.
  `Block` is just a widget that happens to draw a frame, and every other widget's
  `Option<Block>` field is a hand-copied convention, not a framework guarantee.
- **Textual's answers are inherited from web CSS**, including the box model, the
  pseudo-class mechanism, the `border`-vs-`outline` split, and the cascade. Its one
  real departure — `box-sizing: border-box` as the default — is justified on
  layout-stability grounds specific to a fixed-cell grid where a border costs a whole
  row. Textual leans on a full retained DOM, a compositor, a stylesheet parser, and a
  reactive system to deliver this; the chrome design assumes all of them exist.
- **Turbo Vision's `TWindowInit` virtual-base function-pointer hook** is a
  C++-specific construction-order workaround with no Go analogue.
- **Terminal.Gui's `Frame`↔`Viewport` mutual setters** (setting `Viewport.Size`
  resizes `Frame`, and vice versa) create a two-way constraint that is easy to state
  and hard to keep consistent — `SetViewport` carries a hand-written no-op guard for
  the "adornments consume the entire Frame" case. Its mouse bubbling from an adornment
  jumps to `Parent.SuperView`, skipping `Parent` — a deliberate but surprising rule
  someone would have to re-derive.
- **Neither Ratatui nor Textual draws chrome for a *parent* around a *child* as the
  primary mechanism.** Textual's `keyline` is the only instance found in that survey,
  and it is a container drawing into its own content area with the grid layout
  hand-reserving the space. Ratatui has no parent at all.
- **Win32's non-client/client split rests on a separate device context and separate
  paint message**, enforced by the OS window manager and (since Vista) the compositor
  (DWM). A terminal has no compositor and no system-owned pixels outside the
  application's own screen buffer — there is no analogous "the system paints this
  without my involvement" layer.
- **GTK4's CSS box model** (margin/border/padding/content, all independently
  styleable, all affecting layout via a measure/allocate pass) presumes sub-pixel or
  at least fine-grained pixel geometry; a terminal's grid is coarse (character cells),
  so margin/border/padding as independently sized areas may not map cleanly to
  discrete cell counts.
- **CSS's `outline`** is explicitly defined not to consume layout space (unlike
  `border`) — a distinction that depends on painting outside the box's own allocated
  rectangle, which requires the renderer to composite an overdraw. Whether a
  terminal's cell-grid model can express "paint outside my allocated cells without
  consuming layout" is not established by this survey.
- **CSD (GTK)** presumes the *application* can draw window-manager-grade chrome
  indistinguishable from system chrome (shadows, rounded corners, compositor-level
  effects) — a terminal application typically cannot draw outside its allocated
  rectangle or composite against a desktop compositor at all.

---

## Open / unverified

Collected `[UNVERIFIED]` flags from all four source surveys, verbatim in substance:

- Whether ratatui maintainers have ever formally considered making chrome a framework
  layer rather than a widget convention — no RFC or design doc found in the repo.
- Whether Textual's choice to give `render_line` content-relative coordinates while
  giving mouse events outer-relative coordinates was deliberate or incidental — no
  issue, PR description, or comment found stating a rationale.
- Third-party ratatui crates that add focus/hit-testing (e.g. `ratatui-interact` on
  crates.io) — not inspected; outside the scope of "what the framework does".
- **Terminal.Gui v1→v2 rationale in the maintainers' own words** — no design issue or
  discussion thread found; the repo's docs assert the *what*, not the *why*. The
  narrative used above is inference from v1's own source comments, not a maintainer
  statement.
- Whether `IAdornment`/`AdornmentImpl`/`AdornmentView`/`TitleView` as read on
  Terminal.Gui `develop` match the shipped 2.4.17 — not diffed.
- Whether the Borland Turbo Vision 2.0 printed reference documents `TFrame` the same
  way the (magiblot fork) source does — worked from source only, did not obtain the
  manual.
- tview's `Borders` struct default focus colors (`borders.go`) — not fetched in
  detail.
- tview wiki page on `Box` (`github.com/rivo/tview/wiki/Box`) — not fetched.
- Whether GitHub issues on `rivo/tview` discuss the border/chrome model specifically
  — not searched exhaustively.
- Whether any Bubbles component (e.g. `list`) implements its own hit-testing against
  `tea.MouseMsg` — not confirmed against source.
- The specific rationale for the `awesome-gocui` fork beyond general "more actively
  maintained" framing found via web search — not a primary fork-rationale document.
- Control-level focus-rectangle rendering specifics in Win32 Common Controls — not
  covered by the non-client-area docs fetched.
- GTK4's exact `gtk_window_set_decorated`-style API surface for toggling CSD — the
  migration-guide fetch did not return usable primary text.
- The `developer-old.gnome.org` `GtkHeaderBar`/`GtkFrame` pages — returned a DNS
  failure at fetch time; the GtkHeaderBar-specific CSD claim is corroborated only by
  secondary sources.
- GTK4 default focus-ring styling — no primary source fetched successfully.
- GTK4 hit-testing (`pick()`) and CSD resize-edge hit-testing — not covered by
  sources fetched.
- CSS default hit-test box and whether margin participates in pointer
  hit-testing — not verified against a primary source; `pointer-events` page not
  fetched directly for verbatim text.
- The exact phrase "the way boxes should have worked from the start" attributed to
  Paul Irish's `box-sizing: border-box` post — the fetched text supports the
  sentiment but does not contain the exact phrase; treat as paraphrase.

---

## Sources

### Ratatui / Textual

| # | Source | What it establishes |
| --- | --- | --- |
| 1 | `https://pypi.org/pypi/textual/json` (checked 2026-08-26) | Textual 8.2.8 is current |
| 2 | `https://index.crates.io/ra/ta/ratatui` (checked 2026-08-26) | ratatui 0.30.2 is current, not yanked |
| 3 | ratatui `ratatui-widgets/src/block.rs` @ `ratatui-v0.30.2`, lines 1-6, 27-70, 87-89, 100-140, 762-790, 792-796, 799-816, 1133-1141, 1144-1159 | `Block` API, `inner()` arithmetic, title-implies-row, `BlockExt`, `vertical_space` is `pub(crate)` |
| 4 | ratatui `ratatui-widgets/src/paragraph.rs` lines 332-341, 403-416 | Widget renders block then computes inner itself; `line_count` accounts for chrome |
| 5 | ratatui `ratatui-core/src/widgets/widget.rs:70-76`, `stateful_widget.rs:124-134` | `render(self, area, buf)` — widget gets outer rect |
| 6 | ratatui `ratatui-core/src/layout/rect.rs:257-268, 355-360` | `Rect::outer(Margin)`, `Rect::contains` |
| 7 | ratatui `BREAKING-CHANGES.md:19, 66, 148-177, 826-835` | `block::Title` removal in 0.30, `title_on_bottom` deprecation |
| 8 | ratatui `ARCHITECTURE.md` | 0.30 workspace split; `Block` lives in `ratatui-widgets` |
| 9 | GitHub issues ratatui/ratatui #211 (+comments), #738, #1233, #1468, #1914, #1975, #2448 | Regret/friction evidence; joshka + Boltzmachine quotes |
| 10 | `https://ratatui.rs/concepts/rendering/` | Immediate-mode statement, no retained widget objects |
| 11 | Textual `src/textual/_styles_cache.py:36-39, 97-145, 162-200, 300-480` | Framework draws border/padding/title/outline around a `render_line` callback; content-relative y |
| 12 | Textual `src/textual/widget.py:249-276, 313, 384-412, 507-508, 1750-1815, 1881-1901, 2186-2300, 3951-3981, 4250-4269, 4432-4458` | Geometry properties, border-box, pseudo-classes, `border_title`, scrollbars-as-chrome, render contract |
| 13 | Textual `src/textual/css/styles.py:325-365, 607-613, 1393-1404` | `border`/`outline`/`keyline`/`box_sizing` rules, `gutter` = padding + border |
| 14 | Textual `src/textual/box_model.py`, `src/textual/map_geometry.py` | `BoxModel` = content+padding+border; `MapGeometry.region` is the outer region |
| 15 | Textual `src/textual/_compositor.py:829-865` | `get_widget_at` — per-scanline rect hit test on outer regions |
| 16 | Textual `src/textual/screen.py:687-709, 1924-1947` | Border click focuses the widget; event offset is outer-relative |
| 17 | Textual `src/textual/events.py:501-552` | `get_content_offset` returns `None` "if offset is not in content (i.e. padding or border)" |
| 18 | Textual `src/textual/widgets/_input.py:186-243` | Widget-declared default CSS incl. `:focus` border and `-textual-compact` |
| 19 | Textual `src/textual/layout.py:277-306`, `src/textual/layouts/grid.py:82-102` | `keyline` — parent-drawn chrome, layout reserves the space |
| 20 | Textual `src/textual/theme.py:154, 265, 290, 355, 379, 403, 462, 491`, `docs/guide/design.md:260-267` | `$border`/`$border-blurred` focus-indication convention |
| 21 | Textual `docs/guide/styles.md:43, 114-116, 244-320`; `docs/styles/border.md`; `docs/styles/outline.md`; `docs/styles/box_sizing.md` | Border costs 2 rows/cols; border-box rationale; outline overlays content; border+outline exclusion |
| 22 | Textual `CHANGELOG.md:624, 1126, 2035, 2091, 2326, 2733, 2860-2861` | Compact widgets; keyline; breaking min/max change; border-title feature history |
| 23 | GitHub issues/PRs Textualize/textual #1475 (+comments), #3687, #3712, #4410, #4522, #5634, #5778, #5952, #6132 | Border/outline exclusion (rodrigogiraoserrao quote), sizing breaking change, measurement bugs, compact-widget motivation |
| 24 | GitHub issue Textualize/textual #877, re-checked via `gh api` 2026-08-27 | Title, dates, labels and comment count only — the thread states no cause and supports no claim about focus-border reflow |

### Turbo Vision / Terminal.Gui

| # | Source | Type | Accessed |
| --- | --- | --- | --- |
| 1 | `magiblot/tvision` @ `1f10395` — `include/tvision/views.h`, `include/tvision/app.h`, `source/tvision/tframe.cpp`, `twindow.cpp`, `tview.cpp`, `tgroup.cpp`, `teditwnd.cpp`, `tdialog.cpp`, `framelin.cpp`, `examples/tvdemo/*` | Source code (cloned) | 2026-08-26 |
| 2 | `magiblot/tvision` `README.md` | Project doc | 2026-08-26 |
| 3 | `gui-cs/Terminal.Gui` `develop` @ `59e6b4d` — `Terminal.Gui/ViewBase/View.Adornments.cs`, `View.Content.cs`, `View.Layout.cs`, `View.Drawing.cs`, `View.Drawing.Clipping.cs`, `View.ScrollBars.cs`, `ViewBase/Adornment/{IAdornment,Border,BorderView,BorderView.Arrangement,Arranger,TitleView}.cs`, `App/Mouse/ApplicationMouse.cs`, `Views/Window.cs` | Source code (cloned) | 2026-08-26 |
| 4 | `gui-cs/Terminal.Gui` `develop` — `docfx/docs/borders.md` ("Borders Deep Dive") | Official doc (in-repo) | 2026-08-26 |
| 5 | `gui-cs/Terminal.Gui` `develop` — `docfx/docs/layout.md`, `docfx/includes/layout-lexicon.md`, `docfx/includes/view-composition.md` | Official doc (in-repo) | 2026-08-26 |
| 6 | `gui-cs/Terminal.Gui` `develop` — `docfx/docs/migratingfromv1.md`, `docfx/docs/newinv2.md` | Official migration guide | 2026-08-26 |
| 7 | `gui-cs/Terminal.Gui` `v1_release` — `Terminal.Gui/Core/Window.cs`, `Core/Border.cs`, `Core/Toplevel.cs`, `Views/FrameView.cs` (raw.githubusercontent.com) | Source code | 2026-08-26 |
| 8 | `https://api.nuget.org/v3-flatcontainer/terminal.gui/index.json` | Package registry | 2026-08-26 |

Published docs for these files are at `https://tui-cs.github.io/Terminal.Gui/docs/` — the
URLs the in-repo sources themselves reference. The in-repo Markdown, which is the build
input for those pages, was read directly.

### Go ecosystem (tview, Bubble Tea/Lip Gloss/Bubbles, gocui)

| # | Source | Type | Fetched |
| --- | --- | --- | --- |
| 1 | `rivo/tview` GitHub repo, tag/HEAD `master` (release v0.42.0), files `box.go`, `primitive.go`, `list.go`, `table.go`, `textview.go`, `flex.go` | primary source | 2026-08-26 |
| 2 | `rivo/tview` GitHub releases API | version check | 2026-08-26 |
| 3 | `awesome-gocui/gocui` GitHub repo, `master` (release v1.1.0), files `view.go`, `gui.go` | primary source | 2026-08-26 |
| 4 | `awesome-gocui/gocui` GitHub releases API | version check | 2026-08-26 |
| 5 | `jroimartin/gocui` issue #70, "View with no frame -- consume zero character space" | primary source (GitHub issue) | 2026-08-26 |
| 6 | `charmbracelet/lipgloss` GitHub repo, tag `v2.0.6`, files `style.go`, `borders.go`, `size.go`, `get.go` | primary source | 2026-08-26 |
| 7 | `charmbracelet/lipgloss` GitHub repo, tag `v1.1.0`, file `get.go` | primary source (for v1 comparison) | 2026-08-26 |
| 8 | `charmbracelet/lipgloss` GitHub releases/tags API | version check | 2026-08-26 |
| 9 | `charmbracelet/lipgloss` issue #281, "`Style.GetFrameSize()` does not account for border when using only `BorderStyle()`", and PR #282 | primary source (GitHub issue) | 2026-08-26 |
| 10 | `charmbracelet/bubbletea` Discussion #240, "Setting a border around a list results in 'breaks' when scrolling the list" | primary source (GitHub discussion) | 2026-08-26 |
| 11 | `charmbracelet/bubbletea` GitHub repo file listing, tag `v2.0.9` (confirms no border/style-related files in the runtime itself) | primary source | 2026-08-26 |
| 12 | `charmbracelet/bubbles` GitHub repo, tag `v2.2.1`, files `UPGRADE_GUIDE_V2.md`, `textarea/textarea.go`, `table/table.go` | primary source | 2026-08-26 |
| 13 | `charmbracelet/bubbles` GitHub releases API | version check | 2026-08-26 |

Not fetched / [UNVERIFIED] in this pass: tview's `Borders` struct default focus colors
(`borders.go`); tview wiki page on `Box`; whether any Bubbles component does its own
mouse hit-testing; the specific rationale for the `awesome-gocui` fork beyond general
"more actively maintained" framing found via web search.

### Win32 / GTK4 / CSS

| # | Title | URL | Used for |
| --- | --- | --- | --- |
| 1 | Nonclient Area | https://learn.microsoft.com/en-us/windows/win32/gdi/nonclient-area | Who paints chrome |
| 2 | WM_NCPAINT message | https://learn.microsoft.com/en-us/windows/win32/gdi/wm-ncpaint | Who paints chrome |
| 3 | WM_NCACTIVATE message | https://learn.microsoft.com/en-us/windows/win32/winmsg/wm-ncactivate | Who paints chrome; focus |
| 4 | Window Styles (Winuser.h) | https://learn.microsoft.com/en-us/windows/win32/winmsg/window-styles | Declaration |
| 5 | GetWindowRect function | https://learn.microsoft.com/en-us/windows/win32/api/winuser/nf-winuser-getwindowrect | Layout space |
| 6 | AdjustWindowRect function | https://learn.microsoft.com/en-us/windows/win32/api/winuser/nf-winuser-adjustwindowrect | Layout space |
| 7 | WM_NCHITTEST message | https://learn.microsoft.com/en-us/windows/win32/inputdev/wm-nchittest | Hit testing |
| 8 | Custom Window Frame Using DWM | https://learn.microsoft.com/en-us/windows/win32/dwm/customframe | Regret/redesign |
| 9 | microsoft/terminal issue #872 | https://github.com/microsoft/terminal/issues/872 | Regret/redesign corroboration |
| 10 | Gtk.Frame (GTK 4 docs) | https://docs.gtk.org/gtk4/class.Frame.html | Who draws |
| 11 | Coordinate systems in GTK (GTK 4 docs) | https://docs.gtk.org/gtk4/coordinates.html | Box model |
| 12 | CSS boxes in GTK+ (GNOME blog, mclasen) | https://blogs.gnome.org/mclasen/2015/12/22/css-boxes-in-gtk/ | Box model |
| 13 | Gtk.Widget:margin-top property | https://docs.gtk.org/gtk4/property.Widget.margin-top.html | Box model |
| 14 | Client-side decoration (Wikipedia) | https://en.wikipedia.org/wiki/Client-side_decoration | CSD history |
| 15 | Server Side Decorations in GTK: A Proposal (GNOME Discourse) | https://discourse.gnome.org/t/server-side-decorations-in-gtk-a-proposal/16029 | CSD controversy |
| 16 | Server side decorations and Wayland (Martin Gräßlin) | https://blog.martin-graesslin.com/blog/2018/01/server-side-decorations-and-wayland/ | CSD controversy |
| 17 | GTK-NoCSD (OSnews) | https://www.osnews.com/story/144473/gtk-nocsd-an-ld_preload-library-to-disable-csds/ | CSD controversy |
| 18 | Introduction to the CSS box model (MDN) | https://developer.mozilla.org/en-US/docs/Web/CSS/CSS_box_model/Introduction_to_the_CSS_box_model | Who paints |
| 19 | box-sizing (MDN) | https://developer.mozilla.org/en-US/docs/Web/CSS/box-sizing | Declaration; content-box vs border-box |
| 20 | CSS Box Sizing Module Level 3 (W3C) | https://www.w3.org/TR/css-sizing-3/ | Spec default |
| 21 | `* { box-sizing: border-box } FTW`, Paul Irish (2012) | https://www.paulirish.com/2012/box-sizing-border-box-ftw/ | Regret/redesign |
| 22 | `:focus-visible` (MDN) | https://developer.mozilla.org/en-US/docs/Web/CSS/:focus-visible | Focus indication |
| 23 | Managing User Focus with :focus-visible (CSS-Tricks) | https://css-tricks.com/managing-user-focus-with-focus-visible/ | Focus indication |
| 24 | Don't Set outline: 0 or outline: none for Focus-Styles (Eevis Panula) | https://eevis.codes/blog/2022-07-28/dont-set-outline-0-or-outline-none-for-focus/ | Focus indication |
| 25 | All browsers adopted :focus-visible in their UA stylesheets (Stefan Judis) | https://www.stefanjudis.com/today-i-learned/all-browsers-adopted-focus-visible-in-their-ua-stylesheets/ | Focus indication |
| 26 | pointer-events (MDN) | https://developer.mozilla.org/en-US/docs/Web/CSS/pointer-events | Hit testing |

Items marked [UNVERIFIED] in the body could not be confirmed against a primary source
in this research pass (notably: exact GTK4 `gtk_window_set_decorated`-style API
surface for toggling CSD; `developer-old.gnome.org` `GtkHeaderBar`/`GtkFrame` pages,
which returned a DNS failure at fetch time; GTK4 default focus-ring styling and
hit-testing; CSS default hit-test box and margin's participation in pointer
hit-testing).
