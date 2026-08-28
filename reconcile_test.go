package froth_test

import (
	"slices"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/jbeda/froth"
)

// tabs is a container whose children are a function of a list it keeps. The
// list is the desired state and match makes the tree match it, which is the
// shape a reconcile is meant to be written in: no journal of what has been
// asked for, only what should be there now.
type tabs struct {
	want []string
	live map[string]froth.Handle
	log  *[]string

	runs, mounts int

	// askAgain is how many further reconciles to ask for from inside one, and
	// alwaysAsk is the cascade that never settles.
	askAgain    int
	alwaysAsk   bool
	panicIn     string
	askOnMount  bool
	onReconcile func(s *tabs, ac *froth.ApplyCtx)
}

type wantMsg struct{ names []string }
type askTwiceMsg struct{}

// relayMsg asks for a reconcile and posts a second message that asks again, so
// the two requests come from two deliveries rather than from one handler.
type relayMsg struct{}

func (s *tabs) Render(rc *froth.RenderCtx, size froth.Size) string {
	return strings.Repeat(".", size.W)
}

func (s *tabs) Update(uc *froth.UpdateCtx, msg tea.Msg) {
	switch m := msg.(type) {
	case wantMsg:
		s.want = m.names
		uc.Self().Reconcile()
	case askTwiceMsg:
		uc.Self().Reconcile()
		uc.Self().Reconcile()
	case relayMsg:
		uc.Self().Reconcile()
		uc.Self().Post(askTwiceMsg{})
	}
}

func (s *tabs) Mount(ac *froth.ApplyCtx) {
	s.mounts++
	s.live = map[string]froth.Handle{}
	ac.Self().SetLayout(columns{})
	s.match(ac)
	if s.askOnMount {
		ac.Self().Reconcile()
	}
}

func (s *tabs) Reconcile(ac *froth.ApplyCtx) {
	s.runs++
	if s.panicIn == "Reconcile" {
		panic("reconcile broke")
	}
	if s.alwaysAsk {
		ac.Self().Reconcile()
	} else if s.askAgain > 0 {
		s.askAgain--
		ac.Self().Reconcile()
	}
	if s.onReconcile != nil {
		s.onReconcile(s, ac)
		return
	}
	s.match(ac)
}

// match creates what is wanted and not there, and destroys what is there and not
// wanted. Everything it needs to know is in want and live.
func (s *tabs) match(ac *froth.ApplyCtx) {
	for name, h := range s.live {
		if !slices.Contains(s.want, name) {
			ac.Child(h).Destroy()
			delete(s.live, name)
		}
	}
	for _, name := range s.want {
		if _, ok := s.live[name]; ok {
			continue
		}
		s.live[name] = ac.Self().Create(&tab{name: name, log: s.log})
	}
}

// tab is one of the strip's children: one cell wide, upper case while focused,
// and noisy about entering and leaving.
type tab struct {
	name string
	log  *[]string
	self froth.Handle
	kid  froth.Surface
}

func (t *tab) Render(rc *froth.RenderCtx, s froth.Size) string {
	if rc.Focused() {
		return strings.ToUpper(t.name)
	}
	return t.name
}

func (t *tab) Update(*froth.UpdateCtx, tea.Msg) {}

func (t *tab) Declare() froth.Flags { return froth.Flags{TabStop: true} }

func (t *tab) Mount(ac *froth.ApplyCtx) {
	t.self = ac.Self().Handle()
	t.note("mount")
	if t.kid != nil {
		ac.Self().Create(t.kid)
	}
}

func (t *tab) Unmount(*froth.ApplyCtx) { t.note("unmount") }

func (t *tab) note(what string) {
	if t.log != nil {
		*t.log = append(*t.log, what+":"+t.name)
	}
}

func content(t *testing.T, f *froth.Froth) string {
	t.Helper()
	return f.View().Content
}

func TestASurfaceAsksToReconcileAndFrothCallsItInTheFollowingApply(t *testing.T) {
	s := &tabs{want: []string{"a"}}
	f := newSized(t, s, 3, 1)

	if got, want := content(t, f), "a.."; got != want {
		t.Fatalf("before = %q, want %q", got, want)
	}

	f.Update(wantMsg{names: []string{"a", "b"}})

	if s.runs != 1 {
		t.Errorf("Reconcile ran %d times, want 1", s.runs)
	}
	if got, want := content(t, f), "ab."; got != want {
		t.Errorf("after = %q, want %q", got, want)
	}
}

func TestAskingToReconcileTwiceBeforeApplyReconcilesOnce(t *testing.T) {
	s := &tabs{want: []string{"a"}}
	f := newSized(t, s, 3, 1)

	f.Update(askTwiceMsg{})

	if s.runs != 1 {
		t.Errorf("Reconcile ran %d times, want 1", s.runs)
	}
}

