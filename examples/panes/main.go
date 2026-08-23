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

// The actions the application declares. An action is an identity rather than a
// key: the root answers actionQuit wherever focus is, and the panes answer
// actionCycle only while they hold it.
const (
	actionQuit       froth.Action = "quit"
	actionCycle      froth.Action = "cycle"
	actionEnterScope froth.Action = "enter scope"
	actionLeaveScope froth.Action = "leave scope"
)

// root is the surface the application hands froth. It creates the panes it is
// made of, says how they sit, and says which of them starts focused — all in
// Mount, because a surface owns its children and all three are its business and
// nobody else's.
type root struct {
	report *report
	scope  froth.Handle
}

func (rt *root) Mount(ac *froth.ApplyCtx) {
	left := &pane{
		title: "left",
		body: "Every key you press arrives here while this pane has focus, " +
			"and at no other surface.\n\n" +
			"Neither pane knows where focus is except by asking about itself, " +
			"and neither computes a position: the layout is the only thing " +
			"that places them.\n\n" +
			"tab hands focus to the right pane and shift-tab takes it back, " +
			"and neither pane says anything about tab: the binding is froth's. " +
			"The notes pane is skipped.",
	}
	right := &pane{
		title: "right",
		body: "froth: focus is one handle, raw keys go to it, and froth " +
			"walks it.\n\n" +
			"tab and shift-tab are froth's own bindings, serviced by froth " +
			"and delivered to nobody, so no pane implements the walk. The " +
			"pane that sees a key posts it to its sibling, and the frame is " +
			"drawn once, after both have handled it — you never see a " +
			"half-settled screen.\n\n" +
			"d posts a message that comes back to its sender, which is a " +
			"cycle. The iteration cap catches it and reports it in the notes " +
			"pane rather than hanging.\n\n" +
			"q or ctrl+c quits — and tab to the filter field to see why " +
			"the binding names both. g moves focus into the scope on the " +
			"right.",
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

	ac.Create(&filter{}, froth.TabStop(), froth.TextSink())
	leftH, rightH := ac.Create(left, froth.TabStop()), ac.Create(right, froth.TabStop())
	ac.Create(notes)

	// The scope is created without TabStop, so tab neither lands on it nor
	// walks into it. It is reached by name, which is what the root's g binding
	// does — and the same key inside the scope reaches the scope first and
	// takes focus back out.
	rt.scope = ac.Create(&scopePane{out: leftH}, froth.FocusScope())

	left.sibling, right.sibling = rightH, leftH
	ac.SetLayout(filterOnTop{rest: layout.Row(layout.Gap(1))})
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

// Quit belongs to the whole application, so the root declares it and the panes
// know nothing about it.
func (rt *root) Bindings() []froth.Binding {
	return []froth.Binding{{
		Action:        actionQuit,
		Keys:          []string{"q", "ctrl+c"},
		Help:          "quit",
		SurvivesModal: true,
	}, {
		Action: actionEnterScope,
		Keys:   []string{"g"},
		Help:   "focus the scope",
	}}
}

// The root sees no raw keys — those go to the focused pane — so the only thing
// addressed to it is the action it declared.
func (rt *root) Update(uc *froth.UpdateCtx, msg tea.Msg) {
	m, ok := msg.(froth.ActionMsg)
	if !ok {
		return
	}
	switch m.Action {
	case actionQuit:
		uc.Run(tea.Quit)
	case actionEnterScope:
		// Moving focus to a scope enters it: froth lands on the control the
		// scope last had focus on, or its first one.
		uc.Focus(rt.scope)
	}
}

// scopePane is a focus scope. Tab inside it moves between its own two controls and
// never reaches the panes beside it, and the walk out there never reaches the
// controls. Focus crosses the boundary only by being moved, which is what the
// binding below does — and what a dialog's open and close would do.
type scopePane struct {
	out froth.Handle
}

func (g *scopePane) Mount(ac *froth.ApplyCtx) {
	ac.Create(&control{label: "first"}, froth.TabStop())
	ac.Create(&control{label: "second"}, froth.TabStop())
	ac.SetLayout(stack{})
}

// The scope is on the focus chain while one of its controls has focus, and it
// is deeper than the root, so its g wins over the root's while focus is inside.
func (g *scopePane) Bindings() []froth.Binding {
	return []froth.Binding{{
		Action: actionLeaveScope,
		Keys:   []string{"g"},
		Help:   "leave the scope",
	}}
}

func (g *scopePane) Update(uc *froth.UpdateCtx, msg tea.Msg) {
	if m, ok := msg.(froth.ActionMsg); ok && m.Action == actionLeaveScope {
		uc.Focus(g.out)
	}
}

func (g *scopePane) Render(rc *froth.RenderCtx, r froth.Rect) string {
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("240")).
		Padding(0, 1).
		Width(r.W).
		Height(r.H).
		Render("scope\n\ng gets in and back out; tab cycles between the two " +
			"controls below and reaches nothing else. Come back and focus is " +
			"where you left it.")
}

// stack gives each child a row of the space, top to bottom, leaving the first
// two rows to the surface that owns them.
type stack struct{}

func (stack) Arrange(space froth.Rect, children []froth.Child) []froth.Rect {
	out := make([]froth.Rect, len(children))
	y := space.Y + max(space.H-len(children)-1, 0)
	for i := range out {
		out[i] = froth.Rect{X: space.X + 1, Y: y + i, W: max(space.W-2, 0), H: 1}
	}
	return out
}

// control is a one-line surface that does nothing but show whether it has
// focus. It is a tab stop, so the scope's walk lands on it.
type control struct{ label string }

func (c *control) Render(rc *froth.RenderCtx, r froth.Rect) string {
	if rc.Focused() {
		return lipgloss.NewStyle().
			Foreground(lipgloss.Color("214")).
			Bold(true).
			Render("▸ " + c.label)
	}
	return lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render("  " + c.label)
}

func (c *control) Update(*froth.UpdateCtx, tea.Msg) {}

// onDiagnostic is froth's one hook, registered at construction. It is ordinary
// application code called with no phase in progress, so it is free to touch the
// application's own surfaces — which is all this one does, putting what froth
// reported where the next frame will draw it.
func (rt *root) onDiagnostic(d froth.Diagnostic) {
	rt.report.text = d.String()
}

// filterOnTop gives the first child a strip along the top of the space and
// hands what is left to another layout. A layout is an interface the
// application may implement, and this one is the application's.
type filterOnTop struct{ rest froth.Layout }

func (l filterOnTop) Arrange(space froth.Rect, children []froth.Child) []froth.Rect {
	if len(children) == 0 {
		return nil
	}
	top := froth.Rect{X: space.X, Y: space.Y, W: space.W, H: min(3, space.H)}
	below := froth.Rect{X: space.X, Y: space.Y + top.H, W: space.W, H: space.H - top.H}
	return append([]froth.Rect{top}, l.rest.Arrange(below, children[1:])...)
}

// filter is a text sink: while it holds focus, froth matches no bare printable
// key against anybody's bindings and delivers it here as a character instead.
// It collects what it is given and draws it. It is not a text input — there is
// no cursor, and nothing to move one with.
type filter struct{ text string }

func (fl *filter) Render(rc *froth.RenderCtx, r froth.Rect) string {
	focused := rc.Focused()

	border := lipgloss.Color("240")
	if focused {
		border = lipgloss.Color("212")
	}
	body := "filter: " + fl.text
	if !focused && fl.text == "" {
		body = "tab here and type — q lands in the field, ctrl+c still quits"
	}
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(border).
		Padding(0, 1).
		Width(r.W).
		Height(r.H).
		Render(body)
}

// Only a press is typing: a release is the same key arriving a second time, and
// answering both would put every character in twice.
func (fl *filter) Update(uc *froth.UpdateCtx, msg tea.Msg) {
	k, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return
	}
	switch k.String() {
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

// The cycle binding is the pane's own, so it resolves only while this pane is
// on the focus chain.
func (p *pane) Bindings() []froth.Binding {
	return []froth.Binding{{
		Action: actionCycle,
		Keys:   []string{"d"},
		Help:   "post a message that comes back",
	}}
}

func (p *pane) Update(uc *froth.UpdateCtx, msg tea.Msg) {
	switch msg := msg.(type) {
	case froth.ActionMsg:
		if msg.Action == actionCycle {
			// Answering a message with the same message is the cycle the
			// iteration cap is for.
			uc.Post(uc.Self(), cycleMsg{})
		}
	case tea.KeyMsg:
		key := msg.String()
		p.status = "I had focus, so froth gave me " + strconv.Quote(key)
		uc.Post(p.sibling, echoMsg{key: key})
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
