package froth_test

import (
	"context"
	"strconv"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/jbeda/froth"
)

// relay is the drain loop's fixture: it counts what it has seen, counts its own
// renders, and forwards each message to the next surface in a chain. One
// message therefore walks the whole chain within a single update, which is the
// claim the loop exists to make good on.
type relay struct {
	kids []froth.Surface
	next *relay // whose handle to forward to; nil ends the chain

	self    froth.Handle
	seen    int
	renders int
}

func (r *relay) Render(rc *froth.RenderCtx, rect froth.Rect) string {
	r.renders++
	return strconv.Itoa(r.seen)
}

func (r *relay) Update(uc *froth.UpdateCtx, msg tea.Msg) {
	if _, ok := msg.(bumpMsg); !ok {
		return
	}
	r.seen++
	if r.next != nil {
		uc.At(r.next.self).Post(msg)
	}
}

func (r *relay) Mount(ac *froth.ApplyCtx) {
	r.self = ac.Self().Handle()
	for _, k := range r.kids {
		ac.Self().Create(k)
	}
}

// newWatched is newSized for the tests that need to see what froth reported.
func newWatched(t *testing.T, root froth.Surface, w, h int) (*froth.Froth, *[]froth.Diagnostic) {
	t.Helper()
	var got []froth.Diagnostic
	f := froth.New(context.Background(), root,
		froth.OnDiagnostic(func(d froth.Diagnostic) { got = append(got, d) }))
	f.Update(tea.WindowSizeMsg{Width: w, Height: h})
	return f, &got
}

// cycler answers every message with the same message, addressed to itself, which
// is the shortest cascade that never settles.
type cycler struct{}

func (c *cycler) Render(*froth.RenderCtx, froth.Rect) string { return "cycling" }
func (c *cycler) Update(uc *froth.UpdateCtx, msg tea.Msg)    { uc.Self().Post(msg) }

func TestAPostedMessageIsHandledInTheSameFrame(t *testing.T) {
	tail := &relay{}
	root := &relay{next: tail, kids: []froth.Surface{tail}}
	f := newSized(t, root, 4, 1)

	// One update, two deliveries: the root's, and the one it posted. The tail
	// draws over the root, so what the frame shows is the tail's count.
	f.Update(bumpMsg{})

	if got, want := f.View().Content, "1"; got != want {
		t.Errorf("View().Content = %q, want %q — the posted message was not "+
			"handled before the frame was drawn", got, want)
	}
}

// A chain of four settles in one update: nothing waits for the next message
// from Bubble Tea, so no partial state is ever rendered.
func TestAPostCascadeSettlesWithinOneUpdate(t *testing.T) {
	d := &relay{}
	c := &relay{next: d}
	b := &relay{next: c}
	root := &relay{next: b, kids: []froth.Surface{b, c, d}}
	f := newSized(t, root, 4, 1)

	f.Update(bumpMsg{})

	if got, want := f.View().Content, "1"; got != want {
		t.Errorf("View().Content = %q, want %q — the end of the chain never saw "+
			"the message", got, want)
	}
	if got := f.Composite(b.self).Content; got != "1" {
		t.Errorf("middle of the chain rendered %q, want %q", got, "1")
	}
}

func TestASurfaceCanPostToItself(t *testing.T) {
	root := &bouncer{}
	f := newSized(t, root, 4, 1)

	f.Update(bumpMsg{})

	// Two deliveries, one update: the one from Bubble Tea and the one the
	// surface addressed to itself.
	if got, want := f.View().Content, "2"; got != want {
		t.Errorf("View().Content = %q, want %q", got, want)
	}
}

// bouncer posts one message back to itself, which is the shortest thing Self
// is for.
type bouncer struct{ seen int }

func (b *bouncer) Render(*froth.RenderCtx, froth.Rect) string { return strconv.Itoa(b.seen) }

func (b *bouncer) Update(uc *froth.UpdateCtx, msg tea.Msg) {
	b.seen++
	if b.seen == 1 {
		uc.Self().Post(msg)
	}
}

func TestNothingRendersWhileMessagesAreBeingDelivered(t *testing.T) {
	kid := &relay{}
	root := &relay{next: kid, kids: []froth.Surface{kid}}
	f := newSized(t, root, 4, 1)

	f.Update(bumpMsg{})

	if root.renders != 0 || kid.renders != 0 {
		t.Errorf("surfaces rendered %d and %d times during a cascade of "+
			"deliveries, want 0 — Render runs once the queue has drained, not "+
			"per message", root.renders, kid.renders)
	}
}

func TestEverySurfaceRendersExactlyOncePerFrame(t *testing.T) {
	kid := &relay{}
	root := &relay{next: kid, kids: []froth.Surface{kid}}
	f := newSized(t, root, 4, 1)

	// Several deliveries, one frame.
	f.Update(bumpMsg{})
	f.View()

	if root.renders != 1 || kid.renders != 1 {
		t.Errorf("one frame rendered the root %d times and its child %d times, "+
			"want 1 each", root.renders, kid.renders)
	}
}

