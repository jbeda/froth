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

	// Several deliveries, one frame.
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
// is a defined drop rather than an error.
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
// hook: focus only moves when a surface asks froth to move it, so a test moves
// focus by telling a surface to ask.
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
// on to the surface it was told about when it sees a key.
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

// twoPanes builds two surfaces side by side that hand focus to each other. It
// settles the mounts before returning, so the handles are real by the time a
// test names one.
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

// Only keys are routed by focus. Everything else arrives at the root, which is
// where an application's own messages belong.
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

// ---- Create options and keyboard traversal ----------------------------------

// tabMsg asks froth for a relative focus move, the way an application does: it
// tells a surface, and the surface asks froth. times is how many moves one
// delivery asks for, which is the only way to get two relative moves without an
// Apply between them.
type tabMsg struct {
	back  bool
	times int
}

// cells gives every child a one-cell column, left to right, so a frame is one
// character per surface and "who has focus" is readable as a string.
type cells struct{}

func (cells) Arrange(space froth.Rect, children []froth.Child) []froth.Rect {
	out := make([]froth.Rect, len(children))
	for i := range out {
		out[i] = froth.Rect{X: space.X + i, Y: space.Y, W: 1, H: space.H}
	}
	return out
}

// optRoot creates each of its children with the option list it was given for
// that child, and asks froth to move focus when told to. It is the fixture for
// everything about create options: an option is only ever passed at creation,
// so a test that wants one has to own the surface that creates.
type optRoot struct {
	kids    []froth.Surface
	opts    [][]froth.CreateOption
	handles []froth.Handle
}

func (r *optRoot) Render(rc *froth.RenderCtx, rect froth.Rect) string {
	return strings.Repeat(".", rect.W)
}

func (r *optRoot) Update(uc *froth.UpdateCtx, msg tea.Msg) {
	switch m := msg.(type) {
	case tabMsg:
		for i := 0; i < max(m.times, 1); i++ {
			if m.back {
				uc.FocusPrev()
			} else {
				uc.FocusNext()
			}
		}
	case focusMsg:
		uc.Focus(m.to)
	}
}

func (r *optRoot) Mount(ac *froth.ApplyCtx) {
	for i, k := range r.kids {
		var opts []froth.CreateOption
		if i < len(r.opts) {
			opts = r.opts[i]
		}
		r.handles = append(r.handles, ac.Create(k, opts...))
	}
	ac.SetLayout(cells{})
}

// tabRow builds a row of one-cell surfaces, each created with the options given
// for it. The letters are lower case unless the surface holds focus, so the
// frame reads as "l m r" with the focused one shouting.
func tabRow(t *testing.T, letters string, opts ...[]froth.CreateOption) (*froth.Froth, *optRoot) {
	t.Helper()
	root := &optRoot{opts: opts}
	for _, c := range letters {
		root.kids = append(root.kids, &focusable{letter: string(c)})
	}
	f := newSized(t, root, len(letters), 1)
	f.View()
	return f, root
}

// stop is the option list for a surface traversal should land on. Traversal is
// opt-in, so a test that wants a stop has to say so.
var stop = []froth.CreateOption{froth.TabStop()}

func TestTraversalMovesFocusInChildOrderAndWraps(t *testing.T) {
	f, _ := tabRow(t, "lmr", stop, stop, stop)

	// The root holds focus and is not a stop, so the first move lands on the
	// first child rather than stepping off the root onto itself.
	want := []string{"Lmr", "lMr", "lmR", "Lmr"}
	for i, w := range want {
		f.Update(tabMsg{})
		if got := f.View().Content; got != w {
			t.Errorf("after %d moves, View().Content = %q, want %q", i+1, got, w)
		}
	}
}

func TestTraversalBackwardsWalksTheOtherWayAndWraps(t *testing.T) {
	f, _ := tabRow(t, "lmr", stop, stop, stop)

	// Backwards from the root wraps to the last stop, which is what a
	// shift-tab from a fresh application should reach.
	want := []string{"lmR", "lMr", "Lmr", "lmR"}
	for i, w := range want {
		f.Update(tabMsg{back: true})
		if got := f.View().Content; got != w {
			t.Errorf("after %d backward moves, View().Content = %q, want %q", i+1, got, w)
		}
	}
}

