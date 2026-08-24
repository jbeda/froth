// Whether a message keeps travelling after it arrives, and the walk that
// carries it. See ADR 0016.

package froth

import tea "charm.land/bubbletea/v2"

// Rising makes a message type travel. Embed it in a message struct and froth
// offers a value of that type to the surface it was posted to, then to that
// surface's parent, and on up through the root to the instance — stopping at
// the first surface that calls StopRising.
//
//	type ConfirmDelete struct {
//		froth.Rising
//		Row int
//	}
//
// There is no second verb: uc.Post sends it and Update receives it, exactly as
// for a message addressed to one surface. How far it travels is a property of
// the type, so a reader of the type knows its reach without finding the send
// site — and a type rises or it does not, permanently.
//
// A surface that does not recognise a rising message does nothing, and it keeps
// going. Acting on one and letting it continue is simply not calling StopRising.
// One that nothing takes reaches the tail and is reported as a Diagnostic.
type Rising struct{}

// rising is unexported, so nothing outside froth can satisfy riser except by
// embedding Rising.
func (Rising) rising() {}

type riser interface{ rising() }

// Rises reports whether msg travels past the surface it was posted to.
//
// Whether a message rises is fixed by its type, so a surface handling a type it
// wrote already knows. This is for the code that does not: a helper shared by
// both kinds of message, or a surface handling a message type it did not
// declare. It is the question StopRising panics on getting wrong, which is why
// there is a way to ask it.
func Rises(msg tea.Msg) bool {
	_, ok := msg.(riser)
	return ok
}
