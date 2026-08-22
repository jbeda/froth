package froth

// Focus is one handle, not a flag per surface. The focus chain — the path from
// the root down to that surface — is derived from it by walking parents, so the
// two can never disagree: there is no per-surface boolean to leave set behind a
// focus change, and no chain to rebuild.
//
// M2 resolves declared bindings along that chain, deepest first, which is why
// the chain is built here rather than a boolean being stored per surface.
// Restoration state — each container remembering which child was last active,
// so focus entering a scope lands where it left — is M2's too, and hangs off
// the same handle.

// chainTo returns the path from the root down to h, or nil if h is not in the
// tree. Reaching the root is what "in the tree" means: a handle can name a live
// registry entry and still hang off nothing, and a handle from another instance
// names nothing at all.
//
// This is both the focus chain and the validation for setting focus, which is
// deliberate — a target froth cannot build a chain to is a target it could not
// route to either.
func (f *Froth) chainTo(h Handle) []Handle {
	var up []Handle
	for h != (Handle{}) {
		n := f.reg.get(h)
		if n == nil {
			return nil
		}
		up = append(up, h)
		if h == f.root {
			// Reverse in place: the walk is leaf-first, the chain is root-first.
			for i, j := 0, len(up)-1; i < j; i, j = i+1, j-1 {
				up[i], up[j] = up[j], up[i]
			}
			return up
		}
		h = n.parent
	}
	return nil
}

// focusChain returns the path from the root to the focused surface, or nil if
// focus names nothing in the tree. It is what M2 matches bindings along; M1
// reads only the surface at its deep end.
func (f *Froth) focusChain() []Handle { return f.chainTo(f.focus) }

// focused reports the surface a raw key goes to: the deep end of the focus
// chain, or the root when there is no chain.
//
// The fallback is not defensive dressing. A handle outlives the surface it
// names, so once M3 can destroy one, the focused surface can leave the tree
// between the frame that focused it and the key that follows — and input going
// nowhere at all is the one outcome an application cannot recover from.
func (f *Froth) focused() Handle {
	chain := f.focusChain()
	if len(chain) == 0 {
		return f.root
	}
	return chain[len(chain)-1]
}

// setFocus moves focus to h, or reports why it did not. A target that is not in
// the tree is rejected rather than accepted and quietly failing to receive
// anything: focus is where input goes, so a bad handle would silently swallow
// every key the application pressed afterwards.
//
// Rejection is a diagnostic rather than a panic. The handle is very often one
// the application stored earlier and the surface behind it may legitimately
// have gone, which is precisely the case ADR 0001 asks froth to survive.
func (f *Froth) setFocus(h Handle) {
	if f.chainTo(h) == nil {
		f.report(Diagnostic{
			Kind: DiagInvalidFocus,
			Detail: "focus was asked to move to " + h.String() +
				", which is not in the tree; focus is unchanged",
		})
		return
	}
	f.focus = h
}

// applyFocus commits the focus move a surface asked for during Deliver. It runs
// at the top of Apply, before mounts, so a surface mounting a child and
// focusing it immediately wins over a request made in the delivery before —
// which is the later of the two, and so the one that should stand.
func (f *Froth) applyFocus() {
	if !f.focusPending {
		return
	}
	h := f.pendingFocus
	f.pendingFocus, f.focusPending = Handle{}, false
	f.setFocus(h)
}
