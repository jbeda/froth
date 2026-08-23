package froth_test

import (
	"context"
	"strconv"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/jbeda/froth"
)

// pane is the minimal surface: two methods, state on a pointer receiver.
type pane struct {
	title  string
	count  int
	gotRec froth.Rect
}

func (p *pane) Render(rc *froth.RenderCtx, r froth.Rect) string {
	p.gotRec = r
	return p.title + ":" + strings.Repeat("*", p.count)
}

func (p *pane) Update(uc *froth.UpdateCtx, msg tea.Msg) {
	if _, ok := msg.(bumpMsg); ok {
		p.count++
	}
}

type bumpMsg struct{}

// newSized builds a froth instance and gives it a terminal size, the way Bubble
// Tea does on startup.
func newSized(t *testing.T, root froth.Surface, w, h int) *froth.Froth {
	t.Helper()
	f := froth.New(context.Background(), root)
	f.Update(tea.WindowSizeMsg{Width: w, Height: h})
	return f
}

func TestRendersRootSurface(t *testing.T) {
	f := newSized(t, &pane{title: "hello"}, 20, 3)

	// A frame is the whole terminal: three rows, the last two empty.
	if got, want := f.View().Content, "hello:\n\n"; got != want {
		t.Errorf("View().Content = %q, want %q", got, want)
	}
}

func TestPostedMessageReachesTheRoot(t *testing.T) {
	f := newSized(t, &pane{title: "n"}, 20, 3)

	f.Update(bumpMsg{})
	f.Update(bumpMsg{})

	if got, want := f.View().Content, "n:**\n\n"; got != want {
		t.Errorf("View().Content = %q, want %q", got, want)
	}
}

func TestRootRendersAtTheFullTerminalRect(t *testing.T) {
	p := &pane{title: "x"}
	f := newSized(t, p, 40, 12)
	f.View()

	if got, want := p.gotRec, (froth.Rect{X: 0, Y: 0, W: 40, H: 12}); got != want {
		t.Errorf("root rect = %+v, want %+v", got, want)
	}
}

func TestResizeChangesTheRectWithNoMessageToTheSurface(t *testing.T) {
	p := &pane{title: "x"}
	f := newSized(t, p, 40, 12)
	f.View()

	f.Update(tea.WindowSizeMsg{Width: 10, Height: 4})
	f.View()

	if got, want := p.gotRec, (froth.Rect{X: 0, Y: 0, W: 10, H: 4}); got != want {
		t.Errorf("root rect after resize = %+v, want %+v", got, want)
	}
	if p.count != 0 {
		t.Errorf("surface saw %d messages; a resize must not be delivered as one", p.count)
	}
}

func TestOutputIsClippedToTheTerminal(t *testing.T) {
	f := newSized(t, &fixed{content: "ABCDEFGH\n12345678\nWXYZ"}, 4, 2)

	if got, want := f.View().Content, "ABCD\n1234"; got != want {
		t.Errorf("View().Content = %q, want %q", got, want)
	}
}

type fixed struct{ content string }

func (x *fixed) Render(*froth.RenderCtx, froth.Rect) string { return x.content }
func (x *fixed) Update(*froth.UpdateCtx, tea.Msg)           {}

func TestViewIsEmptyBeforeTheFirstSize(t *testing.T) {
	f := froth.New(context.Background(), &pane{title: "hello"})

	if got := f.View().Content; got != "" {
		t.Errorf("View().Content before any size = %q, want empty", got)
	}
}

// The application model is copied by value on every Bubble Tea update; froth is
// held by pointer, so the copy shares one instance.
type appModel struct{ f *froth.Froth }

func (m appModel) Init() tea.Cmd  { return m.f.Init() }
func (m appModel) View() tea.View { return m.f.View() }
func (m appModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	return m, m.f.Update(msg)
}

func TestModelCopiedByValueSharesOneInstance(t *testing.T) {
	p := &pane{title: "n"}
	var m tea.Model = appModel{f: froth.New(context.Background(), p)}

	m, _ = m.Update(tea.WindowSizeMsg{Width: 20, Height: 3})
	m, _ = m.Update(bumpMsg{})
	m, _ = m.Update(bumpMsg{})

	if got, want := m.View().Content, "n:**\n\n"; got != want {
		t.Errorf("View().Content = %q, want %q", got, want)
	}
}

// A surface returns work by handing commands to its context.
type runner struct{ cmd tea.Cmd }

func (r *runner) Render(*froth.RenderCtx, froth.Rect) string { return "" }
func (r *runner) Update(uc *froth.UpdateCtx, msg tea.Msg) {
	if _, ok := msg.(bumpMsg); ok {
		uc.Run(r.cmd)
	}
}

