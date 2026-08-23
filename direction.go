// Directional focus movement: which surface an arrow key hands focus to.
//
// Direction is resolved against the rendered frame and against nothing else. A
// layout returns rects and knows nothing about focus, so a layout written
// tomorrow gets directional movement without implementing any of it. See
// ADR 0013 for why the geometry is the frame already drawn.
//
// Two jobs, split: which surfaces are eligible is a question about the tree, and
// which of their rects wins is a question about geometry. pick answers the
// second from candidates alone, so it can be asked the same question a hundred
// times over one gathered set — which is what a reachability check over a whole
// layout is.

package froth

// A direction is one of the four ways focus moves geometrically.
type direction int

const (
	dirLeft direction = iota
	dirRight
	dirUp
	dirDown
)

func (d direction) horizontal() bool { return d == dirLeft || d == dirRight }

// A candidate is somewhere a directional move could land: the surface, where it
// was in the rendered frame, and how far up the stack it was drawn.
type candidate struct {
	h     Handle
	rect  Rect
	order int
}

// stepDirection resolves a directional move: where focus goes from h travelling
// in d, within h's scope. It reports false when nothing does, which leaves focus
// where it is.
//
// Nothing moves when there is no candidate that way, when nothing has been
// rendered yet, and when h had no cells in the frame that was. All three are the
// same silence to the user, and none is a mistake worth reporting.
func (f *Froth) stepDirection(h Handle, d direction) (Handle, bool) {
	src, ok := f.renderedRect(h)
	if !ok || src.Empty() {
		return Handle{}, false
	}
	return pick(d, src, f.candidatesIn(f.scopeOf(h), h))
}

// candidatesIn gathers the stops a move within scope may land on, excluding
// except and anything that drew no cells. It walks the rendered frame once
// rather than looking each stop up in it, so the cost is the frame plus the
// scope rather than their product.
//
// The frame can name surfaces destroyed since it was drawn. Membership is
// decided by the live tree, so those are absent without being filtered for.
func (f *Froth) candidatesIn(scope, except Handle) []candidate {
	order := f.scopeOrder(scope)
	eligible := make(map[Handle]bool, len(order))
	for _, h := range order {
		if h != except && f.isTabStop(h) {
			eligible[h] = true
		}
	}

	out := make([]candidate, 0, len(eligible))
	for i, p := range f.renderedFrame {
		if eligible[p.h] && !p.rect.Empty() {
			out = append(out, candidate{h: p.h, rect: p.rect, order: i})
		}
	}
	return out
}

// renderedRect reports where h was in the rendered frame, or false for a surface
// that was not in it.
func (f *Froth) renderedRect(h Handle) (Rect, bool) {
	for _, p := range f.renderedFrame {
		if p.h == h {
			return p.rect, true
		}
	}
	return Rect{}, false
}

// pick reports which candidate a move from src in direction d lands on, or false
// when none lies that way. It reads the candidates and nothing else: no tree, no
// scope, no focus. Candidates need not be in the direction — pick filters — so a
// caller with many questions about one layout gathers once and asks repeatedly.
func pick(d direction, src Rect, cands []candidate) (Handle, bool) {
	best := -1
	for i := range cands {
		if !inDirection(d, src, cands[i].rect) {
			continue
		}
		if best < 0 || better(d, src, cands[i], cands[best]) {
			best = i
		}
	}
	if best < 0 {
		return Handle{}, false
	}
	return cands[best].h, true
}

// inDirection reports whether dst lies the way d points: both of its edges have
// moved that way. Requiring both admits a stop nested inside another — a scope
// is a stop within itself, so candidates are not always disjoint — while
// rejecting a surface that merely grew around the source.
func inDirection(d direction, src, dst Rect) bool {
	switch d {
	case dirLeft:
		return dst.X < src.X && dst.right() < src.right()
	case dirRight:
		return dst.X > src.X && dst.right() > src.right()
	case dirUp:
		return dst.Y < src.Y && dst.bottom() < src.bottom()
	default:
		return dst.Y > src.Y && dst.bottom() > src.bottom()
	}
}

// better reports whether a beats b as the target of a move in d. The tests are
// applied in order, and the first that separates the two decides:
//
//  1. a candidate whose perpendicular extent overlaps the source's beats one
//     whose does not, outright;
//  2. least distance along the direction of travel;
//  3. least distance between perpendicular centres;
//  4. reading order — topmost, then leftmost, assuming left-to-right;
//  5. drawn last, and so on top.
//
// The order is total, which matters more here than in a toolkit drawing to
// pixels: froth's rects are integer cells produced by tiling layouts, so a row
// of equal panes ties on every test but the last. That last test ends it
// because two surfaces cannot share a place in the frame, and a surface drawn
// over another is the one the user is looking at.
//
// The fourth test is reading order rather than traversal order deliberately:
// traversal order is declaration order, which a layout is free to place
// against, so tying to it would let tree structure decide a geometric move.
func better(d direction, src Rect, a, b candidate) bool {
	if x, y := inBeam(d, src, a.rect), inBeam(d, src, b.rect); x != y {
		return x
	}
	if x, y := axial(d, src, a.rect), axial(d, src, b.rect); x != y {
		return x < y
	}
	if x, y := alignment(d, src, a.rect), alignment(d, src, b.rect); x != y {
		return x < y
	}
	if a.rect.Y != b.rect.Y {
		return a.rect.Y < b.rect.Y
	}
	if a.rect.X != b.rect.X {
		return a.rect.X < b.rect.X
	}
	return a.order > b.order
}

// inBeam reports whether dst overlaps the band the source sweeps as it travels:
// its vertical extent for a horizontal move, its horizontal extent for a
// vertical one.
func inBeam(d direction, src, dst Rect) bool {
	if d.horizontal() {
		return dst.bottom() > src.Y && dst.Y < src.bottom()
	}
	return dst.right() > src.X && dst.X < src.right()
}

// axial is the gap along the direction of travel, from the source's leading edge
// to the candidate's nearest one. A candidate overlapping the source clamps to
// zero rather than going negative, so reaching further into the source cannot
// buy a candidate a better score than sitting flush against it.
func axial(d direction, src, dst Rect) int {
	var raw int
	switch d {
	case dirLeft:
		raw = src.X - dst.right()
	case dirRight:
		raw = dst.X - src.right()
	case dirUp:
		raw = src.Y - dst.bottom()
	default:
		raw = dst.Y - src.bottom()
	}
	return max(0, raw)
}

// alignment is the distance between the two rects' centres across the direction
// of travel: how far off the candidate sits from straight ahead.
func alignment(d direction, src, dst Rect) int {
	if d.horizontal() {
		return abs(src.midY() - dst.midY())
	}
	return abs(src.midX() - dst.midX())
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}
