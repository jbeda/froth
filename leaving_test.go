package froth_test

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/jbeda/froth"
)

// leaver is a surface that records every callback froth makes to it and can be
// told to panic in any one of them. The log is shared between a whole tree, so
// the order teardown ran in is a slice rather than something to reconstruct.
type leaver struct {
	name  string
	kids  []froth.Surface
	log   *[]string
	self  froth.Handle
	kidHs []froth.Handle

	noStop    bool
	panicIn   string
	declares  []froth.Binding
	onDraw    func(rc *froth.RenderCtx)
	onMsg     func(uc *froth.UpdateCtx, msg tea.Msg)
	onMount   func(ac *froth.ApplyCtx)
	onUnmount func(ac *froth.ApplyCtx)

	updates, renders, unmounts int
}

func (l *leaver) note(what string) {
	if l.log != nil {
		*l.log = append(*l.log, what+":"+l.name)
	}
}

func (l *leaver) Render(rc *froth.RenderCtx, r froth.Rect) string {
	l.renders++
	if l.panicIn == "Render" {
		panic("render broke")
	}
	if l.onDraw != nil {
		l.onDraw(rc)
	}
	return "."
}

func (l *leaver) Update(uc *froth.UpdateCtx, msg tea.Msg) {
	l.updates++
	if l.panicIn == "Update" {
		panic("update broke")
	}
	if l.onMsg != nil {
		l.onMsg(uc, msg)
	}
}

func (l *leaver) Mount(ac *froth.ApplyCtx) {
	l.self = ac.Self().Handle()
	ac.Self().SetBindings(l.declares)
	for _, k := range l.kids {
		var opts []froth.CreateOption
		if kl, ok := k.(*leaver); !ok || !kl.noStop {
			opts = append(opts, froth.TabStop())
		}
		l.kidHs = append(l.kidHs, ac.Self().Create(k, opts...))
	}
	l.note("mount")
	if l.panicIn == "Mount" {
		panic("mount broke")
	}
	if l.onMount != nil {
		l.onMount(ac)
	}
}

func (l *leaver) Unmount(ac *froth.ApplyCtx) {
	l.unmounts++
	l.note("unmount")
	if l.panicIn == "Unmount" {
		panic("unmount broke")
	}
	if l.onUnmount != nil {
		l.onUnmount(ac)
	}
}

// kinds reports the diagnostic kinds seen, in order, so a test says which
// findings it expected rather than matching prose.
func kinds(got []froth.Diagnostic) []froth.DiagnosticKind {
	out := make([]froth.DiagnosticKind, len(got))
	for i, d := range got {
		out[i] = d.Kind
	}
	return out
}

func hasKind(got []froth.Diagnostic, k froth.DiagnosticKind) bool {
	for _, d := range got {
		if d.Kind == k {
			return true
		}
	}
	return false
}

// Destroying a child takes its descendants with it. There is no deep destroy:
// a surface names its own child and the cascade reaches the rest.
func TestDestroyingAChildRemovesItsWholeSubtree(t *testing.T) {
	var log []string
	deep := &leaver{name: "c", log: &log}
	mid := &leaver{name: "b", log: &log, kids: []froth.Surface{deep}}
	top := &leaver{name: "a", log: &log, kids: []froth.Surface{mid}}
	var kids []froth.Handle
	root := &leaver{name: "root", log: &log, kids: []froth.Surface{top}}
	root.onMsg = func(uc *froth.UpdateCtx, msg tea.Msg) {
		if _, ok := msg.(bumpMsg); ok {
			uc.Child(top.self).Destroy()
			return
		}
		kids = uc.Self().Children()
	}
	f := newSized(t, root, 4, 1)

	f.Update(bumpMsg{})
	f.Update(plainMsg{})

	if len(kids) != 0 {
		t.Errorf("the root still has %d children after destroying its only one", len(kids))
	}
	if got, want := strings.Join(log[len(log)-3:], " "), "unmount:c unmount:b unmount:a"; got != want {
		t.Errorf("teardown ran %q, want %q", got, want)
	}
}

