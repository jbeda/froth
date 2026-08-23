// Package froth is window and surface management for Bubble Tea. An application
// hands froth a root surface and forwards Init, Update and View to it; froth
// renders the surface tree through Lip Gloss's compositor and routes messages to
// the surfaces that should see them.
//
// A froth instance is not safe for concurrent use. Bubble Tea drives a program
// from one goroutine, and everything froth does happens on it: there is no lock
// anywhere, and none is wanted. Work a surface starts elsewhere comes back
// through a message rather than by touching froth from the goroutine that did
// it — which is what commands do today, and what producers will do in M5.
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
	// ctx is the root of the per-surface context tree rather than a request
	// scope, which is why it is held on the struct: it outlives every call
	// froth makes and every surface hangs its own cancellation off it.
	ctx  context.Context
	reg  registry
	root Handle
	size Size

	// focus is the one authoritative focus handle: the surface raw keys are
	// delivered to, and the deep end of the chain M2 resolves bindings along.
	// pendingFocus is a move asked for during Deliver, committed in the Apply
	// that follows it, and its own flag rather than a zero handle — a request
	// to focus nothing is a request froth has to reject out loud.
	focus        Handle
	pendingFocus Handle
	focusPending bool

	onDiagnostic func(Diagnostic)
	// flushing says the hook is being called from the stack below, so a hook
	// that drives froth queues its diagnostics for the pass already running
	// rather than starting a nested one.
	flushing bool
	// queue holds the messages waiting to be delivered. Delivery drains it and
	// may add to it, which is the whole point: a surface can post during Deliver
	// and have it handled before anything is drawn.
	queue []envelope
	diags []Diagnostic

	phase phase
	epoch uint64
}

// An envelope is a message and the surface it is addressed to. Bubble Tea's own
// messages arrive with no destination, so froth supplies one; a posted message
// carries the one its sender named.
type envelope struct {
	to  Handle
	msg tea.Msg
}

// New builds a froth instance around a root surface. ctx is the root of the
// per-surface context tree: work a surface starts is cancelled when the surface
// is destroyed, and all of it when ctx is. Options configure the instance for
// its lifetime — today OnDiagnostic, which is the one hook froth reports
// through.
//
// New panics if root is nil: an instance with nothing to render and nowhere to
// deliver has no defined behaviour to fall back on. A nil ctx is taken as
// context.Background, since a caller with no context to give means that one.
func New(ctx context.Context, root Surface, opts ...Option) *Froth {
	if ctx == nil {
		ctx = context.Background()
	}
	if root == nil {
		panic("froth: New called with a nil root surface")
	}
	var o options
	for _, opt := range opts {
		opt(&o)
	}
	f := &Froth{ctx: ctx, onDiagnostic: o.diagnostic}
	f.root = f.reg.add(Handle{}, root)
	// The root holds focus until something moves it, so an application that
	// never mentions focus still receives its keys somewhere.
	f.focus = f.root
	return f
}

// Root reports the handle of the root surface.
func (f *Froth) Root() Handle { return f.root }

// Init returns the work froth needs done before the first frame. Forward it
// from the application model's Init.
func (f *Froth) Init() tea.Cmd { return nil }

// Update takes one message and settles: it delivers, applies, and repeats until
// nothing is left to deliver, then returns the work the surfaces that saw the
// message asked for. Forward it from the application model's Update.
//
// Nothing is rendered here. A cascade of posted messages therefore resolves
// entirely before the frame is drawn, so no half-settled state ever reaches the
// screen and there is no skew between what a surface knows and what it drew.
//
// The terminal size is froth's own business: a resize updates the rect surfaces
// render into and is never delivered to a surface as a message.
func (f *Froth) Update(msg tea.Msg) tea.Cmd {
	if ws, ok := msg.(tea.WindowSizeMsg); ok {
		f.size = Size{W: ws.Width, H: ws.Height}
		return f.settle()
	}
	f.queue = append(f.queue, envelope{to: f.destination(msg), msg: msg})
	return f.settle()
}

// destination is where a message arriving from Bubble Tea goes. A key goes to
// the focused surface and to nothing else; everything else goes to the root.
//
// Raw delivery to the focused surface is the whole of M1's routing. Matching
// declared bindings along the focus chain first, and handing the winner an
// action rather than a key, is M2 — it sits in front of this step rather than
// replacing it, because ADR 0008's third step is exactly what happens here.
//
// The root takes the rest because froth has nothing better to say yet: mouse
// messages are hit-tested against the rendered frame in M4, and the remaining
// Bubble Tea messages are the application's own business, reaching it through
// the surface it handed froth.
func (f *Froth) destination(msg tea.Msg) Handle {
	if _, ok := msg.(tea.KeyMsg); ok {
		return f.focused()
	}
	return f.root
}

// A budget bounds one settle pass. Delivering a message and mounting a surface
// each spend from the same one, because a cascade that will not settle can be
// made of either and the two interleave: a delivery can create a surface whose
// mount posts a message.
type budget struct {
	left    int
	tripped bool
}

// iterationCap is how much one settle pass may spend before froth decides the
// cascade is never going to settle.
//
// The cap exists to turn a hang into a report, not to police enthusiasm, so it
// is set well above anything legitimate rather than close to it. Tripping late
// costs microseconds — an iteration is a map lookup and one surface's handler —
// while tripping early breaks an application that was merely large: controls are
// surfaces, so a screen assembling a few thousand of them in one mount pass is
// ordinary rather than pathological. A real cycle spends the whole budget in
// well under a millisecond, which is invisible next to a frame.
const iterationCap = 10000

