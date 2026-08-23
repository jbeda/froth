// Update, Deliver and Apply: how one message becomes a settled tree.
//
// Everything here is the alternation ADR 0002 describes — Apply, then Deliver,
// repeating until nothing is left to deliver — and the budget that stops it
// running forever. This file never draws; rendering is render.go.

package froth

import (
	"strconv"

	tea "charm.land/bubbletea/v2"
)

// An envelope is a message and the surface it is addressed to. Bubble Tea's own
// messages arrive with no destination, so froth supplies one; a posted message
// carries the one its sender named.
type envelope struct {
	to  Handle
	msg tea.Msg
}

// route addresses a message arriving from Bubble Tea, and is where a key press
// stops being a key. It reports false when there is nothing to deliver, which is
// a press froth answered itself.
//
// A press that matches a binding on the focus chain becomes an ActionMsg
// addressed to the surface that declared it, which is not necessarily the
// focused one — unless the action is one froth services, which froth performs
// here and delivers nowhere. A press that matches nothing, and every other key
// event, goes to the focused surface as it stands. Everything else goes to the
// root, which is the surface the application handed froth and so the one its own
// messages belong at.
//
// Only a press resolves. A release is the same key arriving a second time, so
// matching it would fire every action twice.
func (f *Froth) route(msg tea.Msg) (envelope, bool) {
	if k, ok := msg.(tea.KeyPressMsg); ok {
		if to, a, ok := f.resolve(k); ok {
			if f.service(a) {
				return envelope{}, false
			}
			return envelope{to: to, msg: ActionMsg{Action: a}}, true
		}
	}
	if _, ok := msg.(tea.KeyMsg); ok {
		return envelope{to: f.focused(), msg: msg}, true
	}
	return envelope{to: f.root, msg: msg}, true
}

// A budget bounds one settle pass. Delivering a message and mounting a surface
// each spend from the same one, because a cascade that will not settle can be
// made of either and the two interleave: a delivery can create a surface whose
// mount posts a message.
type budget struct {
	left    int
	tripped bool
}

// iterationCap is how much one settle pass may spend before froth decides the
// cascade is never going to settle.
//
// The cap exists to turn a hang into a report, not to police enthusiasm, so it
// is set well above anything legitimate rather than close to it. Tripping late
// costs microseconds — an iteration is a map lookup and one surface's handler —
// while tripping early breaks an application that was merely large: controls are
// surfaces, so a screen assembling a few thousand of them in one mount pass is
// ordinary rather than pathological. A real cycle spends the whole budget in
// well under a millisecond, which is invisible next to a frame.
const iterationCap = 10000

// spend takes one unit, reporting whether there was one to take.
func (b *budget) spend() bool {
	if b.left <= 0 {
		return false
	}
	b.left--
	return true
}

// trip reports an exhausted budget, once per settle pass. A cascade that runs
// out during Apply would otherwise report again the moment the drain loop
// noticed the queue it abandoned.
func (f *Froth) trip(b *budget, detail string) {
	if b.tripped {
		return
	}
	b.tripped = true
	f.report(Diagnostic{Kind: DiagIterationCap, Detail: detail})
}

// settle runs Deliver and Apply alternately until the message queue drains.
//
// Apply leads because the tree has to exist before anything can be delivered
// into it — the root's own mount is the first Apply any instance runs — and
// because the Apply a delivery asks for is the one at the top of the next turn
// of the loop. Deliver hands one message to one surface, which may change only
// itself, and Apply commits the structural change that delivery asked for.
//
// Delivering a message and mounting a surface both spend from one budget, and
// running out of it means a cascade that is not going to settle. froth abandons
// what it was draining, reports it, and lets the program carry on rather than
// hanging. See iterationCap for why the budget is the size it is.
func (f *Froth) settle() tea.Cmd {
	b := budget{left: iterationCap}
	var cmds []tea.Cmd

	for {
		f.apply(&b)
		if len(f.queue) == 0 {
			break
		}
		if !b.spend() {
			f.trip(&b, "the message queue did not drain within "+
				strconv.Itoa(iterationCap)+" deliveries and mounts; a surface "+
				"is almost certainly posting a message that comes back to it")
			f.queue = nil
			break
		}
		e := f.queue[0]
		// Clear the slot as well as dropping it: the backing array outlives the
		// reslice, and a delivered message should not be kept alive by it.
		f.queue[0] = envelope{}
		f.queue = f.queue[1:]
		if cmd := f.deliver(e.to, e.msg); cmd != nil {
			cmds = append(cmds, cmd)
		}
	}

	f.flush()
	if len(cmds) == 0 {
		return nil
	}
	return tea.Batch(cmds...)
}

// deliver runs the Deliver phase for one message and one surface. A handle that
// names nothing is a defined drop rather than an error, which is the point of
// addressing surfaces by handle.
func (f *Froth) deliver(h Handle, msg tea.Msg) tea.Cmd {
	n := f.reg.get(h)
	if n == nil {
		return nil
	}

	uc := &UpdateCtx{phaseToken: f.enter(phaseDeliver), ctx: f.ctx, self: h}
	defer f.leave()

	n.surface.Update(uc, msg)
	return tea.Batch(uc.cmds...)
}

// apply runs the Apply phase: everything structural that has been asked for
// since the last one.
//
// Focus commits before mounting, so a surface that mounts a child and focuses it
// there wins over a request made in the delivery before — the later of the two,
// and so the one that should stand.
func (f *Froth) apply(b *budget) {
	if !f.reg.hasEntered() && !f.focusPending {
		return
	}

	token := f.enter(phaseApply)
	defer f.leave()

	f.applyFocus()
	f.mountEntered(b, token)
}

// mountEntered mounts the surfaces which have entered the tree and not been
// mounted yet — the root to begin with, and then whatever the tree assembles for
// itself — before froth renders or delivers anything.
//
// It drains a queue rather than recursing over the tree, so a surface created
// late in the pass is mounted in that same pass whoever created it. Recursion
// would have missed one: an ApplyCtx stays valid for the whole of the Apply that
// issued it, so a surface can hand its own context to a sibling, and the child
// that appears under a node already walked past would have rendered without ever
// mounting.
func (f *Froth) mountEntered(b *budget, token phaseToken) {
	for {
		if !b.spend() {
			f.trip(b, "mounting did not settle within "+strconv.Itoa(iterationCap)+
				" deliveries and mounts; a surface is almost certainly mounting "+
				"a child of its own type")
			f.reg.abandonEntered()
			return
		}

		h, ok := f.reg.nextEntered()
		if !ok {
			return
		}

		// A surface that left the tree before it was mounted has nothing to
		// mount.
		n := f.reg.get(h)
		if n == nil {
			continue
		}
		if m, ok := n.surface.(Mounter); ok {
			m.Mount(&ApplyCtx{phaseToken: token, self: h})
		}
	}
}

// applyPending settles the structure before a frame that does not follow an
// Update: the first one, and any Composite taken on its own.
func (f *Froth) applyPending() {
	b := budget{left: iterationCap}
	f.apply(&b)
	f.flush()
}
