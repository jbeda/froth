# Bubble Tea v2 / Lip Gloss v2 compositing primitives

Research date: 2026-08-22. All version facts verified against live git tags, the GitHub API, and pkg.go.dev on that date.

## Bottom line

**No — froth should not write a cell-buffer compositor from scratch.** Lip Gloss v2 (stable, `v2.0.6`, 2026-08-11) ships `Canvas`, `Layer`, `Compositor`, and `LayerHit` in the top-level package: a real cell buffer with absolute X/Y positioning, integer Z-index, nested layers, clipping at canvas bounds, correct wide-grapheme handling, and point hit-testing by layer ID. The maintainer (`meowgorithm`) closed bubbletea#642 pointing at exactly this, saying "We've been using it in production for quite some time." Confidence: **high** for the primitive existing and being the sanctioned path; verified by reading `v2.0.6` source and by running probe programs against it.

Two caveats that shape froth's design rather than reverse the answer. (1) **There is no transparency.** `Layer.Draw` blanks its entire bounding rectangle before painting, so a space inside a layer punches a hole through everything below it — verified empirically. (2) The Bubble Tea boundary is still a **string**: `tea.View.Content` is `string`, so every frame round-trips cells → ANSI string → cells in the renderer. The escape hatch for both is that `lipgloss.Canvas` implements `uv.Screen` (`CellAt`/`SetCell`) and consumes any `uv.Drawable` — froth can implement `uv.Drawable` for its own surface type and get arbitrary clipping, transparency, and blend rules while reusing Charm's buffer, diffing, and grapheme width logic. That is a small amount of code, not a compositor.

---

## 1. Release status (verified live, 2026-08-22)

| Module | Import path | Latest tag | Date | Status |
|---|---|---|---|---|
| Bubble Tea v2 | `charm.land/bubbletea/v2` | `v2.0.9` | 2026-08-19 (commit `73b6d91`) | stable |
| Lip Gloss v2 | `charm.land/lipgloss/v2` | `v2.0.6` | 2026-08-11 | stable |
| ultraviolet | `github.com/charmbracelet/ultraviolet` | **no tags at all** | pinned by pseudo-version | pre-v1, untagged |

- Bubble Tea `v2.0.0` released 2026-02-24 (GitHub API, `releases/tags/v2.0.0`, `published_at: 2026-02-24T15:32:26Z`). Lip Gloss `v2.0.0` released 2026-02-24 (`2026-02-24T15:33:59Z`).
- **Import paths changed to vanity domains.** `go.mod` at lipgloss `v2.0.6` declares `module charm.land/lipgloss/v2`; bubbletea `v2.0.9` declares `module charm.land/bubbletea/v2`. Not `github.com/charmbracelet/...`. Confirmed in the v2.0.0 release notes ("❤️ Charm Land Import Path").
- **Stability risk to note:** both libraries depend on `github.com/charmbracelet/ultraviolet`, which has **zero git tags** (`git tag | wc -l` → 0 on a fresh clone at commit `68fa937`). Lip Gloss v2.0.6 pins `v0.0.0-20260811164956-006e29f97886`; Bubble Tea v2.0.9 pins a *different, older* pseudo-version `v0.0.0-20260703014108-f5a850f9c2b7`. Any froth code that names `uv.` types in its public API is coupling to an untagged, unversioned module whose two consumers are not even on the same commit.

## 2. The primitives (lipgloss v2.0.6, `canvas.go` + `layer.go`)

Verified by reading source at tag `v2.0.6` and cross-checked against pkg.go.dev.

