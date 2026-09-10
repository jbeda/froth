package froth_test

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/jbeda/froth"
)

// patch is a surface that records the last mouse message that reached it and
// draws w×h of its letter, or fills what it is offered when w and h are zero.
// It can take capture on a press and let go on a release, which is the common
// shape a drag has.
type patch struct {
	letter string
	flags  froth.Flags
	w, h   int
	self   froth.Handle
	last   string

	captureOnPress bool
	releaseOnUp    bool
	destroyOnPress bool

	// focusedOnPress records whether this patch held focus as the press
	// reached it.
	focusedOnPress bool

	// kids are created under this patch, each at the rect its layout gives it.
	kids  []nestKid
	rects []froth.Rect
	kidH  []froth.Handle

	pendingCreate []createMsg
}

func (s *patch) Declare() froth.Flags { return s.flags }

func (s *patch) Mount(ac *froth.ApplyCtx) {
	s.self = ac.Self().Handle()
	for _, k := range s.kids {
		s.kidH = append(s.kidH, ac.Self().Create(k.surface, k.opts...))
	}
	if len(s.rects) > 0 {
		ac.Self().SetLayout(atOrEmpty(s.rects))
	}
}

func (s *patch) Render(rc *froth.RenderCtx, size froth.Size) string {
	w, h := s.w, s.h
	if w == 0 {
		w = size.W
	}
	if h == 0 {
		h = size.H
	}
	row := strings.Repeat(s.letter, w)
	return strings.TrimRight(strings.Repeat(row+"\n", h), "\n")
}

func (s *patch) Update(uc *froth.UpdateCtx, msg tea.Msg) {
	switch m := msg.(type) {
	case tea.MouseClickMsg:
		s.last = fmt.Sprintf("click@%d,%d", m.X, m.Y)
		s.focusedOnPress = uc.Query().Focus() == s.self
		if s.captureOnPress {
			uc.Self().Capture()
		}
		if s.destroyOnPress {
			uc.Self().Destroy()
		}
	case tea.MouseReleaseMsg:
		s.last = fmt.Sprintf("release@%d,%d", m.X, m.Y)
		if s.releaseOnUp {
			uc.Self().ReleaseCapture()
		}
	case tea.MouseWheelMsg:
		s.last = fmt.Sprintf("wheel@%d,%d", m.X, m.Y)
	case tea.MouseMotionMsg:
		s.last = fmt.Sprintf("motion@%d,%d", m.X, m.Y)
	case froth.CaptureLostMsg:
		s.last = "lost"
	case captureMsg:
		uc.Self().Capture()
	case releaseMsg:
		uc.Self().ReleaseCapture()
	case destroyMsg:
		uc.Self().Destroy()
	case transparentMsg:
		uc.Child(m.child).SetMouseTransparent(m.v)
	case createMsg:
		s.pendingCreate = append(s.pendingCreate, m)
		uc.Self().Reconcile()
	case postMsg:
		for _, pm := range m.msgs {
			uc.At(m.to).Post(pm)
		}
	}
}

// Reconcile creates whatever a createMsg asked for, with a real Apply context.
func (s *patch) Reconcile(ac *froth.ApplyCtx) {
	for _, c := range s.pendingCreate {
		s.kidH = append(s.kidH, ac.Self().Create(c.surface, c.opts...))
	}
	s.pendingCreate = nil
}

// Messages a test posts to drive a patch from the outside.
type (
	captureMsg     struct{}
	releaseMsg     struct{}
	destroyMsg     struct{}
	transparentMsg struct {
		child froth.Handle
		v     bool
	}
	createMsg struct {
		surface froth.Surface
		opts    []froth.CreateOption
	}
)

// atOrEmpty hands each child the rect at its index, and an empty rect to any
// child created after the list was written — a modal that arrives mid-test.
type atOrEmpty []froth.Rect

