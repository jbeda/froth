package froth_test

import (
	"context"
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/jbeda/froth"
)

// pane is the minimal surface: two methods, state on a pointer receiver.
type pane struct {
	title   string
	count   int
	gotSize froth.Size
}

func (p *pane) Render(rc *froth.RenderCtx, s froth.Size) string {
	p.gotSize = s
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

// newWatched is newSized for the tests that need to see what froth reported.
func newWatched(t *testing.T, root froth.Surface, w, h int) (*froth.Froth, *[]froth.Diagnostic) {
	t.Helper()
	var got []froth.Diagnostic
	f := froth.New(context.Background(), root,
		froth.OnDiagnostic(func(d froth.Diagnostic) { got = append(got, d) }))
	f.Update(tea.WindowSizeMsg{Width: w, Height: h})
	return f, &got
}

// countKind counts the diagnostics of one kind, and hasKind says whether there
// were any.
func countKind(got []froth.Diagnostic, k froth.DiagnosticKind) int {
	n := 0
	for _, d := range got {
		if d.Kind == k {
			n++
		}
	}
	return n
}

func hasKind(got []froth.Diagnostic, k froth.DiagnosticKind) bool {
	return countKind(got, k) > 0
}

type fixed struct{ content string }

func (x *fixed) Render(*froth.RenderCtx, froth.Size) string { return x.content }
func (x *fixed) Update(*froth.UpdateCtx, tea.Msg)           {}

// The application model is copied by value on every Bubble Tea update; froth is
// held by pointer, so the copy shares one instance.
type appModel struct{ f *froth.Froth }

func (m appModel) Init() tea.Cmd  { return m.f.Init() }
func (m appModel) View() tea.View { return m.f.View() }
func (m appModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	return m, m.f.Update(msg)
}

// panicText renders a recovered panic value as the text it would print. froth
// raises its own panics as an unexported error type, so that quarantine can
// tell a contract violation from a surface that failed and re-raise it; a test
// asserting on the message asks for the message rather than for the type.
func panicText(r any) string {
	if r == nil {
		return ""
	}
	return fmt.Sprint(r)
}
