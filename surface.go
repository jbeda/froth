// The interfaces a surface may satisfy. Surface is the two methods froth
// requires; the rest are optional and discovered by type assertion, so a
// surface opts into each by writing the method. They live together because what
// a surface can be asked is one question, whatever part of froth does the
// asking.
//
// Layout is not here: it is implemented by a layout, not by a surface.

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
// Mount is for structure a surface defines about itself: a confirm dialog
// always has OK and Cancel, and its creator should not have to know that.
type Mounter interface {
	Mount(ac *ApplyCtx)
}

// An Unmounter is told when its surface leaves the tree, whichever route it
// leaves by: destroyed, cascaded away under an ancestor, or unmounted at
// shutdown. It runs once, during Apply, and never for a surface that panicked.
//
// It is the mirror of Mount and runs leaf-up, so a child tears down before its
// parent and can still reach it while doing so. It is for what froth does not
// own — a file handle, a registration in something outside froth — and most
// surfaces need it not at all.
//
// Posting from it is ordinary: it runs in Apply and a rising message posted
// there behaves like any other. During Shutdown nothing will drain the queue,
// so a post there is dropped and reported.
type Unmounter interface {
	Unmount(ac *ApplyCtx)
}

// A Binder declares the shortcuts a surface answers to. froth asks for them
// while resolving a key press and while answering a query, which is a
// synchronous call into a surface — rare enough in froth to be worth naming.
//
// Bindings must therefore be a read-only answer about the surface as it stands.
// It may be called from any phase, more than once, and about a surface other
// than the one froth is currently talking to.
type Binder interface {
	Bindings() []Binding
}
