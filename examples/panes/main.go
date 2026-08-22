// Command panes is froth's running example. It grows with each milestone; today
// it is a root surface that assembles two child panes and attaches a row layout
// to place them side by side.
//
// The root draws a field of dots behind them, which is visible in the one-cell
// gap the layout leaves between the panes — the panes cover the rest of it,
// because each fills the rect the layout gave it.
package main

import (
	"context"
	"fmt"
	"log"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/jbeda/froth"
	"github.com/jbeda/froth/layout"
)

// root is the surface the application hands froth. It creates the panes it is
// made of and says how they sit, both in Mount: a surface owns its children, so
// their layout is its business and nobody else's.
//
// Only the root sees messages, so the panes are static: a keystroke reaches
// whichever surface is active, and focus arrives later in this milestone.
type root struct{}

func (rt *root) Mount(ac *froth.ApplyCtx) {
	ac.Create(&pane{
		title: "left",
		body: "Each pane is given a rect by the layout and fills it, so " +
			"resizing the terminal resizes both.\n\n" +
			"Neither pane knows the other exists, and neither computes a " +
			"position: the layout is the only thing that places them.",
	})
	ac.Create(&pane{
		title: "right",
		body: "froth — M1: two panes placed by a layout.\n\n" +
			"A row layout divides the root's rect between the panes and leaves " +
			"a one-cell gap, which is where the dots behind them show " +
			"through.\n\n" +
			"A pane is clipped to its rect, so this paragraph stops at the " +
			"pane's edge however long it gets and never bleeds over its " +
			"sibling.\n\n" +
			"q or ctrl+c quits.",
	})
	ac.SetLayout(layout.Row(layout.Gap(1)))
}

func (rt *root) Render(rc *froth.RenderCtx, r froth.Rect) string {
	row := strings.Repeat("·", r.W)
	rows := make([]string, r.H)
	for i := range rows {
		rows[i] = row
	}
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color("240")).
		Render(strings.Join(rows, "\n"))
}

func (rt *root) Update(uc *froth.UpdateCtx, msg tea.Msg) {
	if key, ok := msg.(tea.KeyPressMsg); ok {
		switch key.String() {
		case "q", "ctrl+c":
			uc.Run(tea.Quit)
		}
	}
}

// pane is a child surface: two methods, and its state on a pointer receiver. It
// fills whatever rect it is given, which is the layout's answer rather than a
// size of its own.
type pane struct {
	title string
	body  string
}

func (p *pane) Render(rc *froth.RenderCtx, r froth.Rect) string {
	// Lip Gloss counts the border and padding inside Width and Height, so a box
	// styled to the rect fills it exactly.
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		Padding(0, 1).
		Width(r.W).
		Height(r.H).
		Render(p.title + "\n\n" + p.body)
}

func (p *pane) Update(uc *froth.UpdateCtx, msg tea.Msg) {}

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
	m := model{f: froth.New(context.Background(), &root{})}

	if _, err := tea.NewProgram(m).Run(); err != nil {
		log.Fatal(fmt.Errorf("running panes: %w", err))
	}
}
