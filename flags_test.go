package froth_test

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/jbeda/froth"
)

// flagged declares the flags it was built with and draws what froth says its
// flags are now, so a frame reports the set froth stored rather than the field
// the surface was constructed from.
type flagged struct {
	flags   froth.Flags
	atMount *froth.Flags
	saw     string
}

// setFlagsMsg tells a flagged surface to replace its whole set; readFlagsMsg tells it
// to say what froth has without changing anything.
type setFlagsMsg struct{ to froth.Flags }

type readFlagsMsg struct{}

func (d *flagged) Declare() froth.Flags { return d.flags }

func (d *flagged) Render(rc *froth.RenderCtx, s froth.Size) string { return d.saw }

func (d *flagged) Mount(ac *froth.ApplyCtx) {
	if d.atMount != nil {
		ac.Self().SetFlags(*d.atMount)
	}
	d.saw = flagLetters(ac.Self().GetFlags())
}

func (d *flagged) Update(uc *froth.UpdateCtx, msg tea.Msg) {
	if m, ok := msg.(setFlagsMsg); ok {
		uc.Self().SetFlags(m.to)
	}
	d.saw = flagLetters(uc.Self().GetFlags())
}

// flagLetters spells a flag set one column per flag, so a frame says which are
// on rather than how many.
func flagLetters(fl froth.Flags) string {
	out := []byte("---")
	if fl.TabStop {
		out[0] = 'T'
	}
	if fl.ClickFocus {
		out[1] = 'C'
	}
	if fl.TextSink {
		out[2] = 'S'
	}
	return string(out)
}

// flagRoot stands one child in its whole rect and hands it whatever it is told,
// which is how a test reaches a surface that is not the root.
type flagRoot struct {
	kid   froth.Surface
	child froth.Handle
}

func (r *flagRoot) Render(*froth.RenderCtx, froth.Size) string { return "" }
func (r *flagRoot) Update(uc *froth.UpdateCtx, msg tea.Msg)    { uc.At(r.child).Post(msg) }
func (r *flagRoot) Mount(ac *froth.ApplyCtx)                   { r.child = ac.Self().Create(r.kid) }

// A surface restates the whole set, so what it leaves out is off rather than
// kept — the same shape SetBindings has, and safe here because nothing else
// writes a surface's own flags.
func TestSetFlagsReplacesTheWholeSetAndGetFlagsReadsItBack(t *testing.T) {
	kid := &flagged{flags: froth.Flags{TabStop: true, TextSink: true}}
	f := newSized(t, &flagRoot{kid: kid}, 3, 1)

	f.Update(readFlagsMsg{})
	if got, want := f.View().Content, "T-S"; got != want {
		t.Errorf("View().Content = %q, want %q — the type's declaration is what froth stored",
			got, want)
	}

	f.Update(setFlagsMsg{to: froth.Flags{ClickFocus: true}})

	if got, want := f.View().Content, "-C-"; got != want {
		t.Errorf("View().Content = %q, want %q — the flags left out of the new set are off",
			got, want)
	}
}

// Apply has the pair too, so a surface assembling itself can settle what it is
// alongside the children it creates.
func TestASurfaceCanReplaceAndReadItsFlagsWhileItMounts(t *testing.T) {
	kid := &flagged{
		flags:   froth.Flags{TabStop: true},
		atMount: &froth.Flags{TextSink: true},
	}
	f := newSized(t, &flagRoot{kid: kid}, 3, 1)

	if got, want := f.View().Content, "--S"; got != want {
		t.Errorf("View().Content = %q, want %q", got, want)
	}
}

// eagerScope creates its children and moves focus onto itself in one Mount,
// which is before any of those children has been mounted. It is a focus scope
// admitting nothing, so the move redirects onto whichever child the walk
// admits — and finding one at all means the child's declaration was already in
// force.
type eagerScope struct{}

func (s *eagerScope) Render(*froth.RenderCtx, froth.Size) string { return "" }
func (s *eagerScope) Update(*froth.UpdateCtx, tea.Msg)           {}

func (s *eagerScope) Mount(ac *froth.ApplyCtx) {
	ac.Self().Create(&focusable{letter: "a"})
	ac.Self().Create(&focusable{letter: "b", flags: stop})
	ac.Self().SetLayout(cells{})
	ac.Self().Focus()
}

func TestATypesDeclarationIsInForceBeforeTheChildIsMounted(t *testing.T) {
	root := &optRoot{
		kids: []froth.Surface{&eagerScope{}},
		opts: [][]froth.CreateOption{scope},
		lay:  columns{w: []int{2}},
	}
	f := newSized(t, root, 2, 1)

	if got, want := f.View().Content, "aB"; got != want {
		t.Errorf("View().Content = %q, want %q", got, want)
	}
}

