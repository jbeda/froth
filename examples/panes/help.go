package main

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/jbeda/froth"
)

// helpBar draws the shortcuts that are live right now. It knows no keys and no
// actions: it asks froth what a key press would currently do and prints the
// answer, so it says whatever the surfaces on the focus chain are saying and
// changes as focus moves.
//
// Greying a disabled binding rather than hiding it is this application's
// choice. froth reports the binding either way and says nothing about colour.
type helpBar struct{}

func (helpBar) Render(rc *froth.RenderCtx, s froth.Size) string {
	key := lipgloss.NewStyle().Foreground(lipgloss.Color("212"))
	desc := lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	off := lipgloss.NewStyle().Foreground(lipgloss.Color("238"))

	sep := desc.Render("  ·  ")
	bar, width := "", 0
	for _, b := range rc.Query().LiveBindings() {
		keys := b.KeyHelp
		if keys == "" {
			keys = strings.Join(b.Keys, "/")
		}
		part := key.Render(keys) + " " + desc.Render(b.Help)
		if b.Disabled {
			part = off.Render(keys + " " + b.Help)
		}
		// Whatever does not fit is left off rather than cut mid-word. The bar
		// is in resolution order, so what goes first is what is nearest focus.
		w := lipgloss.Width(keys) + 1 + lipgloss.Width(b.Help)
		if bar != "" {
			w += lipgloss.Width(sep)
		}
		if width+w > s.W {
			break
		}
		if bar != "" {
			bar += sep
		}
		bar, width = bar+part, width+w
	}

	return lipgloss.NewStyle().Width(s.W).MaxHeight(s.H).Render(bar)
}

func (helpBar) Update(*froth.UpdateCtx, tea.Msg) {}