func (l atOrEmpty) Arrange(space froth.Rect, children []froth.Child) []froth.Rect {
	out := make([]froth.Rect, len(children))
	for i := range out {
		if i < len(l) {
			out[i] = l[i]
		}
	}
	return out
}

func click(x, y int) tea.MouseClickMsg {
	return tea.MouseClickMsg{X: x, Y: y, Button: tea.MouseLeft}
}

func release(x, y int) tea.MouseReleaseMsg {
	return tea.MouseReleaseMsg{X: x, Y: y, Button: tea.MouseLeft}
}

// mouseTree draws the first frame, so there is a rendered frame for a click to
// resolve against.
func mouseTree(t *testing.T, root *patch, w, h int) *froth.Froth {
	t.Helper()
	f := newSized(t, root, w, h)
	f.View()
	return f
}

// Two siblings overlapping on one cell: the later one is drawn on top, so it
// is the one a click there reaches.
func TestAClickResolvesToTheTopmostSurfaceUnderIt(t *testing.T) {
	a, b := &patch{letter: "a"}, &patch{letter: "b"}
	root := &patch{letter: ".",
		kids:  []nestKid{{surface: a}, {surface: b}},
		rects: []froth.Rect{{X: 0, Y: 0, W: 2, H: 1}, {X: 1, Y: 0, W: 2, H: 1}},
	}
	f := mouseTree(t, root, 4, 1)

	f.Update(click(1, 0))

	if a.last != "" || b.last != "click@1,0" {
		t.Errorf("a saw %q and b saw %q; want the click on b alone", a.last, b.last)
	}

	f.Update(click(0, 0))
	if a.last != "click@0,0" {
		t.Errorf("a saw %q, want the click where b does not cover it", a.last)
	}
}

// A click is resolved against what a surface drew, not what it was offered:
// the cells a child underfills still belong to whatever was painted beneath.
func TestAClickOutsideWhatASurfaceDrewReachesWhatIsBeneath(t *testing.T) {
	kid := &patch{letter: "k", w: 1, h: 1}
	root := &patch{letter: ".",
		kids:  []nestKid{{surface: kid}},
		rects: []froth.Rect{{X: 0, Y: 0, W: 3, H: 1}},
	}
	f := mouseTree(t, root, 3, 1)

	f.Update(click(2, 0))

	if kid.last != "" {
		t.Errorf("the child saw %q for a click on cells it did not draw", kid.last)
	}
	if root.last != "click@2,0" {
		t.Errorf("the root saw %q, want the click that fell through", root.last)
	}
}

// A mouse transparent surface is skipped, and so is everything under it: a
// click lands on whatever is beneath in Z. Its creator can put it back.
func TestAMouseTransparentSubtreeIsSkippedByTheHitWalk(t *testing.T) {
	grandkid := &patch{letter: "g"}
	kid := &patch{letter: "k",
		kids:  []nestKid{{surface: grandkid}},
		rects: []froth.Rect{{X: 0, Y: 0, W: 1, H: 1}},
	}
	root := &patch{letter: ".",
		kids:  []nestKid{{surface: kid, opts: []froth.CreateOption{froth.MouseTransparent()}}},
		rects: []froth.Rect{{X: 0, Y: 0, W: 2, H: 1}},
	}
	f := mouseTree(t, root, 2, 1)

	f.Update(click(0, 0))
	if grandkid.last != "" || kid.last != "" {
		t.Errorf("a transparent subtree saw a click: kid %q, grandkid %q", kid.last, grandkid.last)
	}
	if root.last != "click@0,0" {
		t.Errorf("the root saw %q, want the click that passed through", root.last)
	}

	f.Update(transparentMsg{child: root.kidH[0], v: false})
	f.Update(click(0, 0))
	if grandkid.last != "click@0,0" {
		t.Errorf("after being put back, the grandchild saw %q", grandkid.last)
	}
}

