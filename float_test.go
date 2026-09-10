package froth_test

import (
	"slices"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/jbeda/froth"
)

// floatHost is a container that creates its children with options and records
// how froth enumerates them afterwards.
type floatHost struct {
	body   string
	layout froth.Layout
	kids   []nestKid

	self             froth.Handle
	children, floats []froth.Handle
}

func (h *floatHost) Render(*froth.RenderCtx, froth.Size) string { return h.body }
func (h *floatHost) Update(*froth.UpdateCtx, tea.Msg)           {}

func (h *floatHost) Mount(ac *froth.ApplyCtx) {
	h.self = ac.Self().Handle()
	for _, k := range h.kids {
		ac.Self().Create(k.surface, k.opts...)
	}
	if h.layout != nil {
		ac.Self().SetLayout(h.layout)
	}
	h.children = ac.Self().Children()
	h.floats = ac.Self().FloatChildren()
}

func floating(s froth.Surface) nestKid {
	return nestKid{surface: s, opts: []froth.CreateOption{froth.Float()}}
}

// A float is placed against the screen, centred on the rectangle its output
// makes, and its parent's layout never sees it: the layout arranges the other
// children as though the float were not there.
func TestAFloatIsCentredOnTheScreenAndLeftOutOfItsParentsLayout(t *testing.T) {
	b := &block{ch: "b", w: 1, h: 1}
	fl := &block{ch: "f", w: 2, h: 2}
	lay := &halves{}
	inner := &floatHost{body: "....", layout: lay, kids: []nestKid{{surface: b}, floating(fl)}}
	root := &laidOut{
		parent: parent{body: "........\n........\n........\n........", kids: []froth.Surface{&block{ch: ".", w: 1, h: 1}, inner}},
		layout: &halves{},
	}
	f := newSized(t, root, 8, 4)
	frame := f.Composite(f.Root())

	if got, want := frame.Rects[fl.self], (froth.Rect{X: 3, Y: 1, W: 2, H: 2}); got != want {
		t.Errorf("float rect = %v, want %v (centred on the screen, not on its parent's half)", got, want)
	}
	if got, want := fl.size, (froth.Size{W: 8, H: 4}); got != want {
		t.Errorf("float was offered %v, want the whole screen %v", got, want)
	}
	if got, want := frame.Content, "....b...\n...ff...\n...ff...\n........"; got != want {
		t.Errorf("View().Content = %q, want %q", got, want)
	}
	if want := []froth.Handle{b.self}; !slices.Equal(handlesOf(lay.got), want) {
		t.Errorf("the layout arranged %v, want %v", handlesOf(lay.got), want)
	}
	if got, want := frame.Rects[b.self], (froth.Rect{X: 4, Y: 0, W: 2, H: 4}); got != want {
		t.Errorf("sibling rect = %v, want %v", got, want)
	}
}

func handlesOf(cs []froth.Child) []froth.Handle {
	out := make([]froth.Handle, len(cs))
	for i, c := range cs {
		out[i] = c.Handle
	}
	return out
}

// A container with no layout stacks its children at its own rect; a float is
// still placed against the screen.
func TestAFloatUnderALayoutLessContainerIsStillPlacedAgainstTheScreen(t *testing.T) {
	fl := &block{ch: "f", w: 2, h: 1}
	inner := &floatHost{body: "....", kids: []nestKid{floating(fl)}}
	root := &laidOut{
		parent: parent{body: "........", kids: []froth.Surface{&block{ch: ".", w: 1, h: 1}, inner}},
		layout: &halves{},
	}
	f := newSized(t, root, 8, 1)

	if got, want := f.Composite(f.Root()).Rects[fl.self], (froth.Rect{X: 3, Y: 0, W: 2, H: 1}); got != want {
		t.Errorf("float rect = %v, want %v", got, want)
	}
}