func TestASurfaceThatIsNotATabStopIsPassedOver(t *testing.T) {
	f, _ := tabRow(t, "lmr", stop, nil, stop)

	f.Update(tabMsg{})
	if got, want := f.View().Content, "Lmr"; got != want {
		t.Errorf("first move: View().Content = %q, want %q", got, want)
	}

	// The middle surface is skipped rather than reordered: the move goes
	// straight past it to the one after it.
	f.Update(tabMsg{})
	if got, want := f.View().Content, "lmR"; got != want {
		t.Errorf("second move: View().Content = %q, want %q", got, want)
	}
}

// Being outside traversal is not being reordered, and not being hidden: the
// surface is still where its parent's layout put it.
func TestASurfaceOutsideTraversalKeepsItsPosition(t *testing.T) {
	f, root := tabRow(t, "lmr", stop, nil, stop)

	rects := f.Composite(f.Root()).Rects
	if got, want := rects[root.handles[1]], (froth.Rect{X: 1, Y: 0, W: 1, H: 1}); got != want {
		t.Errorf("the skipped surface sits at %+v, want %+v", got, want)
	}
	if got, want := f.View().Content, "lmr"; got != want {
		t.Errorf("View().Content = %q, want %q — it should still draw", got, want)
	}
}

// TabStop says where the keyboard walks, not who may hold focus. A surface a
// container focuses by name still gets it.
func TestASurfaceOutsideTraversalCanStillBeFocusedByName(t *testing.T) {
	f, root := tabRow(t, "lmr", stop, nil, stop)

	f.Update(focusMsg{to: root.handles[1]})

	if got, want := f.View().Content, "lMr"; got != want {
		t.Errorf("View().Content = %q, want %q", got, want)
	}
}

// A container never has to be a stop to keep its contents reachable: its
// children are walked whether or not traversal can land on it.
func TestChildrenOfANonStopSurfaceAreStillTraversed(t *testing.T) {
	inner := &focusable{letter: "i"}
	wrapper := &optRoot{kids: []froth.Surface{inner}, opts: [][]froth.CreateOption{stop}}
	root := &optRoot{kids: []froth.Surface{wrapper}}
	f := newSized(t, root, 1, 1)
	f.View()

	f.Update(tabMsg{})

	if got, want := f.View().Content, "I"; got != want {
		t.Errorf("View().Content = %q, want %q", got, want)
	}
}

// Every option sets a flag rather than toggling one, so a caller that passes the
// same thing twice — or builds an option list in a loop that repeats itself —
// gets what it asked for once.
func TestPassingTheSameOptionTwiceSaysWhatPassingItOnceSaid(t *testing.T) {
	f, _ := tabRow(t, "lm",
		[]froth.CreateOption{froth.TabStop(), froth.TabStop()},
		stop)

	f.Update(tabMsg{})
	f.Update(tabMsg{})

	if got, want := f.View().Content, "lM"; got != want {
		t.Errorf("View().Content = %q, want %q", got, want)
	}
}

// Traversal is opt-in, so a surface whose creator said nothing about it is not
// a stop. An empty list and a nil list say the same nothing.
func TestASurfaceCreatedWithNoOptionsIsNotATabStop(t *testing.T) {
	f, _ := tabRow(t, "lm", []froth.CreateOption{}, stop)

	f.Update(tabMsg{})
	f.Update(tabMsg{})

	if got, want := f.View().Content, "lM"; got != want {
		t.Errorf("View().Content = %q, want %q — the first surface is not a stop", got, want)
	}
}

// A tree with nowhere to go is not an error. Nothing moves, nothing is
// reported, and nothing hangs looking for a stop that is not there.
func TestTraversalWithNoStopAtAllLeavesFocusWhereItIs(t *testing.T) {
	root := &optRoot{
		kids: []froth.Surface{&focusable{letter: "a"}, &focusable{letter: "b"}},
	}
	var got []froth.Diagnostic
	f := froth.New(context.Background(), root,
		froth.OnDiagnostic(func(d froth.Diagnostic) { got = append(got, d) }))
	f.Update(tea.WindowSizeMsg{Width: 2, Height: 1})

	f.Update(tabMsg{})

	if content, want := f.View().Content, "ab"; content != want {
		t.Errorf("View().Content = %q, want %q — focus should not have moved", content, want)
	}
	if len(got) != 0 {
		t.Errorf("froth reported %v; having nowhere to tab to is not a diagnostic", got)
	}
}

