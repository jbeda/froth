// Reconcile: a surface re-expressing the children it owns, part-way through an
// Apply.
//
// A request is a bit on the node and the handle on a work list. The bit is what
// makes asking twice ask once; the list is what makes servicing the requests a
// drain rather than a walk of the tree looking for marks, which is the shape
// mounting already has.

package froth

import "fmt"

// requestReconcile records that this surface wants Reconcile called. It is
// never immediate, whatever phase asks: a request made from inside Reconcile is
// the same request as one made from Deliver, and running it where it was asked
// would put a surface inside its own callback.
func (r router) requestReconcile() {
	r.check()
	n := r.f.reg.get(r.h)
	if n == nil {
		return
	}
	if _, ok := n.surface.(Reconciler); !ok {
		r.f.report(Diagnostic{
			Kind: DiagNoReconciler,
			Detail: fmt.Sprintf("%s asked to be reconciled, but %T has no "+
				"Reconcile method; nothing was queued", r.h, n.surface),
		})
		return
	}
	if n.wantsReconcile {
		return
	}
	n.wantsReconcile = true
	r.f.pendingReconcile = append(r.f.pendingReconcile, r.h)
}

// nextReconcile takes the surface that has been waiting longest, skipping the
// ones that have since left the tree or been quarantined. The bit is cleared
// before the call, so a surface asking again from inside its own Reconcile is
// asking afresh rather than being told it has already asked.
func (f *Froth) nextReconcile() (Handle, bool) {
	for len(f.pendingReconcile) > 0 {
		h := f.pendingReconcile[0]
		f.pendingReconcile = f.pendingReconcile[1:]
		n := f.reg.get(h)
		if n == nil || n.panicked {
			continue
		}
		n.wantsReconcile = false
		return h, true
	}
	return Handle{}, false
}

// callReconcile hands the surface a full Apply context under quarantine.
func (f *Froth) callReconcile(token phaseToken, h Handle) {
	defer f.quarantine(h, "Reconcile")
	f.reg.get(h).surface.(Reconciler).Reconcile(&ApplyCtx{phaseToken: token, self: h})
}

// abandonReconciles drops the requests still waiting when a pass runs out of
// budget. The bits go with them: a surface left marked would be unable to ask
// again, since the mark is what says it already has.
func (f *Froth) abandonReconciles() {
	for _, h := range f.pendingReconcile {
		if n := f.reg.get(h); n != nil {
			n.wantsReconcile = false
		}
	}
	f.pendingReconcile = nil
}
