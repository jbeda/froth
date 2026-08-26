// Bindings and their resolution: how a key press becomes an action. Who acts on
// it is not decided here — delivery is a separate walk, in update.go, and so is
// the raw key that reaches the focused surface when nothing matched.

package froth

import (
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"
)

// An Action is the stable identity of something an application can be asked to
// do, chosen by whoever declares it. A key binding is one way to ask and not the
// only one: Invoke takes an identity and no key.
//
// Identity is a caller's to keep unique. froth compares actions only against
// the ones it owns itself, so two surfaces using the same string are talking
// about the same action only if they meant to.
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

// service is froth's turn at the tail of a rising walk: an action that climbed
// past every surface with nobody taking it, and that froth implements itself.
// It reports whether froth took it, which is what keeps an action froth handled
// out of the untaken diagnostic.
//
// Being last is what lets an application pre-empt froth without an override
// table — taking the message is the whole mechanism. Anything other than an
// action froth implements is not froth's, so it reports false and the caller
// reports the walk untaken.
//
// It reports true even when nothing moved. The action was froth's either way,
// and a tree with nowhere to tab to is not a key the application should start
// seeing raw.
func (f *Froth) service(msg tea.Msg) bool {
	m, ok := msg.(ActionMsg)
	if !ok {
		return false
	}
	from := f.stepFrom()
	var to Handle
	var moved bool
	switch m.Action {
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

// An ActionMsg carries an action to whoever will act on it. It rises: froth
// offers it to the active surface, then to that surface's parent, and on up
// through the root to froth itself, which is where the actions froth implements
// are handled. The first surface to call StopRising ends the walk.
//
// Declaring the binding that produced it is not how a surface receives it.
// Resolution answers which action a key means here and nothing else, so an
// action can also arrive with no key pressed at all — see UpdateCtx.Invoke.
type ActionMsg struct {
	Rising
	Action Action
}

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
	// still see it. Disabling is how a shortcut becomes context-sensitive — and
	// it is the field most likely to go stale, because nothing recomputes it: a
	// surface whose state moves on declares its bindings again.
	//
	// It is phrased as the exception so the zero Binding is one that works.
	Disabled bool

	// SurvivesModal keeps a binding resolving from outside an open modal, which
	// otherwise stops resolution at itself. froth declares none of these: what
	// an application must be able to do from inside a modal is the
	// application's to decide.
	SurvivesModal bool
}

// A ChainBinding is one declaration as it stands on the focus chain: the
// binding, and the surface that declared it.
//
// Keys is narrowed to what the question being asked cares about, so the same
// declaration reports different keys through Query.LiveBindings and
// Query.Actions. See each of them.
type ChainBinding struct {
	Binding

	// From is the surface that declared this binding. It is the zero Handle for
	// a stock binding, which no surface declared.
	//
	// It says where the declaration came from and nothing about where the
	// action goes: an ActionMsg rises from the active surface, so the declaring
	// surface receives it only by being on that path.
	From Handle
}

// A chainEntry is one declaration gathered by the walk, with the two narrowings
// of its keys the two views need. Both are computed in the one walk, because a
// second walk is how help and dispatch start disagreeing.
type chainEntry struct {
	ChainBinding

	// live is Keys with the shadowed ones removed and, while a text sink holds
	// focus, the typing ones too: exactly what would fire right now. Empty means
	// the declaration cannot fire.
	live []string

	// listed is Keys with only the shadowed ones removed. See ADR 0008 for why
	// a text sink narrows one of these and not the other.
	listed []string
}

// setBindings replaces the surface's declarations with the ones given. Alone
// among the things a surface asks for on itself during Deliver it takes effect
// at once rather than at the next Apply: nothing structural changes, and the
// key press being handled resolved before the pass began, so there is no
// ordering a delay would protect. What can see the difference is a query asked
// later in the same settle pass, which is the behaviour pulling already had.
//
// The set is copied, so a surface may keep and reuse the slice it passed.
func (r router) setBindings(bs []Binding) {
	r.check()
	n := r.f.reg.get(r.h)
	if n == nil {
		return
	}
	n.bindings = cloneBindings(bs)
}

// cloneBindings copies a declaration set deeply enough that nothing a caller
// still holds can change what froth stored. Keys is the only slice inside a
// Binding. StockBindings hands out froth's own declarations and needs exactly
// the same depth, so it is the other caller.
func cloneBindings(bs []Binding) []Binding {
	if len(bs) == 0 {
		return nil
	}
	out := slices.Clone(bs)
	for i := range out {
		out[i].Keys = slices.Clone(out[i].Keys)
	}
	return out
}

// chainBindings walks the focus chain deepest first and gathers the bindings
// currently in effect, in the order resolution consults them. Resolution, the
// help query and the action listing are all consumers of it, which is what makes
// it impossible for them to describe different worlds.
//
// The chain is walked from the focused surface rather than from f.focus, so
// resolution and a raw key agree about where the deep end is when focus names
// nothing. froth's own bindings come last, and only for actions no surface on
// the chain declared: declaring one is how an application rebinds or disables a
// froth key, through the machinery it already uses for its own.
//
// This is the only place the chain is decided. Anything that narrows it — a
// surface taking input for a whole subtree, so that what is above it stops
// contributing — narrows it here, or the consumers stop agreeing.
func (f *Froth) chainBindings() []chainEntry {
	focused := f.focused()
	var sink bool
	if n := f.reg.get(focused); n != nil {
		sink = n.meta.textSink
	}

	// Two sets of claims rather than one: a key a text sink takes as typing is
	// not claimed by anybody, so what shadows what differs between the views
	// whenever a sink holds focus.
	live, listed := make(map[string]bool), make(map[string]bool)
	var out []chainEntry
	var taken map[Action]bool

	add := func(d Binding, from Handle) {
		out = append(out, chainEntry{
			ChainBinding: ChainBinding{Binding: d, From: from},
			live:         narrow(&d, live, sink),
			listed:       narrow(&d, listed, false),
		})
	}

	chain := f.chainTo(focused)
	for i := len(chain) - 1; i >= 0; i-- {
		n := f.reg.get(chain[i])
		if n == nil {
			continue
		}
		if n.panicked {
			continue
		}
		for _, d := range n.bindings {
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
			add(d, chain[i])
		}
	}

	for i := range frothBindings {
		if taken[frothBindings[i].Action] {
			continue
		}
		add(frothBindings[i], Handle{})
	}
	return out
}

// narrow reports the keys of a declaration that would actually arrive at it, and
// takes those keys out of play for everything behind it. Two things take a key
// away: one closer to focus already answers to it, and — where the caller asks
// for it — ordinary typing while a text sink holds focus, which is never matched
// as a shortcut anywhere on the chain.
//
// An enabled binding claims the keys it keeps, which is what shadows the ones
// behind it. A disabled one claims nothing, because it does not stop the key
// resolving further up either.
func narrow(d *Binding, claimed map[string]bool, sink bool) []string {
	var keys []string
	for _, k := range d.Keys {
		if claimed[k] || (sink && keyIsTyping(k)) {
			continue
		}
		keys = append(keys, k)
	}
	if !d.Disabled {
		for _, k := range keys {
			claimed[k] = true
		}
	}
	return keys
}

// resolve matches a key press against the bindings currently in effect and
// reports the action it means here. Shadowing has already been settled by the
// walk, so the first entry listing the key is the winner by construction.
//
// It answers what the key means and not who acts on it. Those are two questions
// and delivery settles the second: an ActionMsg rises from the active surface,
// so the surface that declared the binding is not the destination by virtue of
// having declared it.
func (f *Froth) resolve(k tea.KeyPressMsg) (Action, bool) {
	pressed := k.String()
	collected := f.chainBindings()
	for i := range collected {
		if collected[i].Disabled {
			continue
		}
		if slices.Contains(collected[i].live, pressed) {
			return collected[i].Action, true
		}
	}
	return "", false
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