// Self-destroy is the same operation named from the other end, and it records
// like every other structural change asked for during Deliver.
func TestASurfaceDestroysItselfAndCommitsAtTheFollowingApply(t *testing.T) {
	var log []string
	kid := &leaver{name: "kid", log: &log}
	var duringDelivery []froth.Handle
	root := &leaver{name: "root", log: &log, kids: []froth.Surface{kid}}
	var rootH froth.Handle
	kid.onMsg = func(uc *froth.UpdateCtx, msg tea.Msg) {
		uc.Self().Destroy()
		uc.At(rootH).Post(checkMsg{})
	}
	root.onMsg = func(uc *froth.UpdateCtx, msg tea.Msg) {
		rootH = uc.Self().Handle()
		switch msg.(type) {
		case startMsg:
			uc.Child(kid.self).Post(plainMsg{})
		case checkMsg:
			duringDelivery = uc.Self().Children()
		}
	}
	f := newSized(t, root, 4, 1)

	f.Update(startMsg{})

	if len(duringDelivery) != 1 {
		t.Errorf("the child had already gone during the delivery that destroyed it")
	}
	var after []froth.Handle
	root.onMsg = func(uc *froth.UpdateCtx, msg tea.Msg) { after = uc.Self().Children() }
	f.Update(startMsg{})
	if len(after) != 0 {
		t.Errorf("the child was still there after the Apply that should have removed it")
	}
	if kid.unmounts != 1 {
		t.Errorf("kid unmounted %d times, want 1", kid.unmounts)
	}
}

// Apply is where structural change commits, so a destroy asked for there has
// nothing to wait for.
func TestDestroyingDuringApplyTakesEffectAtOnce(t *testing.T) {
	var seen []froth.Handle
	doomed := &leaver{name: "doomed"}
	keeper := &leaver{name: "keeper"}
	root := &leaver{name: "root", kids: []froth.Surface{doomed, keeper}}
	root.onMount = func(ac *froth.ApplyCtx) {
		ac.Child(root.kidHs[0]).Destroy()
		seen = ac.Self().Children()
	}
	newSized(t, root, 4, 1)

	if len(seen) != 1 || seen[0] != root.kidHs[1] {
		t.Errorf("children immediately after an Apply-time destroy = %v, want just %v",
			seen, root.kidHs[1])
	}
	// Teardown mirrors assembly, so a surface that left before its mount step
	// ever reached it has nothing to undo and is not asked to.
	if doomed.unmounts != 0 {
		t.Errorf("a surface destroyed before it mounted unmounted %d times, want 0",
			doomed.unmounts)
	}
}

// The two ways of naming a handle wrongly get the two postures ADR 0014 sets:
// reaching past your own children is a bug, and a surface that has gone is the
// condition a handle exists to survive.
func TestDestroyingAHandleThatIsNotAnImmediateChildPanics(t *testing.T) {
	grandkid := &leaver{name: "g"}
	kid := &leaver{name: "k", kids: []froth.Surface{grandkid}}
	root := &leaver{name: "root", kids: []froth.Surface{kid}}
	var got any
	root.onMsg = func(uc *froth.UpdateCtx, msg tea.Msg) {
		defer func() { got = recover() }()
		uc.Child(grandkid.self).Destroy()
	}
	f := newSized(t, root, 4, 1)

	f.Update(plainMsg{})

	if !strings.Contains(panicText(got), "not an immediate child") {
		t.Errorf("panic was %v, want one naming the ownership rule", got)
	}
}

func TestDestroyingAnAlreadyDeadHandleIsADiagnosticAndADrop(t *testing.T) {
	kid := &leaver{name: "kid"}
	root := &leaver{name: "root", kids: []froth.Surface{kid}}
	var stale froth.Handle
	root.onMsg = func(uc *froth.UpdateCtx, msg tea.Msg) {
		if _, ok := msg.(bumpMsg); ok {
			stale = kid.self
			uc.Child(stale).Destroy()
			return
		}
		uc.Child(stale).Destroy()
	}
	f, diags := newWatched(t, root, 4, 1)

	f.Update(bumpMsg{})
	*diags = nil
	f.Update(plainMsg{})

	if !hasKind(*diags, froth.DiagDeadHandle) {
		t.Errorf("destroying a dead handle reported %v, want a dead handle", kinds(*diags))
	}
	if kid.unmounts != 1 {
		t.Errorf("the child unmounted %d times, want 1", kid.unmounts)
	}
}

