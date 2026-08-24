package froth_test

import (
	"slices"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/jbeda/froth"
)

// speaker declares whatever bindings it was given and draws the last thing that
// reached it — an action's identity, or a raw key — falling back to its own
// letter until something does. Every action and key in these tests is one
// character wide, so a frame reads as "who got what".
type speaker struct {
	letter   string
	bindings []froth.Binding
	last     string

	// takes are the actions this speaker ends the walk for. A speaker given
	// none records what reached it and lets it carry on, which is what silence
	// means: acting on a rising message is not taking it.
	takes []froth.Action
}

func (s *speaker) Bindings() []froth.Binding { return s.bindings }

func (s *speaker) Render(rc *froth.RenderCtx, r froth.Rect) string {
	if s.last != "" {
		return s.last
	}
	return s.letter
}

func (s *speaker) Update(uc *froth.UpdateCtx, msg tea.Msg) {
	switch m := msg.(type) {
	case froth.ActionMsg:
		s.last = string(m.Action)
		if slices.Contains(s.takes, m.Action) {
			uc.StopRising()
		}
	case tea.KeyMsg:
		s.last = m.String()
	}
}

// under puts each child in a one-cell column on the row below its parent's,
// leaving the parent's own first row to draw for itself.
type under struct{}

func (under) Arrange(space froth.Rect, children []froth.Child) []froth.Rect {
	out := make([]froth.Rect, len(children))
	for i := range out {
		out[i] = froth.Rect{X: space.X + i, Y: space.Y + 1, W: 1, H: 1}
	}
	return out
}

// bindRoot is the container the binding tests hang off. It speaks on the top
// row and its children speak on the row below, so one frame says which of them
// a key reached.
type bindRoot struct {
	speaker
	kids    []froth.Surface
	opts    [][]froth.CreateOption
	handles []froth.Handle
}

func (r *bindRoot) Mount(ac *froth.ApplyCtx) {
	for i, k := range r.kids {
		var opts []froth.CreateOption
		if i < len(r.opts) {
			opts = r.opts[i]
		}
		r.handles = append(r.handles, ac.Self().Create(k, opts...))
	}
	ac.Self().SetLayout(under{})
}

func (r *bindRoot) Render(rc *froth.RenderCtx, rect froth.Rect) string {
	return r.speaker.Render(rc, rect) + strings.Repeat(".", rect.W-1) +
		"\n" + strings.Repeat(".", rect.W)
}

func (r *bindRoot) Update(uc *froth.UpdateCtx, msg tea.Msg) {
	if m, ok := msg.(focusMsg); ok {
		uc.At(m.to).Focus()
		return
	}
	r.speaker.Update(uc, msg)
}

// binds is one binding on one key, which is all any of these tests needs.
func binds(k string, a froth.Action) []froth.Binding {
	return []froth.Binding{{Action: a, Keys: []string{k}, Help: string(a)}}
}

// bindTree stands a root over its children and settles the mounts, so the
// handles are real by the time a test focuses one.
func bindTree(t *testing.T, root *bindRoot) *froth.Froth {
	t.Helper()
	f := newSized(t, root, len(root.kids), 2)
	f.View()
	return f
}

func TestAnAncestorsBindingResolvesWhileAChildHasFocus(t *testing.T) {
	left, right := &speaker{letter: "l"}, &speaker{letter: "r"}
	root := &bindRoot{
		speaker: speaker{letter: ".", bindings: binds("q", "Q")},
		kids:    []froth.Surface{left, right},
	}
	f := bindTree(t, root)
	f.Update(focusMsg{to: root.handles[0]})

	f.Update(keyPress('q'))

	if got, want := f.View().Content, "Q.\nQr"; got != want {
		t.Errorf("View().Content = %q, want %q: the key resolves against the "+
			"whole chain and the action then rises through it, so the focused "+
			"surface and the one that declared the binding both see it", got, want)
	}
}

