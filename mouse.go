// Mouse: where a mouse message goes, and the one claim that overrides it.
//
// Without capture a message is hit-tested: the surface under the cursor in the
// rendered frame gets it, and a click there may move focus. While a surface
// holds capture, every mouse message goes to it wherever it landed. See ADR
// 0019 for the hit walk and ADR 0020 for capture.

package froth

import (
	"fmt"

	tea "charm.land/bubbletea/v2"
)

// CaptureLostMsg is delivered to a surface that held capture when froth takes it
// away: another surface took it, or a modal opened over the holder. Its
// contract is to abandon whatever the capture was for, never to complete it —
// a drag interrupted by a dialog leaves nothing half-applied.
//
// It is not delivered when the holder is destroyed, since a surface leaving the
// tree can complete nothing; Unmount runs for it. Nor when the holder
// releases capture itself, since it asked.
type CaptureLostMsg struct{}

// routeMouse addresses a mouse message. Capture is checked first and answers
// alone: the holder gets every mouse message with nothing else consulted, which
// is what lets it see a click land outside itself. Otherwise the surface under
// the cursor gets it, and a press there moves focus if the surface admits it.
//
// It reports false for a message that goes nowhere — nothing was under the
// cursor, or an open modal has taken input away from what was — which is the
// same drop a raw key aimed behind a modal takes.
func (f *Froth) routeMouse(msg tea.MouseMsg) (envelope, bool) {
	if f.capture != (Handle{}) {
		return envelope{to: f.capture, msg: msg}, true
	}
	m := msg.Mouse()
	h, ok := f.hit(Pt(m.X, m.Y))
	if !ok || f.blockedByModal(h) {
		return envelope{}, false
	}
	if _, press := msg.(tea.MouseClickMsg); press && f.admitsClick(h) {
		// Recorded here and committed by the Apply that leads the settle, so
		// the target holds focus by the time the press reaches it — and a
		// target that destroys itself on the press is rehomed from, rather
		// than refused as a move onto a surface that has gone.
		f.pendingFocus, f.focusPending = h, true
	}
	return envelope{to: h, msg: msg}, true
}

// hit reports the surface under p in the rendered frame: the topmost placement
// whose drawn rect contains the point, skipping every surface that is mouse
// transparent or sits under one. It resolves against the drawn rect rather than
// the surface rect because that is what was painted; see ADR 0018.
//
// The frame is the one the user was looking at, so a placement can name a
// surface destroyed since it was drawn. That surface is still the answer — what
// lies beneath it was not on screen — and delivering to it is the ordinary
// dead-handle drop.
func (f *Froth) hit(p Point) (Handle, bool) {
	for i := len(f.renderedFrame) - 1; i >= 0; i-- {
		pl := f.renderedFrame[i]
		drawn := Rect{X: pl.rect.X, Y: pl.rect.Y, W: pl.drawn.W, H: pl.drawn.H}
		if !drawn.Contains(p) || f.mouseTransparent(pl.h) {
			continue
		}
		return pl.h, true
	}
	return Handle{}, false
}

// mouseTransparent reports whether h or any ancestor of it was marked
// MouseTransparent, which takes a whole subtree out of the hit walk. A handle
// not in the tree answers false: there is nothing to read the mark from.
func (f *Froth) mouseTransparent(h Handle) bool {
	for _, anc := range f.chainTo(h) {
		if f.reg.get(anc).meta.mouseTransparent {
			return true
		}
	}
	return false
}

// admitsClick reports whether a press on h moves focus to it. There is no walk
// to an ancestor that would have admitted it: a surface that should pass a
// click onward is marked transparent by its creator instead. See ADR 0019.
func (f *Froth) admitsClick(h Handle) bool {
	n := f.reg.get(h)
	return n != nil && n.meta.flags.ClickFocus
}

// takeCapture makes h the capture holder, at once in either phase. A surface
// that already holds it keeps it and hears nothing; one that held it loses it
// and is told. A surface an open modal has taken input away from is refused.
// See ADR 0020 for why each of those.
func (f *Froth) takeCapture(h Handle) {
	if f.reg.get(h) == nil || f.capture == h {
		return
	}
	if f.blockedByModal(h) {
		f.report(Diagnostic{
			Kind: DiagCaptureBlocked,
			Detail: fmt.Sprintf("%s asked for capture while an open modal takes "+
				"input away from it; capture is unchanged", h),
		})
		return
	}
	f.loseCapture()
	f.capture = h
}

// loseCapture takes capture away from its holder and tells it. The message is
// queued rather than delivered, so it arrives after the loss has taken effect.
func (f *Froth) loseCapture() {
	if f.capture != (Handle{}) {
		f.queue = append(f.queue, envelope{to: f.capture, msg: CaptureLostMsg{}})
	}
	f.clearCapture()
}

// clearCapture releases capture and says nothing to the holder.
func (f *Froth) clearCapture() { f.capture = Handle{} }

// releaseCapture is an explicit release by the holder. A surface that does not
// hold capture releases nothing, silently: a press that captured and a release
// that lets go will routinely find a modal has already taken it in between.
func (f *Froth) releaseCapture(h Handle) {
	if f.capture == h {
		f.clearCapture()
	}
}

// modalOpened releases capture if the modal that just entered the tree takes
// input away from the holder. A modal parented elsewhere leaves it alone: the
// holder is not in the subtree that modal blocks, so nothing has changed for
// it.
func (f *Froth) modalOpened() {
	if f.capture != (Handle{}) && f.blockedByModal(f.capture) {
		f.loseCapture()
	}
}
