package froth_test

import (
	"slices"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/jbeda/froth"
)

// modalNest is a speaker that also creates children, each with whatever create
// options the test asked for. That is the whole fixture modality needs: a modal
// is an ordinary child that says it is one.
type modalNest struct {
	speaker
	kids    []nestKid
	handles []froth.Handle
}

type nestKid struct {
	surface froth.Surface
	opts    []froth.CreateOption
}

func (n *modalNest) Mount(ac *froth.ApplyCtx) {
	n.speaker.Mount(ac)
	for _, k := range n.kids {
		n.handles = append(n.handles, ac.Self().Create(k.surface, k.opts...))
	}
	ac.Self().SetLayout(under{})
}

func (n *modalNest) Render(rc *froth.RenderCtx, s froth.Size) string {
	rows := make([]string, max(1, s.H))
	rows[0] = n.speaker.Render(rc, s) + strings.Repeat(".", max(0, s.W-1))
	for i := 1; i < len(rows); i++ {
		rows[i] = strings.Repeat(".", s.W)
	}
	return strings.Join(rows, "\n")
}

func (n *modalNest) Update(uc *froth.UpdateCtx, msg tea.Msg) {
	if m, ok := msg.(focusMsg); ok {
		uc.At(m.to).Focus()
		return
	}
	n.speaker.Update(uc, msg)
}

// modal is the option list a test spells out often enough to name.
var modal = []froth.CreateOption{froth.Modal()}

// modalTree settles the mounts, so the handles a test focuses are real.
func modalTree(t *testing.T, root *modalNest, w, h int) *froth.Froth {
	t.Helper()
	f := newSized(t, root, w, h)
	f.View()
	return f
}

// The ancestor is outside the modal, so its declaration does not resolve — and
// the key that matched nothing reaches the focused surface raw, because that
// surface is inside the modal rather than blocked by it.
func TestAModalTakesAnAncestorsBindingOutOfPlay(t *testing.T) {
	dlg := &speaker{letter: "d"}
	root := &modalNest{
		speaker: speaker{letter: ".", bindings: binds("q", "Q")},
		kids:    []nestKid{{surface: dlg, opts: modal}},
	}
	f := modalTree(t, root, 2, 2)
	f.Update(focusMsg{to: root.handles[0]})

	f.Update(keyPress('q'))

	if root.last != "" {
		t.Errorf("the root saw %q, want nothing: it is outside the modal, so "+
			"its declaration does not resolve", root.last)
	}
	if got, want := dlg.last, "q"; got != want {
		t.Errorf("the modal saw %q raw, want %q: the key matched no binding "+
			"that survives, and the focused surface is inside the modal", got, want)
	}
}

func TestAModalsOwnBindingStillResolves(t *testing.T) {
	dlg := &speaker{letter: "d", bindings: binds("m", "M")}
	root := &modalNest{
		speaker: speaker{letter: ".", bindings: binds("q", "Q")},
		kids:    []nestKid{{surface: dlg, opts: modal}},
	}
	f := modalTree(t, root, 2, 2)
	f.Update(focusMsg{to: root.handles[0]})

	f.Update(keyPress('m'))

	if got, want := dlg.last, "M"; got != want {
		t.Errorf("the modal saw %q, want %q", got, want)
	}
}

// A surviving declaration resolves from outside and then reaches the surface
// that declared it, because delivery does not truncate: the rise from inside the
// modal passes the modal's parent on its way to the root.
func TestASurvivingBindingResolvesFromOutsideAndReachesItsDeclarer(t *testing.T) {
	dlg := &speaker{letter: "d"}
	root := &modalNest{
		speaker: speaker{letter: ".", bindings: []froth.Binding{
			{Action: "Q", Keys: []string{"q"}, SurvivesModal: true},
		}},
		kids: []nestKid{{surface: dlg, opts: modal}},
	}
	f := modalTree(t, root, 2, 2)
	f.Update(focusMsg{to: root.handles[0]})

	f.Update(keyPress('q'))

	if got, want := root.last, "Q"; got != want {
		t.Errorf("the root saw %q, want %q: a surviving binding resolves from "+
			"outside the modal and the action rises to whoever declared it", got, want)
	}
}