// The queue drains completely before Apply runs, so two deliveries in one turn
// are still one Apply — and one reconcile.
func TestTwoDeliveriesAskingBeforeOneApplyReconcileOnce(t *testing.T) {
	s := &tabs{want: []string{"a"}}
	f := newSized(t, s, 3, 1)

	f.Update(relayMsg{})

	if s.runs != 1 {
		t.Errorf("Reconcile ran %d times, want 1", s.runs)
	}
}

// Rearranging is the third thing a reconcile may do to its own children, and it
// is SetLayout rather than a reordering call: sibling order is creation order,
// and where the children sit is the layout's answer.
func TestAReconcileRearrangesItsChildrenByAttachingALayout(t *testing.T) {
	s := &tabs{want: []string{"a", "b"}}
	f := newSized(t, s, 3, 1)

	if got, want := content(t, f), "ab."; got != want {
		t.Fatalf("before = %q, want %q", got, want)
	}

	s.onReconcile = func(s *tabs, ac *froth.ApplyCtx) {
		ac.Self().SetLayout(columns{w: []int{2, 1}})
	}
	f.Update(wantMsg{names: []string{"a", "b"}})

	// The second child moved from the middle cell to the last one. The first
	// draws one character into the two it now has, so the strip shows through.
	if got, want := content(t, f), "a.b"; got != want {
		t.Errorf("after = %q, want %q", got, want)
	}
}

func TestReconcileCreatesAndDestroysTheSurfacesOwnChildren(t *testing.T) {
	var log []string
	s := &tabs{want: []string{"a", "b"}, log: &log}
	f := newSized(t, s, 3, 1)

	f.Update(wantMsg{names: []string{"b", "c"}})

	if got, want := content(t, f), "bc."; got != want {
		t.Errorf("frame = %q, want %q", got, want)
	}
	if !slices.Contains(log, "unmount:a") {
		t.Errorf("the removed surface never unmounted; log = %v", log)
	}
	if !slices.Contains(log, "mount:c") {
		t.Errorf("the added surface never mounted; log = %v", log)
	}
}

// The removed surface is gone from the frame, not merely undrawn: nothing it
// owned survives it either.
func TestSurfacesAReconcileRemovesAreGoneFromTheFrame(t *testing.T) {
	s := &tabs{want: []string{"a", "b"}}
	f := newSized(t, s, 3, 1)

	f.Update(wantMsg{names: nil})

	if got, want := content(t, f), "..."; got != want {
		t.Errorf("frame = %q, want %q", got, want)
	}
}

// A surface created during a reconcile mounts in that same Apply, so what its
// own Mount builds is on the very next frame rather than one behind.
func TestASurfaceCreatedDuringAReconcileIsMountedInTheSameApply(t *testing.T) {
	var log []string
	s := &tabs{log: &log}
	s.onReconcile = func(s *tabs, ac *froth.ApplyCtx) {
		ac.Self().Create(&tab{name: "n", log: s.log, kid: &fixed{content: "g"}})
	}
	f := newSized(t, s, 3, 1)

	f.Update(wantMsg{})

	if got, want := content(t, f), "g.."; got != want {
		t.Errorf("frame = %q, want %q; the child's own Mount did not run in time", got, want)
	}
	if !slices.Contains(log, "mount:n") {
		t.Errorf("the created surface never mounted; log = %v", log)
	}
}

func TestAReconcileDoesNotRerunMount(t *testing.T) {
	s := &tabs{want: []string{"a"}}
	f := newSized(t, s, 3, 1)

	f.Update(wantMsg{names: []string{"a", "b"}})
	f.Update(wantMsg{names: []string{"a"}})

	if s.mounts != 1 {
		t.Errorf("Mount ran %d times, want 1", s.mounts)
	}
	if s.runs != 2 {
		t.Errorf("Reconcile ran %d times, want 2", s.runs)
	}
}

// Asking from inside Reconcile is the same request as asking from Deliver: it
// is serviced later in the same Apply rather than by calling back in.
func TestAskingToReconcileFromInsideReconcileRunsAgainWithoutRecursing(t *testing.T) {
	s := &tabs{askAgain: 1}
	f := newSized(t, s, 3, 1)

	f.Update(wantMsg{names: []string{"a"}})

	if s.runs != 2 {
		t.Errorf("Reconcile ran %d times, want 2", s.runs)
	}
	if got, want := content(t, f), "a.."; got != want {
		t.Errorf("frame = %q, want %q", got, want)
	}
}

func TestAReconcileThatNeverStopsAskingTripsTheIterationCap(t *testing.T) {
	s := &tabs{alwaysAsk: true}
	f, diags := newWatched(t, s, 3, 1)

	f.Update(wantMsg{names: []string{"a"}})

	if !hasKind(*diags, froth.DiagIterationCap) {
		t.Fatalf("a reconcile cascade that never settles was not reported; got %v", *diags)
	}
}

