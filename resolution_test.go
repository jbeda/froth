package froth

import "testing"

// Resolving a direction to a surface is a self-contained algorithm over rects:
// given where a move starts and where the candidates are, one of them wins. It
// reaches for no tree, no scope and no focus, so it is tested as the function it
// is rather than through a surface tree built to produce the geometry by hand.
//
// The wiring around it — which surfaces are candidates at all, and what a key
// press does — is tested through the seam in direction_test.go.

// A spot is one candidate in a case, named so a failure says which surface won
// rather than which handle did.
type spot struct {
	name string
	rect Rect
}

func at(name string, x, y, w, h int) spot {
	return spot{name: name, rect: Rect{X: x, Y: y, W: w, H: h}}
}

// resolve runs the algorithm over spots and reports the winner's name, or "" when
// nothing lies that way. Paint order is the order the spots are written in, so a
// case that turns on it says so by its ordering.
func resolve(d direction, src Rect, spots []spot) string {
	cands := make([]candidate, len(spots))
	for i, s := range spots {
		cands[i] = candidate{h: Handle{id: uint64(i + 1)}, rect: s.rect, order: i}
	}
	won, ok := pick(d, src, cands)
	if !ok {
		return ""
	}
	return spots[won.id-1].name
}

func TestResolvingADirectionToASurface(t *testing.T) {
	// The source in most cases: a two-by-one at the origin of an open field.
	cases := []struct {
		name  string
		d     direction
		src   Rect
		spots []spot
		want  string
	}{{
		name: "nothing lies that way",
		d:    dirLeft,
		src:  Rect{X: 0, Y: 0, W: 2, H: 1},
		spots: []spot{
			at("right", 3, 0, 2, 1),
		},
		want: "",
	}, {
		// The beam test is absolute: a candidate the source is travelling
		// straight at wins over a nearer one off to the side, which could be
		// reached by pressing a different direction.
		name: "in the beam beats nearer out of it",
		d:    dirUp,
		src:  Rect{X: 0, Y: 4, W: 2, H: 1},
		spots: []spot{
			at("ahead", 0, 0, 2, 1),
			at("aside", 3, 3, 2, 1),
		},
		want: "ahead",
	}, {
		// Distance, put in conflict with alignment below so that dropping the
		// distance test would take the other candidate.
		name: "nearest along the axis beats better aligned",
		d:    dirRight,
		src:  Rect{X: 0, Y: 0, W: 1, H: 6},
		spots: []spot{
			at("near", 2, 0, 1, 1),
			at("far", 5, 3, 1, 1),
		},
		want: "near",
	}, {
		// The clamp on the distance test. Reaching further into the source
		// must not score as being closer to it.
		name: "overlapping scores no nearer than flush",
		d:    dirLeft,
		src:  Rect{X: 4, Y: 0, W: 4, H: 1},
		spots: []spot{
			at("overlapping", 2, 0, 4, 1),
			at("flush", 0, 0, 4, 1),
		},
		want: "flush", // both score zero, so reading order takes the leftmost
	}, {
		name: "equally distant candidates break by alignment",
		d:    dirUp,
		src:  Rect{X: 0, Y: 2, W: 6, H: 1},
		spots: []spot{
			at("off centre", 0, 0, 2, 1),
			at("dead ahead", 2, 0, 2, 1),
		},
		want: "dead ahead",
	}, {
		// Both are two cells off centre, so alignment ties and reading order
		// takes the leftmost.
		name: "tied on alignment, break by reading order",
		d:    dirUp,
		src:  Rect{X: 0, Y: 2, W: 6, H: 1},
		spots: []spot{
			at("left", 0, 0, 2, 1),
			at("right", 4, 0, 2, 1),
		},
		want: "left",
	}, {
		// The last test, which is what makes the order total. Two stops at one
		// rect tie on everything above, and the frame still says which one the
		// user is looking at.
		name: "same rect resolves to the one on top",
		d:    dirRight,
		src:  Rect{X: 0, Y: 0, W: 2, H: 1},
		spots: []spot{
			at("underneath", 3, 0, 2, 1),
			at("on top", 3, 0, 2, 1),
		},
		want: "on top",
	}, {
		// The predicate: both edges have to have moved. A surface that started
		// left of the source but ends right of it grew around it rather than
		// moving, and pressing left inside something wider is not a request to
		// focus the thing you are already inside.
		name: "a surface that merely grew past the source is not in the direction",
		d:    dirLeft,
		src:  Rect{X: 4, Y: 0, W: 2, H: 1},
		spots: []spot{
			at("wider", 2, 0, 6, 1),
			at("neighbour", 0, 0, 2, 1),
		},
		want: "neighbour",
	}, {
		// The other half of the predicate. Overlap is allowed, because a scope
		// is a stop within itself and so a stop can contain another one.
		name: "overlapping the source is still in the direction",
		d:    dirLeft,
		src:  Rect{X: 2, Y: 0, W: 3, H: 1},
		spots: []spot{
			at("overlapping", 0, 0, 3, 1),
		},
		want: "overlapping",
	}}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := resolve(c.d, c.src, c.spots); got != c.want {
				t.Errorf("the move landed on %q, want %q", got, c.want)
			}
		})
	}
}
