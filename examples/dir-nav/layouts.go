package main

import "github.com/jbeda/froth"

// The board's layouts. All of them are deliberately ragged: button widths,
// sidebar row heights and the right column's split line up with nothing, so no
// move on this board is a move across a tidy grid.

// minDetailW is the narrowest the middle column is allowed to get. Below a
// couple of cells a surface is a focusable thing you cannot see, which is a
// layout bug rather than a navigation question.
const minDetailW = 12

// minW and minH are the smallest board that arranges. Below it the parts would
// be given rects with no cells, and a focusable surface you cannot see is a
// layout bug rather than a navigation question — judging the rule against one
// would teach the wrong lesson. The root says so instead.
const (
	minW = 80
	minH = 24
)

// buttonGap is the space between toolbar buttons. One cell of nothing means the
// buttons are neighbours geometrically without their borders touching.
const buttonGap = 1

// splits are the board's dividers: everything the layouts read that a key can
// move. The root owns one and the layouts hold a pointer to it, so a nudge shows
// up in the next frame without any layout being set again.
type splits struct {
	toolbarH int
	filterH  int
	sidebarW int
	rightW   int
	metaH    int
	labelH   int
	buttonW  []int
	rowH     []int

	// body is the space the shell divided the last time it arranged, which is
	// what a nudge clamps against — the root has no other way to know how much
	// screen there is.
	body froth.Rect
}

func defaultSplits() splits {
	return splits{
		toolbarH: 3,
		filterH:  3,
		sidebarW: 18,
		rightW:   30,
		metaH:    7,
		labelH:   7,
		buttonW:  []int{13, 17, 10},
		rowH:     []int{4, 3, 4, 3},
	}
}

// fit reports the two column widths to use, shrinking them a cell at a time from
// whichever is wider until the middle column has room. The stored values are
// left alone: a window narrowed and widened again should come back to the sizes
// that were set, not to whatever the narrow window could afford.
func (s *splits) fit(bodyW int) (sidebarW, rightW int) {
	sidebarW, rightW = s.sidebarW, s.rightW
	for sidebarW+rightW+minDetailW > bodyW && sidebarW+rightW > 0 {
		if sidebarW >= rightW {
			sidebarW--
		} else {
			rightW--
		}
	}
	return max(sidebarW, 0), max(rightW, 0)
}

// shell divides the screen into the six parts the board is made of. It reads its
// children by position, in the order the root creates them: toolbar, sidebar,
// detail, right column, filter, banner. The banner is created last so it renders
// last, and placed first so it reads along the top.
type shell struct{ s *splits }

func (l shell) Arrange(space froth.Rect, children []froth.Child) []froth.Rect {
	rects := make([]froth.Rect, len(children))
	if len(children) != 6 || tooSmall(space.Size()) {
		return rects
	}

	banner := froth.Rect{X: space.X, Y: space.Y, W: space.W, H: 1}
	toolbar := froth.Rect{X: space.X, Y: banner.Y + banner.H, W: space.W, H: l.s.toolbarH}
	filter := froth.Rect{X: space.X, Y: space.Y + space.H - l.s.filterH, W: space.W, H: l.s.filterH}

	bodyY := toolbar.Y + toolbar.H
	body := froth.Rect{X: space.X, Y: bodyY, W: space.W, H: max(filter.Y-bodyY, 0)}
	l.s.body = body

	sidebarW, rightW := l.s.fit(body.W)
	sidebar := froth.Rect{X: body.X, Y: body.Y, W: sidebarW, H: body.H}
	right := froth.Rect{X: body.X + body.W - rightW, Y: body.Y, W: rightW, H: body.H}
	detailX := sidebar.X + sidebar.W
	detail := froth.Rect{X: detailX, Y: body.Y, W: max(right.X-detailX, 0), H: body.H}

	rects[0], rects[1], rects[2] = toolbar, sidebar, detail
	rects[3], rects[4], rects[5] = right, filter, banner
	return rects
}

