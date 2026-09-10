package froth_test

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/jbeda/froth"
)

// halves is a layout that splits its space down the middle. It is written here
// rather than taken from froth/layout so the froth tests exercise the interface
// a user implements, not one stock implementation of it.
type halves struct{ got []froth.Child }

func (h *halves) Arrange(space froth.Rect, children []froth.Child) []froth.Rect {
	h.got = children
	out := make([]froth.Rect, len(children))
	w := space.W / 2
	for i := range out {
		out[i] = froth.Rect{X: space.X + i*w, Y: space.Y, W: w, H: space.H}
	}
	return out
}

// laidOut is a container that attaches a layout to itself when it mounts, which
// is the self-relative form: it owns the children, so it says how they sit.
type laidOut struct {
	parent
	layout froth.Layout
}

func (l *laidOut) Mount(ac *froth.ApplyCtx) {
	l.parent.Mount(ac)
	ac.Self().SetLayout(l.layout)
}

func TestLayoutPlacesChildrenSideBySide(t *testing.T) {
	a, b := &block{ch: "a", w: 2, h: 1}, &block{ch: "b", w: 2, h: 1}
	root := &laidOut{
		parent: parent{body: "....\n....", kids: []froth.Surface{a, b}},
		layout: &halves{},
	}
	f := newSized(t, root, 4, 2)

	frame := f.Composite(f.Root())
	if got, want := frame.Content, "aabb\n...."; got != want {
		t.Errorf("View().Content = %q, want %q", got, want)
	}
	if got, want := frame.Rects[a.self], (froth.Rect{X: 0, Y: 0, W: 2, H: 2}); got != want {
		t.Errorf("first child rect = %+v, want %+v", got, want)
	}
	if got, want := frame.Rects[b.self], (froth.Rect{X: 2, Y: 0, W: 2, H: 2}); got != want {
		t.Errorf("second child rect = %+v, want %+v", got, want)
	}
}

// A layout is handed the container's rect, not the screen's: a laid-out
// container nested inside another arranges within what it was given.
func TestANestedLayoutArrangesWithinItsContainersRect(t *testing.T) {
	deep := &block{ch: "d", w: 1, h: 1}
	inner := &laidOut{
		parent: parent{body: "", kids: []froth.Surface{deep}},
		layout: &halves{},
	}
	root := &laidOut{
		parent: parent{body: "........", kids: []froth.Surface{&block{ch: ".", w: 1, h: 1}, inner}},
		layout: &halves{},
	}
	f := newSized(t, root, 8, 1)

	if got, want := f.Composite(f.Root()).Rects[deep.self], (froth.Rect{X: 4, Y: 0, W: 2, H: 1}); got != want {
		t.Errorf("grandchild rect = %+v, want %+v", got, want)
	}
}

// The layout sees handles, in sibling order. It never sees a surface, which is
// what lets it be tested as a pure function.
func TestALayoutSeesItsChildrenAsHandlesInSiblingOrder(t *testing.T) {
	a, b := &block{ch: "a", w: 1, h: 1}, &block{ch: "b", w: 1, h: 1}
	lay := &halves{}
	root := &laidOut{
		parent: parent{body: "..", kids: []froth.Surface{a, b}},
		layout: lay,
	}
	f := newSized(t, root, 4, 1)
	f.View()

	want := []froth.Child{{Handle: a.self}, {Handle: b.self}}
	if len(lay.got) != len(want) {
		t.Fatalf("layout saw %d children, want %d", len(lay.got), len(want))
	}
	for i := range want {
		if lay.got[i] != want[i] {
			t.Errorf("child %d = %+v, want %+v", i, lay.got[i], want[i])
		}
	}
}

// Without a layout the children stack at the container's whole rect, which is
// what a container that never calls SetLayout gets.
func TestChildrenStackAtTheContainersRectWithoutALayout(t *testing.T) {
	a, b := &block{ch: "a", w: 4, h: 1}, &block{ch: "b", w: 2, h: 1}
	root := &parent{body: "....", kids: []froth.Surface{a, b}}
	f := newSized(t, root, 4, 1)

	if got, want := f.View().Content, "bbaa"; got != want {
		t.Errorf("View().Content = %q, want %q", got, want)
	}
}

// shorted returns fewer rects than it was handed children, which no layout may
// do: a child with no rect has nowhere to render.
type shorted struct{}

func (shorted) Arrange(space froth.Rect, children []froth.Child) []froth.Rect {
	return nil
}