func TestCommandsRunByASurfaceComeBackFromUpdate(t *testing.T) {
	want := bumpMsg{}
	f := newSized(t, &runner{cmd: func() tea.Msg { return want }}, 10, 2)

	cmd := f.Update(bumpMsg{})
	if cmd == nil {
		t.Fatal("Update returned no command")
	}
	if got := cmd(); got != tea.Msg(want) {
		t.Errorf("command produced %#v, want %#v", got, want)
	}
}

func TestUpdateReturnsNoCommandWhenNoneWasRun(t *testing.T) {
	f := newSized(t, &pane{title: "n"}, 10, 2)

	if cmd := f.Update(bumpMsg{}); cmd != nil {
		t.Errorf("Update returned a command, want nil")
	}
}

// A surface that stashes its context and uses it in a later phase is the one
// illegal thing the phase types cannot catch.
type stasher struct {
	uc  *froth.UpdateCtx
	got any
}

func (s *stasher) Render(rc *froth.RenderCtx, r froth.Rect) string {
	if s.uc != nil {
		func() {
			defer func() { s.got = recover() }()
			s.uc.Run(nil)
		}()
	}
	return ""
}

func (s *stasher) Update(uc *froth.UpdateCtx, msg tea.Msg) { s.uc = uc }

func TestRetainedContextPanicsWhenUsedInALaterPhase(t *testing.T) {
	s := &stasher{}
	f := newSized(t, s, 10, 2)

	f.Update(bumpMsg{})
	f.View()

	if s.got == nil {
		t.Fatal("using a retained context in a later phase did not panic")
	}
}

// Wide graphemes are the compositor's business: a string-assembled frame would
// count 日本 as four columns and clip in the wrong place.
func TestWideGraphemesOccupyTheirRealWidth(t *testing.T) {
	f := newSized(t, &fixed{content: "日本語"}, 4, 1)

	if got, want := f.View().Content, "日本"; got != want {
		t.Errorf("View().Content = %q, want %q", got, want)
	}
}

// reenterer calls back into its own froth instance, which nothing in the design
// permits.
type reenterer struct{ f *froth.Froth }

func (r *reenterer) Render(*froth.RenderCtx, froth.Rect) string { return "" }
func (r *reenterer) Update(uc *froth.UpdateCtx, msg tea.Msg)    { r.f.View() }

func TestCallingBackIntoFrothFromASurfacePanics(t *testing.T) {
	s := &reenterer{}
	f := newSized(t, s, 10, 2)
	s.f = f

	var got any
	func() {
		defer func() { got = recover() }()
		f.Update(bumpMsg{})
	}()

	if got == nil {
		t.Fatal("calling View from inside a surface's Update did not panic")
	}
}

// ---- Registry, handles and mount ----------------------------------------

// parent assembles its children when it enters the tree. Each child is a box of
// its own size, so a child drawn later is visible on top of one drawn earlier.
type parent struct {
	self    froth.Handle
	kids    []froth.Surface
	handles []froth.Handle
	mounts  int
	body    string
}

func (p *parent) Render(rc *froth.RenderCtx, r froth.Rect) string { return p.body }

func (p *parent) Update(uc *froth.UpdateCtx, msg tea.Msg) {}

func (p *parent) Mount(ac *froth.ApplyCtx) {
	p.mounts++
	p.self = ac.Self()
	for _, k := range p.kids {
		p.handles = append(p.handles, ac.Create(k))
	}
}

// block renders a solid rectangle of one character, which makes overlap
// obvious: whatever is on top wins every cell it covers.
type block struct {
	ch   string
	w, h int
	self froth.Handle
	rect froth.Rect
}

func (b *block) Render(rc *froth.RenderCtx, r froth.Rect) string {
	b.rect = r
	rows := make([]string, b.h)
	for i := range rows {
		rows[i] = strings.Repeat(b.ch, b.w)
	}
	return strings.Join(rows, "\n")
}

func (b *block) Update(uc *froth.UpdateCtx, msg tea.Msg) {}
func (b *block) Mount(ac *froth.ApplyCtx)                { b.self = ac.Self() }

func TestMountCreatesChildrenBeforeTheFirstRender(t *testing.T) {
	kid := &block{ch: "k", w: 2, h: 1}
	root := &parent{body: "rrrr\nrrrr", kids: []froth.Surface{kid}}
	f := newSized(t, root, 4, 2)

	if got, want := f.View().Content, "kkrr\nrrrr"; got != want {
		t.Errorf("View().Content = %q, want %q", got, want)
	}
}