// The declaring surface is not the destination by virtue of having declared it.
// Here the focused surface declares nothing, sees the action first, and ends
// the walk — so the surface whose binding matched never hears about it.
func TestTheFocusedSurfaceSeesAnActionBeforeTheSurfaceThatDeclaredIt(t *testing.T) {
	left := &speaker{letter: "l", takes: []froth.Action{"Q"}}
	root := &bindRoot{
		speaker: speaker{letter: ".", bindings: binds("q", "Q")},
		kids:    []froth.Surface{left, &speaker{letter: "r"}},
	}
	f := bindTree(t, root)
	f.Update(focusMsg{to: root.handles[0]})

	f.Update(keyPress('q'))

	if got, want := f.View().Content, "..\nQr"; got != want {
		t.Errorf("View().Content = %q, want %q", got, want)
	}
	if root.last != "" {
		t.Errorf("the declaring surface saw %q; the walk ended below it", root.last)
	}
}

func TestADeeperBindingBeatsAnAncestorsForTheSameKey(t *testing.T) {
	left, right := &speaker{letter: "l", bindings: binds("x", "L")}, &speaker{letter: "r"}
	root := &bindRoot{
		speaker: speaker{letter: ".", bindings: binds("x", "R")},
		kids:    []froth.Surface{left, right},
	}
	f := bindTree(t, root)
	f.Update(focusMsg{to: root.handles[0]})

	f.Update(keyPress('x'))

	if got, want := f.View().Content, "L.\nLr"; got != want {
		t.Errorf("View().Content = %q, want %q", got, want)
	}
}

func TestADisabledBindingFallsThroughToAnAncestor(t *testing.T) {
	left := &speaker{letter: "l", bindings: binds("x", "L")}
	left.bindings[0].Disabled = true
	root := &bindRoot{
		speaker: speaker{letter: ".", bindings: binds("x", "R")},
		kids:    []froth.Surface{left, &speaker{letter: "r"}},
	}
	f := bindTree(t, root)
	f.Update(focusMsg{to: root.handles[0]})

	f.Update(keyPress('x'))

	if got, want := f.View().Content, "R.\nRr"; got != want {
		t.Errorf("View().Content = %q, want %q", got, want)
	}
}

func TestADisabledBindingWithNothingAboveItLetsTheRawKeyThrough(t *testing.T) {
	left := &speaker{letter: "l", bindings: binds("x", "L")}
	left.bindings[0].Disabled = true
	root := &bindRoot{
		speaker: speaker{letter: "."},
		kids:    []froth.Surface{left, &speaker{letter: "r"}},
	}
	f := bindTree(t, root)
	f.Update(focusMsg{to: root.handles[0]})

	f.Update(keyPress('x'))

	if got, want := f.View().Content, "..\nxr"; got != want {
		t.Errorf("View().Content = %q, want %q", got, want)
	}
}

func TestAnUnboundKeyStillReachesTheFocusedSurface(t *testing.T) {
	root := &bindRoot{
		speaker: speaker{letter: ".", bindings: binds("q", "Q")},
		kids:    []froth.Surface{&speaker{letter: "l"}, &speaker{letter: "r"}},
	}
	f := bindTree(t, root)
	f.Update(focusMsg{to: root.handles[0]})

	f.Update(keyPress('x'))

	if got, want := f.View().Content, "..\nxr"; got != want {
		t.Errorf("View().Content = %q, want %q", got, want)
	}
}

func TestABoundKeyNeverReachesASurfaceThatDeclaresNothing(t *testing.T) {
	left := &focusable{letter: "l"}
	root := &bindRoot{
		speaker: speaker{letter: ".", bindings: binds("q", "Q")},
		kids:    []froth.Surface{left, &focusable{letter: "r"}},
	}
	f := bindTree(t, root)
	f.Update(focusMsg{to: root.handles[0]})

	f.Update(keyPress('q'))
	f.Update(keyPress('x'))

	if got, want := strings.Join(left.keys, ","), "x"; got != want {
		t.Errorf("the focused surface saw %q, want %q: a key that matched a "+
			"binding is not also delivered raw", got, want)
	}
}