// A message cycle is a bug, and the loop's job is to make it a report rather
// than a freeze.
func TestAMessageCycleTripsTheIterationCapAndIsReported(t *testing.T) {
	a, b := &relay{}, &relay{}
	a.next, b.next = b, a
	root := &relay{next: a, kids: []froth.Surface{a, b}}

	f, got := newWatched(t, root, 4, 1)

	f.Update(bumpMsg{}) // returns rather than hanging

	if len(*got) != 1 {
		t.Fatalf("a message cycle reported %d diagnostics, want 1: %v", len(*got), *got)
	}
	if (*got)[0].Kind != froth.DiagIterationCap {
		t.Errorf("diagnostic kind = %q, want %q", (*got)[0].Kind, froth.DiagIterationCap)
	}
	if (*got)[0].Detail == "" {
		t.Error("the diagnostic carried no detail saying what did not settle")
	}
}

// Tripping the cap abandons the cascade; it does not wedge the instance.
func TestFrothStillDrawsAfterTheCapTrips(t *testing.T) {
	f, _ := newWatched(t, &cycler{}, 8, 1)

	f.Update(bumpMsg{})

	if got := f.View().Content; got == "" {
		t.Error("nothing rendered after the cap tripped")
	}
	// The queue was abandoned rather than left in place, so the next update
	// starts from a drained queue and trips on its own cascade, not the old one.
	f.Update(bumpMsg{})
	if got := f.View().Content; got == "" {
		t.Error("nothing rendered on the frame after the cap tripped")
	}
}

// A surface that mounts a child of its own type never settles either, and it
// spends the same budget as delivery does.
func TestAMountCascadeTripsTheIterationCapAndIsReported(t *testing.T) {
	_, got := newWatched(t, &spawner{}, 4, 1)

	if len(*got) != 1 {
		t.Fatalf("a mount cascade reported %d diagnostics, want 1: %v", len(*got), *got)
	}
	if (*got)[0].Kind != froth.DiagIterationCap {
		t.Errorf("diagnostic kind = %q, want %q", (*got)[0].Kind, froth.DiagIterationCap)
	}
}

type spawner struct{}

func (s *spawner) Render(*froth.RenderCtx, froth.Rect) string { return "" }
func (s *spawner) Update(*froth.UpdateCtx, tea.Msg)           {}
func (s *spawner) Mount(ac *froth.ApplyCtx)                   { ac.Self().Create(&spawner{}) }

// A handle outlives the surface it names, so addressing one that names nothing
// is a defined drop rather than an error. It is reported, because minting a
// context is where froth learns the surface has gone, and the sender carries on
// either way.
func TestAPostToAHandleThatNamesNothingIsDroppedAndReported(t *testing.T) {
	f, diags := newWatched(t, &dropper{}, 4, 1)

	f.Update(bumpMsg{})

	if got, want := f.View().Content, "ok"; got != want {
		t.Errorf("View().Content = %q, want %q", got, want)
	}
	if len(*diags) != 1 || (*diags)[0].Kind != froth.DiagDeadHandle {
		t.Fatalf("froth reported %v, want one %q", *diags, froth.DiagDeadHandle)
	}
}

type dropper struct{}

func (d *dropper) Render(*froth.RenderCtx, froth.Rect) string { return "ok" }
func (d *dropper) Update(uc *froth.UpdateCtx, msg tea.Msg)    { uc.At(froth.Handle{}).Post(msg) }

// The hook is application code, so froth calls it with no phase in progress —
// otherwise a hook that looked at the tree would trip the re-entrancy check
// instead of reporting what it was called about.
func TestTheDiagnosticHookCanCallBackIntoFroth(t *testing.T) {
	var content string
	var f *froth.Froth
	f = froth.New(context.Background(), &cycler{},
		froth.OnDiagnostic(func(froth.Diagnostic) { content = f.View().Content }))
	f.Update(tea.WindowSizeMsg{Width: 8, Height: 1})

	f.Update(bumpMsg{})

	if content == "" {
		t.Error("the hook could not render from inside itself")
	}
}

// A hook is application code, and application code may drive froth. A hook that
// provokes the very diagnostic it is handling would otherwise never return —
// each report calling the hook that causes the next one. froth bounds the
// rounds and drops what is left rather than recursing.
func TestAHookThatProvokesItsOwnDiagnosticTerminates(t *testing.T) {
	calls := 0
	var f *froth.Froth
	f = froth.New(context.Background(), &cycler{},
		froth.OnDiagnostic(func(froth.Diagnostic) {
			calls++
			f.Update(bumpMsg{}) // trips the cap again, from inside the report of it
		}))
	f.Update(tea.WindowSizeMsg{Width: 8, Height: 1})

	f.Update(bumpMsg{})

	if calls == 0 {
		t.Fatal("the hook never fired")
	}
	if got, want := f.View().Content, "cycling"; got != want {
		t.Errorf("View().Content = %q, want %q; froth stopped drawing", got, want)
	}
}