```go
// canvas.go — a cell buffer
type Canvas struct{ scr uv.ScreenBuffer }   // implements uv.Screen and uv.Drawable
func NewCanvas(width, height int) *Canvas
func (c *Canvas) Compose(drawer uv.Drawable) *Canvas
func (c *Canvas) Draw(scr uv.Screen, area uv.Rectangle)
func (c *Canvas) CellAt(x, y int) *uv.Cell
func (c *Canvas) SetCell(x, y int, cell *uv.Cell)
func (c *Canvas) Bounds() uv.Rectangle
func (c *Canvas) Resize(width, height int)
func (c *Canvas) Clear()
func (c *Canvas) Render() string            // uv.TrimSpace(c.scr.Render())

// layer.go — positioned content
type Layer struct{ id, content string; width, height, x, y, z int; layers []*Layer }
func NewLayer(content string, layers ...*Layer) *Layer
func (l *Layer) X(int) *Layer               // chainable setters, all relative to parent
func (l *Layer) Y(int) *Layer
func (l *Layer) Z(int) *Layer
func (l *Layer) ID(string) *Layer
func (l *Layer) AddLayers(...*Layer) *Layer
func (l *Layer) GetLayer(id string) *Layer
func (l *Layer) MaxZ() int
func (l *Layer) Draw(scr uv.Screen, area uv.Rectangle)   // implements uv.Drawable

// layer.go — flattening + z-sort + hit test
type Compositor struct{ root *Layer; layers []compositeLayer; index map[string]*Layer; bounds image.Rectangle }
func NewCompositor(layers ...*Layer) *Compositor
func (c *Compositor) AddLayers(...*Layer) *Compositor
func (c *Compositor) Refresh()              // MUST call after mutating layer tree/positions
func (c *Compositor) Bounds() image.Rectangle
func (c *Compositor) Draw(scr uv.Screen, area image.Rectangle)
func (c *Compositor) Hit(x, y int) LayerHit
func (c *Compositor) GetLayer(id string) *Layer
func (c *Compositor) Render() string

type LayerHit struct{ id string; layer *Layer; bounds image.Rectangle }
func (lh LayerHit) Empty() bool
func (lh LayerHit) ID() string
func (lh LayerHit) Layer() *Layer
func (lh LayerHit) Bounds() image.Rectangle
```

Note that the `v2.0.0-beta.2` release notes show a *different* API (`NewCanvas(a.X(5).Y(10).Z(1), b.X(3).Y(7))`). That shipped API no longer exists: `Canvas` became a sized cell buffer and layer management moved into `Compositor`. Treat the beta release notes as stale; read the source.

## 3. Capability matrix (what it does and does not do)

Verified empirically by running probe programs against `charm.land/lipgloss/v2@v2.0.6` under Go 1.25.0.