// The two admissions are independent, and this is the direction that is easy to
// get wrong: a surface a click may focus is not thereby somewhere tab lands.
func TestASurfaceThatAdmitsOnlyClicksIsNotReachedByTraversal(t *testing.T) {
	f, _ := tabRow(t, "lm", clickOnly, stop)

	f.Update(tabMsg{})

	if got, want := f.View().Content, "lM"; got != want {
		t.Errorf("View().Content = %q, want %q", got, want)
	}
}

// clickScope builds "a [b c]" with b admitting clicks alone and c a tab stop,
// which is the tree the two entry predicates disagree about.
func clickScope(t *testing.T) (*froth.Froth, *optRoot, *optRoot) {
	t.Helper()
	scoped := &optRoot{kids: []froth.Surface{
		&focusable{letter: "b", flags: clickOnly},
		&focusable{letter: "c", flags: stop},
	}}
	root := &optRoot{
		kids: []froth.Surface{&focusable{letter: "a", flags: stop}, scoped},
		opts: [][]froth.CreateOption{nil, scope},
		lay:  columns{w: []int{1, 2}},
	}
	f := newSized(t, root, 3, 1)
	f.View()
	return f, root, scoped
}

// Restoration puts focus back where it was, and a click is a legitimate way to
// have got there — so what a scope remembers is checked against admitting focus
// at all rather than against the walk.
func TestAScopeRestoresToARememberedSurfaceThatOnlyAdmitsClicks(t *testing.T) {
	f, root, scoped := clickScope(t)

	f.Update(focusMsg{to: scoped.handles[0]})
	if got, want := f.View().Content, "aBc"; got != want {
		t.Fatalf("View().Content = %q, want %q", got, want)
	}
	f.Update(focusMsg{to: root.handles[0]})

	f.Update(focusMsg{to: root.handles[1]})

	if got, want := f.View().Content, "aBc"; got != want {
		t.Errorf("re-entering the scope: View().Content = %q, want %q", got, want)
	}
}

// Nothing is remembered, so froth is choosing a start rather than restoring
// one, and a keyboard entry should not land on a surface the keyboard cannot
// leave the same way.
func TestEnteringAScopeFreshSkipsASurfaceThatOnlyAdmitsClicks(t *testing.T) {
	f, root, _ := clickScope(t)

	f.Update(focusMsg{to: root.handles[1]})

	if got, want := f.View().Content, "abC"; got != want {
		t.Errorf("View().Content = %q, want %q", got, want)
	}
}

// A scope admitting focus by no route is entered wherever the move came from,
// so focus never lands on a surface it could be tabbed away from and never
// returned to.
func TestFocusMovedToAScopeThatAdmitsNothingIsRedirectedFromInsideToo(t *testing.T) {
	f, root, _ := scopedRow(t, scope, froth.Flags{})
	f.Update(focusMsg{to: root.handles[1]})
	f.Update(tabMsg{})
	if got, want := f.View().Content, "abCd"; got != want {
		t.Fatalf("View().Content = %q, want %q", got, want)
	}

	f.Update(focusMsg{to: root.handles[1]})

	if got, want := f.View().Content, "abCd"; got != want {
		t.Errorf("View().Content = %q, want %q — the move redirected onto what "+
			"the scope remembers rather than landing on the scope", got, want)
	}
}

// Admitting by any route is enough for the move to be literal, so a scope that
// takes clicks and not the walk can be focused as itself — which is what makes
// "focus the container" expressible at all.
func TestFocusMovedToAScopeThatAdmitsClicksLandsOnIt(t *testing.T) {
	f, root, scoped := scopedRow(t, scope, clickOnly)

	f.Update(focusMsg{to: root.handles[1]})
	if got, want := f.View().Content, "abcd"; got != want {
		t.Fatalf("from outside: View().Content = %q, want %q — no letter is "+
			"upper case because the scope surface itself holds focus", got, want)
	}

	// From inside the same move is the same move: tab off the scope onto one of
	// its stops, then ask for the scope again.
	f.Update(tabMsg{})
	if got, want := f.View().Content, "aBcd"; got != want {
		t.Fatalf("after tab inside the scope: View().Content = %q, want %q", got, want)
	}
	f.Update(focusMsg{to: scoped.handles[0]})
	f.Update(focusMsg{to: root.handles[1]})

	if got, want := f.View().Content, "abcd"; got != want {
		t.Errorf("from inside: View().Content = %q, want %q", got, want)
	}
}
