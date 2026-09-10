// The handle-scoped contexts: what a surface may do to a surface it names.
//
// A Handle confers nothing. Acting on one means asking the phase context you
// are already holding for a context scoped to that handle, and the type you get
// back says what may be done. The parentage the ownership rule turns on is
// checked once, here, when the context is minted; every method downstream of
// that is unconditionally safe. See ADR 0014.

package froth

import (
	"fmt"
	"slices"

	tea "charm.land/bubbletea/v2"
)

// A HandleReader is what every handle-scoped context can answer about the
// surface it names. Reads are on every tier, for any live handle, in every
// phase: a read changes nothing, so there is nothing for a tier to withhold.
//
// The interface is sealed — froth defines the whole set of types that satisfy
// it, so a method added to it later cannot break an implementation elsewhere.
// The same reasoning the standard library states at testing.TB.
type HandleReader interface {
	// Handle reports the handle this context was minted for.
	Handle() Handle

	// Parent reports the surface this one hangs off. The root and a surface
	// that has left the tree both answer the zero Handle.
	Parent() Handle

	// Children reports this surface's children in sibling order, which is
	// creation order and therefore stacking order. A surface with none, and a
	// handle that names no live surface, both answer nothing.
	Children() []Handle

	// Rect reports this surface's rect in the rendered frame — the last frame
	// drawn, which is what a mouse message was resolved against — and false
	// for a surface that was not in it. It is the surface rect, the one a
	// Frame reports; a click is hit-tested against the drawn rect, which is
	// smaller only for a surface that underfills what it was offered.
	Rect() (Rect, bool)

	sealed()
}

// reader is the read set every handle-scoped context carries, and the seal
// every one of them inherits. It is embedded rather than restated per tier
// because the tiers genuinely agree about reads.
type reader struct {
	phaseToken
	h Handle
}

func (r reader) sealed() {}

func (r reader) Handle() Handle {
	r.check()
	return r.h
}

func (r reader) Parent() Handle {
	r.check()
	n := r.f.reg.get(r.h)
	if n == nil {
		return Handle{}
	}
	return n.parent
}

func (r reader) Children() []Handle {
	r.check()
	n := r.f.reg.get(r.h)
	if n == nil {
		return nil
	}
	return slices.Clone(n.children)
}

func (r reader) Rect() (Rect, bool) {
	r.check()
	return r.f.renderedRect(r.h)
}

// router is the reach set: naming a surface without restructuring it. Every
// tier above the render one carries it, because a surface that may name a
// handle at all may post to it.
type router struct{ reader }

// Post addresses a message to this surface. It is queued rather than delivered:
// the current delivery finishes, and the message is handled later in the same
// update, before anything is rendered. Nothing in froth is delivered
// re-entrantly, so a surface is never inside two of its own handlers.
func (r router) Post(msg tea.Msg) {
	r.check()
	if r.f.shutdown {
		r.f.report(Diagnostic{
			Kind: DiagPostAfterShutdown,
			Detail: fmt.Sprintf("a %T was posted to %s during shutdown; "+
				"nothing will drain the queue, so it was dropped", msg, r.h),
		})
		return
	}
	r.f.queue = append(r.f.queue, envelope{to: r.h, msg: msg})
}

// Focus moves focus to this surface.
//
// During Deliver the move is recorded and commits in the Apply that follows, so
// a key that moves focus and the frame that shows the move are still one
// update. During Apply — which is where structural change commits — it happens
// at once.
//
// A surface created with FocusScope is entered rather than focused, unless it
// admits focus itself: the move lands on the child the scope last had focus
// on, or on its first tab stop.
func (r router) Focus() {
	r.check()
	if r.f.reg.get(r.h) == nil {
		return
	}
	if r.phase == phaseApply {
		r.f.setFocus(r.h)
		return
	}
	r.f.pendingFocus, r.f.focusPending = r.h, true
}

func (r router) capture() {
	r.check()
	r.f.takeCapture(r.h)
}

func (r router) releaseCapture() {
	r.check()
	r.f.releaseCapture(r.h)
}

func (r router) setFlags(fl Flags) {
	r.check()
	n := r.f.reg.get(r.h)
	if n == nil {
		return
	}
	n.meta.flags = fl
}

func (r router) getFlags() Flags {
	r.check()
	n := r.f.reg.get(r.h)
	if n == nil {
		return Flags{}
	}
	return n.meta.flags
}

// A ReadCtx is what Render mints, for a surface's own handle, one of its
// children, or a stranger alike. Render changes nothing, so the three tiers
// permit identical operations and collapse to one type.
type ReadCtx struct{ reader }

// A SelfUpdateCtx is what a surface holds on itself while handling a message.
// Structural change is Apply's, so what is here is what a surface can do to
// itself without restructuring anything.
type SelfUpdateCtx struct{ router }

// Destroy takes this surface out of the tree, along with everything below it.
// It is recorded and commits at the next Apply, so the surface stays live for
// the rest of this update: messages already queued to it are still delivered,
// and a rising walk passing through it still completes.
func (sc SelfUpdateCtx) Destroy() { sc.destroy() }

