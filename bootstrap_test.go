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

func TestRootRendersAtTheFullTerminalSize(t *testing.T) {
	p := &pane{title: "x"}
	f := newSized(t, p, 40, 12)
	f.View()

	if got, want := p.gotSize, (froth.Size{W: 40, H: 12}); got != want {
		t.Errorf("root size = %+v, want %+v", got, want)
	}
}

func TestResizeChangesTheSizeWithNoMessageToTheSurface(t *testing.T) {
	p := &pane{title: "x"}
	f := newSized(t, p, 40, 12)
	f.View()

	f.Update(tea.WindowSizeMsg{Width: 10, Height: 4})
	f.View()

	if got, want := p.gotSize, (froth.Size{W: 10, H: 4}); got != want {
		t.Errorf("root size after resize = %+v, want %+v", got, want)
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

// firstRoot moves focus and declares a binding during Mount, which is the only
// chance it gets before the instance's first message.
type firstRoot struct {
	speaker
	kid   froth.Surface
	child froth.Handle
}

func (r *firstRoot) Mount(ac *froth.ApplyCtx) {
	r.speaker.Mount(ac)
	r.child = ac.Self().Create(r.kid)
	ac.Self().SetLayout(under{})
	ac.At(r.child).Focus()
}

// A message is addressed against the tree, so the tree has to exist first. On a
// fresh instance the root is registered and not yet mounted, so it has declared
// nothing and has not had its chance to move focus — and a key press arriving
// before anything else would otherwise resolve against an empty chain and land
// on the root rather than on what Mount focused.
func TestTheFirstMessageIsAddressedAgainstAMountedTree(t *testing.T) {
	t.Run("a press resolves", func(t *testing.T) {
		kid := &speaker{letter: "l"}
		root := &firstRoot{speaker: speaker{letter: ".", bindings: binds("q", "Q")}, kid: kid}
		f := froth.New(context.Background(), root)

		f.Update(keyPress('q'))
		f.Update(tea.WindowSizeMsg{Width: 1, Height: 2})

		if got, want := f.View().Content, "Q\nQ"; got != want {
			t.Errorf("View().Content = %q, want %q: the very first key press "+
				"resolved against a tree that had not been built", got, want)
		}
	})

	t.Run("a raw key reaches what Mount focused", func(t *testing.T) {
		kid := &speaker{letter: "l"}
		root := &firstRoot{speaker: speaker{letter: "."}, kid: kid}
		f := froth.New(context.Background(), root)

		f.Update(keyPress('z'))
		f.Update(tea.WindowSizeMsg{Width: 1, Height: 2})

		if got, want := f.View().Content, ".\nz"; got != want {
			t.Errorf("View().Content = %q, want %q: the key went to the root "+
				"rather than to the surface Mount focused", got, want)
		}
	})
}
