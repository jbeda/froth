// Command panes is froth's running example. It grows with each milestone; today
// it is a root surface that assembles two child panes when it enters the tree.
//
// The children are both placed at the root's whole rect, because no layout
// exists yet — so they stack rather than sit side by side, which is what makes
// the ordering visible. Pane two was created second and draws over pane one.
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

// root is the surface the application hands froth. It draws the backdrop and
// creates the panes it is made of.
type root struct {
	keys []string
}

func (rt *root) Mount(ac *froth.ApplyCtx) {
	ac.Create(&pane{title: "pane one", body: "created first — drawn below", w: 46, h: 9})
	ac.Create(&pane{title: "pane two", body: "created second — drawn above", w: 32, h: 5})
}

func (rt *root) Render(rc *froth.RenderCtx, r froth.Rect) string {
	body := "froth — M1: a root with two child panes\n\n" +
		"Both children sit at the root's whole rect until a layout has a say.\n" +
		"Press keys to see them arrive. q or ctrl+c quits."
	if len(rt.keys) > 0 {
		body += "\n\n" + strings.Join(rt.keys, " ")
	}

	// Lip Gloss counts the border and padding inside Width and Height, so a box
	// styled to the rect fills it exactly.
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		Padding(0, 1).
		Width(r.W).
		Height(r.H).
		AlignVertical(lipgloss.Bottom).
		Render(body)
}

func (rt *root) Update(uc *froth.UpdateCtx, msg tea.Msg) {
	key, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return
	}
	switch key.String() {
	case "q", "ctrl+c":
		uc.Run(tea.Quit)
	default:
		rt.keys = append(rt.keys, key.String())
		if len(rt.keys) > 12 {
			rt.keys = rt.keys[len(rt.keys)-12:]
		}
	}
}

// pane is a child surface: two methods, and its state on a pointer receiver. It
// draws at a fixed size for now, so both panes stay visible where they overlap.
type pane struct {
	title string
	body  string
	w, h  int
}

func (p *pane) Render(rc *froth.RenderCtx, r froth.Rect) string {
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		Padding(0, 1).
		Width(min(p.w, r.W)).
		Height(min(p.h, r.H)).
		// Bottom-aligned so the pane underneath still says what it is where it
		// is not covered.
		AlignVertical(lipgloss.Bottom).
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
