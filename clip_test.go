package froth

import (
	"testing"

	"charm.land/lipgloss/v2"
)

// Clipping is a self-contained algorithm over a string: given what a surface
// drew and the size it was offered, what reaches the compositor and what
// rectangle that makes. It reaches for no tree, no phase and no handle, so it is
// tested as the function it is.
//
// The rule is ADR 0018. The wiring around it — that froth clips every surface's
// output, and what the drawn rect is then used for — is tested through the seam.

func TestClippingToWhatWasOffered(t *testing.T) {
	cases := []struct {
		name  string
		in    string
		size  Size
		want  string
		drawn Size
	}{{
		name:  "a surface that fills its offer is left alone",
		in:    "ab\ncd",
		size:  Size{W: 2, H: 2},
		want:  "ab\ncd",
		drawn: Size{W: 2, H: 2},
	}, {
		name:  "lines past the offered height are dropped",
		in:    "ab\ncd\nef",
		size:  Size{W: 2, H: 2},
		want:  "ab\ncd",
		drawn: Size{W: 2, H: 2},
	}, {
		name:  "a line past the offered width is truncated",
		in:    "abcd\nef",
		size:  Size{W: 3, H: 2},
		want:  "abc\nef ",
		drawn: Size{W: 3, H: 2},
	}, {
		name:  "ragged lines are padded out to the longest one",
		in:    "a\nbbb\ncc",
		size:  Size{W: 5, H: 5},
		want:  "a  \nbbb\ncc ",
		drawn: Size{W: 3, H: 3},
	}, {
		name:  "underfilling both ways draws less than it was offered",
		in:    "ab",
		size:  Size{W: 10, H: 4},
		want:  "ab",
		drawn: Size{W: 2, H: 1},
	}, {
		name:  "a wide grapheme straddling the edge is dropped, not halved",
		in:    "a世",
		size:  Size{W: 2, H: 1},
		want:  "a",
		drawn: Size{W: 1, H: 1},
	}, {
		name:  "a wide grapheme counts its columns when it fits",
		in:    "世界\nab",
		size:  Size{W: 4, H: 2},
		want:  "世界\nab  ",
		drawn: Size{W: 4, H: 2},
	}, {
		name:  "an offer with no cells takes nothing and draws nothing",
		in:    "ab\ncd",
		size:  Size{W: 0, H: 3},
		want:  "",
		drawn: Size{},
	}}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, drawn := clip(c.in, c.size)
			if got != c.want {
				t.Errorf("clip(%q, %v) = %q, want %q", c.in, c.size, got, c.want)
			}
			if drawn != c.drawn {
				t.Errorf("clip(%q, %v) drew %v, want %v", c.in, c.size, drawn, c.drawn)
			}
		})
	}
}

// The drawn rect is what the compositor derives from the same string, or
// hit-testing answers about cells nobody painted. Lip Gloss takes a layer's
// bounds from its content, so the two measurements have to agree line for line.
//
// This is the one place outside render.go that names the back end, and it is
// here because the agreement is the assumption ADR 0018 rests on: swapping the
// compositor has to bring this test with it.
func TestTheDrawnSizeIsWhatTheCompositorWouldMeasure(t *testing.T) {
	for _, in := range []string{"", "a", "a\nbbb\ncc", "世界\nab", "a\n\nb"} {
		got, drawn := clip(in, Size{W: 20, H: 20})
		if w, h := lipgloss.Width(got), lipgloss.Height(got); w != drawn.W || h != drawn.H {
			t.Errorf("clip(%q, …) drew %v, compositor measures %dx%d", in, drawn, w, h)
		}
	}
}