// froth's one input drop. The surface is in the subtree the modal blocks and is
// not inside the modal, so a key matching nothing reaches nothing at all.
func TestARawKeyAimedAtABlockedSurfaceIsDropped(t *testing.T) {
	plain := &speaker{letter: "p"}
	root := &modalNest{
		speaker: speaker{letter: "."},
		kids: []nestKid{
			{surface: plain},
			{surface: &speaker{letter: "d"}, opts: modal},
		},
	}
	f := modalTree(t, root, 3, 2)
	f.Update(focusMsg{to: root.handles[0]})

	f.Update(keyPress('x'))

	if plain.last != "" {
		t.Errorf("the blocked surface saw %q raw, want nothing", plain.last)
	}
}

// The same tree with the modal option removed, so the drop above is modality
// doing it rather than the key never having arrived.
func TestARawKeyReachesTheSameSurfaceWhenNothingIsModal(t *testing.T) {
	plain := &speaker{letter: "p"}
	root := &modalNest{
		speaker: speaker{letter: "."},
		kids: []nestKid{
			{surface: plain},
			{surface: &speaker{letter: "d"}},
		},
	}
	f := modalTree(t, root, 3, 2)
	f.Update(focusMsg{to: root.handles[0]})

	f.Update(keyPress('x'))

	if got, want := plain.last, "x"; got != want {
		t.Errorf("the focused surface saw %q raw, want %q", got, want)
	}
}

// froth services traversal, so its own bindings are not the modal's to take
// away — otherwise a dialog with two buttons could not be tabbed between.
func TestFrothsStockTraversalStillWorksInsideAModal(t *testing.T) {
	a, b := &focusable{letter: "a", flags: stop}, &focusable{letter: "b", flags: stop}
	dlg := &modalNest{
		speaker: speaker{letter: "d"},
		kids:    []nestKid{{surface: a}, {surface: b}},
	}
	root := &modalNest{
		speaker: speaker{letter: "."},
		kids:    []nestKid{{surface: dlg, opts: modal}},
	}
	f := modalTree(t, root, 4, 3)
	f.Update(focusMsg{to: dlg.handles[0]})

	f.Update(tabPress())

	if got, want := f.Query().Focus(), dlg.handles[1]; got != want {
		t.Errorf("focus is %v, want %v: tab is froth's own and survives a modal", got, want)
	}
}

// A rebound froth action keeps the key it was rebound to. The modal changes
// which keys resolve, never which key an action answers to.
func TestARebindingOfAFrothActionSurvivesAModalOnItsOwnKey(t *testing.T) {
	a, b := &focusable{letter: "a", flags: stop}, &focusable{letter: "b", flags: stop}
	dlg := &modalNest{
		speaker: speaker{letter: "d"},
		kids:    []nestKid{{surface: a}, {surface: b}},
	}
	root := &modalNest{
		speaker: speaker{letter: ".", bindings: []froth.Binding{
			{Action: froth.ActionFocusNext, Keys: []string{"n"}},
		}},
		kids: []nestKid{{surface: dlg, opts: modal}},
	}
	f := modalTree(t, root, 4, 3)
	f.Update(focusMsg{to: dlg.handles[0]})

	f.Update(keyPress('n'))
	if got, want := f.Query().Focus(), dlg.handles[1]; got != want {
		t.Errorf("focus is %v, want %v: the rebound key still moves focus "+
			"inside a modal", got, want)
	}

	f.Update(tabPress())
	if got, want := f.Query().Focus(), dlg.handles[1]; got != want {
		t.Errorf("focus is %v, want %v: the application took tab away from "+
			"focus.next, and a modal does not hand it back", got, want)
	}
}

