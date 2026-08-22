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