// A relative move counts from the move already pending, not from the focus that
// has yet to be replaced, so two moves in one delivery advance two stops.
func TestTwoMovesInOneDeliveryAdvanceTwoStops(t *testing.T) {
	f, _ := tabRow(t, "lmr", stop, stop, stop)

	f.Update(tabMsg{times: 2})

	if got, want := f.View().Content, "lMr"; got != want {
		t.Errorf("View().Content = %q, want %q", got, want)
	}
}

// Bindings and actions
//
// speaker declares whatever bindings it was given and draws the last thing that
// reached it — an action's identity, or a raw key — falling back to its own
// letter until something does. Every action and key in these tests is one
// character wide, so a frame reads as "who got what".
type speaker struct {
	letter   string
	bindings []froth.Binding
	last     string
}

func (s *speaker) Bindings() []froth.Binding { return s.bindings }

func (s *speaker) Render(rc *froth.RenderCtx, r froth.Rect) string {
	if s.last != "" {
		return s.last
	}
	return s.letter
}

func (s *speaker) Update(uc *froth.UpdateCtx, msg tea.Msg) {
	switch m := msg.(type) {
	case froth.ActionMsg:
		s.last = string(m.Action)
	case tea.KeyMsg:
		s.last = m.String()
	}
}

// under puts each child in a one-cell column on the row below its parent's,
// leaving the parent's own first row to draw for itself.
type under struct{}

func (under) Arrange(space froth.Rect, children []froth.Child) []froth.Rect {
	out := make([]froth.Rect, len(children))
	for i := range out {
		out[i] = froth.Rect{X: space.X + i, Y: space.Y + 1, W: 1, H: 1}
	}
	return out
}

// bindRoot is the container the binding tests hang off. It speaks on the top
// row and its children speak on the row below, so one frame says which of them
// a key reached.
type bindRoot struct {
	speaker
	kids    []froth.Surface
	opts    [][]froth.CreateOption
	handles []froth.Handle
}

func (r *bindRoot) Mount(ac *froth.ApplyCtx) {
	for i, k := range r.kids {
		var opts []froth.CreateOption
		if i < len(r.opts) {
			opts = r.opts[i]
		}
		r.handles = append(r.handles, ac.Create(k, opts...))
	}
	ac.SetLayout(under{})
}

func (r *bindRoot) Render(rc *froth.RenderCtx, rect froth.Rect) string {
	return r.speaker.Render(rc, rect) + strings.Repeat(".", rect.W-1) +
		"\n" + strings.Repeat(".", rect.W)
}

func (r *bindRoot) Update(uc *froth.UpdateCtx, msg tea.Msg) {
	if m, ok := msg.(focusMsg); ok {
		uc.Focus(m.to)
		return
	}
	r.speaker.Update(uc, msg)
}

// binds is one binding on one key, which is all any of these tests needs.
func binds(k string, a froth.Action) []froth.Binding {
	return []froth.Binding{{Action: a, Keys: []string{k}, Help: string(a)}}
}

// bindTree stands a root over its children and settles the mounts, so the
// handles are real by the time a test focuses one.
func bindTree(t *testing.T, root *bindRoot) *froth.Froth {
	t.Helper()
	f := newSized(t, root, len(root.kids), 2)
	f.View()
	return f
}

func TestAnAncestorsBindingFiresWhileAChildHasFocus(t *testing.T) {
	left, right := &speaker{letter: "l"}, &speaker{letter: "r"}
	root := &bindRoot{
		speaker: speaker{letter: ".", bindings: binds("q", "Q")},
		kids:    []froth.Surface{left, right},
	}
	f := bindTree(t, root)
	f.Update(focusMsg{to: root.handles[0]})

	f.Update(keyPress('q'))

	if got, want := f.View().Content, "Q.\nlr"; got != want {
		t.Errorf("View().Content = %q, want %q: the surface that declared the "+
			"binding gets the action, not the one that had focus", got, want)
	}
}