func TestALayoutReturningTheWrongNumberOfRectsPanics(t *testing.T) {
	root := &laidOut{
		parent: parent{body: "..", kids: []froth.Surface{&block{ch: "a", w: 1, h: 1}}},
		layout: shorted{},
	}
	f := newSized(t, root, 4, 1)

	var got any
	func() {
		defer func() { got = recover() }()
		f.View()
	}()

	if got == nil {
		t.Fatal("a layout returning too few rects did not panic")
	}
}

func TestASurfaceIsClippedToItsRect(t *testing.T) {
	root := &laidOut{
		parent: parent{body: "........\n........", kids: []froth.Surface{
			&block{ch: "a", w: 8, h: 2},
			&block{ch: "b", w: 8, h: 2},
		}},
		// The later sibling sits on the left, so its overflow runs into the
		// earlier one — which it would otherwise win, being above it.
		layout: at{{X: 4, Y: 0, W: 4, H: 2}, {X: 0, Y: 0, W: 4, H: 2}},
	}
	f := newSized(t, root, 8, 2)

	// Both children render eight columns wide into four-column rects. Neither
	// may bleed over its sibling.
	if got, want := f.View().Content, "bbbbaaaa\nbbbbaaaa"; got != want {
		t.Errorf("View().Content = %q, want %q", got, want)
	}
}

// at is a layout that places children at rects the test fixes, which is the
// shortest way to hand one surface an awkward rect.
type at []froth.Rect

func (a at) Arrange(space froth.Rect, children []froth.Child) []froth.Rect { return a }

func TestASurfaceIsClippedVertically(t *testing.T) {
	root := &laidOut{
		parent: parent{body: "", kids: []froth.Surface{&fixed{content: "11\n22\n33"}}},
		layout: at{{X: 0, Y: 0, W: 2, H: 2}},
	}
	f := newSized(t, root, 2, 3)

	// The third row is the child's, and it was not given room for it.
	if got, want := f.View().Content, "11\n22\n"; got != want {
		t.Errorf("View().Content = %q, want %q", got, want)
	}
}

// Clipping counts columns, not bytes: a wide grapheme that straddles the edge
// is dropped rather than half-drawn.
func TestClippingCountsColumnsNotBytes(t *testing.T) {
	root := &laidOut{
		parent: parent{body: "      ", kids: []froth.Surface{
			&fixed{content: "xxxxxx"},
			&fixed{content: "日本語"},
		}},
		layout: at{{X: 0, Y: 0, W: 6, H: 1}, {X: 3, Y: 0, W: 3, H: 1}},
	}
	f := newSized(t, root, 6, 1)

	// 日本 is four columns and the rect is three, so the second grapheme is
	// dropped whole rather than half-drawn over the sibling underneath.
	if got, want := f.View().Content, "xxx日x"; got != want {
		t.Errorf("View().Content = %q, want %q", got, want)
	}
}

// A layout can hand a child a rect with no cells — a row starves the children
// it cannot fit — and a layout with an arithmetic bug can hand it a negative
// extent. Compositing that subtree answers the zero Frame rather than sizing a
// canvas from the rect, which for a negative one panics inside the compositor.
func TestCompositingASubtreeWithNoCellsReturnsTheZeroFrame(t *testing.T) {
	for name, r := range map[string]froth.Rect{
		"empty":    {X: 0, Y: 0, W: 0, H: 1},
		"negative": {X: 0, Y: 0, W: -2, H: 1},
	} {
		t.Run(name, func(t *testing.T) {
			kid := &block{ch: "a", w: 2, h: 1}
			root := &laidOut{
				parent: parent{body: "..", kids: []froth.Surface{kid}},
				layout: at{r},
			}
			f := newSized(t, root, 2, 1)

			if got := f.Composite(kid.self); got.Content != "" || got.Rects != nil {
				t.Errorf("Composite of a subtree with no cells = %+v, want the zero Frame", got)
			}
			// The rest of the frame is unaffected: the root still draws.
			if got, want := f.View().Content, ".."; got != want {
				t.Errorf("View().Content = %q, want %q", got, want)
			}
		})
	}
}

// A layout can be a function: LayoutFunc adapts one for SetLayout.
func TestALayoutFuncArrangesChildren(t *testing.T) {
	a, b := &block{ch: "a", w: 1, h: 1}, &block{ch: "b", w: 1, h: 1}
	root := &laidOut{
		parent: parent{body: "....", kids: []froth.Surface{a, b}},
		layout: froth.LayoutFunc(func(space froth.Rect, children []froth.Child) []froth.Rect {
			out := make([]froth.Rect, len(children))
			for i := range out {
				out[i] = froth.Rect{X: space.X + space.W - 1 - i, Y: space.Y, W: 1, H: 1}
			}
			return out
		}),
	}
	f := newSized(t, root, 4, 1)

	if got, want := f.View().Content, "..ba"; got != want {
		t.Errorf("View().Content = %q, want %q", got, want)
	}
}

