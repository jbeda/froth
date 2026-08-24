package main

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/jbeda/froth"
)

// The actions the board declares. An action is an identity rather than a key:
// the root answers these wherever focus is, and the scope answers its own only
// while focus is inside it.
const (
	actionQuit       froth.Action = "quit"
	actionEnterScope froth.Action = "enter scope"
	actionLeaveScope froth.Action = "leave scope"
	actionArrowMode  froth.Action = "arrow mode"
	actionNarrow     froth.Action = "narrow"
	actionWiden      froth.Action = "widen"
	actionShorter    froth.Action = "shorter"
	actionTaller     froth.Action = "taller"
)

// legend is the detail pane's body. The keys have to be somewhere, and the
// detail pane is the one box with room for them that you are always looking at.
const legend = "tab        next stop\n" +
	"shift+←↑→↓ move by geometry\n" +
	"m          bare arrows\n" +
	"g          scope, in and out\n" +
	"- =        narrow / widen\n" +
	"[ ]        shorter / taller\n" +
	"q          quit"

// root is the surface the application hands froth. It owns the splits every
// layout reads and the mode the arrow keys are in, and it is the only surface
// that changes either.
type root struct {
	bd   *board
	rcol *rightCol
}

func (rt *root) Mount(ac *froth.ApplyCtx) {
	self := ac.Self()
	self.Create(&toolbar{bd: rt.bd})
	self.Create(&sidebar{bd: rt.bd})
	detail := self.Create(&panel{
		name:   "Detail",
		region: regDetail,
		body:   legend,
		bd:     rt.bd,
	}, froth.TabStop())

	rt.rcol = &rightCol{bd: rt.bd, out: detail}
	self.Create(rt.rcol)
	self.Create(&filter{bd: rt.bd}, froth.TabStop(), froth.TextSink())

	// The banner is created last so it renders last, which is what lets it
	// report on the frame the other surfaces have just recorded into. The shell
	// places it along the top regardless.
	self.Create(&banner{bd: rt.bd})

	self.SetLayout(shell{s: &rt.bd.splits})
	ac.At(detail).Focus()
}

// Declaring the four directional actions replaces froth's shift+arrow keys for
// them outright — there is no second override table — so the two modes are two
// answers to one question rather than two sets of keys that both work.
func (rt *root) Bindings() []froth.Binding {
	bs := []froth.Binding{
		{Action: actionQuit, Keys: []string{"q", "ctrl+c"}, Help: "quit", SurvivesModal: true},
		{Action: actionEnterScope, Keys: []string{"g"}, Help: "focus the scope"},
		{Action: actionArrowMode, Keys: []string{"m"}, Help: "swap the arrow keys"},
		{Action: actionNarrow, Keys: []string{"-"}, Help: "narrow what focus is in"},
		{Action: actionWiden, Keys: []string{"="}, Help: "widen what focus is in"},
		{Action: actionShorter, Keys: []string{"["}, Help: "shorten what focus is in"},
		{Action: actionTaller, Keys: []string{"]"}, Help: "heighten what focus is in"},
	}
	if !rt.bd.bare {
		return bs
	}
	return append(bs,
		froth.Binding{Action: froth.ActionFocusLeft, Keys: []string{"left"}, Help: "focus left"},
		froth.Binding{Action: froth.ActionFocusRight, Keys: []string{"right"}, Help: "focus right"},
		froth.Binding{Action: froth.ActionFocusUp, Keys: []string{"up"}, Help: "focus up"},
		froth.Binding{Action: froth.ActionFocusDown, Keys: []string{"down"}, Help: "focus down"},
	)
}

