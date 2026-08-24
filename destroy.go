// Leaving the tree: destroy, the cascade under it, the leaf-up unmount walk,
// shutdown, and the quarantine that removes a surface which panicked.
//
// All four routes out are one walk. A surface destroyed on its own, a subtree
// destroyed by cascade, the whole tree at shutdown and a surface froth took out
// after it panicked all leave through removeSubtree, so a teardown hook cannot
// run for one route and silently not for another.

package froth

import "fmt"

// An Unmounter is told when its surface leaves the tree, whichever route it
// leaves by: destroyed, cascaded away under an ancestor, or unmounted at
// shutdown. It runs once, during Apply, and never for a surface that panicked.
//
// It is the mirror of Mount and runs leaf-up, so a child tears down before its
// parent and can still reach it while doing so. It is for what froth does not
// own — a file handle, a registration in something outside froth — and most
// surfaces need it not at all.
//
// Posting from it is ordinary: it runs in Apply and a rising message posted
// there behaves like any other. During Shutdown nothing will drain the queue,
// so a post there is dropped and reported.
type Unmounter interface {
	Unmount(ac *ApplyCtx)
}

// destroy takes a surface out of the tree, or records that it should be. It is
// immediate during Apply, which is where structural change commits, and
// recorded anywhere else — during Deliver, so the drain finishes against the
// tree it started with, and during Render, which is what quarantine uses.
//
// The root has no route here: no surface can mint an accessor naming it as a
// child, and asking for it through Self is the one way to reach this with it.
func (r router) destroy() {
	r.check()
	if r.h == r.f.root {
		bug("froth: the root surface cannot be destroyed; nobody created it, " +
			"and the tree it heads is the instance")
	}
	r.f.recordDestroy(r.h)
}

// recordDestroy is destroy without the accessor, which is what quarantine needs:
// a panicking surface has to leave from wherever it panicked.
func (f *Froth) recordDestroy(h Handle) {
	if f.reg.get(h) == nil {
		return
	}
	if f.phase == phaseApply {
		f.removeSubtree(h)
		return
	}
	f.pendingDestroy = append(f.pendingDestroy, h)
}

// applyDestroys commits the destroys recorded since the last Apply. It runs at
// the top of Apply, before focus and before mounting: the tree shrinks first, so
// a focus move recorded in the same delivery is validated against the tree that
// will exist rather than committed onto a surface that is about to go.
//
// It loops because an Unmount may destroy something else. A destroy asked for
// during Apply is immediate, so that one is already done by the time this looks
// again; the loop is what makes that true rather than something to rely on.
func (f *Froth) applyDestroys() {
	for len(f.pendingDestroy) > 0 {
		pending := f.pendingDestroy
		f.pendingDestroy = nil
		for _, h := range pending {
			f.removeSubtree(h)
		}
	}
}

// removeSubtree unmounts h and everything below it, leaf-up, and then takes the
// whole subtree out of the tree.
//
// Nothing is deleted until every Unmount has run. That is what lets a child
// reach its parent while tearing down, which is the entire reason the walk is
// leaf-up rather than the mirror of the mount order.
func (f *Froth) removeSubtree(h Handle) {
	n := f.reg.get(h)
	if n == nil || n.leaving {
		return
	}

	var order []Handle
	f.collectLeafUp(h, &order)
	for _, g := range order {
		f.reg.get(g).leaving = true
	}

	for _, g := range order {
		f.unmount(g)
	}

	// Where focus goes is decided before the tree is cut, because afterwards
	// there is nothing left to ask whether focus was inside.
	rehome := f.reg.descends(f.focus, h)
	parent := n.parent

	for _, g := range order {
		f.reg.remove(g)
	}
	if rehome {
		f.rehome(parent)
	}
}

// collectLeafUp appends h's subtree in leaf-up order: every descendant of a
// surface comes before the surface itself.
func (f *Froth) collectLeafUp(h Handle, out *[]Handle) {
	n := f.reg.get(h)
	if n == nil {
		return
	}
	for _, c := range n.children {
		f.collectLeafUp(c, out)
	}
	*out = append(*out, h)
}

// unmount runs one surface's teardown. A surface that never reached its mount
// step has nothing to undo, and one froth has quarantined is not asked again —
// its Unmount is code from the same surface that has already failed once.
func (f *Froth) unmount(h Handle) {
	n := f.reg.get(h)
	if n == nil || !n.mounted || n.panicked {
		return
	}
	u, ok := n.surface.(Unmounter)
	if !ok {
		return
	}
	defer f.quarantine(h, "Unmount")
	u.Unmount(&ApplyCtx{phaseToken: f.current(), self: h})
}

// rehome puts focus somewhere real after the surface holding it left the tree.
// It lands on the nearest surviving ancestor and then descends the way entering
// a scope does — onto what that surface remembers, or its first tab stop — so
// closing one pane of several hands the keyboard to a sibling rather than
// teleporting it to the root.
//
// It is not a requested move and reports nothing. Nobody asked for it; froth is
// repairing a chain that a destroy broke.
func (f *Froth) rehome(p Handle) {
	if f.reg.get(p) == nil {
		f.focus = f.root
		f.remember()
		return
	}
	to := p
	if h, ok := f.enterScope(p); ok {
		to = h
	}
	f.focus = to
	f.remember()
}

// Shutdown tears the instance down: the same leaf-up unmount walk every destroy
// runs, once from the root, so a teardown hook fires at the one moment every
// application expects it to. Call it when the program is quitting.
//
// The instance is spent afterwards. The tree is empty, every handle into it is
// dead, and Update and View are defined but do nothing. Calling it twice is
// harmless.
//
// Posting during the walk is dropped and reported: nothing will drain the queue
// after this returns.
func (f *Froth) Shutdown() {
	if f.shutdown {
		return
	}
	f.shutdown = true

	if f.reg.get(f.root) != nil {
		f.enter(phaseApply)
		f.removeSubtree(f.root)
		f.leave()
	}
	f.pendingDestroy = nil
	f.queue = nil
	f.reg.abandonEntered()
	f.flush()
}

// quarantine recovers a panic that came out of a surface, reports it, and takes
// that surface out of the tree by the ordinary destroy path. The program carries
// on: a broken surface is not a reason to lose the terminal.
//
// A panic froth raised itself is re-raised rather than recovered. Those say a
// program has violated froth's contract — a context retained past its phase, a
// handle that is not a child where a child was required — and burying one in a
// diagnostic would leave the bug in place and the report far from its cause.
//
// It is deferred, so it must be called as the deferred function itself; recover
// answers nothing a frame further down.
func (f *Froth) quarantine(h Handle, what string) {
	r := recover()
	if r == nil {
		return
	}
	if p, ok := r.(frothPanic); ok {
		panic(p)
	}

	// Marked before the destroy is recorded, because a destroy asked for during
	// Deliver commits at the next Apply and the surface would otherwise be
	// asked again in between. This is what "not called again" rests on; the
	// removal is what it means afterwards.
	if n := f.reg.get(h); n != nil {
		n.panicked = true
	}
	f.report(Diagnostic{
		Kind: DiagSurfacePanic,
		Detail: fmt.Sprintf("%s panicked in %s: %v; it has been taken out of "+
			"the tree and will not be called again", h, what, r),
	})
	f.recordDestroy(h)
}
