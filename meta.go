package froth

// Flags are what a surface declares about itself: how it takes part in focus
// and in input. A surface's type declares them through Declarer as it enters
// the tree, and the surface restates the whole set with SetFlags. froth stores
// the answer and never asks again.
//
// What a creator says about a child — FocusScope, MouseTransparent — is not
// here. Every flag has exactly one author, which is what makes replacing the
// whole set safe. See ADR 0019.
type Flags struct {
	// TabStop admits this surface to keyboard traversal, so tab and shift-tab
	// can land on it. It does not gate focus, only walking to it: a surface
	// without it can still be focused by name.
	TabStop bool

	// ClickFocus admits this surface to the mouse: a click on it moves focus
	// to it. Independent of TabStop in both directions.
	ClickFocus bool

	// TextSink routes bare printable keys to this surface as characters rather
	// than matching them as shortcuts while it holds focus. Modified chords
	// still resolve.
	//
	// It is one flag rather than a per-component key list because a component
	// cannot be asked which keys it wants, and text entry is not describable
	// as bindings — a text field consumes arbitrary runes that appear in none.
	TextSink bool
}

// A CreateOption is something a surface says about a child when it creates it.
// The set of them is froth's: an application can name the type but cannot
// construct one, because the only thing a CreateOption may touch is unexported.
//
// An option says how a child sits in the composition. What the child is — see
// Flags — its own type declares.
//
// The zero value is a nil function, which Create skips rather than calling.
type CreateOption func(*meta)

// meta is froth's record about one surface: what its creator described it as,
// and what its own type declared. Every field defaults false and every option
// sets rather than toggles, so passing no options and passing the same one
// twice are both well defined.
type meta struct {
	// flags are the surface's own declaration about itself.
	flags Flags

	// focusScope confines keyboard traversal to this surface's subtree.
	focusScope bool

	// mouseTransparent takes this surface and its subtree out of the hit walk,
	// so a click passes through to whatever lies beneath it in Z.
	mouseTransparent bool

	// modal takes input for this surface's parent's whole subtree.
	modal bool
}

func (m *meta) apply(opts []CreateOption) {
	for _, opt := range opts {
		if opt == nil {
			continue
		}
		opt(m)
	}
}

// FocusScope confines keyboard traversal to a surface's subtree, so tab cycles
// within it rather than escaping into the rest of the tree. Dialogs and modals
// are scopes; plain layout containers are not.
//
// The boundary holds both ways: traversal outside a scope steps over the scope
// and everything under it, so focus crosses only by being moved by name. Such a
// move enters the scope — it lands on the child the scope last had focus on, or
// on its first tab stop — unless the scope surface itself admits focus, in
// which case the move is taken literally.
//
// There is no setter for it anywhere: a scope boundary is fixed for a surface's
// lifetime.
func FocusScope() CreateOption { return func(m *meta) { m.focusScope = true } }

// MouseTransparent takes a surface, and everything below it, out of the walk
// that finds what a click belongs to, so the click reaches whatever lies
// beneath it in Z. It is not visual transparency: the surface still draws.
//
// The creator says it because it describes how the surface is being used rather
// than what it is. ChildCtx.SetMouseTransparent changes it later.
func MouseTransparent() CreateOption { return func(m *meta) { m.mouseTransparent = true } }

// Modal makes a surface take input for its parent's whole subtree: a shortcut
// declared anywhere else in that subtree does not resolve, and a raw key aimed
// there is dropped. A surface parented to the root is modal to the whole
// application; one parented lower blocks only that subtree.
//
// Two things get past it and nothing else does: a Binding marked SurvivesModal,
// and any declaration of an action froth services itself. Where several are open
// over one surface, the one on top governs.
//
// There is no setter: closing a modal is destroying it. Being modal says nothing
// about traversal, so a modal that should also hold the tab walk declares
// FocusScope as well.
func Modal() CreateOption { return func(m *meta) { m.modal = true } }
