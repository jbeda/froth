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
// Arrange may be called more than once for one frame once measurement exists,
// so it must not accumulate state across calls.
type Layout interface {
	Arrange(space Rect, children []Child) []Rect
}

// A Child is one of the children a layout is arranging. Today it carries the
// handle alone; the metadata a layout weighs — a child's weight, its measured
// size — is added here as later milestones introduce it, which is why a layout
// is handed a struct rather than a bare handle.
type Child struct {
	Handle Handle
}