func TestBindingsOnOneSurfaceResolveInDeclarationOrder(t *testing.T) {
	left := &speaker{letter: "l", bindings: []froth.Binding{
		{Action: "A", Keys: []string{"x"}, Help: "first"},
		{Action: "B", Keys: []string{"x"}, Help: "second"},
	}}
	root := &bindRoot{
		speaker: speaker{letter: "."},
		kids:    []froth.Surface{left, &speaker{letter: "r"}},
	}
	f := bindTree(t, root)
	f.Update(focusMsg{to: root.handles[0]})

	f.Update(keyPress('x'))

	if got, want := f.View().Content, "A.\nAr"; got != want {
		t.Errorf("View().Content = %q, want %q", got, want)
	}
}

func TestABindingOffTheFocusChainDoesNotFire(t *testing.T) {
	left, right := &speaker{letter: "l"}, &speaker{letter: "r", bindings: binds("x", "R")}
	root := &bindRoot{
		speaker: speaker{letter: "."},
		kids:    []froth.Surface{left, right},
	}
	f := bindTree(t, root)
	f.Update(focusMsg{to: root.handles[0]})

	f.Update(keyPress('x'))

	if got, want := f.View().Content, "..\nxr"; got != want {
		t.Errorf("View().Content = %q, want %q: only the focus chain is "+
			"searched, so a sibling's binding is not in play", got, want)
	}
}

func TestOneBindingCanCarrySeveralKeys(t *testing.T) {
	root := &bindRoot{
		speaker: speaker{letter: ".", bindings: []froth.Binding{
			{Action: "Q", Keys: []string{"q", "ctrl+c"}, Help: "quit"},
		}},
		kids: []froth.Surface{&speaker{letter: "l"}, &speaker{letter: "r"}},
	}
	f := bindTree(t, root)
	f.Update(focusMsg{to: root.handles[0]})

	f.Update(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})

	if got, want := f.View().Content, "Q.\nQr"; got != want {
		t.Errorf("View().Content = %q, want %q", got, want)
	}
}

// Text sinks
//
// sink is the option list for a surface that consumes ordinary typing.
var sink = []froth.CreateOption{froth.TextSink()}

// typed is a key press as a terminal delivers ordinary typing: the character it
// produced, in Text as well as Code.
func typed(r rune) tea.KeyPressMsg { return tea.KeyPressMsg{Code: r, Text: string(r)} }

// sinkTree stands a root that binds bound over a text sink and a plain sibling,
// and gives the sink focus.
func sinkTree(t *testing.T, bound []froth.Binding) (*froth.Froth, *bindRoot) {
	t.Helper()
	root := &bindRoot{
		speaker: speaker{letter: ".", bindings: bound},
		kids:    []froth.Surface{&speaker{letter: "f"}, &speaker{letter: "o"}},
		opts:    [][]froth.CreateOption{sink},
	}
	f := bindTree(t, root)
	f.Update(focusMsg{to: root.handles[0]})
	return f, root
}

func TestABarePrintableKeyReachesAFocusedTextSinkRaw(t *testing.T) {
	f, _ := sinkTree(t, binds("q", "Q"))

	f.Update(typed('q'))

	if got, want := f.View().Content, "..\nqo"; got != want {
		t.Errorf("View().Content = %q, want %q: typing must reach the sink and "+
			"fire nobody's binding", got, want)
	}
}

func TestATextSinkSuppressesItsOwnBindingsToo(t *testing.T) {
	root := &bindRoot{
		speaker: speaker{letter: "."},
		kids: []froth.Surface{
			&speaker{letter: "f", bindings: binds("q", "Q")},
			&speaker{letter: "o"},
		},
		opts: [][]froth.CreateOption{sink},
	}
	f := bindTree(t, root)
	f.Update(focusMsg{to: root.handles[0]})

	f.Update(typed('q'))

	if got, want := f.View().Content, "..\nqo"; got != want {
		t.Errorf("View().Content = %q, want %q: a bare printable matches no "+
			"binding anywhere, the sink's own included", got, want)
	}
}

func TestAPrintableKeyWithNoTextIsStillTyping(t *testing.T) {
	f, _ := sinkTree(t, binds("q", "Q"))

	f.Update(keyPress('q'))

	if got, want := f.View().Content, "..\nqo"; got != want {
		t.Errorf("View().Content = %q, want %q", got, want)
	}
}