// Focus follows a press only where the surface admits it, and it does not
// look for an ancestor that would have. The message is delivered either way.
func TestAPressMovesFocusOnlyToASurfaceThatAdmitsIt(t *testing.T) {
	yes := &patch{letter: "y", flags: froth.Flags{ClickFocus: true}}
	no := &patch{letter: "n", flags: froth.Flags{TabStop: true}}
	root := &patch{letter: ".", flags: froth.Flags{ClickFocus: true},
		kids:  []nestKid{{surface: yes}, {surface: no}},
		rects: []froth.Rect{{X: 0, Y: 0, W: 1, H: 1}, {X: 1, Y: 0, W: 1, H: 1}},
	}
	f := mouseTree(t, root, 2, 1)

	f.Update(click(0, 0))
	if got := f.Query().Focus(); got != yes.self {
		t.Errorf("focus after a press on a ClickFocus surface = %v, want %v", got, yes.self)
	}

	f.Update(click(1, 0))
	if got := f.Query().Focus(); got != yes.self {
		t.Errorf("focus after a press on a surface without ClickFocus = %v, want unchanged", got)
	}
	if no.last != "click@1,0" {
		t.Errorf("the surface without ClickFocus saw %q, want the click anyway", no.last)
	}
}

// The target holds focus by the time the press reaches it, and a target that
// destroys itself on the press leaves focus rehomed rather than refused: froth
// asked for the move, so nothing is reported.
func TestATargetHoldsFocusAsThePressArrives(t *testing.T) {
	kid := &patch{letter: "k", flags: froth.Flags{ClickFocus: true}, destroyOnPress: true}
	root := &patch{letter: ".",
		kids:  []nestKid{{surface: kid}},
		rects: []froth.Rect{{X: 0, Y: 0, W: 1, H: 1}},
	}
	f, diags := newWatched(t, root, 2, 1)
	f.View()

	f.Update(click(0, 0))

	if !kid.focusedOnPress {
		t.Error("the target did not hold focus as the press reached it")
	}
	if got := f.Query().Focus(); got != root.self {
		t.Errorf("focus = %v after the target destroyed itself, want rehomed to the root", got)
	}
	if len(*diags) != 0 {
		t.Errorf("froth reported %v for its own focus move onto a surface that then left", *diags)
	}
}

// Motion is routed like everything else: hit-tested, or to the holder.
func TestMotionGoesWhereAClickWould(t *testing.T) {
	a, b := &patch{letter: "a", captureOnPress: true}, &patch{letter: "b"}
	root := &patch{letter: ".",
		kids:  []nestKid{{surface: a}, {surface: b}},
		rects: []froth.Rect{{X: 0, Y: 0, W: 1, H: 1}, {X: 1, Y: 0, W: 1, H: 1}},
	}
	f := mouseTree(t, root, 2, 1)

	f.Update(tea.MouseMotionMsg{X: 1, Y: 0, Button: tea.MouseLeft})
	if b.last != "motion@1,0" {
		t.Errorf("b saw %q, want motion hit-tested to it", b.last)
	}

	f.Update(click(0, 0))
	f.Update(tea.MouseMotionMsg{X: 1, Y: 0, Button: tea.MouseLeft})
	if a.last != "motion@1,0" {
		t.Errorf("the holder saw %q, want the motion over its sibling", a.last)
	}
}

// The frame a click resolves against is the one the user saw, so a surface
// destroyed since it was drawn is still the answer, and the click is dropped
// rather than falling through to what was never on screen.
func TestAClickOnASurfaceGoneSinceTheFrameIsDropped(t *testing.T) {
	kid := &patch{letter: "k", destroyOnPress: true}
	root := &patch{letter: ".",
		kids:  []nestKid{{surface: kid}},
		rects: []froth.Rect{{X: 0, Y: 0, W: 1, H: 1}},
	}
	f := mouseTree(t, root, 2, 1)

	f.Update(click(0, 0)) // destroys kid; no frame is drawn after
	f.Update(click(0, 0))

	if root.last != "" {
		t.Errorf("the root saw %q for a click on a cell the frame still gave to its child", root.last)
	}
}

