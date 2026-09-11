package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/jbeda/froth"
	"github.com/jbeda/froth/producer"
)

// echoMsg is what the focused pane forwards to its sibling. It is not a Bubble
// Tea message: it is a froth message, addressed to one surface and delivered to
// nothing else.
type echoMsg struct{ key string }

// cycleMsg is the deliberate mistake: a pane answers it by posting it back to
// itself, so the cascade never settles.
type cycleMsg struct{}

// noteMsg travels. Embedding froth.Rising means froth offers it to the surface
// it was posted to, then to that surface's parent, and on up — so a control
// deep in the tree says what happened without knowing who cares.
type noteMsg struct {
	froth.Rising
	from string
}

// loadedMsg is what the slow load command produces. It is an ordinary Bubble
// Tea command result; froth brings it back to the pane that ran the command.
type loadedMsg struct{ took time.Duration }

// linesReadyMsg is the wake the log pane names for its producer: it says
// lines are waiting to be pulled, and nothing about which ones.
type linesReadyMsg struct{}

// producerStoppedMsg is what the log producer's goroutine posts to the root
// when its context is cancelled, through a post capability the root handed the
// pane at construction. The pane it fed is gone by then, so the root is who can
// show it. A producer that ends on its own posts nothing: the pane is still
// there to pull the end.
type producerStoppedMsg struct{ err error }

// loadTime is how long the slow command takes: long enough to open the dialog
// over it.
const loadTime = 3 * time.Second

// logKeep is how many lines the log pane keeps to draw, and logEnd is how many
// the producer says before it ends on its own. Lines come in bursts of
// logBurst, a second apart, because a burst is what batching is for: one wake
// for the burst, not one per line.
const (
	logKeep  = 200
	logEnd   = 200
	logBurst = 5
)

// report is where the diagnostic hook leaves what froth said. The hook is
// called with no phase in progress and so has no context to post through; a
// value shared with the surface that draws it is the way onto the screen.
type report struct{ text string }

// pane is a child surface: two methods, and its state on a pointer receiver. It
// fills whatever rect it is given, which is the layout's answer rather than a
// size of its own, and it draws itself differently depending on whether it holds
// focus.
type pane struct {
	title   string
	body    string
	status  string
	sibling froth.Handle
	report  *report // nil for a pane that does not draw diagnostics
	note    *report // nil for a pane that does not draw what rose to the root
	torn    *report // every pane writes here on the way out; the notes pane draws it

	// tabStop and clickFocus are what differs between the panes: the notes
	// pane is out of the keyboard walk and the other two are in it, and all
	// three take focus from a click.
	tabStop    bool
	clickFocus bool

	// sticker, when set, is drawn as a child in the pane's top right corner.
	// The child is created mouse transparent, so a click on it reaches the
	// pane beneath.
	sticker string

	// closing says a confirm dialog has been asked for and not yet opened.
	// Opening needs an Apply, so the key sets this and the reconcile opens.
	closing bool
	// asking is the dialog that is up, so its answer can be told from any
	// other dialog's.
	asking froth.Handle

	// tell, when set, makes this the log pane: it starts a producer in Mount
	// and its goroutine reports through tell when it stops. The producer is
	// held here so the pane can pull from it, and lines is what it has pulled.
	tell  froth.PostFunc
	lines *producer.Producer[[]string]
	log   []string
}

func (p *pane) Declare() froth.Flags {
	return froth.Flags{TabStop: p.tabStop, ClickFocus: p.clickFocus}
}

// Unmount is where a surface releases what froth does not own. This one owns
// nothing, so it only says it ran — which is the point worth seeing: it runs
// when the pane is closed, and again for every pane still standing when the
// program shuts froth down.
func (p *pane) Unmount(*froth.ApplyCtx) {
	if p.torn != nil {
		p.torn.text = "the " + p.title + " pane unmounted"
	}
}

