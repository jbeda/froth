package main

import (
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/jbeda/froth"
)

// dialogW and dialogH are the box the dialog draws, border included. A float is
// offered the whole screen and takes the rect its output makes, so a dialog
// sized to what it was offered would be full-screen.
const (
	dialogW = 32
	dialogH = 9
)

// dialog is the board's Actions dialog: a modal float holding three controls.
// Being modal makes it a focus scope, takes focus as it opens and hands focus
// back where it found it as it goes, and takes the board's own keys away while
// it is up — the size keys included, which is why the board cannot be resized
// from inside it.
//
// It goes on g, on esc, and on a control being picked. It carries the handle of
// the surface that opened it because that surface holds the dialog's handle
// and has to be told it is gone: froth ties the dialog's lifetime to its opener
// and tells nobody when it leaves.
type dialog struct {
	bd     *board
	opener froth.Handle
}

// dialogGone is what the dialog tells its opener as it leaves.
type dialogGone struct{}

func (d *dialog) Mount(ac *froth.ApplyCtx) {
	self := ac.Self()
	for _, name := range []string{"Reply", "Forward", "Archive"} {
		self.Create(&control{name: name, bd: d.bd})
	}
	self.SetLayout(froth.LayoutFunc(dialogInner))
	self.SetBindings([]froth.Binding{
		{Action: actionCloseDialog, Keys: []string{"g"}, Help: "close the dialog"},
		{Action: froth.ActionCancel, Keys: []string{"esc"}, Help: "close the dialog"},
	})
}

// A pick rises from the control that took it, which has already written the
// name on the board: what the control cannot do is take the dialog down, since
// a surface destroys itself and its own children and nothing else.
func (d *dialog) Update(uc *froth.UpdateCtx, msg tea.Msg) {
	m, ok := msg.(froth.ActionMsg)
	if !ok {
		return
	}
	switch m.Action {
	case actionCloseDialog, actionPick, froth.ActionCancel:
		uc.At(d.opener).Post(dialogGone{})
		uc.Self().Destroy()
		uc.StopRising()
	}
}

func (d *dialog) Render(_ *froth.RenderCtx, size froth.Size) string {
	if size.W < dialogW || size.H < dialogH {
		return ""
	}
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("212")).
		Background(lipgloss.Color("235")).
		Padding(0, 1).
		Width(dialogW).
		Height(dialogH).
		Render("Actions\n\nenter picks · esc closes")
}
