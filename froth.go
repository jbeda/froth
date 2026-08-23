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

	tea "charm.land/bubbletea/v2"
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
