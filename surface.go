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

// A Mounter assembles its own children when it enters the tree. froth calls
// Mount once, during the Apply in which the surface is created, after its
// handle exists and before its first render — never per frame.
//
// It is one of three ways to get children, and the one for structure a surface
// defines about itself: a confirm dialog always has OK and Cancel, and its
// creator should not have to know that.
type Mounter interface {
	Mount(ac *ApplyCtx)
}
