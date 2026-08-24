package froth_test

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/jbeda/froth"
)

// press builds the key press a binding's key names, for the spellings these
// tests use. It is the inverse of what Bubble Tea renders, which is how a
// Binding spells its keys in the first place.
func press(key string) tea.KeyPressMsg {
	k := tea.KeyPressMsg{}
	for {
		switch {
		case strings.HasPrefix(key, "ctrl+"):
			k.Mod, key = k.Mod|tea.ModCtrl, key[len("ctrl+"):]
		case strings.HasPrefix(key, "shift+"):
			k.Mod, key = k.Mod|tea.ModShift, key[len("shift+"):]
		default:
			switch key {
			case "tab":
				k.Code = tea.KeyTab
			case "left":
				k.Code = tea.KeyLeft
			case "right":
				k.Code = tea.KeyRight
			case "up":
				k.Code = tea.KeyUp
			case "down":
				k.Code = tea.KeyDown
			default:
				k.Code = []rune(key)[0]
			}
			return k
		}
	}
}

// keysOf finds a binding by its action and returns the keys the query says will
// reach it, or nil if it was not reported at all.
func keysOf(live []froth.Binding, a froth.Action) []string {
	for _, b := range live {
		if b.Action == a {
			return b.Keys
		}
	}
	return nil
}

func reports(live []froth.Binding, a froth.Action) bool {
	return slices.ContainsFunc(live, func(b froth.Binding) bool { return b.Action == a })
}

func TestTheQueryDropsAKeyAnotherSurfaceAnswersToFirst(t *testing.T) {
	left := &speaker{letter: "l", bindings: binds("x", "L")}
	root := &bindRoot{
		speaker: speaker{letter: ".", bindings: []froth.Binding{
			{Action: "R", Keys: []string{"x", "q"}, Help: "root"},
		}},
		kids: []froth.Surface{left, &speaker{letter: "r"}},
	}
	f := bindTree(t, root)
	f.Update(focusMsg{to: root.handles[0]})

	live := f.Query().LiveBindings()

	if got, want := keysOf(live, "L"), []string{"x"}; !slices.Equal(got, want) {
		t.Errorf("keys for L = %q, want %q", got, want)
	}
	if got, want := keysOf(live, "R"), []string{"q"}; !slices.Equal(got, want) {
		t.Errorf("keys for R = %q, want %q: x is answered closer to focus, so "+
			"reporting it here would promise something pressing x does not do",
			got, want)
	}
}

func TestEveryKeyTheQueryReportsDoesWhatItSaysItDoes(t *testing.T) {
	left := &speaker{letter: "l", bindings: binds("x", "L")}
	root := &bindRoot{
		speaker: speaker{letter: ".", bindings: []froth.Binding{
			{Action: "R", Keys: []string{"x", "q"}, Help: "root"},
		}},
		kids: []froth.Surface{left, &speaker{letter: "r"}},
	}
	f := bindTree(t, root)
	f.Update(focusMsg{to: root.handles[0]})

	for _, b := range f.Query().LiveBindings() {
		for _, k := range b.Keys {
			t.Run(fmt.Sprintf("%s/%s", b.Action, k), func(t *testing.T) {
				left.last, root.last = "", ""
				f.Update(press(k))
				drawn := f.View().Content

				if strings.HasPrefix(string(b.Action), "froth.") {
					// froth services its own actions and delivers them
					// nowhere, so what is observable is that the key was
					// taken rather than left to the focused surface.
					if strings.Contains(drawn, k) {
						t.Errorf("View().Content = %q, contains the raw key %q: "+
							"the query offered it as a shortcut", drawn, k)
					}
					return
				}
				if !strings.Contains(drawn, string(b.Action)) {
					t.Errorf("View().Content = %q, want it to contain %q: the "+
						"query offered %q for that action", drawn, b.Action, k)
				}
			})
		}
	}
}

func TestADisabledBindingIsReportedWithItsKeysAndStillDoesNotFire(t *testing.T) {
	left := &speaker{letter: "l", bindings: binds("x", "L")}
	left.bindings[0].Disabled = true
	root := &bindRoot{
		speaker: speaker{letter: "."},
		kids:    []froth.Surface{left, &speaker{letter: "r"}},
	}
	f := bindTree(t, root)
	f.Update(focusMsg{to: root.handles[0]})

	live := f.Query().LiveBindings()
	if got, want := keysOf(live, "L"), []string{"x"}; !slices.Equal(got, want) {
		t.Fatalf("keys for L = %q, want %q: a disabled binding keeps its keys "+
			"so help can grey it rather than lose it", got, want)
	}
	i := slices.IndexFunc(live, func(b froth.Binding) bool { return b.Action == "L" })
	if !live[i].Disabled {
		t.Error("the reported binding is not marked Disabled, so nothing tells " +
			"a help display that pressing it does nothing")
	}

	f.Update(press("x"))

	if got, want := f.View().Content, "..\nxr"; got != want {
		t.Errorf("View().Content = %q, want %q: the key reached the focused "+
			"surface raw, which is what a disabled binding means", got, want)
	}
}

