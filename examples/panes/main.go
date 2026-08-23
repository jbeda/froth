// Command panes is froth's running example: a root surface that assembles three
// child panes, places them side by side with a row layout, and hands keys to
// whichever of them holds focus.
//
// The root draws a field of dots behind them, visible in the one-cell gap the
// layout leaves between the panes.
//
// Focus is one handle held by froth. Tab moves it between the two panes created
// with froth.TabStop(); the notes pane is created without it, so the walk passes
// over it while it keeps its place in the row and its share of the space. No
// surface implements traversal — the panes ask froth to move focus, and froth
// decides where it goes.
//
// One keystroke shows what the update loop buys: the focused pane posts what it
// saw to its sibling, and both have handled it before anything is drawn.
// Pressing "d" posts a message that comes straight back, which is a cycle;
// froth abandons it, reports it through the diagnostic hook, and the program
// carries on.
//
// The quit and tab keys sit on the panes rather than the root because a raw key
// reaches only the focused surface. Bindings declared along the focus chain are
// what move them.
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
// called with no phase in progress and so has no context to post through; a
// value shared with the surface that draws it is the way onto the screen.
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
			"that places them.\n\n" +
			"tab hands focus to the right pane and shift-tab takes it back. " +
			"The notes pane is skipped.",
	}
	right := &pane{
		title: "right",
		body: "froth: focus is one handle, raw keys go to it, and froth " +
			"walks it.\n\n" +
			"tab and shift-tab move focus, and neither pane implements the " +
			"walk: they ask froth, and froth answers from the tree. The pane " +
			"that sees a key posts it to its sibling, and the frame is drawn " +
			"once, after both have handled it — you never see a half-settled " +
			"screen.\n\n" +
			"d posts a message that comes back to its sender, which is a " +
			"cycle. The iteration cap catches it and reports it in the notes " +
			"pane rather than hanging.\n\n" +
			"q or ctrl+c quits.",
	}
	notes := &pane{
		title:  "notes",
		report: rt.report,
		body: "This pane was created without froth.TabStop(), so tab never " +
			"lands here — but it still sits third in the row, still gets its " +
			"share of the space, and could still be given focus by name. " +
			"Being outside the keyboard walk is not being moved and not " +
			"being hidden.\n\n" +
			"Whatever froth reports through the diagnostic hook shows up " +
			"under the title above.",
	}

	leftH, rightH := ac.Create(left, froth.TabStop()), ac.Create(right, froth.TabStop())
	ac.Create(notes)
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
		switch {
		case focused:
			status = "nothing yet — press a key, or tab to hand focus over"
		case p.report != nil:
			status = "nothing reported yet — press d"
		default:
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
		case "tab", "shift+tab":
			// froth walks the tree; the pane only says which way. Focus is
			// structural, so the move commits in the Apply after this delivery
			// — still within this update, and before anything is drawn.
			if key == "tab" {
				uc.FocusNext()
			} else {
				uc.FocusPrev()
			}
			p.status = "asked froth for the " + key + " stop, and it walked the tree"
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
