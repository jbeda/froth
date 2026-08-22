package layout_test

import (
	"testing"

	"github.com/jbeda/froth"
	"github.com/jbeda/froth/layout"
)

// children builds the argument a layout is handed. A layout sees handles and
// nothing else, so a test needs no surfaces and no froth instance: this is the
// whole input.
func children(n int) []froth.Child {
	c := make([]froth.Child, n)
	return c
}

func TestRowDividesSpaceEvenly(t *testing.T) {
	got := layout.Row().Arrange(froth.Rect{X: 0, Y: 0, W: 12, H: 4}, children(3))

	want := []froth.Rect{
		{X: 0, Y: 0, W: 4, H: 4},
		{X: 4, Y: 0, W: 4, H: 4},
		{X: 8, Y: 0, W: 4, H: 4},
	}
	assertRects(t, got, want)
}

// The row starts at the space it was given, not at the screen origin: a nested
// container arranges inside its own rect.
func TestRowArrangesWithinTheSpaceItIsGiven(t *testing.T) {
	got := layout.Row().Arrange(froth.Rect{X: 5, Y: 2, W: 4, H: 3}, children(2))

	want := []froth.Rect{
		{X: 5, Y: 2, W: 2, H: 3},
		{X: 7, Y: 2, W: 2, H: 3},
	}
	assertRects(t, got, want)
}

// Cells do not divide evenly and cannot be split, so the remainder goes to the
// leftmost children — one cell each, never two to one child while another
// starves.
func TestRowGivesTheRemainderToTheLeftmostChildren(t *testing.T) {
	got := layout.Row().Arrange(froth.Rect{W: 10, H: 1}, children(4))

	want := []froth.Rect{
		{X: 0, W: 3, H: 1},
		{X: 3, W: 3, H: 1},
		{X: 6, W: 2, H: 1},
		{X: 8, W: 2, H: 1},
	}
	assertRects(t, got, want)
}

func TestRowGapSitsBetweenChildrenAndNotAtTheEdges(t *testing.T) {
	got := layout.Row(layout.Gap(2)).Arrange(froth.Rect{W: 12, H: 1}, children(3))

	want := []froth.Rect{
		{X: 0, W: 3, H: 1},
		{X: 5, W: 3, H: 1},
		{X: 10, W: 2, H: 1},
	}
	assertRects(t, got, want)
}

func TestRowWithOneChildFillsTheSpaceRegardlessOfGap(t *testing.T) {
	got := layout.Row(layout.Gap(3)).Arrange(froth.Rect{W: 7, H: 2}, children(1))

	assertRects(t, got, []froth.Rect{{W: 7, H: 2}})
}

func TestRowWithNoChildrenReturnsNoRects(t *testing.T) {
	if got := layout.Row().Arrange(froth.Rect{W: 7, H: 2}, nil); len(got) != 0 {
		t.Errorf("Arrange with no children returned %d rects, want none", len(got))
	}
}

// Too little space is not an error: every child still gets a rect, and the ones
// that cannot fit get an empty one. A layout that returned fewer rects than it
// was handed children would be a bug froth has to police.
func TestRowStarvesChildrenItCannotFitRatherThanDroppingThem(t *testing.T) {
	got := layout.Row(layout.Gap(1)).Arrange(froth.Rect{W: 3, H: 1}, children(3))

	want := []froth.Rect{
		{X: 0, W: 1, H: 1},
		{X: 2, W: 0, H: 1},
		{X: 3, W: 0, H: 1},
	}
	assertRects(t, got, want)
}

func assertRects(t *testing.T, got, want []froth.Rect) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("Arrange returned %d rects, want %d: %+v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("rect %d = %+v, want %+v", i, got[i], want[i])
		}
	}
}
