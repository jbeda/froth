package main

import (
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/jbeda/froth"
)

// filter is the bar along the bottom, and a text sink: while it holds focus,
// bare printable keys arrive here as characters rather than being matched as
// shortcuts. That is where the arrow-key question bites hardest — swap to bare
// arrows and this field can never have a cursor, and m cannot swap back while
// the field holds focus, because m is a character now too.
type filter struct {
	text string
	bd   *board
}

func (f *filter) Render(rc *froth.RenderCtx, r froth.Rect) string {
	if r.Empty() {
		return ""
	}
	focused := rc.Focused()
	if focused {
		f.bd.saw(spot{region: regFilter, name: "Filter", rect: r})
	}

	border := lipgloss.Color("240")
	if focused {
		border = lipgloss.Color("212")
	}
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(border).
		Padding(0, 1).
		Width(r.W).
		Height(r.H).
		Render("filter: " + f.text)
}

// Only a press is typing: a release is the same key arriving a second time, and
// answering both would put every character in twice.
func (f *filter) Update(uc *froth.UpdateCtx, msg tea.Msg) {
	k, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return
	}
	if k.String() == "backspace" {
		if r := []rune(f.text); len(r) > 0 {
			f.text = string(r[:len(r)-1])
		}
		return
	}
	f.text += k.Text
}