func TestAModifiedChordStillResolvesWhileATextSinkHasFocus(t *testing.T) {
	f, _ := sinkTree(t, []froth.Binding{{Action: "Q", Keys: []string{"ctrl+c"}, Help: "quit"}})

	f.Update(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})

	if got, want := f.View().Content, "Q.\nQo"; got != want {
		t.Errorf("View().Content = %q, want %q: a chord is not typing, so an "+
			"ancestor's binding still fires", got, want)
	}
}

// A named key prints nothing, so it is not typing however few modifiers were
// held — a dialog's escape has to keep working while a field holds focus.
func TestKeysThatPrintNothingStillResolveWhileATextSinkHasFocus(t *testing.T) {
	for _, c := range []struct {
		name string
		key  tea.KeyPressMsg
	}{
		{"enter", tea.KeyPressMsg{Code: tea.KeyEnter}},
		{"tab", tea.KeyPressMsg{Code: tea.KeyTab}},
		{"esc", tea.KeyPressMsg{Code: tea.KeyEscape}},
		{"up", tea.KeyPressMsg{Code: tea.KeyUp}},
		{"shift+tab", tea.KeyPressMsg{Code: tea.KeyTab, Mod: tea.ModShift}},
	} {
		t.Run(c.name, func(t *testing.T) {
			f, _ := sinkTree(t, binds(c.name, "K"))

			f.Update(c.key)

			if got, want := f.View().Content, "K.\nKo"; got != want {
				t.Errorf("View().Content = %q, want %q", got, want)
			}
		})
	}
}

// Shift makes a different character, not a chord, so a capital and a "?" are
// ordinary typing and reach the sink.
func TestAShiftedCharacterIsTypingWhileATextSinkHasFocus(t *testing.T) {
	for _, c := range []struct {
		name string
		key  tea.KeyPressMsg
		want string
	}{
		{"capital", tea.KeyPressMsg{Code: 'a', Text: "A", Mod: tea.ModShift}, "..\nAo"},
		{"question", tea.KeyPressMsg{Code: '/', Text: "?", Mod: tea.ModShift}, "..\n?o"},
	} {
		t.Run(c.name, func(t *testing.T) {
			f, _ := sinkTree(t, binds(c.key.String(), "H"))

			f.Update(c.key)

			if got := f.View().Content; got != c.want {
				t.Errorf("View().Content = %q, want %q", got, c.want)
			}
		})
	}
}

func TestASurfaceThatIsNotATextSinkMatchesOrdinaryTyping(t *testing.T) {
	f, root := sinkTree(t, binds("q", "Q"))
	f.Update(focusMsg{to: root.handles[1]})

	f.Update(typed('q'))

	if got, want := f.View().Content, "Q.\nfQ"; got != want {
		t.Errorf("View().Content = %q, want %q", got, want)
	}
}

func TestMovingFocusOffATextSinkRestoresMatching(t *testing.T) {
	f, root := sinkTree(t, binds("q", "Q"))
	f.Update(typed('q'))

	f.Update(focusMsg{to: root.handles[1]})
	f.Update(typed('q'))

	if got, want := f.View().Content, "Q.\nqQ"; got != want {
		t.Errorf("View().Content = %q, want %q: the flag follows focus and "+
			"leaves nothing set behind the move", got, want)
	}
}

// deep is a speaker with a child of its own, for a text sink that sits on the
// focus chain without being its deep end.
type deep struct {
	speaker
	kid   froth.Surface
	child froth.Handle
}

func (d *deep) Mount(ac *froth.ApplyCtx) {
	d.child = ac.Self().Create(d.kid)
	ac.Self().SetLayout(under{})
}

