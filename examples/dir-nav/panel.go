package main

import (
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/jbeda/froth"
)

// panel is a bordered box on the board. Every focusable part of the shell is
// one; what differs between them is a name, a region, and where the layout puts
// it. There is no content beyond that on purpose — the thing being judged is
// where the highlight goes, and anything else on screen competes with it.
type panel struct {
	name   string
	region region
	index  int
	body   string
	bd     *board
}

func (p *panel) Render(rc *froth.RenderCtx, r froth.Rect) string {
	if r.Empty() {
		return ""
	}
	focused := rc.Focused()
	if focused {
		p.bd.saw(spot{region: p.region, index: p.index, name: p.name, rect: r})
	}

	// The marker's column is reserved whether or not it is drawn: focus changes
	// how a surface looks and never how much room it has, so a title that grew
	// when focused would wrap inside a box that had not moved.
	border, title := lipgloss.Color("240"), "  "+p.name
	if focused {
		border, title = lipgloss.Color("212"), "▸ "+p.name
	}

	content := lipgloss.NewStyle().Bold(focused).Render(title)
	if p.body != "" {
		content += "\n" + p.body
	}
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(border).
		Padding(0, 1).
		Width(r.W).
		Height(r.H).
		Render(content)
}

func (p *panel) Update(*froth.UpdateCtx, tea.Msg) {}

// control is a one-line surface inside the scope, where there is no room for a
// border.
type control struct {
	name string
	bd   *board
}

func (c *control) Render(rc *froth.RenderCtx, r froth.Rect) string {
	if r.Empty() {
		return ""
	}
	if rc.Focused() {
		c.bd.saw(spot{region: regScope, name: c.name, rect: r})
		return lipgloss.NewStyle().
			Foreground(lipgloss.Color("214")).
			Bold(true).
			Render("▸ " + c.name)
	}
	return lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render("  " + c.name)
}

func (c *control) Update(*froth.UpdateCtx, tea.Msg) {}
