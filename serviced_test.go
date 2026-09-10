package froth_test

import (
	"testing"

	"github.com/jbeda/froth"
)

// froth's published actions divide into the ones it services and the one it
// merely publishes: ActionCancel has an identity and no implementation. Two
// things turn on that line, and both are observable from outside. An action
// froth services is taken at the tail of a rising walk, so invoking it is never
// reported untaken; and a declaration of it is exempt from modality, so it
// resolves behind an open modal. An action froth only publishes gets neither.
//
// The two are asserted together, over the published set, so that adding a
// serviced action to one of froth's lists and not the other fails here.
func TestEveryStockActionIsServicedOrPublishedOnBothCounts(t *testing.T) {
	for _, b := range froth.StockBindings() {
		serviced := b.Action != froth.ActionCancel
		t.Run(string(b.Action), func(t *testing.T) {
			declared := []froth.Binding{{Action: b.Action, Keys: []string{"c"}}}

			// Untaken: the root declares the action on a key and does not
			// take it, so it rises to froth.
			plain := &modalNest{speaker: speaker{letter: ".", bindings: declared}}
			f, diags := newWatched(t, plain, 2, 2)
			f.View()
			f.Update(keyPress('c'))
			if untaken := len(*diags) == 1 && (*diags)[0].Kind == froth.DiagUntakenMessage; untaken == serviced {
				t.Errorf("invoking it reported %v; want untaken = %v", *diags, !serviced)
			}

			// Exemption: the same declaration behind a modal resolves only
			// if froth services the action.
			dlg := &speaker{letter: "d"}
			blocked := &modalNest{
				speaker: speaker{letter: ".", bindings: declared},
				kids:    []nestKid{{surface: dlg, opts: modal}},
			}
			g := modalTree(t, blocked, 2, 2)
			g.Update(focusMsg{to: blocked.handles[0]})
			g.Update(keyPress('c'))
			if resolved := dlg.last != "c"; resolved != serviced {
				t.Errorf("behind a modal the declaration resolved = %v, want %v", resolved, serviced)
			}
		})
	}
}