// SetBindings declares the shortcuts this surface answers to, replacing
// whatever it declared before. It takes effect at once rather than at the next
// Apply: the key press being handled has already resolved, so there is nothing
// for a delay to protect.
//
// See SelfApplyCtx.SetBindings for the rest.
func (sc SelfUpdateCtx) SetBindings(bs []Binding) { sc.setBindings(bs) }

// SetFlags replaces what this surface declared about itself, whole set at a
// time: a field left zero is off, whatever it was before. Read the current set
// with GetFlags to change one and leave the rest.
//
// It reaches only the surface's own flags. What its creator said about it —
// FocusScope, MouseTransparent — is not here and cannot be clobbered from here.
func (sc SelfUpdateCtx) SetFlags(fl Flags) { sc.setFlags(fl) }

// GetFlags reports what this surface last declared about itself, which is its
// type's declaration until it replaces it.
func (sc SelfUpdateCtx) GetFlags() Flags { return sc.getFlags() }

// Reconcile asks froth to call this surface's Reconcile during the next Apply.
// Asking more than once before froth gets there reconciles once, so a handler
// that changes several things need not track whether it has already asked.
//
// A surface that does not implement Reconciler reports a diagnostic and queues
// nothing.
func (sc SelfUpdateCtx) Reconcile() { sc.requestReconcile() }

// Capture claims every mouse message for this surface, wherever it lands, until
// ReleaseCapture, until the surface leaves the tree, or until another surface
// takes it. It takes effect at once, so a press that captures sees the release
// that follows it.
//
// At most one surface holds capture. Taking it from another surface posts that
// surface CaptureLostMsg; so does a modal opening over the holder. A surface an
// open modal has taken input away from is refused with a diagnostic.
func (sc SelfUpdateCtx) Capture() { sc.capture() }

// ReleaseCapture lets go of capture. A surface that does not hold it releases
// nothing and hears nothing, so releasing on every button-up is safe whether or
// not something took it away in between.
func (sc SelfUpdateCtx) ReleaseCapture() { sc.releaseCapture() }

// A SelfApplyCtx is what a surface holds on itself during Apply, and the only
// place structure a surface owns is assembled. Create and SetLayout name no
// target, so creating a grandchild or arranging somebody else's children is not
// expressible rather than refused.
type SelfApplyCtx struct{ router }

// A ChildCtx names an immediate child of the surface that minted it. The
// parentage was checked at the mint, so nothing here rechecks it.
type ChildCtx struct{ router }

// Destroy takes this child out of the tree, along with everything below it. A
// deep branch goes by destroying your own child and letting the cascade reach
// the rest — which is the ownership rule holding rather than a limitation,
// since the surfaces down there are not yours to name.
//
// It takes effect at once when called during Apply, and is recorded and commits
// at the next Apply when called during Deliver.
func (cc ChildCtx) Destroy() { cc.destroy() }

// SetMouseTransparent takes this child, and everything below it, out of the
// walk that finds what a click belongs to, or puts it back. It is the creator's
// to say, which is why it is here and not on the child's own context.
func (cc ChildCtx) SetMouseTransparent(v bool) {
	cc.check()
	n := cc.f.reg.get(cc.h)
	if n == nil {
		return
	}
	n.meta.mouseTransparent = v
}

// An AnyCtx names any live surface, whoever owns it. It carries what can be
// done to a stranger — reaching it, not restructuring it — which is why
// Query.Focus handing out a handle to a surface the caller never created is
// safe: there is no type that could destroy one.
type AnyCtx struct{ router }

// Create adds a child of this surface and returns its handle. It takes no
// parent: a surface creates a child of itself, so "own yourself and your
// immediate children" is structural rather than validated.
//
// The options say how the child sits in the composition and are recorded
// against its handle: creation is the only moment a surface's creator gets to
// describe it, and only MouseTransparent can be changed afterwards. What the
// child is rather than where it sits comes from its own type, through Declarer,
// which froth asks here.
//
// The child is mounted before the frame it was created for is drawn.
//
// Create panics if s is nil: the handle would be indistinguishable from one
// whose surface has died.
func (sc SelfApplyCtx) Create(s Surface, opts ...CreateOption) Handle {
	sc.check()
	h := sc.f.reg.add(sc.h, s, opts...)
	sc.f.entered(h, s)
	return h
}

// OpenModal adds a modal parented to the root and opened by this surface, and
// returns its handle. The root decides what it blocks — everything — and this
// surface decides how long it lives: the modal leaves the tree when this
// surface does, by any route. See ADR 0006.
//
// Modal is implied. Everything else, Float and FocusScope included, is the
// caller's to say, exactly as on Create. The handle comes back with read and
// post: only the root's child tier can destroy what the root parents.
//
// The modal takes focus once it has mounted, unless its Mount already put
// focus inside it, and gives focus back to where it was when it closes.
func (sc SelfApplyCtx) OpenModal(s Surface, opts ...CreateOption) Handle {
	sc.check()
	h := sc.f.reg.add(sc.f.root, s, slices.Concat(opts, []CreateOption{Modal()})...)
	sc.f.reg.get(h).opener = sc.h
	sc.f.entered(h, s)
	return h
}

