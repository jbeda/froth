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
	ac.Self().SetLayout(r.layout)
}

func (r *focusRoot) Update(uc *froth.UpdateCtx, msg tea.Msg) {
	switch msg := msg.(type) {
	case focusMsg:
		uc.At(msg.to).Focus()
	case selfFocusMsg:
		uc.At(msg.to).Post(selfFocusMsg{})
	}
}

// focusable draws one letter, upper case while it holds focus, and moves focus
// on to the surface it was told about when it sees a key.
type focusable struct {
	letter string
	toward *froth.Handle // where a key sends focus; nil to keep it
	flags  froth.Flags

	keys  []string
	bumps int
}

func (x *focusable) Declare() froth.Flags { return x.flags }

func (x *focusable) Render(rc *froth.RenderCtx, s froth.Size) string {
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
			uc.At(*x.toward).Focus()
		}
	case selfFocusMsg:
		uc.Self().Focus()
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

// tabPress and shiftTabPress are the two keys froth binds itself. They are
// named rather than spelled at every call site because tab carries no text and
// so cannot be written as a rune the way an ordinary character can.
func tabPress() tea.KeyPressMsg { return tea.KeyPressMsg{Code: tea.KeyTab} }

func shiftTabPress() tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: tea.KeyTab, Mod: tea.ModShift}
}

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
//
// The key is an ordinary one rather than tab: these panes hand focus to each
// other themselves, and tab is froth's own.
func TestAKeyMovesFocusAndTheFrameFollows(t *testing.T) {
	f, root, left, right := twoPanes(t)
	f.Update(focusMsg{to: root.handles[0]})

	f.Update(keyPress('x'))
	if got, want := f.View().Content, "lR"; got != want {
		t.Errorf("after one key, View().Content = %q, want %q", got, want)
	}

	f.Update(keyPress('x'))
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

func TestFocusingAHandleThatIsNotInTheTreeIsReportedAndDoesNothing(t *testing.T) {
	elsewhere := froth.New(context.Background(), &pane{title: "elsewhere"})

	left, right := &focusable{letter: "l"}, &focusable{letter: "r"}
	root := &focusRoot{
		parent: parent{body: "  ", kids: []froth.Surface{left, right}},
		layout: &halves{},
	}
	f, diags := newWatched(t, root, 2, 1)
	f.View()
	f.Update(focusMsg{to: root.handles[0]})

	// A handle from another instance names nothing here, so the context minted
	// for it does nothing — the same answer as a handle whose surface has gone.
	f.Update(focusMsg{to: elsewhere.Root()})

	if len(*diags) != 1 {
		t.Fatalf("froth reported %d diagnostics, want 1: %v", len(*diags), *diags)
	}
	if got, want := (*diags)[0].Kind, froth.DiagDeadHandle; got != want {
		t.Errorf("diagnostic kind = %q, want %q", got, want)
	}
	if got, want := f.View().Content, "Lr"; got != want {
		t.Errorf("focus moved anyway: View().Content = %q, want %q", got, want)
	}
}

// The zero handle names nothing, so it is answered like any other handle whose
// surface is not there. It is worth its own test because it is the one dead
// handle an application can write down without having created anything.
func TestFocusingTheZeroHandleFromDeliverIsReportedAndDoesNothing(t *testing.T) {
	left, right := &focusable{letter: "l"}, &focusable{letter: "r"}
	root := &focusRoot{
		parent: parent{body: "  ", kids: []froth.Surface{left, right}},
		layout: &halves{},
	}
	f, diags := newWatched(t, root, 2, 1)
	f.View()
	f.Update(focusMsg{to: root.handles[0]})

	f.Update(focusMsg{to: froth.Handle{}})

	if len(*diags) != 1 || (*diags)[0].Kind != froth.DiagDeadHandle {
		t.Fatalf("froth reported %v, want one %q", *diags, froth.DiagDeadHandle)
	}
	if got, want := f.View().Content, "Lr"; got != want {
		t.Errorf("focus moved anyway: View().Content = %q, want %q", got, want)
	}
}

// Apply mints the same way, though a move made through a context minted there
// takes effect immediately rather than being recorded.
func TestFocusingAnInvalidHandleFromApplyIsReportedAndDoesNothing(t *testing.T) {
	elsewhere := froth.New(context.Background(), &pane{title: "elsewhere"})
	f, diags := newWatched(t, &focuserOnMount{to: elsewhere.Root()}, 1, 1)
	f.View()

	if len(*diags) != 1 || (*diags)[0].Kind != froth.DiagDeadHandle {
		t.Fatalf("froth reported %v, want one %q", *diags, froth.DiagDeadHandle)
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

// Apply is where structural change commits, so a move made there is not
// recorded for later — it has happened by the time the next thing in the same
// Apply looks. A container that focuses one child and then mounts another sees
// the move it just made.
func TestAMoveMadeDuringApplyHasHappenedByTheNextMountInThatApply(t *testing.T) {
	asker := &focusAsker{}
	root := &focusingParent{
		focusRoot: focusRoot{
			parent: parent{body: "  ", kids: []froth.Surface{&focusable{letter: "l"}, asker}},
			layout: &halves{},
		},
		which: 0,
	}
	newSized(t, root, 2, 1)

	if got, want := asker.sawFocus, root.handles[0]; got != want {
		t.Errorf("a later mount saw focus at %v, want %v — the move was recorded rather than made", got, want)
	}
}

// focusAsker records where focus was when it mounted.
type focusAsker struct{ sawFocus froth.Handle }

func (x *focusAsker) Render(*froth.RenderCtx, froth.Size) string { return "a" }
func (x *focusAsker) Update(*froth.UpdateCtx, tea.Msg)           {}
func (x *focusAsker) Mount(ac *froth.ApplyCtx)                   { x.sawFocus = ac.Query().Focus() }

// focuserOnMount focuses a handle from Apply, where the move commits at once.
type focuserOnMount struct{ to froth.Handle }

func (x *focuserOnMount) Render(rc *froth.RenderCtx, s froth.Size) string {
	if rc.Focused() {
		return "X"
	}
	return "x"
}
func (x *focuserOnMount) Update(*froth.UpdateCtx, tea.Msg) {}
func (x *focuserOnMount) Mount(ac *froth.ApplyCtx)         { ac.At(x.to).Focus() }

// focusingParent mounts its children and focuses one of them, which is what a
// container owning a starting selection does.
type focusingParent struct {
	focusRoot
	which int
}

func (p *focusingParent) Mount(ac *froth.ApplyCtx) {
	p.focusRoot.Mount(ac)
	ac.At(p.handles[p.which]).Focus()
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
// so a test that wants one has to own the surface that creates. What a child
// declares about itself the child carries, because that is where it lives.
type optRoot struct {
	kids    []froth.Surface
	opts    [][]froth.CreateOption
	flags   froth.Flags
	lay     froth.Layout // cells when nil
	handles []froth.Handle
}

func (r *optRoot) Declare() froth.Flags { return r.flags }

func (r *optRoot) Render(rc *froth.RenderCtx, s froth.Size) string {
	return strings.Repeat(".", s.W)
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
		uc.At(m.to).Focus()
	}
}

func (r *optRoot) Mount(ac *froth.ApplyCtx) {
	for i, k := range r.kids {
		var opts []froth.CreateOption
		if i < len(r.opts) {
			opts = r.opts[i]
		}
		r.handles = append(r.handles, ac.Self().Create(k, opts...))
	}
	if r.lay != nil {
		ac.Self().SetLayout(r.lay)
		return
	}
	ac.Self().SetLayout(cells{})
}

// tabRow builds a row of one-cell surfaces, each declaring the flags given for
// it. The letters are lower case unless the surface holds focus, so the frame
// reads as "l m r" with the focused one shouting.
func tabRow(t *testing.T, letters string, flags ...froth.Flags) (*froth.Froth, *optRoot) {
	t.Helper()
	root := &optRoot{}
	for i, c := range letters {
		var fl froth.Flags
		if i < len(flags) {
			fl = flags[i]
		}
		root.kids = append(root.kids, &focusable{letter: string(c), flags: fl})
	}
	f := newSized(t, root, len(letters), 1)
	f.View()
	return f, root
}

// stop is what a surface declares when traversal should land on it. Traversal
// is opt-in, so a test that wants a stop has to say so.
var stop = froth.Flags{TabStop: true}

// clickOnly admits focus from a click and not from the walk, which is the pair
// of admissions being independent in the direction that is easy to get wrong.
var clickOnly = froth.Flags{ClickFocus: true}

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
	f, _ := tabRow(t, "lmr", stop, froth.Flags{}, stop)

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
	f, root := tabRow(t, "lmr", stop, froth.Flags{}, stop)

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
	f, root := tabRow(t, "lmr", stop, froth.Flags{}, stop)

	f.Update(focusMsg{to: root.handles[1]})

	if got, want := f.View().Content, "lMr"; got != want {
		t.Errorf("View().Content = %q, want %q", got, want)
	}
}

// A container never has to be a stop to keep its contents reachable: its
// children are walked whether or not traversal can land on it.
func TestChildrenOfANonStopSurfaceAreStillTraversed(t *testing.T) {
	inner := &focusable{letter: "i", flags: stop}
	wrapper := &optRoot{kids: []froth.Surface{inner}}
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
	f, _, _ := scopedRow(t,
		[]froth.CreateOption{froth.FocusScope(), froth.FocusScope()},
		froth.Flags{})

	want := []string{"Abcd", "abcD", "Abcd"}
	for i, w := range want {
		f.Update(tabMsg{})
		if got := f.View().Content; got != w {
			t.Errorf("after %d moves, View().Content = %q, want %q", i+1, got, w)
		}
	}
}

// Traversal is opt-in, so a surface whose type declares nothing is not a stop.
func TestASurfaceThatDeclaresNothingIsNotATabStop(t *testing.T) {
	f, _ := tabRow(t, "lm", froth.Flags{}, stop)

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
	// The key froth binds itself takes the same walk and finds the same
	// nothing.
	f.Update(tabPress())

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

// Focus scopes
//
// A scope's subtree is a walk of its own: traversal above it steps over it, and
// traversal inside it cycles at its ends. These build a row where every surface
// is one cell wide, so a frame reads as one letter per surface with the focused
// one in upper case.

// columns gives each child a column of the width it was told, left to right, so
// a container can be wider than one cell and still lay its own children out in
// cells of the space it was given.
type columns struct{ w []int }

func (l columns) Arrange(space froth.Rect, children []froth.Child) []froth.Rect {
	out := make([]froth.Rect, len(children))
	x := space.X
	for i := range out {
		w := 1
		if i < len(l.w) {
			w = l.w[i]
		}
		out[i] = froth.Rect{X: x, Y: space.Y, W: w, H: space.H}
		x += w
	}
	return out
}

var scope = []froth.CreateOption{froth.FocusScope()}

// scopedRow builds "a [b c] d": two stops either side of a two-stop container,
// created with the options it is given and declaring the flags it is given. It
// returns the instance, the outer root and that container, which is what a test
// names to move focus into it.
func scopedRow(t *testing.T, inner []froth.CreateOption, mid froth.Flags) (*froth.Froth, *optRoot, *optRoot) {
	t.Helper()
	middle := &optRoot{
		kids:  []froth.Surface{&focusable{letter: "b", flags: stop}, &focusable{letter: "c", flags: stop}},
		flags: mid,
	}
	root := &optRoot{
		kids: []froth.Surface{&focusable{letter: "a", flags: stop}, middle, &focusable{letter: "d", flags: stop}},
		opts: [][]froth.CreateOption{nil, inner, nil},
		lay:  columns{w: []int{1, 2, 1}},
	}
	f := newSized(t, root, 4, 1)
	f.View()
	return f, root, middle
}

func TestTraversalInsideAFocusScopeCyclesAtItsEnds(t *testing.T) {
	f, root, _ := scopedRow(t, scope, froth.Flags{})
	f.Update(focusMsg{to: root.handles[1]})

	// Focus entered the scope, so it starts on the scope's first stop rather
	// than on the scope surface.
	if got, want := f.View().Content, "aBcd"; got != want {
		t.Fatalf("on entering the scope, View().Content = %q, want %q", got, want)
	}

	want := []string{"abCd", "aBcd", "abCd"}
	for i, w := range want {
		f.Update(tabMsg{})
		if got := f.View().Content; got != w {
			t.Errorf("after %d moves inside the scope, View().Content = %q, want %q",
				i+1, got, w)
		}
	}
}

// Traversal never crosses a scope boundary in either direction, so the walk
// above one steps over the scope surface and everything under it. Reaching a
// scope is an explicit move, which is what opening a dialog already is.
func TestTraversalAboveAScopeStepsOverItAndItsContents(t *testing.T) {
	f, _, _ := scopedRow(t, scope, froth.Flags{})

	want := []string{"Abcd", "abcD", "Abcd"}
	for i, w := range want {
		f.Update(tabMsg{})
		if got := f.View().Content; got != w {
			t.Errorf("after %d moves, View().Content = %q, want %q", i+1, got, w)
		}
	}
}

// Without the flag the same tree is one walk: the middle surface is an ordinary
// container and its children are stops in the row like any other.
func TestWithoutTheFlagTheSameSubtreeIsWalkedThrough(t *testing.T) {
	f, _, _ := scopedRow(t, nil, froth.Flags{})

	want := []string{"Abcd", "aBcd", "abCd", "abcD", "Abcd"}
	for i, w := range want {
		f.Update(tabMsg{})
		if got := f.View().Content; got != w {
			t.Errorf("after %d moves, View().Content = %q, want %q", i+1, got, w)
		}
	}
}

// A scope inside a scope walks itself, and its boundary behaves against the
// scope above it exactly as that one's does against the root.
func TestANestedScopeTraversesWithinItselfAndItsBoundaryHolds(t *testing.T) {
	inner := &optRoot{
		kids: []froth.Surface{&focusable{letter: "c", flags: stop}, &focusable{letter: "d", flags: stop}},
	}
	outer := &optRoot{
		kids: []froth.Surface{&focusable{letter: "b", flags: stop}, inner, &focusable{letter: "e", flags: stop}},
		opts: [][]froth.CreateOption{nil, scope},
		lay:  columns{w: []int{1, 2, 1}},
	}
	root := &optRoot{
		kids: []froth.Surface{&focusable{letter: "a", flags: stop}, outer},
		opts: [][]froth.CreateOption{nil, scope},
		lay:  columns{w: []int{1, 4}},
	}
	f := newSized(t, root, 5, 1)
	f.View()

	// The root's walk sees one stop, the outer scope having taken the rest.
	f.Update(tabMsg{})
	if got, want := f.View().Content, "Abcde"; got != want {
		t.Fatalf("the root's walk reached %q, want %q", got, want)
	}

	// The outer scope's walk sees its own two stops and steps over the inner
	// scope, exactly as the root stepped over it.
	f.Update(focusMsg{to: root.handles[1]})
	want := []string{"aBcde", "abcdE", "aBcde"}
	for i, w := range want {
		if got := f.View().Content; got != w {
			t.Errorf("after %d moves in the outer scope, View().Content = %q, want %q", i, got, w)
		}
		f.Update(tabMsg{})
	}

	// The inner scope's walk is its own two stops and nothing else.
	f.Update(focusMsg{to: outer.handles[1]})
	want = []string{"abCde", "abcDe", "abCde"}
	for i, w := range want {
		if got := f.View().Content; got != w {
			t.Errorf("after %d moves in the inner scope, View().Content = %q, want %q", i, got, w)
		}
		f.Update(tabMsg{})
	}
}

// Restoration: the scope remembers the child focus was on, and coming back
// lands there rather than at the first stop.
func TestFocusReenteringAScopeLandsWhereItLeft(t *testing.T) {
	f, root, _ := scopedRow(t, scope, froth.Flags{})
	f.Update(focusMsg{to: root.handles[1]})
	f.Update(tabMsg{})
	if got, want := f.View().Content, "abCd"; got != want {
		t.Fatalf("View().Content = %q, want %q", got, want)
	}

	f.Update(focusMsg{to: root.handles[0]})
	f.Update(focusMsg{to: root.handles[1]})

	if got, want := f.View().Content, "abCd"; got != want {
		t.Errorf("re-entering the scope: View().Content = %q, want %q", got, want)
	}
}

// What a scope remembers is where focus lands when it re-enters, and nothing
// else. It is never an answer to where focus is: that is one handle, and the
// chain is derived from it.
func TestARememberedChildIsNotFocused(t *testing.T) {
	f, root, scoped := scopedRow(t, scope, froth.Flags{})
	f.Update(focusMsg{to: root.handles[1]})
	f.Update(tabMsg{})

	f.Update(focusMsg{to: root.handles[0]})

	if got, want := f.View().Content, "Abcd"; got != want {
		t.Errorf("View().Content = %q, want %q — only the focused surface is focused",
			got, want)
	}
	// The remembered surface is not on the chain, so a key goes to the surface
	// that holds focus rather than to the one the scope remembers.
	f.Update(keyPress('x'))
	if got := scoped.kids[1].(*focusable).keys; len(got) != 0 {
		t.Errorf("the remembered surface saw %v, want nothing", got)
	}
}

// A remembered child its creator kept out of traversal is not somewhere focus
// can be walked to, so entry falls back to the scope's first stop rather than
// landing on it.
func TestARememberedChildOutsideTraversalFallsBackToTheFirstStop(t *testing.T) {
	scoped := &optRoot{
		kids: []froth.Surface{&focusable{letter: "b", flags: stop}, &focusable{letter: "c"}},
	}
	root := &optRoot{
		kids: []froth.Surface{&focusable{letter: "a", flags: stop}, scoped},
		opts: [][]froth.CreateOption{nil, scope},
		lay:  columns{w: []int{1, 2}},
	}
	f := newSized(t, root, 3, 1)
	f.View()

	// Focused by name, so the scope remembers a surface traversal cannot reach.
	f.Update(focusMsg{to: scoped.handles[1]})
	if got, want := f.View().Content, "abC"; got != want {
		t.Fatalf("View().Content = %q, want %q", got, want)
	}

	f.Update(focusMsg{to: root.handles[0]})
	f.Update(focusMsg{to: root.handles[1]})

	if got, want := f.View().Content, "aBc"; got != want {
		t.Errorf("re-entering the scope: View().Content = %q, want %q", got, want)
	}
}

// A scope surface is a stop within its own walk and in no other, which is the
// same rule that puts its children there: everything about a scope belongs to
// the scope, including the surface that declares it.
func TestAScopeThatIsATabStopIsAStopWithinItselfOnly(t *testing.T) {
	f, _, scoped := scopedRow(t, scope, stop)

	// The walk above it steps over it, flag or no flag.
	f.Update(tabMsg{})
	f.Update(tabMsg{})
	if got, want := f.View().Content, "abcD"; got != want {
		t.Fatalf("the walk above the scope reached %q, want %q", got, want)
	}

	// Inside, it is the head of its own walk: three stops — itself, b and c —
	// so three moves from b return to b, and the middle one lands on the scope
	// surface, where no letter is upper case.
	f.Update(focusMsg{to: scoped.handles[0]})
	if got, want := f.View().Content, "aBcd"; got != want {
		t.Fatalf("View().Content = %q, want %q", got, want)
	}
	for i, w := range []string{"abCd", "abcd", "aBcd"} {
		f.Update(tabMsg{})
		if got := f.View().Content; got != w {
			t.Errorf("after %d moves inside the scope, View().Content = %q, want %q",
				i+1, got, w)
		}
	}
}

// A scope with nowhere to go inside it keeps focus on its own surface, which is
// a defined place to be rather than a rejected move.
func TestFocusMovedToAScopeWithNoStopInItStaysOnTheScope(t *testing.T) {
	empty := &optRoot{kids: []froth.Surface{&focusable{letter: "b"}}}
	root := &optRoot{
		kids: []froth.Surface{&focusable{letter: "a", flags: stop}, empty},
		opts: [][]froth.CreateOption{nil, scope},
		lay:  columns{w: []int{1, 1}},
	}
	f, diags := newWatched(t, root, 2, 1)
	f.View()

	f.Update(focusMsg{to: root.handles[1]})
	f.Update(tabMsg{})

	if got, want := f.View().Content, "ab"; got != want {
		t.Errorf("View().Content = %q, want %q — the scope holds focus and nothing moves",
			got, want)
	}
	if len(*diags) != 0 {
		t.Errorf("froth reported %v; a scope with no stop in it is not a diagnostic", *diags)
	}
}

// Restoration reaches through a scope inside a scope. The remembered surface
// here is out of traversal, so the walk up the remembered chain reaches the
// inner scope — and a remembered scope is entered rather than skipped, so focus
// comes back to the inner scope's own stop rather than to the outer scope's
// first one.
func TestRestorationFallingBackOntoANestedScopeEntersIt(t *testing.T) {
	inner := &optRoot{
		kids: []froth.Surface{&focusable{letter: "c", flags: stop}, &focusable{letter: "d"}},
	}
	outer := &optRoot{
		kids: []froth.Surface{&focusable{letter: "b", flags: stop}, inner},
		opts: [][]froth.CreateOption{nil, scope},
		lay:  columns{w: []int{1, 2}},
	}
	root := &optRoot{
		kids: []froth.Surface{&focusable{letter: "a", flags: stop}, outer},
		opts: [][]froth.CreateOption{nil, scope},
		lay:  columns{w: []int{1, 3}},
	}
	f := newSized(t, root, 4, 1)
	f.View()

	// Focus rests in the inner scope, so the outer one remembers the inner
	// scope surface and the inner one remembers d.
	f.Update(focusMsg{to: inner.handles[1]})
	if got, want := f.View().Content, "abcD"; got != want {
		t.Fatalf("View().Content = %q, want %q", got, want)
	}

	f.Update(focusMsg{to: root.handles[0]})
	f.Update(focusMsg{to: root.handles[1]})

	if got, want := f.View().Content, "abCd"; got != want {
		t.Errorf("re-entering the outer scope: View().Content = %q, want %q", got, want)
	}
}

// withinBox is a container that shows whether focus is anywhere at or below it,
// which is the question Focused cannot answer. It draws one character in the
// top row of its space and leaves the row underneath to its child.
type withinBox struct {
	parent
}

func (b *withinBox) Mount(ac *froth.ApplyCtx) {
	b.parent.Mount(ac)
	ac.Self().SetLayout(underneath{})
}

func (b *withinBox) Render(rc *froth.RenderCtx, s froth.Size) string {
	if rc.FocusWithin() {
		return "+"
	}
	return "-"
}

// underneath gives each child the row below the surface that owns them.
type underneath struct{}

func (underneath) Arrange(space froth.Rect, children []froth.Child) []froth.Rect {
	out := make([]froth.Rect, len(children))
	for i := range out {
		out[i] = froth.Rect{X: space.X, Y: space.Y + 1, W: space.W, H: 1}
	}
	return out
}

// A container asking about focus below it is the whole of #16's demo: the box
// on the left changes while its child holds the keyboard, the box on the right
// does not, and the child's own letter shows where focus actually is.
func TestAContainerSeesFocusBelowItAndASiblingDoesNot(t *testing.T) {
	leftKid, rightKid := &focusable{letter: "l"}, &focusable{letter: "r"}
	leftBox := &withinBox{parent: parent{kids: []froth.Surface{leftKid}}}
	rightBox := &withinBox{parent: parent{kids: []froth.Surface{rightKid}}}
	root := &focusRoot{
		parent: parent{kids: []froth.Surface{leftBox, rightBox}},
		layout: &halves{},
	}
	f := newSized(t, root, 2, 2)
	f.View()

	if got, want := f.View().Content, "--\nlr"; got != want {
		t.Errorf("with the root focused, View().Content = %q, want %q", got, want)
	}

	f.Update(focusMsg{to: leftBox.handles[0]})

	if got, want := f.View().Content, "+-\nLr"; got != want {
		t.Errorf("with the left child focused, View().Content = %q, want %q", got, want)
	}

	// The focused surface itself reports true, so FocusWithin is a superset of
	// Focused rather than an ancestors-only question.
	f.Update(focusMsg{to: root.handles[0]})

	if got, want := f.View().Content, "+-\nlr"; got != want {
		t.Errorf("with the left box itself focused, View().Content = %q, want %q", got, want)
	}
}
