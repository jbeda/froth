package froth

import tea "charm.land/bubbletea/v2"

// A Surface is an independently addressed unit of screen and behaviour. These
// two methods are all froth requires; everything else a surface can do is an
// optional interface discovered by type assertion.
type Surface interface {
	// Render draws the surface's client area into the rect it has been given.
	// It is called once per frame, at final geometry.
	Render(rc *RenderCtx, r Rect) string

	// Update handles one message addressed to this surface. It may change only
	// this surface, and returns work through uc rather than as a value.
	Update(uc *UpdateCtx, msg tea.Msg)
}
