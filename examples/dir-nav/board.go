package main

import "github.com/jbeda/froth"

// A region is the part of the board a surface belongs to. Two things need to
// know it and neither can ask froth: the banner says where focus is, and the
// root decides which divider the size keys move.
type region int

const (
	regNone region = iota
	regButton
	regRow
	regDetail
	regRight
	regDialog
	regTwin
	regFilter
)

// A spot is where focus is: which region, which surface within it, its name, and
// the size it was offered. The size is here because the rule being judged reads
// geometry and nothing else, and the geometry on screen stops matching the
// geometry in your head after a few size keys.
//
// It is a size rather than a rect because that is all a surface is told: where
// it sits is its layout's decision, and Render is offered no coordinates it
// could act on.
type spot struct {
	region region
	index  int
	name   string
	size   froth.Size
}

// board is what the surfaces share. It carries the splits the layouts read, and
// it is how the rest of the program finds out where focus is: a surface learns
// it is focused during Render, and froth tells nobody else — a directional move
// is a froth action nobody here takes, so what announces it is the next frame
// rather than a message. Each surface records itself as it draws.
//
// The banner is the root's last child and the walk that renders is depth-first
// in child order, so the banner draws after every surface it reports on and
// reads the frame they have just written rather than the one before.
type board struct {
	splits splits

	// bare swaps the arrow keys for the shift+arrow ones. It lives here because
	// the banner says which is in force and the root's bindings decide it.
	bare bool

	focus spot
	trail []string

	// diag is the last thing froth reported through the diagnostic hook. The
	// banner shows it: a board you press unfamiliar keys at should not drop
	// what froth said about them.
	diag string

	// pick is the control the dialog last had taken, which the banner shows.
	// The dialog is gone by the time anyone reads it, so the result has to
	// outlive it.
	pick string
}

// trailLen is how many steps of focus history the banner shows. Two would say
// where the last move went; three says it while still showing where you were
// before the move you are second-guessing.
const trailLen = 3

// saw records the surface that drew itself focused.
func (b *board) saw(s spot) {
	b.focus = s
	if n := len(b.trail); n == 0 || b.trail[n-1] != s.name {
		b.trail = append(b.trail, s.name)
	}
	if len(b.trail) > trailLen {
		b.trail = b.trail[len(b.trail)-trailLen:]
	}
}
