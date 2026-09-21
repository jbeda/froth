package froth_test

import (
	"context"
	"reflect"
	"slices"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/jbeda/froth"
)

// A command's result comes back to the surface that ran it. These tests run
// the command Update returns, exactly as Bubble Tea would, and hand what it
// produces back to Update.

// runOnBump makes p run cmd when a bumpMsg reaches it.
func runOnBump(p *poster, cmd tea.Cmd) {
	p.onMsg = func(uc *froth.UpdateCtx, msg tea.Msg) {
		if _, ok := msg.(bumpMsg); ok {
			uc.Run(cmd)
		}
	}
}

// rootOver builds a root over kid that forwards a bump to it, so a test reaches
// kid without caring where focus is. A posted message reaching the root runs
// onPosted, which is how a test changes the tree between the command and its
// result.
func rootOver(log *[]string, kid *poster, onPosted func(uc *froth.UpdateCtx)) *poster {
	root := &poster{name: "root", log: log, kids: []froth.Surface{kid}}
	root.onMsg = func(uc *froth.UpdateCtx, msg tea.Msg) {
		switch msg.(type) {
		case bumpMsg:
			uc.Child(root.kidHs[0]).Post(bumpMsg{})
		case posted:
			if onPosted != nil {
				onPosted(uc)
			}
		}
	}
	return root
}

func TestAResultGoesToTheSurfaceThatRanTheCommand(t *testing.T) {
	var log []string
	kid := &poster{name: "kid", log: &log}
	runOnBump(kid, func() tea.Msg { return posted{"done"} })
	f := newSized(t, rootOver(&log, kid, nil), 4, 1)

	res := f.Update(bumpMsg{})()
	f.Update(res)

	if !slices.Equal(log, []string{"kid:done"}) {
		t.Errorf("delivered %v, want the result at kid and nowhere else", log)
	}
}

func TestCompositeDefersCommandsUntilTheNextUpdate(t *testing.T) {
	var log []string
	root := &poster{name: "root", log: &log}
	root.onMount = func(ac *froth.ApplyCtx) {
		ac.Self().Post(bumpMsg{})
	}
	runOnBump(root, func() tea.Msg { return posted{"done"} })
	f := froth.New(context.Background(), root)

	f.Composite(f.Root())
	if got := len(root.seen); got != 1 {
		t.Fatalf("Composite delivered %d messages, want the one Mount posted", got)
	}
	if len(log) != 0 {
		t.Fatalf("Composite ran the command, delivered %v", log)
	}

	cmd := f.Update(tea.WindowSizeMsg{Width: 4, Height: 1})
	if cmd == nil {
		t.Fatal("the next Update returned no deferred command")
	}
	runAll(t, f, cmd())

	if !slices.Equal(log, []string{"root:done"}) {
		t.Errorf("delivered %v, want the deferred result at root", log)
	}
}

func TestQuitAndInterruptPassThroughUnwrapped(t *testing.T) {
	cases := []struct {
		name string
		cmd  tea.Cmd
		want tea.Msg
	}{
		{"quit", tea.Quit, tea.QuitMsg{}},
		{"interrupt", tea.Interrupt, tea.InterruptMsg{}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var log []string
			kid := &poster{name: "kid", log: &log}
			runOnBump(kid, c.cmd)
			f := newSized(t, rootOver(&log, kid, nil), 4, 1)

			if got := f.Update(bumpMsg{})(); got != c.want {
				t.Errorf("the command produced %#v, want %#v untouched", got, c.want)
			}
		})
	}
}

// A batch has to come back as a tea.BatchMsg, because the loop runs its inner
// commands only for that concrete type; and each inner result is addressed.
func TestABatchStillRunsAndEachResultIsAddressed(t *testing.T) {
	var log []string
	kid := &poster{name: "kid", log: &log}
	runOnBump(kid, tea.Batch(
		func() tea.Msg { return posted{"a"} },
		func() tea.Msg { return posted{"b"} },
	))
	f := newSized(t, rootOver(&log, kid, nil), 4, 1)

	batch, ok := f.Update(bumpMsg{})().(tea.BatchMsg)
	if !ok || len(batch) != 2 {
		t.Fatalf("the command produced %#v, want a tea.BatchMsg of two", batch)
	}
	for _, inner := range batch {
		f.Update(inner())
	}

	if !slices.Equal(log, []string{"kid:a", "kid:b"}) {
		t.Errorf("delivered %v, want both results at kid", log)
	}
}

func TestANestedBatchIsRewrappedAllTheWayDown(t *testing.T) {
	var log []string
	kid := &poster{name: "kid", log: &log}
	runOnBump(kid, tea.Batch(
		func() tea.Msg { return posted{"outer"} },
		tea.Batch(
			func() tea.Msg { return posted{"inner 1"} },
			func() tea.Msg { return posted{"inner 2"} },
		),
	))
	f := newSized(t, rootOver(&log, kid, nil), 4, 1)

	outer := f.Update(bumpMsg{})().(tea.BatchMsg)
	f.Update(outer[0]())
	inner, ok := outer[1]().(tea.BatchMsg)
	if !ok || len(inner) != 2 {
		t.Fatalf("the inner command produced %#v, want a tea.BatchMsg of two", inner)
	}
	for _, c := range inner {
		f.Update(c())
	}

	want := []string{"kid:outer", "kid:inner 1", "kid:inner 2"}
	if !slices.Equal(log, want) {
		t.Errorf("delivered %v, want %v", log, want)
	}
}

