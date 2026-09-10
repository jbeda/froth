package froth_test

import (
	"slices"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/jbeda/froth"
)

// openMsg asks a leaver to open its dialog; closeMsg asks a surface to destroy
// itself. A message handed to Update lands on the root, so the root of the
// test tree forwards a sendMsg to whoever it names, and send is how a test
// addresses one.
type openMsg struct{}
type closeMsg struct{}
type sendMsg struct {
	to  froth.Handle
	msg tea.Msg
}

func send(f *froth.Froth, to froth.Handle, msg tea.Msg) { f.Update(sendMsg{to: to, msg: msg}) }

// on adds a handler for one message type ahead of whatever the leaver already
// handles.
func on[M tea.Msg](l *leaver, fn func(uc *froth.UpdateCtx, m M)) {
	prev := l.onMsg
	l.onMsg = func(uc *froth.UpdateCtx, msg tea.Msg) {
		if m, ok := msg.(M); ok {
			fn(uc, m)
			return
		}
		if prev != nil {
			prev(uc, msg)
		}
	}
}

// opener wires a leaver so that an openMsg makes it open dialog as a modal on
// the next Apply, with opts, recording the handle it got back in opened.
// Closing goes through the dialog itself.
func opener(l *leaver, dialog froth.Surface, opened *froth.Handle, opts ...froth.CreateOption) {
	on(l, func(uc *froth.UpdateCtx, _ openMsg) { uc.Self().Reconcile() })
	l.onReconcile = func(ac *froth.ApplyCtx) {
		*opened = ac.Self().OpenModal(dialog, opts...)
	}
}

// closer makes a leaver destroy itself on closeMsg.
func closer(l *leaver) {
	on(l, func(uc *froth.UpdateCtx, _ closeMsg) { uc.Self().Destroy() })
}

// A two-pane application with a row inside the left pane, which is where focus
// sits when a dialog opens. Returns the instance with focus on the row.
type modalApp struct {
	row, left, right, panes, root *leaver
	log                           []string
}

func newModalApp(t *testing.T) (*froth.Froth, *modalApp) {
	t.Helper()
	f, a, _ := newWatchedModalApp(t)
	return f, a
}

func newWatchedModalApp(t *testing.T) (*froth.Froth, *modalApp, *[]froth.Diagnostic) {
	t.Helper()
	a := &modalApp{}
	a.row = &leaver{name: "row", log: &a.log}
	a.left = &leaver{name: "left", noStop: true, kids: []froth.Surface{a.row}, log: &a.log}
	a.right = &leaver{name: "right", log: &a.log}
	a.panes = &leaver{name: "panes", noStop: true, kids: []froth.Surface{a.left, a.right}, log: &a.log}
	a.root = &leaver{name: "root", noStop: true, kids: []froth.Surface{a.panes}, log: &a.log}
	a.root.onMsg = func(uc *froth.UpdateCtx, msg tea.Msg) {
		switch m := msg.(type) {
		case focusMsg:
			uc.At(m.to).Focus()
		case sendMsg:
			uc.At(m.to).Post(m.msg)
		}
	}
	f, seen := newWatched(t, a.root, 4, 2)
	f.Update(focusMsg{to: a.row.self})
	if got := f.Query().Focus(); got != a.row.self {
		t.Fatalf("setup: focus = %v, want the row %v", got, a.row.self)
	}
	return f, a, seen
}

// A modal is parented to the root, whoever opened it, and the opener gets its
// handle back.
func TestOpenModalParentsToTheRootAndReturnsTheHandle(t *testing.T) {
	f, a := newModalApp(t)
	var parent, opened froth.Handle
	dialog := &leaver{name: "dialog", log: &a.log}
	dialog.onMount = func(ac *froth.ApplyCtx) { parent = ac.Self().Parent() }
	opener(a.left, dialog, &opened)

	send(f, a.left.self, openMsg{})

	if opened == (froth.Handle{}) || opened != dialog.self {
		t.Errorf("OpenModal returned %v, want the dialog's own handle %v", opened, dialog.self)
	}
	if parent != f.Root() {
		t.Errorf("the dialog's parent = %v, want the root %v", parent, f.Root())
	}
}

