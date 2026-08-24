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
// A surface created with FocusScope is entered rather than focused: the move
// lands on the child the scope last had focus on, or on its first tab stop.
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

// A ReadCtx is what Render mints, for a surface's own handle, one of its
// children, or a stranger alike. Render changes nothing, so the three tiers
// permit identical operations and collapse to one type.
type ReadCtx struct{ reader }

// A SelfUpdateCtx is what a surface holds on itself while handling a message.
// Structural change is Apply's, so what is here is what a surface can do to
// itself without restructuring anything.
type SelfUpdateCtx struct{ router }

// Destroy removes this surface and everything below it. The removal is recorded
// and commits at the next Apply, so the surface stays live for the rest of this
// update: messages already queued to it are still delivered, and a rising walk
// passing through it still completes.
func (sc SelfUpdateCtx) Destroy() { sc.destroy() }

// A SelfApplyCtx is what a surface holds on itself during Apply, and the only
// place structure a surface owns is assembled. Create and SetLayout name no
// target, so creating a grandchild or arranging somebody else's children is not
// expressible rather than refused.
type SelfApplyCtx struct{ router }

// A ChildCtx names an immediate child of the surface that minted it. The
// parentage was checked at the mint, so nothing here rechecks it.
type ChildCtx struct{ router }

// Destroy removes this child and everything below it. Removing a deep branch
// means destroying your own child and letting the cascade do the rest — which
// is the ownership rule holding rather than a limitation, since the surfaces
// down there are not yours to name.
//
// It takes effect at once when called during Apply, and is recorded and commits
// at the next Apply when called during Deliver.
func (cc ChildCtx) Destroy() { cc.destroy() }

// An AnyCtx names any live surface, whoever owns it. It carries what can be
// done to a stranger — reaching it, not restructuring it — which is why
// Query.Focus handing out a handle to a surface the caller never created is
// safe: there is no type that could destroy one.
type AnyCtx struct{ router }

// Create adds a child of this surface and returns its handle. It takes no
// parent: a surface creates a child of itself, so "own yourself and your
// immediate children" is structural rather than validated.
//
// The options say what froth should know about the child and are recorded
// against its handle for its lifetime: creation is the only moment a surface's
// creator gets to describe it. Passing none means every flag is off.
//
// The child is mounted before the frame it was created for is drawn.
//
// Create panics if s is nil: the handle would be indistinguishable from one
// whose surface has died.
func (sc SelfApplyCtx) Create(s Surface, opts ...CreateOption) Handle {
	sc.check()
	return sc.f.reg.add(sc.h, s, opts...)
}

// Destroy removes this surface and everything below it, at once: Apply is where
// structural change commits, so there is nothing to record.
func (sc SelfApplyCtx) Destroy() { sc.destroy() }

// SetLayout attaches a layout to this surface, which places its children from
// then on. A surface with no layout stacks its children at its own whole rect.
func (sc SelfApplyCtx) SetLayout(l Layout) {
	sc.check()
	sc.f.reg.get(sc.h).layout = l
}

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
