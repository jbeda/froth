// Command panes is froth's running example: a root surface that assembles a
// filter field and three panes, places the field along the top and the panes
// side by side under it, and hands keys to whichever of them holds focus.
//
// The root draws a field of dots behind them, visible in the one-cell gap the
// layout leaves between the panes.
//
// Focus is one handle held by froth. Tab moves it between the surfaces whose
// types declare TabStop; the notes pane declares none, so the walk passes over
// it while it keeps its place in the row and its share of the space. No
// surface implements traversal and none binds tab: tab and shift-tab are froth's
// own bindings, and an application rebinds or disables them by declaring them
// like any other. froth is the last stop on an action's walk rather than the
// first, so a surface that wanted tab for something else could also simply take
// the message and never let it get there.
//
// The scope on the right is a focus scope. Tab neither lands on it nor walks
// into it, and tab inside it reaches nothing outside: g moves focus in, and the
// same key inside reaches the scope first — it is deeper on the chain than the
// root — and moves focus back out. Come back later and focus lands on the
// control it was on, because a surface remembers which child was last active.
// Its border lights while focus is inside it: the scope is not the focused
// surface, and asking whether it is would say no, so it asks the other
// question — whether focus is at or below it.
//
// shift with an arrow key moves focus geometrically, against where the surfaces
// actually ended up: shift+left and shift+right between the panes, shift+up from
// a pane to the filter field, shift+down back. No layout implements any of it,
// and a direction with nothing that way leaves focus alone rather than
// complaining.
//
// The modifier is there because bare arrows belong to whatever holds focus — a
// list's selection, a table's row. A TUI that wants them for focus instead
// declares froth's actions with the keys it wants, which takes them over
// entirely:
//
//	func (rt *root) Mount(ac *froth.ApplyCtx) {
//		ac.Self().SetBindings([]froth.Binding{
//			{Action: froth.ActionFocusLeft, Keys: []string{"left"}, Help: "focus left"},
//			{Action: froth.ActionFocusRight, Keys: []string{"right"}, Help: "focus right"},
//		})
//	}
//
// One keystroke shows what the update loop buys: the focused pane posts what it
// saw to its sibling, and both have handled it before anything is drawn.
// Pressing "d" posts a message that comes straight back, which is a cycle;
// froth abandons it, reports it through the diagnostic hook, and the program
// carries on.
//
// Quit is a binding declared by the root, so it fires while a pane holds focus.
// Declaring it is not how the root receives it: matching the key answers which
// action it means here and nothing else, and the action then starts at whatever
// holds focus and climbs to the root, which takes it. Anything on the way could
// have taken it first.
//
// The filter field along the top declares itself a text sink, and that is what
// the two halves of that quit binding are for. While it holds focus, "q"
// is ordinary typing and lands in the field; "ctrl+c" is a chord, so it still
// resolves to the root and quits. Tab it away and "q" is a shortcut again.
//
// The field is also the smallest command palette there is. Type into it and it
// matches what froth says the focus chain currently offers — every action, not
// every key — and enter asks for the first match by identity, with no key
// pressed at all. Type "quit" and press enter. That listing is the same
// declarations the bar at the bottom reads, asked a different question: it keeps
// an action nobody bound a key to, and it does not change when a text sink takes
// focus, which matters because the palette's own field is one.
//
// Pressing "n" inside the scope shows the other kind of delivery. The item posts
// a message whose type embeds froth.Rising, so it does not stop where it was
// posted: the scope sees it and counts it without ending the walk, and the root
// sees it next and takes it by calling StopRising. Nothing above the root hears
// about it. A message that nobody took would reach the tail and be reported
// through the same hook the cycle uses — which is what a forgotten case looks
// like from the outside.
//
// Pressing "x" closes the focused pane. The pane removes itself and nothing
// else is involved: no parent is told, no list is edited, and the layout gives
// the space to whatever is left. Focus was on the pane that just left, so froth
// puts it on the nearest surface that survives rather than sending the keyboard
// back to the root — close the left pane and the right one has focus. The pane's
// Unmount runs on the way out and says so in the notes pane, and the same walk
// runs for every surface still standing when the program quits.
//
// The scope's list is where structure stops being fixed. Inside it, + adds an
// item and - removes the one holding focus, and neither key edits the tree: the
// handler changes two ordinary fields — the labels the scope should be showing
// and which of them should hold focus — and asks for a reconcile. froth calls
// Reconcile during the Apply that follows, with a real Apply context, and the
// scope makes the tree match what those fields now say. Nothing keeps a list of
// pending additions and nothing flags an item as on its way out, because the
// desired state is the memory. Press + twice before froth gets to Apply and it
// reconciles once, against a list that grew by two. The item it creates is
// mounted and focused inside that same Apply, so the very next frame shows it
// with the keyboard on it.
//
// Removing shows who decides where focus goes. froth rehomes focus when the
// surface holding it is destroyed — nearest surviving ancestor, then down — but
// that is a repair for what nobody decided, not a preference. The scope decides:
// before dropping the label it names the item above, or the one below if the
// first went, and its reconcile puts focus there. A reconcile runs after Apply's
// focus step and focus during Apply is immediate, so the surface that knows the
// list has the last word.
//
// An item sends two rising messages and they stop in different places, which is
// the other half of the lesson. "t" posts one the scope takes; "n" posts one
// that passes the scope — which counts it without ending the walk — and the root
// takes, appearing in the notes pane. Everything a modal contains is parented to
// the root, so the root sees every rising message from inside every dialog in
// the program; behaviour that is not genuinely global belongs on a surface that
// strangers' chains do not pass through.
//
// The bar along the bottom is the same story told forwards. It is built entirely
// from a query — it asks froth what a key press would currently do and prints the
// answer — so it knows no keys and no actions of its own and cannot describe
// anything other than what happens. Watch it as focus moves: "q/ctrl+c quit"
// loses the "q" the moment the filter field takes focus, because there it is
// typing, and the field's own "ctrl+u clear the filter" appears greyed until
// there is something to clear. Nothing in the bar knows what a text sink is or
// when clearing applies.
package main

