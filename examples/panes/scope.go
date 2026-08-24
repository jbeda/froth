package main

import (
	"strconv"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/jbeda/froth"
)

// scopePane is a focus scope. Tab inside it moves between its own two controls and
// never reaches the panes beside it, and the walk out there never reaches the
// controls. Focus crosses the boundary only by being moved, which is what the
// binding below does — and what a dialog's open and close would do.
type scopePane struct {
	out   froth.Handle
	notes int
}

func (g *scopePane) Mount(ac *froth.ApplyCtx) {
	self := ac.Self()
	self.Create(&control{label: "first"}, froth.TabStop())
	self.Create(&control{label: "second"}, froth.TabStop())
	self.SetLayout(stack{})
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
	switch m := msg.(type) {
	case froth.ActionMsg:
		if m.Action == actionLeaveScope {
			uc.At(g.out).Focus()
		}
	case noteMsg:
		// Acting on a rising message is not taking it: the walk ends only where
		// something calls StopRising, and this surface deliberately does not.
		g.notes++
	}
}

// The scope is never the focused surface while one of its controls is — but it
// is on the chain above it, which is what FocusWithin asks and Focused cannot.
func (g *scopePane) Render(rc *froth.RenderCtx, r froth.Rect) string {
	border := lipgloss.Color("240")
	if rc.FocusWithin() {
		border = lipgloss.Color("212")
	}
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(border).
		Padding(0, 1).
		Width(r.W).
		Height(r.H).
		Render("scope\n\ng gets in and back out; tab cycles between the two " +
			"controls below and reaches nothing else. Come back and focus is " +
			"where you left it.\n\nn from a control sends a message up past " +
			"me: counted " + strconv.Itoa(g.notes) + ", taken by the root.")
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

// The control says what happened and does not know who acts on it: n posts a
// message that travels, and the surfaces above decide.
func (c *control) Bindings() []froth.Binding {
	return []froth.Binding{{
		Action: actionNote,
		Keys:   []string{"n"},
		Help:   "send a note upward",
	}}
}

func (c *control) Render(rc *froth.RenderCtx, r froth.Rect) string {
	if rc.Focused() {
		return lipgloss.NewStyle().
			Foreground(lipgloss.Color("214")).
			Bold(true).
			Render("▸ " + c.label)
	}
	return lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render("  " + c.label)
}

func (c *control) Update(uc *froth.UpdateCtx, msg tea.Msg) {
	if m, ok := msg.(froth.ActionMsg); ok && m.Action == actionNote {
		uc.Self().Post(noteMsg{from: c.label})
	}
}
