// Command dir-nav is a board for judging directional focus movement: a screen
// shaped like a real application, laid out so that no move across it is a move
// across a tidy grid, and keys that slide the geometry around under a fixed
// focus. It is meant to be sat in front of and pressed at.
//
// README.md in this directory is the guide to driving it — the board, the keys,
// which of better()'s tests each press exercises, and what driving it has taught
// so far. How the board finds out where focus is, which is the one thing here
// that is not ordinary froth usage, is on the board type.
package main

import (
	"context"
	"fmt"
	"log"

	tea "charm.land/bubbletea/v2"
	"github.com/jbeda/froth"
)

// model is the application root. It owns froth by pointer and forwards Bubble
// Tea's three methods.
type model struct{ f *froth.Froth }

func (m model) Init() tea.Cmd { return m.f.Init() }

func (m model) View() tea.View {
	v := m.f.View()
	v.AltScreen = true
	return v
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	return m, m.f.Update(msg)
}

func main() {
	bd := &board{splits: defaultSplits()}
	rt := &root{bd: bd}
	m := model{f: froth.New(context.Background(), rt, froth.OnDiagnostic(func(d froth.Diagnostic) {
		bd.diag = d.String()
	}))}

	if _, err := tea.NewProgram(m).Run(); err != nil {
		log.Fatal(fmt.Errorf("running dir-nav: %w", err))
	}
}
