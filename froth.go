// Package froth is window and surface management for Bubble Tea. An application
// hands froth a root surface and forwards Init, Update and View to it; froth
// renders the surface tree through Lip Gloss's compositor and routes messages to
// the surfaces that should see them.
//
// A froth instance is not safe for concurrent use. Bubble Tea drives a program
// from one goroutine and everything froth does happens on it, so there is no
// lock anywhere. Work a surface starts elsewhere must come back through a
// message rather than by touching froth from the goroutine that did it.
package froth

import (
	"context"

	tea "charm.land/bubbletea/v2"
)

// Froth owns a surface tree and drives it. An application holds one by pointer
// and forwards Bubble Tea's three model methods to it, so copying the
// application model by value — which Bubble Tea does on every update — copies a
// pointer rather than the tree.
type Froth struct {
	// ctx is the parent every surface's context is derived from, so cancelling
	// it cancels all of them. It outlives every call froth makes, which is why
	// it is held here rather than passed as a request scope.
	ctx  context.Context
	reg  registry
	root Handle
	size Size

	// focus is the one authoritative focus handle: the surface raw keys are
	// delivered to. pendingFocus is a move asked for during Deliver and
	// committed in the Apply that follows; it carries its own flag rather than
	// using a zero handle, because a request to focus nothing has to be
	// rejected out loud rather than read as no request at all.
	focus        Handle
	pendingFocus Handle
	focusPending bool

	// renderedFrame is where every surface was when the frame was last drawn,
	// in paint order. It is the only geometry outside Render, and it can name
	// surfaces destroyed since. See ADR 0013.
	renderedFrame []placement

	// capture is the surface every mouse message goes to while it is set,
	// wherever the message landed. Zero when nothing holds it.
	capture Handle

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

	// pendingReconcile holds the surfaces waiting for Reconcile, in the order
	// they asked. Each one carries a bit saying it is here, which is what makes
	// asking twice ask once.
	pendingReconcile []Handle

	// pendingDestroy holds the surfaces asked for during Deliver or Render and
	// removed at the next Apply. shutdown says the instance has been torn down,
	// after which posting is a reported drop and nothing else happens at all.
	pendingDestroy []Handle

	// openedModals holds the modals created during this Apply, so focus can be
	// moved into each once the mount drain has given it something to land on.
	openedModals []Handle
	shutdown     bool

	phase phase
	epoch uint64
}

// New builds a froth instance around a root surface. ctx is the parent every
// surface's context is derived from; cancelling it cancels all of them.
// Options configure the instance for its lifetime.
//
// New panics if root is nil: an instance with nothing to render and nowhere to
// deliver has no defined behaviour to fall back on. A nil ctx is taken as
// context.Background.
func New(ctx context.Context, root Surface, opts ...Option) *Froth {
	if ctx == nil {
		ctx = context.Background()
	}
	if root == nil {
		bug("froth: New called with a nil root surface")
	}
	var o options
	for _, opt := range opts {
		opt(&o)
	}
	f := &Froth{ctx: ctx, onDiagnostic: o.diagnostic}
	f.root = f.reg.add(Handle{}, root)
	// New is the root's entry into the tree, so it is where the root's type is
	// asked what it is. A panic here is left to escape: nothing is in progress
	// to unwind and there is no tree to keep consistent, so a surface that
	// cannot say what it is fails at construction rather than quietly.
	if d, ok := root.(Declarer); ok {
		f.reg.get(f.root).meta.flags = d.Declare()
	}
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
// message asked for. Forward it from the application model's Update. After
// Shutdown it does nothing.
//
// Nothing is rendered here. A cascade of posted messages therefore resolves
// entirely before the frame is drawn, so no half-settled state ever reaches the
// screen and there is no skew between what a surface knows and what it drew.
//
// The terminal size is froth's own business: a resize updates the rect surfaces
// render into and is never delivered to a surface as a message.
func (f *Froth) Update(msg tea.Msg) tea.Cmd {
	if f.shutdown {
		return nil
	}
	if ws, ok := msg.(tea.WindowSizeMsg); ok {
		f.size = Size{W: ws.Width, H: ws.Height}
		return f.settle()
	}
	// A message is addressed against the tree, so the tree has to exist first.
	// Routing resolves a key press against the declarations on the focus chain
	// and reads where focus is, and on a fresh instance neither has been
	// established: the root is registered but not mounted, so it has declared
	// nothing and has not had its chance to move focus. Every later message
	// finds this a no-op, because settle always ends on an Apply.
	//
	// One budget covers the Apply and the settle that follows, so a cascade
	// that starts in a mount and continues through delivery is bounded once.
	b := budget{left: iterationCap}
	f.apply(&b)

	if e, ok := f.route(msg); ok {
		f.queue = append(f.queue, e)
	}
	return f.settleWith(&b)
}