// Bubble Tea unwraps a container's inner results by recursion, so a sequence
// inside a batch and a batch inside a sequence are each rewrapped from a
// nested position.
func TestMixedNestingIsRewrappedEitherWayRound(t *testing.T) {
	cases := []struct {
		name string
		cmd  tea.Cmd
	}{
		{"sequence in batch", tea.Batch(
			func() tea.Msg { return posted{"a"} },
			tea.Sequence(
				func() tea.Msg { return posted{"b"} },
				func() tea.Msg { return posted{"c"} },
			),
		)},
		{"batch in sequence", tea.Sequence(
			func() tea.Msg { return posted{"a"} },
			tea.Batch(
				func() tea.Msg { return posted{"b"} },
				func() tea.Msg { return posted{"c"} },
			),
		)},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var log []string
			kid := &poster{name: "kid", log: &log}
			runOnBump(kid, c.cmd)
			f := newSized(t, rootOver(&log, kid, nil), 4, 1)

			runAll(t, f, f.Update(bumpMsg{})())

			want := []string{"kid:a", "kid:b", "kid:c"}
			if !slices.Equal(log, want) {
				t.Errorf("delivered %v, want %v", log, want)
			}
		})
	}
}

// runAll does what the loop does with a result: runs a batch's or a sequence's
// inner commands, recursing, and hands anything else to Update.
func runAll(t *testing.T, f *froth.Froth, res tea.Msg) {
	t.Helper()
	specimen := tea.Sequence(tea.Quit, tea.Quit)()
	var inner []tea.Cmd
	switch {
	case reflect.TypeOf(res) == reflect.TypeOf(specimen):
		inner = reflect.ValueOf(res).Convert(reflect.TypeFor[[]tea.Cmd]()).Interface().([]tea.Cmd)
	default:
		b, ok := res.(tea.BatchMsg)
		if !ok {
			f.Update(res)
			return
		}
		inner = b
	}
	for _, c := range inner {
		runAll(t, f, c())
	}
}

func TestANilResultReachesTheIssuerAsNil(t *testing.T) {
	var log []string
	kid := &poster{name: "kid", log: &log}
	runOnBump(kid, func() tea.Msg { return nil })
	f := newSized(t, rootOver(&log, kid, nil), 4, 1)

	f.Update(f.Update(bumpMsg{})())

	if got := kid.seen[len(kid.seen)-1]; got != nil {
		t.Errorf("kid last saw %#v, want the nil result", got)
	}
}

// A sequence is an unexported type the loop matches by concrete type, so the
// rewrapped value has to be that type still; the test reads it back the way
// froth builds it, through reflect.
func TestASequenceStillRunsAndEachResultIsAddressed(t *testing.T) {
	var log []string
	kid := &poster{name: "kid", log: &log}
	runOnBump(kid, tea.Sequence(
		func() tea.Msg { return posted{"first"} },
		func() tea.Msg { return posted{"second"} },
	))
	f := newSized(t, rootOver(&log, kid, nil), 4, 1)

	res := f.Update(bumpMsg{})()
	specimen := tea.Sequence(tea.Quit, tea.Quit)()
	if reflect.TypeOf(res) != reflect.TypeOf(specimen) {
		t.Fatalf("the command produced a %T, want a %T", res, specimen)
	}
	inner := reflect.ValueOf(res).Convert(reflect.TypeFor[[]tea.Cmd]()).Interface().([]tea.Cmd)
	if len(inner) != 2 {
		t.Fatalf("the sequence has %d commands, want 2", len(inner))
	}
	for _, c := range inner {
		f.Update(c())
	}

	if !slices.Equal(log, []string{"kid:first", "kid:second"}) {
		t.Errorf("delivered %v, want both results at kid, in order", log)
	}
}

func TestAResultForADestroyedIssuerIsDroppedSilently(t *testing.T) {
	var log []string
	kid := &poster{name: "kid", log: &log}
	runOnBump(kid, func() tea.Msg { return posted{"late"} })
	var root *poster
	root = rootOver(&log, kid, func(uc *froth.UpdateCtx) {
		uc.Child(root.kidHs[0]).Destroy()
	})
	f, diags := newWatched(t, root, 4, 1)

	res := f.Update(bumpMsg{})()
	f.Update(posted{"destroy"})
	f.Update(res)

	if !slices.Equal(log, []string{"root:destroy"}) {
		t.Errorf("delivered %v, want nothing after the destroy", log)
	}
	if len(*diags) != 0 {
		t.Errorf("froth reported %v, want nothing: a late result is expected", *diags)
	}
}

// route blocks only keys and mouse under a modal; a result is addressed, so it
// reaches the surface that asked for it however the tree looks by then.
func TestAResultReachesASurfaceUnderAModal(t *testing.T) {
	var log []string
	kid := &poster{name: "kid", log: &log}
	runOnBump(kid, func() tea.Msg { return posted{"under the modal"} })
	dialog := &poster{name: "dialog", log: &log}
	root := rootOver(&log, kid, func(uc *froth.UpdateCtx) { uc.Self().Reconcile() })
	root.onReconcile = func(ac *froth.ApplyCtx) { ac.Self().OpenModal(dialog) }
	f := newSized(t, root, 4, 1)

	res := f.Update(bumpMsg{})()
	f.Update(posted{"open"})
	if f.Query().Focus() != dialog.self {
		t.Fatal("the modal did not open")
	}
	f.Update(res)

	if !slices.Equal(log, []string{"root:open", "kid:under the modal"}) {
		t.Errorf("delivered %v, want the result at kid under the modal", log)
	}
}