// Nobody created the root, so no surface can mint an accessor that names it as
// a child — and asking for it through Self is the one way to reach the rule.
func TestTheRootCannotBeDestroyed(t *testing.T) {
	root := &leaver{name: "root"}
	var got any
	root.onMsg = func(uc *froth.UpdateCtx, msg tea.Msg) {
		defer func() { got = recover() }()
		uc.Self().Destroy()
	}
	f := newSized(t, root, 4, 1)

	f.Update(plainMsg{})

	if !strings.Contains(panicText(got), "root surface cannot be destroyed") {
		t.Errorf("panic was %v, want one refusing to destroy the root", got)
	}
}

// A handle outlives what it names. That is the whole reason a surface can store
// one without asking whether its target is still there.
func TestPostingToADestroyedSurfaceIsAReportedDrop(t *testing.T) {
	kid := &leaver{name: "kid"}
	root := &leaver{name: "root", kids: []froth.Surface{kid}}
	var stale froth.Handle
	root.onMsg = func(uc *froth.UpdateCtx, msg tea.Msg) {
		if _, ok := msg.(bumpMsg); ok {
			stale = kid.self
			uc.Child(stale).Destroy()
			return
		}
		uc.At(stale).Post(bumpMsg{})
	}
	f, diags := newWatched(t, root, 4, 1)

	f.Update(bumpMsg{})
	before := kid.updates
	*diags = nil
	f.Update(plainMsg{})

	if !hasKind(*diags, froth.DiagDeadHandle) {
		t.Errorf("posting to a destroyed surface reported %v, want a dead handle", kinds(*diags))
	}
	if kid.updates != before {
		t.Error("a destroyed surface was still delivered to")
	}
}

// Focus is where the keyboard goes, so a destroy that breaks the chain has to
// put it somewhere real — and near what the user was looking at, not at the
// root of the application.
func TestFocusLandsOnTheNearestSurvivorWhenItsHolderIsDestroyed(t *testing.T) {
	list := &leaver{name: "list"}
	right := &leaver{name: "right", kids: []froth.Surface{list}}
	left := &leaver{name: "left"}
	panes := &leaver{name: "panes", noStop: true, kids: []froth.Surface{left, right}}
	root := &leaver{name: "root", kids: []froth.Surface{panes}}
	root.onMsg = func(uc *froth.UpdateCtx, msg tea.Msg) {
		switch msg.(type) {
		case bumpMsg:
			uc.At(list.self).Focus()
		case plainMsg:
			uc.At(panes.self).Post(destroyRight{})
		}
	}
	panes.onMsg = func(uc *froth.UpdateCtx, msg tea.Msg) {
		if _, ok := msg.(destroyRight); ok {
			uc.Child(right.self).Destroy()
		}
	}
	f := newSized(t, root, 4, 1)

	f.Update(bumpMsg{})
	if got := f.Query().Focus(); got != list.self {
		t.Fatalf("focus = %v, want the list %v", got, list.self)
	}
	f.Update(plainMsg{})

	if got := f.Query().Focus(); got != left.self {
		t.Errorf("focus after destroying its holder = %v, want the surviving "+
			"sibling %v rather than the root %v", got, left.self, f.Root())
	}
}

type destroyRight struct{}

// Teardown is the mirror of assembly, and it runs the other way round so a
// child can still reach its parent while releasing what it holds.
func TestUnmountRunsLeafUp(t *testing.T) {
	var log []string
	var parentSeen froth.Handle
	deep := &leaver{name: "deep", log: &log}
	mid := &leaver{name: "mid", log: &log, kids: []froth.Surface{deep}}
	root := &leaver{name: "root", log: &log, kids: []froth.Surface{mid}}
	deep.onUnmount = func(ac *froth.ApplyCtx) { parentSeen = ac.Self().Parent() }
	root.onMsg = func(uc *froth.UpdateCtx, msg tea.Msg) { uc.Child(mid.self).Destroy() }
	f := newSized(t, root, 4, 1)

	f.Update(plainMsg{})

	if got, want := strings.Join(log[len(log)-2:], " "), "unmount:deep unmount:mid"; got != want {
		t.Errorf("teardown ran %q, want %q", got, want)
	}
	if parentSeen != mid.self {
		t.Errorf("a surface tearing down saw its parent as %v, want %v", parentSeen, mid.self)
	}
}

