package froth_test

import (
	"fmt"
	"slices"
	"strings"
	"testing"
	"unicode"

	tea "charm.land/bubbletea/v2"
	"github.com/jbeda/froth"
)

// atRects places each child at the rect written for it, so a test states the
// geometry the rule resolves against rather than deriving it from a layout and
// hoping. Directional movement reads rects and nothing else, so this is the
// whole of what a layout contributes to it.
type atRects struct{ rects []froth.Rect }

func (a atRects) Arrange(space froth.Rect, children []froth.Child) []froth.Rect {
	if len(a.rects) != len(children) {
		panic(fmt.Sprintf("atRects: %d rects for %d children", len(a.rects), len(children)))
	}
	return slices.Clone(a.rects)
}

// arrow is one of froth's stock directional keys.
func arrow(code rune) tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: code, Mod: tea.ModShift}
}

// grid builds a root of one-letter stops at the rects given. Every child is a
// tab stop, because a surface that is not one is not a directional target
// either and the tests that care say so themselves.
func grid(t *testing.T, w, h int, letters string, rects ...froth.Rect) (*froth.Froth, *optRoot) {
	t.Helper()
	root := gridRoot(letters, rects)
	f := newSized(t, root, w, h)
	f.View()
	return f, root
}

// gridRoot is grid's tree without the instance, for the tests that have to build
// their own — one that never renders, and one that needs a diagnostic hook.
func gridRoot(letters string, rects []froth.Rect) *optRoot {
	root := &optRoot{lay: atRects{rects: rects}}
	for _, c := range letters {
		root.kids = append(root.kids, &focusable{letter: string(c)})
		root.opts = append(root.opts, stop)
	}
	return root
}

// holder reports which surface has focus, by the letter it is shouting. It reads
// the composited frame rather than asking froth, so a test asserts on what a
// user would see.
func holder(t *testing.T, f *froth.Froth) string {
	t.Helper()
	for _, r := range f.View().Content {
		if unicode.IsUpper(r) {
			return strings.ToLower(string(r))
		}
	}
	return ""
}

// The acceptance demo: four surfaces in two dimensions, one move in each
// direction, and no layout implementing anything.
func TestFocusMovesInEachDirectionBetweenTiledSurfaces(t *testing.T) {
	f, root := grid(t, 5, 3, "abcd",
		froth.Rect{X: 0, Y: 0, W: 2, H: 1}, // a  top left
		froth.Rect{X: 3, Y: 0, W: 2, H: 1}, // b  top right
		froth.Rect{X: 0, Y: 2, W: 2, H: 1}, // c  bottom left
		froth.Rect{X: 3, Y: 2, W: 2, H: 1}, // d  bottom right
	)
	f.Update(focusMsg{to: root.handles[0]})

	steps := []struct {
		key  rune
		want string
	}{
		{tea.KeyRight, "b"},
		{tea.KeyDown, "d"},
		{tea.KeyLeft, "c"},
		{tea.KeyUp, "a"},
	}
	for _, s := range steps {
		f.Update(arrow(s.key))
		if got := holder(t, f); got != s.want {
			t.Errorf("after %q, focus is on %q, want %q", s.key, got, s.want)
		}
	}
}

// A direction with nothing that way is not an error and not a diagnostic. It is
// the same silence as a wall, which is what it is.
func TestADirectionWithNoCandidateLeavesFocusWhereItIs(t *testing.T) {
	root := gridRoot("ab", []froth.Rect{{X: 0, Y: 0, W: 2, H: 1}, {X: 3, Y: 0, W: 2, H: 1}})
	var reported []froth.Diagnostic
	f := froth.New(t.Context(), root,
		froth.OnDiagnostic(func(d froth.Diagnostic) { reported = append(reported, d) }))
	f.Update(tea.WindowSizeMsg{Width: 5, Height: 1})
	f.View()
	f.Update(focusMsg{to: root.handles[0]})

	f.Update(arrow(tea.KeyLeft))
	f.Update(arrow(tea.KeyUp))

	if got := holder(t, f); got != "a" {
		t.Errorf("focus moved to %q, want it to stay on %q", got, "a")
	}
	if len(reported) != 0 {
		t.Errorf("a move with no candidate reported %d diagnostics, want none", len(reported))
	}
}

