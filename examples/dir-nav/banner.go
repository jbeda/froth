package main

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/jbeda/froth"
)

// banner is the line along the top: where focus has been, which arrow keys are
// in force, and the size the focused surface was actually offered. It is not a
// tab stop and never takes focus.
//
// The size earns its place. When a move surprises you the first question is how
// big the rule thought the boxes were, and after a few size keys that is not
// what you think it is.
type banner struct{ bd *board }

func (b *banner) Render(_ *froth.RenderCtx, s froth.Size) string {
	if s.Empty() {
		return ""
	}

	mode := "shift+arrow"
	if b.bd.bare {
		mode = "bare arrows"
	}

	where := "nowhere yet"
	if f := b.bd.focus; f.name != "" {
		where = fmt.Sprintf("%s %d×%d", f.name, f.size.W, f.size.H)
	}

	if b.bd.diag != "" {
		return lipgloss.NewStyle().
			Foreground(lipgloss.Color("203")).
			Width(s.W).
			Render(" " + b.bd.diag)
	}

	line := fmt.Sprintf(" %s   ·   %s   ·   %s",
		strings.Join(b.bd.trail, " ▸ "), mode, where)
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color("214")).
		Width(s.W).
		Render(line)
}

func (b *banner) Update(*froth.UpdateCtx, tea.Msg) {}