// entered is what every create does once the surface is in the registry:
// asks it what it is, and — for a modal — saves where focus is, releases a
// capture it now blocks, and queues the focus move into it for the end of
// this Apply, when its children exist to land on. A surface that panicked
// answering has already left by quarantine and is nothing to a modal's
// bookkeeping.
func (f *Froth) entered(h Handle, s Surface) {
	f.declareFlags(h, s)
	n := f.reg.get(h)
	if n == nil || !n.meta.modal {
		return
	}
	n.restoreFocus = f.focus
	f.openedModals = append(f.openedModals, h)
	f.modalOpened()
}

// declareFlags asks a Declarer what its surface is and stores the answer, here
// rather than later so that a child is never in the tree with flags it did not
// choose. A surface that panics answering leaves by the ordinary quarantine
// route with every flag off, which is what it would have had anyway.
func (f *Froth) declareFlags(h Handle, s Surface) {
	d, ok := s.(Declarer)
	if !ok {
		return
	}
	defer f.quarantine(h, "Declare")
	f.reg.get(h).meta.flags = d.Declare()
}

// Destroy takes this surface out of the tree, along with everything below it,
// at once: Apply is where structural change commits, so there is nothing to
// record.
func (sc SelfApplyCtx) Destroy() { sc.destroy() }

// Reconcile asks froth to call this surface's Reconcile. Unlike everything else
// on this context it is a request rather than an immediate effect: the call
// comes later in this same Apply, once whatever froth is doing now has
// finished. That is what lets a surface ask from inside its own Reconcile
// without recursing into it.
//
// See SelfUpdateCtx.Reconcile for the rest.
func (sc SelfApplyCtx) Reconcile() { sc.requestReconcile() }

// SetLayout attaches a layout to this surface, which places its children from
// then on. A surface with no layout stacks its children at its own whole rect.
func (sc SelfApplyCtx) SetLayout(l Layout) {
	sc.check()
	sc.f.reg.get(sc.h).layout = l
}

// SetBindings declares the shortcuts this surface answers to, replacing
// whatever it declared before. froth stores them, so resolution and both
// binding queries read froth's own state and never call back into a surface.
//
// Nothing recomputes a declaration. A surface whose state moves on from what it
// declared — a binding that should now be Disabled, a key that should no longer
// be offered — declares again, from wherever the state changed. That is usually
// Update, which has SetBindings for exactly this reason.
func (sc SelfApplyCtx) SetBindings(bs []Binding) { sc.setBindings(bs) }

// SetFlags replaces what this surface declared about itself. See
// SelfUpdateCtx.SetFlags.
func (sc SelfApplyCtx) SetFlags(fl Flags) { sc.setFlags(fl) }

// Capture claims every mouse message for this surface. See
// SelfUpdateCtx.Capture.
func (sc SelfApplyCtx) Capture() { sc.capture() }

// ReleaseCapture lets go of capture. See SelfUpdateCtx.ReleaseCapture.
func (sc SelfApplyCtx) ReleaseCapture() { sc.releaseCapture() }

// GetFlags reports what this surface last declared about itself. See
// SelfUpdateCtx.GetFlags.
func (sc SelfApplyCtx) GetFlags() Flags { return sc.getFlags() }

// mintSelf builds the read set for the surface holding the phase context. It names
// no target and so cannot fail.
func (t phaseToken) mintSelf(h Handle) reader {
	t.check()
	return reader{phaseToken: t, h: h}
}

// mintAt builds the read set for any handle.
func (t phaseToken) mintAt(minter string, h Handle) reader {
	t.check()
	if t.f.reg.get(h) == nil {
		t.reportDead(minter, h)
	}
	return reader{phaseToken: t, h: h}
}

// mintChild builds the read set for an immediate child of self. The two ways it
// can be wrong get the two answers: reaching past your own children is a
// programming error, because no data makes a stranger's handle turn up where a
// child's was meant to go, while the surface having gone is the condition a
// handle exists to survive.
func (t phaseToken) mintChild(self, h Handle) reader {
	t.check()
	switch n := t.f.reg.get(h); {
	case n == nil:
		t.reportDead("Child", h)
	case n.parent != self:
		bug("froth: Child was asked for %s, which is not an "+
			"immediate child of %s; a surface owns itself and its immediate "+
			"children, and reaching further is a bug rather than a condition",
			h, self)
	}
	return reader{phaseToken: t, h: h}
}

// reportDead says minter was asked for a handle that names no live surface. The
// context is handed back anyway — a second return value would put a branch in
// front of every mint site — and nothing asked of it reaches a surface.
func (t phaseToken) reportDead(minter string, h Handle) {
	t.f.report(Diagnostic{
		Kind: DiagDeadHandle,
		Detail: fmt.Sprintf("%s was asked for %s, which names no live "+
			"surface; the context it returned does nothing", minter, h),
	})
}