// A suppressed declaration is skipped whole rather than narrowed to no keys. If
// it still claimed its key it would shadow froth's own binding behind it, and
// tab would die inside every modal for want of a declaration that cannot fire.
func TestASuppressedDeclarationDoesNotShadowFrothsOwnBinding(t *testing.T) {
	a, b := &focusable{letter: "a", flags: stop}, &focusable{letter: "b", flags: stop}
	dlg := &modalNest{
		speaker: speaker{letter: "d"},
		kids:    []nestKid{{surface: a}, {surface: b}},
	}
	root := &modalNest{
		speaker: speaker{letter: ".", bindings: binds("tab", "Q")},
		kids:    []nestKid{{surface: dlg, opts: modal}},
	}
	f := modalTree(t, root, 4, 3)
	f.Update(focusMsg{to: dlg.handles[0]})

	f.Update(tabPress())

	if got, want := f.Query().Focus(), dlg.handles[1]; got != want {
		t.Errorf("focus is %v, want %v: the root's claim on tab is suppressed, "+
			"so froth's own binding is what tab resolves against", got, want)
	}
	// The root still sees the froth action rise past it — delivery does not
	// truncate — but never its own, which is what suppression took away.
	if got := root.last; got == "Q" {
		t.Errorf("the root saw %q, want its own declaration suppressed", got)
	}
}

// One walk, so what a query publishes cannot describe a different world from
// what a key press does.
func TestASuppressedDeclarationIsAbsentFromBothQueryViews(t *testing.T) {
	dlg := &speaker{letter: "d", bindings: binds("m", "M")}
	root := &modalNest{
		speaker: speaker{letter: ".", bindings: binds("q", "Q")},
		kids:    []nestKid{{surface: dlg, opts: modal}},
	}
	f := modalTree(t, root, 2, 2)
	f.Update(focusMsg{to: root.handles[0]})

	for _, c := range []struct {
		name string
		got  []froth.ChainBinding
	}{
		{"LiveBindings", f.Query().LiveBindings()},
		{"Actions", f.Query().Actions()},
	} {
		actions := make([]froth.Action, 0, len(c.got))
		for _, b := range c.got {
			actions = append(actions, b.Action)
		}
		if !slices.Contains(actions, froth.Action("M")) {
			t.Errorf("%s = %v, want the modal's own action in it", c.name, actions)
		}
		if slices.Contains(actions, froth.Action("Q")) {
			t.Errorf("%s = %v, want the suppressed declaration absent rather "+
				"than listed with no keys", c.name, actions)
		}
	}
}

// Two modals over one surface is the ordinary case rather than an error: a
// modal opening a modal parents both to the root. The one on top governs.
func TestTheModalOnTopGovernsWhenTwoAreOpen(t *testing.T) {
	first := &speaker{letter: "1", bindings: binds("a", "A")}
	second := &speaker{letter: "2", bindings: binds("b", "B")}
	root := &modalNest{
		speaker: speaker{letter: "."},
		kids: []nestKid{
			{surface: first, opts: modal},
			{surface: second, opts: modal},
		},
	}
	f := modalTree(t, root, 3, 2)

	f.Update(focusMsg{to: root.handles[1]})
	f.Update(keyPress('b'))
	if got, want := second.last, "B"; got != want {
		t.Errorf("the modal on top saw %q, want %q", got, want)
	}

	f.Update(focusMsg{to: root.handles[0]})
	f.Update(keyPress('a'))
	if first.last != "" {
		t.Errorf("the modal underneath saw %q, want nothing: the one on top "+
			"governs, and this one is outside it", first.last)
	}
}

// A modal parented inside another suppresses the outer one the same way it
// suppresses anything else outside it.
func TestAModalInsideAModalSuppressesTheOuterOne(t *testing.T) {
	inner := &speaker{letter: "i"}
	outer := &modalNest{
		speaker: speaker{letter: "o", bindings: binds("q", "Q")},
		kids:    []nestKid{{surface: inner, opts: modal}},
	}
	root := &modalNest{
		speaker: speaker{letter: "."},
		kids:    []nestKid{{surface: outer, opts: modal}},
	}
	f := modalTree(t, root, 4, 3)
	f.Update(focusMsg{to: outer.handles[0]})

	f.Update(keyPress('q'))

	if outer.last != "" {
		t.Errorf("the outer modal saw %q, want nothing", outer.last)
	}
}