// The dialog is not in its opener's subtree, so a downward cascade would never
// reach it; the opener edge is what takes it, and it is unmounted before the
// opener for the same reason a child is unmounted before its parent.
func TestAModalLeavesWhenItsOpenerIsDestroyed(t *testing.T) {
	f, a := newModalApp(t)
	var opened froth.Handle
	dialog := &leaver{name: "dialog", log: &a.log}
	opener(a.left, dialog, &opened)
	a.panes.onMsg = func(uc *froth.UpdateCtx, msg tea.Msg) {
		if _, ok := msg.(destroyLeft); ok {
			uc.Child(a.left.self).Destroy()
		}
	}
	send(f, a.left.self, openMsg{})
	f.View()
	if dialog.unmounts != 0 {
		t.Fatal("setup: the dialog left before its opener was asked to")
	}
	a.log = nil

	send(f, a.panes.self, destroyLeft{})
	f.View()
	if dialog.unmounts != 1 {
		t.Errorf("the dialog was unmounted %d times, want once: its opener left", dialog.unmounts)
	}
	di, li := slices.Index(a.log, "unmount:dialog"), slices.Index(a.log, "unmount:left")
	if di < 0 || li < 0 || di > li {
		t.Errorf("unmount order %v, want the dialog before its opener", a.log)
	}
	if got := f.Composite(opened); got.Content != "" {
		t.Errorf("the dialog still composites after its opener left: %q", got.Content)
	}
}

type destroyLeft struct{}

// A modal opened from a modal forms a chain, and the chain goes together.
func TestOpenerEdgesFormAChainThatLeavesTogether(t *testing.T) {
	f, a := newModalApp(t)
	var first, second froth.Handle
	inner := &leaver{name: "inner", log: &a.log}
	outer := &leaver{name: "outer", log: &a.log}
	opener(outer, inner, &second)
	opener(a.left, outer, &first)
	closer(a.left)

	send(f, a.left.self, openMsg{})
	f.View()
	send(f, outer.self, openMsg{})
	f.View()
	if inner.unmounts+outer.unmounts != 0 {
		t.Fatal("setup: something left before the opener was closed")
	}
	a.log = nil

	send(f, a.left.self, closeMsg{})
	f.View()

	if len(a.log) != 4 {
		t.Fatalf("teardown ran %v, want inner, outer, row and left once each", a.log)
	}
	idx := func(name string) int { return slices.Index(a.log, "unmount:"+name) }
	if idx("inner") > idx("outer") || idx("outer") > idx("left") || idx("row") > idx("left") {
		t.Errorf("teardown order = %v, want each modal before its opener and the row "+
			"before its parent", a.log)
	}
}

// Quarantine is one of the four routes out, so a modal opened by a surface
// that panicked goes with it.
func TestAModalLeavesWhenItsOpenerIsQuarantined(t *testing.T) {
	f, a := newModalApp(t)
	var opened froth.Handle
	dialog := &leaver{name: "dialog", log: &a.log}
	opener(a.left, dialog, &opened)
	send(f, a.left.self, openMsg{})
	f.View()

	a.left.panicIn = "Update"
	send(f, a.left.self, plainMsg{})
	f.View()

	if dialog.unmounts != 1 {
		t.Errorf("the dialog was unmounted %d times, want once: its opener was quarantined", dialog.unmounts)
	}
}

