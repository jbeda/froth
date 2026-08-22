// Package froth is window and surface management for Bubble Tea. An application
// hands froth a root surface and forwards Init, Update and View to it; froth
// renders the surface tree through Lip Gloss's compositor and routes messages to
// the surfaces that should see them.
package froth

import (
	"context"
	"strconv"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// Froth owns a surface tree and drives it. An application holds one by pointer
// and forwards Bubble Tea's three model methods to it, so copying the
// application model by value — which Bubble Tea does on every update — copies a
// pointer rather than the tree.
type Froth struct {
	ctx  context.Context
	reg  registry
	root Handle
	size Size

	phase phase
	epoch uint64
}

// New builds a froth instance around a root surface. ctx is the root of the
// per-surface context tree: work a surface starts is cancelled when the surface
// is destroyed, and all of it when ctx is.
func New(ctx context.Context, root Surface) *Froth {
	if ctx == nil {
		ctx = context.Background()
	}
	if root == nil {
		panic("froth: New called with a nil root surface")
	}
	f := &Froth{ctx: ctx}
	f.root = f.reg.add(Handle{}, root)
	return f
}

// Root reports the handle of the root surface.
func (f *Froth) Root() Handle { return f.root }

// Init returns the work froth needs done before the first frame. Forward it
// from the application model's Init.
func (f *Froth) Init() tea.Cmd { return nil }

// Update delivers one message and returns the work the surfaces that saw it
// asked for. Forward it from the application model's Update.
//
// The terminal size is froth's own business: a resize updates the rect surfaces
// render into and is never delivered to a surface as a message.
func (f *Froth) Update(msg tea.Msg) tea.Cmd {
	f.ready()
	if ws, ok := msg.(tea.WindowSizeMsg); ok {
		f.size = Size{W: ws.Width, H: ws.Height}
		return nil
	}
	return f.deliver(f.root, msg)
}

// View composites the surface tree into a frame. Forward it from the
// application model's View.
func (f *Froth) View() tea.View {
	return tea.NewView(f.Composite(f.root).Content)
}

// deliver runs the Deliver phase for one message and one surface. A handle that
// names nothing is a defined drop rather than an error, which is the point of
// addressing surfaces by handle.
func (f *Froth) deliver(h Handle, msg tea.Msg) tea.Cmd {
	n := f.reg.get(h)
	if n == nil {
		return nil
	}

	uc := &UpdateCtx{phaseToken: f.enter(phaseDeliver), ctx: f.ctx}
	defer f.leave()

	n.surface.Update(uc, msg)
	return tea.Batch(uc.cmds...)
}

// ready mounts everything that has entered the tree and not been mounted yet —
// the root to begin with, and then whatever the tree assembles for itself —
// before froth renders or delivers anything.
//
// It drains a queue rather than recursing over the tree, so a surface created
// late in the pass is mounted in that same pass whoever created it. Recursion
// would have missed one: an ApplyCtx stays valid for the whole pass, so a
// surface can hand its own context to a sibling, and the child that appears
// under a node already walked past would have rendered without ever mounting.
func (f *Froth) ready() {
	if len(f.reg.entered) == 0 {
		return
	}

	token := f.enter(phaseApply)
	defer f.leave()

	for budget := mountBudget; len(f.reg.entered) > 0; budget-- {
		if budget <= 0 {
			panic("froth: mounting created more than " + strconv.Itoa(mountBudget) +
				" surfaces; a surface is almost certainly mounting a child of its own type")
		}

		h := f.reg.entered[0]
		f.reg.entered = f.reg.entered[1:]
		if m, ok := f.reg.get(h).surface.(Mounter); ok {
			m.Mount(&ApplyCtx{phaseToken: token, self: h})
		}
	}
}

// mountBudget bounds one mount pass. A surface that mounts a child of its own
// type never settles; the drain loop gains a shared cap and a diagnostic to
// report it through, and this stands in until then.
const mountBudget = 10000

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
// the first terminal size arrives, since nothing has been drawn yet, and so
// does one whose surface a layout gave a rect with no cells in it.
//
// The whole tree renders either way: rects resolve from the root down, and Z
// ordinals are global, so a subtree cannot be drawn without placing everything
// around it. This is froth's own render path — View is Composite of the root —
// rather than a second one that could drift from it.
func (f *Froth) Composite(h Handle) Frame {
	f.ready()

	screen := Rect{W: f.size.W, H: f.size.H}
	if f.reg.get(h) == nil || screen.Empty() {
		return Frame{}
	}

	// A subtree is contiguous in depth-first order, so it is a slice of the
	// placements rather than a second walk. The root's subtree is all of them,
	// which is the frame every ordinary View asks for.
	sub := f.place(screen)
	if h != f.root {
		start := 0
		for start < len(sub) && sub[start].h != h {
			start++
		}
		if start == len(sub) {
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
// and how far up the stack.
type placement struct {
	h       Handle
	rect    Rect
	content string
	z       int
}

// place renders every surface, clips it to its rect, and assigns each one a Z
// ordinal. Even with one surface the frame goes through Lip Gloss's canvas and
// compositor rather than being assembled as a string, so wide graphemes and Z
// ordering are the compositor's job from the first frame onwards. Clipping is
// only partly the compositor's: Lip Gloss drops what falls outside the canvas,
// but a layer's own bounds come from the size of the string it was given rather
// than from the rect the surface was allotted, so clipping a surface to its
// rect is froth's job.
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
	f.placeSubtree(token, f.root, screen, &out)
	return out
}

func (f *Froth) placeSubtree(token phaseToken, h Handle, r Rect, out *[]placement) {
	n := f.reg.get(h)
	rc := &RenderCtx{phaseToken: token, rect: r}
	content := clip(n.surface.Render(rc, r), r.Size())
	*out = append(*out, placement{h: h, rect: r, content: content, z: len(*out) + 1})

	// A container with no layout stacks its children at its own whole rect,
	// which is what a surface that never calls SetLayout gets, and which needs
	// no rect slice to say.
	if n.layout == nil {
		for _, c := range n.children {
			f.placeSubtree(token, c, r, out)
		}
		return
	}

	rects := f.arrange(n, r)
	for i, c := range n.children {
		f.placeSubtree(token, c, rects[i], out)
	}
}

// arrange asks a container's layout where its children go. It is only called
// for a container that has one; placeSubtree handles the layout-less case,
// where every child takes the container's whole rect.
//
// A layout returning the wrong number of rects is a bug in the layout: a child
// with no rect has nowhere to render, and a rect with no child is an answer
// about a surface that does not exist. Neither has a sane interpretation, so
// froth says so rather than guessing.
func (f *Froth) arrange(n *node, r Rect) []Rect {
	children := make([]Child, len(n.children))
	for i, c := range n.children {
		children[i] = Child{Handle: c}
	}

	rects := n.layout.Arrange(r, children)
	if len(rects) != len(children) {
		panic("froth: a layout returned " + strconv.Itoa(len(rects)) +
			" rects for " + strconv.Itoa(len(children)) +
			" children; a layout must return exactly one rect per child, in order")
	}
	return rects
}
