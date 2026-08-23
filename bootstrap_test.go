package froth_test

import (
	"context"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/jbeda/froth"
)

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

func TestViewIsEmptyBeforeTheFirstSize(t *testing.T) {
	f := froth.New(context.Background(), &pane{title: "hello"})

	if got := f.View().Content; got != "" {
		t.Errorf("View().Content before any size = %q, want empty", got)
	}
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

// Wide graphemes are the compositor's business: a string-assembled frame would
// count 日本 as four columns and clip in the wrong place.
func TestWideGraphemesOccupyTheirRealWidth(t *testing.T) {
	f := newSized(t, &fixed{content: "日本語"}, 4, 1)

	if got, want := f.View().Content, "日本"; got != want {
		t.Errorf("View().Content = %q, want %q", got, want)
	}
}
