package main

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/jbeda/froth"
)

// confirmSpec is everything a confirm dialog is told: what to ask, what its two
// buttons say, and who is waiting for the answer. The dialog knows nothing
// else about its opener, and the opener holds no reference to the dialog it
// does not get from OpenModal.
type confirmSpec struct {
	title   string
	body    string
	yes, no string
	replyTo froth.Handle
}

// confirmResult is what the dialog posts to replyTo before it leaves. Dialog is
// the handle OpenModal returned, so an opener with more than one question in
// flight can tell the answers apart.
type confirmResult struct {
	ok     bool
	dialog froth.Handle
}

// pressedMsg is what a button posts to the surface it reports to. It names the
// button by handle rather than by meaning, so a button is the same type
// whichever answer it stands for.
type pressedMsg struct{ button froth.Handle }

// confirmWidth is the dialog's outer width. A float takes the rectangle its
// output makes, so the dialog sizes itself rather than filling what it was
// offered — filling the offer would be full-screen.
const confirmWidth = 48

// confirmDialog asks a yes-or-no question over the whole application. It is
// opened with OpenModal and Float, holds two buttons, and answers to y, n,
// enter on a button, a click on a button, and esc. Whichever way it is
// answered it posts one confirmResult to the spec's replyTo and destroys
// itself, and froth returns focus to where it was.
type confirmDialog struct {
	spec    confirmSpec
	self    froth.Handle
	yes, no froth.Handle
}

func newConfirmDialog(spec *confirmSpec) *confirmDialog { return &confirmDialog{spec: *spec} }

func (d *confirmDialog) Mount(ac *froth.ApplyCtx) {
	self := ac.Self()
	d.self = self.Handle()
	d.yes = self.Create(newButton(d.spec.yes, d.self))
	d.no = self.Create(newButton(d.spec.no, d.self))
	self.SetLayout(buttonRow{})
	self.SetBindings([]froth.Binding{
		{Action: actionYes, Keys: []string{"y"}, Help: d.spec.yes},
		{Action: actionNo, Keys: []string{"n"}, Help: d.spec.no},
		{Action: froth.ActionCancel, Keys: []string{"esc"}, Help: "back out"},
	})
	// The modal would take focus at its first stop, the yes button. Starting
	// on no means enter alone cannot confirm something destructive.
	ac.Child(d.no).Focus()
}

func (d *confirmDialog) Update(uc *froth.UpdateCtx, msg tea.Msg) {
	switch m := msg.(type) {
	case froth.ActionMsg:
		switch m.Action {
		case actionYes:
			d.answer(uc, true)
		case actionNo, froth.ActionCancel:
			d.answer(uc, false)
		default:
			return
		}
		uc.StopRising()
	case pressedMsg:
		d.answer(uc, m.button == d.yes)
	}
}

// answer replies and leaves. Both are recorded now and commit at the next
// Apply, so the opener sees the result in this same update, while the dialog
// is still on screen.
func (d *confirmDialog) answer(uc *froth.UpdateCtx, ok bool) {
	uc.At(d.spec.replyTo).Post(confirmResult{ok: ok, dialog: d.self})
	uc.Self().Destroy()
}

// The box is drawn to its own size and the layout below places the buttons
// against its inside bottom edge, so the two have to agree on the border and
// padding; both read them from here.
const (
	confirmPad    = 1
	confirmInsetX = 1 + confirmPad // border, then padding
)

func (d *confirmDialog) Render(rc *froth.RenderCtx, _ froth.Size) string {
	inner := confirmWidth - 2*confirmInsetX
	body := lipgloss.NewStyle().Width(inner).Render(d.spec.body)
	content := lipgloss.NewStyle().Bold(true).Render(d.spec.title) + "\n\n" + body + "\n\n" +
		strings.Repeat(" ", inner) // the buttons' row; they draw over it
	border := lipgloss.Color("240")
	if rc.FocusWithin() {
		border = lipgloss.Color("212")
	}
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(border).
		Padding(0, confirmPad).
		Width(confirmWidth).
		Render(content)
}

// buttonRow places a dialog's children left to right along the last line
// inside its border.
type buttonRow struct{}

func (buttonRow) Arrange(space froth.Rect, children []froth.Child) []froth.Rect {
	out := make([]froth.Rect, len(children))
	x, y := space.X+confirmInsetX, space.Y+space.H-2
	for i := range children {
		out[i] = froth.Rect{X: x, Y: y, W: buttonWidth, H: 1}
		x += buttonWidth + 2
	}
	return out
}

// buttonWidth is what every button is given. The labels here are short, and
// one width keeps the row's spacing even.
const buttonWidth = 12

// button is a one-line control that reports a press to one surface. It does not
// know what pressing it means: the surface it reports to matches the handle.
type button struct {
	label    string
	reportTo froth.Handle
}

func newButton(label string, reportTo froth.Handle) *button {
	return &button{label: label, reportTo: reportTo}
}

func (b *button) Declare() froth.Flags { return froth.Flags{TabStop: true, ClickFocus: true} }

func (b *button) Mount(ac *froth.ApplyCtx) {
	ac.Self().SetBindings([]froth.Binding{{
		Action: actionPress,
		Keys:   []string{"enter"},
		Help:   "press " + b.label,
	}})
}

func (b *button) Update(uc *froth.UpdateCtx, msg tea.Msg) {
	switch m := msg.(type) {
	case froth.ActionMsg:
		if m.Action != actionPress {
			return
		}
		uc.StopRising()
	case tea.MouseClickMsg:
		// The click moved focus here already; the press is the same as enter.
	default:
		return
	}
	uc.At(b.reportTo).Post(pressedMsg{button: uc.Self().Handle()})
}

func (b *button) Render(rc *froth.RenderCtx, s froth.Size) string {
	style := lipgloss.NewStyle().Width(s.W).Align(lipgloss.Center).
		Foreground(lipgloss.Color("240"))
	if rc.Focused() {
		style = style.Foreground(lipgloss.Color("214")).Bold(true).Reverse(true)
	}
	return style.Render("[ " + b.label + " ]")
}
