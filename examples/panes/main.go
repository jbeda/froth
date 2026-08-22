// Command panes is froth's running example. It grows with each milestone; today
// it is one surface, rendered through froth into the terminal.
package main

import (
	"context"
	"fmt"
	"log"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/jbeda/froth"
)

// pane is a surface: two methods, and its state on a pointer receiver.
type pane struct {
	title string
	keys  []string
}

func (p *pane) Render(rc *froth.RenderCtx, r froth.Rect) string {
	body := "Press keys to see them arrive. q or ctrl+c quits."
	if len(p.keys) > 0 {
		body += "\n\n" + strings.Join(p.keys, " ")
	}

	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		Padding(0, 1).
		// The border eats two cells in each direction, and a surface never
		// assumes the rect it was given is big enough for its chrome.
		Width(max(0, r.W-2)).
		Height(max(0, r.H-2))

	return box.Render(p.title + "\n\n" + body)
}

func (p *pane) Update(uc *froth.UpdateCtx, msg tea.Msg) {
	key, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return
	}
	switch key.String() {
	case "q", "ctrl+c":
		uc.Run(tea.Quit)
	default:
		p.keys = append(p.keys, key.String())
		if len(p.keys) > 12 {
			p.keys = p.keys[len(p.keys)-12:]
		}
	}
}

// model is the application root. It owns froth by pointer and forwards Bubble
// Tea's three methods; the value copy on every update is free.
type model struct{ f *froth.Froth }

func (m model) Init() tea.Cmd { return m.f.Init() }

func (m model) View() tea.View {
	// Program-wide terminal settings are the application's, not froth's: only
	// the root model's View reaches the renderer.
	v := m.f.View()
	v.AltScreen = true
	return v
}
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	return m, m.f.Update(msg)
}

func main() {
	root := &pane{title: "froth — M1: one pane"}
	m := model{f: froth.New(context.Background(), root)}

	if _, err := tea.NewProgram(m).Run(); err != nil {
		log.Fatal(fmt.Errorf("running panes: %w", err))
	}
}
