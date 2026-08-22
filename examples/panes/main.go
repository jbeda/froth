// Command panes is froth's running example. It grows with each milestone; today
// it is a root surface that assembles two child panes, places them side by side
// with a row layout, and hands keys to whichever of them holds focus.
//
// The root draws a field of dots behind them, which is visible in the one-cell
// gap the layout leaves between the panes — the panes cover the rest of it,
// because each fills the rect the layout gave it.
//
// Focus is one handle held by froth. Tab moves it between the panes, each pane
// reads its own focus while rendering and draws itself differently for it, and
// every keystroke is delivered to the focused pane and to no other surface.
//
// What the update loop buys is visible in one keystroke: the focused pane posts
// what it saw to its sibling, and both have handled it before anything is drawn.
// Pressing "d" posts a message that comes straight back, which is a cycle; froth
// abandons it, reports it through the diagnostic hook, and the program carries
// on.
//
// The quit key is on both panes rather than on the root, because a raw key
// reaches only the focused surface. A global shortcut that works wherever focus
// is arrives in M2, when keys resolve against bindings declared along the focus
// chain before anything is delivered.
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

// echoMsg is what the focused pane forwards to its sibling. It is not a Bubble
// Tea message: it is a froth message, addressed to one surface and delivered to
// nothing else.
type echoMsg struct{ key string }

// cycleMsg is the deliberate mistake: a pane answers it by posting it back to
// itself, so the cascade never settles.
type cycleMsg struct{}

// report is where the diagnostic hook leaves what froth said. The hook is
// application code, called with no phase in progress and so with no context to
// post through; a value the application shares with the surface that draws it is
// the honest way onto the screen, and it keeps surface pointers out of the
// application — a handle is the only name a surface has.
type report struct{ text string }

// root is the surface the application hands froth. It creates the panes it is
// made of, says how they sit, and says which of them starts focused — all in
// Mount, because a surface owns its children and all three are its business and
// nobody else's.
type root struct {
	report *report
}

func (rt *root) Mount(ac *froth.ApplyCtx) {
	left := &pane{
		title: "left",
		body: "Every key you press arrives here while this pane has focus, " +
			"and at no other surface.\n\n" +
			"Neither pane knows where focus is except by asking about itself, " +
			"and neither computes a position: the layout is the only thing " +
			"that places them.",
	}
	right := &pane{
		title:  "right",
		report: rt.report,
		body: "froth — M1: focus is one handle, and raw keys go to it.\n\n" +
			"tab moves focus. The pane that sees a key posts it to its " +
			"sibling, and the frame is drawn once, after both have handled " +
			"it — you never see a half-settled screen.\n\n" +
			"d posts a message that comes back to its sender, which is a " +
			"cycle. The iteration cap catches it and reports it below rather " +
			"than hanging.\n\n" +
			"q or ctrl+c quits.",
	}

	leftH, rightH := ac.Create(left), ac.Create(right)
	left.sibling, right.sibling = rightH, leftH
	ac.SetLayout(layout.Row(layout.Gap(1)))
	ac.Focus(leftH)
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

// The root sees no keys — those go to the focused pane — and nothing else is
// addressed to it, so it has nothing to handle.
func (rt *root) Update(uc *froth.UpdateCtx, msg tea.Msg) {}

// onDiagnostic is froth's one hook, registered at construction. It is ordinary
// application code called with no phase in progress, so it is free to touch the
// application's own surfaces — which is all this one does, putting what froth
// reported where the next frame will draw it.
func (rt *root) onDiagnostic(d froth.Diagnostic) {
	rt.report.text = d.String()
}

// pane is a child surface: two methods, and its state on a pointer receiver. It
// fills whatever rect it is given, which is the layout's answer rather than a
// size of its own, and it draws itself differently depending on whether it holds
// focus.
type pane struct {
	title   string
	body    string
	status  string
	sibling froth.Handle
	report  *report // nil for a pane that does not draw diagnostics
}

func (p *pane) Render(rc *froth.RenderCtx, r froth.Rect) string {
	focused := rc.Focused()

	title := p.title
	border, accent := lipgloss.Color("240"), lipgloss.Color("240")
	if focused {
		title = "▸ " + p.title + " — focused"
		border, accent = lipgloss.Color("212"), lipgloss.Color("214")
	}

	status := p.status
	if status == "" {
		if focused {
			status = "nothing yet — press a key, or tab to hand focus over"
		} else {
			status = "tab moves focus here"
		}
	}
	if p.report != nil && p.report.text != "" {
		status += "\n\n" + p.report.text
	}

	// The status sits under the title rather than under the body, so it stays
	// on screen in a short terminal: the body is what the clip takes first.
	content := lipgloss.NewStyle().Bold(focused).Render(title) + "\n" +
		lipgloss.NewStyle().Foreground(accent).Render(status) + "\n\n" +
		p.body

	// Lip Gloss counts the border and padding inside Width and Height, so a box
	// styled to the rect fills it exactly.
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(border).
		Padding(0, 1).
		Width(r.W).
		Height(r.H).
		Render(content)
}

func (p *pane) Update(uc *froth.UpdateCtx, msg tea.Msg) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch key := msg.String(); key {
		case "q", "ctrl+c":
			uc.Run(tea.Quit)
		case "tab":
			// Focus is structural, so it commits in the Apply after this
			// delivery — still within this update, and before anything is
			// drawn.
			uc.Focus(p.sibling)
			p.status = "handed focus to the other pane"
		case "d":
			// Answering a message with the same message is the cycle the
			// iteration cap is for.
			uc.Post(uc.Self(), cycleMsg{})
		default:
			p.status = "I had focus, so froth gave me " + strconv.Quote(key)
			uc.Post(p.sibling, echoMsg{key: key})
		}
	case echoMsg:
		p.status = "the focused pane passed on " + strconv.Quote(msg.key) +
			", same frame as the keystroke"
	case cycleMsg:
		uc.Post(uc.Self(), msg)
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
