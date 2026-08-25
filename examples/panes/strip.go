package main

import (
	"slices"
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/jbeda/froth"
)

// tallyMsg travels the way noteMsg does, and stops somewhere else. The strip
// takes it; the root never sees one.
type tallyMsg struct {
	froth.Rising
	from string
}

// strip is a tab strip whose panes come and go while the program runs. Its
// children are a function of two fields — the titles it should be showing and
// which of them should hold focus — and Reconcile is the one place the tree is
// made to match them.
//
// Nothing here records what has been asked for. There is no list of pending
// additions, no flag saying a pane is on its way out, and no ordering between
// the two: pressing + twice before froth gets to Apply reconciles once, against
// a titles slice that has grown by two. The desired state is the memory.
type strip struct {
	titles []string
	// live maps a title to the surface showing it, which is what makes the
	// difference between wanted and present computable without walking the
	// tree. It is the strip's own bookkeeping and nothing reads it but match.
	live map[string]froth.Handle
	// focused is the title that should hold focus after the tree matches.
	focused string
	// minted counts the titles ever handed out, so closing a pane and opening
	// another does not reuse a name and hide a stale handle.
	minted int
	tally  int
	// tallied is the pane the last note came from, which is what the message
	// carried: the strip is told who, not merely that.
	tallied string
}

func (s *strip) Mount(ac *froth.ApplyCtx) {
	s.live = map[string]froth.Handle{}
	ac.Self().SetLayout(stripRows{})
	s.add()
	s.add()
	// The strip has an opinion about focus only once somebody has asked it for
	// a pane. On the way in it has none: the root said where focus starts, and
	// a surface that grabbed it while mounting would overrule whoever built it.
	s.focused = ""
	s.match(ac)
}

// Reconcile is Mount's as-needed half. The two share match and stay separate
// methods, because Mount also does setup that is not structural — the layout
// here, and a producer or an open file in a real surface — and running that
// again would do it twice.
func (s *strip) Reconcile(ac *froth.ApplyCtx) {
	s.match(ac)
}

// match is the whole idea: destroy what is present and unwanted, create what is
// wanted and absent, and put focus where the strip says it belongs. It is
// written against the fields rather than against what just happened, so it is
// the same code whether one pane changed or five did.
func (s *strip) match(ac *froth.ApplyCtx) {
	for title, h := range s.live {
		if !slices.Contains(s.titles, title) {
			ac.Child(h).Destroy()
			delete(s.live, title)
		}
	}
	for _, title := range s.titles {
		if _, ok := s.live[title]; !ok {
			s.live[title] = ac.Self().Create(&stripPane{title: title}, froth.TabStop())
		}
	}
	// Focus during Apply is immediate, so a pane created a line ago can be
	// focused here: it is in the tree, and being mounted is a later step.
	if h, ok := s.live[s.focused]; ok {
		ac.Child(h).Focus()
	}
}

func (s *strip) add() {
	s.minted++
	title := "pane " + strconv.Itoa(s.minted)
	s.titles = append(s.titles, title)
	s.focused = title
}

func (s *strip) remove() {
	if len(s.titles) == 0 {
		return
	}
	s.titles = s.titles[:len(s.titles)-1]
	s.focused = ""
	if len(s.titles) > 0 {
		s.focused = s.titles[len(s.titles)-1]
	}
}

// The strip is on the focus chain whenever one of its panes has focus, so these
// resolve there and nowhere else.
func (s *strip) Bindings() []froth.Binding {
	return []froth.Binding{{
		Action: actionAddPane,
		Keys:   []string{"+", "="},
		Help:   "add a pane",
	}, {
		Action: actionRemovePane,
		Keys:   []string{"-"},
		Help:   "remove the last pane",
	}}
}

func (s *strip) Update(uc *froth.UpdateCtx, msg tea.Msg) {
	switch m := msg.(type) {
	case froth.ActionMsg:
		switch m.Action {
		case actionAddPane:
			s.add()
		case actionRemovePane:
			s.remove()
		default:
			return
		}
		// Deliver may not restructure anything, so the handler says what the
		// strip should look like and asks for the phase that can. The call is
		// idempotent: two presses in one update still reconcile once.
		uc.Self().Reconcile()
		uc.StopRising()

	case tallyMsg:
		// Taken here rather than at the root, and that placement is the point.
		// Everything a modal contains is parented to the root, so the root sees
		// every rising message from inside every dialog in the program — put a
		// case for this here and the root would count notes from surfaces that
		// have nothing to do with the strip. Behaviour that is not genuinely
		// global belongs on a surface the strangers' chains do not pass
		// through.
		s.tally++
		s.tallied = m.from
		uc.StopRising()
	}
}

func (s *strip) Render(rc *froth.RenderCtx, r froth.Rect) string {
	border := lipgloss.Color("240")
	if rc.FocusWithin() {
		border = lipgloss.Color("212")
	}
	body := "tab strip\n\n+ adds a pane and - takes the last one away, both " +
		"through Reconcile. t inside a pane sends a note that stops here " +
		"rather than at the root, and r sends one that does: counted " +
		strconv.Itoa(s.tally) + "."
	if s.tallied != "" {
		body += " Last from " + s.tallied + "."
	}
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(border).
		Padding(0, 1).
		Width(r.W).
		Height(r.H).
		Render(body)
}

// stripRows lists the panes down the lower half of the strip's own box, one row
// each, leaving the rows above for the text. The list grows and shrinks as the
// panes do, and no pane is told where it is.
type stripRows struct{}

func (stripRows) Arrange(space froth.Rect, children []froth.Child) []froth.Rect {
	out := make([]froth.Rect, len(children))
	y := space.Y + max(space.H-len(children)-1, 0)
	for i := range out {
		out[i] = froth.Rect{X: space.X + 1, Y: y + i, W: max(space.W-2, 0), H: 1}
	}
	return out
}

// stripPane is one of the strip's children: a tab stop that says whether it has
// focus and can send a note upward. It never removes itself — the strip owns
// which panes exist, and a child that destroyed itself would leave the strip's
// idea of the world wrong until something else corrected it.
type stripPane struct{ title string }

func (p *stripPane) Bindings() []froth.Binding {
	return []froth.Binding{{
		Action: actionTally,
		Keys:   []string{"t"},
		Help:   "tally this pane",
	}, {
		// The same message the scope's controls send, from a surface with a
		// different chain above it. It passes the strip untouched, because the
		// strip has no case for it, and the root takes it.
		Action: actionNote,
		Keys:   []string{"r"},
		Help:   "send a note to the root",
	}}
}

func (p *stripPane) Update(uc *froth.UpdateCtx, msg tea.Msg) {
	m, ok := msg.(froth.ActionMsg)
	if !ok {
		return
	}
	switch m.Action {
	case actionTally:
		uc.Self().Post(tallyMsg{from: p.title})
	case actionNote:
		uc.Self().Post(noteMsg{from: p.title})
	default:
		return
	}
	uc.StopRising()
}

func (p *stripPane) Render(rc *froth.RenderCtx, r froth.Rect) string {
	label := p.title
	style := lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	if rc.Focused() {
		label = "▸ " + label
		style = lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Bold(true)
	}
	if len(label) > r.W {
		label = label[:max(r.W, 0)]
	}
	return style.Render(label + strings.Repeat(" ", max(r.W-len(label), 0)))
}
