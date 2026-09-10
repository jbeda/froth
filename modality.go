// Modality: which surfaces an open modal takes input away from, and which modal
// wins when more than one is open.
//
// A modal takes input for its parent's whole subtree, so the modals that block a
// surface are exactly the modal children of that surface's chain to the root.
// That is what makes this a walk up the chain rather than an index froth has to
// keep or a scan of the tree per key press. See ADR 0008.

package froth

import "slices"

// governingModal reports the modal whose block h falls under, or false when no
// open modal reaches it. h may itself be inside the modal returned;
// blockedByModal is what separates the two cases.
//
// Several modals can be open over one surface. The one on top governs, in
// froth's depth-first order over the live tree — the order place assigns Z
// ordinals in, so the answer agrees with what is drawn without waiting for a
// frame. See ADR 0008 for why several at once is ordinary rather than an error.
func (f *Froth) governingModal(h Handle) (Handle, bool) {
	var best Handle
	var found bool
	for _, anc := range f.chainTo(h) {
		n := f.reg.get(anc)
		if n == nil {
			continue
		}
		for _, c := range n.children {
			cn := f.reg.get(c)
			// A quarantined modal draws nothing and declares nothing, so it
			// blocks nothing either: it is waiting to leave the tree, and an
			// invisible surface holding the whole subtree hostage is the one
			// state nobody could act on.
			if cn == nil || cn.panicked || !cn.meta.modal {
				continue
			}
			if !found || f.beforeInTree(best, c) {
				best, found = c, true
			}
		}
	}
	return best, found
}

// blockedByModal reports whether an open modal has taken input away from h:
// something governs h and h is not inside it.
func (f *Froth) blockedByModal(h Handle) bool {
	g, ok := f.governingModal(h)
	return ok && !f.reg.descends(h, g)
}

// beforeInTree reports whether a comes before b in the depth-first walk that
// decides stacking. Sibling order is creation order, so the later of two
// siblings — and everything under it — draws over the earlier one, and an
// ancestor is visited before its own descendants.
func (f *Froth) beforeInTree(a, b Handle) bool {
	pa, pb := f.chainTo(a), f.chainTo(b)
	// Both walks start at the root, so the first place they can differ is the
	// pair of children of their deepest common ancestor. Every handle in a
	// chain resolved on the way up — chainTo abandons the walk otherwise — so
	// the parent looked up below is live.
	for i := 1; i < len(pa) && i < len(pb); i++ {
		if pa[i] == pb[i] {
			continue
		}
		n := f.reg.get(pa[i-1])
		return slices.Index(n.children, pa[i]) < slices.Index(n.children, pb[i])
	}
	return len(pa) < len(pb)
}
