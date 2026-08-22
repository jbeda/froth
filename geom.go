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
func (r Rect) Empty() bool { return r.W <= 0 || r.H <= 0 }