func (p *pane) Render(rc *froth.RenderCtx, s froth.Size) string {
	focused := rc.Focused()

	title := p.title
	border, accent := lipgloss.Color("240"), lipgloss.Color("240")
	if focused {
		title = "▸ " + p.title + " — focused"
		border, accent = lipgloss.Color("212"), lipgloss.Color("214")
	}

	status := p.status
	if status == "" {
		switch {
		case focused:
			status = "nothing yet — press a key, or tab to hand focus over"
		case p.report != nil:
			status = "nothing reported yet — press d"
		default:
			status = "tab moves focus here"
		}
	}
	if p.report != nil && p.report.text != "" {
		status += "\n\n" + p.report.text
	}
	if p.note != nil && p.note.text != "" {
		status += "\n\n" + p.note.text
	}
	// Every pane writes what it says on the way out; the pane that draws
	// diagnostics is the one that draws this too.
	if p.report != nil && p.torn != nil && p.torn.text != "" {
		status += "\n\n" + p.torn.text
	}

	body := p.body
	if p.tell != nil {
		// The newest lines are what fit under the title and status, which
		// take the top six rows with the border: the clip takes the bottom.
		from := max(0, len(p.log)-max(0, s.H-6))
		body = strings.Join(p.log[from:], "\n")
	}

	// The status sits under the title rather than under the body, so it stays
	// on screen in a short terminal: the body is what the clip takes first.
	content := lipgloss.NewStyle().Bold(focused).Render(title) + "\n" +
		lipgloss.NewStyle().Foreground(accent).Render(status) + "\n\n" +
		body

	// Lip Gloss counts the border and padding inside Width and Height, so a box
	// styled to the rect fills it exactly.
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(border).
		Padding(0, 1).
		Width(s.W).
		Height(s.H).
		Render(content)
}

// A surface with no children still has a Mount, because declaring bindings is
// something a surface does about itself. These are the pane's own, so they
// resolve only while this pane is on the focus chain.
func (p *pane) Mount(ac *froth.ApplyCtx) {
	if p.sticker != "" {
		// What the sticker is — a label — is its type's business. That it
		// should not be found by a click is how this pane is using it, so the
		// pane says so here, at the one place it describes the child.
		ac.Self().Create(&label{text: p.sticker}, froth.MouseTransparent())
		ac.Self().SetLayout(topRight{w: lipgloss.Width(p.sticker)})
	}
	ac.Self().SetBindings([]froth.Binding{{
		Action: actionCycle,
		Keys:   []string{"d"},
		Help:   "post a message that comes back",
	}, {
		Action: actionLoad,
		Keys:   []string{"l"},
		Help:   "load something slowly",
	}, {
		Action: actionClose,
		Keys:   []string{"x"},
		Help:   "close this pane",
	}})
	if p.tell != nil {
		// The producer takes this pane's context, so it is cancelled as the
		// pane leaves the tree, and this pane's post capability, so the wake
		// comes here. The step function is a closure over nothing the pane
		// draws: the goroutine never touches surface state.
		p.lines = producer.Collect(ac.Ctx(), ac.PostFunc(), linesReadyMsg{}, tail(p.tell))
	}
}

// tail is the log pane's step function: a burst of lines every second until
// it has said enough, then io.EOF. It blocks in the producer's goroutine and
// returns when the context is cancelled, telling the root so on the way out.
// The post's answer is ignored: it is false only when the program is shutting
// down, and there is nobody left to tell.
func tail(tell froth.PostFunc) func(context.Context) (string, error) {
	n := 0
	return func(ctx context.Context) (string, error) {
		wait := time.Duration(0)
		if n%logBurst == 0 {
			wait = time.Second
		}
		select {
		case <-ctx.Done():
			tell(producerStoppedMsg{err: ctx.Err()})
			return "", ctx.Err()
		case t := <-time.After(wait):
			n++
			if n > logEnd {
				return "", io.EOF
			}
			return fmt.Sprintf("%3d  %s", n, t.Format("15:04:05.000")), nil
		}
	}
}

