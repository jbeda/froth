package froth_test

import (
	"context"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/jbeda/froth"
)

// cancelled reports whether a context is already done, without waiting for one
// that is not: every cancellation here happens on the test's own goroutine, so
// there is nothing to wait for.
func cancelled(ctx context.Context) bool {
	select {
	case <-ctx.Done():
		return true
	default:
		return false
	}
}

// A surface has one context, however many times and from whichever phase it
// asks for it.
func TestASurfaceAsksForItsContextAndGetsTheSameOneEveryTime(t *testing.T) {
	var got []context.Context
	root := &leaver{name: "root"}
	root.onMount = func(ac *froth.ApplyCtx) { got = append(got, ac.Ctx()) }
	root.onMsg = func(uc *froth.UpdateCtx, _ tea.Msg) {
		got = append(got, uc.Ctx(), uc.Ctx())
	}
	f := newSized(t, root, 4, 1)

	f.Update(bumpMsg{})
	f.Update(bumpMsg{})

	if len(got) != 5 {
		t.Fatalf("the surface was asked %d times, want 5", len(got))
	}
	for i, ctx := range got {
		if ctx != got[0] {
			t.Errorf("ask %d returned a different context from the first", i)
		}
	}
}

// Destroying a subtree cancels every context in it, and a surface tearing down
// sees its children already cancelled and itself still live.
func TestDestroyingASubtreeCancelsItsContextsLeafUp(t *testing.T) {
	var rootCtx, sibCtx, topCtx, midCtx, leafCtx context.Context
	var midWasCancelled, topWasLive bool

	leaf := &leaver{name: "leaf"}
	leaf.onMount = func(ac *froth.ApplyCtx) { leafCtx = ac.Ctx() }
	mid := &leaver{name: "mid", kids: []froth.Surface{leaf}}
	mid.onMount = func(ac *froth.ApplyCtx) { midCtx = ac.Ctx() }
	top := &leaver{name: "top", kids: []froth.Surface{mid}}
	top.onMount = func(ac *froth.ApplyCtx) { topCtx = ac.Ctx() }
	top.onUnmount = func(ac *froth.ApplyCtx) {
		midWasCancelled = cancelled(midCtx)
		topWasLive = !cancelled(ac.Ctx())
	}
	sib := &leaver{name: "sib"}
	sib.onMount = func(ac *froth.ApplyCtx) { sibCtx = ac.Ctx() }

	root := &leaver{name: "root", kids: []froth.Surface{top, sib}}
	root.onMount = func(ac *froth.ApplyCtx) { rootCtx = ac.Ctx() }
	root.onMsg = func(uc *froth.UpdateCtx, _ tea.Msg) { uc.Child(top.self).Destroy() }
	f := newSized(t, root, 4, 1)

	f.Update(bumpMsg{})

	for _, c := range []struct {
		name string
		ctx  context.Context
	}{{"top", topCtx}, {"mid", midCtx}, {"leaf", leafCtx}} {
		if !cancelled(c.ctx) {
			t.Errorf("%s left the tree with its context still live", c.name)
		}
	}
	if !midWasCancelled {
		t.Error("top unmounted with its child's context still live")
	}
	if !topWasLive {
		t.Error("top unmounted with its own context already cancelled")
	}
	if cancelled(rootCtx) {
		t.Error("destroying a child cancelled the root's context")
	}
	if cancelled(sibCtx) {
		t.Error("destroying a surface cancelled a sibling's context")
	}
}

// Shutdown is the same walk from the root, so it cancels everything.
func TestShutdownCancelsTheContextsItUnmounts(t *testing.T) {
	var rootCtx, kidCtx context.Context
	kid := &leaver{name: "kid"}
	kid.onMount = func(ac *froth.ApplyCtx) { kidCtx = ac.Ctx() }
	root := &leaver{name: "root", kids: []froth.Surface{kid}}
	root.onMount = func(ac *froth.ApplyCtx) { rootCtx = ac.Ctx() }
	f := newSized(t, root, 4, 1)

	f.Shutdown()

	if !cancelled(rootCtx) {
		t.Error("shutdown left the root's context live")
	}
	if !cancelled(kidCtx) {
		t.Error("shutdown left a child's context live")
	}
}

// Every surface's context hangs off the instance's, so cancelling that one
// cancels them all.
func TestCancellingTheInstanceContextCancelsTheSurfaceContextsUnderIt(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var got context.Context
	root := &leaver{name: "root"}
	root.onMount = func(ac *froth.ApplyCtx) { got = ac.Ctx() }
	f := froth.New(ctx, root)
	f.Update(tea.WindowSizeMsg{Width: 4, Height: 1})

	if got == nil {
		t.Fatal("the root never reached its mount")
	}
	if cancelled(got) {
		t.Fatal("the surface's context was cancelled before the instance's was")
	}
	cancel()
	if !cancelled(got) {
		t.Error("cancelling the instance's context left the surface's live")
	}
}
