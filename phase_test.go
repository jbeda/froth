package froth_test

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/jbeda/froth"
)

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

// A context is handed to a surface, never constructed. The zero value is the
// shape a first unit test reaches for, so it says what is wrong rather than
// dereferencing nil.
func TestAConstructedContextPanicsWithADiagnostic(t *testing.T) {
	var got any
	func() {
		defer func() { got = recover() }()
		(&froth.RenderCtx{}).Rect()
	}()

	msg := panicText(got)
	if !strings.Contains(msg, "not handed out by froth") {
		t.Errorf("panic was %v, want one naming the constructed context", got)
	}
}
