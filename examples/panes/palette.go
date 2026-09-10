package main

import (
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/jbeda/froth"
)

// palette is a floating field that runs an action by name. It holds capture
// from the moment it mounts, so every mouse message reaches it wherever it
// lands, and a click outside its own rect dismisses it — the click is spent on
// that and reaches nothing beneath. It also leaves when froth takes capture
// away, which a modal opening over it does.
//
// It is not modal: the panes behind it still draw, tab would still walk them,
// and only capture stands between a click and them. That is the point of it.
//
// It is not a modal, so froth does not hand focus back when it leaves; it is
// told where focus was and puts it back itself.
//
// What it lists is what the focus chain offers, and while it is up the chain
// runs from the palette itself, so the actions of the surface it was opened
// over are not among them.
type palette struct {
	text string
	back froth.Handle
}

func newPalette(back froth.Handle) *palette { return &palette{back: back} }

func (p *palette) Declare() froth.Flags {
	return froth.Flags{ClickFocus: true, TextSink: true}
}

func (p *palette) Mount(ac *froth.ApplyCtx) {
	ac.Self().Capture()
	ac.Self().SetBindings([]froth.Binding{{
		Action: froth.ActionCancel,
		Keys:   []string{"esc"},
		Help:   "dismiss the palette",
	}})
}

func (p *palette) Update(uc *froth.UpdateCtx, msg tea.Msg) {
	switch m := msg.(type) {
	case froth.CaptureLostMsg:
		p.leave(uc)
	case tea.MouseClickMsg:
		if r, ok := uc.Self().Rect(); ok && !r.Contains(froth.Pt(m.X, m.Y)) {
			p.leave(uc)
		}
	case froth.ActionMsg:
		if m.Action == froth.ActionCancel {
			uc.StopRising()
		}
		// Any action starts here while the palette holds focus, its own
		// invocation included. It rises on to whoever answers it, and the
		// palette is gone by the time the next frame draws.
		p.leave(uc)
	case tea.KeyPressMsg:
		switch m.String() {
		case "enter":
			if a, ok := match(uc.Query().Actions(), p.text); ok {
				uc.Invoke(a.Action)
			}
		case "backspace":
			if r := []rune(p.text); len(r) > 0 {
				p.text = string(r[:len(r)-1])
			}
		default:
			p.text += m.Text
		}
	}
}

// leave puts focus back where it was and takes the palette down. Focus first:
// a focus move onto a surface that has left is refused, and this one is still
// live until the Apply that commits the destroy.
func (p *palette) leave(uc *froth.UpdateCtx) {
	uc.At(p.back).Focus()
	uc.Self().Destroy()
}

// paletteWidth is the palette's outer width; a float is the size of what it
// draws.
const paletteWidth = 40

func (p *palette) Render(rc *froth.RenderCtx, _ froth.Size) string {
	body := "> " + p.text
	if a, ok := match(rc.Query().Actions(), p.text); ok {
		body += "\n  ⏎ " + a.Help
	} else {
		body += "\n  type an action; esc or click outside"
	}
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("214")).
		Padding(0, 1).
		Width(paletteWidth).
		Render(body)
}
