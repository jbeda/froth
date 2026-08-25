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
	// Moving focus to a scope from outside means entering it; from inside the
	// same move is literal. See ADR 0011.
	if f.isFocusScope(h) && !f.reg.descends(f.focused(), h) {
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
// or the surface itself — marked FocusScope, and the root when nothing on the
// way up is one. Answering "itself" for a scope surface is what keeps a scope
// closed.
func (f *Froth) focusScopeOf(h Handle) Handle {
	for h != (Handle{}) {
		n := f.reg.get(h)
		if n == nil {
			break
		}
		if h == f.root || n.meta.focusScope {
			return h
		}
		h = n.parent
	}
	return f.root
}

func (f *Froth) isFocusScope(h Handle) bool {
	n := f.reg.get(h)
	return n != nil && n.meta.focusScope
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
	if n == nil || n.meta.focusScope {
		return
	}
	*out = append(*out, h)
	for _, c := range n.children {
		f.collectFocusScoped(c, out)
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

// enterFocusScope reports where focus lands when it is moved to the scope s: the
// deepest surface s remembers that is still a stop, and the first stop within s
// when it remembers nothing usable. It reports false for a scope holding no stop
// at all, which leaves focus on the scope surface itself.
//
// The remembered chain is walked deepest-first because that is what "where focus
// left" means — a dialog remembers a button, not the row the button sits in.
// Every link is checked against the tree and against still being a stop, so a
// remembered surface that has died, or that its creator has kept out of
// traversal, costs a fallback rather than stranding focus. A remembered surface
// that is itself a scope is entered rather than landed on, so the rule holds
// however deep restoration reaches.
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
		if f.isTabStop(h) {
			return h, true
		}
	}

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
