// Update, Deliver and Apply: how one message becomes a settled tree.
//
// Everything here is the alternation ADR 0002 describes — Apply, then Deliver,
// repeating until nothing is left to deliver — and the budget that stops it
// running forever. This file never draws; rendering is render.go.

package froth

import (
	"fmt"

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
// stops being a key.
//
// A press that matches a binding on the focus chain becomes an ActionMsg, which
// rises: resolution says which action the key means and delivery says who acts
// on it. A press that matches nothing, and every other key event, goes to the
// focused surface as it stands. A command's result arrives addressed to the
// surface that ran it and goes there. Everything else goes to the root, which
// is the surface the application handed froth and so the one its own messages
// belong at.
//
// Only a press resolves. A release is the same key arriving a second time, so
// matching it would fire every action twice.
//
// A mouse message goes to the surface holding capture, or else to the surface
// under the cursor; see routeMouse.
//
// It reports false for input that goes nowhere: a raw key or a mouse message
// aimed inside the subtree an open modal blocks, or a mouse message with nothing
// under it. Resolution above has already offered a press the only two ways past
// a modal, and a key arriving here matched neither.
func (f *Froth) route(msg tea.Msg) (envelope, bool) {
	if a, ok := msg.(addressedMsg); ok {
		return envelope(a), true
	}
	if k, ok := msg.(tea.KeyPressMsg); ok {
		if a, ok := f.resolve(k); ok {
			return f.action(a), true
		}
	}
	if _, ok := msg.(tea.KeyMsg); ok {
		focused := f.focused()
		if f.blockedByModal(focused) {
			return envelope{}, false
		}
		return envelope{to: focused, msg: msg}, true
	}
	if m, ok := msg.(tea.MouseMsg); ok {
		return f.routeMouse(m)
	}
	return envelope{to: f.root, msg: msg}, true
}

// action addresses an action at the active surface, where the walk that delivers
// it starts. A resolved key press and a keyless invocation both come through
// here, so a command palette needs no path of its own.
func (f *Froth) action(a Action) envelope {
	return envelope{to: f.focused(), msg: ActionMsg{Action: a}}
}

// A budget bounds one settle pass. Delivering a message and mounting a surface
// each spend from the same one, because a cascade that will not settle can be
// made of either and the two interleave: a delivery can create a surface whose
// mount posts a message.
type budget struct {
	left    int
	tripped bool

	// untaken records what has already been reported as untaken during this
	// pass. See reportUntaken and untakenKind.
	untaken map[string]bool
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

// settle applies, drains the whole message queue, and repeats while anything
// structural is still pending.
//
// Apply leads because the tree has to exist before anything can be delivered
// into it — the root's own mount is the first Apply any instance runs. It then
// runs only once the queue is empty, so no delivery ever has the tree change
// under it: a rising message cannot be halfway up a path that is being
// rebuilt, and a message posted to a surface destroyed later in the same turn
// is delivered, because the destroy commits at the Apply after the drain. The
// price is that a delivery asking for Apply waits for the whole drain, so the
// messages queued behind a focus request see the old focus.
//
// Delivering a message and mounting a surface both spend from one budget, and
// running out of it means a cascade that is not going to settle. froth abandons
// what it was draining, reports it, and lets the program carry on rather than
// hanging. See iterationCap for why the budget is the size it is.
func (f *Froth) settle() tea.Cmd {
	b := budget{left: iterationCap}
	return f.settleWith(&b)
}

// settleWith is settle against a budget the caller already holds, for when
// something has been spent before the first delivery. One budget per message
// rather than per call is what keeps a cascade that starts in Apply and
// continues through Deliver bounded by one cap.
func (f *Froth) settleWith(b *budget) tea.Cmd {
	var cmds []tea.Cmd

	for {
		f.apply(b)
		if len(f.queue) == 0 || !f.drain(b, &cmds) {
			break
		}
	}

	f.flush()
	return tea.Batch(cmds...)
}

// drain delivers the whole queue, including what delivering it adds. It reports
// false when the budget ran out, having abandoned what was left.
func (f *Froth) drain(b *budget, cmds *[]tea.Cmd) bool {
	for len(f.queue) > 0 {
		e := f.queue[0]
		// Clear the slot as well as dropping it: the backing array outlives the
		// reslice, and a delivered message should not be kept alive by it.
		f.queue[0] = envelope{}
		f.queue = f.queue[1:]
		if !f.deliverEntry(e, b, cmds) {
			f.queue = nil
			return false
		}
	}
	return true
}

// deliverEntry delivers one queue entry: one surface for an addressed message,
// and the whole climb for a rising one. It reports false when the budget ran
// out partway.
//
// A walk is one entry rather than one per hop, which is what makes it
// uninterruptible: a message posted while a rising one is passing through goes
// to the tail of the queue and waits, rather than being delivered between two
// of its hops.
func (f *Froth) deliverEntry(e envelope, b *budget, cmds *[]tea.Cmd) bool {
	rising := Rises(e.msg)
	h := e.to
	for {
		if !b.spend() {
			f.trip(b, fmt.Sprintf("the message queue did not drain within %d "+
				"deliveries and mounts; a surface is almost certainly posting "+
				"a message that comes back to it", iterationCap))
			return false
		}

		n := f.reg.get(h)
		if n == nil {
			// A handle that names nothing is a defined drop rather than an
			// error, which is the point of addressing surfaces by handle. It
			// ends a walk too: a path froth cannot climb is not one it can
			// guess the rest of.
			return true
		}
		if n.panicked {
			// Quarantined and waiting for the Apply that removes it. A walk
			// still climbs past it, because the surfaces above it are fine.
			if !rising || n.parent == (Handle{}) {
				return true
			}
			h = n.parent
			continue
		}
		cmd, stopped := f.deliver(n, h, e.msg, rising)
		if cmd != nil {
			*cmds = append(*cmds, cmd)
		}
		if !rising || stopped {
			return true
		}
		if n.parent == (Handle{}) {
			break
		}
		h = n.parent
	}

	// froth is the tail. It takes the actions it implements itself, after every
	// surface has had its turn, so an application pre-empts one by taking the
	// message and needs no override table.
	if !f.service(e.msg) {
		f.reportUntaken(b, e)
	}
	return true
}

// reportUntaken says a rising message climbed past every surface with nobody
// taking it — once per finding per settle pass, where the finding is what
// untakenKind names.
//
// Coalescing is what keeps one bug from arriving as thousands of reports: what
// is missing is a case, so two hundred rows posting the same untaken message is
// one thing to fix, and a cascade of them would otherwise spend the whole budget
// reporting. Two distinct findings are reported separately. The handle in the
// detail is the first one seen, as an example of where to look.
func (f *Froth) reportUntaken(b *budget, e envelope) {
	if b.untaken == nil {
		b.untaken = make(map[string]bool)
	}
	kind := untakenKind(e.msg)
	if b.untaken[kind] {
		return
	}
	b.untaken[kind] = true
	f.report(Diagnostic{
		Kind: DiagUntakenMessage,
		Detail: fmt.Sprintf("a %s posted to %s rose past every surface and "+
			"nothing took it", kind, e.to),
	})
}

// untakenKind names the finding a walk that nobody ended represents, which is
// what reportUntaken coalesces on. It is the message type, except for an
// ActionMsg: every action is one type, and the missing case is per action
// rather than per type, so reporting one of them would hide the rest.
func untakenKind(msg tea.Msg) string {
	if m, ok := msg.(ActionMsg); ok {
		return fmt.Sprintf("%T for %q", msg, m.Action)
	}
	return fmt.Sprintf("%T", msg)
}

// deliver runs the Deliver phase for one message and one surface, reporting
// whether the surface ended the walk it was on.
func (f *Froth) deliver(n *node, h Handle, msg tea.Msg, rising bool) (tea.Cmd, bool) {
	uc := &UpdateCtx{
		phaseToken: f.enter(phaseDeliver),
		self:       h,
		msg:        msg,
		rising:     rising,
	}
	defer f.leave()

	f.callUpdate(uc, n, h, msg)
	return tea.Batch(uc.cmds...), uc.stopped
}

// callUpdate hands the message to the surface under quarantine. A surface that
// panics has done nothing as far as the walk is concerned: it cannot have ended
// one, so a rising message carries on to its parent exactly as it would past a
// surface that ignored it.
func (f *Froth) callUpdate(uc *UpdateCtx, n *node, h Handle, msg tea.Msg) {
	defer f.quarantine(h, "Update")
	n.surface.Update(uc, msg)
}

// apply runs the Apply phase: everything structural that has been asked for
// since the last one.
//
// The order is destroy, focus, then reconcile and mount together. Destroys go
// first so that a focus move recorded in the same delivery is validated against
// the tree that will exist — which is what makes a move onto a surface that has
// since left the tree a reported refusal rather than a focus committed and
// immediately undone. Focus commits before mounting, so a surface that mounts a
// child and focuses it there wins over a request made in the delivery before:
// the later of the two, and so the one that should stand. Reconciles come last,
// after focus has been validated against the tree they have not yet cut, and
// they share a drain with mounting because each feeds the other: a reconcile
// creates surfaces to mount, and a mount may ask for a reconcile.
func (f *Froth) apply(b *budget) {
	if f.shutdown {
		return
	}
	if !f.reg.hasEntered() && !f.focusPending &&
		len(f.pendingDestroy) == 0 && len(f.pendingReconcile) == 0 {
		return
	}

	token := f.enter(phaseApply)
	defer f.leave()

	f.applyDestroys()
	f.applyFocus()
	f.reconcileAndMount(b, token)
	f.focusOpenedModals()
}

// reconcileAndMount runs the two callbacks that build the tree out: Reconcile
// for the surfaces that asked for one, and Mount for the surfaces which have
// entered the tree and not been mounted yet — the root to begin with, and then
// whatever the tree assembles for itself.
//
// The two share one drain because each makes work for the other, and a
// reconcile asked for from inside a reconcile has to be serviced somewhere. It
// drains queues rather than recursing over the tree, so a surface created late
// in the pass is reached in that same pass whoever created it. Recursion would
// have missed one: an ApplyCtx stays valid for the whole of the Apply that
// issued it, so a surface can hand its own context to a sibling, and the child
// that appears under a node already walked past would have rendered without ever
// mounting.
//
// Reconciles go first each time round, so a surface waiting to re-express
// itself does so before froth mounts what the last one produced. A cascade of
// either spends from the budget that stops it.
func (f *Froth) reconcileAndMount(b *budget, token phaseToken) {
	for {
		if !b.spend() {
			f.trip(b, fmt.Sprintf("reconciling and mounting did not settle "+
				"within %d deliveries and mounts; a surface is almost certainly "+
				"mounting a child of its own type, or asking to reconcile from "+
				"inside its own Reconcile", iterationCap))
			f.reg.abandonEntered()
			f.abandonReconciles()
			return
		}

		if h, ok := f.nextReconcile(); ok {
			f.callReconcile(token, h)
			continue
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
		if n.panicked {
			continue
		}
		n.mounted = true
		if m, ok := n.surface.(Mounter); ok {
			f.callMount(token, m, h)
		}
	}
}

func (f *Froth) callMount(token phaseToken, m Mounter, h Handle) {
	defer f.quarantine(h, "Mount")
	m.Mount(&ApplyCtx{phaseToken: token, self: h})
}