func TestADeeperBindingBeatsAnAncestorsForTheSameKey(t *testing.T) {
	left, right := &speaker{letter: "l", bindings: binds("x", "L")}, &speaker{letter: "r"}
	root := &bindRoot{
		speaker: speaker{letter: ".", bindings: binds("x", "R")},
		kids:    []froth.Surface{left, right},
	}
	f := bindTree(t, root)
	f.Update(focusMsg{to: root.handles[0]})

	f.Update(keyPress('x'))

	if got, want := f.View().Content, "..\nLr"; got != want {
		t.Errorf("View().Content = %q, want %q", got, want)
	}
}

func TestADisabledBindingFallsThroughToAnAncestor(t *testing.T) {
	left := &speaker{letter: "l", bindings: binds("x", "L")}
	left.bindings[0].Disabled = true
	root := &bindRoot{
		speaker: speaker{letter: ".", bindings: binds("x", "R")},
		kids:    []froth.Surface{left, &speaker{letter: "r"}},
	}
	f := bindTree(t, root)
	f.Update(focusMsg{to: root.handles[0]})

	f.Update(keyPress('x'))

	if got, want := f.View().Content, "R.\nlr"; got != want {
		t.Errorf("View().Content = %q, want %q", got, want)
	}
}

func TestADisabledBindingWithNothingAboveItLetsTheRawKeyThrough(t *testing.T) {
	left := &speaker{letter: "l", bindings: binds("x", "L")}
	left.bindings[0].Disabled = true
	root := &bindRoot{
		speaker: speaker{letter: "."},
		kids:    []froth.Surface{left, &speaker{letter: "r"}},
	}
	f := bindTree(t, root)
	f.Update(focusMsg{to: root.handles[0]})

	f.Update(keyPress('x'))

	if got, want := f.View().Content, "..\nxr"; got != want {
		t.Errorf("View().Content = %q, want %q", got, want)
	}
}

func TestAnUnboundKeyStillReachesTheFocusedSurface(t *testing.T) {
	root := &bindRoot{
		speaker: speaker{letter: ".", bindings: binds("q", "Q")},
		kids:    []froth.Surface{&speaker{letter: "l"}, &speaker{letter: "r"}},
	}
	f := bindTree(t, root)
	f.Update(focusMsg{to: root.handles[0]})

	f.Update(keyPress('x'))

	if got, want := f.View().Content, "..\nxr"; got != want {
		t.Errorf("View().Content = %q, want %q", got, want)
	}
}

func TestABoundKeyNeverReachesASurfaceThatDeclaresNothing(t *testing.T) {
	left := &focusable{letter: "l"}
	root := &bindRoot{
		speaker: speaker{letter: ".", bindings: binds("q", "Q")},
		kids:    []froth.Surface{left, &focusable{letter: "r"}},
	}
	f := bindTree(t, root)
	f.Update(focusMsg{to: root.handles[0]})

	f.Update(keyPress('q'))
	f.Update(keyPress('x'))

	if got, want := strings.Join(left.keys, ","), "x"; got != want {
		t.Errorf("the focused surface saw %q, want %q: a key that matched a "+
			"binding is not also delivered raw", got, want)
	}
}

func TestBindingsOnOneSurfaceResolveInDeclarationOrder(t *testing.T) {
	left := &speaker{letter: "l", bindings: []froth.Binding{
		{Action: "A", Keys: []string{"x"}, Help: "first"},
		{Action: "B", Keys: []string{"x"}, Help: "second"},
	}}
	root := &bindRoot{
		speaker: speaker{letter: "."},
		kids:    []froth.Surface{left, &speaker{letter: "r"}},
	}
	f := bindTree(t, root)
	f.Update(focusMsg{to: root.handles[0]})

	f.Update(keyPress('x'))

	if got, want := f.View().Content, "..\nAr"; got != want {
		t.Errorf("View().Content = %q, want %q", got, want)
	}
}

func TestABindingOffTheFocusChainDoesNotFire(t *testing.T) {
	left, right := &speaker{letter: "l"}, &speaker{letter: "r", bindings: binds("x", "R")}
	root := &bindRoot{
		speaker: speaker{letter: "."},
		kids:    []froth.Surface{left, right},
	}
	f := bindTree(t, root)
	f.Update(focusMsg{to: root.handles[0]})

	f.Update(keyPress('x'))

	if got, want := f.View().Content, "..\nxr"; got != want {
		t.Errorf("View().Content = %q, want %q: only the focus chain is "+
			"searched, so a sibling's binding is not in play", got, want)
	}
}