// countMsg tells a counted container how many children it should have.
type countMsg int

// counted is a container with no layout that keeps as many children as it was
// last told to, through Reconcile.
type counted struct {
	live []froth.Handle
	want int

	pending   froth.Layout
	setLayout bool
}

func (c *counted) Render(*froth.RenderCtx, froth.Size) string { return "...." }

func (c *counted) Update(uc *froth.UpdateCtx, msg tea.Msg) {
	if n, ok := msg.(countMsg); ok {
		c.want = int(n)
		uc.Self().Reconcile()
	}
	c.handleLayout(uc, msg)
}

func (c *counted) Mount(ac *froth.ApplyCtx) { c.Reconcile(ac) }

func (c *counted) Reconcile(ac *froth.ApplyCtx) {
	if c.setLayout {
		ac.Self().SetLayout(c.pending)
		c.setLayout = false
	}
	for len(c.live) > c.want {
		last := len(c.live) - 1
		ac.Child(c.live[last]).Destroy()
		c.live = c.live[:last]
	}
	for len(c.live) < c.want {
		c.live = append(c.live, ac.Self().Create(&block{ch: "x", w: 1, h: 1}))
	}
}

// A container with no layout and more than one child is reported, once: the
// frames that follow with the same children say nothing new. It is reported
// again once the condition has gone away and come back.
func TestSeveralChildrenWithNoLayoutAreReportedOncePerOccurrence(t *testing.T) {
	root := &counted{want: 2}
	f, diags := newWatched(t, root, 4, 1)

	f.View()
	f.View()
	if got := countKind(*diags, froth.DiagNoLayout); got != 1 {
		t.Fatalf("two frames reported %d no-layout diagnostics, want 1: %v", got, *diags)
	}

	f.Update(countMsg(1))
	f.View()
	if got := countKind(*diags, froth.DiagNoLayout); got != 1 {
		t.Fatalf("dropping to one child reported %d no-layout diagnostics, want still 1: %v", got, *diags)
	}

	f.Update(countMsg(3))
	f.View()
	f.View()
	if got := countKind(*diags, froth.DiagNoLayout); got != 2 {
		t.Fatalf("returning to several children reported %d no-layout diagnostics in all, want 2: %v", got, *diags)
	}
}

// One child and no layout is the ordinary case and is not reported.
func TestOneChildWithNoLayoutIsNotReported(t *testing.T) {
	root := &counted{want: 1}
	f, diags := newWatched(t, root, 4, 1)

	f.View()
	if hasKind(*diags, froth.DiagNoLayout) {
		t.Errorf("one child and no layout was reported: %v", *diags)
	}
}

// A float is placed against the screen, not by a layout, so it does not count
// towards the children a layout would have to arrange.
func TestAFloatDoesNotCountTowardsTheNoLayoutReport(t *testing.T) {
	root := &floatHost{body: "....", kids: []nestKid{
		{surface: &block{ch: "a", w: 1, h: 1}},
		floating(&block{ch: "f", w: 1, h: 1}),
	}}
	f, diags := newWatched(t, root, 4, 1)

	f.View()
	if hasKind(*diags, froth.DiagNoLayout) {
		t.Errorf("one arranged child and one float was reported: %v", *diags)
	}
}

// layoutMsg attaches or removes a layout on a counted container.
type layoutMsg struct{ layout froth.Layout }

func (c *counted) handleLayout(uc *froth.UpdateCtx, msg tea.Msg) {
	if m, ok := msg.(layoutMsg); ok {
		c.pending, c.setLayout = m.layout, true
		uc.Self().Reconcile()
	}
}

// Attaching a layout ends the condition as surely as losing a child does, so
// removing the layout again with several children still there is a fresh
// occurrence and is reported again.
func TestAttachingALayoutResetsTheNoLayoutReport(t *testing.T) {
	root := &counted{want: 2}
	f, diags := newWatched(t, root, 4, 1)

	f.View()
	f.Update(layoutMsg{layout: &halves{}})
	f.View()
	f.Update(layoutMsg{layout: nil})
	f.View()
	if got := countKind(*diags, froth.DiagNoLayout); got != 2 {
		t.Fatalf("removing a layout again reported %d no-layout diagnostics in all, want 2: %v", got, *diags)
	}
}
