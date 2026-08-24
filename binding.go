// Bindings and their resolution: how a key press becomes an action, and which
// surface receives it. This is ADR 0008's first two steps; the third — a key
// that matched nothing reaching the focused surface — is in update.go.

package froth

import (
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

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

	// ActionFocusLeft, ActionFocusRight, ActionFocusUp and ActionFocusDown move
	// focus geometrically, to the surface that way in the frame last drawn.
	// froth services them and binds shift with the arrow keys. Bare arrows
	// belong to whatever holds focus — a list's selection, a table's row — and
	// a surface declaring them already wins over these, so froth taking them
	// would leave the gesture working only where there is least to navigate.
	// An application that wants the bare keys declares these actions with them,
	// the same way it rebinds tab.
	ActionFocusLeft  Action = "froth.focus.left"
	ActionFocusRight Action = "froth.focus.right"
	ActionFocusUp    Action = "froth.focus.up"
	ActionFocusDown  Action = "froth.focus.down"

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
	{Action: ActionFocusLeft, Keys: []string{"shift+left"}, Help: "move focus left"},
	{Action: ActionFocusRight, Keys: []string{"shift+right"}, Help: "move focus right"},
	{Action: ActionFocusUp, Keys: []string{"shift+up"}, Help: "move focus up"},
	{Action: ActionFocusDown, Keys: []string{"shift+down"}, Help: "move focus down"},
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
	from := f.stepFrom()
	var to Handle
	var moved bool
	switch a {
	case ActionFocusNext:
		to, moved = f.stepFocus(from, 1)
	case ActionFocusPrev:
		to, moved = f.stepFocus(from, -1)
	case ActionFocusLeft:
		to, moved = f.stepDirection(from, dirLeft)
	case ActionFocusRight:
		to, moved = f.stepDirection(from, dirRight)
	case ActionFocusUp:
		to, moved = f.stepDirection(from, dirUp)
	case ActionFocusDown:
		to, moved = f.stepDirection(from, dirDown)
	default:
		return false
	}
	if moved {
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
// while resolving a key press and while answering a query, which is a
// synchronous call into a surface — rare enough in froth to be worth naming.
//
// Bindings must therefore be a read-only answer about the surface as it stands.
// It may be called from any phase, more than once, and about a surface other
// than the one froth is currently talking to.
type Binder interface {
	Bindings() []Binding
}

// A liveBinding is one entry of the collected chain: a binding as it will
// actually be seen, and the surface that declared it. A zero handle is froth's
// own declaration, which no surface made.
type liveBinding struct {
	Binding
	from Handle
}

// liveBindings walks the focus chain deepest first and collects the bindings
// currently in effect, in the order resolution consults them. Resolution and the
// help query are both consumers of it, which is what makes it impossible for
// help to describe something other than what a key press does.
//
// The chain is walked from the focused surface rather than from f.focus, so
// resolution and a raw key agree about where the deep end is when focus names
// nothing. froth's own bindings come last, and only for actions no surface on
// the chain declared: declaring one is how an application rebinds or disables a
// froth key, through the machinery it already uses for its own.
//
// This is the only place the chain is decided. Anything that narrows it — a
// surface taking input for a whole subtree, so that what is above it stops
// contributing — narrows it here, or the two consumers stop agreeing.
func (f *Froth) liveBindings() []liveBinding {
	if f.querying {
		f.report(Diagnostic{
			Kind: DiagReentrantQuery,
			Detail: "a query was asked from inside one, which would not " +
				"terminate; the nested query answered nothing. A surface " +
				"must answer Bindings() from its own state rather than by " +
				"asking froth",
		})
		return nil
	}
	f.querying = true
	defer func() { f.querying = false }()

	focused := f.focused()
	var sink bool
	if n := f.reg.get(focused); n != nil {
		sink = n.meta.textSink
	}

	claimed := make(map[string]bool)
	var out []liveBinding
	var taken map[Action]bool

	chain := f.chainTo(focused)
	for i := len(chain) - 1; i >= 0; i-- {
		n := f.reg.get(chain[i])
		if n == nil {
			continue
		}
		b, ok := n.surface.(Binder)
		if !ok {
			continue
		}
		for _, d := range b.Bindings() {
			// Any declaration of a froth action takes it over, a disabled one
			// included — which is what makes disabling a froth binding as inert
			// as disabling an application one. It counts even when the
			// declaration keeps no live key of its own.
			if frothOwned(d.Action) {
				if taken == nil {
					taken = make(map[Action]bool)
				}
				taken[d.Action] = true
			}
			if lb, ok := claim(&d, chain[i], claimed, sink); ok {
				out = append(out, lb)
			}
		}
	}

	for i := range frothBindings {
		if taken[frothBindings[i].Action] {
			continue
		}
		if lb, ok := claim(&frothBindings[i], Handle{}, claimed, sink); ok {
			out = append(out, lb)
		}
	}
	return out
}

// claim narrows a declaration to the keys that would actually arrive at it,
// takes those keys out of play for everything behind it, and reports whether any
// survived. Two things take a key away: one closer to
// focus already answers to it, and ordinary typing while a text sink holds focus,
// which is never matched as a shortcut anywhere on the chain.
//
// An enabled binding claims the keys it keeps, which is what shadows the ones
// behind it. A disabled one claims nothing, because it does not stop the key
// resolving further up either — it is reported with its keys intact so help can
// grey it rather than lose it.
//
// A binding left with no keys is dropped rather than reported empty, so nothing
// that cannot fire is ever offered. That is also what drops the froth actions
// bound to no key at all.
func claim(d *Binding, from Handle, claimed map[string]bool, sink bool) (liveBinding, bool) {
	var keys []string
	for _, k := range d.Keys {
		if claimed[k] || (sink && keyIsTyping(k)) {
			continue
		}
		keys = append(keys, k)
	}
	if len(keys) == 0 {
		return liveBinding{}, false
	}
	if !d.Disabled {
		for _, k := range keys {
			claimed[k] = true
		}
	}
	out := *d
	out.Keys = keys
	return liveBinding{Binding: out, from: from}, true
}

// resolve matches a key press against the bindings currently in effect and
// reports the surface that declared the winning one along with its action.
// Shadowing has already been settled by the walk, so the first entry listing the
// key is the winner by construction.
func (f *Froth) resolve(k tea.KeyPressMsg) (Handle, Action, bool) {
	pressed := k.String()
	for _, lb := range f.liveBindings() {
		if lb.Disabled {
			continue
		}
		if slices.Contains(lb.Keys, pressed) {
			return lb.from, lb.Action, true
		}
	}
	return Handle{}, "", false
}

// keyMods are the modifier spellings a key can carry, in the order Bubble Tea
// writes them. Shift and the lock keys are not chords: they decide which
// character the key produces, not whether it produced one, so a capital and a
// "?" are typing as much as a lower-case letter is.
var keyMods = []struct {
	prefix string
	chord  bool
}{
	{"ctrl+", true},
	{"alt+", true},
	{"shift+", false},
	{"meta+", true},
	{"hyper+", true},
	{"super+", true},
}

// keyIsTyping reports whether a key, spelled the way a Binding spells it, is one
// a press produces by typing a character rather than by striking a chord. A text
// sink takes those as text, so they match no shortcut while it holds focus.
//
// It reads the spelling because that is all a declaration has: help is asked
// about keys nobody has pressed. Bubble Tea renders a typing press as its text,
// so what is left after the modifiers is a single printable rune — or the word
// "space", the one invisible character it spells out.
func keyIsTyping(key string) bool {
	for _, m := range keyMods {
		rest, ok := strings.CutPrefix(key, m.prefix)
		if !ok {
			continue
		}
		if m.chord {
			return false
		}
		key = rest
	}
	if key == "space" {
		return true
	}
	r, n := utf8.DecodeRuneInString(key)
	return n == len(key) && unicode.IsPrint(r)
}
