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

func (p *panel) Render(rc *froth.RenderCtx, s froth.Size) string {
	if s.Empty() {
		return ""
	}
	focused := rc.Focused()
	if focused {
		p.bd.saw(spot{region: p.region, index: p.index, name: p.name, size: s})
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
		Width(s.W).
		Height(s.H).
		Render(content)
}

func (p *panel) Update(*froth.UpdateCtx, tea.Msg) {}

// Declare is how a surface says what it is. A panel is somewhere the keyboard
// walk lands, and its creator does not get a say: every panel on this board is
// a stop, and nobody assembling one has to remember that.
func (p *panel) Declare() froth.Flags { return froth.Flags{TabStop: true} }

// control is a one-line surface inside the dialog, where there is no room for a
// border. enter picks one: it writes its name on the board for the banner, and
// lets the action rise to the dialog, which is what can take the dialog down.
type control struct {
	name string
	bd   *board
}

func (c *control) Mount(ac *froth.ApplyCtx) {
	ac.Self().SetBindings([]froth.Binding{{
		Action: actionPick,
		Keys:   []string{"enter"},
		Help:   "pick this control",
	}})
}

func (c *control) Render(rc *froth.RenderCtx, s froth.Size) string {
	if s.Empty() {
		return ""
	}
	if rc.Focused() {
		c.bd.saw(spot{region: regDialog, name: c.name, size: s})
		return lipgloss.NewStyle().
			Foreground(lipgloss.Color("214")).
			Background(lipgloss.Color("235")).
			Bold(true).
			Render("▸ " + c.name)
	}
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color("245")).
		Background(lipgloss.Color("235")).
		Render("  " + c.name)
}

func (c *control) Update(_ *froth.UpdateCtx, msg tea.Msg) {
	if m, ok := msg.(froth.ActionMsg); ok && m.Action == actionPick {
		c.bd.pick = "picked " + c.name
	}
}

func (c *control) Declare() froth.Flags { return froth.Flags{TabStop: true} }
