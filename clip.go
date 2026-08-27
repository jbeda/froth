package froth

import (
	"strings"

	"github.com/charmbracelet/x/ansi"
)

// clip trims what a surface drew to the rect it was allotted, so a surface that
// renders more than it was given cannot bleed over its sibling, and pads its
// ragged lines out to its longest one, so what reaches the compositor is always
// a solid rectangle. It reports that rectangle's size: the drawn rect, which is
// what the surface occupies and what a click resolves against.
//
// Lip Gloss drops what falls outside the canvas but takes a layer's own bounds
// from the string it was handed, so the trim is froth's job rather than the
// compositor's.
//
// It compensates for that property rather than holding an invariant of froth's:
// a back end that clips by construction, or one drawing outside a rectangle,
// makes this redundant or wrong.
//
// The padding is froth's for a different reason. The back end clears a layer's
// box before painting it, so a ragged tail is already blank without help —
// relying on that would be a second unstated dependency on compositor
// behaviour, and a drawn rect should be something froth computed rather than
// something it observed. See ADR 0018.
//
// Width is counted in columns, not bytes: a wide grapheme straddling the edge is
// dropped rather than half-drawn, so a truncated line is measured again rather
// than assumed to be exactly as wide as it was allowed.
func clip(s string, size Size) (string, Size) {
	if size.Empty() {
		return "", Size{}
	}

	lines := strings.Split(s, "\n")
	if len(lines) > size.H {
		lines = lines[:size.H]
	}

	widths := make([]int, len(lines))
	widest := 0
	for i, line := range lines {
		w := ansi.StringWidth(line)
		if w > size.W {
			lines[i] = ansi.Truncate(line, size.W, "")
			w = ansi.StringWidth(lines[i])
		}
		widths[i], widest = w, max(widest, w)
	}
	for i, line := range lines {
		if widths[i] < widest {
			lines[i] = line + strings.Repeat(" ", widest-widths[i])
		}
	}
	return strings.Join(lines, "\n"), Size{W: widest, H: len(lines)}
}