func TestATextSinkTakesTypingOutOfTheQueryEverywhere(t *testing.T) {
	field := &speaker{letter: "f"}
	root := &bindRoot{
		speaker: speaker{letter: ".", bindings: []froth.Binding{
			{Action: "Q", Keys: []string{"q", "ctrl+c"}, Help: "quit"},
		}},
		kids: []froth.Surface{field, &speaker{letter: "r"}},
		opts: [][]froth.CreateOption{{froth.TextSink()}},
	}
	f := bindTree(t, root)

	if got, want := keysOf(f.Query().LiveBindings(), "Q"), []string{"q", "ctrl+c"}; !slices.Equal(got, want) {
		t.Errorf("keys for Q with the root focused = %q, want %q", got, want)
	}

	f.Update(focusMsg{to: root.handles[0]})

	if got, want := keysOf(f.Query().LiveBindings(), "Q"), []string{"ctrl+c"}; !slices.Equal(got, want) {
		t.Errorf("keys for Q with the text sink focused = %q, want %q: q is "+
			"typing there, so offering it would promise a quit that types a "+
			"letter instead", got, want)
	}
}

func TestFrothsOwnBindingsAreReportedUntilSomethingTakesThemOver(t *testing.T) {
	root := &bindRoot{
		speaker: speaker{letter: "."},
		kids:    []froth.Surface{&speaker{letter: "l"}, &speaker{letter: "r"}},
	}
	f := bindTree(t, root)

	live := f.Query().LiveBindings()
	if got, want := keysOf(live, froth.ActionFocusNext), []string{"tab"}; !slices.Equal(got, want) {
		t.Errorf("keys for %s = %q, want %q: no surface declares tab and "+
			"pressing it still moves focus", froth.ActionFocusNext, got, want)
	}
	if reports(live, froth.ActionCancel) {
		t.Error("froth.cancel is reported, and froth binds no key to it: a " +
			"help display would offer a shortcut with no key")
	}

	root.bindings = []froth.Binding{
		{Action: froth.ActionFocusNext, Keys: []string{"n"}, Help: "next"},
	}

	if got, want := keysOf(f.Query().LiveBindings(), froth.ActionFocusNext), []string{"n"}; !slices.Equal(got, want) {
		t.Errorf("keys for %s after a declaration = %q, want %q: declaring a "+
			"froth action replaces froth's keys for it", froth.ActionFocusNext, got, want)
	}
}

// asker answers Bindings() by asking froth what the live bindings are, which is
// the one thing a query must not let a surface do.
type asker struct {
	speaker
	q froth.Query
}

func (a *asker) Bindings() []froth.Binding {
	a.q.LiveBindings()
	return a.bindings
}

func TestAQueryAskedFromInsideOneAnswersNothingAndSaysSo(t *testing.T) {
	var got []froth.Diagnostic
	root := &asker{speaker: speaker{letter: ".", bindings: binds("x", "X")}}
	f := froth.New(context.Background(), root,
		froth.OnDiagnostic(func(d froth.Diagnostic) { got = append(got, d) }))
	root.q = f.Query()
	f.Update(tea.WindowSizeMsg{Width: 2, Height: 1})

	f.Update(press("x"))

	if len(got) == 0 || got[0].Kind != froth.DiagReentrantQuery {
		t.Fatalf("diagnostics = %v, want one of kind %q", got, froth.DiagReentrantQuery)
	}
	if want := "X"; f.View().Content != want {
		t.Errorf("View().Content = %q, want %q: the outer resolution should "+
			"still have delivered the action", f.View().Content, want)
	}
}

func TestFocusReportsWhereARawKeyWouldGo(t *testing.T) {
	root := &bindRoot{
		speaker: speaker{letter: "."},
		kids:    []froth.Surface{&speaker{letter: "l"}, &speaker{letter: "r"}},
	}
	f := bindTree(t, root)

	if got, want := f.Query().Focus(), f.Root(); got != want {
		t.Errorf("Focus() = %v, want the root %v", got, want)
	}

	f.Update(focusMsg{to: root.handles[1]})

	if got, want := f.Query().Focus(), root.handles[1]; got != want {
		t.Errorf("Focus() = %v, want %v", got, want)
	}
}

func TestStockBindingsHandsBackACopy(t *testing.T) {
	stock := froth.StockBindings()
	i := slices.IndexFunc(stock, func(b froth.Binding) bool {
		return b.Action == froth.ActionFocusNext
	})
	if i < 0 {
		t.Fatalf("StockBindings() = %v, want it to include %s", stock, froth.ActionFocusNext)
	}
	stock[i].Keys[0] = "z"

	root := &bindRoot{
		speaker: speaker{letter: "."},
		kids:    []froth.Surface{&speaker{letter: "l"}, &speaker{letter: "r"}},
	}
	f := bindTree(t, root)

	if got, want := keysOf(f.Query().LiveBindings(), froth.ActionFocusNext), []string{"tab"}; !slices.Equal(got, want) {
		t.Errorf("keys for %s = %q, want %q: writing to what StockBindings "+
			"returned changed froth's own", froth.ActionFocusNext, got, want)
	}
}

// counter draws how many bindings are live, asked for through the context it is
// rendered with rather than through an instance it was handed.
type counter struct{}

func (counter) Render(rc *froth.RenderCtx, _ froth.Rect) string {
	return fmt.Sprint(len(rc.Query().LiveBindings()))
}
func (counter) Update(*froth.UpdateCtx, tea.Msg) {}

func TestASurfaceAsksThroughItsContext(t *testing.T) {
	root := &bindRoot{
		speaker: speaker{letter: ".", bindings: binds("q", "Q")},
		kids:    []froth.Surface{counter{}, &speaker{letter: "r"}},
	}
	f := bindTree(t, root)

	// The root's own binding plus every froth binding that has a key.
	want := 1
	for _, b := range froth.StockBindings() {
		if len(b.Keys) > 0 {
			want++
		}
	}
	if got := f.View().Content; got != ".."+"\n"+fmt.Sprint(want)+"r" {
		t.Errorf("View().Content = %q, want the counter to draw %d", got, want)
	}
}