func TestMountRunsOncePerSurfaceNotPerFrame(t *testing.T) {
	root := &parent{body: "x", kids: []froth.Surface{&block{ch: "k", w: 1, h: 1}}}
	f := newSized(t, root, 4, 2)

	f.View()
	f.Update(bumpMsg{})
	f.View()

	if root.mounts != 1 {
		t.Errorf("Mount ran %d times, want 1", root.mounts)
	}
	if got := len(root.handles); got != 1 {
		t.Errorf("root has %d children, want 1", got)
	}
}

func TestHandlesAreDistinctAndUsableAsMapKeys(t *testing.T) {
	a, b := &block{ch: "a", w: 1, h: 1}, &block{ch: "b", w: 1, h: 1}
	root := &parent{body: "..", kids: []froth.Surface{a, b}}
	f := newSized(t, root, 4, 2)
	f.View()

	seen := map[froth.Handle]string{f.Root(): "root", a.self: "a", b.self: "b"}
	if len(seen) != 3 {
		t.Errorf("three surfaces produced %d distinct handles: %v", len(seen), seen)
	}
	if a.self == (froth.Handle{}) {
		t.Error("a live surface has the zero handle, which names nothing")
	}
}

func TestLaterSiblingStacksAboveEarlier(t *testing.T) {
	root := &parent{body: "....\n....", kids: []froth.Surface{
		&block{ch: "a", w: 4, h: 2},
		&block{ch: "b", w: 2, h: 1},
	}}
	f := newSized(t, root, 4, 2)

	// b was created second, so it draws over a where they overlap.
	if got, want := f.View().Content, "bbaa\naaaa"; got != want {
		t.Errorf("View().Content = %q, want %q", got, want)
	}
}

// A subtree stacks as a unit: a deep descendant of an earlier sibling still
// draws below a later sibling, which is the per-parent stacking context ADR
// 0007 claims froth-assigned depth-first ordinals give.
func TestAnEarlierSiblingsSubtreeStaysBelowALaterSibling(t *testing.T) {
	deep := &parent{body: "aaaa\naaaa", kids: []froth.Surface{
		&parent{body: "cccc", kids: []froth.Surface{
			&block{ch: "d", w: 4, h: 1},
		}},
	}}
	root := &parent{body: "....\n....", kids: []froth.Surface{
		deep,
		&block{ch: "z", w: 2, h: 2},
	}}
	f := newSized(t, root, 4, 2)

	if got, want := f.View().Content, "zzdd\nzzaa"; got != want {
		t.Errorf("View().Content = %q, want %q", got, want)
	}
}

func TestChildrenRenderAtTheParentsRect(t *testing.T) {
	kid := &block{ch: "k", w: 1, h: 1}
	root := &parent{body: "x", kids: []froth.Surface{kid}}
	f := newSized(t, root, 8, 3)
	f.View()

	if got, want := kid.rect, (froth.Rect{W: 8, H: 3}); got != want {
		t.Errorf("child rect = %+v, want %+v", got, want)
	}
}

// ---- The harness: compositing a subtree ---------------------------------

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

// An ApplyCtx stays valid for the whole mount pass, so a surface can be handed
// one by another surface and create a child under a node the pass has already
// walked past. However a surface got into the tree, it mounts before it draws.
type stash struct{ ac **froth.ApplyCtx }

func (s *stash) Render(*froth.RenderCtx, froth.Rect) string { return ".." }
func (s *stash) Update(*froth.UpdateCtx, tea.Msg)           {}
func (s *stash) Mount(ac *froth.ApplyCtx)                   { *s.ac = ac }

type latecomer struct {
	ac  **froth.ApplyCtx
	kid froth.Surface
}

func (l *latecomer) Render(*froth.RenderCtx, froth.Rect) string { return "" }
func (l *latecomer) Update(*froth.UpdateCtx, tea.Msg)           {}
func (l *latecomer) Mount(*froth.ApplyCtx)                      { (*l.ac).Create(l.kid) }

func TestASurfaceCreatedLateInTheMountPassIsStillMounted(t *testing.T) {
	var borrowed *froth.ApplyCtx
	late := &block{ch: "L", w: 1, h: 1}
	root := &parent{body: "..", kids: []froth.Surface{
		&stash{ac: &borrowed},
		&latecomer{ac: &borrowed, kid: late},
	}}
	f := newSized(t, root, 2, 1)

	if got, want := f.View().Content, "L."; got != want {
		t.Errorf("View().Content = %q, want %q", got, want)
	}
	if late.self == (froth.Handle{}) {
		t.Error("a surface rendered without ever being mounted")
	}
}