// Root's own modal has no opener, so a surface leaving elsewhere in the tree
// does not take it.
func TestAModalTheRootCreatedHasNoOpener(t *testing.T) {
	f, a := newModalApp(t)
	dialog := &leaver{name: "dialog", log: &a.log}
	a.root.onReconcile = func(ac *froth.ApplyCtx) { ac.Self().Create(dialog, froth.Modal()) }
	on(a.root, func(uc *froth.UpdateCtx, _ openMsg) { uc.Self().Reconcile() })
	closer(a.left)
	f.Update(openMsg{})
	f.View()

	send(f, a.left.self, closeMsg{})
	f.View()

	if dialog.unmounts != 0 {
		t.Errorf("root's own modal left when an unrelated surface did")
	}
}

// Every modal is a scope: with no FocusScope option at all, tab wraps within
// the dialog rather than walking out to a surface whose keys would be dropped.
func TestAModalBoundsTraversalWithoutBeingDeclaredAScope(t *testing.T) {
	f, a := newModalApp(t)
	var opened froth.Handle
	ok := &leaver{name: "ok", log: &a.log}
	cancel := &leaver{name: "cancel", log: &a.log}
	dialog := &leaver{name: "dialog", noStop: true, kids: []froth.Surface{ok, cancel}, log: &a.log}
	opener(a.left, dialog, &opened)
	send(f, a.left.self, openMsg{})
	f.View()

	f.Update(focusMsg{to: cancel.self})
	f.Update(tabPress())

	if got := f.Query().Focus(); got != ok.self {
		t.Errorf("tab from the dialog's last control landed on %v, want its first %v", got, ok.self)
	}
}

// Opening a modal moves focus into it, onto its first tab stop — which exists
// only once the modal has mounted, so the move waits for that.
func TestOpeningAModalMovesFocusIntoIt(t *testing.T) {
	f, a := newModalApp(t)
	var opened froth.Handle
	ok := &leaver{name: "ok", log: &a.log}
	dialog := &leaver{name: "dialog", noStop: true, kids: []froth.Surface{ok}, log: &a.log}
	opener(a.left, dialog, &opened)

	send(f, a.left.self, openMsg{})

	if got := f.Query().Focus(); got != ok.self {
		t.Errorf("focus after opening = %v, want the dialog's first stop %v", got, ok.self)
	}
}

// A Mount that focuses a child of its own is the later decision, and stands.
func TestAModalThatFocusesItsOwnChildOnMountWins(t *testing.T) {
	f, a := newModalApp(t)
	var opened froth.Handle
	ok := &leaver{name: "ok", log: &a.log}
	// cancel admits focus by no route, so a redirect into the dialog could
	// never land on it: only Mount's own move can.
	cancel := &leaver{name: "cancel", noStop: true, log: &a.log}
	dialog := &leaver{name: "dialog", noStop: true, kids: []froth.Surface{ok, cancel}, log: &a.log}
	dialog.onMount = func(ac *froth.ApplyCtx) { ac.At(dialog.kidHs[1]).Focus() }
	opener(a.left, dialog, &opened)

	send(f, a.left.self, openMsg{})

	if got := f.Query().Focus(); got != cancel.self {
		t.Errorf("focus after opening = %v, want what Mount chose %v", got, cancel.self)
	}
}

// A modal holding nothing focusable holds focus itself.
func TestAModalWithNoStopHoldsFocusItself(t *testing.T) {
	f, a := newModalApp(t)
	var opened froth.Handle
	dialog := &leaver{name: "dialog", noStop: true, log: &a.log}
	opener(a.left, dialog, &opened)

	send(f, a.left.self, openMsg{})

	if got := f.Query().Focus(); got != dialog.self {
		t.Errorf("focus after opening = %v, want the dialog itself %v", got, dialog.self)
	}
}

