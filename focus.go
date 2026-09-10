package froth

import (
	"fmt"
	"slices"
)

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
			// The walk is leaf-first; the chain is root-first.
			slices.Reverse(up)
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
			Detail: fmt.Sprintf("focus was asked to move to %s, which is not "+
				"in the tree; focus is unchanged", h),
		})
		return
	}
	f.landFocus(h)
}

// landFocus commits focus on a surface known to be in the tree. A scope that
// admits focus by no route is entered rather than landed on; one that admits
// it by any route takes the move literally, wherever focus was. See ADR 0019.
func (f *Froth) landFocus(h Handle) {
	if f.isFocusScope(h) && !f.admitsFocus(h) {
		if to, ok := f.enterFocusScope(h); ok {
			h = to
		}
	}
	f.focus = h
	f.rememberFocusChain()
}

// rememberFocusChain records the chain as restoration state: every surface on it
// notes which of its children the chain went through. It is written on every
// commit rather than on the way out of a subtree, so a surface that dies while
// focused leaves its ancestors' records already correct.
func (f *Froth) rememberFocusChain() {
	chain := f.focusChain()
	for i := 0; i+1 < len(chain); i++ {
		f.reg.get(chain[i]).lastActive = chain[i+1]
	}
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
//
// A focus scope bounds that walk, in both directions: the walk above a scope
// steps over it and everything under it, and the walk inside one cycles at its
// ends. Focus crosses only by being moved by name. See ADR 0011.

// focusScopeOf reports the scope a surface traverses within: the nearest ancestor —
// or the surface itself — that is a scope or a modal, and the root when nothing
// on the way up is one. Answering "itself" for a scope surface is what keeps a scope
// closed.
func (f *Froth) focusScopeOf(h Handle) Handle {
	for h != (Handle{}) {
		n := f.reg.get(h)
		if n == nil {
			break
		}
		if h == f.root || n.scopes() {
			return h
		}
		h = n.parent
	}
	return f.root
}

func (f *Froth) isFocusScope(h Handle) bool {
	n := f.reg.get(h)
	return n != nil && n.scopes()
}

// focusScopeOrder returns the surfaces the walk within s visits, depth-first and s
// first. A nested scope and everything under it are absent: they belong to that
// scope's own walk, not to this one. s itself is present, so a scope marked
// TabStop is a stop within itself and in no other scope.
func (f *Froth) focusScopeOrder(s Handle) []Handle {
	n := f.reg.get(s)
	if n == nil {
		return nil
	}
	out := []Handle{s}
	for _, c := range n.children {
		f.collectFocusScoped(c, &out)
	}
	return out
}

func (f *Froth) collectFocusScoped(h Handle, out *[]Handle) {
	n := f.reg.get(h)
	if n == nil || n.scopes() {
		return
	}
	*out = append(*out, h)
	for _, c := range n.children {
		f.collectFocusScoped(c, out)
	}
}

// isTabStop reports whether traversal may land on h. Declaring a surface a stop
// admits that surface alone: its children are walked either way, so a container
// never has to be a stop to keep its contents reachable.
func (f *Froth) isTabStop(h Handle) bool {
	n := f.reg.get(h)
	return n != nil && n.meta.flags.TabStop
}

// admitsFocus reports whether h takes focus by any route it declares — the
// keyboard walk or a click. Being focused by name needs no admission at all,
// so this is narrower than "can hold focus".
func (f *Froth) admitsFocus(h Handle) bool {
	n := f.reg.get(h)
	return n != nil && (n.meta.flags.TabStop || n.meta.flags.ClickFocus)
}

// enterFocusScope reports where focus lands when it is moved to the scope s: the
// deepest surface s remembers that still admits focus, and the first stop
// within s when it remembers nothing usable. It reports false for a scope
// holding no stop at all, which leaves focus on the scope surface itself.
//
// The remembered chain is walked deepest-first because that is what "where focus
// left" means — a dialog remembers a button, not the row the button sits in.
// Every link is checked against the tree and against still admitting focus, so
// a remembered surface that has died, or that has since declared itself out of
// focus altogether, costs a fallback rather than stranding focus. A remembered
// surface that is itself a scope is entered rather than landed on, so the rule
// holds however deep restoration reaches.
func (f *Froth) enterFocusScope(s Handle) (Handle, bool) {
	var remembered []Handle
	for cur := s; ; {
		n := f.reg.get(cur)
		if n == nil || n.lastActive == (Handle{}) || f.reg.get(n.lastActive) == nil {
			break
		}
		remembered = append(remembered, n.lastActive)
		cur = n.lastActive
	}
	for i := len(remembered) - 1; i >= 0; i-- {
		h := remembered[i]
		if f.isFocusScope(h) {
			if to, ok := f.enterFocusScope(h); ok {
				return to, true
			}
			continue
		}
		if f.admitsFocus(h) {
			return h, true
		}
	}

	// The two loops ask different questions on purpose: restoring puts focus
	// back where it was and a click is a legitimate way to have got there,
	// while an unremembered scope is being started fresh and should not open on
	// a click-only surface. See ADR 0019.
	for _, h := range f.focusScopeOrder(s) {
		if f.isTabStop(h) {
			return h, true
		}
	}
	return Handle{}, false
}

// stepFocus resolves a relative focus move: the next place focus can go after
// from, within from's scope, or the previous one when delta is -1. It wraps at
// the scope's ends.
//
// It reports false when the scope holds nowhere to go, which leaves focus where
// it is. A from outside the tree walks the root's scope.
func (f *Froth) stepFocus(from Handle, delta int) (Handle, bool) {
	s := f.focusScopeOf(from)
	order := f.focusScopeOrder(s)
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

// rehomeFocus puts focus somewhere real after the surface holding it left the
// tree. It lands on the nearest surviving ancestor and then descends the way
// entering a focus scope does — onto what that surface remembers, or its first
// tab stop — so closing one pane of several hands the keyboard to a sibling
// rather than teleporting it to the root.
//
// It is not a requested move and reports nothing. Nobody asked for it; froth is
// repairing a chain that a destroy broke.
func (f *Froth) rehomeFocus(p Handle) {
	if f.reg.get(p) == nil {
		f.focus = f.root
		f.rememberFocusChain()
		return
	}
	to := p
	if h, ok := f.enterFocusScope(p); ok {
		to = h
	}
	f.focus = to
	f.rememberFocusChain()
}

// focusOpenedModals moves focus into each modal created during this Apply.
// It runs after the mount drain because a modal has no children at Create,
// so the redirect would have found nothing to land on; and it stands down for
// a modal that already holds focus, so a Mount that focused its own child
// wins, as the Apply ordering promises. A modal that left again before this
// ran has nothing to take.
func (f *Froth) focusOpenedModals() {
	opened := f.openedModals
	f.openedModals = nil
	for _, h := range opened {
		if f.reg.get(h) == nil || f.reg.descends(f.focus, h) {
			continue
		}
		f.setFocus(h)
	}
}

// restoreTarget reports where focus goes back to when the surfaces in leaving
// go: the saved position of the innermost leaving modal holding focus,
// resolved through any other leaving modal it sits in. It reports false when
// focus is not inside a leaving modal, or nothing saved survives — the
// ordinary rehome's case either way.
//
// It runs before the registry is cut, because a saved position inside a modal
// that is also leaving is only recognisable while that modal's ancestry can
// still be read.
func (f *Froth) restoreTarget(leaving map[Handle]bool) (Handle, bool) {
	target := f.focus
	for {
		m, ok := f.leavingModalAbove(target, leaving)
		if !ok {
			return Handle{}, false
		}
		target = f.reg.get(m).restoreFocus
		if f.reg.get(target) == nil {
			return Handle{}, false
		}
		if !leaving[target] {
			return target, true
		}
	}
}

// leavingModalAbove reports the nearest modal at or above h that is leaving.
func (f *Froth) leavingModalAbove(h Handle, leaving map[Handle]bool) (Handle, bool) {
	for h != (Handle{}) {
		n := f.reg.get(h)
		if n == nil {
			return Handle{}, false
		}
		if leaving[h] && n.meta.modal {
			return h, true
		}
		h = n.parent
	}
	return Handle{}, false
}
