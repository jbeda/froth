package froth

// Focus is one handle, not a flag per surface. The focus chain — the path from
// the root down to that surface — is derived from it by walking parents, so the
// two can never disagree: there is no per-surface boolean to leave set behind a
// focus change, and no chain to rebuild.

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
// focus names nothing in the tree.
func (f *Froth) focusChain() []Handle { return f.chainTo(f.focus) }

// focused reports the surface a raw key goes to: the deep end of the focus
// chain, or the root when there is no chain.
//
// The fallback is not defensive dressing: a handle outlives the surface it
// names, and input going nowhere at all is the one outcome an application
// cannot recover from.
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

// Keyboard traversal
//
// Tab order is child order. There are two orders in froth and no third: the
// child list drives what a layout arranges, the depth-first walk that assigns Z,
// and keyboard traversal. Z affects stacking alone.

// preorder returns every surface in the tree depth-first, root first — the same
// order place() draws in and Z ordinals are assigned in.
func (f *Froth) preorder() []Handle {
	var out []Handle
	f.collectPreorder(f.root, &out)
	return out
}

func (f *Froth) collectPreorder(h Handle, out *[]Handle) {
	n := f.reg.get(h)
	if n == nil {
		return
	}
	*out = append(*out, h)
	for _, c := range n.children {
		f.collectPreorder(c, out)
	}
}

// isTabStop reports whether traversal may land on h. Marking a surface a stop
// admits that surface alone: its children are walked either way, so a container
// never has to be a stop to keep its contents reachable.
//
// The root is never a stop. It is handed to New, which takes instance options
// rather than create options, so nothing can mark it one.
func (f *Froth) isTabStop(h Handle) bool {
	n := f.reg.get(h)
	return n != nil && n.meta.tabStop
}

// stepFocus resolves a relative focus move: the next tab stop after from in
// depth-first order, or the previous one when delta is -1. It wraps.
//
// It reports false when no surface is a tab stop, which leaves focus where it
// is. A from outside the tree starts the walk at the root.
func (f *Froth) stepFocus(from Handle, delta int) (Handle, bool) {
	order := f.preorder()
	if len(order) == 0 {
		return Handle{}, false
	}

	start := 0
	for i, h := range order {
		if h == from {
			start = i
			break
		}
	}

	for k := 1; k <= len(order); k++ {
		i := (start + delta*k) % len(order)
		if i < 0 {
			i += len(order)
		}
		if f.isTabStop(order[i]) {
			return order[i], true
		}
	}
	return Handle{}, false
}

// stepFrom is where a relative move counts from: the move already pending, if
// one is, and the committed focus otherwise. Two relative moves in one settle
// pass therefore advance two stops rather than landing on the same one twice.
func (f *Froth) stepFrom() Handle {
	if f.focusPending {
		return f.pendingFocus
	}
	return f.focused()
}
