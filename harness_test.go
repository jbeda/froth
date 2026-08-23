package froth_test

import (
	"testing"

	"github.com/jbeda/froth"
)

func TestCompositeReturnsASubtreesOutputAndARectPerSurface(t *testing.T) {
	inner := &block{ch: "i", w: 2, h: 1}
	branch := &parent{body: "bbbb\nbbbb", kids: []froth.Surface{inner}}
	root := &parent{body: "....\n....", kids: []froth.Surface{branch}}
	f := newSized(t, root, 4, 2)

	frame := f.Composite(branch.self)

	if got, want := frame.Content, "iibb\nbbbb"; got != want {
		t.Errorf("subtree content = %q, want %q", got, want)
	}
	want := map[froth.Handle]froth.Rect{
		branch.self: {W: 4, H: 2},
		inner.self:  {W: 4, H: 2},
	}
	if len(frame.Rects) != len(want) {
		t.Fatalf("subtree has %d rects, want %d", len(frame.Rects), len(want))
	}
	for h, r := range want {
		if got := frame.Rects[h]; got != r {
			t.Errorf("rect for %v = %+v, want %+v", h, got, r)
		}
	}
	if _, ok := frame.Rects[f.Root()]; ok {
		t.Error("a subtree frame reported a rect for a surface outside it")
	}
}

func TestCompositeOfAHandleThatNamesNothingIsEmpty(t *testing.T) {
	f := newSized(t, &pane{title: "x"}, 4, 2)

	if got := f.Composite(froth.Handle{}); got.Content != "" || len(got.Rects) != 0 {
		t.Errorf("Composite of the zero handle = %+v, want an empty frame", got)
	}
}

// Three levels with siblings at each: every surface draws over its parent, and
// every later sibling over its earlier sibling's whole subtree.
func TestNestedSiblingsRenderInDepthFirstOrder(t *testing.T) {
	root := &parent{body: "....\n....", kids: []froth.Surface{
		&parent{body: "aaaa\naaaa", kids: []froth.Surface{
			&block{ch: "1", w: 2, h: 1},
			&block{ch: "2", w: 1, h: 1},
		}},
		&parent{body: "bb", kids: []froth.Surface{
			&block{ch: "3", w: 1, h: 1},
		}},
	}}
	f := newSized(t, root, 4, 2)

	if got, want := f.View().Content, "3baa\naaaa"; got != want {
		t.Errorf("View().Content = %q, want %q", got, want)
	}
}

// A handle names a surface in the instance that issued it, and nothing at all
// anywhere else — otherwise two instances that created the same number of
// surfaces would alias each other's tree.
func TestAHandleFromAnotherInstanceNamesNothing(t *testing.T) {
	a := newSized(t, &pane{title: "a"}, 4, 1)
	b := newSized(t, &pane{title: "b"}, 4, 1)

	if a.Root() == b.Root() {
		t.Fatal("two instances issued the same root handle")
	}
	if got := b.Composite(a.Root()); got.Content != "" || len(got.Rects) != 0 {
		t.Errorf("compositing another instance's handle = %+v, want an empty frame", got)
	}
}
