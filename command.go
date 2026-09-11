// Command envelopes: how a command's result finds its way back to the surface
// that ran it.
//
// Bubble Tea hands a command's result to Update with no sender, so froth wraps
// every command Run is given and addresses the result on the command's own
// goroutine. See ADR 0021.

package froth

import (
	"reflect"

	tea "charm.land/bubbletea/v2"
)

// addressedMsg is what a wrapped command returns: an envelope on Bubble Tea's
// channel rather than froth's queue, which route moves across.
type addressedMsg envelope

// The sequence type is unexported, so it is matched and rebuilt through a
// specimen; the loop matches it by concrete type.
var (
	sequenceType = reflect.TypeOf(tea.Sequence(tea.Quit, tea.Quit)())
	cmdsType     = reflect.TypeFor[[]tea.Cmd]()
)

// addressCmd wraps cmd so its result comes back addressed to h. A nil command
// stays nil, which the loop skips.
func addressCmd(h Handle, cmd tea.Cmd) tea.Cmd {
	if cmd == nil {
		return nil
	}
	return func() tea.Msg { return addressResult(h, cmd()) }
}

// addressResult is the wrapper's work: it runs after the command returns, on
// the command's goroutine, and decides what the loop sees. A quit and an
// interrupt are consumed by the loop before Update, so wrapped they would stop
// nothing; a batch and a sequence never reach Update either, so each inner
// command is wrapped and the container rebuilt as the type the loop matches.
func addressResult(h Handle, msg tea.Msg) tea.Msg {
	switch m := msg.(type) {
	case tea.QuitMsg, tea.InterruptMsg:
		return msg
	case tea.BatchMsg:
		return tea.BatchMsg(addressCmds(h, m))
	}
	if v := reflect.ValueOf(msg); v.IsValid() && v.Type() == sequenceType {
		inner := addressCmds(h, v.Convert(cmdsType).Interface().([]tea.Cmd))
		return reflect.ValueOf(inner).Convert(sequenceType).Interface()
	}
	return addressedMsg{to: h, msg: msg}
}

// addressCmds wraps each command in a fresh slice; the one Bubble Tea handed
// over may be shared, and a sequence's is the original's backing array.
func addressCmds(h Handle, cmds []tea.Cmd) []tea.Cmd {
	out := make([]tea.Cmd, len(cmds))
	for i, c := range cmds {
		out[i] = addressCmd(h, c)
	}
	return out
}
