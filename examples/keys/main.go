// Command keys prints what Bubble Tea reports for every key you press, which is
// exactly the string froth matches a Binding's Keys against.
//
// It exists because what a keyboard sends and what a program receives are not
// the same thing, and the gap is where a binding that looks right stops working.
// A terminal may rewrite a combination before any program sees it — Ghostty and
// Terminal.app both send "esc b" for alt+left on macOS, so a program receives
// alt+b — and the window manager may take one above the terminal, as macOS does
// with ctrl+left. Neither is visible from inside the program, and no amount of
// reasoning about which keys "should" work substitutes for pressing them.
//
// Run it wherever the answer matters — over ssh, inside tmux, in the terminal
// your users actually have — and press the combination you are thinking of
// binding. What it prints is what to write in Keys.
package main

import (
	"fmt"
	"log"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// suggestions are the combinations worth trying first: the ones froth binds, and
// the ones that are commonly taken before a program sees them.
var suggestions = []string{
	"shift+left / shift+right / shift+up / shift+down   what froth binds for directional focus",
	"tab / shift+tab                                    what froth binds for traversal",
	"alt+left / alt+right                               often rewritten to alt+b / alt+f",
	"ctrl+left / ctrl+right                             often taken by the window manager",
	"the bare arrows, and whatever else you are considering",
}

// history is how many presses stay on screen. Older ones scroll off rather than
// growing the view past the terminal.
const history = 16

type model struct{ seen []string }

func (m model) Init() tea.Cmd { return nil }

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	k, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return m, nil
	}
	// ctrl+c is the way out, so it is reported and then obeyed: a reader should
	// still see what it looks like.
	m.seen = append(m.seen, describe(k))
	if len(m.seen) > history {
		m.seen = m.seen[len(m.seen)-history:]
	}
	if k.String() == "ctrl+c" {
		return m, tea.Quit
	}
	return m, nil
}

// describe renders one press: the string a Binding matches, then the parts it
// was assembled from, which is what tells you whether a modifier arrived at all.
func describe(k tea.KeyPressMsg) string {
	text := k.Text
	if text == "" {
		text = "—"
	}
	return fmt.Sprintf("%-22s code=%-12q mod=%-3d text=%s",
		k.String(), k.Code, k.Mod, text)
}

func (m model) View() tea.View {
	dim := lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	bold := lipgloss.NewStyle().Bold(true)

	var b strings.Builder
	b.WriteString(bold.Render("Press a key. The first column is what to put in a Binding's Keys.") + "\n\n")
	for _, s := range suggestions {
		b.WriteString(dim.Render("  "+s) + "\n")
	}
	b.WriteString("\n")
	if len(m.seen) == 0 {
		b.WriteString(dim.Render("  nothing pressed yet") + "\n")
	}
	for _, l := range m.seen {
		b.WriteString("  " + l + "\n")
	}
	b.WriteString("\n" + dim.Render("  ctrl+c quits") + "\n")

	v := tea.NewView(b.String())
	return v
}

func main() {
	if _, err := tea.NewProgram(model{}).Run(); err != nil {
		log.Fatal(fmt.Errorf("running keys: %w", err))
	}
}
