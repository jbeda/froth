package froth

import (
	"strings"

	"github.com/charmbracelet/x/ansi"
)

// clip trims what a surface drew to the rect it was allotted, so a surface that
// renders more than it was given cannot bleed over its sibling. Lip Gloss drops
// what falls outside the canvas but takes a layer's own bounds from the string
// it was handed, so this is froth's job rather than the compositor's.
//
// It compensates for that property rather than holding an invariant of froth's:
// a back end that clips by construction, or one drawing outside a rectangle,
// makes this redundant or wrong.
//
// Width is counted in columns, not bytes: a wide grapheme straddling the edge is
// dropped rather than half-drawn.
func clip(s string, size Size) string {
	if size.W <= 0 || size.H <= 0 {
		return ""
	}

	lines := strings.Split(s, "\n")
	if len(lines) > size.H {
		lines = lines[:size.H]
	}
	for i, line := range lines {
		if ansi.StringWidth(line) > size.W {
			lines[i] = ansi.Truncate(line, size.W, "")
		}
	}
	return strings.Join(lines, "\n")
}