// Only a press admits focus. A release or wheel on a ClickFocus surface is
// delivered and moves nothing.
func TestOnlyAPressMovesFocus(t *testing.T) {
	kid := &patch{letter: "k", flags: froth.Flags{ClickFocus: true}}
	root := &patch{letter: ".",
		kids:  []nestKid{{surface: kid}},
		rects: []froth.Rect{{X: 0, Y: 0, W: 1, H: 1}},
	}
	f := mouseTree(t, root, 2, 1)

	f.Update(release(0, 0))
	f.Update(tea.MouseWheelMsg{X: 0, Y: 0, Button: tea.MouseWheelDown})

	if got := f.Query().Focus(); got != root.self {
		t.Errorf("focus = %v after a release and a wheel, want still on the root", got)
	}
	if kid.last != "wheel@0,0" {
		t.Errorf("the surface saw %q, want the wheel delivered", kid.last)
	}
}

// A click with nothing under it goes nowhere — not to the root either. Before
// any frame is drawn there is nothing under anything.
func TestAClickOnNobodyIsDropped(t *testing.T) {
	root := &patch{letter: ".", w: 1, h: 1}
	f := mouseTree(t, root, 3, 1)

	f.Update(click(2, 0))
	if root.last != "" {
		t.Errorf("the root saw %q for a click on cells it did not draw", root.last)
	}

	fresh := &patch{letter: "."}
	g := newSized(t, fresh, 3, 1)
	g.Update(click(0, 0))
	if fresh.last != "" {
		t.Errorf("a click before the first frame reached the root: %q", fresh.last)
	}
}

// A click aimed into the subtree an open modal blocks is dropped, and focus
// does not move there either. The same tree without the modal delivers it.
func TestAClickBehindAModalIsDropped(t *testing.T) {
	for _, tc := range []struct {
		name string
		opts []froth.CreateOption
		want string
	}{
		{"with a modal", modal, ""},
		{"without one", nil, "click@0,0"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			behind := &patch{letter: "b", flags: froth.Flags{ClickFocus: true}}
			dialog := &patch{letter: "d"}
			root := &patch{letter: ".",
				kids:  []nestKid{{surface: behind}, {surface: dialog, opts: tc.opts}},
				rects: []froth.Rect{{X: 0, Y: 0, W: 1, H: 1}, {X: 1, Y: 0, W: 1, H: 1}},
			}
			f := mouseTree(t, root, 2, 1)

			f.Update(click(0, 0))

			if behind.last != tc.want {
				t.Errorf("the surface behind saw %q, want %q", behind.last, tc.want)
			}
			focused := f.Query().Focus() == behind.self
			if focused != (tc.want != "") {
				t.Errorf("focus moved to the surface behind = %v, want %v", focused, tc.want != "")
			}
		})
	}
}

// While a surface holds capture every mouse message goes to it, wherever it
// landed, and a press elsewhere moves focus nowhere.
func TestACaptureHolderGetsEveryMouseMessage(t *testing.T) {
	holder := &patch{letter: "h"}
	other := &patch{letter: "o", flags: froth.Flags{ClickFocus: true}}
	root := &patch{letter: ".",
		kids:  []nestKid{{surface: holder}, {surface: other}},
		rects: []froth.Rect{{X: 0, Y: 0, W: 1, H: 1}, {X: 1, Y: 0, W: 1, H: 1}},
	}
	f := mouseTree(t, root, 2, 1)
	postTo(f, holder.self, captureMsg{})
	if holder.last != "" {
		t.Fatalf("taking capture posted %q to the taker", holder.last)
	}

	f.Update(click(1, 0))

	if holder.last != "click@1,0" || other.last != "" {
		t.Errorf("holder saw %q and other saw %q; want the click at the holder alone", holder.last, other.last)
	}
	if got := f.Query().Focus(); got != root.self {
		t.Errorf("focus = %v after a captured press, want unchanged", got)
	}
}