// Whatever the cap abandoned, the surface can still ask again: the mark that
// says it is already waiting goes with the request it was recording.
func TestASurfaceCanAskAgainAfterTheCapAbandonedItsRequest(t *testing.T) {
	s := &tabs{alwaysAsk: true}
	f := newSized(t, s, 3, 1)
	f.Update(wantMsg{names: []string{"a"}})

	before := s.runs
	s.alwaysAsk = false
	f.Update(wantMsg{names: []string{"a"}})

	if s.runs != before+1 {
		t.Errorf("Reconcile ran %d more times, want 1", s.runs-before)
	}
}

func TestAskingToReconcileWithoutAReconcileMethodIsReported(t *testing.T) {
	root := &unreconcilable{}
	f, diags := newWatched(t, root, 3, 1)

	f.Update(bumpMsg{})

	if !hasKind(*diags, froth.DiagNoReconciler) {
		t.Fatalf("asking to reconcile with no Reconcile method was not reported; got %v", *diags)
	}
}

// unreconcilable asks to be reconciled and cannot be: the method is the opt-in, and the
// request names one that is not there.
type unreconcilable struct{ pane }

func (u *unreconcilable) Update(uc *froth.UpdateCtx, msg tea.Msg) { uc.Self().Reconcile() }

// A reconcile is where a surface first meets an already-mounted child of its
// own, so the destroy that runs there is immediate rather than recorded.
func TestAReconcileDestroysAnAlreadyMountedChildAtOnce(t *testing.T) {
	var log []string
	s := &tabs{want: []string{"a"}, log: &log}
	s.onReconcile = func(s *tabs, ac *froth.ApplyCtx) {
		ac.Child(s.live["a"]).Destroy()
		delete(s.live, "a")
		// The frame is built from what is left, so the unmount has already run
		// by the time this reconcile returns.
		if len(ac.Self().Children()) != 0 {
			t.Errorf("the child was still there after Destroy: %v", ac.Self().Children())
		}
	}
	f := newSized(t, s, 3, 1)

	f.Update(wantMsg{names: []string{"a"}})

	if got, want := content(t, f), "..."; got != want {
		t.Errorf("frame = %q, want %q", got, want)
	}
	if !slices.Contains(log, "unmount:a") {
		t.Errorf("the destroyed child never unmounted; log = %v", log)
	}
}

// Focus during Apply is immediate, so a reconcile can create a surface and put
// focus on it in one go.
func TestAReconcileCanFocusASurfaceItJustCreated(t *testing.T) {
	s := &tabs{}
	s.onReconcile = func(s *tabs, ac *froth.ApplyCtx) {
		h := ac.Self().Create(&tab{name: "n"})
		ac.Child(h).Focus()
	}
	f := newSized(t, s, 3, 1)

	f.Update(wantMsg{})

	if got, want := content(t, f), "N.."; got != want {
		t.Errorf("frame = %q, want %q; the new surface did not take focus", got, want)
	}
}

// The ownership rule is the context tiers, and Reconcile holds an ordinary
// ApplyCtx: reaching past your own children is a bug rather than a refusal.
func TestAReconcileCannotRestructureASurfaceItDoesNotOwn(t *testing.T) {
	r := &reacher{kid: &tab{name: "a", kid: &fixed{content: "g"}}}
	f := newSized(t, r, 3, 1)

	var got any
	func() {
		defer func() { got = recover() }()
		f.Update(bumpMsg{})
	}()

	if msg := panicText(got); !strings.Contains(msg, "immediate child") {
		t.Errorf("reaching past a child from Reconcile panicked with %q", msg)
	}
}

// reacher reconciles by reaching for its grandchild, which is not its to name.
type reacher struct{ kid *tab }

func (r *reacher) Render(*froth.RenderCtx, froth.Size) string { return "." }

func (r *reacher) Update(uc *froth.UpdateCtx, _ tea.Msg) { uc.Self().Reconcile() }

func (r *reacher) Mount(ac *froth.ApplyCtx) { ac.Self().Create(r.kid) }

func (r *reacher) Reconcile(ac *froth.ApplyCtx) {
	ac.Child(ac.At(r.kid.self).Children()[0])
}

func TestASurfaceThatPanicsInReconcileIsRemoved(t *testing.T) {
	s := &tabs{want: []string{"a"}, panicIn: "Reconcile", askOnMount: true}
	root := &parent{body: "...", kids: []froth.Surface{&fixed{content: "k"}, s}}
	f, diags := newWatched(t, root, 3, 1)

	if !hasKind(*diags, froth.DiagSurfacePanic) {
		t.Fatalf("a panic in Reconcile was not reported; got %v", *diags)
	}
	if got, want := content(t, f), "k.."; got != want {
		t.Errorf("frame = %q, want %q; the surface that panicked is still drawn", got, want)
	}
}