func TestOneBindingCanCarrySeveralKeys(t *testing.T) {
	root := &bindRoot{
		speaker: speaker{letter: ".", bindings: []froth.Binding{
			{Action: "Q", Keys: []string{"q", "ctrl+c"}, Help: "quit"},
		}},
		kids: []froth.Surface{&speaker{letter: "l"}, &speaker{letter: "r"}},
	}
	f := bindTree(t, root)
	f.Update(focusMsg{to: root.handles[0]})

	f.Update(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})

	if got, want := f.View().Content, "Q.\nlr"; got != want {
		t.Errorf("View().Content = %q, want %q", got, want)
	}
}

// Text sinks
//
// sink is the option list for a surface that consumes ordinary typing.
var sink = []froth.CreateOption{froth.TextSink()}

// typed is a key press as a terminal delivers ordinary typing: the character it
// produced, in Text as well as Code.
func typed(r rune) tea.KeyPressMsg { return tea.KeyPressMsg{Code: r, Text: string(r)} }

// sinkTree stands a root that binds bound over a text sink and a plain sibling,
// and gives the sink focus.
func sinkTree(t *testing.T, bound []froth.Binding) (*froth.Froth, *bindRoot) {
	t.Helper()
	root := &bindRoot{
		speaker: speaker{letter: ".", bindings: bound},
		kids:    []froth.Surface{&speaker{letter: "f"}, &speaker{letter: "o"}},
		opts:    [][]froth.CreateOption{sink},
	}
	f := bindTree(t, root)
	f.Update(focusMsg{to: root.handles[0]})
	return f, root
}

func TestABarePrintableKeyReachesAFocusedTextSinkRaw(t *testing.T) {
	f, _ := sinkTree(t, binds("q", "Q"))

	f.Update(typed('q'))

	if got, want := f.View().Content, "..\nqo"; got != want {
		t.Errorf("View().Content = %q, want %q: typing must reach the sink and "+
			"fire nobody's binding", got, want)
	}
}

func TestATextSinkSuppressesItsOwnBindingsToo(t *testing.T) {
	root := &bindRoot{
		speaker: speaker{letter: "."},
		kids: []froth.Surface{
			&speaker{letter: "f", bindings: binds("q", "Q")},
			&speaker{letter: "o"},
		},
		opts: [][]froth.CreateOption{sink},
	}
	f := bindTree(t, root)
	f.Update(focusMsg{to: root.handles[0]})

	f.Update(typed('q'))

	if got, want := f.View().Content, "..\nqo"; got != want {
		t.Errorf("View().Content = %q, want %q: a bare printable matches no "+
			"binding anywhere, the sink's own included", got, want)
	}
}

func TestAPrintableKeyWithNoTextIsStillTyping(t *testing.T) {
	f, _ := sinkTree(t, binds("q", "Q"))

	f.Update(keyPress('q'))

	if got, want := f.View().Content, "..\nqo"; got != want {
		t.Errorf("View().Content = %q, want %q", got, want)
	}
}

func TestAModifiedChordStillResolvesWhileATextSinkHasFocus(t *testing.T) {
	f, _ := sinkTree(t, []froth.Binding{{Action: "Q", Keys: []string{"ctrl+c"}, Help: "quit"}})

	f.Update(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})

	if got, want := f.View().Content, "Q.\nfo"; got != want {
		t.Errorf("View().Content = %q, want %q: a chord is not typing, so an "+
			"ancestor's binding still fires", got, want)
	}
}

// A named key prints nothing, so it is not typing however few modifiers were
// held — a dialog's escape has to keep working while a field holds focus.
func TestKeysThatPrintNothingStillResolveWhileATextSinkHasFocus(t *testing.T) {
	for _, c := range []struct {
		name string
		key  tea.KeyPressMsg
	}{
		{"enter", tea.KeyPressMsg{Code: tea.KeyEnter}},
		{"tab", tea.KeyPressMsg{Code: tea.KeyTab}},
		{"esc", tea.KeyPressMsg{Code: tea.KeyEscape}},
		{"up", tea.KeyPressMsg{Code: tea.KeyUp}},
		{"shift+tab", tea.KeyPressMsg{Code: tea.KeyTab, Mod: tea.ModShift}},
	} {
		t.Run(c.name, func(t *testing.T) {
			f, _ := sinkTree(t, binds(c.name, "K"))

			f.Update(c.key)

			if got, want := f.View().Content, "K.\nfo"; got != want {
				t.Errorf("View().Content = %q, want %q", got, want)
			}
		})
	}
}