// Capture taken while handling a press is in force for the release that
// follows, and letting go on the release puts the next click back through
// hit-testing.
func TestCaptureOnPressAndReleaseOnUp(t *testing.T) {
	drag := &patch{letter: "d", captureOnPress: true, releaseOnUp: true}
	other := &patch{letter: "o"}
	root := &patch{letter: ".",
		kids:  []nestKid{{surface: drag}, {surface: other}},
		rects: []froth.Rect{{X: 0, Y: 0, W: 1, H: 1}, {X: 1, Y: 0, W: 1, H: 1}},
	}
	f := mouseTree(t, root, 2, 1)

	f.Update(click(0, 0))
	f.Update(release(1, 0))
	if drag.last != "release@1,0" || other.last != "" {
		t.Errorf("drag saw %q and other saw %q; want the release at the presser", drag.last, other.last)
	}

	f.Update(click(1, 0))
	if other.last != "click@1,0" {
		t.Errorf("after release, other saw %q; want the click hit-tested again", other.last)
	}
}

// Taking capture from another surface tells the one that lost it, and the taker
// holds it from then on. Taking it again while holding it says nothing.
func TestTakingCaptureFromAnotherSurfaceTellsIt(t *testing.T) {
	a, b := &patch{letter: "a"}, &patch{letter: "b"}
	root := &patch{letter: ".",
		kids:  []nestKid{{surface: a}, {surface: b}},
		rects: []froth.Rect{{X: 0, Y: 0, W: 1, H: 1}, {X: 1, Y: 0, W: 1, H: 1}},
	}
	f := mouseTree(t, root, 3, 1)

	postTo(f, a.self, captureMsg{})
	postTo(f, b.self, captureMsg{})

	if a.last != "lost" {
		t.Errorf("the surface that lost capture saw %q, want CaptureLostMsg", a.last)
	}
	b.last = ""
	postTo(f, b.self, captureMsg{})
	if b.last != "" {
		t.Errorf("re-taking capture posted %q to the holder", b.last)
	}

	f.Update(click(0, 0))
	if b.last != "click@0,0" {
		t.Errorf("the taker saw %q, want every click", b.last)
	}
}

// Releasing capture nobody holds, or that somebody else holds, does nothing.
func TestReleasingCaptureYouDoNotHoldIsNothing(t *testing.T) {
	a, b := &patch{letter: "a"}, &patch{letter: "b"}
	root := &patch{letter: ".",
		kids:  []nestKid{{surface: a}, {surface: b}},
		rects: []froth.Rect{{X: 0, Y: 0, W: 1, H: 1}, {X: 1, Y: 0, W: 1, H: 1}},
	}
	f, diags := newWatched(t, root, 3, 1)
	f.View()

	postTo(f, a.self, releaseMsg{})
	postTo(f, a.self, captureMsg{})
	postTo(f, b.self, releaseMsg{})

	f.Update(click(1, 0))
	if a.last != "click@1,0" {
		t.Errorf("after a stranger's release, the holder saw %q; want it still holding", a.last)
	}
	if len(*diags) != 0 {
		t.Errorf("releasing capture not held reported %v", *diags)
	}
}

// The holder leaving the tree releases capture with no message: the next
// click is hit-tested.
func TestDestroyingTheHolderReleasesCapture(t *testing.T) {
	holder := &patch{letter: "h"}
	other := &patch{letter: "o"}
	root := &patch{letter: ".",
		kids:  []nestKid{{surface: holder}, {surface: other}},
		rects: []froth.Rect{{X: 0, Y: 0, W: 1, H: 1}, {X: 1, Y: 0, W: 1, H: 1}},
	}
	f := mouseTree(t, root, 2, 1)
	postTo(f, holder.self, captureMsg{})
	postTo(f, holder.self, destroyMsg{})
	f.View() // the layout hands the survivor the first rect

	f.Update(click(0, 0))

	if other.last != "click@0,0" {
		t.Errorf("after the holder was destroyed, other saw %q; want the click hit-tested", other.last)
	}
}

