package froth_test

import (
	"slices"
	"sync"
	"testing"
	"testing/synctest"

	tea "charm.land/bubbletea/v2"
	"github.com/jbeda/froth"
)

// fakeProgram stands in for *tea.Program: it records what was sent rather
// than blocking on a loop nobody is running, and the test hands each wake to
// Update itself.
type fakeProgram struct {
	mu   sync.Mutex
	sent []tea.Msg
}

func (p *fakeProgram) Send(m tea.Msg) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.sent = append(p.sent, m)
}

// wakes reports how many messages the inbox has sent, once every goroutine that
// might send one has finished.
func (p *fakeProgram) wakes() int {
	synctest.Wait()
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.sent)
}

// deliverWake hands the last wake sent to Update, the way Bubble Tea would.
func (p *fakeProgram) deliverWake(t *testing.T, f *froth.Froth) {
	t.Helper()
	synctest.Wait()
	p.mu.Lock()
	n := len(p.sent)
	var last tea.Msg
	if n > 0 {
		last = p.sent[n-1]
	}
	p.mu.Unlock()
	if n == 0 {
		t.Fatal("no wake was sent")
	}
	f.Update(last)
}

type posted struct{ tag string }

// poster is a surface that mints its post capability in Mount and records what
// reaches it, in a log shared with every other poster in the test.
type poster struct {
	name  string
	body  string
	self  froth.Handle
	post  froth.PostFunc
	log   *[]string
	kids  []froth.Surface
	kidHs []froth.Handle
	onMsg func(uc *froth.UpdateCtx, msg tea.Msg)
	// seen is every message delivered, whatever its type.
	seen []tea.Msg
	// onReconcile runs in the Apply a Reconcile request asks for, which is
	// where a modal is opened from.
	onReconcile func(ac *froth.ApplyCtx)
	onMount     func(ac *froth.ApplyCtx)
	onUnmount   func(ac *froth.ApplyCtx)
}

func (p *poster) Unmount(ac *froth.ApplyCtx) {
	if p.onUnmount != nil {
		p.onUnmount(ac)
	}
}

func (p *poster) Reconcile(ac *froth.ApplyCtx) {
	if p.onReconcile != nil {
		p.onReconcile(ac)
	}
}

func (p *poster) Render(*froth.RenderCtx, froth.Size) string {
	if p.body != "" {
		return p.body
	}
	return "."
}

func (p *poster) Mount(ac *froth.ApplyCtx) {
	p.self = ac.Self().Handle()
	p.post = ac.PostFunc()
	for _, k := range p.kids {
		p.kidHs = append(p.kidHs, ac.Self().Create(k))
	}
	if p.onMount != nil {
		p.onMount(ac)
	}
}

func (p *poster) Update(uc *froth.UpdateCtx, msg tea.Msg) {
	p.seen = append(p.seen, msg)
	if m, ok := msg.(posted); ok {
		*p.log = append(*p.log, p.name+":"+m.tag)
	}
	if p.onMsg != nil {
		p.onMsg(uc, msg)
	}
}

// attached builds a sized instance with a fake program attached.
func attached(t *testing.T, root froth.Surface) (*froth.Froth, *fakeProgram) {
	t.Helper()
	f := newSized(t, root, 4, 1)
	p := &fakeProgram{}
	f.Attach(p)
	return f, p
}

func TestCompositeDrainsPosts(t *testing.T) {
	var log []string
	root := &poster{name: "root", log: &log}
	root.onMsg = func(*froth.UpdateCtx, tea.Msg) { root.body = "1" }
	f := newSized(t, root, 4, 1)

	if !root.post(posted{"outside"}) {
		t.Fatal("post to a mounted root failed")
	}
	frame := f.Composite(f.Root())

	if got, want := frame.Content, "1"; got != want {
		t.Errorf("Composite content = %q, want %q", got, want)
	}
	if !slices.Equal(log, []string{"root:outside"}) {
		t.Errorf("delivered %v, want the queued post", log)
	}
}

func TestPostsArriveInOrderAcrossPosters(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		var log []string
		a := &poster{name: "a", log: &log}
		b := &poster{name: "b", log: &log}
		root := &poster{name: "root", log: &log, kids: []froth.Surface{a, b}}
		f, p := attached(t, root)

		a.post(posted{"1"})
		b.post(posted{"2"})
		a.post(posted{"3"})
		p.deliverWake(t, f)

		want := []string{"a:1", "b:2", "a:3"}
		if !slices.Equal(log, want) {
			t.Errorf("delivered %v, want %v", log, want)
		}
	})
}

