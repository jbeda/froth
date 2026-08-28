package main

import (
	"slices"
	"strconv"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/jbeda/froth"
)

// tallyMsg travels the way noteMsg does and stops somewhere else. The scope
// takes it; the root never sees one.
type tallyMsg struct {
	froth.Rising
	from string
}

// scopePane is a focus scope holding a list that grows and shrinks while the
// program runs. Tab inside it moves between its own controls and never reaches
// the panes beside it, and the walk out there never reaches the controls; focus
// crosses the boundary only by being moved, which is what the binding below
// does — and what a dialog's open and close would do.
//
// Its children are a function of two fields: the labels it should be showing
// and which of them should hold focus. Reconcile is the one place the tree is
// made to match them. Nothing here records what has been asked for — no list of
// pending additions, no flag saying a control is on its way out — because the
// desired state is the memory. Press + twice before froth reaches Apply and it
// reconciles once, against a list that grew by two.
type scopePane struct {
	out   froth.Handle
	notes int
	tally int
	// tallied is the control the last tally came from, which is what the
	// message carried: the scope is told who, not merely that.
	tallied string

	labels []string
	// live maps a label to the surface showing it, which is what makes the
	// difference between wanted and present computable without walking the
	// tree. It is the scope's own bookkeeping and nothing reads it but match.
	live map[string]froth.Handle
	// focused is the label that should hold focus once the tree matches.
	focused string
	// minted counts the labels ever handed out, so removing a control and
	// adding another does not reuse a name and hide a stale handle.
	minted int
}

func (g *scopePane) Mount(ac *froth.ApplyCtx) {
	g.live = map[string]froth.Handle{}
	ac.Self().SetLayout(stack{})
	// The scope is on the focus chain while one of its controls has focus, and
	// it is deeper than the root, so its g wins over the root's while focus is
	// inside. The same is what makes + and - the scope's rather than anybody
	// else's.
	ac.Self().SetBindings([]froth.Binding{{
		Action: actionLeaveScope,
		Keys:   []string{"g"},
		Help:   "leave the scope",
	}, {
		Action: actionAddItem,
		Keys:   []string{"+", "="},
		Help:   "add an item",
	}, {
		Action: actionRemoveItem,
		Keys:   []string{"-"},
		Help:   "remove the focused item",
	}})
	g.add()
	g.add()
	// The scope has an opinion about focus only once somebody has asked it for
	// a control. On the way in it has none: the root said where focus starts,
	// and a surface that grabbed it while mounting would overrule whoever built
	// it, since children mount after their parent.
	g.focused = ""
	g.match(ac)
}

// Reconcile is Mount's as-needed half. The two share match and stay separate
// methods, because Mount also does setup that is not structural — the layout
// here, and a producer or an open file in a real surface — and running that
// again would do it twice.
func (g *scopePane) Reconcile(ac *froth.ApplyCtx) {
	g.match(ac)
}

// match is the whole idea: destroy what is present and unwanted, create what is
// wanted and absent, and put focus where the scope says it belongs. It is
// written against the fields rather than against what just happened, so it is
// the same code whether one control changed or five did.
func (g *scopePane) match(ac *froth.ApplyCtx) {
	for label, h := range g.live {
		if !slices.Contains(g.labels, label) {
			ac.Child(h).Destroy()
			delete(g.live, label)
		}
	}
	for _, label := range g.labels {
		if _, ok := g.live[label]; !ok {
			g.live[label] = ac.Self().Create(&control{label: label})
		}
	}
	// froth rehomes focus when the surface holding it is destroyed, by its own
	// rule: nearest surviving ancestor, then down. That rule is a repair, not a
	// preference, and a surface that knows better says so — here. Focus during
	// Apply is immediate and a reconcile runs after Apply's focus step, so what
	// this says last is what stands.
	if h, ok := g.live[g.focused]; ok {
		ac.Child(h).Focus()
	}
}

func (g *scopePane) add() {
	g.minted++
	label := "item " + strconv.Itoa(g.minted)
	g.labels = append(g.labels, label)
	g.focused = label
}