// Shift makes a different character, not a chord, so a capital and a "?" are
// ordinary typing and reach the sink.
func TestAShiftedCharacterIsTypingWhileATextSinkHasFocus(t *testing.T) {
	for _, c := range []struct {
		name string
		key  tea.KeyPressMsg
		want string
	}{
		{"capital", tea.KeyPressMsg{Code: 'a', Text: "A", Mod: tea.ModShift}, "..\nAo"},
		{"question", tea.KeyPressMsg{Code: '/', Text: "?", Mod: tea.ModShift}, "..\n?o"},
	} {
		t.Run(c.name, func(t *testing.T) {
			f, _ := sinkTree(t, binds(c.key.String(), "H"))

			f.Update(c.key)

			if got := f.View().Content; got != c.want {
				t.Errorf("View().Content = %q, want %q", got, c.want)
			}
		})
	}
}

func TestASurfaceThatIsNotATextSinkMatchesOrdinaryTyping(t *testing.T) {
	f, root := sinkTree(t, binds("q", "Q"))
	f.Update(focusMsg{to: root.handles[1]})

	f.Update(typed('q'))

	if got, want := f.View().Content, "Q.\nfo"; got != want {
		t.Errorf("View().Content = %q, want %q", got, want)
	}
}

func TestMovingFocusOffATextSinkRestoresMatching(t *testing.T) {
	f, root := sinkTree(t, binds("q", "Q"))
	f.Update(typed('q'))

	f.Update(focusMsg{to: root.handles[1]})
	f.Update(typed('q'))

	if got, want := f.View().Content, "Q.\nqo"; got != want {
		t.Errorf("View().Content = %q, want %q: the flag follows focus and "+
			"leaves nothing set behind the move", got, want)
	}
}

// deep is a speaker with a child of its own, for a text sink that sits on the
// focus chain without being its deep end.
type deep struct {
	speaker
	kid   froth.Surface
	child froth.Handle
}

func (d *deep) Mount(ac *froth.ApplyCtx) {
	d.child = ac.Create(d.kid)
	ac.SetLayout(under{})
}

func TestATextSinkAboveTheFocusedSurfaceSuppressesNothing(t *testing.T) {
	leaf := &speaker{letter: "l"}
	mid := &deep{speaker: speaker{letter: "p"}, kid: leaf}
	root := &bindRoot{
		speaker: speaker{letter: ".", bindings: binds("q", "Q")},
		kids:    []froth.Surface{mid},
		opts:    [][]froth.CreateOption{sink},
	}
	f := newSized(t, root, 1, 3)
	f.View()
	f.Update(focusMsg{to: mid.child})

	f.Update(typed('q'))

	if got, want := f.View().Content, "Q\np\nl"; got != want {
		t.Errorf("View().Content = %q, want %q: only the focused surface's own "+
			"flag suppresses matching", got, want)
	}
}

// Space prints a character, so it is typing — even though it is the one key
// whose String() is a name rather than the character it produced. The sink
// draws "space" clipped to its one cell.
func TestSpaceIsTypingWhileATextSinkHasFocus(t *testing.T) {
	f, _ := sinkTree(t, binds("space", "S"))

	f.Update(tea.KeyPressMsg{Code: tea.KeySpace, Text: " "})

	if got, want := f.View().Content, "..\nso"; got != want {
		t.Errorf("View().Content = %q, want %q", got, want)
	}
}

// The shape the example ships: one binding on a bare printable and a chord.
// While the sink has focus the printable is typing and the chord still quits.
func TestABindingOnBothAPrintableAndAChordKeepsTheChordWhileATextSinkHasFocus(t *testing.T) {
	f, _ := sinkTree(t, []froth.Binding{
		{Action: "Q", Keys: []string{"q", "ctrl+c"}, Help: "quit"},
	})

	f.Update(typed('q'))
	if got, want := f.View().Content, "..\nqo"; got != want {
		t.Errorf("View().Content = %q, want %q", got, want)
	}

	f.Update(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})
	if got, want := f.View().Content, "Q.\nqo"; got != want {
		t.Errorf("View().Content = %q, want %q", got, want)
	}
}