func TestATextSinkAboveTheFocusedSurfaceSuppressesNothing(t *testing.T) {
	leaf := &speaker{letter: "l"}
	mid := &deep{speaker: speaker{letter: "p"}, kid: leaf}
	root := &bindRoot{
		speaker: speaker{letter: ".", bindings: binds("q", "Q")},
		kids:    []froth.Surface{mid},
		opts:    [][]froth.CreateOption{sink},
	}
	f := newSized(t, root, 1, 3)
	f.View()
	f.Update(focusMsg{to: mid.child})

	f.Update(typed('q'))

	if got, want := f.View().Content, "Q\nQ\nQ"; got != want {
		t.Errorf("View().Content = %q, want %q: only the focused surface's own "+
			"flag suppresses matching", got, want)
	}
}

// Space prints a character, so it is typing — even though it is the one key
// whose String() is a name rather than the character it produced. The sink
// draws "space" clipped to its one cell.
func TestSpaceIsTypingWhileATextSinkHasFocus(t *testing.T) {
	f, _ := sinkTree(t, binds("space", "S"))

	f.Update(tea.KeyPressMsg{Code: tea.KeySpace, Text: " "})

	if got, want := f.View().Content, "..\nso"; got != want {
		t.Errorf("View().Content = %q, want %q", got, want)
	}
}

// The shape the example ships: one binding on a bare printable and a chord.
// While the sink has focus the printable is typing and the chord still quits.
func TestABindingOnBothAPrintableAndAChordKeepsTheChordWhileATextSinkHasFocus(t *testing.T) {
	f, _ := sinkTree(t, []froth.Binding{
		{Action: "Q", Keys: []string{"q", "ctrl+c"}, Help: "quit"},
	})

	f.Update(typed('q'))
	if got, want := f.View().Content, "..\nqo"; got != want {
		t.Errorf("View().Content = %q, want %q", got, want)
	}

	f.Update(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})
	if got, want := f.View().Content, "Q.\nQo"; got != want {
		t.Errorf("View().Content = %q, want %q", got, want)
	}
}

// froth's own navigation bindings
//
// Tab and shift-tab resolve to froth-owned actions with froth-owned keys, and
// froth handles them at the tail of the walk, after every surface has had its
// turn. An application takes one over by declaring a Binding for it, which is
// the machinery it already uses for its own — and pre-empts froth's handling by
// taking the message, which is the machinery it already uses for that.

// focusRow is the row the children draw on, which is where these tests read
// focus from. The root draws the last thing that reached it on the row above,
// and a froth action reaching it is what several of them are about — so they
// assert on that separately rather than folding it into one string.
func focusRow(f *froth.Froth) string {
	_, row, _ := strings.Cut(f.View().Content, "\n")
	return row
}

// navRow stands a root over one-cell focusable children, all of them tab stops,
// with whatever bindings the root was given. The root's own row draws the last
// action it received, so a frame says both where focus is and whether anything
// was delivered.
func navRow(t *testing.T, bindings []froth.Binding, letters string) (*froth.Froth, *bindRoot) {
	t.Helper()
	root := &bindRoot{speaker: speaker{letter: ".", bindings: bindings}}
	for _, c := range letters {
		root.kids = append(root.kids, &focusable{letter: string(c)})
		root.opts = append(root.opts, []froth.CreateOption{froth.TabStop()})
	}
	return bindTree(t, root), root
}

func TestTabMovesFocusWithoutTheApplicationBindingAnything(t *testing.T) {
	f, _ := navRow(t, nil, "lr")

	f.Update(tabPress())
	if got, want := focusRow(f), "Lr"; got != want {
		t.Errorf("after tab, the focus row = %q, want %q", got, want)
	}

	f.Update(tabPress())
	if got, want := focusRow(f), "lR"; got != want {
		t.Errorf("after a second tab, the focus row = %q, want %q", got, want)
	}
}

func TestShiftTabMovesFocusTheOtherWay(t *testing.T) {
	f, _ := navRow(t, nil, "lr")

	f.Update(shiftTabPress())

	if got, want := focusRow(f), "lR"; got != want {
		t.Errorf("after shift-tab, the focus row = %q, want %q", got, want)
	}
}