// Whichever route a surface leaves by, teardown runs once. Shutdown is the
// route that used to have none at all.
func TestShutdownRunsTheSameWalkOnceFromTheRoot(t *testing.T) {
	var log []string
	deep := &leaver{name: "deep", log: &log}
	mid := &leaver{name: "mid", log: &log, kids: []froth.Surface{deep}}
	root := &leaver{name: "root", log: &log, kids: []froth.Surface{mid}}
	f := newSized(t, root, 4, 1)

	f.Shutdown()
	f.Shutdown()

	if got, want := strings.Join(log[len(log)-3:], " "), "unmount:deep unmount:mid unmount:root"; got != want {
		t.Errorf("shutdown ran %q, want %q", got, want)
	}
	for _, l := range []*leaver{root, mid, deep} {
		if l.unmounts != 1 {
			t.Errorf("%s unmounted %d times, want 1", l.name, l.unmounts)
		}
	}
	if got := f.View().Content; got != "" {
		t.Errorf("a shut-down instance drew %q, want nothing", got)
	}
}

// A surface destroyed once does not unmount again at shutdown.
func TestUnmountRunsExactlyOncePerSurfaceAcrossDestroyAndShutdown(t *testing.T) {
	kid := &leaver{name: "kid"}
	root := &leaver{name: "root", kids: []froth.Surface{kid}}
	root.onMsg = func(uc *froth.UpdateCtx, msg tea.Msg) { uc.Child(kid.self).Destroy() }
	f := newSized(t, root, 4, 1)

	f.Update(plainMsg{})
	f.Shutdown()

	if kid.unmounts != 1 {
		t.Errorf("the child unmounted %d times, want 1", kid.unmounts)
	}
}

func TestPostingDuringShutdownIsReportedAndDropped(t *testing.T) {
	kid := &leaver{name: "kid"}
	root := &leaver{name: "root", kids: []froth.Surface{kid}}
	kid.onUnmount = func(ac *froth.ApplyCtx) { ac.Self().Post(bumpMsg{}) }
	f, diags := newWatched(t, root, 4, 1)
	*diags = nil

	f.Shutdown()

	if !hasKind(*diags, froth.DiagPostAfterShutdown) {
		t.Errorf("posting during shutdown reported %v, want a post after shutdown", kinds(*diags))
	}
	if kid.updates != 0 {
		t.Error("a message posted during shutdown was delivered")
	}
}

// The destroy commits after the drain, so a surface stays live for the whole
// turn it asked to leave in.
func TestAMessagePostedToASurfaceThatDestroyedItselfIsStillDelivered(t *testing.T) {
	kid := &leaver{name: "kid"}
	root := &leaver{name: "root", kids: []froth.Surface{kid}}
	kid.onMsg = func(uc *froth.UpdateCtx, msg tea.Msg) {
		if _, ok := msg.(plainMsg); ok {
			uc.Self().Destroy()
		}
	}
	root.onMsg = func(uc *froth.UpdateCtx, msg tea.Msg) {
		uc.At(kid.self).Post(plainMsg{})
		uc.At(kid.self).Post(bumpMsg{})
	}
	f := newSized(t, root, 4, 1)

	f.Update(startMsg{})

	if kid.updates != 2 {
		t.Errorf("the destroying surface saw %d messages, want both of them", kid.updates)
	}
}

type startMsg struct{}

type checkMsg struct{}

// A surface that panics does not take the terminal with it. It is reported,
// removed by the ordinary destroy path, and never called again — including its
// own teardown, which is code from the same surface that has already failed.
func TestASurfaceThatPanicsIsReportedAndRemoved(t *testing.T) {
	for _, in := range []string{"Update", "Render", "Mount"} {
		t.Run(in, func(t *testing.T) {
			kid := &leaver{name: "kid", panicIn: in}
			root := &leaver{name: "root", kids: []froth.Surface{kid}}
			root.onMsg = func(uc *froth.UpdateCtx, msg tea.Msg) {
				uc.At(kid.self).Focus()
				uc.At(kid.self).Post(plainMsg{})
			}
			f, diags := newWatched(t, root, 4, 1)
			f.View()
			f.Update(startMsg{})
			f.Update(tea.KeyPressMsg{Code: 'q', Text: "q"})
			f.View()

			if !hasKind(*diags, froth.DiagSurfacePanic) {
				t.Fatalf("a panic in %s reported %v, want a surface panic", in, kinds(*diags))
			}
			var kids []froth.Handle
			root.onMsg = func(uc *froth.UpdateCtx, msg tea.Msg) { kids = uc.Self().Children() }
			f.Update(startMsg{})
			if len(kids) != 0 {
				t.Errorf("the panicking surface is still in the tree")
			}
			if kid.unmounts != 0 {
				t.Errorf("a quarantined surface was asked to unmount")
			}
		})
	}
}