// spend takes one unit, reporting whether there was one to take.
func (b *budget) spend() bool {
	if b.left <= 0 {
		return false
	}
	b.left--
	return true
}

// trip reports an exhausted budget, once per settle pass. A cascade that runs
// out during Apply would otherwise report again the moment the drain loop
// noticed the queue it abandoned.
func (f *Froth) trip(b *budget, detail string) {
	if b.tripped {
		return
	}
	b.tripped = true
	f.report(Diagnostic{Kind: DiagIterationCap, Detail: detail})
}

// settle runs Deliver and Apply alternately until the message queue drains.
//
// Apply leads because the tree has to exist before anything can be delivered
// into it — the root's own mount is the first Apply any instance runs — and
// because the Apply a delivery asks for is the one at the top of the next turn
// of the loop. Deliver hands one message to one surface, which may change only
// itself, and Apply commits the structural change that delivery asked for.
//
// Within Apply the order is fixed: intents first, then structural changes, so a
// surface that raises an intent and closes itself is still alive when the intent
// lands. Intents are M3; the order is established here so M3 adds a step rather
// than reordering the loop.
//
// Delivering a message and mounting a surface both spend from one budget, and
// running out of it means a cascade that is not going to settle. froth abandons
// what it was draining, reports it, and lets the program carry on rather than
// hanging. See iterationCap for why the budget is the size it is.
func (f *Froth) settle() tea.Cmd {
	b := budget{left: iterationCap}
	var cmds []tea.Cmd

	for {
		f.apply(&b)
		if len(f.queue) == 0 {
			break
		}
		if !b.spend() {
			f.trip(&b, "the message queue did not drain within "+
				strconv.Itoa(iterationCap)+" deliveries and mounts; a surface "+
				"is almost certainly posting a message that comes back to it")
			f.queue = nil
			break
		}
		e := f.queue[0]
		// Clear the slot as well as dropping it: the backing array outlives the
		// reslice, and a delivered message should not be kept alive by it.
		f.queue[0] = envelope{}
		f.queue = f.queue[1:]
		if cmd := f.deliver(e.to, e.msg); cmd != nil {
			cmds = append(cmds, cmd)
		}
	}

	f.flush()
	if len(cmds) == 0 {
		return nil
	}
	return tea.Batch(cmds...)
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

	uc := &UpdateCtx{phaseToken: f.enter(phaseDeliver), ctx: f.ctx, self: h}
	defer f.leave()

	n.surface.Update(uc, msg)
	return tea.Batch(uc.cmds...)
}

// apply runs the Apply phase: everything structural that has been asked for
// since the last one.
//
// The order within it is fixed — intents first, then structural changes, so a
// surface that raises an intent and closes itself is still alive when the intent
// lands. Intents are M3, which adds its step ahead of these two.
//
// Focus commits before mounting, so a surface that mounts a child and focuses it
// there wins over a request made in the delivery before — the later of the two,
// and so the one that should stand.
func (f *Froth) apply(b *budget) {
	if len(f.reg.entered) == 0 && !f.focusPending {
		return
	}

	token := f.enter(phaseApply)
	defer f.leave()

	f.applyFocus()
	f.mountEntered(b, token)
}

// mountEntered mounts the surfaces which have entered the tree and not been
// mounted yet — the root to begin with, and then whatever the tree assembles for
// itself — before froth renders or delivers anything.
//
// It drains a queue rather than recursing over the tree, so a surface created
// late in the pass is mounted in that same pass whoever created it. Recursion
// would have missed one: an ApplyCtx stays valid for the whole of the Apply that
// issued it, so a surface can hand its own context to a sibling, and the child
// that appears under a node already walked past would have rendered without ever
// mounting.
func (f *Froth) mountEntered(b *budget, token phaseToken) {
	for len(f.reg.entered) > 0 {
		if !b.spend() {
			f.trip(b, "mounting did not settle within "+strconv.Itoa(iterationCap)+
				" deliveries and mounts; a surface is almost certainly mounting "+
				"a child of its own type")
			f.reg.entered = nil
			return
		}

		h := f.reg.entered[0]
		f.reg.entered = f.reg.entered[1:]

		// A surface that entered the tree and left it again before it was
		// mounted has nothing to mount. Unreachable until M3 can destroy one,
		// and cheaper to allow for than to discover as a nil dereference then.
		n := f.reg.get(h)
		if n == nil {
			continue
		}
		if m, ok := n.surface.(Mounter); ok {
			m.Mount(&ApplyCtx{phaseToken: token, self: h})
		}
	}
}

// applyPending settles the structure before a frame is drawn, for the frames
// that do not follow an Update: the first one, and every Composite a test takes
// of its own accord.
func (f *Froth) applyPending() {
	b := budget{left: iterationCap}
	f.apply(&b)
	f.flush()
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
// the first terminal size arrives, since nothing has been drawn yet, and so
// does one whose surface a layout gave a rect with no cells in it.
//
// The whole tree renders either way: rects resolve from the root down, and Z
// ordinals are global, so a subtree cannot be drawn without placing everything
// around it. This is froth's own render path — View is Composite of the root —
// rather than a second one that could drift from it.
func (f *Froth) Composite(h Handle) Frame {
	f.applyPending()

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
	// A handle that names nothing draws nothing, and its children are not
	// reachable through it either. Unreachable until M3 can destroy a surface;
	// the alternative there is a nil dereference in the render path.
	n := f.reg.get(h)
	if n == nil {
		return
	}
	rc := &RenderCtx{phaseToken: token, self: h, rect: r}
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
