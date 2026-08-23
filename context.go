package froth

import (
	"context"

	tea "charm.land/bubbletea/v2"
)

// RenderCtx is a surface's handle on froth during Render. It carries nothing
// that can change structure: a surface cannot create or destroy anything while
// drawing, and that is a compile error rather than a check.
type RenderCtx struct {
	phaseToken
	self Handle
	rect Rect
}

// Self reports the handle of the surface this context belongs to.
func (rc *RenderCtx) Self() Handle {
	rc.check()
	return rc.self
}

// Focused reports whether this surface is the focused one — the deep end of the
// focus chain, and where a raw key would be delivered.
//
// It answers for this surface alone, so an ancestor of the focused surface
// reports false.
func (rc *RenderCtx) Focused() bool {
	rc.check()
	return rc.f.focus == rc.self
}

// Rect reports the region this surface is being rendered into.
func (rc *RenderCtx) Rect() Rect {
	rc.check()
	return rc.rect
}

// UpdateCtx is a surface's handle on froth during Deliver, when it is handling
// one message addressed to it.
type UpdateCtx struct {
	phaseToken
	ctx  context.Context
	self Handle
	cmds []tea.Cmd
}

// Self reports the handle of the surface this context belongs to, which is what
// a surface posts to when it wants to continue in a later delivery rather than
// finish in this one.
func (uc *UpdateCtx) Self() Handle {
	uc.check()
	return uc.self
}

// Post addresses a message to a surface. It is queued rather than delivered: the
// current delivery finishes, and the message is handled later in the same
// update, before anything is rendered. Nothing in froth is delivered
// re-entrantly, so a surface is never inside two of its own handlers.
//
// A handle that names nothing — a surface that has been destroyed, or one from
// another froth instance — is a defined drop rather than an error. That is what
// makes a handle safe to store: the sender needs no way to ask whether the
// destination is still alive.
func (uc *UpdateCtx) Post(to Handle, msg tea.Msg) {
	uc.check()
	uc.f.queue = append(uc.f.queue, envelope{to: to, msg: msg})
}

// Focus asks froth to move focus to a surface. Focus is structural, so it is
// recorded here and commits in the Apply that follows this delivery — a key
// that moves focus and the frame that shows the move are still one update.
//
// The target is validated when it commits: a handle that is not in the tree is
// reported through the diagnostic hook and focus does not move. It is not a
// panic, because a stored handle whose surface has gone is exactly what handles
// are meant to survive.
func (uc *UpdateCtx) Focus(to Handle) {
	uc.check()
	uc.f.pendingFocus, uc.f.focusPending = to, true
}

// FocusSelf asks froth to move focus to this surface. It names no target, so it
// cannot fail: a surface handling a message is in the tree by construction.
func (uc *UpdateCtx) FocusSelf() {
	uc.check()
	uc.f.pendingFocus, uc.f.focusPending = uc.self, true
}

// FocusNext moves focus to the next surface created with TabStop, in
// depth-first child order, wrapping at the end. No surface implements the walk.
//
// It names no target, so it cannot fail: a tree with no stop in it leaves focus
// where it is rather than reporting a diagnostic.
//
// Like Focus, the move commits in the Apply that follows this delivery. The
// target is resolved now rather than then, so two moves in one delivery advance
// two stops.
func (uc *UpdateCtx) FocusNext() { uc.stepFocus(1) }

// FocusPrev moves focus to the previous surface in traversal order. See
// FocusNext.
func (uc *UpdateCtx) FocusPrev() { uc.stepFocus(-1) }

func (uc *UpdateCtx) stepFocus(delta int) {
	uc.check()
	to, ok := uc.f.stepFocus(uc.f.stepFrom(), delta)
	if !ok {
		return
	}
	uc.f.pendingFocus, uc.f.focusPending = to, true
}

// Ctx reports the context this surface's asynchronous work should use. Every
// surface gets froth's root context, so work started here outlives the surface
// that started it and is cancelled only when the instance's context is.
func (uc *UpdateCtx) Ctx() context.Context {
	uc.check()
	return uc.ctx
}

// Run schedules a Bubble Tea command. It is returned to the program from the
// update froth is currently servicing.
func (uc *UpdateCtx) Run(cmd tea.Cmd) {
	uc.check()
	if cmd == nil {
		return
	}
	uc.cmds = append(uc.cmds, cmd)
}

// ApplyCtx is a surface's handle on froth during Apply, the phase in which
// structural change commits. Only this context can change the tree, so a
// surface creating a child while rendering is a compile error rather than a
// check.
type ApplyCtx struct {
	phaseToken
	self Handle
}

// Self reports the handle of the surface this context belongs to.
func (ac *ApplyCtx) Self() Handle {
	ac.check()
	return ac.self
}

// Create adds a child of this surface and returns its handle. It takes no
// parent: a surface creates a child of itself, so "own yourself and your
// immediate children" is structural rather than validated, and creating a
// grandchild or a sibling is not expressible.
//
// The options say what froth should know about the child and are recorded
// against its handle for its lifetime: creation is the only moment a surface's
// creator gets to describe it. Passing none means every flag is off.
//
// The child is mounted before the frame it was created for is drawn.
//
// Create panics if s is nil: the handle would be indistinguishable from one
// whose surface has died.
func (ac *ApplyCtx) Create(s Surface, opts ...CreateOption) Handle {
	ac.check()
	return ac.f.reg.add(ac.self, s, opts...)
}

// Focus moves focus to a surface, immediately — this is Apply, which is where
// structural change commits, so there is nothing to defer it to.
//
// Unlike Create and SetLayout it names a target, because focus is not
// self-relative: a container mounts its children and says which of them starts
// focused. The handle is validated, and one that is not in the tree is reported
// through the diagnostic hook rather than accepted.
func (ac *ApplyCtx) Focus(to Handle) {
	ac.check()
	ac.f.setFocus(to)
}

// FocusSelf moves focus to this surface. It names no target, so it cannot fail.
func (ac *ApplyCtx) FocusSelf() {
	ac.check()
	ac.f.setFocus(ac.self)
}

// SetLayout attaches a layout to this surface, which places its children from
// then on. Like Create it names no target: a surface owns its own children, so
// saying how they sit is self-relative, and a surface cannot rearrange somebody
// else's.
//
// A surface with no layout stacks its children at its own whole rect.
func (ac *ApplyCtx) SetLayout(l Layout) {
	ac.check()
	ac.f.reg.get(ac.self).layout = l
}
