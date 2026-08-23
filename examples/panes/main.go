// Command panes is froth's running example: a root surface that assembles a
// filter field and three panes, places the field along the top and the panes
// side by side under it, and hands keys to whichever of them holds focus.
//
// The root draws a field of dots behind them, visible in the one-cell gap the
// layout leaves between the panes.
//
// Focus is one handle held by froth. Tab moves it between the surfaces created
// with froth.TabStop(); the notes pane is created without it, so the walk passes
// over it while it keeps its place in the row and its share of the space. No
// surface implements traversal and none binds tab: tab and shift-tab are froth's
// own bindings for actions froth services itself, and an application rebinds or
// disables them by declaring them like any other.
//
// The scope on the right is a focus scope. Tab neither lands on it nor walks
// into it, and tab inside it reaches nothing outside: g moves focus in, and the
// same key inside reaches the scope first — it is deeper on the chain than the
// root — and moves focus back out. Come back later and focus lands on the
// control it was on, because a surface remembers which child was last active.
//
// One keystroke shows what the update loop buys: the focused pane posts what it
// saw to its sibling, and both have handled it before anything is drawn.
// Pressing "d" posts a message that comes straight back, which is a cycle;
// froth abandons it, reports it through the diagnostic hook, and the program
// carries on.
//
// Quit is a binding declared by the root, so it fires while a pane holds focus:
// froth matches the key against the focus chain and hands the action to the
// surface that declared it. The panes declare one of their own, and tab is
// still a raw key they match themselves.
//
// The filter field along the top is created with froth.TextSink(), and it is
// what the two halves of that quit binding are for. While it holds focus, "q"
// is ordinary typing and lands in the field; "ctrl+c" is a chord, so it still
// resolves to the root and quits. Tab it away and "q" is a shortcut again.
package main

import (
	"context"
	"fmt"
	"log"

	tea "charm.land/bubbletea/v2"
	"github.com/jbeda/froth"
)

// The actions the application declares. An action is an identity rather than a
// key: the root answers actionQuit wherever focus is, and the panes answer
// actionCycle only while they hold it.
const (
	actionQuit       froth.Action = "quit"
	actionCycle      froth.Action = "cycle"
	actionEnterScope froth.Action = "enter scope"
	actionLeaveScope froth.Action = "leave scope"
)

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
	rt := &root{report: &report{}}
	m := model{f: froth.New(context.Background(), rt, froth.OnDiagnostic(rt.onDiagnostic))}

	if _, err := tea.NewProgram(m).Run(); err != nil {
		log.Fatal(fmt.Errorf("running panes: %w", err))
	}
}