func TestOneWakeIsOutstandingHoweverManyPosts(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		var log []string
		root := &poster{name: "root", log: &log}
		f, p := attached(t, root)

		root.post(posted{"1"})
		root.post(posted{"2"})
		root.post(posted{"3"})
		if got := p.wakes(); got != 1 {
			t.Fatalf("sent %d wakes for three posts, want 1", got)
		}
		p.deliverWake(t, f)
		if len(log) != 3 {
			t.Fatalf("delivered %v, want three", log)
		}

		// The wake has been serviced, so the next post owes a new one.
		root.post(posted{"4"})
		if got := p.wakes(); got != 2 {
			t.Fatalf("sent %d wakes after the first was serviced, want 2", got)
		}
	})
}

func TestAnyUpdateDrainsTheInbox(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		var log []string
		root := &poster{name: "root", log: &log}
		f, p := attached(t, root)

		root.post(posted{"1"})
		f.Update(bumpMsg{})
		if !slices.Equal(log, []string{"root:1"}) {
			t.Fatalf("delivered %v before the wake, want the post", log)
		}
		// The wake still arrives; it finds nothing, delivers nothing twice, and
		// is not itself a message any surface sees.
		p.deliverWake(t, f)
		if len(log) != 1 {
			t.Errorf("delivered %v after the wake, want no repeat", log)
		}
		if len(root.seen) != 2 {
			t.Errorf("the root saw %v, want only the bump and the post", root.seen)
		}
	})
}

func TestAPostFromInsideAPhaseArrivesAtTheNextUpdate(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		var log []string
		root := &poster{name: "root", log: &log}
		root.onMsg = func(uc *froth.UpdateCtx, msg tea.Msg) {
			if _, ok := msg.(bumpMsg); ok {
				if !root.post(posted{"from update"}) {
					t.Error("a post from Update reported the surface dead")
				}
			}
		}
		f, p := attached(t, root)

		f.Update(bumpMsg{})
		if len(log) != 0 {
			t.Fatalf("delivered %v in the same update; a post is not delivered re-entrantly", log)
		}
		p.deliverWake(t, f)
		if !slices.Equal(log, []string{"root:from update"}) {
			t.Errorf("delivered %v, want the post", log)
		}
	})
}

func TestAPostToADestroyedSurfaceReportsFalseAndDropsSilently(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		var log []string
		kid := &poster{name: "kid", log: &log}
		root := &poster{name: "root", log: &log, kids: []froth.Surface{kid}}
		root.onMsg = func(uc *froth.UpdateCtx, msg tea.Msg) {
			if _, ok := msg.(bumpMsg); ok {
				uc.Child(root.kidHs[0]).Destroy()
			}
		}
		f, got := newWatched(t, root, 4, 1)
		p := &fakeProgram{}
		f.Attach(p)

		if !kid.post(posted{"alive"}) {
			t.Fatal("a post to a live surface reported false")
		}
		f.Update(bumpMsg{})
		if kid.post(posted{"dead"}) {
			t.Error("a post to a destroyed surface reported true")
		}
		p.deliverWake(t, f)
		if !slices.Equal(log, []string{"kid:alive"}) {
			t.Errorf("delivered %v, want only the post made while alive", log)
		}
		if len(*got) != 0 {
			t.Errorf("reported %v; a post to a dead surface drops silently", *got)
		}
	})
}

// Nothing drains after Shutdown, so a post made during its walk or after it
// reports false rather than queueing for nobody.
func TestAPostDuringOrAfterShutdownReportsFalse(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		var log []string
		var duringWalk bool
		root := &poster{name: "root", log: &log}
		root.onUnmount = func(*froth.ApplyCtx) { duringWalk = root.post(posted{"unmounting"}) }
		f, p := attached(t, root)
		f.Shutdown()
		if duringWalk {
			t.Error("a post from Unmount during Shutdown reported true")
		}
		if root.post(posted{"late"}) {
			t.Error("a post after Shutdown reported true")
		}
		if got := p.wakes(); got != 0 {
			t.Errorf("sent %d wakes for posts nothing will drain", got)
		}
	})
}