import (
	"context"
	"fmt"
	"log"

	tea "charm.land/bubbletea/v2"
	"github.com/jbeda/froth"
)

// The actions the application declares. An action is an identity rather than a
// key: the root answers actionQuit wherever focus is, and the panes answer
// actionCycle only while they hold it.
const (
	actionQuit       froth.Action = "quit"
	actionCycle      froth.Action = "cycle"
	actionEnterScope froth.Action = "enter scope"
	actionLeaveScope froth.Action = "leave scope"
	actionClear      froth.Action = "clear filter"
	actionNote       froth.Action = "note this"
	actionClose      froth.Action = "close this pane"
	actionAddItem    froth.Action = "add an item"
	actionRemoveItem froth.Action = "remove the focused item"
	actionTally      froth.Action = "note this, for the scope"
)

// model is the application root. It owns froth by pointer and forwards Bubble
// Tea's three methods; the value copy on every update is free.
type model struct{ f *froth.Froth }

func (m model) Init() tea.Cmd { return m.f.Init() }

func (m model) View() tea.View {
	// Program-wide terminal settings are the application's, not froth's: only
	// the root model's View reaches the renderer.
	v := m.f.View()
	v.AltScreen = true
	return v
}
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	return m, m.f.Update(msg)
}

func main() {
	rt := &root{report: &report{}, note: &report{}, torn: &report{}}
	m := model{f: froth.New(context.Background(), rt, froth.OnDiagnostic(rt.onDiagnostic))}

	_, err := tea.NewProgram(m).Run()

	// Shutting froth down runs the same leaf-up unmount walk a destroy runs,
	// once from the root, so a surface holding something froth does not own
	// gets to release it when the program quits rather than only when it is
	// closed. Nothing here holds anything, and it still has to be called: the
	// walk is the guarantee, and an application that only calls it sometimes
	// does not have one.
	m.f.Shutdown()

	if err != nil {
		log.Fatal(fmt.Errorf("running panes: %w", err))
	}
}
