package froth

// Rect is a region of the terminal in cells, with its origin at the top left of
// the screen.
type Rect struct {
	X, Y, W, H int
}

// Size is an extent in cells.
type Size struct {
	W, H int
}

// Point is a position in cells.
type Point struct {
	X, Y int
}

// Pt is shorthand for a Point.
func Pt(x, y int) Point { return Point{X: x, Y: y} }

// Size reports r's extent.
func (r Rect) Size() Size { return Size{W: r.W, H: r.H} }

// Empty reports whether r has no cells.
func (r Rect) Empty() bool { return r.Size().Empty() }

// Empty reports whether s has no cells.
func (s Size) Empty() bool { return s.W <= 0 || s.H <= 0 }

// The far edges and centres, in the same screen coordinates as X and Y. A rect
// is a position and an extent rather than two corners, so the arithmetic is
// written once here instead of at every comparison.
func (r Rect) right() int  { return r.X + r.W }
func (r Rect) bottom() int { return r.Y + r.H }
func (r Rect) midX() int   { return r.X + r.W/2 }
func (r Rect) midY() int   { return r.Y + r.H/2 }
