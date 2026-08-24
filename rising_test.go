package froth_test

import (
	"strconv"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/jbeda/froth"
)

// riseMsg travels; plainMsg does not. The pair is what every test here turns
// on, since whether a message keeps going is a property of its type.
type riseMsg struct{ froth.Rising }

// otherRiseMsg rises too and nothing recognises it, which is what a second
// untaken type looks like.
type otherRiseMsg struct{ froth.Rising }

type plainMsg struct{}

// postMsg is how a test reaches a surface that is not the root: froth addresses
// by handle and a test has no way in except through the root, so the root is
// asked to post on its behalf. It is not itself rising.
type postMsg struct {
	to   froth.Handle
	msgs []tea.Msg
}

// postTo asks the root to post to h, which takes one delivery before the one
// under test. Several messages go in one update, and so in one settle pass.
func postTo(f *froth.Froth, h froth.Handle, msgs ...tea.Msg) {
	f.Update(postMsg{to: h, msgs: msgs})
}

// climber notes when it sees a message and which one, so a test reads the walk
// off one shared trace rather than out of each surface.
type climber struct {
	name  string
	trace *[]string
	kids  []froth.Surface

	// stop ends the walk when this surface sees a rising message. onRise and
	// onPlain are what a surface does beyond noting that it saw one.
	stop    bool
	onRise  func(*froth.UpdateCtx)
	onPlain func(*froth.UpdateCtx)

	acted int
	self  froth.Handle
}

func (c *climber) Mount(ac *froth.ApplyCtx) {
	c.self = ac.Self().Handle()
	for _, k := range c.kids {
		ac.Self().Create(k)
	}
}

func (c *climber) Render(*froth.RenderCtx, froth.Rect) string {
	return c.name + ":" + strconv.Itoa(c.acted)
}

func (c *climber) Update(uc *froth.UpdateCtx, msg tea.Msg) {
	switch m := msg.(type) {
	case riseMsg:
		*c.trace = append(*c.trace, c.name)
		c.acted++
		if c.onRise != nil {
			c.onRise(uc)
		}
		if c.stop {
			uc.StopRising()
		}
	case plainMsg:
		*c.trace = append(*c.trace, c.name+":plain")
		if c.onPlain != nil {
			c.onPlain(uc)
		}
	case postMsg:
		for _, out := range m.msgs {
			uc.At(m.to).Post(out)
		}
	}
}

// deaf sits in the middle of a path and recognises nothing, which is how a
// surface that does not participate is meant to behave: silence passes.
type deaf struct{ kids []froth.Surface }

func (d *deaf) Mount(ac *froth.ApplyCtx) {
	for _, k := range d.kids {
		ac.Self().Create(k)
	}
}
func (d *deaf) Render(*froth.RenderCtx, froth.Rect) string { return "" }
func (d *deaf) Update(*froth.UpdateCtx, tea.Msg)           {}

// chain builds root → mid → leaf, the smallest tree with something between the
// ends of a walk.
func chain(trace *[]string) (root, mid, leaf *climber) {
	leaf = &climber{name: "leaf", trace: trace}
	mid = &climber{name: "mid", trace: trace, kids: []froth.Surface{leaf}}
	root = &climber{name: "root", trace: trace, kids: []froth.Surface{mid}}
	return root, mid, leaf
}

func TestARisingMessageIsOfferedToEachAncestorInOrder(t *testing.T) {
	var trace []string
	root, _, leaf := chain(&trace)
	f := newSized(t, root, 8, 3)

	postTo(f, leaf.self, riseMsg{})

	if got, want := strings.Join(trace, ","), "leaf,mid,root"; got != want {
		t.Errorf("the walk visited %q, want %q — a rising message starts at the "+
			"surface it was posted to and climbs to the root", got, want)
	}
}

func TestStopRisingEndsTheWalk(t *testing.T) {
	var trace []string
	root, mid, leaf := chain(&trace)
	mid.stop = true
	f, diags := newWatched(t, root, 8, 3)

	postTo(f, leaf.self, riseMsg{})

	if got, want := strings.Join(trace, ","), "leaf,mid"; got != want {
		t.Errorf("the walk visited %q, want %q — nothing above the surface that "+
			"called StopRising should see the message", got, want)
	}
	if len(*diags) != 0 {
		t.Errorf("froth reported %v, want nothing: the message was taken", *diags)
	}
}

func TestASurfaceThatIgnoresARisingMessageDoesNotEndTheWalk(t *testing.T) {
	var trace []string
	leaf := &climber{name: "leaf", trace: &trace}
	root := &climber{name: "root", trace: &trace,
		kids: []froth.Surface{&deaf{kids: []froth.Surface{leaf}}}}
	f := newSized(t, root, 8, 3)

	postTo(f, leaf.self, riseMsg{})

	if got, want := strings.Join(trace, ","), "leaf,root"; got != want {
		t.Errorf("the walk visited %q, want %q — a surface that recognises "+
			"nothing passes the message on", got, want)
	}
}

