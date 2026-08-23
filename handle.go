package froth

import (
	"fmt"
	"sync/atomic"
)

// A Handle is the stable identity of a surface. It is opaque, comparable, and
// valid to store and to name as a destination; the zero Handle names nothing.
//
// A handle outlives the surface it names, so froth can validate it at delivery
// and drop the message rather than call into a destroyed surface. It also names
// the instance that issued it, so a handle from one froth instance is a dead
// handle to another rather than an accidental alias for whatever surface
// happens to have been created in the same order. Surface pointers stay inside
// the registry — nothing outside froth ever holds one because it holds a
// handle.
type Handle struct{ reg, id uint64 }

// String renders a handle for diagnostics. It is also the layer ID froth gives
// the compositor.
func (h Handle) String() string {
	if h.id == 0 {
		return "froth.Handle(none)"
	}
	return fmt.Sprintf("froth.Handle(%d:%d)", h.reg, h.id)
}

// A node is froth's record of one surface: the surface itself, the parent it
// hangs off, its children in sibling order, the layout that places them, and
// what its creator said about it. Sibling order is creation order, and it is
// what depth-first Z assignment turns into stacking order.
//
// The metadata sits here rather than on the Handle: a handle is copied
// everywhere and outlives what it names, so a copy carrying stale flags would
// be a second answer to a question the registry already answers. See ADR
// 0001's amendment.
type node struct {
	surface  Surface
	parent   Handle
	children []Handle
	layout   Layout
	meta     meta

	// lastActive is the child of this surface that most recently sat on the
	// focus chain. It is restoration state — where focus lands when traversal
	// re-enters this subtree — and is never consulted to answer where focus is:
	// that is f.focus and the chain derived from it, and nothing else.
	lastActive Handle
}

// registries counts froth instances, so each one's handles are distinguishable
// from every other one's.
var registries atomic.Uint64

// A registry maps handles to the surfaces they name. It is the only thing that
// holds a surface pointer.
type registry struct {
	nodes map[Handle]*node
	id    uint64

	lastID uint64
	// entered records handles in the order their surfaces entered the tree,
	// which is the order they are mounted in. add is the only thing that puts
	// a handle here and nextEntered the only thing that takes one out.
	entered []Handle
}

// add registers s as a child of parent and returns its handle. A zero parent
// registers a root, which is the one surface froth is handed rather than told
// about by another surface.
//
// The options are folded into the node's metadata here, at the only moment a
// surface's creator gets to describe it: parenthood is fixed for a surface's
// lifetime (ADR 0006's amendment), and so are the flags decided alongside it.
// A root gets no options because New takes instance options, not create ones.
func (r *registry) add(parent Handle, s Surface, opts ...CreateOption) Handle {
	if s == nil {
		panic("froth: a nil surface cannot enter the tree")
	}
	if r.nodes == nil {
		r.nodes = make(map[Handle]*node)
		r.id = registries.Add(1)
	}
	r.lastID++
	h := Handle{reg: r.id, id: r.lastID}
	n := &node{surface: s, parent: parent}
	n.meta.apply(opts)
	r.nodes[h] = n
	r.entered = append(r.entered, h)
	if p := r.nodes[parent]; p != nil {
		p.children = append(p.children, h)
	}
	return h
}

// hasEntered reports whether any surface is waiting to be mounted.
func (r *registry) hasEntered() bool { return len(r.entered) > 0 }

// nextEntered takes the surface that has been waiting longest to mount,
// reporting false when none is.
func (r *registry) nextEntered() (Handle, bool) {
	if len(r.entered) == 0 {
		return Handle{}, false
	}
	h := r.entered[0]
	r.entered = r.entered[1:]
	return h, true
}

// abandonEntered drops the surfaces still waiting to mount. They stay in the
// registry and stay in the tree; they are simply never mounted, which is what
// froth is left with when a mount cascade runs out of budget.
func (r *registry) abandonEntered() { r.entered = nil }

// get returns the node h names, or nil if it names nothing.
func (r *registry) get(h Handle) *node { return r.nodes[h] }

// descends reports whether h is anc or hangs below it.
func (r *registry) descends(h, anc Handle) bool {
	for h != (Handle{}) {
		if h == anc {
			return true
		}
		n := r.nodes[h]
		if n == nil {
			return false
		}
		h = n.parent
	}
	return false
}
