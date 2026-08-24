package froth

import (
	"context"
	"slices"

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

// Self mints a context on the surface this one belongs to. It names no target,
// so it cannot fail.
func (rc *RenderCtx) Self() ReadCtx { return ReadCtx{rc.mintSelf(rc.self)} }

// Child mints a context on an immediate child of this surface, and panics for a
// handle that is not one.
func (rc *RenderCtx) Child(h Handle) ReadCtx { return ReadCtx{rc.mintChild(rc.self, h)} }

// At mints a context on any live surface.
func (rc *RenderCtx) At(h Handle) ReadCtx { return ReadCtx{rc.mintAt("At", h)} }

// Focused reports whether this surface is the focused one — the deep end of the
// focus chain, and where a raw key would be delivered.
//
// It answers for this surface alone, so an ancestor of the focused surface
// reports false; FocusWithin is the question that includes ancestors.
func (rc *RenderCtx) Focused() bool {
	rc.check()
	return rc.f.focus == rc.self
}

// FocusWithin reports whether focus is at or below this surface: true for the
// focused surface itself and for every ancestor of it, false for everything
// else. A container draws itself active with this while a child holds the
// keyboard, the way CSS :focus-within lets an ancestor style itself.
//
// It is a superset of Focused rather than a rival to it, derived from the same
// one handle: a surface being rendered is in the tree by definition, so the
// focused surface always finds itself on the chain.
func (rc *RenderCtx) FocusWithin() bool {
	rc.check()
	return slices.Contains(rc.f.focusChain(), rc.self)
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

// Self mints a context on the surface this one belongs to — what a surface
// posts to when it wants to continue in a later delivery rather than finish in
// this one. It names no target, so it cannot fail.
func (uc *UpdateCtx) Self() SelfUpdateCtx {
	return SelfUpdateCtx{router{uc.mintSelf(uc.self)}}
}

// Child mints a context on an immediate child of this surface, and panics for a
// handle that is not one.
func (uc *UpdateCtx) Child(h Handle) ChildCtx {
	return ChildCtx{router{uc.mintChild(uc.self, h)}}
}

// At mints a context on any live surface. A handle that names nothing — a
// surface that has been destroyed, or one from another froth instance — is a
// reported drop rather than an error. That is what makes a handle safe to
// store: the sender needs no way to ask whether the destination is still alive.
func (uc *UpdateCtx) At(h Handle) AnyCtx {
	return AnyCtx{router{uc.mintAt("At", h)}}
}

// FocusNext moves focus to the next surface created with TabStop, in
// depth-first child order, wrapping at the end. No surface implements the walk.
//
// The walk is bounded by the focus scope this surface sits in and never leaves
// it. froth binds tab to this, so a surface calls it only to move focus from
// something other than that key.
//
// It names no target, so it cannot fail: a scope with no stop in it leaves focus
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

// Self mints a context on the surface this one belongs to, which is the only
// one it may restructure. It names no target, so it cannot fail.
func (ac *ApplyCtx) Self() SelfApplyCtx {
	return SelfApplyCtx{router{ac.mintSelf(ac.self)}}
}

// Child mints a context on an immediate child of this surface, and panics for a
// handle that is not one.
func (ac *ApplyCtx) Child(h Handle) ChildCtx {
	return ChildCtx{router{ac.mintChild(ac.self, h)}}
}

// At mints a context on any live surface. See UpdateCtx.At for what a handle
// that names nothing does.
func (ac *ApplyCtx) At(h Handle) AnyCtx {
	return AnyCtx{router{ac.mintAt("At", h)}}
}