// An action rises from whatever holds focus, so the root sees the ones nothing
// below it took. Taking one means ending the walk: without that it carries on
// past the root to froth, which reports that nobody took it.
func (rt *root) Update(uc *froth.UpdateCtx, msg tea.Msg) {
	m, ok := msg.(froth.ActionMsg)
	if !ok {
		return
	}
	switch m.Action {
	case actionQuit:
		uc.Run(tea.Quit)
	case actionEnterScope:
		uc.At(rt.rcol.scope).Focus()
	case actionArrowMode:
		rt.bd.bare = !rt.bd.bare
	case actionNarrow:
		rt.resize(-1, false)
	case actionWiden:
		rt.resize(1, false)
	case actionShorter:
		rt.resize(-1, true)
	case actionTaller:
		rt.resize(1, true)
	default:
		// froth's own focus actions are among these: the board rebinds them to
		// the bare arrows and leaves them to rise, so froth moves focus.
		return
	}
	uc.StopRising()
}

// resize moves the divider next to whatever holds focus, by one cell. There is
// no divider to select: the one you want is the one beside what you are looking
// at, so the region focus is in decides which of them moves.
//
// Where focus is comes from the board rather than from froth, for the same
// reason the banner's does — froth tells nobody where focus went.
func (rt *root) resize(by int, vertical bool) {
	s := &rt.bd.splits
	f := rt.bd.focus
	switch f.region {
	case regButton:
		if vertical {
			s.toolbarH = clamp(s.toolbarH+by, 3, 8)
		} else if f.index < len(s.buttonW) {
			s.buttonW[f.index] = clamp(s.buttonW[f.index]+by, 6, 40)
		}
	case regRow:
		if !vertical {
			s.sidebarW = clamp(s.sidebarW+by, 8, s.body.W-s.rightW-minDetailW)
			return
		}
		// The divider under the focused row: it grows at the next row's
		// expense, so the sidebar keeps its total height.
		if len(s.rowH) > 1 {
			next := (f.index + 1) % len(s.rowH)
			if s.rowH[f.index]+by >= 3 && s.rowH[next]-by >= 3 {
				s.rowH[f.index] += by
				s.rowH[next] -= by
			}
		}
	case regDetail:
		if vertical {
			s.filterH = clamp(s.filterH+by, 3, 10)
			return
		}
		// The detail pane has no width of its own — it is what the two columns
		// leave — so widening it narrows both of them.
		s.sidebarW = clamp(s.sidebarW-by, 8, s.body.W)
		s.rightW = clamp(s.rightW-by, 14, s.body.W)
	case regRight, regScope:
		if vertical {
			// The divider under whichever panel focus is in — and under the
			// lower one for the scope, which is the divider above it.
			if f.region == regRight && f.index == 0 {
				s.metaH = clamp(s.metaH+by, 3, max(s.body.H-s.labelH-5, 3))
			} else {
				s.labelH = clamp(s.labelH+by, 3, max(s.body.H-s.metaH-5, 3))
			}
		} else {
			s.rightW = clamp(s.rightW+by, 14, s.body.W-s.sidebarW-minDetailW)
		}
	case regFilter:
		if vertical {
			s.filterH = clamp(s.filterH+by, 3, 10)
		}
	}
}

func clamp(n, lo, hi int) int { return min(max(n, lo), max(lo, hi)) }

// Render draws the ground the board sits on: a field of dots, so the space the
// layouts leave between and around the panels is visible rather than black.
// Seeing the gaps is most of seeing why a move went where it did.
func (rt *root) Render(_ *froth.RenderCtx, r froth.Rect) string {
	if r.Empty() {
		return ""
	}
	if tooSmall(r) {
		return lipgloss.NewStyle().
			Foreground(lipgloss.Color("214")).
			Width(r.W).
			Height(r.H).
			Render(fmt.Sprintf("dir-nav needs a window of at least %d×%d; this one is "+
				"%d×%d.\n\nArranging the board any smaller would hand surfaces rects "+
				"with no cells in them, and navigation across those is a layout bug "+
				"rather than anything worth judging.", minW, minH, r.W, r.H))
	}

	row := strings.Repeat("·", r.W)
	rows := make([]string, r.H)
	for i := range rows {
		rows[i] = row
	}
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color("238")).
		Render(strings.Join(rows, "\n"))
}