func TestAPanickingSurfaceIsNotCalledAgain(t *testing.T) {
	kid := &leaver{name: "kid", panicIn: "Update"}
	root := &leaver{name: "root", kids: []froth.Surface{kid}}
	root.onMsg = func(uc *froth.UpdateCtx, msg tea.Msg) {
		uc.At(kid.self).Post(plainMsg{})
		uc.At(kid.self).Post(bumpMsg{})
	}
	f, _ := newWatched(t, root, 4, 1)
	f.View()
	renders := kid.renders

	f.Update(startMsg{})
	f.View()
	f.Update(startMsg{})

	if kid.updates != 1 {
		t.Errorf("the panicking surface saw %d messages, want only the one it "+
			"panicked on", kid.updates)
	}
	if kid.renders != renders {
		t.Errorf("the panicking surface was drawn %d more times after it was "+
			"quarantined", kid.renders-renders)
	}
}

// A surface panicking below a parent takes its subtree with it, because
// quarantine removes it by the same path as any other destroy.
func TestQuarantineCascadesToTheSubtree(t *testing.T) {
	grandkid := &leaver{name: "g"}
	kid := &leaver{name: "kid", panicIn: "Update", kids: []froth.Surface{grandkid}}
	root := &leaver{name: "root", kids: []froth.Surface{kid}}
	root.onMsg = func(uc *froth.UpdateCtx, msg tea.Msg) { uc.At(kid.self).Post(plainMsg{}) }
	f, _ := newWatched(t, root, 4, 1)

	f.Update(startMsg{})
	f.Update(startMsg{})

	if grandkid.unmounts != 1 {
		t.Errorf("the child of a quarantined surface unmounted %d times, want 1",
			grandkid.unmounts)
	}
}

// Quarantine is for surfaces that failed, not for programs that violated
// froth's contract. Burying one of those in a diagnostic would leave the bug in
// place and report it far from its cause.
func TestFrothsOwnPanicIsNotQuarantined(t *testing.T) {
	grandkid := &leaver{name: "g"}
	kid := &leaver{name: "kid", kids: []froth.Surface{grandkid}}
	root := &leaver{name: "root", kids: []froth.Surface{kid}}
	root.onMsg = func(uc *froth.UpdateCtx, msg tea.Msg) {
		uc.Child(grandkid.self).Destroy()
	}
	f, diags := newWatched(t, root, 4, 1)

	var got any
	func() {
		defer func() { got = recover() }()
		f.Update(startMsg{})
	}()

	if !strings.Contains(panicText(got), "not an immediate child") {
		t.Fatalf("froth's own panic came back as %v; it should reach the caller", got)
	}
	if hasKind(*diags, froth.DiagSurfacePanic) {
		t.Error("froth's own panic was reported as a surface panic")
	}
}

// A focus move recorded during Deliver is validated at the Apply that commits
// it, and the surface may have left the tree in between.
func TestFocusingASurfaceDestroyedInTheSameTurnIsRefused(t *testing.T) {
	kid := &leaver{name: "kid"}
	root := &leaver{name: "root", kids: []froth.Surface{kid}}
	root.onMsg = func(uc *froth.UpdateCtx, msg tea.Msg) {
		uc.At(kid.self).Focus()
		uc.Child(kid.self).Destroy()
	}
	f, diags := newWatched(t, root, 4, 1)
	*diags = nil

	f.Update(startMsg{})

	if !hasKind(*diags, froth.DiagInvalidFocus) {
		t.Errorf("focusing a surface destroyed in the same turn reported %v, "+
			"want an invalid focus", kinds(*diags))
	}
	if got := f.Query().Focus(); got != f.Root() {
		t.Errorf("focus = %v, want the root %v", got, f.Root())
	}
}