// The exclusions are traversal's, not a second set: a surface the tab walk
// passes over is not somewhere an arrow key can land either.
func TestASurfaceOutsideTraversalIsNotADirectionalTarget(t *testing.T) {
	root := &optRoot{
		kids: []froth.Surface{
			&focusable{letter: "a"}, &focusable{letter: "n"}, &focusable{letter: "b"},
		},
		opts: [][]froth.CreateOption{stop, nil, stop},
		lay: atRects{rects: []froth.Rect{
			{X: 0, Y: 0, W: 1, H: 1}, // a
			{X: 2, Y: 0, W: 1, H: 1}, // n  nearer, but not a stop
			{X: 4, Y: 0, W: 1, H: 1}, // b
		}},
	}
	f := newSized(t, root, 5, 1)
	f.View()
	f.Update(focusMsg{to: root.handles[0]})

	f.Update(arrow(tea.KeyRight))

	if got := holder(t, f); got != "b" {
		t.Errorf("focus moved to %q, want the nearest stop %q", got, "b")
	}
}

// A scope bounds an arrow key exactly as it bounds tab, and the wall is silent
// in both directions: the geometrically obvious target is simply not a candidate.
func TestAFocusScopeBoundsDirectionalMovement(t *testing.T) {
	f, root, _ := scopedRow(t, scope)

	// From outside, the scope's contents are not candidates: right from "a"
	// steps over "b" and "c" and lands on "d".
	f.Update(focusMsg{to: root.handles[0]})
	f.Update(arrow(tea.KeyRight))
	if got := holder(t, f); got != "d" {
		t.Errorf("from outside the scope, focus moved to %q, want %q", got, "d")
	}

	// From inside, the walk reaches the scope's own stops and stops there.
	f.Update(focusMsg{to: root.handles[1]})
	if got := holder(t, f); got != "b" {
		t.Fatalf("entering the scope landed on %q, want %q", got, "b")
	}
	f.Update(arrow(tea.KeyRight))
	if got := holder(t, f); got != "c" {
		t.Errorf("inside the scope, focus moved to %q, want %q", got, "c")
	}
	f.Update(arrow(tea.KeyRight))
	if got := holder(t, f); got != "c" {
		t.Errorf("at the scope's edge, focus moved to %q, want it to stay on %q", got, "c")
	}
}

// There is no geometry before the first frame, so there is nothing to resolve
// against. The criterion is that this is defined rather than a panic.
func TestADirectionalMoveBeforeTheFirstFrameDoesNothing(t *testing.T) {
	root := gridRoot("ab", []froth.Rect{{X: 0, Y: 0, W: 1, H: 1}, {X: 2, Y: 0, W: 1, H: 1}})
	f := froth.New(t.Context(), root)
	f.Update(tea.WindowSizeMsg{Width: 3, Height: 1})

	// No View has been called, so nothing has been placed.
	f.Update(arrow(tea.KeyRight))

	if got := holder(t, f); got != "" {
		t.Errorf("focus moved to %q before the first frame, want it unmoved", got)
	}
}

// froth's directional keys are rebound through the machinery an application
// already uses for its own, which is what makes bare arrows a four-line change
// for the TUIs that need them.
func TestTheDirectionalKeysAreRebindableLikeAnyOther(t *testing.T) {
	root := &rebinder{optRoot: *gridRoot("ab",
		[]froth.Rect{{X: 0, Y: 0, W: 1, H: 1}, {X: 2, Y: 0, W: 1, H: 1}})}
	f := newSized(t, root, 3, 1)
	f.View()
	f.Update(focusMsg{to: root.handles[0]})

	// The stock key no longer moves anything: a declaration takes the action
	// over rather than adding to it.
	f.Update(arrow(tea.KeyRight))
	if got := holder(t, f); got != "a" {
		t.Errorf("shift+right moved focus to %q after being rebound, want %q", got, "a")
	}

	f.Update(tea.KeyPressMsg{Code: tea.KeyRight})
	if got := holder(t, f); got != "b" {
		t.Errorf("the rebound bare arrow moved focus to %q, want %q", got, "b")
	}
}

// rebinder is a root that takes froth's rightward move over and binds the bare
// arrow key to it.
type rebinder struct{ optRoot }

func (r *rebinder) Bindings() []froth.Binding {
	return []froth.Binding{{
		Action: froth.ActionFocusRight,
		Keys:   []string{"right"},
		Help:   "move focus right",
	}}
}
