package main

import (
	"slices"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/jbeda/froth"
	"github.com/jbeda/froth/layout"
)

// stickerText is what the notes pane's corner child says.
const stickerText = "click me"

// root is the surface the application hands froth. It creates the panes it is
// made of, says how they sit, and says which of them starts focused — all in
// Mount, because a surface owns its children and all three are its business and
// nobody else's.
type root struct {
	report *report
	note   *report
	torn   *report
	scope  froth.Handle

	// wantPalette says the key was pressed and the palette not yet created.
	// Creating needs an Apply, so the key sets this and Reconcile creates.
	wantPalette bool
	palette     froth.Handle
}

func (rt *root) Mount(ac *froth.ApplyCtx) {
	ac.Self().SetEnv(paneThemeKey, paneTheme{
		muted:       lipgloss.Color("240"),
		focusBorder: lipgloss.Color("212"),
		focusAccent: lipgloss.Color("214"),
	})

	// Quit belongs to the whole application, so the root declares it and the
	// panes know nothing about it. These never change, so declaring them once
	// on the way in is the whole of it.
	ac.Self().SetBindings([]froth.Binding{{
		Action:        actionQuit,
		Keys:          []string{"q", "ctrl+c"},
		Help:          "quit",
		SurvivesModal: true,
	}, {
		Action: actionEnterScope,
		Keys:   []string{"g"},
		Help:   "focus the scope",
	}, {
		Action: actionPalette,
		Keys:   []string{":", "ctrl+p"},
		Help:   "open the palette",
	}})

	left := &pane{
		title:      "left",
		tabStop:    true,
		clickFocus: true,
		torn:       rt.torn,
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
		title:      "right",
		tabStop:    true,
		clickFocus: true,
		torn:       rt.torn,
		body: "froth: focus is one handle, raw keys go to it, and froth " +
			"walks it.\n\n" +
			"tab and shift-tab are froth's own bindings. The action rises " +
			"from whatever holds focus, nothing takes it, and froth moves " +
			"focus at the end of the walk — so no pane implements it, and " +
			"any surface on the way could have pre-empted froth simply by " +
			"taking the message. The pane that sees a key posts it to its " +
			"sibling, and the frame is " +
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
		title:      "notes",
		clickFocus: true,
		sticker:    stickerText,
		report:     rt.report,
		note:       rt.note,
		torn:       rt.torn,
		body: "This pane declares no tab stop, so tab never " +
			"lands here — but it still sits third in the row, still gets its " +
			"share of the space, and could still be given focus by name. " +
			"Being outside the keyboard walk is not being moved and not " +
			"being hidden. It does admit a click: click here and focus " +
			"arrives, and the sticker in the corner is mouse transparent, so " +
			"a click on it lands on this pane rather than on the sticker.\n\n" +
			"Whatever froth reports through the diagnostic hook shows up " +
			"under the title above, and so does whatever rose all the way to " +
			"the root.",
	}

	logs := &pane{
		title:      "log",
		tabStop:    true,
		clickFocus: true,
		torn:       rt.torn,
		// The root's own post capability, so the producer's goroutine can
		// reach the root after the pane it fed is gone. Handed over here, at
		// the one place the root describes the pane.
		tell: ac.PostFunc(),
	}

	self := ac.Self()
	self.Create(&filter{})
	self.Create(helpBar{})
	leftH, rightH := self.Create(left), self.Create(right)
	self.Create(notes)
	self.Create(logs)

	// The scope declares no tab stop, so tab neither lands on it nor walks
	// into it. It is reached by name, which is what the root's g binding does —
	// and the same key inside the scope reaches the scope first and takes focus
	// back out.
	rt.scope = self.Create(&scopePane{out: leftH}, froth.FocusScope())

	left.sibling, right.sibling = rightH, leftH
	self.SetLayout(fieldAndBar{rest: layout.Row(layout.Gap(1))})
	ac.At(leftH).Focus()
}

func (rt *root) Render(rc *froth.RenderCtx, s froth.Size) string {
	row := strings.Repeat("·", s.W)
	rows := make([]string, s.H)
	for i := range rows {
		rows[i] = row
	}
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color("240")).
		Render(strings.Join(rows, "\n"))
}

// The root sees no raw keys — those go to the focused pane. What reaches it is
// what rose: an action starts at whatever holds focus and climbs, whoever
// declared it, and so does a message a control posted. Both stop here only
// because this surface says so; anything it does not take carries on to froth.
func (rt *root) Update(uc *froth.UpdateCtx, msg tea.Msg) {
	switch m := msg.(type) {
	case froth.ActionMsg:
		switch m.Action {
		case actionQuit:
			uc.Run(tea.Quit)
		case actionEnterScope:
			// Moving focus to a scope enters it: froth lands on the control the
			// scope last had focus on, or its first one.
			uc.At(rt.scope).Focus()
		case actionPalette:
			rt.wantPalette = true
			uc.Self().Reconcile()
		default:
			// Anything else keeps rising, which is how tab reaches froth.
			return
		}
		uc.StopRising()
	case noteMsg:
		// The walk ends here rather than at the tail. Saying nothing would let
		// it past the root to the instance, where froth would report that
		// nobody took it.
		rt.note.text = "rose from " + m.from + " and the root took it"
		uc.StopRising()
	case producerStoppedMsg:
		// Posted from the log producer's goroutine as it ended, after the
		// pane that started it had left the tree.
		rt.note.text = "the log producer stopped: " + m.err.Error()
	}
}

// Reconcile creates the palette the key asked for, as a float of the root's
// own: it is placed against the screen, not in the row, and it lives as long
// as the root does or until it takes itself down. The palette takes itself
// down without telling the root, so whether the handle still names a child is
// read back rather than remembered.
func (rt *root) Reconcile(ac *froth.ApplyCtx) {
	if !rt.wantPalette {
		return
	}
	rt.wantPalette = false
	if slices.Contains(ac.Self().FloatChildren(), rt.palette) {
		ac.Child(rt.palette).Focus()
		return
	}
	rt.palette = ac.Self().Create(newPalette(ac.Query().Focus()), froth.Float())
	ac.Child(rt.palette).Focus()
}

// onDiagnostic is froth's one hook, registered at construction. It is ordinary
// application code called with no phase in progress, so it is free to touch the
// application's own surfaces — which is all this one does, putting what froth
// reported where the next frame will draw it.
func (rt *root) onDiagnostic(d froth.Diagnostic) {
	rt.report.text = d.String()
}

// fieldAndBar gives the first child a strip along the top of the space, the
// second a single row along the bottom, and hands what is left to another
// layout. A layout is an interface the application may implement, and this one
// is the application's.
type fieldAndBar struct{ rest froth.Layout }

func (l fieldAndBar) Arrange(space froth.Rect, children []froth.Child) []froth.Rect {
	if len(children) < 2 {
		return make([]froth.Rect, len(children))
	}
	top := froth.Rect{X: space.X, Y: space.Y, W: space.W, H: min(3, space.H)}
	bar := froth.Rect{X: space.X, Y: space.Y + space.H - 1, W: space.W, H: 1}
	middle := froth.Rect{
		X: space.X,
		Y: space.Y + top.H,
		W: space.W,
		H: space.H - top.H - bar.H,
	}
	return append([]froth.Rect{top, bar}, l.rest.Arrange(middle, children[2:])...)
}