// A goroutine started in Mount can post before Mount has returned; the post
// arrives at the next Update like any other.
func TestAPostFromMountArrivesAtTheNextUpdate(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		var log []string
		var fromMount bool
		root := &poster{name: "root", log: &log}
		root.onMount = func(ac *froth.ApplyCtx) { fromMount = ac.PostFunc()(posted{"from mount"}) }
		f, p := attached(t, root)

		if !fromMount {
			t.Fatal("a post from Mount reported the surface dead")
		}
		if len(log) != 0 {
			t.Fatalf("delivered %v during the mount; a post is not delivered re-entrantly", log)
		}
		p.deliverWake(t, f)
		if !slices.Equal(log, []string{"root:from mount"}) {
			t.Errorf("delivered %v, want the post", log)
		}
	})
}

func TestAPostBeforeAttachQueuesAndIsReportedOnce(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		var log []string
		root := &poster{name: "root", log: &log}
		f, got := newWatched(t, root, 4, 1)

		if !root.post(posted{"early"}) {
			t.Fatal("a post before Attach reported false")
		}
		p := &fakeProgram{}
		if got := p.wakes(); got != 0 {
			t.Fatalf("sent %d wakes with no program attached", got)
		}
		f.Attach(p)
		if got := p.wakes(); got != 1 {
			t.Fatalf("sent %d wakes on attach, want the one owed", got)
		}
		p.deliverWake(t, f)
		if !slices.Equal(log, []string{"root:early"}) {
			t.Errorf("delivered %v, want the early post", log)
		}
		if n := countKind(*got, froth.DiagPostBeforeAttach); n != 1 {
			t.Errorf("reported %d post-before-attach diagnostics, want 1", n)
		}

		root.post(posted{"later"})
		p.deliverWake(t, f)
		if n := countKind(*got, froth.DiagPostBeforeAttach); n != 1 {
			t.Errorf("reported %d after a post made with a program attached, want still 1", n)
		}
	})
}

// A quarantined surface is never called again, so its post capability reports
// false from the moment it panicked rather than from the Apply that removes it.
func TestAPostToAQuarantinedSurfaceReportsFalse(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		var log []string
		kid := &poster{name: "kid", log: &log}
		kid.onMsg = func(uc *froth.UpdateCtx, msg tea.Msg) {
			if _, ok := msg.(posted); ok {
				panic("kid fails")
			}
		}
		root := &poster{name: "root", log: &log, kids: []froth.Surface{kid}}
		root.onMsg = func(uc *froth.UpdateCtx, msg tea.Msg) {
			if _, ok := msg.(bumpMsg); ok {
				kid.post(posted{"boom"})
			}
		}
		f, got := newWatched(t, root, 4, 1)
		p := &fakeProgram{}
		f.Attach(p)

		f.Update(bumpMsg{})
		// The wake delivers the post and the kid panics on it. The probe is
		// queued behind it, so the root's post lands after quarantine and
		// before the Apply that removes the kid.
		var afterPanic bool
		root.onMsg = func(uc *froth.UpdateCtx, msg tea.Msg) {
			if _, ok := msg.(posted); ok {
				afterPanic = kid.post(posted{"again"})
			}
		}
		root.post(posted{"probe"})
		p.deliverWake(t, f)
		if !hasKind(*got, froth.DiagSurfacePanic) {
			t.Fatal("setup: the kid did not panic")
		}
		if afterPanic {
			t.Error("a post to a surface that had just panicked reported true")
		}
	})
}

// route blocks only keys and mouse under a modal. A post is addressed, so it
// reaches a surface the modal covers.
func TestAPostPassesAModal(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		var log []string
		kid := &poster{name: "kid", log: &log}
		root := &poster{name: "root", log: &log, kids: []froth.Surface{kid}}
		root.onMsg = func(uc *froth.UpdateCtx, msg tea.Msg) {
			if _, ok := msg.(bumpMsg); ok {
				uc.Self().Reconcile()
			}
		}
		root.onReconcile = func(ac *froth.ApplyCtx) {
			ac.Self().OpenModal(&poster{name: "dialog", log: &log})
		}
		f, p := attached(t, root)
		f.Update(bumpMsg{})

		kid.post(posted{"under the modal"})
		p.deliverWake(t, f)
		if !slices.Equal(log, []string{"kid:under the modal"}) {
			t.Errorf("delivered %v, want the post to reach the covered surface", log)
		}
	})
}