// A context is handed to a surface, never constructed. The zero value is the
// shape a first unit test reaches for, so it says what is wrong rather than
// dereferencing nil.
func TestAConstructedContextPanicsWithADiagnostic(t *testing.T) {
	var got any
	func() {
		defer func() { got = recover() }()
		(&froth.RenderCtx{}).Rect()
	}()

	msg, _ := got.(string)
	if !strings.Contains(msg, "not handed out by froth") {
		t.Errorf("panic was %v, want one naming the constructed context", got)
	}
}

// ---- Layout ---------------------------------------------------------------

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
	ac.SetLayout(l.layout)
}

func TestLayoutPlacesChildrenSideBySide(t *testing.T) {
	a, b := &block{ch: "a", w: 2, h: 1}, &block{ch: "b", w: 2, h: 1}
	root := &laidOut{
		parent: parent{body: "....\n....", kids: []froth.Surface{a, b}},
		layout: &halves{},
	}
	f := newSized(t, root, 4, 2)

	if got, want := f.View().Content, "aabb\n...."; got != want {
		t.Errorf("View().Content = %q, want %q", got, want)
	}
	if got, want := a.rect, (froth.Rect{X: 0, Y: 0, W: 2, H: 2}); got != want {
		t.Errorf("first child rect = %+v, want %+v", got, want)
	}
	if got, want := b.rect, (froth.Rect{X: 2, Y: 0, W: 2, H: 2}); got != want {
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

	f.View()
	if got, want := deep.rect, (froth.Rect{X: 4, Y: 0, W: 2, H: 1}); got != want {
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
// what M0 shipped and what a container that never calls SetLayout still gets.
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

// ---- Clipping -------------------------------------------------------------

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
// extent. Compositing that subtree drew nothing, so it answers the zero Frame
// rather than sizing a canvas from the rect, which for a negative one used to
// panic inside the compositor.
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

// ---- The drain loop -------------------------------------------------------

// relay is the drain loop's fixture: it counts what it has seen, counts its own
// renders, and forwards each message to the next surface in a chain. One
// message therefore walks the whole chain within a single update, which is the
// claim the loop exists to make good on.
type relay struct {
	kids []froth.Surface
	next *relay // whose handle to forward to; nil ends the chain

	self    froth.Handle
	seen    int
	renders int
}

func (r *relay) Render(rc *froth.RenderCtx, rect froth.Rect) string {
	r.renders++
	return strconv.Itoa(r.seen)
}

func (r *relay) Update(uc *froth.UpdateCtx, msg tea.Msg) {
	if _, ok := msg.(bumpMsg); !ok {
		return
	}
	r.seen++
	if r.next != nil {
		uc.Post(r.next.self, msg)
	}
}

func (r *relay) Mount(ac *froth.ApplyCtx) {
	r.self = ac.Self()
	for _, k := range r.kids {
		ac.Create(k)
	}
}

// newWatched is newSized for the tests that need to see what froth reported.
func newWatched(t *testing.T, root froth.Surface, w, h int) (*froth.Froth, *[]froth.Diagnostic) {
	t.Helper()
	var got []froth.Diagnostic
	f := froth.New(context.Background(), root,
		froth.OnDiagnostic(func(d froth.Diagnostic) { got = append(got, d) }))
	f.Update(tea.WindowSizeMsg{Width: w, Height: h})
	return f, &got
}

// cycler answers every message with the same message, addressed to itself, which
// is the shortest cascade that never settles.
type cycler struct{}

func (c *cycler) Render(*froth.RenderCtx, froth.Rect) string { return "cycling" }
func (c *cycler) Update(uc *froth.UpdateCtx, msg tea.Msg)    { uc.Post(uc.Self(), msg) }

func TestAPostedMessageIsHandledInTheSameFrame(t *testing.T) {
	tail := &relay{}
	root := &relay{next: tail, kids: []froth.Surface{tail}}
	f := newSized(t, root, 4, 1)

	// One update, two deliveries: the root's, and the one it posted. The tail
	// draws over the root, so what the frame shows is the tail's count.
	f.Update(bumpMsg{})

	if got, want := f.View().Content, "1"; got != want {
		t.Errorf("View().Content = %q, want %q — the posted message was not "+
			"handled before the frame was drawn", got, want)
	}
}

// A chain of four settles in one update: nothing waits for the next message
// from Bubble Tea, so no partial state is ever rendered.
func TestAPostCascadeSettlesWithinOneUpdate(t *testing.T) {
	d := &relay{}
	c := &relay{next: d}
	b := &relay{next: c}
	root := &relay{next: b, kids: []froth.Surface{b, c, d}}
	f := newSized(t, root, 4, 1)

	f.Update(bumpMsg{})

	if got, want := f.View().Content, "1"; got != want {
		t.Errorf("View().Content = %q, want %q — the end of the chain never saw "+
			"the message", got, want)
	}
	if got := f.Composite(b.self).Content; got != "1" {
		t.Errorf("middle of the chain rendered %q, want %q", got, "1")
	}
}

func TestASurfaceCanPostToItself(t *testing.T) {
	root := &bouncer{}
	f := newSized(t, root, 4, 1)

	f.Update(bumpMsg{})

	// Two deliveries, one update: the one from Bubble Tea and the one the
	// surface addressed to itself.
	if got, want := f.View().Content, "2"; got != want {
		t.Errorf("View().Content = %q, want %q", got, want)
	}
}

// bouncer posts one message back to itself, which is the shortest thing Self
// is for.
type bouncer struct{ seen int }

func (b *bouncer) Render(*froth.RenderCtx, froth.Rect) string { return strconv.Itoa(b.seen) }

func (b *bouncer) Update(uc *froth.UpdateCtx, msg tea.Msg) {
	b.seen++
	if b.seen == 1 {
		uc.Post(uc.Self(), msg)
	}
}

func TestNothingRendersWhileMessagesAreBeingDelivered(t *testing.T) {
	kid := &relay{}
	root := &relay{next: kid, kids: []froth.Surface{kid}}
	f := newSized(t, root, 4, 1)

	f.Update(bumpMsg{})

	if root.renders != 0 || kid.renders != 0 {
		t.Errorf("surfaces rendered %d and %d times during a cascade of "+
			"deliveries, want 0 — Render runs once the queue has drained, not "+
			"per message", root.renders, kid.renders)
	}
}

func TestEverySurfaceRendersExactlyOncePerFrame(t *testing.T) {
	kid := &relay{}
	root := &relay{next: kid, kids: []froth.Surface{kid}}
	f := newSized(t, root, 4, 1)

	// An update that cascades is the case the drain loop introduces: several
	// deliveries, one frame.
	f.Update(bumpMsg{})
	f.View()

	if root.renders != 1 || kid.renders != 1 {
		t.Errorf("one frame rendered the root %d times and its child %d times, "+
			"want 1 each", root.renders, kid.renders)
	}
}

// A message cycle is a bug, and the loop's job is to make it a report rather
// than a freeze.
func TestAMessageCycleTripsTheIterationCapAndIsReported(t *testing.T) {
	a, b := &relay{}, &relay{}
	a.next, b.next = b, a
	root := &relay{next: a, kids: []froth.Surface{a, b}}

	f, got := newWatched(t, root, 4, 1)

	f.Update(bumpMsg{}) // returns rather than hanging

	if len(*got) != 1 {
		t.Fatalf("a message cycle reported %d diagnostics, want 1: %v", len(*got), *got)
	}
	if (*got)[0].Kind != froth.DiagIterationCap {
		t.Errorf("diagnostic kind = %q, want %q", (*got)[0].Kind, froth.DiagIterationCap)
	}
	if (*got)[0].Detail == "" {
		t.Error("the diagnostic carried no detail saying what did not settle")
	}
}

// Tripping the cap abandons the cascade; it does not wedge the instance.
func TestFrothStillDrawsAfterTheCapTrips(t *testing.T) {
	f, _ := newWatched(t, &cycler{}, 8, 1)

	f.Update(bumpMsg{})

	if got := f.View().Content; got == "" {
		t.Error("nothing rendered after the cap tripped")
	}
	// The queue was abandoned rather than left in place, so the next update
	// starts from a drained queue and trips on its own cascade, not the old one.
	f.Update(bumpMsg{})
	if got := f.View().Content; got == "" {
		t.Error("nothing rendered on the frame after the cap tripped")
	}
}

// A surface that mounts a child of its own type never settles either, and it
// spends the same budget as delivery does.
func TestAMountCascadeTripsTheIterationCapAndIsReported(t *testing.T) {
	_, got := newWatched(t, &spawner{}, 4, 1)

	if len(*got) != 1 {
		t.Fatalf("a mount cascade reported %d diagnostics, want 1: %v", len(*got), *got)
	}
	if (*got)[0].Kind != froth.DiagIterationCap {
		t.Errorf("diagnostic kind = %q, want %q", (*got)[0].Kind, froth.DiagIterationCap)
	}
}

type spawner struct{}

func (s *spawner) Render(*froth.RenderCtx, froth.Rect) string { return "" }
func (s *spawner) Update(*froth.UpdateCtx, tea.Msg)           {}
func (s *spawner) Mount(ac *froth.ApplyCtx)                   { ac.Create(&spawner{}) }

// A handle outlives the surface it names, so addressing one that names nothing
// is a defined drop rather than an error. M5 counts them.
func TestAPostToAHandleThatNamesNothingIsDropped(t *testing.T) {
	f := newSized(t, &dropper{}, 4, 1)

	f.Update(bumpMsg{})

	if got, want := f.View().Content, "ok"; got != want {
		t.Errorf("View().Content = %q, want %q", got, want)
	}
}

type dropper struct{}

func (d *dropper) Render(*froth.RenderCtx, froth.Rect) string { return "ok" }
func (d *dropper) Update(uc *froth.UpdateCtx, msg tea.Msg)    { uc.Post(froth.Handle{}, msg) }

// The hook is application code, so froth calls it with no phase in progress —
// otherwise a hook that looked at the tree would trip the re-entrancy check
// instead of reporting what it was called about.
func TestTheDiagnosticHookCanCallBackIntoFroth(t *testing.T) {
	var content string
	var f *froth.Froth
	f = froth.New(context.Background(), &cycler{},
		froth.OnDiagnostic(func(froth.Diagnostic) { content = f.View().Content }))
	f.Update(tea.WindowSizeMsg{Width: 8, Height: 1})

	f.Update(bumpMsg{})

	if content == "" {
		t.Error("the hook could not render from inside itself")
	}
}

// A hook is application code, and application code may drive froth. A hook that
// provokes the very diagnostic it is handling would otherwise never return —
// each report calling the hook that causes the next one. froth bounds the
// rounds and drops what is left rather than recursing.
func TestAHookThatProvokesItsOwnDiagnosticTerminates(t *testing.T) {
	calls := 0
	var f *froth.Froth
	f = froth.New(context.Background(), &cycler{},
		froth.OnDiagnostic(func(froth.Diagnostic) {
			calls++
			f.Update(bumpMsg{}) // trips the cap again, from inside the report of it
		}))
	f.Update(tea.WindowSizeMsg{Width: 8, Height: 1})

	f.Update(bumpMsg{})

	if calls == 0 {
		t.Fatal("the hook never fired")
	}
	if got, want := f.View().Content, "cycling"; got != want {
		t.Errorf("View().Content = %q, want %q; froth stopped drawing", got, want)
	}
}

// ---- Focus and raw key delivery ---------------------------------------------

// focusMsg asks the root to move focus somewhere, and selfFocusMsg asks the
// surface named to focus itself. They are ordinary messages rather than a test
// hook: nothing in M1 moves focus except a surface asking froth to, so a test
// moves focus by telling a surface to ask.
type focusMsg struct{ to froth.Handle }

type selfFocusMsg struct{ to froth.Handle }

// focusRoot is the container the focus tests hang off: it splits its space
// between its children and does what it is told about focus. It draws nothing,
// so a frame is exactly what its children drew.
type focusRoot struct {
	parent
	layout froth.Layout
}

func (r *focusRoot) Mount(ac *froth.ApplyCtx) {
	r.parent.Mount(ac)
	ac.SetLayout(r.layout)
}

func (r *focusRoot) Update(uc *froth.UpdateCtx, msg tea.Msg) {
	switch msg := msg.(type) {
	case focusMsg:
		uc.Focus(msg.to)
	case selfFocusMsg:
		uc.Post(msg.to, selfFocusMsg{})
	}
}

// focusable draws one letter, upper case while it holds focus, and moves focus
// on to the surface it was told about when it sees a key. In M1 the surface
// holding the key is the only thing that can move focus; tab navigation is
// froth's own job from M2.
type focusable struct {
	letter string
	toward *froth.Handle // where a key sends focus; nil to keep it

	keys  []string
	bumps int
}

func (x *focusable) Render(rc *froth.RenderCtx, r froth.Rect) string {
	if rc.Focused() {
		return strings.ToUpper(x.letter)
	}
	return x.letter
}

func (x *focusable) Update(uc *froth.UpdateCtx, msg tea.Msg) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		x.keys = append(x.keys, msg.String())
		if x.toward != nil {
			uc.Focus(*x.toward)
		}
	case selfFocusMsg:
		uc.FocusSelf()
	case bumpMsg:
		x.bumps++
	}
}

// twoPanes is the shape the milestone demo has: two surfaces side by side that
// hand focus to each other. It settles the mounts before returning, so the
// handles are real by the time a test names one.
func twoPanes(t *testing.T) (f *froth.Froth, root *focusRoot, left, right *focusable) {
	t.Helper()
	left, right = &focusable{letter: "l"}, &focusable{letter: "r"}
	root = &focusRoot{
		parent: parent{body: "  ", kids: []froth.Surface{left, right}},
		layout: &halves{},
	}
	f = newSized(t, root, 2, 1)
	f.View()
	left.toward, right.toward = &root.handles[1], &root.handles[0]
	return f, root, left, right
}

func keyPress(code rune) tea.KeyPressMsg { return tea.KeyPressMsg{Code: code} }

// Nothing has moved focus, so the root holds it: an application that never
// mentions focus still receives its keys somewhere.
func TestTheRootHoldsFocusUntilSomethingMovesIt(t *testing.T) {
	root := &focusable{letter: "x"}
	f := newSized(t, root, 1, 1)

	f.Update(keyPress('a'))

	if got, want := strings.Join(root.keys, ","), "a"; got != want {
		t.Errorf("the root saw %q, want %q", got, want)
	}
	if got, want := f.View().Content, "X"; got != want {
		t.Errorf("View().Content = %q, want %q", got, want)
	}
}

func TestARawKeyReachesTheFocusedSurfaceAndNoOther(t *testing.T) {
	f, root, left, right := twoPanes(t)

	// The root still holds focus, so neither pane sees this one.
	f.Update(keyPress('a'))
	if len(left.keys) != 0 || len(right.keys) != 0 {
		t.Fatalf("a key reached a pane before either held focus: left %v, right %v",
			left.keys, right.keys)
	}

	f.Update(focusMsg{to: root.handles[0]})
	f.Update(keyPress('b'))

	if got, want := strings.Join(left.keys, ","), "b"; got != want {
		t.Errorf("the focused surface saw %q, want %q", got, want)
	}
	if len(right.keys) != 0 {
		t.Errorf("an unfocused surface saw %v, want nothing", right.keys)
	}
}

func TestASurfaceReadsItsOwnFocusWhileRendering(t *testing.T) {
	f, root, _, _ := twoPanes(t)

	if got, want := f.View().Content, "lr"; got != want {
		t.Errorf("with the root focused, View().Content = %q, want %q", got, want)
	}

	f.Update(focusMsg{to: root.handles[0]})

	if got, want := f.View().Content, "Lr"; got != want {
		t.Errorf("with the left pane focused, View().Content = %q, want %q", got, want)
	}
}

// The acceptance demo through the seam: a key moves focus and the composited
// output changes to match. The move commits within the update that carried the
// key, because focus commits in Apply and nothing is drawn until the queue has
// drained.
func TestAKeyMovesFocusAndTheFrameFollows(t *testing.T) {
	f, root, left, right := twoPanes(t)
	f.Update(focusMsg{to: root.handles[0]})

	f.Update(keyPress('\t'))
	if got, want := f.View().Content, "lR"; got != want {
		t.Errorf("after one key, View().Content = %q, want %q", got, want)
	}

	f.Update(keyPress('\t'))
	if got, want := f.View().Content, "Lr"; got != want {
		t.Errorf("after two keys, View().Content = %q, want %q", got, want)
	}

	// Each key was handled by the pane that held focus when it arrived, not by
	// the one focus landed on: the move happens after the delivery, not during.
	if got, want := len(left.keys), 1; got != want {
		t.Errorf("the left pane saw %d keys, want %d", got, want)
	}
	if got, want := len(right.keys), 1; got != want {
		t.Errorf("the right pane saw %d keys, want %d", got, want)
	}
}

// Only keys are routed by focus. Everything else still arrives at the root,
// which is where an application's own messages belong until something claims
// them — mouse messages are hit-tested against the rendered frame in M4.
func TestANonKeyMessageStillGoesToTheRoot(t *testing.T) {
	f, root, left, right := twoPanes(t)
	f.Update(focusMsg{to: root.handles[0]})

	f.Update(bumpMsg{})

	if left.bumps != 0 || right.bumps != 0 {
		t.Errorf("a non-key message reached a pane: left %d, right %d",
			left.bumps, right.bumps)
	}
}

func TestFocusingAHandleThatIsNotInTheTreeIsRejectedAndReported(t *testing.T) {
	elsewhere := froth.New(context.Background(), &pane{title: "elsewhere"})

	left, right := &focusable{letter: "l"}, &focusable{letter: "r"}
	root := &focusRoot{
		parent: parent{body: "  ", kids: []froth.Surface{left, right}},
		layout: &halves{},
	}
	f, diags := newWatched(t, root, 2, 1)
	f.View()
	f.Update(focusMsg{to: root.handles[0]})

	// A handle from another instance names nothing here, which is the same
	// rejection as a handle whose surface has gone.
	f.Update(focusMsg{to: elsewhere.Root()})

	if len(*diags) != 1 {
		t.Fatalf("froth reported %d diagnostics, want 1: %v", len(*diags), *diags)
	}
	if got, want := (*diags)[0].Kind, froth.DiagInvalidFocus; got != want {
		t.Errorf("diagnostic kind = %q, want %q", got, want)
	}
	if got, want := f.View().Content, "Lr"; got != want {
		t.Errorf("focus moved anyway: View().Content = %q, want %q", got, want)
	}
}

// The zero handle names nothing, so it is rejected like any other target that
// is not in the tree. It is worth its own test because "no pending move" and
// "a pending move to nothing" would be the same value if froth recorded the
// pending move as a handle alone, and the second would then be swallowed.
func TestFocusingTheZeroHandleFromDeliverIsRejectedAndReported(t *testing.T) {
	left, right := &focusable{letter: "l"}, &focusable{letter: "r"}
	root := &focusRoot{
		parent: parent{body: "  ", kids: []froth.Surface{left, right}},
		layout: &halves{},
	}
	f, diags := newWatched(t, root, 2, 1)
	f.View()
	f.Update(focusMsg{to: root.handles[0]})

	f.Update(focusMsg{to: froth.Handle{}})

	if len(*diags) != 1 || (*diags)[0].Kind != froth.DiagInvalidFocus {
		t.Fatalf("froth reported %v, want one %q", *diags, froth.DiagInvalidFocus)
	}
	if got, want := f.View().Content, "Lr"; got != want {
		t.Errorf("focus moved anyway: View().Content = %q, want %q", got, want)
	}
}

// Apply rejects an invalid target the same way, though there it takes effect
// immediately rather than being recorded.
func TestFocusingAnInvalidHandleFromApplyIsRejectedAndReported(t *testing.T) {
	elsewhere := froth.New(context.Background(), &pane{title: "elsewhere"})
	f, diags := newWatched(t, &focuserOnMount{to: elsewhere.Root()}, 1, 1)
	f.View()

	if len(*diags) != 1 || (*diags)[0].Kind != froth.DiagInvalidFocus {
		t.Fatalf("froth reported %v, want one %q", *diags, froth.DiagInvalidFocus)
	}
	if got, want := f.View().Content, "X"; got != want {
		t.Errorf("View().Content = %q, want %q — the root should still hold focus", got, want)
	}
}

// FocusSelf names no target, so it cannot fail: a surface handling a message is
// in the tree by construction.
func TestASurfaceCanFocusItself(t *testing.T) {
	f, root, _, _ := twoPanes(t)

	f.Update(selfFocusMsg{to: root.handles[1]})

	if got, want := f.View().Content, "lR"; got != want {
		t.Errorf("View().Content = %q, want %q", got, want)
	}
}

// A container mounting its children says which of them starts focused, and the
// first frame already shows it — Mount runs before anything is drawn.
func TestAContainerCanFocusAChildAsItMounts(t *testing.T) {
	left, right := &focusable{letter: "l"}, &focusable{letter: "r"}
	root := &focusingParent{
		focusRoot: focusRoot{
			parent: parent{body: "  ", kids: []froth.Surface{left, right}},
			layout: &halves{},
		},
		which: 1,
	}
	f := newSized(t, root, 2, 1)

	if got, want := f.View().Content, "lR"; got != want {
		t.Errorf("View().Content = %q, want %q", got, want)
	}
}

// focuserOnMount focuses a handle from Apply, where the move commits at once.
type focuserOnMount struct{ to froth.Handle }

func (x *focuserOnMount) Render(rc *froth.RenderCtx, r froth.Rect) string {
	if rc.Focused() {
		return "X"
	}
	return "x"
}
func (x *focuserOnMount) Update(*froth.UpdateCtx, tea.Msg) {}
func (x *focuserOnMount) Mount(ac *froth.ApplyCtx)         { ac.Focus(x.to) }

// focusingParent mounts its children and focuses one of them, which is what a
// container owning a starting selection does.
type focusingParent struct {
	focusRoot
	which int
}

func (p *focusingParent) Mount(ac *froth.ApplyCtx) {
	p.focusRoot.Mount(ac)
	ac.Focus(p.handles[p.which])
}
