package main

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/jbeda/froth"
	"github.com/jbeda/froth/layout"
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
