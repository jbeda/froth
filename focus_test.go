package froth_test

import (
	"context"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/jbeda/froth"
)

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
