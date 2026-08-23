// Bindings and their resolution: how a key press becomes an action, and which
// surface receives it. This is ADR 0008's first two steps; the third — a key
// that matched nothing reaching the focused surface — is in update.go.

package froth

import tea "charm.land/bubbletea/v2"

// An Action is the stable identity of a shortcut, chosen by whoever declares
// it. A surface handles actions rather than keys, so the keys that reach it can
// change without the surface changing.
//
// Identity is a caller's to keep unique. froth compares actions nowhere: it
// carries the one whose binding matched to the surface that declared it, and
// two surfaces using the same string are talking about the same action only if
// they meant to.
type Action string

// An ActionMsg is what a surface receives when one of its bindings matched. It
// arrives in Update like any other message, in place of the key.
type ActionMsg struct{ Action Action }

// A Binding declares that a surface answers to some keys, under one action,
// with help text describing what it does. Declaring it is the whole of it: the
// surface never matches the key itself.
//
// The zero value is enabled and does not survive a modal, so a binding that
// says nothing about either is the ordinary case.
type Binding struct {
	// Action is what the surface receives when one of Keys matches.
	Action Action

	// Keys are the key presses that match, spelled as Bubble Tea's KeyMsg
	// renders itself: "q", "ctrl+c", "shift+tab", "?".
	Keys []string

	// Help describes what the action does, in the words the application would
	// put in front of a user.
	Help string

	// KeyHelp is how the keys are spelled in help, for when listing Keys is
	// not what a reader wants to see — "↑/k" for a binding on "up" and "k".
	// Empty means Keys says it well enough.
	KeyHelp string

	// Disabled takes the binding out of play without removing it: it neither
	// fires nor stops the key resolving further up the chain, and help can
	// still see it. Disabling is how a shortcut becomes context-sensitive —
	// a surface recomputes the field from its own state each time it is asked.
	//
	// It is phrased as the exception so the zero Binding is one that works.
	Disabled bool

	// SurvivesModal keeps a binding resolving from outside an open modal, which
	// otherwise stops resolution at itself. froth declares none of these: what
	// an application must be able to do from inside a modal is the
	// application's to decide.
	SurvivesModal bool
}

// A Binder declares the shortcuts a surface answers to. froth asks for them
// while resolving a key press, which is a synchronous call into a surface —
// rare enough in froth to be worth naming.
//
// Bindings must therefore be a read-only answer about the surface as it stands.
// It is called outside every phase, with no context to reach froth through, and
// may be called more than once for one key press.
type Binder interface {
	Bindings() []Binding
}

// matches reports whether k is one of the binding's keys and the binding is in
// play.
func (b *Binding) matches(k string) bool {
	if b.Disabled {
		return false
	}
	for _, want := range b.Keys {
		if want == k {
			return true
		}
	}
	return false
}

// resolve matches a key press against the focus chain, deepest first, and
// reports the surface that declared the winning binding along with its action.
// Within one surface, declaration order decides: the first binding listing the
// key wins, so binding a key twice gets the answer written first rather than an
// arbitrary one.
//
// The chain is walked from the focused surface rather than from f.focus, so
// resolution and a raw key agree about where the deep end is when focus names
// nothing.
func (f *Froth) resolve(k tea.KeyPressMsg) (Handle, Action, bool) {
	pressed := k.String()
	chain := f.chainTo(f.focused())
	for i := len(chain) - 1; i >= 0; i-- {
		n := f.reg.get(chain[i])
		if n == nil {
			continue
		}
		b, ok := n.surface.(Binder)
		if !ok {
			continue
		}
		declared := b.Bindings()
		for j := range declared {
			if declared[j].matches(pressed) {
				return chain[i], declared[j].Action, true
			}
		}
	}
	return Handle{}, "", false
}
