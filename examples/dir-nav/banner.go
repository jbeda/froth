package main

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/jbeda/froth"
)

// banner is the line along the top: where focus has been, which arrow keys are
// in force, and the rect the focused surface was actually given. It is not a tab
// stop and never takes focus.
//
// The rect earns its place. When a move surprises you the first question is
// where the rule thought the boxes were, and after a few size keys that is not
// where you think they are.
type banner struct{ bd *board }

func (b *banner) Render(_ *froth.RenderCtx, r froth.Rect) string {
	if r.Empty() {
		return ""
	}

	mode := "shift+arrow"
	if b.bd.bare {
		mode = "bare arrows"
	}

	where := "nowhere yet"
	if f := b.bd.focus; f.name != "" {
		where = fmt.Sprintf("%s at %d,%d %d×%d", f.name, f.rect.X, f.rect.Y, f.rect.W, f.rect.H)
	}

	if b.bd.diag != "" {
		return lipgloss.NewStyle().
			Foreground(lipgloss.Color("203")).
			Width(r.W).
			Render(" " + b.bd.diag)
	}

	line := fmt.Sprintf(" %s   ·   %s   ·   %s",
		strings.Join(b.bd.trail, " ▸ "), mode, where)
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color("214")).
		Width(r.W).
		Render(line)
}

func (b *banner) Update(*froth.UpdateCtx, tea.Msg) {}
