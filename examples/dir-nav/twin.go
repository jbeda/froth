package main

import (
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/jbeda/froth"
)

// twinInset is how much narrower than the screen a twin draws. Halved by the
// centring, it puts both left edges 12 cells in — inside the sidebar, whatever
// the screen is — and both right edges past the detail pane, which is what
// leaves a twin ahead of the detail pane for a move rightwards out of a row.
const twinInset = 24

// twin is one of a pair of one-line floats across the middle of the board. They
// are what reaches the directional rule's last tie-break: two candidates with
// the same rect, which a tiling layout never produces, decided by which of them
// was drawn last and is therefore the one on screen. Both draw one width from the screen they
// are offered, so both centre on one rect.
//
// A float is offered the whole screen, so the size a twin reports to the banner
// is the screen's rather than the strip it drew.
type twin struct {
	name string
	bd   *board
}

func (t *twin) Render(rc *froth.RenderCtx, s froth.Size) string {
	if s.W <= twinInset {
		return ""
	}
	focused := rc.Focused()
	if focused {
		t.bd.saw(spot{region: regTwin, name: t.name, size: s})
	}

	fg, marker := lipgloss.Color("245"), "  "
	if focused {
		fg, marker = lipgloss.Color("214"), "▸ "
	}
	return lipgloss.NewStyle().
		Foreground(fg).
		Background(lipgloss.Color("236")).
		Bold(focused).
		Width(s.W - twinInset).
		Render(marker + t.name)
}

func (t *twin) Update(*froth.UpdateCtx, tea.Msg) {}

func (t *twin) Declare() froth.Flags { return froth.Flags{TabStop: true} }