// A float that fills what it is offered is full-screen, because what it is
// offered is the screen.
func TestAFloatThatFillsItsOfferIsFullScreen(t *testing.T) {
	fl := &patch{letter: "f"}
	root := &floatHost{body: "....\n....", kids: []nestKid{floating(fl)}}
	f := newSized(t, root, 4, 2)
	frame := f.Composite(f.Root())

	if got, want := frame.Rects[fl.self], (froth.Rect{X: 0, Y: 0, W: 4, H: 2}); got != want {
		t.Errorf("float rect = %v, want %v", got, want)
	}
	if got, want := frame.Content, "ffff\nffff"; got != want {
		t.Errorf("View().Content = %q, want %q", got, want)
	}
}

// A float is a container like any other: its own layout arranges its children
// within the rect it resolved to.
func TestAFloatsChildrenAreArrangedWithinItsRect(t *testing.T) {
	c := &block{ch: "c", w: 1, h: 1}
	fl := &laidOut{parent: parent{body: "....\n....", kids: []froth.Surface{c}}, layout: &halves{}}
	root := &floatHost{body: "", kids: []nestKid{floating(fl)}}
	f := newSized(t, root, 8, 4)
	frame := f.Composite(f.Root())

	if got, want := frame.Rects[fl.self], (froth.Rect{X: 2, Y: 1, W: 4, H: 2}); got != want {
		t.Errorf("float rect = %v, want %v", got, want)
	}
	if got, want := frame.Rects[c.self], (froth.Rect{X: 2, Y: 1, W: 2, H: 2}); got != want {
		t.Errorf("float's child rect = %v, want %v", got, want)
	}
}

// Children enumerates what the layout arranges, FloatChildren the rest, both in
// sibling order.
func TestChildrenOmitsFloatsAndFloatChildrenReportsThem(t *testing.T) {
	a, fl1, b, fl2 := &block{ch: "a", w: 1, h: 1}, &block{ch: "1", w: 1, h: 1}, &block{ch: "b", w: 1, h: 1}, &block{ch: "2", w: 1, h: 1}
	root := &floatHost{body: "....", kids: []nestKid{{surface: a}, floating(fl1), {surface: b}, floating(fl2)}}
	newSized(t, root, 4, 1)

	if want := []froth.Handle{a.self, b.self}; !slices.Equal(root.children, want) {
		t.Errorf("Children() = %v, want %v", root.children, want)
	}
	if want := []froth.Handle{fl1.self, fl2.self}; !slices.Equal(root.floats, want) {
		t.Errorf("FloatChildren() = %v, want %v", root.floats, want)
	}
}

// Stacking is ordinary: a float sits in creation order among its siblings, so
// one created before a sibling that covers it is drawn under that sibling.
func TestAFloatStacksInCreationOrderAmongItsSiblings(t *testing.T) {
	t.Run("float first", func(t *testing.T) {
		root := &floatHost{body: "....", kids: []nestKid{floating(&block{ch: "f", w: 1, h: 1}), {surface: &patch{letter: "s"}}}}
		f := newSized(t, root, 4, 1)
		if got, want := f.View().Content, "ssss"; got != want {
			t.Errorf("View().Content = %q, want %q", got, want)
		}
	})
	t.Run("float last", func(t *testing.T) {
		root := &floatHost{body: "....", kids: []nestKid{{surface: &patch{letter: "s"}}, floating(&block{ch: "f", w: 1, h: 1})}}
		f := newSized(t, root, 4, 1)
		if got, want := f.View().Content, "sfss"; got != want {
			t.Errorf("View().Content = %q, want %q", got, want)
		}
	})
}

// A click on a float reaches it, and a click beside it reaches what lies
// beneath: the float occupies the rectangle it drew, where it was centred.
func TestAClickReachesAFloatWhereItWasCentred(t *testing.T) {
	under := &patch{letter: "u"}
	fl := &patch{letter: "f", w: 2, h: 1}
	root := &patch{letter: "r", kids: []nestKid{{surface: under}, floating(fl)}}
	f := mouseTree(t, root, 6, 1)

	f.Update(click(2, 0))
	if fl.last != "click@2,0" || under.last != "" {
		t.Errorf("click on the float: float got %q, surface beneath got %q", fl.last, under.last)
	}
	f.Update(click(4, 0))
	if fl.last != "click@2,0" || under.last != "click@4,0" {
		t.Errorf("click beside the float: float got %q, surface beneath got %q", fl.last, under.last)
	}
}