// removeFocused takes out the control that currently holds the keyboard and
// names its successor: the one above it, or the one below if it was first.
// Naming it is the whole answer to "where does focus go" — froth's own rehoming
// only decides what nobody decided.
func (g *scopePane) removeFocused(uc *froth.UpdateCtx) {
	held := uc.Query().Focus()
	i := slices.IndexFunc(g.labels, func(l string) bool { return g.live[l] == held })
	if i < 0 {
		// Focus is on the scope itself rather than on one of its controls,
		// which happens once the list is empty. There is nothing to remove.
		return
	}
	switch {
	case i > 0:
		g.focused = g.labels[i-1]
	case len(g.labels) > 1:
		g.focused = g.labels[1]
	default:
		g.focused = ""
	}
	g.labels = slices.Delete(g.labels, i, i+1)
}

func (g *scopePane) Update(uc *froth.UpdateCtx, msg tea.Msg) {
	switch m := msg.(type) {
	case froth.ActionMsg:
		switch m.Action {
		case actionLeaveScope:
			uc.At(g.out).Focus()
		case actionAddItem:
			g.add()
			// Deliver may not restructure anything, so the handler says what
			// the scope should look like and asks for the phase that can. The
			// call is idempotent: two presses in one update reconcile once.
			uc.Self().Reconcile()
		case actionRemoveItem:
			g.removeFocused(uc)
			uc.Self().Reconcile()
		default:
			return
		}
		uc.StopRising()

	case tallyMsg:
		// Taken here rather than at the root, and that placement is the point.
		// Everything a modal contains is parented to the root, so the root sees
		// every rising message from inside every dialog in the program; put a
		// case for this at the top and the root would count notes from surfaces
		// that have nothing to do with this list. Behaviour that is not
		// genuinely global belongs on a surface strangers' chains do not pass
		// through.
		g.tally++
		g.tallied = m.from
		uc.StopRising()

	case noteMsg:
		// Acting on a rising message is not taking it: the walk ends only where
		// something calls StopRising, and this surface deliberately does not.
		g.notes++
	}
}

// The scope is never the focused surface while one of its controls is — but it
// is on the chain above it, which is what FocusWithin asks and Focused cannot.
func (g *scopePane) Render(rc *froth.RenderCtx, s froth.Size) string {
	border := lipgloss.Color("240")
	if rc.FocusWithin() {
		border = lipgloss.Color("212")
	}
	body := "scope\n\ng gets in and back out; tab cycles between the items " +
		"below and reaches nothing else. Come back and focus is where you " +
		"left it.\n\n+ adds an item and - removes the focused one, both " +
		"through Reconcile — and the scope names which item takes focus " +
		"next rather than letting the removal decide.\n\nn from an item " +
		"sends a message up past me: counted " + strconv.Itoa(g.notes) +
		", taken by the root. t sends one I take instead: counted " +
		strconv.Itoa(g.tally) + "."
	if g.tallied != "" {
		body += " Last from " + g.tallied + "."
	}
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(border).
		Padding(0, 1).
		Width(s.W).
		Height(s.H).
		Render(body)
}

// stack gives each child a row of the space, bottom-aligned, so the list grows
// upward as items are added and the text above it keeps its place.
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
// focus. It is a tab stop, so the scope's walk lands on it. It never removes
// itself: the scope owns which items exist, and a child that destroyed itself
// would leave the scope's idea of the world wrong until something corrected it.
type control struct{ label string }

// The control says what happened and does not know who acts on it. Both keys
// post a message that travels; the surfaces above decide where each one stops.
func (c *control) Mount(ac *froth.ApplyCtx) {
	ac.Self().SetBindings([]froth.Binding{{
		Action: actionNote,
		Keys:   []string{"n"},
		Help:   "send a note to the root",
	}, {
		Action: actionTally,
		Keys:   []string{"t"},
		Help:   "send a note the scope takes",
	}})
}

func (c *control) Render(rc *froth.RenderCtx, s froth.Size) string {
	if rc.Focused() {
		return lipgloss.NewStyle().
			Foreground(lipgloss.Color("214")).
			Bold(true).
			Render("▸ " + c.label)
	}
	return lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render("  " + c.label)
}

func (c *control) Declare() froth.Flags { return froth.Flags{TabStop: true} }

func (c *control) Update(uc *froth.UpdateCtx, msg tea.Msg) {
	m, ok := msg.(froth.ActionMsg)
	if !ok {
		return
	}
	switch m.Action {
	case actionNote:
		uc.Self().Post(noteMsg{from: c.label})
	case actionTally:
		uc.Self().Post(tallyMsg{from: c.label})
	default:
		return
	}
	uc.StopRising()
}