// A surface that panics while drawing takes its subtree off the frame with it.
// froth has nothing to composite the children over and no reason to believe
// what the parent would have said about where they go.
func TestASurfaceThatPanicsWhileDrawingHidesItsSubtree(t *testing.T) {
	kid := &leaver{name: "kid"}
	broken := &leaver{name: "broken", panicIn: "Render", kids: []froth.Surface{kid}}
	root := &leaver{name: "root", kids: []froth.Surface{broken}}
	f, diags := newWatched(t, root, 4, 1)

	frame := f.Composite(f.Root())

	if !hasKind(*diags, froth.DiagSurfacePanic) {
		t.Fatalf("a panic while drawing reported %v, want a surface panic", kinds(*diags))
	}
	if _, ok := frame.Rects[broken.self]; ok {
		t.Error("the surface that panicked was placed in the frame")
	}
	if _, ok := frame.Rects[kid.self]; ok {
		t.Error("a child of the surface that panicked was drawn")
	}
	if kid.renders != 0 {
		t.Errorf("the child drew %d times, want 0", kid.renders)
	}
}

// Teardown may itself destroy something, and what it names may already be
// inside the removal that is running. Unmount runs once per surface, so the
// second request is a no-op rather than a second teardown.
func TestDestroyingFromInsideUnmountDoesNotTearDownTwice(t *testing.T) {
	grandkid := &leaver{name: "g"}
	kid := &leaver{name: "kid", kids: []froth.Surface{grandkid}}
	root := &leaver{name: "root", kids: []froth.Surface{kid}}
	kid.onUnmount = func(ac *froth.ApplyCtx) { ac.Child(grandkid.self).Destroy() }
	root.onMsg = func(uc *froth.UpdateCtx, msg tea.Msg) { uc.Child(kid.self).Destroy() }
	f := newSized(t, root, 4, 1)

	f.Update(startMsg{})

	if grandkid.unmounts != 1 {
		t.Errorf("the grandchild unmounted %d times, want 1", grandkid.unmounts)
	}
}

// Nothing destroys the root — no surface can name it. Quarantine is not a
// surface asking, and it has to take the root out or go on calling something
// that has already failed. What is left is an instance with an empty tree,
// which draws nothing and delivers nothing to anybody.
func TestARootThatPanicsEndsTheInstance(t *testing.T) {
	kid := &leaver{name: "kid"}
	root := &leaver{name: "root", panicIn: "Update", kids: []froth.Surface{kid}}
	f, diags := newWatched(t, root, 4, 1)

	f.Update(startMsg{})

	if !hasKind(*diags, froth.DiagSurfacePanic) {
		t.Fatalf("a panicking root reported %v, want a surface panic", kinds(*diags))
	}
	if kid.unmounts != 1 {
		t.Errorf("the root's child unmounted %d times, want 1", kid.unmounts)
	}
	before := root.updates
	f.Update(startMsg{})
	if root.updates != before {
		t.Error("a quarantined root was asked again")
	}
	if got := f.View().Content; got != "" {
		t.Errorf("an instance whose root panicked drew %q, want nothing", got)
	}
}

// A quarantined surface's declarations are still on its node — the destroy that
// removes it commits at the next Apply — so the walk has to skip it. Pulling
// skipped it by not making the call; storing means skipping a read of live
// data, which is a different mechanism for the same rule and needs its own test.
func TestAQuarantinedSurfaceStopsContributingItsBindings(t *testing.T) {
	broken := &leaver{
		name:     "broken",
		panicIn:  "Update",
		declares: []froth.Binding{{Action: "Q", Keys: []string{"q"}, Help: "quit"}},
	}
	asking := &leaver{name: "asking"}
	root := &leaver{name: "root", kids: []froth.Surface{broken, asking}}
	root.onMount = func(ac *froth.ApplyCtx) { ac.Child(root.kidHs[0]).Focus() }

	var live []froth.ChainBinding
	asking.onMsg = func(uc *froth.UpdateCtx, msg tea.Msg) { live = uc.Query().LiveBindings() }
	root.onMsg = func(uc *froth.UpdateCtx, msg tea.Msg) {
		uc.At(broken.self).Post(plainMsg{})
		uc.At(asking.self).Post(checkMsg{})
	}
	f, diags := newWatched(t, root, 4, 1)

	f.Update(startMsg{})

	if !hasKind(*diags, froth.DiagSurfacePanic) {
		t.Fatalf("reported %v, want a surface panic", kinds(*diags))
	}
	if reports(live, "Q") {
		t.Error("a quarantined surface is still contributing its bindings to " +
			"the chain, so a key would resolve to a surface froth will not call")
	}
}