// Closing a modal puts focus back where it was when the modal opened.
func TestClosingAModalRestoresFocus(t *testing.T) {
	f, a := newModalApp(t)
	var opened froth.Handle
	ok := &leaver{name: "ok", log: &a.log}
	dialog := &leaver{name: "dialog", noStop: true, kids: []froth.Surface{ok}, log: &a.log}
	closer(dialog)
	opener(a.left, dialog, &opened)
	// The right pane rather than the row: the row is also where a broken chain
	// falls back to, so landing there would not show a restore happened.
	f.Update(focusMsg{to: a.right.self})
	send(f, a.left.self, openMsg{})
	if got := f.Query().Focus(); got != ok.self {
		t.Fatalf("setup: focus = %v, want inside the dialog", got)
	}

	send(f, dialog.self, closeMsg{})

	if got := f.Query().Focus(); got != a.right.self {
		t.Errorf("focus after closing = %v, want the pane it was on %v", got, a.right.self)
	}
}

// Focus moved behind the modal by name while it was open is a choice the
// application made, and closing the modal does not undo it.
func TestClosingAModalLeavesFocusMovedBehindItAlone(t *testing.T) {
	f, a := newModalApp(t)
	var opened froth.Handle
	ok := &leaver{name: "ok", log: &a.log}
	dialog := &leaver{name: "dialog", noStop: true, kids: []froth.Surface{ok}, log: &a.log}
	closer(dialog)
	opener(a.left, dialog, &opened)
	send(f, a.left.self, openMsg{})
	f.Update(focusMsg{to: a.right.self})

	send(f, dialog.self, closeMsg{})

	if got := f.Query().Focus(); got != a.right.self {
		t.Errorf("focus after closing = %v, want where the application put it %v", got, a.right.self)
	}
}

// The saved position may have died while the dialog was open; focus then goes
// where any broken chain goes, near the survivors, with nothing reported.
func TestClosingAModalWhoseSavedPositionDiedRehomes(t *testing.T) {
	f, a, seen := newWatchedModalApp(t)
	var opened froth.Handle
	ok := &leaver{name: "ok", log: &a.log}
	dialog := &leaver{name: "dialog", noStop: true, kids: []froth.Surface{ok}, log: &a.log}
	closer(dialog)
	opener(a.left, dialog, &opened)
	a.left.onMsg = func(uc *froth.UpdateCtx, msg tea.Msg) {
		switch msg.(type) {
		case openMsg:
			uc.Self().Reconcile()
		case plainMsg:
			uc.Child(a.row.self).Destroy()
		}
	}
	send(f, a.left.self, openMsg{})
	send(f, a.left.self, plainMsg{})

	send(f, dialog.self, closeMsg{})

	if got := f.Query().Focus(); got != a.right.self {
		t.Errorf("focus after closing = %v, want the surviving stop %v", got, a.right.self)
	}
	if len(*seen) != 0 {
		t.Errorf("rehoming reported %v, want nothing: nobody asked for a move", kinds(*seen))
	}
}

// A dialog closing from two deep takes the one it opened, and focus returns to
// where the first dialog was opened from — through the inner dialog's saved
// position, which is inside the outer one and leaving with it. Without that
// resolution focus would fall back to the root's first stop, the row.
func TestClosingAnOuterModalRestoresPastTheInnerOne(t *testing.T) {
	f, a := newModalApp(t)
	var first, second froth.Handle
	inner := &leaver{name: "inner", log: &a.log}
	ok := &leaver{name: "ok", log: &a.log}
	outer := &leaver{name: "outer", noStop: true, kids: []froth.Surface{ok}, log: &a.log}
	closer(outer)
	opener(outer, inner, &second)
	opener(a.left, outer, &first)
	f.Update(focusMsg{to: a.right.self})
	send(f, a.left.self, openMsg{})
	send(f, outer.self, openMsg{})
	f.View()
	if got := f.Query().Focus(); got != inner.self {
		t.Fatalf("setup: focus = %v, want the inner dialog %v", got, inner.self)
	}

	send(f, outer.self, closeMsg{})

	if inner.unmounts != 1 {
		t.Errorf("the inner dialog was unmounted %d times, want once", inner.unmounts)
	}
	if got := f.Query().Focus(); got != a.right.self {
		t.Errorf("focus after closing both = %v, want the pane the first was opened from %v", got, a.right.self)
	}
}

