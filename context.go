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
// It is a read, not a claim: a surface renders differently when focused, and
// that is what a focus indicator is made of. It answers for this surface alone,
// so an ancestor of the focused surface reports false; the chrome froth draws
// around a surface on the chain is M2's, from handle metadata rather than from
// here.
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

// Ctx reports the context this surface's asynchronous work should use.
//
// Today every surface gets froth's root context. The per-surface tree — where a
// surface's context is cancelled when the surface is destroyed — arrives with
// the registry, since it needs surface lifetimes to hang off. Until then, work
// started here outlives the surface that started it.
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
// The child is mounted before the frame it was created for is drawn.
func (ac *ApplyCtx) Create(s Surface) Handle {
	ac.check()
	return ac.f.reg.add(ac.self, s)
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
