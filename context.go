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

// UpdateCtx is a surface's handle on froth during Deliver, when it is handling
// one message addressed to it.
type UpdateCtx struct {
	phaseToken
	self Handle
	cmds []tea.Cmd

	// msg is the message being delivered, kept only so StopRising can name it
	// when a surface calls it for a message that has no walk to end.
	msg     tea.Msg
	rising  bool
	stopped bool
}

// Rises reports whether the message being delivered keeps travelling after
// this surface has seen it, and so whether StopRising has a walk to end here.
//
// It answers from the same field StopRising checks, so the two cannot disagree:
// where this is true, StopRising is safe, and where it is false, StopRising
// panics.
func (uc *UpdateCtx) Rises() bool {
	uc.check()
	return uc.rising
}

// StopRising ends the walk a rising message is on: no surface above this one
// sees it. Nothing else stops it — a surface that ignores a rising message, or
// acts on it and says nothing, lets it keep going.
//
// It is named for what it does to the message rather than for what the surface
// did with it, which is what keeps "act on this and let it continue" sayable.
//
// Calling it while handling a message that does not embed Rising panics.
// Whether a message travels is a property of its type, so there is no data that
// makes such a call happen; a surface that believes it ended a walk that never
// existed is wrong about the type it is holding.
func (uc *UpdateCtx) StopRising() {
	uc.check()
	if !uc.rising {
		bug("froth: StopRising was called while handling a %T, "+
			"which does not embed froth.Rising; only a rising message has a "+
			"walk to end", uc.msg)
	}
	uc.stopped = true
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

// FocusNext moves focus to the next surface that declared TabStop, in
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

// Invoke delivers an action as an ActionMsg to the focused surface, which rises
// from there. No key is involved; a binding is one way to ask for an action and
// not the only one.
//
// It names no target, so it cannot fail.
func (uc *UpdateCtx) Invoke(a Action) {
	uc.check()
	uc.f.queue = append(uc.f.queue, uc.f.action(a))
}

// Ctx reports the context this surface's asynchronous work should use. It is
// cancelled as the surface leaves the tree, by whichever route, so work started
// here stops with the surface that wanted it; asking again returns the same
// context. Cancelling the instance's context cancels every surface's.
func (uc *UpdateCtx) Ctx() context.Context {
	uc.check()
	return uc.f.surfaceCtx(uc.self)
}

// PostFunc mints the post capability for this surface: what it hands to a
// goroutine, a service or a timer so that they can reach it. The PostFunc type
// says what the function guarantees.
func (uc *UpdateCtx) PostFunc() PostFunc {
	uc.check()
	return uc.f.postFunc(uc.self)
}

// Run schedules a Bubble Tea command. It is returned to the program from the
// update froth is currently servicing, and its result is delivered to this
// surface — not to the root — in a later Update, a nil result included. A
// result that embeds Rising rises from here, as a posted one would. A result
// for a surface that has since left the tree is dropped without a diagnostic.
//
// tea.Quit and tea.Interrupt still stop the program, and a batch or a sequence
// still runs each of its commands, with every result addressed here.
func (uc *UpdateCtx) Run(cmd tea.Cmd) {
	uc.check()
	if cmd == nil {
		return
	}
	uc.cmds = append(uc.cmds, addressCmd(uc.self, cmd))
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

// Ctx reports the context this surface's asynchronous work should use, which is
// where a goroutine started in Mount takes its cancellation from. See
// UpdateCtx.Ctx.
func (ac *ApplyCtx) Ctx() context.Context {
	ac.check()
	return ac.f.surfaceCtx(ac.self)
}

// PostFunc mints the post capability for this surface. See UpdateCtx.PostFunc.
func (ac *ApplyCtx) PostFunc() PostFunc {
	ac.check()
	return ac.f.postFunc(ac.self)
}

// surfaceCtx returns h's context, making it the first time h asks: most
// surfaces never do. It hangs off the instance context rather than the parent
// surface's; ADR 0015 says why.
func (f *Froth) surfaceCtx(h Handle) context.Context {
	n := f.reg.get(h)
	if n == nil {
		bug("froth: %s asked for its context and names no live surface", h)
	}
	if n.ctx == nil {
		n.ctx, n.cancel = context.WithCancel(f.ctx)
	}
	return n.ctx
}