func TestASurfaceCanActOnARisingMessageAndLetItContinue(t *testing.T) {
	var trace []string
	root, mid, leaf := chain(&trace)
	f := newSized(t, root, 8, 3)

	postTo(f, leaf.self, riseMsg{})

	if mid.acted != 1 {
		t.Fatalf("the middle surface acted %d times, want 1", mid.acted)
	}
	if root.acted != 1 {
		t.Error("the walk stopped at a surface that acted on the message " +
			"without calling StopRising; acting is not taking")
	}
}

func TestARisingMessageNothingTakesIsReported(t *testing.T) {
	var trace []string
	root, _, leaf := chain(&trace)
	f, diags := newWatched(t, root, 8, 3)

	postTo(f, leaf.self, riseMsg{})

	if len(*diags) != 1 || (*diags)[0].Kind != froth.DiagUntakenMessage {
		t.Fatalf("froth reported %v, want one %q", *diags, froth.DiagUntakenMessage)
	}
	detail := (*diags)[0].Detail
	if !strings.Contains(detail, "riseMsg") {
		t.Errorf("the diagnostic detail %q does not name the message type", detail)
	}
	if !strings.Contains(detail, leaf.self.String()) {
		t.Errorf("the diagnostic detail %q does not say where the walk started", detail)
	}
}

func TestAnAddressedMessageNeverRises(t *testing.T) {
	var trace []string
	root, _, leaf := chain(&trace)
	f, diags := newWatched(t, root, 8, 3)

	postTo(f, leaf.self, plainMsg{})

	if got, want := strings.Join(trace, ","), "leaf:plain"; got != want {
		t.Errorf("the message reached %q, want %q — addressing would mean "+
			"nothing if a message continued past the surface it named", got, want)
	}
	if len(*diags) != 0 {
		t.Errorf("froth reported %v, want nothing: an addressed message nobody "+
			"acts on is ordinary", *diags)
	}
}

func TestNothingIsDeliveredBetweenTwoHopsOfAWalk(t *testing.T) {
	var trace []string
	root, _, leaf := chain(&trace)
	f := newSized(t, root, 8, 3)

	// The leaf posts while the walk is passing through it. The new message goes
	// to the tail of the queue, so it lands after the walk finishes rather than
	// between two of its hops.
	leaf.onRise = func(uc *froth.UpdateCtx) { uc.At(root.self).Post(plainMsg{}) }

	postTo(f, leaf.self, riseMsg{})

	if got, want := strings.Join(trace, ","), "leaf,mid,root,root:plain"; got != want {
		t.Errorf("delivery order was %q, want %q — a walk can cause later "+
			"traffic but cannot be split by it", got, want)
	}
}

// Two surfaces handing the same rising message back and forth is a cascade like
// any other, and every hop spends from the same budget.
func TestARisingMessageBouncedBetweenTwoSurfacesTripsTheCap(t *testing.T) {
	var trace []string
	a := &climber{name: "a", trace: &trace, stop: true}
	b := &climber{name: "b", trace: &trace, stop: true}
	root := &climber{name: "root", trace: &trace, kids: []froth.Surface{a, b}}
	f, diags := newWatched(t, root, 8, 3)
	a.onRise = func(uc *froth.UpdateCtx) { uc.At(b.self).Post(riseMsg{}) }
	b.onRise = func(uc *froth.UpdateCtx) { uc.At(a.self).Post(riseMsg{}) }

	postTo(f, a.self, riseMsg{}) // returns rather than hanging

	if len(*diags) != 1 || (*diags)[0].Kind != froth.DiagIterationCap {
		t.Fatalf("froth reported %v, want one %q", *diags, froth.DiagIterationCap)
	}
}

// Whether a message travels is a property of its type, so there is no data that
// makes a surface call StopRising for one that does not.
func TestStopRisingPanicsForAMessageThatDoesNotRise(t *testing.T) {
	var trace []string
	root := &climber{name: "root", trace: &trace}
	root.onPlain = func(uc *froth.UpdateCtx) { uc.StopRising() }
	f := newSized(t, root, 8, 1)

	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("StopRising did not panic for a message with no walk to end")
		}
		if got, ok := r.(string); !ok || !strings.Contains(got, "plainMsg") {
			t.Errorf("the panic was %v; it should name the message it was "+
				"called for", r)
		}
	}()

	postTo(f, root.self, plainMsg{})
}

// A missing case is a property of a type, so a pass full of untaken messages is
// as many findings as there are types in it — not as many as there are
// messages, which for a cascade would be thousands.
func TestUntakenMessagesAreReportedOncePerTypeInOneUpdate(t *testing.T) {
	var trace []string
	root, _, leaf := chain(&trace)
	f, diags := newWatched(t, root, 8, 3)

	postTo(f, leaf.self, riseMsg{}, riseMsg{}, riseMsg{}, otherRiseMsg{})

	if len(*diags) != 2 {
		t.Fatalf("froth reported %d diagnostics, want 2 — one per untaken "+
			"type: %v", len(*diags), *diags)
	}
	both := (*diags)[0].Detail + (*diags)[1].Detail
	for _, want := range []string{"riseMsg", "otherRiseMsg"} {
		if !strings.Contains(both, want) {
			t.Errorf("no diagnostic named %s: %v", want, *diags)
		}
	}
}
