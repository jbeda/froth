package main

import (
	"strconv"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/jbeda/froth"
)

// echoMsg is what the focused pane forwards to its sibling. It is not a Bubble
// Tea message: it is a froth message, addressed to one surface and delivered to
// nothing else.
type echoMsg struct{ key string }

// cycleMsg is the deliberate mistake: a pane answers it by posting it back to
// itself, so the cascade never settles.
type cycleMsg struct{}

// noteMsg travels. Embedding froth.Rising means froth offers it to the surface
// it was posted to, then to that surface's parent, and on up — so a control
// deep in the tree says what happened without knowing who cares.
type noteMsg struct {
	froth.Rising
	from string
}

// report is where the diagnostic hook leaves what froth said. The hook is
// called with no phase in progress and so has no context to post through; a
// value shared with the surface that draws it is the way onto the screen.
type report struct{ text string }

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
	note    *report // nil for a pane that does not draw what rose to the root
	torn    *report // every pane writes here on the way out; the notes pane draws it

	// tabStop is the one thing about a pane that differs between them: the
	// notes pane is out of the walk and the other two are in it.
	tabStop bool
}

func (p *pane) Declare() froth.Flags { return froth.Flags{TabStop: p.tabStop} }

// Unmount is where a surface releases what froth does not own. This one owns
// nothing, so it only says it ran — which is the point worth seeing: it runs
// when the pane is closed, and again for every pane still standing when the
// program shuts froth down.
func (p *pane) Unmount(*froth.ApplyCtx) {
	if p.torn != nil {
		p.torn.text = "the " + p.title + " pane unmounted"
	}
}

func (p *pane) Render(rc *froth.RenderCtx, s froth.Size) string {
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
	if p.note != nil && p.note.text != "" {
		status += "\n\n" + p.note.text
	}
	// Every pane writes what it says on the way out; the pane that draws
	// diagnostics is the one that draws this too.
	if p.report != nil && p.torn != nil && p.torn.text != "" {
		status += "\n\n" + p.torn.text
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
		Width(s.W).
		Height(s.H).
		Render(content)
}

// A surface with no children still has a Mount, because declaring bindings is
// something a surface does about itself. These are the pane's own, so they
// resolve only while this pane is on the focus chain.
func (p *pane) Mount(ac *froth.ApplyCtx) {
	ac.Self().SetBindings([]froth.Binding{{
		Action: actionCycle,
		Keys:   []string{"d"},
		Help:   "post a message that comes back",
	}, {
		Action: actionClose,
		Keys:   []string{"x"},
		Help:   "close this pane",
	}})
}

func (p *pane) Update(uc *froth.UpdateCtx, msg tea.Msg) {
	switch msg := msg.(type) {
	case froth.ActionMsg:
		switch msg.Action {
		case actionCycle:
			// Answering a message with the same message is the cycle the
			// iteration cap is for.
			uc.Self().Post(cycleMsg{})
			uc.StopRising()
		case actionClose:
			// A surface removes itself. The removal commits at the next Apply,
			// so this pane is still live for the rest of this update — and
			// froth moves focus off it rather than leaving the keyboard
			// pointing at nothing.
			uc.Self().Destroy()
			uc.StopRising()
		}
	case tea.KeyMsg:
		key := msg.String()
		p.status = "I had focus, so froth gave me " + strconv.Quote(key)
		uc.At(p.sibling).Post(echoMsg{key: key})
	case echoMsg:
		p.status = "the focused pane passed on " + strconv.Quote(msg.key) +
			", same frame as the keystroke"
	case cycleMsg:
		uc.Self().Post(msg)
	}
}