// buttons lays the toolbar out left to right at the widths the splits give,
// leaving whatever is left over empty.
type buttons struct{ s *splits }

func (l buttons) Arrange(space froth.Rect, children []froth.Child) []froth.Rect {
	rects := make([]froth.Rect, len(children))
	x := space.X
	for i := range rects {
		w := 0
		if i < len(l.s.buttonW) {
			w = min(l.s.buttonW[i], max(space.X+space.W-x, 0))
		}
		rects[i] = froth.Rect{X: x, Y: space.Y, W: w, H: space.H}
		x += w + buttonGap
	}
	return rects
}

// rows stacks the sidebar top to bottom at the heights the splits give, leaving
// whatever is left over empty. The rows have different heights, so a move out of
// the sidebar and back cannot rely on the row it left being the one straight
// ahead.
type rows struct{ s *splits }

func (l rows) Arrange(space froth.Rect, children []froth.Child) []froth.Rect {
	rects := make([]froth.Rect, len(children))
	y := space.Y
	for i := range rects {
		h := 0
		if i < len(l.s.rowH) {
			h = min(l.s.rowH[i], max(space.Y+space.H-y, 0))
		}
		rects[i] = froth.Rect{X: space.X, Y: y, W: space.W, H: h}
		y += h
	}
	return rects
}

// scopeMinH is the least the scope is given. Below it the controls inside have
// nowhere to go, and focus can be moved into a scope holding nothing visible.
const scopeMinH = 8

// stack gives the right column's two panels a band each off the top and the rest
// of the space to the scope beneath them. Two panels rather than one because a
// move rightwards out of the detail pane should have to choose.
//
// The panels give way to the scope rather than the other way round: their
// heights are what the keys move, so they are what should absorb a screen too
// short for all three.
type stack struct{ s *splits }

func (l stack) Arrange(space froth.Rect, children []froth.Child) []froth.Rect {
	rects := make([]froth.Rect, len(children))
	if len(children) != 3 {
		return rects
	}

	top, mid := l.s.metaH, l.s.labelH
	for top+mid+scopeMinH > space.H && top+mid > 0 {
		if top >= mid {
			top--
		} else {
			mid--
		}
	}

	y := space.Y
	for i, h := range []int{max(top, 0), max(mid, 0)} {
		rects[i] = froth.Rect{X: space.X, Y: y, W: space.W, H: h}
		y += h
	}
	rects[2] = froth.Rect{X: space.X, Y: y, W: space.W, H: max(space.Y+space.H-y, 0)}
	return rects
}

// scopeInner places the scope's controls: two side by side and one below,
// offset from both, so a move inside the scope has something to decide rather
// than one obvious neighbour each way.
//
// They are placed against the bottom of the scope rather than against the top,
// so the text the scope draws for itself cannot collide with them however the
// panels above are resized.
type scopeInner struct{}

func (scopeInner) Arrange(space froth.Rect, children []froth.Child) []froth.Rect {
	rects := make([]froth.Rect, len(children))
	x, y := space.X+2, space.Y+space.H-4
	at := []froth.Rect{
		{X: x, Y: y, W: 9, H: 1},
		{X: x + 11, Y: y, W: 12, H: 1},
		{X: x + 4, Y: y + 2, W: 14, H: 1},
	}
	for i := range rects {
		if i < len(at) {
			rects[i] = within(at[i], space)
		}
	}
	return rects
}

// tooSmall reports whether the screen is below the size the board needs.
func tooSmall(size froth.Size) bool {
	return size.W < minW || size.H < minH
}

// within reports r if it fits inside space, and an empty rect if it does not. A
// control that does not fit is better absent from the frame than drawn over the
// border of the thing containing it.
func within(r, space froth.Rect) froth.Rect {
	if r.X < space.X || r.Y < space.Y ||
		r.X+r.W > space.X+space.W || r.Y+r.H > space.Y+space.H {
		return froth.Rect{}
	}
	return r
}
