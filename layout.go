package froth

// A Layout turns a container's rect and its children into a rect per child. It
// is attached to a container by that container, and it is the only thing that
// decides where a child sits.
//
// Arrange sees handles and metadata, never surface state, which is what lets a
// layout be written and tested as a pure function from space to rects. It must
// return exactly one rect per child, in the order it was given them; a child
// with no rect has nowhere to render, so froth treats a short answer as a bug
// rather than guessing.
//
// Arrange may be called more than once for one frame, so it must not
// accumulate state across calls.
type Layout interface {
	Arrange(space Rect, children []Child) []Rect
}

// A LayoutFunc is a Layout written as a function, for a container whose
// arrangement has no state to carry between calls.
type LayoutFunc func(space Rect, children []Child) []Rect

// Arrange calls f.
func (f LayoutFunc) Arrange(space Rect, children []Child) []Rect { return f(space, children) }

// A Child is one of the children a layout is arranging. It is a struct rather
// than a bare handle so that the metadata a layout weighs can hang off it.
type Child struct {
	Handle Handle
}