**Z-order — yes, with a caveat.** `Compositor.flatten()` sorts the flattened layer list with `slices.SortFunc(c.layers, func(a, b compositeLayer) int { return a.layer.z - b.layer.z })` and draws low-to-high. Z is **absolute, not hierarchical**: `flattenRecursive` accumulates X and Y from the parent but copies Z verbatim, so a child with `Z(0)` inside a parent with `Z(10)` sorts *below* an unrelated `Z(5)` layer. There is no per-subtree stacking context, unlike CSS. Also, `slices.SortFunc` is **not stable** — layers sharing a Z value have no guaranteed relative order. Froth should assign unique Z values rather than rely on insertion order. (Charm's own `bubbletea/examples/clickable` sidesteps this by assigning `Z(i+1)` per dialog.)

**Absolute positioning — yes.** `Layer.X/Y` are offsets relative to the parent layer; the root of a `Compositor` is an implicit empty layer at the origin, so top-level layers are effectively absolute.

**Clipping — partial, and only at the buffer edge.** `Compositor.Draw` calls `cl.layer.Draw(scr, cl.bounds)` with the layer's *own* absolute bounds — it ignores the `area` argument it was handed. Clipping therefore falls out of `Buffer.SetCell` dropping out-of-range writes (`buffer.go:296-302`: `if y < 0 || y >= len(b.Lines) { return }`). Consequences:
- Drawing onto a fixed-size `Canvas` clips correctly at the canvas rectangle. Probe: a 2-line layer at `X(7).Y(4)` on a 10×5 canvas rendered `XXXXXXXYYY` on the last row with the second row discarded.
- **There is no viewport/scissor rect.** You cannot say "draw this layer but clip it to this sub-rectangle." A pane that must clip its child content to its own frame has to either pre-truncate the content string or compose it on a private `Canvas` of the right size first and use that canvas as the drawable.
- **Negative coordinates are lost, and `Compositor.Render()` mis-sizes.** `Render()` builds `NewCanvas(c.bounds.Dx(), c.bounds.Dy())` but never translates the origin. Probe: layers at `X(-2).Y(-1)` and `X(0).Y(0)` gave `bounds=(-2,-1)-(4,3)` (6×4) yet rendered a 4×3 block with the off-origin content silently dropped. Never rely on `Compositor.Render()` when any coordinate can go negative — allocate the `Canvas` yourself at known screen size and use `Canvas.Compose`.

**Transparency — no. This is the significant limitation.** `uv.StyledString.Draw` (`ultraviolet/styled.go:50-61`) begins with:
```go
// Clear the area before drawing.
for y := area.Min.Y; y < area.Max.Y; y++ {
    for x := area.Min.X; x < area.Max.X; x++ {
        buf.SetCell(x, y, nil)   // nil == EmptyCell, a space
    }
}
```
Since `Layer.Draw` is just `uv.NewStyledString(l.content).Draw(scr, area)`, **every layer is fully opaque over its entire bounding rectangle**, including the spaces inside it and any ragged short lines. Probe: layer content `"AB\n  \nCD"` placed over a field of `X` rendered `XXABXXXXXX / XX··XXXXXX / XXCDXXXXXX` — the two spaces punched through. There is no alpha, no "skip this cell", no blend mode. Non-rectangular or see-through surfaces are not expressible with `Layer` as shipped.

**Hit testing — yes, better than expected.** `Compositor.Hit(x, y)` walks the z-sorted list from highest to lowest and returns the topmost ID-bearing layer whose bounds contain the point. It works on bounding rectangles only (no per-cell hit masks) and skips layers with an empty ID. This covers the input-routing half of surface management for mouse events, and is roughly what `bubblezone` did for v1.

**Wide graphemes — handled.** `Canvas` sets `Method = ansi.GraphemeWidth`; probe placed `日本` at X(1) over a 6-wide field and got `X日本X` (6 columns). Cell width bookkeeping is in `uv.Cell.Width`.

**Mutation cost.** `Compositor` caches a flattened slice; `Refresh()` re-walks and re-sorts the whole tree, and `AddLayers` calls `flatten()` unconditionally. There is no incremental/dirty-region update at the `Compositor` level. Below it, `uv.RenderBuffer.SetCell` does track dirty lines (`TouchLine`), and Bubble Tea's cursed renderer diffs frames, so the redundant work is bounded at composition, not at the terminal write.

## 4. bubbletea#642 — what it actually says

Source: GitHub API, `repos/charmbracelet/bubbletea/issues/642` and `/comments`.

- Title: "Support for modals, overlapping windows". Opened by `rambhosale` 2023-01-23. **Closed the same day**, 2023-01-23T14:46:07Z, `state_reason: completed`. 3 comments.
- `meowgorithm` [MEMBER], 2023-01-23, closing comment: "Compositing is still a work in progress, however for now there's a PR in Lip Gloss that may get you most of the way there: lipgloss#102 ... Otherwise, please follow bubbletea#79 for updates. (Duplicate of bubbletea#79)"
- `MoAlshatti`, 2025-07-22: "any update on this?"
- `meowgorithm` [MEMBER], 2025-07-22: **"This is now available in Lip Gloss v2 Beta and can be used to draw modals in Bubble Tea. We've been using it in production for quite some time."** (links `lipgloss/releases/tag/v2.0.0-beta.2`)

The README's framing of #642 as an open question is out of date: it has been closed since 2023 and answered definitively in 2025.

### The real tracking issue: bubbletea#79

`repos/charmbracelet/bubbletea/issues/79` — "Node-based rendering", opened 2021-04-05, **still open** as of 2026-08-22, 8 comments.

- `meowgorithm` [MEMBER], 2023-01-23: "given that Bubblezone is a thing now, the solution here will most likely only need to be implemented in Lip Gloss."
- `meowgorithm` [MEMBER], 2025-07-22: **"Just a note that this is now in Lip Gloss v2 Beta. We're planning a tighter integration into Bubble Tea v2 in the near future, but this is something you can take advantage of in your Bubble Tea applications immediately."**

That "tighter integration" is **announced but not shipped** as of bubbletea v2.0.9 — see §5. There are no open Bubble Tea issues or PRs matching `layer`/`compositing`/`overlay` (`gh search issues/prs --repo charmbracelet/bubbletea --state open` → empty).

## 5. The `View() string` contract has changed — but not to a buffer

`Model.View()` now returns a **`tea.View` struct**, not a string (`bubbletea/tea.go:64`, `View() View`). But the content field is still a string:

```go
// tea.go:82-96
// View represents a terminal view that can be composed of multiple layers.
// It can also contain a cursor that will be rendered on top of the layers.
type View struct {
    Content string       // "styled strings ... with styles and hyperlinks encoded as ANSI escape codes"
    OnMouse func(msg MouseMsg) Cmd
    Cursor  *Cursor
    BackgroundColor, ForegroundColor color.Color
    WindowTitle string
    ProgressBar *ProgressBar
    AltScreen bool
    ReportFocus bool
    DisableBracketedPasteMode bool
    MouseMode MouseMode
    KeyboardEnhancements KeyboardEnhancements
}
func NewView(s string) View
func (v *View) SetContent(s string)
```

So the v2 change is **declarative terminal state + a cursor + a mouse hook**, not a drawable-returning view. Notably:
- The v2 `View` is genuinely cursor-aware (`Cursor *Cursor`), which removes one classic reason to hand-roll a renderer.
- `OnMouse func(MouseMsg) Cmd` is the intended pairing with `Compositor.Hit` — it lets a view close over the frame it just rendered and translate a click into a layer-ID message without breaking unidirectional flow.
- **Evidence the integration is planned but unlanded:** `tea.go:62` documents `View()` as returning something "which can be a string or a `[Layer]`" — a dangling godoc reference; there is no `Layer` type anywhere in package `tea` (`grep -rn '\[Layer\]\|tea.Layer' *.go` → that one comment only). `UPGRADE_GUIDE_V2.md` and the v2.0.0 release notes contain **zero** mentions of layer, canvas, compositing, overlay, or modal.
- Practical consequence for froth: at the Bubble Tea boundary the frame is serialized to an ANSI string and immediately re-parsed — `cursed_renderer.go:301` does `content := uv.NewStyledString(view.Content)` and draws it into its own `uv.ScreenBuffer`. Compositing above that line can be in cell space; the handoff is not. This costs one serialize/parse per frame and, more importantly, means anything not expressible in an ANSI string (per-cell metadata, transparency) cannot cross it.

## 6. Maintainer-sanctioned pattern for overlapping UI

The canonical example is in the Bubble Tea repo itself: `bubbletea/examples/clickable/main.go` at v2.0.9 — draggable, overlapping, click-to-raise dialog boxes. Its `View()`:

```go
root := lipgloss.NewLayer(bg).ID("bg")
for i, d := range m.dialogs {
    root.AddLayers(d.view().Z(i + 1))     // z from slice order; drag moves the dialog to the end
}
comp := lipgloss.NewCompositor(root)
v.MouseMode = tea.MouseModeAllMotion
v.AltScreen = true
v.OnMouse = func(msg tea.MouseMsg) tea.Cmd {
    return func() tea.Msg {
        mouse := msg.Mouse()
        if id := comp.Hit(mouse.X, mouse.Y).ID(); id != "" {
            return LayerHitMsg{ID: id, Mouse: msg}
        }
        return nil
    }
}
v.SetContent(comp.Render())
```

That is the whole sanctioned stack: build `Layer`s from rendered strings, stack with `Z`, `Compositor.Hit` for mouse routing, `Render()` into `View.Content`. Also see `lipgloss/examples/canvas/main.go` (nested layers, per-layer Z).

**Implication for froth.** The rendering half is largely solved for the common case; froth's differentiated value is the parts Lip Gloss deliberately does not do: surface lifecycle, focus and input routing beyond a point hit test, tiling/layout policy, per-surface clipping viewports, and z-index management as a first-class concept rather than a raw int. Where froth needs behavior `Layer` cannot express — transparency, clipping to a sub-rect, non-rectangular surfaces — the right move is to implement `uv.Drawable` on froth's own surface type and hand it to `Canvas.Compose`, rather than to build a buffer. `Canvas` exposes `CellAt`/`SetCell`/`Bounds`/`WidthMethod`, so a custom `Draw` gets full per-cell control over an existing, tested cell buffer. The cost of that coupling is a direct dependency on untagged `ultraviolet` (§1).

## Sources

| Source | Established |
|---|---|
| `git tag --sort=-creatordate` on fresh clones of charmbracelet/lipgloss and /bubbletea (2026-08-22) | Latest tags v2.0.6 and v2.0.9; full v2 tag history |
| `https://pkg.go.dev/charm.land/lipgloss/v2` | v2.0.6 latest, published Aug 11 2026; full exported API for Canvas/Layer/Compositor/LayerHit |
| lipgloss `v2.0.6` `canvas.go`, `layer.go` | Exact types, signatures, z-sort implementation, flatten semantics, Render() sizing |
| lipgloss `v2.0.6` `go.mod` | Module path `charm.land/lipgloss/v2`; ultraviolet pinned at `v0.0.0-20260811164956-006e29f97886` |
| bubbletea `v2.0.9` `tea.go` (lines 60-190, 279) | `View() View` contract; `View` struct fields; `SetContent`; dangling `[Layer]` doc reference |
| bubbletea `v2.0.9` `cursed_renderer.go:301`, `go.mod` | Frame is re-parsed from string via `uv.NewStyledString`; different ultraviolet pseudo-version |
| bubbletea `v2.0.9` `examples/clickable/main.go` | Maintainer-authored overlapping-window pattern with Compositor + Hit + OnMouse |
| lipgloss `v2.0.6` `examples/canvas/main.go` | Nested layers, per-layer X/Y/Z |
| ultraviolet @ `68fa937` `styled.go:50-61`, `buffer.go:296-302`, `uv.go:20-53`, `cell.go` | Area is cleared before draw (no transparency); out-of-bounds writes dropped; Drawable/Screen interfaces; EmptyCell |
| `github.com/charmbracelet/ultraviolet` clone, `git tag \| wc -l` → 0 | ultraviolet has no released tags |
| GitHub API `repos/charmbracelet/bubbletea/issues/642` + `/comments` | Closed 2023-01-23 as duplicate of #79; meowgorithm 2025-07-22 "available in Lip Gloss v2 Beta ... in production for quite some time" |
| GitHub API `repos/charmbracelet/bubbletea/issues/79` + `/comments` | Still open; meowgorithm 2025-07-22 "planning a tighter integration into Bubble Tea v2 in the near future" |
| GitHub API `releases/tags/v2.0.0` (both repos), `lipgloss/releases/tags/v2.0.0-beta.2` | Release dates; charm.land import paths; beta.2 compositing announcement (API since changed) |
| `gh search issues/prs --repo charmbracelet/bubbletea 'layer OR compositing OR overlay' --state open` | No open work tracking the Bubble Tea-side integration |
| Probe programs run against `charm.land/lipgloss/v2@v2.0.6`, Go 1.25.0 | Transparency: spaces punch holes. Clipping at canvas edge works. Negative coords lost + Render() mis-sizes. Wide runes correct. Later-added equal-z layer won (unstable sort — not guaranteed) |

Authority note: the two decisive maintainer statements are single GitHub comments by `meowgorithm` (Christian Rocha, Charm founder, MEMBER association) on issues #642 and #79. They are authoritative as to intent, but the "tighter integration into Bubble Tea v2 in the near future" claim is from 2025-07-22 and has not materialized in 13 months of releases through v2.0.9 — do not plan around it.

## Open / unverified

- **Whether the Bubble Tea-side layer integration is still planned.** No open issue, PR, or discussion tracks it; the only evidence is the 2025-07-22 comment and the dangling `[Layer]` godoc reference in `tea.go:62`. I did not search Discord or the charmbracelet Discussions tab. [UNVERIFIED]
- **Whether transparency support is on anyone's roadmap.** Not found in issues, PRs, or release notes for lipgloss v2.0.0 through v2.0.6. [UNVERIFIED]
- **Performance at froth's likely scale** (tens of surfaces, 60fps). `Compositor.Refresh()` is O(n log n) per tree change with a full re-walk, and every frame re-serializes to a string. Not benchmarked here. [UNVERIFIED]
- **ultraviolet's API stability commitments.** It is untagged v0.0.0 with no documented policy found; the fact that lipgloss and bubbletea pin different pseudo-versions of it suggests the API is still moving. [UNVERIFIED]
- **Bubbles v2 status and whether any component exposes Layers.** Not investigated; out of scope for this question.
- **Behavior of `Compositor.Draw` when the `area` argument is smaller than a layer's bounds.** Read from source (the argument is used only for an `Overlaps` test, then ignored in favor of `cl.bounds`), not confirmed by a probe.
