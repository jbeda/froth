package main

import (
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/jbeda/froth"
)

// scope is a focus scope, and on this board it is the wall rather than the
// dialog it will become: a directional move never crosses into it or out of it,
// and tab neither lands on it nor walks inside. The only way across is being
// moved by name, which is what g does from either side.
//
// What that costs is the thing to have an opinion about. Pressing an arrow at
// the boundary does nothing at all, and nothing says a boundary is why.
type scope struct {
	bd  *board
	out froth.Handle
}

func (s *scope) Mount(ac *froth.ApplyCtx) {
	self := ac.Self()
	for _, name := range []string{"Reply", "Forward", "Archive"} {
		self.Create(&control{name: name, bd: s.bd})
	}
	self.SetLayout(froth.LayoutFunc(scopeInner))
	// The scope is on the focus chain while one of its controls holds focus,
	// and it is deeper than the root, so its g wins over the root's from
	// inside.
	self.SetBindings([]froth.Binding{{
		Action: actionLeaveScope,
		Keys:   []string{"g"},
		Help:   "leave the scope",
	}})
}

func (s *scope) Update(uc *froth.UpdateCtx, msg tea.Msg) {
	if m, ok := msg.(froth.ActionMsg); ok && m.Action == actionLeaveScope {
		uc.At(s.out).Focus()
		uc.StopRising()
	}
}

func (s *scope) Render(rc *froth.RenderCtx, size froth.Size) string {
	if size.Empty() {
		return ""
	}
	border := lipgloss.Color("240")
	if rc.FocusWithin() {
		border = lipgloss.Color("212")
	}
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(border).
		Padding(0, 1).
		Width(size.W).
		Height(size.H).
		Render("Actions\n\na focus scope: g in\nand out, arrows never")
}