// froth is the tail rather than an interception in front of delivery: the
// action is offered to every surface on the way up, and froth moves focus only
// because nobody took it.
func TestAFrothActionIsOfferedToEverySurfaceBeforeFrothTakesIt(t *testing.T) {
	f, root := navRow(t, []froth.Binding{
		{Action: froth.ActionFocusNext, Keys: []string{"n"}, Help: "next"},
	}, "lr")

	f.Update(keyPress('n'))

	if got, want := focusRow(f), "Lr"; got != want {
		t.Errorf("the focus row = %q, want %q — froth still moved focus", got, want)
	}
	if got, want := root.last, string(froth.ActionFocusNext); got != want {
		t.Errorf("the root saw %q, want %q: declaring a froth action's keys "+
			"and not taking the message is a supported path, and the message "+
			"still arrives", got, want)
	}
}

// Taking it is the whole override mechanism. The root declares one of froth's
// own actions, ends the walk, and froth never gets its turn.
func TestASurfaceThatTakesAFrothActionPreemptsFroth(t *testing.T) {
	f, root := navRow(t, []froth.Binding{
		{Action: froth.ActionFocusNext, Keys: []string{"n"}, Help: "next"},
	}, "lr")
	root.takes = []froth.Action{froth.ActionFocusNext}

	f.Update(keyPress('n'))

	if got, want := focusRow(f), "lr"; got != want {
		t.Errorf("the focus row = %q, want %q — focus should not have moved", got, want)
	}
	if got, want := root.last, string(froth.ActionFocusNext); got != want {
		t.Errorf("the root saw %q, want %q", got, want)
	}
}

// Rebinding is declaring: the declaration takes the action over, so froth's own
// key for it stops matching.
func TestAnApplicationRebindsAFrothNavigationAction(t *testing.T) {
	f, root := navRow(t, []froth.Binding{
		{Action: froth.ActionFocusNext, Keys: []string{"n"}, Help: "next"},
	}, "lr")

	f.Update(tabPress())

	// The key matched nothing anywhere, so it reached the focused surface raw
	// and the root drew it — clipped to the two cells it has.
	if got, want := f.View().Content, "ta\nlr"; got != want {
		t.Errorf("View().Content = %q, want %q — tab is no longer bound", got, want)
	}
	if got := root.last; got != "tab" {
		t.Errorf("the focused surface saw %q raw, want %q", got, "tab")
	}
}

// A disabled froth binding is as inert as a disabled application one: the
// action has no keys at all, and tab is an ordinary key again.
func TestAnApplicationDisablesAFrothNavigationAction(t *testing.T) {
	f, root := navRow(t, []froth.Binding{
		{Action: froth.ActionFocusNext, Disabled: true, Help: "next"},
	}, "lr")

	f.Update(tabPress())

	if got, want := f.View().Content, "ta\nlr"; got != want {
		t.Errorf("View().Content = %q, want %q — focus should not have moved", got, want)
	}
	if got := root.last; got != "tab" {
		t.Errorf("the focused surface saw %q raw, want %q", got, "tab")
	}
	// shift-tab is a separate action and is untouched by disabling this one.
	f.Update(shiftTabPress())
	if got, want := focusRow(f), "lR"; got != want {
		t.Errorf("after shift-tab, the focus row = %q, want %q", got, want)
	}
}

// A surface's own binding on tab wins, because froth's are matched only after
// the whole chain has been asked.
func TestASurfacesOwnBindingOnTabBeatsFroths(t *testing.T) {
	f, _ := navRow(t, []froth.Binding{
		{Action: "T", Keys: []string{"tab"}, Help: "mine"},
	}, "lr")

	f.Update(tabPress())

	if got, want := f.View().Content, "T.\nlr"; got != want {
		t.Errorf("View().Content = %q, want %q", got, want)
	}
}

// Tab prints nothing, so it is not typing and still navigates out of a text
// sink. That is what makes the sink escapable with no key of its own.
func TestTabStillNavigatesWhileATextSinkHasFocus(t *testing.T) {
	root := &bindRoot{
		speaker: speaker{letter: "."},
		kids:    []froth.Surface{&focusable{letter: "l"}, &focusable{letter: "r"}},
		opts: [][]froth.CreateOption{
			{froth.TabStop(), froth.TextSink()},
			{froth.TabStop()},
		},
	}
	f := bindTree(t, root)
	f.Update(focusMsg{to: root.handles[0]})

	f.Update(tabPress())

	if got, want := focusRow(f), "lR"; got != want {
		t.Errorf("the focus row = %q, want %q", got, want)
	}
}

