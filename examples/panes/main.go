// Command panes is froth's running example. It grows with each milestone; today
// it is a root surface that assembles two child panes, places them side by side
// with a row layout, and routes what it sees to them by posting messages.
//
// The root draws a field of dots behind them, which is visible in the one-cell
// gap the layout leaves between the panes — the panes cover the rest of it,
// because each fills the rect the layout gave it.
//
// What this milestone's update loop buys is visible in one keystroke: the root
// posts to the left pane, which posts to the right pane, and all three have
// handled it before anything is drawn. Pressing "d" posts a message that comes
// straight back, which is a cycle; froth abandons it, reports it through the
// diagnostic hook, and the program carries on.
package main

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/jbeda/froth"
	"github.com/jbeda/froth/layout"
)

// keyMsg is what the root forwards to the left pane, and echoMsg what the left
// pane forwards on to the right one. Neither is a Bubble Tea message: they are
// froth messages, addressed to one surface and delivered to nothing else.
type keyMsg struct{ key string }

type echoMsg struct{ key string }

// cycleMsg is the deliberate mistake: the root answers it by posting it back to
// itself, so the cascade never settles.
type cycleMsg struct{}

// report is where the diagnostic hook leaves what froth said. The hook is
// application code, called with no phase in progress and so with no context to
// post through; a value the application shares with the surface that draws it is
// the honest way onto the screen, and it keeps surface pointers out of the
// application — a handle is the only name a surface has.
type report struct{ text string }

// root is the surface the application hands froth. It creates the panes it is
// made of and says how they sit, both in Mount: a surface owns its children, so
// their layout is its business and nobody else's.
//
// It also holds the left pane's handle, because the root is what messages arrive
// at until focus lands, and a handle is what you need to address one.
type root struct {
	left   froth.Handle
	report *report
}

func (rt *root) Mount(ac *froth.ApplyCtx) {
	left := &pane{
		title: "left",
		body: "Every key you press arrives at the root, which posts it here. " +
			"This pane posts it on to its sibling.\n\n" +
			"Neither pane knows where the message came from, and neither " +
			"computes a position: the layout is the only thing that places them.",
	}
	right := &pane{
		title:  "right",
		report: rt.report,
		body: "froth — M1: Deliver and Apply drain before anything is drawn.\n\n" +
			"Three surfaces handle each keystroke — root, left, right — and " +
			"the frame is drawn once, after all three. You never see a half-" +
			"settled screen.\n\n" +
			"d posts a message that comes back to its sender, which is a " +
			"cycle. The iteration cap catches it and reports it below rather " +
			"than hanging.\n\n" +
			"q or ctrl+c quits.",
	}

	rt.left = ac.Create(left)
	left.echoTo = ac.Create(right)
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
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch key := msg.String(); key {
		case "q", "ctrl+c":
			uc.Run(tea.Quit)
		case "d":
			uc.Post(uc.Self(), cycleMsg{})
		default:
			uc.Post(rt.left, keyMsg{key: key})
		}
	case cycleMsg:
		// Answering a message with the same message is the cycle the cap is for.
		uc.Post(uc.Self(), msg)
	}
}

// onDiagnostic is froth's one hook, registered at construction. It is ordinary
// application code called with no phase in progress, so it is free to touch the
// application's own surfaces — which is all this one does, putting what froth
// reported where the next frame will draw it.
func (rt *root) onDiagnostic(d froth.Diagnostic) {
	rt.report.text = d.String()
}

// pane is a child surface: two methods, and its state on a pointer receiver. It
// fills whatever rect it is given, which is the layout's answer rather than a
// size of its own.
type pane struct {
	title  string
	body   string
	status string
	echoTo froth.Handle // zero for the pane at the end of the chain
	report *report      // nil for a pane that does not draw diagnostics
}

func (p *pane) Render(rc *froth.RenderCtx, r froth.Rect) string {
	status := p.status
	if status == "" {
		status = "nothing yet — press a key"
	}
	if p.report != nil && p.report.text != "" {
		status += "\n\n" + p.report.text
	}
	content := p.title + "\n\n" + p.body + "\n\n" +
		lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Render(status)

	// Lip Gloss counts the border and padding inside Width and Height, so a box
	// styled to the rect fills it exactly.
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		Padding(0, 1).
		Width(r.W).
		Height(r.H).
		Render(content)
}

func (p *pane) Update(uc *froth.UpdateCtx, msg tea.Msg) {
	switch msg := msg.(type) {
	case keyMsg:
		p.status = "root posted me " + strconv.Quote(msg.key)
		uc.Post(p.echoTo, echoMsg{key: msg.key})
	case echoMsg:
		p.status = "left pane passed on " + strconv.Quote(msg.key) +
			", same frame as the keystroke"
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
	rt := &root{report: &report{}}
	m := model{f: froth.New(context.Background(), rt, froth.OnDiagnostic(rt.onDiagnostic))}

	if _, err := tea.NewProgram(m).Run(); err != nil {
		log.Fatal(fmt.Errorf("running panes: %w", err))
	}
}
