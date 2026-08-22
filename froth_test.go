package froth_test

import (
	"context"
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
