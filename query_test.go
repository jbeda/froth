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
func keysOf(live []froth.ChainBinding, a froth.Action) []string {
	for _, b := range live {
		if b.Action == a {
			return b.Keys
		}
	}
	return nil
}

func reports(live []froth.ChainBinding, a froth.Action) bool {
	return slices.ContainsFunc(live, func(b froth.ChainBinding) bool { return b.Action == a })
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
					// A froth action rises like any other and the speakers
					// here draw its identity, but what this test is about is
					// that the key resolved at all rather than reaching the
					// focused surface raw.
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
	i := slices.IndexFunc(live, func(b froth.ChainBinding) bool { return b.Action == "L" })
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

// The action listing
//
// The same declarations, asked what can be invoked here rather than what a key
// would do. Two of the key-shaped rules are wrong for that question and are not
// applied; the third, shadowing, is right for both.

func TestTheListingKeepsAnActionNobodyBoundAKeyTo(t *testing.T) {
	root := &bindRoot{
		speaker: speaker{letter: ".", bindings: []froth.Binding{
			{Action: "K", Help: "keyless"},
		}},
		kids: []froth.Surface{&speaker{letter: "l"}, &speaker{letter: "r"}},
	}
	f := bindTree(t, root)

	actions := f.Query().Actions()

	if !reports(actions, "K") {
		t.Error("a declaration with no keys is missing from the listing, so " +
			"nothing can offer it")
	}
	if !reports(actions, froth.ActionCancel) {
		t.Errorf("%s is missing, and froth binds no key to it: the identity "+
			"froth publishes for backing out is unofferable", froth.ActionCancel)
	}
	if reports(f.Query().LiveBindings(), "K") {
		t.Error("a keyless declaration is offered as a shortcut, which promises " +
			"a key that does not exist")
	}
}

func TestATextSinkLeavesTheListingAlone(t *testing.T) {
	field := &speaker{letter: "f"}
	root := &bindRoot{
		speaker: speaker{letter: ".", bindings: []froth.Binding{
			{Action: "Q", Keys: []string{"q", "ctrl+c"}, Help: "quit"},
		}},
		kids: []froth.Surface{field, &speaker{letter: "r"}},
		opts: [][]froth.CreateOption{{froth.TextSink()}},
	}
	f := bindTree(t, root)
	f.Update(focusMsg{to: root.handles[0]})

	if got, want := keysOf(f.Query().Actions(), "Q"), []string{"q", "ctrl+c"}; !slices.Equal(got, want) {
		t.Errorf("keys for Q in the listing = %q, want %q: a palette's own "+
			"filter field is a text sink, so a listing that lost typing would "+
			"empty itself the moment it opened", got, want)
	}
	if got, want := keysOf(f.Query().LiveBindings(), "Q"), []string{"ctrl+c"}; !slices.Equal(got, want) {
		t.Errorf("keys for Q as a shortcut = %q, want %q", got, want)
	}
}

// Shadowing is the one narrowing both views share: a key another surface
// answers to first genuinely means something else here.
func TestTheListingDropsAKeyAnotherSurfaceAnswersToFirst(t *testing.T) {
	left := &speaker{letter: "l", bindings: binds("x", "L")}
	root := &bindRoot{
		speaker: speaker{letter: ".", bindings: []froth.Binding{
			{Action: "R", Keys: []string{"x", "q"}, Help: "root"},
		}},
		kids: []froth.Surface{left, &speaker{letter: "r"}},
	}
	f := bindTree(t, root)
	f.Update(focusMsg{to: root.handles[0]})

	actions := f.Query().Actions()

	if got, want := keysOf(actions, "R"), []string{"q"}; !slices.Equal(got, want) {
		t.Errorf("keys for R = %q, want %q", got, want)
	}
	if !reports(actions, "R") {
		t.Error("R is missing entirely, and it is still invocable")
	}
}

func TestEachListedActionNamesTheSurfaceThatDeclaredIt(t *testing.T) {
	left := &speaker{letter: "l", bindings: binds("x", "L")}
	root := &bindRoot{
		speaker: speaker{letter: ".", bindings: binds("q", "Q")},
		kids:    []froth.Surface{left, &speaker{letter: "r"}},
	}
	f := bindTree(t, root)
	f.Update(focusMsg{to: root.handles[0]})

	from := map[froth.Action]froth.Handle{}
	for _, a := range f.Query().Actions() {
		from[a.Action] = a.From
	}
	if got, want := from["L"], root.handles[0]; got != want {
		t.Errorf("L was declared by %v, want %v", got, want)
	}
	if got, want := from["Q"], f.Root(); got != want {
		t.Errorf("Q was declared by %v, want %v", got, want)
	}
	if got := from[froth.ActionFocusNext]; got != (froth.Handle{}) {
		t.Errorf("%s was declared by %v, want the zero handle: it is froth's "+
			"own and no surface declared it", froth.ActionFocusNext, got)
	}
}

// One set of declarations, two questions. Anything a key press would fire is
// something the listing offers, or help and a palette describe different worlds.
func TestEveryLiveBindingIsAlsoListedAsAnAction(t *testing.T) {
	left := &speaker{letter: "l", bindings: binds("x", "L")}
	root := &bindRoot{
		speaker: speaker{letter: ".", bindings: []froth.Binding{
			{Action: "R", Keys: []string{"x", "q"}, Help: "root"},
			{Action: "D", Keys: []string{"d"}, Help: "off", Disabled: true},
		}},
		kids: []froth.Surface{left, &speaker{letter: "r"}},
		opts: [][]froth.CreateOption{{froth.TextSink()}},
	}
	f := bindTree(t, root)
	f.Update(focusMsg{to: root.handles[0]})

	actions := f.Query().Actions()
	for _, b := range f.Query().LiveBindings() {
		if !reports(actions, b.Action) {
			t.Errorf("%s resolves from a key and is not in the listing", b.Action)
		}
	}
	if !reports(actions, "D") {
		t.Error("a disabled declaration is missing from the listing: an " +
			"application cannot offer it greyed and decide for itself")
	}
}

// counting is what makes "one set of declarations" checkable: a surface asked
// twice could answer differently, and the two views would drift with nothing to
// catch it.
type counting struct {
	speaker
	asked int
}

func (c *counting) Bindings() []froth.Binding {
	c.asked++
	return c.bindings
}

func TestOneViewAsksEachSurfaceForItsBindingsExactlyOnce(t *testing.T) {
	left := &counting{speaker: speaker{letter: "l", bindings: binds("x", "L")}}
	root := &bindRoot{
		speaker: speaker{letter: ".", bindings: binds("q", "Q")},
		kids:    []froth.Surface{left, &speaker{letter: "r"}},
	}
	f := bindTree(t, root)
	f.Update(focusMsg{to: root.handles[0]})

	for _, c := range []struct {
		name string
		ask  func()
	}{
		{"LiveBindings", func() { f.Query().LiveBindings() }},
		{"Actions", func() { f.Query().Actions() }},
		{"a key press", func() { f.Update(press("x")) }},
	} {
		t.Run(c.name, func(t *testing.T) {
			left.asked = 0
			c.ask()
			if left.asked != 1 {
				t.Errorf("Bindings() was asked %d times, want 1: both views and "+
					"resolution read one walk, and a second walk is how they "+
					"start describing different worlds", left.asked)
			}
		})
	}
}

// A key every one of whose declarations is shadowed leaves the one behind it
// with nothing, and the listing says so rather than inventing a key. It reads
// the same as a keyless declaration because it amounts to the same thing.
func TestAFullyShadowedDeclarationIsListedWithNoKeys(t *testing.T) {
	left := &speaker{letter: "l", bindings: binds("x", "L")}
	root := &bindRoot{
		speaker: speaker{letter: ".", bindings: binds("x", "R")},
		kids:    []froth.Surface{left, &speaker{letter: "r"}},
	}
	f := bindTree(t, root)
	f.Update(focusMsg{to: root.handles[0]})

	actions := f.Query().Actions()

	if !reports(actions, "R") {
		t.Fatal("a declaration whose only key is shadowed is missing from the " +
			"listing, and it is still invocable by identity")
	}
	if got := keysOf(actions, "R"); len(got) != 0 {
		t.Errorf("keys for R = %q, want none: x means L here", got)
	}
	if reports(f.Query().LiveBindings(), "R") {
		t.Error("R is offered as a shortcut with no key that reaches it")
	}
}
