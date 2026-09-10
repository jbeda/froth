package main

import (
	tea "charm.land/bubbletea/v2"
	"github.com/jbeda/froth"
)

// The three containers the board is divided into. A container draws nothing and
// is not a tab stop, so traversal walks through it without landing on it and a
// directional move never resolves to it — the panels inside are what navigation
// sees.

// toolbar is the row of buttons along the top: three of them, at three
// different widths, aligned with nothing below.
type toolbar struct{ bd *board }

func (t *toolbar) Mount(ac *froth.ApplyCtx) {
	self := ac.Self()
	for i, name := range []string{"Compose", "Refresh", "Help"} {
		self.Create(&panel{name: name, region: regButton, index: i, bd: t.bd})
	}
	self.SetLayout(buttons{s: &t.bd.splits})
}

func (t *toolbar) Render(*froth.RenderCtx, froth.Size) string { return "" }
func (t *toolbar) Update(*froth.UpdateCtx, tea.Msg)           {}

// sidebar is the list down the left. Its rows are surfaces rather than lines in
// one surface, because controls are surfaces — which is what makes a move out of
// the list and back into it a question about geometry rather than about a
// selection index.
type sidebar struct{ bd *board }

func (s *sidebar) Mount(ac *froth.ApplyCtx) {
	self := ac.Self()
	for i, name := range []string{"Inbox", "Starred", "Drafts", "Sent"} {
		self.Create(&panel{name: name, region: regRow, index: i, bd: s.bd})
	}
	self.SetLayout(rows{s: &s.bd.splits})
}

func (s *sidebar) Render(*froth.RenderCtx, froth.Size) string { return "" }
func (s *sidebar) Update(*froth.UpdateCtx, tea.Msg)           {}

// rightCol is two panels at two heights. Two, so a move rightwards out of the
// detail pane has to choose between them rather than having one obvious answer,
// and at different heights so the choice is not a coin toss either.
type rightCol struct{ bd *board }

func (rc *rightCol) Mount(ac *froth.ApplyCtx) {
	self := ac.Self()
	self.Create(&panel{name: "Metadata", region: regRight, index: 0, bd: rc.bd})
	self.Create(&panel{name: "Labels", region: regRight, index: 1, bd: rc.bd})
	self.SetLayout(stack{s: &rc.bd.splits})
}

func (rc *rightCol) Render(*froth.RenderCtx, froth.Size) string { return "" }
func (rc *rightCol) Update(*froth.UpdateCtx, tea.Msg)           {}
