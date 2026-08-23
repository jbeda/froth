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
	// ctx is the context every surface's asynchronous work runs under. It
	// outlives every call froth makes, which is why it is held here rather
	// than passed as a request scope.
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

// New builds a froth instance around a root surface. ctx is the context every
// surface's asynchronous work runs under; cancelling it cancels all of it.
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
	if e, ok := f.route(msg); ok {
		f.queue = append(f.queue, e)
	}
	return f.settle()
}
