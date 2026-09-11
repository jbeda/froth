// Leaving the tree: destroy, the cascade under it, the leaf-up unmount walk,
// shutdown, and the quarantine that removes a surface which panicked.
//
// All four routes out are one walk. A surface destroyed on its own, a subtree
// destroyed by cascade, the whole tree at shutdown and a surface froth took out
// after it panicked all leave through removeSubtree, so a teardown hook cannot
// run for one route and silently not for another.

package froth

import "fmt"

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

// removeSubtree unmounts h, everything below it, and every modal opened from
// within that subtree, leaf-up, and then takes the lot out of the tree.
//
// Nothing is deleted until every Unmount has run. That is what lets a child
// reach its parent while tearing down, which is the entire reason the walk is
// leaf-up rather than the mirror of the mount order. A modal is unmounted
// before its opener for the same reason.
func (f *Froth) removeSubtree(h Handle) {
	n := f.reg.get(h)
	if n == nil || n.leaving {
		return
	}

	var order []Handle
	leaving := map[Handle]bool{}
	f.collectLeafUp(h, &order, leaving)
	for _, g := range order {
		f.reg.get(g).leaving = true
	}

	for _, g := range order {
		f.unmount(g)
		f.cancelCtx(g)
	}

	// Where focus goes is decided before the tree is cut, because afterwards
	// there is nothing left to ask whether focus was inside, or where a modal
	// that is going had saved it.
	rehome := leaving[f.focus]
	restore, restoring := f.restoreTarget(leaving)
	parent := n.parent
	// Capture goes with the holder, on every route out and without a message:
	// a surface leaving the tree can complete nothing, so the abandon the
	// message would ask for is already guaranteed.
	if leaving[f.capture] {
		f.clearCapture()
	}

	// A surface in its Unmount is still in the tree and may mint its post
	// capability there, so the handles are retired only once every Unmount
	// has run.
	f.inbox.retire(order)
	for _, g := range order {
		f.reg.remove(g)
	}
	switch {
	case restoring:
		// Nobody asked for this move, so it reports nothing; the target was
		// checked live and staying before the cut.
		f.landFocus(restore)
	case rehome:
		f.rehomeFocus(parent)
	}
}

// collectLeafUp appends h's subtree in leaf-up order — every descendant of a
// surface before the surface itself — and folds in the modals each surface
// opened, ahead of it, so an opener edge is walked like a parent link. seen
// records every handle appended: a modal is reached as the root's child and
// again through its opener, and must not be walked twice when the root
// itself is going.
func (f *Froth) collectLeafUp(h Handle, out *[]Handle, seen map[Handle]bool) {
	n := f.reg.get(h)
	if n == nil || seen[h] {
		return
	}
	seen[h] = true
	for _, c := range n.children {
		f.collectLeafUp(c, out, seen)
	}
	for _, m := range f.openedBy(h) {
		f.collectLeafUp(m, out, seen)
	}
	*out = append(*out, h)
}

// openedBy reports the modals h opened. They are the root's children, so the
// scan is over that list alone, and modals are few enough that no index is
// kept in the other direction.
func (f *Froth) openedBy(h Handle) []Handle {
	var out []Handle
	for _, c := range f.reg.get(f.root).children {
		if n := f.reg.get(c); n != nil && n.opener == h {
			out = append(out, c)
		}
	}
	return out
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

// cancelCtx runs after the surface's own Unmount, so a surface tears down
// under a live context, and for every surface in the walk: a quarantined
// surface is not asked to unmount but may have started work.
func (f *Froth) cancelCtx(h Handle) {
	if n := f.reg.get(h); n != nil && n.cancel != nil {
		n.cancel()
	}
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
	f.inbox.close()

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
	f.inbox.retire([]Handle{h})
	f.report(Diagnostic{
		Kind: DiagSurfacePanic,
		Detail: fmt.Sprintf("%s panicked in %s: %v; it has been taken out of "+
			"the tree and will not be called again", h, what, r),
	})
	f.recordDestroy(h)
}
