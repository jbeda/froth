// Render: the walk that turns a tree of surfaces into a frame.
//
// This is the only file that knows which compositor froth drives. Everything
// above it — the depth-first walk, the rects, the clip, the Z ordinals — is
// froth's own, so substituting the back-end named in the README is a change
// confined to here. See ADR 0007 for why the ordinals are froth's.

package froth

import (
	"slices"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// View composites the surface tree into a frame. Forward it from the
// application model's View.
//
// It is Composite of the root and shares its behaviour, including that pending
// structure is applied before anything is drawn.
func (f *Froth) View() tea.View {
	return tea.NewView(f.Composite(f.root).Content)
}

// A Frame is what a subtree drew: the composited output for the subtree alone,
// and the rect each surface in it resolved to. The rects are screen
// coordinates, so "is the button where I think it is" is answerable even when
// the string is a small window onto the frame.
type Frame struct {
	Content string
	Rects   map[Handle]Rect
}

// Composite renders the tree and returns the frame for the subtree rooted at h.
// A handle that names nothing returns the zero Frame; so does any handle before
// the first terminal size arrives, and so does one whose surface a layout gave a
// rect with no cells in it.
//
// The whole tree renders either way: rects resolve from the root down and Z
// ordinals are global, so a subtree cannot be drawn without placing everything
// around it.
//
// Compositing is not read-only, and a caller has to know it: structure still
// waiting to be applied is applied first, so Composite can run Mount — which is
// application code assembling a subtree of its own — and hand the diagnostics
// that produces to the hook before it returns. An instance composited before it
// is ever updated mounts its whole tree here.
func (f *Froth) Composite(h Handle) Frame {
	f.applyPending()
	// Drawing reports too — a dead handle minted from a RenderCtx, a query
	// asked from inside one, a surface that panicked — and those are found
	// after applyPending has already flushed. Without this they would wait for
	// whatever drove froth next.
	defer f.flush()

	screen := Rect{W: f.size.W, H: f.size.H}
	if f.reg.get(h) == nil || screen.Empty() {
		return Frame{}
	}

	// A subtree is contiguous in depth-first order, so it is a slice of the
	// placements rather than a second walk. The root's subtree is all of them,
	// which is the frame every ordinary View asks for.
	sub := f.place(screen)
	if h != f.root {
		start := slices.IndexFunc(sub, func(p placement) bool { return p.h == h })
		if start < 0 {
			return Frame{} // registered but not in the tree: nothing was drawn
		}
		end := start + 1
		for end < len(sub) && f.reg.descends(sub[end].h, h) {
			end++
		}
		sub = sub[start:end]
	}

	// A subtree whose root has no cells drew nothing, so it answers the same
	// zero Frame as a handle that names nothing. A layout can hand a child an
	// empty rect — a row starves the children it cannot fit — and a buggy one
	// can hand it a negative extent; either would size the canvas below, where
	// a negative one is a makeslice panic from inside the compositor.
	origin := sub[0].rect
	if origin.Empty() {
		return Frame{}
	}

	rects := make(map[Handle]Rect, len(sub))
	layers := make([]*lipgloss.Layer, 0, len(sub))
	for _, p := range sub {
		rects[p.h] = p.rect
		layers = append(layers, lipgloss.NewLayer(p.content).
			ID(p.h.String()).
			X(p.rect.X-origin.X).
			Y(p.rect.Y-origin.Y).
			Z(p.z))
	}

	canvas := lipgloss.NewCanvas(origin.W, origin.H)
	canvas.Compose(lipgloss.NewCompositor(layers...))
	return Frame{Content: canvas.Render(), Rects: rects}
}

// A placement is one surface's contribution to a frame: what it drew, where,
// and how far up the stack. drawn is the size of the rectangle its output makes,
// which is never larger than rect and is smaller for a surface that underfills
// what it was offered. See ADR 0018.
type placement struct {
	h       Handle
	rect    Rect
	drawn   Size
	content string
	z       int
}

// place renders every surface, clips it to its rect, and assigns each one a Z
// ordinal. Even a single surface goes through Lip Gloss's canvas and
// compositor rather than being assembled as a string, so wide graphemes and Z
// ordering are the compositor's job from the first frame onwards.
//
// Ordinals are unique, global and assigned depth-first, because Lip Gloss
// accumulates position through the layer tree but sorts Z across the whole tree
// with an unstable sort. Depth-first means a subtree occupies a contiguous
// range, which is what makes a surface's Z order it among its siblings and
// never against a surface under another parent. They start at 1: a compositor
// wraps its layers in an implicit root at Z 0, and a tie there would be exactly
// the undefined order the ordinals exist to avoid.
func (f *Froth) place(screen Rect) []placement {
	token := f.enter(phaseRender)
	defer f.leave()

	var out []placement
	f.placeSubtree(token, f.root, screen, screen, &out)
	f.renderedFrame = out
	return out
}

// placeSubtree draws h in r and then its children below it. A float is handed
// the whole screen as r and takes the rectangle its output makes, centred: its
// rect is not known until it has drawn.
func (f *Froth) placeSubtree(token phaseToken, h Handle, r, screen Rect, out *[]placement) {
	// A handle that names nothing draws nothing, and its children are not
	// reachable through it either.
	n := f.reg.get(h)
	if n == nil {
		return
	}
	// Quarantined earlier in this same frame: a surface drawn before this one
	// can ask a query, and a query walks the focus chain through surfaces that
	// have not been drawn yet.
	if n.panicked {
		return
	}
	content, drawn, ok := f.callRender(token, n, h, r)
	if !ok {
		// It drew nothing and its subtree is not reachable through it. The
		// destroy quarantine recorded commits at the next Apply; Render cannot
		// change structure, so this frame is drawn from the tree as it stands
		// with a hole where the surface was.
		return
	}
	if n.meta.float {
		r = Rect{X: screen.X + (screen.W-drawn.W)/2, Y: screen.Y + (screen.H-drawn.H)/2, W: drawn.W, H: drawn.H}
	}
	*out = append(*out, placement{h: h, rect: r, drawn: drawn, content: content, z: len(*out) + 1})

	// A container with no layout stacks its children at its own whole rect,
	// which is what a surface that never calls SetLayout gets, and which needs
	// no rect slice to say. Floats are placed in sibling order alongside the
	// arranged children, because stacking is depth-first and a float stacks as
	// any child does.
	arranged := f.reg.childrenOf(h, false)
	var rects []Rect
	if n.layout != nil {
		rects = f.arrange(n, r, arranged)
	}
	i := 0
	for _, c := range n.children {
		switch {
		case f.reg.get(c).meta.float:
			f.placeSubtree(token, c, screen, screen, out)
		case n.layout == nil:
			f.placeSubtree(token, c, r, screen, out)
		default:
			f.placeSubtree(token, c, rects[i], screen, out)
			i++
		}
	}
}

// callRender draws one surface under quarantine, reporting whether it returned.
func (f *Froth) callRender(token phaseToken, n *node, h Handle, r Rect) (content string, drawn Size, ok bool) {
	defer f.quarantine(h, "Render")
	content, drawn = clip(n.surface.Render(&RenderCtx{phaseToken: token, self: h}, r.Size()), r.Size())
	return content, drawn, true
}

// arrange asks a container's layout where the children it arranges go — every
// child but the floats. It is only called for a container that has a layout;
// placeSubtree handles the layout-less case, where every child takes the
// container's whole rect.
//
// A layout returning the wrong number of rects is a bug in the layout: a child
// with no rect has nowhere to render, and a rect with no child is an answer
// about a surface that does not exist. Neither has a sane interpretation, so
// froth says so rather than guessing.
func (f *Froth) arrange(n *node, r Rect, arranged []Handle) []Rect {
	children := make([]Child, len(arranged))
	for i, c := range arranged {
		children[i] = Child{Handle: c}
	}

	rects := n.layout.Arrange(r, children)
	if len(rects) != len(children) {
		bug("froth: a layout returned %d rects for %d children; "+
			"a layout must return exactly one rect per child, in order",
			len(rects), len(children))
	}
	return rects
}
