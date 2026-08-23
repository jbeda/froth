package froth

// A CreateOption is something a surface says about a child when it creates it.
// The set of them is froth's: an application can name the type but cannot
// construct one, because the only thing a CreateOption may touch is unexported.
//
// The zero value is a nil function, which Create skips rather than calling.
type CreateOption func(*meta)

// meta is froth's record about one surface, as its creator described it. Every
// field defaults false and every option sets rather than toggles, so passing no
// options and passing the same one twice are both well defined.
type meta struct {
	// tabStop admits this surface to keyboard traversal. Off by default:
	// controls are surfaces (ADR 0003), so most of a tree is structural and
	// would otherwise be walked through.
	tabStop bool

	// textSink routes bare printable keys to this surface as characters rather
	// than matching them as shortcuts while it holds focus. Modified chords
	// still resolve.
	textSink bool

	// focusScope confines keyboard traversal to this surface's subtree.
	focusScope bool
}

func (m *meta) apply(opts []CreateOption) {
	for _, opt := range opts {
		if opt == nil {
			continue
		}
		opt(m)
	}
}

// TabStop admits a surface to keyboard traversal, so tab and shift-tab can land
// on it. A surface that takes keys wants this; a container that only arranges
// its children does not.
//
// It does not gate focus, only walking to it: an unmarked surface can still be
// focused by name, by its parent, or by itself.
func TabStop() CreateOption { return func(m *meta) { m.tabStop = true } }

// TextSink marks a surface that consumes ordinary typing: while it holds focus,
// bare printable keys reach it as characters instead of being matched as
// shortcuts. Modified chords are unaffected.
//
// It is one flag rather than a per-component key list because a component
// cannot be asked which keys it wants, and text entry is not describable as
// bindings — a text field consumes arbitrary runes that appear in none.
func TextSink() CreateOption { return func(m *meta) { m.textSink = true } }

// FocusScope confines keyboard traversal to a surface's subtree, so tab cycles
// within it rather than escaping into the rest of the tree. Dialogs and modals
// are scopes; plain layout containers are not.
//
// The boundary holds both ways: traversal outside a scope steps over the scope
// and everything under it, so focus crosses only by being moved by name. Such a
// move enters the scope — it lands on the child the scope last had focus on, or
// on its first tab stop — which is what a dialog wants when it opens.
func FocusScope() CreateOption { return func(m *meta) { m.focusScope = true } }
