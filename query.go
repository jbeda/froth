// Queries: the read-only questions an application or a surface may ask about
// the tree as it currently stands.

package froth

import "slices"

// A Query is a read-only view of a froth instance. It answers questions about
// the tree as it stands and changes nothing, which is what makes it safe to hold
// and safe to hand to a surface: there is nothing on it that can drive froth.
//
// Every instance and every context hands one out, so a surface that needs to
// know something about the world reaches for this rather than for the instance.
// It cannot be constructed — the zero Query panics on use.
type Query struct{ f *Froth }

// Query returns a read-only view of this instance.
func (f *Froth) Query() Query { return Query{f: f} }

// Query returns a read-only view of froth. Everything on it is safe to ask while
// rendering: it changes nothing.
func (rc *RenderCtx) Query() Query {
	rc.check()
	return Query{f: rc.f}
}

// Query returns a read-only view of froth.
func (uc *UpdateCtx) Query() Query {
	uc.check()
	return Query{f: uc.f}
}

// Query returns a read-only view of froth.
func (ac *ApplyCtx) Query() Query {
	ac.check()
	return Query{f: ac.f}
}

func (q Query) check() {
	if q.f == nil {
		panic("froth: this Query was not handed out by froth; ask an instance " +
			"or a context for one")
	}
}

// Focus reports the surface a raw key press would be delivered to. An
// application compares it against handles it created to learn whether one of
// them holds the keyboard.
//
// It answers the root when focus names nothing in the tree, because that is
// where the key would go.
func (q Query) Focus() Handle {
	q.check()
	return q.f.focused()
}

// LiveBindings reports what a key press would currently do, in the order presses
// are matched: deepest on the focus chain first, then froth's own. It is the
// walk resolution uses, so help cannot describe something other than what
// pressing the key does.
//
// Each binding carries only the keys that would actually reach it. A key some
// surface closer to focus already answers to is gone from the binding behind it,
// and a text sink holding focus takes every key that is ordinary typing out of
// all of them. A binding left with no keys is not reported.
//
// A disabled binding is reported with its Disabled field set rather than
// omitted, so an application can grey it instead of losing it.
func (q Query) LiveBindings() []ChainBinding {
	q.check()
	collected := q.f.chainBindings()
	out := make([]ChainBinding, 0, len(collected))
	for i := range collected {
		if len(collected[i].live) == 0 {
			continue
		}
		cb := collected[i].ChainBinding
		cb.Keys = collected[i].live
		out = append(out, cb)
	}
	return out
}

// Actions reports every action the focus chain currently offers, in the same
// order LiveBindings uses, each with the handle that declared it. An action with
// no keys is reported like any other, and a text sink holding focus changes
// nothing: the question is what can be asked for, not what a key would do.
//
// Keys carries whatever keys reach the action, which may be none. Disabled is
// reported rather than omitted and gates nothing — Invoke takes an identity, so
// what to do with a disabled entry is the application's call.
func (q Query) Actions() []ChainBinding {
	q.check()
	collected := q.f.chainBindings()
	out := make([]ChainBinding, 0, len(collected))
	for i := range collected {
		cb := collected[i].ChainBinding
		cb.Keys = collected[i].listed
		out = append(out, cb)
	}
	return out
}

// StockBindings returns the bindings froth declares for the actions it services
// itself — the ones a help display shows without any surface having declared
// anything.
//
// They are already matched, after every surface on the focus chain, so an
// application needs this only to change them: declare the ones it wants with its
// own keys, or with Disabled set, and the declaration takes the action over.
// Nothing has to be handed back to froth.
func StockBindings() []Binding {
	out := slices.Clone(frothBindings)
	for i := range out {
		out[i].Keys = slices.Clone(out[i].Keys)
	}
	return out
}