func (p *pane) Update(uc *froth.UpdateCtx, msg tea.Msg) {
	switch m := msg.(type) {
	case froth.ActionMsg:
		switch m.Action {
		case actionCycle:
			// Answering a message with the same message is the cycle the
			// iteration cap is for.
			uc.Self().Post(cycleMsg{})
			uc.StopRising()
		case actionLoad:
			// The command runs on its own goroutine and its result comes
			// back to this pane, not to the root, however the tree looks by
			// then: open the dialog over it and watch the status change.
			p.status = "loading — press x now and watch this pane under the dialog"
			started := time.Now()
			uc.Run(func() tea.Msg {
				time.Sleep(loadTime)
				return loadedMsg{took: time.Since(started).Round(time.Millisecond)}
			})
			uc.StopRising()
		case actionClose:
			// Closing asks first. Opening a modal is an Apply-tier call, so
			// the key records the wish and Reconcile opens the dialog.
			p.closing = true
			uc.Self().Reconcile()
			uc.StopRising()
		}
	case loadedMsg:
		p.status = "loaded after " + m.took.String()
		if p.asking != (froth.Handle{}) {
			p.status += " — while the dialog was up; it blocks keys, not results"
		}
	case linesReadyMsg:
		// One wake for however many lines arrived since the last pull.
		lines, err := p.lines.Take()
		p.log = append(p.log, lines...)
		if len(p.log) > logKeep {
			p.log = p.log[len(p.log)-logKeep:]
		}
		switch {
		case err == nil:
			p.status = fmt.Sprintf("%d lines so far; the last wake brought %d", len(p.log), len(lines))
		case errors.Is(err, io.EOF):
			p.status = fmt.Sprintf("the producer ended after %d lines; close this pane and the root hears nothing", len(p.log))
		default:
			p.status = "the producer stopped: " + err.Error()
		}
	case confirmResult:
		if m.dialog != p.asking {
			return
		}
		p.asking = froth.Handle{}
		if !m.ok {
			p.status = "kept — the dialog said no, and focus came back here"
			return
		}
		// A surface removes itself. The removal commits at the next Apply,
		// so this pane is still live for the rest of this update. The dialog
		// leaves in the same Apply and hands focus back here first; then this
		// pane goes, and froth rehomes focus rather than leaving the keyboard
		// pointing at nothing.
		uc.Self().Destroy()
	case tea.KeyMsg:
		key := m.String()
		p.status = "I had focus, so froth gave me " + strconv.Quote(key)
		uc.At(p.sibling).Post(echoMsg{key: key})
	case tea.MouseClickMsg:
		// A click arrives at whatever was drawn under it, focused or not; it
		// moved focus here only because this pane's type admits it.
		p.status = "a click landed on me at " + froth.Pt(m.X, m.Y).String()
	case echoMsg:
		p.status = "the focused pane passed on " + strconv.Quote(m.key) +
			", same frame as the keystroke"
	case cycleMsg:
		uc.Self().Post(m)
	}
}

// Reconcile opens the dialog the key asked for. The dialog is parented to the
// root and opened by this pane: the root decides what it blocks, and this pane
// decides how long it lives — close the pane and the dialog goes with it.
func (p *pane) Reconcile(ac *froth.ApplyCtx) {
	if !p.closing {
		return
	}
	p.closing = false
	p.asking = ac.Self().OpenModal(newConfirmDialog(&confirmSpec{
		title:   "Close the " + p.title + " pane?",
		body:    "Its Unmount runs on the way out, and focus goes to the nearest surface left standing.",
		yes:     "Close",
		no:      "Keep",
		replyTo: ac.Self().Handle(),
	}), froth.Float())
}

// label is a one-line surface with nothing to say about input.
type label struct{ text string }

func (l *label) Render(*froth.RenderCtx, froth.Size) string {
	return lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Render(l.text)
}

func (l *label) Update(*froth.UpdateCtx, tea.Msg) {}

// topRight puts each of a pane's children, w cells wide, inside its top right
// corner, just in from the border.
type topRight struct{ w int }

func (l topRight) Arrange(space froth.Rect, children []froth.Child) []froth.Rect {
	out := make([]froth.Rect, len(children))
	for i := range out {
		w := min(l.w, max(0, space.W-4))
		out[i] = froth.Rect{X: space.X + space.W - 2 - w, Y: space.Y + 1, W: w, H: 1}
	}
	return out
}