// A modal opening over the holder takes capture away and says so; one opening
// somewhere the holder is not blocked leaves it alone.
func TestAModalOpeningOverTheHolderReleasesCapture(t *testing.T) {
	for _, tc := range []struct {
		name     string
		underKid bool // the modal is created under the holder's sibling
		lost     bool
	}{
		{"parented to the root", false, true},
		{"parented under a sibling", true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			holder := &patch{letter: "h"}
			sibling := &patch{letter: "s", rects: []froth.Rect{{X: 1, Y: 0, W: 1, H: 1}}}
			root := &patch{letter: ".",
				kids:  []nestKid{{surface: holder}, {surface: sibling}},
				rects: []froth.Rect{{X: 0, Y: 0, W: 1, H: 1}, {X: 1, Y: 0, W: 1, H: 1}},
			}
			f := mouseTree(t, root, 3, 1)
			postTo(f, holder.self, captureMsg{})

			opener := root
			if tc.underKid {
				opener = sibling
			}
			postTo(f, opener.self, createMsg{surface: &patch{letter: "m"}, opts: modal})
			f.View()

			if got := holder.last == "lost"; got != tc.lost {
				t.Errorf("holder saw %q; lost = %v, want %v", holder.last, got, tc.lost)
			}
		})
	}
}

// A surface an open modal has taken input away from cannot take capture, and
// is told why.
func TestCaptureFromABlockedSurfaceIsRefused(t *testing.T) {
	behind := &patch{letter: "b"}
	dialog := &patch{letter: "d"}
	root := &patch{letter: ".",
		kids:  []nestKid{{surface: behind}, {surface: dialog, opts: modal}},
		rects: []froth.Rect{{X: 0, Y: 0, W: 1, H: 1}, {X: 1, Y: 0, W: 1, H: 1}},
	}
	f, diags := newWatched(t, root, 2, 1)
	f.View()

	postTo(f, behind.self, captureMsg{})

	if len(*diags) != 1 || (*diags)[0].Kind != froth.DiagCaptureBlocked {
		t.Fatalf("diagnostics = %v, want one %q", *diags, froth.DiagCaptureBlocked)
	}
	f.Update(click(1, 0))
	if dialog.last != "click@1,0" {
		t.Errorf("the dialog saw %q; want the click hit-tested, since nothing holds capture", dialog.last)
	}
}

// A surface can read where it was in the rendered frame, which is what lets a
// capture holder tell a click inside itself from one outside.
func TestASurfaceCanReadItsRenderedRect(t *testing.T) {
	kid := &rectReader{}
	root := &patch{letter: ".",
		kids:  []nestKid{{surface: kid}},
		rects: []froth.Rect{{X: 1, Y: 0, W: 2, H: 1}},
	}
	f := mouseTree(t, root, 4, 1)

	f.Update(click(1, 0))

	if want := (froth.Rect{X: 1, Y: 0, W: 2, H: 1}); kid.rect != want || !kid.ok {
		t.Errorf("Rect() = %v, %v; want %v, true", kid.rect, kid.ok, want)
	}
}

// rectReader asks for its own rendered rect on a click.
type rectReader struct {
	rect froth.Rect
	ok   bool
}

func (r *rectReader) Render(*froth.RenderCtx, froth.Size) string { return "rr" }

func (r *rectReader) Update(uc *froth.UpdateCtx, msg tea.Msg) {
	if _, ok := msg.(tea.MouseClickMsg); ok {
		r.rect, r.ok = uc.Self().Rect()
	}
}
