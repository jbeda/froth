package main

import (
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/jbeda/froth"
)

// filter is a text sink: while it holds focus, froth matches no bare printable
// key against anybody's bindings and delivers it here as a character instead.
// It collects what it is given and draws it. It is not a text input — there is
// no cursor, and nothing to move one with.
type filter struct{ text string }

func (fl *filter) Render(rc *froth.RenderCtx, r froth.Rect) string {
	focused := rc.Focused()

	border := lipgloss.Color("240")
	if focused {
		border = lipgloss.Color("212")
	}
	body := "filter: " + fl.text
	if !focused && fl.text == "" {
		body = "tab here and type — q lands in the field, ctrl+c still quits"
	}
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(border).
		Padding(0, 1).
		Width(r.W).
		Height(r.H).
		Render(body)
}

// Only a press is typing: a release is the same key arriving a second time, and
// answering both would put every character in twice.
func (fl *filter) Update(uc *froth.UpdateCtx, msg tea.Msg) {
	k, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return
	}
	switch k.String() {
	case "backspace":
		if r := []rune(fl.text); len(r) > 0 {
			fl.text = string(r[:len(r)-1])
		}
	default:
		// Whatever the terminal says the key produced, which is empty for the
		// keys that print nothing.
		fl.text += k.Text
	}
}