// The blocked subtree is the modal's parent's, so a modal parented lower leaves
// everything outside that subtree alone.
func TestAModalParentedLowerLeavesASiblingSubtreeAlone(t *testing.T) {
	branch := &modalNest{
		speaker: speaker{letter: "a"},
		kids:    []nestKid{{surface: &speaker{letter: "d"}, opts: modal}},
	}
	other := &speaker{letter: "b"}
	root := &modalNest{
		speaker: speaker{letter: ".", bindings: binds("q", "Q")},
		kids:    []nestKid{{surface: branch}, {surface: other}},
	}
	f := modalTree(t, root, 4, 3)
	f.Update(focusMsg{to: root.handles[1]})

	f.Update(keyPress('q'))

	if got, want := root.last, "Q"; got != want {
		t.Errorf("the root saw %q, want %q: the modal blocks its own parent's "+
			"subtree, which this surface is not in", got, want)
	}
}

// froth publishes ActionCancel's identity and services nothing for it, so a
// surface declaring that one is an application talking to itself under a froth
// name. The exemption is for what froth implements, not for what it publishes.
func TestDeclaringAFrothIdentityFrothDoesNotServiceIsSuppressed(t *testing.T) {
	dlg := &speaker{letter: "d"}
	root := &modalNest{
		speaker: speaker{letter: ".", bindings: []froth.Binding{
			{Action: froth.ActionCancel, Keys: []string{"c"}},
		}},
		kids: []nestKid{{surface: dlg, opts: modal}},
	}
	f := modalTree(t, root, 2, 2)
	f.Update(focusMsg{to: root.handles[0]})

	f.Update(keyPress('c'))

	if root.last != "" {
		t.Errorf("the root saw %q, want nothing: froth services no cancel, so "+
			"a declaration of it is suppressed like any other", root.last)
	}
	if got, want := dlg.last, "c"; got != want {
		t.Errorf("the modal saw %q raw, want %q", got, want)
	}
}

// brokenModal is quarantined the first time it draws.
type brokenModal struct{ speaker }

func (b *brokenModal) Render(*froth.RenderCtx, froth.Size) string { panic("render broke") }

// A quarantined surface draws nothing and declares nothing. It blocks nothing
// either, or an invisible modal would hold its whole subtree until the destroy
// it is already queued for commits — with nothing the application could act on.
func TestAQuarantinedModalStopsGoverning(t *testing.T) {
	plain := &speaker{letter: "p"}
	root := &modalNest{
		speaker: speaker{letter: ".", bindings: binds("q", "Q")},
		kids: []nestKid{
			{surface: plain},
			{surface: &brokenModal{speaker: speaker{letter: "d"}}, opts: modal},
		},
	}
	var got []froth.Diagnostic
	f := froth.New(t.Context(), root,
		froth.OnDiagnostic(func(d froth.Diagnostic) { got = append(got, d) }))
	f.Update(tea.WindowSizeMsg{Width: 3, Height: 2})
	f.Update(focusMsg{to: root.handles[0]})

	// Drawing quarantines it, and the query below asks before the Apply that
	// takes it out of the tree — which is the whole window this is about.
	f.View()
	if len(got) == 0 {
		t.Fatal("the modal did not panic while drawing, so nothing was quarantined")
	}

	actions := make([]froth.Action, 0)
	for _, b := range f.Query().LiveBindings() {
		actions = append(actions, b.Action)
	}
	if !slices.Contains(actions, froth.Action("Q")) {
		t.Errorf("LiveBindings = %v, want the root's binding live: a quarantined "+
			"modal declares nothing and blocks nothing", actions)
	}
}