// A modal created and destroyed within one Apply is never focused and never
// restored from: there is nothing to do and nothing to trip over.
func TestAModalThatLeavesInTheApplyItWasOpenedInIsNoEvent(t *testing.T) {
	f, a := newModalApp(t)
	dialog := &leaver{name: "dialog", log: &a.log}
	a.root.onReconcile = func(ac *froth.ApplyCtx) {
		h := ac.Self().OpenModal(dialog)
		ac.Child(h).Destroy()
	}
	on(a.root, func(uc *froth.UpdateCtx, _ openMsg) { uc.Self().Reconcile() })

	f.Update(openMsg{})

	if got := f.Query().Focus(); got != a.row.self {
		t.Errorf("focus = %v, want unchanged on the row %v", got, a.row.self)
	}
}

// Capture goes with the holder on every route out, and a modal leaving by its
// opener edge is one of them: the next click is hit-tested, not sent to a
// surface that has gone.
func TestAModalHoldingCaptureLosesItWhenItsOpenerLeaves(t *testing.T) {
	f, a := newModalApp(t)
	var opened froth.Handle
	dialog := &leaver{name: "dialog", log: &a.log}
	dialog.onMsg = func(uc *froth.UpdateCtx, msg tea.Msg) {
		if _, ok := msg.(plainMsg); ok {
			uc.Self().Capture()
		}
	}
	closer(a.left)
	var clicks int
	a.right.onMsg = func(uc *froth.UpdateCtx, msg tea.Msg) {
		if _, ok := msg.(tea.MouseClickMsg); ok {
			clicks++
		}
	}
	opener(a.left, dialog, &opened)
	send(f, a.left.self, openMsg{})
	f.View()
	send(f, dialog.self, plainMsg{})
	send(f, a.left.self, closeMsg{})
	f.View()

	f.Update(click(0, 0))

	if clicks != 1 {
		t.Errorf("the surviving pane saw %d clicks, want 1: capture left with the dialog", clicks)
	}
}

// A modal whose Declare panics leaves by quarantine like any other surface,
// and froth carries on with focus where it was.
func TestAModalThatPanicsDeclaringIsQuarantined(t *testing.T) {
	f, a, seen := newWatchedModalApp(t)
	var opened froth.Handle
	dialog := &leaver{name: "dialog", panicIn: "Declare", log: &a.log}
	opener(a.left, dialog, &opened)

	send(f, a.left.self, openMsg{})

	if !hasKind(*seen, froth.DiagSurfacePanic) {
		t.Errorf("diagnostics = %v, want the panic reported", kinds(*seen))
	}
	if got := f.Query().Focus(); got != a.row.self {
		t.Errorf("focus = %v, want unchanged on the row %v", got, a.row.self)
	}
}

// Options after the implied Modal are the caller's and reach the modal as
// they would through Create; passing Modal again is harmless. A transparent
// modal passes a click through to surfaces it blocks, so nobody sees it —
// where the same dialog without the option is what the click lands on.
func TestOpenModalPassesTheCallersOptionsThrough(t *testing.T) {
	for _, tc := range []struct {
		name string
		opts []froth.CreateOption
		want int
	}{
		{"plain", nil, 1},
		{"transparent", []froth.CreateOption{froth.MouseTransparent(), froth.Modal()}, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f, a := newModalApp(t)
			var opened froth.Handle
			dialog := &leaver{name: "dialog", log: &a.log}
			clicks := 0
			on(dialog, func(uc *froth.UpdateCtx, _ tea.MouseClickMsg) { clicks++ })
			opener(a.left, dialog, &opened, tc.opts...)
			send(f, a.left.self, openMsg{})
			f.View()

			f.Update(click(0, 0))

			if clicks != tc.want {
				t.Errorf("the dialog saw %d clicks, want %d", clicks, tc.want)
			}
		})
	}
}
