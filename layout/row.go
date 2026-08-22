// Package layout holds froth's stock layouts. A layout turns a container's rect
// and its children into a rect per child; it sees handles and metadata rather
// than surface state, so everything here is a pure function from space to rects
// and is tested as one.
package layout

import "github.com/jbeda/froth"

// An Option configures a stock layout.
type Option func(*options)

type options struct {
	gap int
}

// Gap puts n cells between adjacent children. It never appears at the edges of
// the space, so one child fills it whatever the gap is.
func Gap(n int) Option {
	return func(o *options) { o.gap = max(n, 0) }
}

// Row divides its space horizontally, left to right, giving every child the
// full height and an equal share of the width.
//
// Cells cannot be split, so a width that does not divide evenly leaves a
// remainder, and it goes one cell at a time to the leftmost children. Space too
// small for the children in it starves the rightmost ones rather than dropping
// them: every child gets a rect, and a child that did not fit gets an empty one.
func Row(opts ...Option) froth.Layout {
	var o options
	for _, opt := range opts {
		opt(&o)
	}
	return row{gap: o.gap}
}

type row struct{ gap int }

func (r row) Arrange(space froth.Rect, children []froth.Child) []froth.Rect {
	if len(children) == 0 {
		return nil
	}

	share := max(space.W-r.gap*(len(children)-1), 0)
	each, extra := share/len(children), share%len(children)

	rects := make([]froth.Rect, len(children))
	x := space.X
	for i := range rects {
		w := each
		if i < extra {
			w++
		}
		rects[i] = froth.Rect{X: x, Y: space.Y, W: w, H: space.H}
		x += w + r.gap
	}
	return rects
}
