package main

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/jbeda/froth"
)

// filter is a text sink: while it holds focus, froth matches no bare printable
// key against anybody's bindings and delivers it here as a character instead.
// It collects what it is given and draws it. It is not a text input — there is
// no cursor, and nothing to move one with.
//
// It is also the smallest command palette there is. What it types is matched
// against froth's list of the actions the focus chain currently offers, and
// enter asks for the first match by identity, with no key pressed. Two things
// that list does and the help bar's does not are the whole reason a palette can
// be built on it: it keeps a keyless action, so froth.cancel is offerable at
// all, and it is unmoved by a text sink holding focus — which this surface is,
// so a list that stripped typing would empty itself the moment anyone used it.
type filter struct{ text string }

// match reports the first action whose help or identity contains what has been
// typed. Nothing here knows an action by name.
func match(actions []froth.ChainBinding, typed string) (froth.ChainBinding, bool) {
	typed = strings.ToLower(strings.TrimSpace(typed))
	if typed == "" {
		return froth.ChainBinding{}, false
	}
	for _, a := range actions {
		hay := strings.ToLower(a.Help + " " + string(a.Action))
		if strings.Contains(hay, typed) {
			return a, true
		}
	}
	return froth.ChainBinding{}, false
}

func (fl *filter) Render(rc *froth.RenderCtx, s froth.Size) string {
	focused := rc.Focused()

	border := lipgloss.Color("240")
	if focused {
		border = lipgloss.Color("212")
	}
	body := "filter: " + fl.text
	if a, ok := match(rc.Query().Actions(), fl.text); ok {
		body += "  ⏎ " + a.Help
	}
	if !focused && fl.text == "" {
		body = "tab here and type — q lands in the field, ctrl+c still quits"
	}
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(border).
		Padding(0, 1).
		Width(s.W).
		Height(s.H).
		Render(body)
}

// The clear binding is disabled while there is nothing to clear, so the help bar
// greys it out and pressing it does nothing.
//
// It is a chord because the field is a text sink. A bare letter would be typing
// while this field holds focus, so it would never reach a binding — and the help
// bar would rightly refuse to offer it.
func (fl *filter) bindings() []froth.Binding {
	return []froth.Binding{{
		Action:   actionClear,
		Keys:     []string{"ctrl+u"},
		Help:     "clear the filter",
		Disabled: fl.text == "",
	}}
}

func (fl *filter) Mount(ac *froth.ApplyCtx) { ac.Self().SetBindings(fl.bindings()) }

// This surface is the one that pays for push. Disabled is a function of the
// text, the text changes on almost every key, and nothing recomputes a
// declaration — so every path that can change it has to declare again. One call
// after the handler is that, rather than one per branch: the branches are what
// would drift.
func (fl *filter) Update(uc *froth.UpdateCtx, msg tea.Msg) {
	fl.handle(uc, msg)
	uc.Self().SetBindings(fl.bindings())
}

// Only a press is typing: a release is the same key arriving a second time, and
// answering both would put every character in twice.
func (fl *filter) handle(uc *froth.UpdateCtx, msg tea.Msg) {
	if m, ok := msg.(froth.ActionMsg); ok {
		if m.Action == actionClear {
			fl.text = ""
			uc.StopRising()
		}
		// Everything else keeps rising: this surface holds focus, so every
		// action starts here whoever declared it and whoever will act on it.
		return
	}
	k, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return
	}
	switch k.String() {
	case "enter":
		// Invoking is the same walk a key press gets, minus the resolving. The
		// action goes to whatever holds focus — this field — and rises from
		// there, so quit typed here quits exactly as ctrl+c does.
		if a, ok := match(uc.Query().Actions(), fl.text); ok {
			fl.text = ""
			uc.Invoke(a.Action)
		}
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
