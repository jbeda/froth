// Bindings and their resolution: how a key press becomes an action, and which
// surface receives it. This is ADR 0008's first two steps; the third — a key
// that matched nothing reaching the focused surface — is in update.go.

package froth

import (
	"unicode"

	tea "charm.land/bubbletea/v2"
)

// An Action is the stable identity of a shortcut, chosen by whoever declares
// it. A surface handles actions rather than keys, so the keys that reach it can
// change without the surface changing.
//
// Identity is a caller's to keep unique. froth compares actions nowhere: it
// carries the one whose binding matched to the surface that declared it, and
// two surfaces using the same string are talking about the same action only if
// they meant to.
type Action string

// The actions froth owns. They name what froth is asked to do rather than what
// an application is: moving focus is froth's job, so the action that asks for it
// is froth's too, and froth binds default keys to the ones it services itself.
//
// An application takes one over by declaring a Binding for it like any other:
// a declaration on the focus chain replaces froth's default keys for that
// action, and a declaration marked Disabled leaves the action with no keys at
// all. There is no second override table.
const (
	// ActionFocusNext moves focus to the next stop in traversal order. froth
	// services it and binds tab to it.
	ActionFocusNext Action = "froth.focus.next"

	// ActionFocusPrev moves focus to the previous stop. froth services it and
	// binds shift+tab to it.
	ActionFocusPrev Action = "froth.focus.prev"

	// ActionCancel asks whatever is in front to back out — dismiss a dialog,
	// abandon an edit. froth owns the identity so surfaces and applications
	// agree on one spelling, and binds no key to it: nothing froth implements
	// services it, so a key bound here would be a key taken from the
	// application for nothing.
	ActionCancel Action = "froth.cancel"
)

// frothBindings is every action froth owns and the keys it binds to each. It is
// one list rather than a set of identities beside a list of keys, so an action
// cannot be owned without being declared. An entry with no keys is an identity
// froth publishes and binds nothing to.
//
// They are matched after every surface on the focus chain, so a surface that
// wants tab for something else simply declares it and wins.
var frothBindings = []Binding{
	{Action: ActionFocusNext, Keys: []string{"tab"}, Help: "move focus to the next surface"},
	{Action: ActionFocusPrev, Keys: []string{"shift+tab"}, Help: "move focus to the previous surface"},
	{Action: ActionCancel, Help: "back out of what is in front"},
}

// frothOwned reports whether an action is one of froth's identities, and so one
// a declaration takes over rather than merely shares a name with.
func frothOwned(a Action) bool {
	for i := range frothBindings {
		if frothBindings[i].Action == a {
			return true
		}
	}
	return false
}

// service performs an action froth implements itself, reporting whether it was
// one. A serviced action is not delivered anywhere: the surface that declared
// the binding asked froth to do the thing, not to be told about it.
//
// It reports true even when nothing moved. The action was froth's either way,
// and a tree with nowhere to tab to is not a key the application should start
// seeing raw.
func (f *Froth) service(a Action) bool {
	var delta int
	switch a {
	case ActionFocusNext:
		delta = 1
	case ActionFocusPrev:
		delta = -1
	default:
		return false
	}
	if to, ok := f.stepFocus(f.stepFrom(), delta); ok {
		f.pendingFocus, f.focusPending = to, true
	}
	return true
}

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
//
// A focused text sink is asked first: it takes typing as characters, so a bare
// printable matches nothing anywhere on the chain, its own bindings included.
//
// froth's own stock bindings are matched last, and only for actions no surface
// on the chain declared. Declaring one is therefore how an application rebinds
// or disables a froth key, through the machinery it already uses for its own.
func (f *Froth) resolve(k tea.KeyPressMsg) (Handle, Action, bool) {
	focused := f.focused()
	if isTyping(k) {
		if n := f.reg.get(focused); n != nil && n.meta.textSink {
			return Handle{}, "", false
		}
	}

	pressed := k.String()
	chain := f.chainTo(focused)
	var taken map[Action]bool
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
			// Any declaration of a froth action takes it over, a disabled one
			// included — which is what makes disabling a froth binding as inert
			// as disabling an application one.
			if frothOwned(declared[j].Action) {
				if taken == nil {
					taken = make(map[Action]bool)
				}
				taken[declared[j].Action] = true
			}
		}
	}

	for i := range frothBindings {
		if taken[frothBindings[i].Action] {
			continue
		}
		if frothBindings[i].matches(pressed) {
			return Handle{}, frothBindings[i].Action, true
		}
	}
	return Handle{}, "", false
}

// chordMods are the modifiers that make a press a chord rather than typing.
// Shift and the lock keys are not among them: they decide which character the
// key produces, not whether it produced one, so a capital and a "?" are typing
// as much as a lower-case letter is.
const chordMods = tea.ModCtrl | tea.ModAlt | tea.ModMeta | tea.ModHyper | tea.ModSuper

// isTyping reports whether a press is a bare printable key: it produced a
// character and no chord modifier was held.
//
// Text is the terminal's answer and is empty for keys that print nothing, so
// enter, tab, escape and the arrows are never typing. Code covers a press froth
// was handed rather than decoded from a terminal, which carries no text.
func isTyping(k tea.KeyPressMsg) bool {
	return k.Mod&chordMods == 0 && (k.Text != "" || unicode.IsPrint(k.Code))
}
