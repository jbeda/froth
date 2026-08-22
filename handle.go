package froth

import (
	"strconv"
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
// the compositor, which is what a later milestone hit-tests a click against.
func (h Handle) String() string {
	if h.id == 0 {
		return "froth.Handle(none)"
	}
	return "froth.Handle(" + strconv.FormatUint(h.reg, 10) +
		":" + strconv.FormatUint(h.id, 10) + ")"
}

// A node is froth's record of one surface: the surface itself, the parent it
// hangs off, and its children in sibling order. Sibling order is creation
// order, and it is what depth-first Z assignment turns into stacking order.
type node struct {
	surface  Surface
	parent   Handle
	children []Handle
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
	// which is the order they are mounted in.
	entered []Handle
}

// add registers s as a child of parent and returns its handle. A zero parent
// registers a root, which is the one surface froth is handed rather than told
// about by another surface.
func (r *registry) add(parent Handle, s Surface) Handle {
	if s == nil {
		panic("froth: a nil surface cannot enter the tree")
	}
	if r.nodes == nil {
		r.nodes = make(map[Handle]*node)
		r.id = registries.Add(1)
	}
	r.lastID++
	h := Handle{reg: r.id, id: r.lastID}
	r.nodes[h] = &node{surface: s, parent: parent}
	r.entered = append(r.entered, h)
	if p := r.nodes[parent]; p != nil {
		p.children = append(p.children, h)
	}
	return h
}

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
