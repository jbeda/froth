package froth

import (
	"strings"

	"github.com/charmbracelet/x/ansi"
)

// clip trims what a surface drew to the size it was offered and pads its ragged
// lines out to its longest one, so what reaches the compositor is always a solid
// rectangle. It reports that rectangle's size: the drawn rect.
//
// Width is counted in columns, not bytes: a wide grapheme straddling the edge is
// dropped rather than half-drawn, so a truncated line is measured again rather
// than assumed to be as wide as it was allowed. See ADR 0018.
func clip(content string, size Size) (string, Size) {
	if size.Empty() {
		return "", Size{}
	}

	lines := strings.Split(content, "\n")
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
