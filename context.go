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
	rect Rect
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
	cmds []tea.Cmd
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