// Invoking without a key
//
// An action can be asked for by identity, which is what a command palette does:
// the same walk a resolved key press gets, minus the resolving.

// invoker asks for actions when a key it was told about arrives raw. Nothing it
// does goes through a binding, so nothing about it depends on what resolves.
type invoker struct {
	speaker
	on   string
	what []froth.Action
}

func (i *invoker) Update(uc *froth.UpdateCtx, msg tea.Msg) {
	if k, ok := msg.(tea.KeyPressMsg); ok && k.String() == i.on {
		for _, a := range i.what {
			uc.Invoke(a)
		}
		return
	}
	i.speaker.Update(uc, msg)
}

func TestAnInvokedActionRisesFromTheActiveSurface(t *testing.T) {
	left := &invoker{speaker: speaker{letter: "l"}, on: "z", what: []froth.Action{"Q"}}
	root := &bindRoot{
		speaker: speaker{letter: ".", bindings: binds("q", "Q"), takes: []froth.Action{"Q"}},
		kids:    []froth.Surface{left, &speaker{letter: "r"}},
	}
	f := bindTree(t, root)
	f.Update(focusMsg{to: root.handles[0]})

	f.Update(keyPress('z'))

	if got, want := f.View().Content, "Q.\nQr"; got != want {
		t.Errorf("View().Content = %q, want %q: an invoked action starts at the "+
			"active surface and climbs, exactly as a resolved key press does",
			got, want)
	}
}

// Disabled takes a binding's keys out of play and says nothing about the
// action, so the key does nothing and the identity still works.
func TestADisabledBindingDoesNotGateAnInvocation(t *testing.T) {
	left := &invoker{speaker: speaker{letter: "l"}, on: "z", what: []froth.Action{"Q"}}
	root := &bindRoot{
		speaker: speaker{letter: ".", takes: []froth.Action{"Q"}, bindings: []froth.Binding{
			{Action: "Q", Keys: []string{"q"}, Help: "quit", Disabled: true},
		}},
		kids: []froth.Surface{left, &speaker{letter: "r"}},
	}
	f := bindTree(t, root)
	f.Update(focusMsg{to: root.handles[0]})

	f.Update(keyPress('q'))
	if got, want := f.View().Content, "..\nqr"; got != want {
		t.Fatalf("after the disabled key, View().Content = %q, want %q", got, want)
	}

	f.Update(keyPress('z'))

	if got, want := f.View().Content, "Q.\nQr"; got != want {
		t.Errorf("View().Content = %q, want %q: disabling is about keys", got, want)
	}
}

// An action nobody takes reaches froth, and froth implements nothing for this
// one, so the walk ends untaken and is reported. One finding per action rather
// than per message type: every action is an ActionMsg, so coalescing on the type
// would report the first and hide the rest.
func TestAnUntakenActionIsReportedOncePerAction(t *testing.T) {
	left := &invoker{
		speaker: speaker{letter: "l"},
		on:      "z",
		what:    []froth.Action{"A", "B", "A"},
	}
	root := &bindRoot{speaker: speaker{letter: "."}, kids: []froth.Surface{left, &speaker{letter: "r"}}}
	f, got := newWatched(t, root, 2, 2)
	f.View()
	f.Update(focusMsg{to: root.handles[0]})

	f.Update(keyPress('z'))

	var actions []string
	for _, d := range *got {
		if d.Kind != froth.DiagUntakenMessage {
			t.Fatalf("diagnostics = %v, want only %q", *got, froth.DiagUntakenMessage)
		}
		actions = append(actions, d.Detail)
	}
	if len(actions) != 2 {
		t.Fatalf("reported %d untaken actions, want 2: %v", len(actions), actions)
	}
	if !strings.Contains(actions[0], `"A"`) || !strings.Contains(actions[1], `"B"`) {
		t.Errorf("details = %v, want the first to name A and the second B", actions)
	}
}
