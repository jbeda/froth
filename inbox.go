// The inbox: how a message gets to a surface from outside the update loop.
//
// Bubble Tea's Send blocks until its loop is at the read, so calling it from
// inside Update deadlocks, and nothing can tell which goroutine it is on. So a
// post from anywhere appends here under a lock and wakes the loop with one
// payload-free message sent from a goroutine of its own. See ADR 0021.

package froth

import (
	"fmt"
	"sync"

	tea "charm.land/bubbletea/v2"
)

// A Program is what Attach takes: the interface *tea.Program satisfies, with
// the one method froth calls.
type Program interface {
	Send(tea.Msg)
}

// A PostFunc posts a message to one surface from outside any phase — from a
// goroutine, a service, a timer. It never blocks, from anywhere, including from
// inside a surface's own Update; the message arrives at a later Update. It
// reports false once the surface has left the tree, and the message is then
// dropped without a diagnostic: a late result is the expected outcome of
// destroying a surface with work in flight.
//
// Messages arrive in the order they were posted, across every holder. There is
// no cap and no back-pressure.
type PostFunc func(tea.Msg) bool

// wakeMsg is what the inbox sends the program to make the loop come round and
// drain. It carries nothing; the payloads wait in the inbox.
type wakeMsg struct{}

// inbox is the mutex-guarded queue every PostFunc appends to, and the one
// piece of froth touched from more than one goroutine.
//
// live is decided under the same lock as the append, so the answer a post gets
// is not a race against the unmount walk. wakePending says a wake is owed and,
// once a program is attached, that the goroutine sending it is outstanding;
// it is cleared when the wake arrives at Update, not when the inbox is drained,
// so a drain by some other message never loses the wake a later post relies
// on.
type inbox struct {
	mu          sync.Mutex
	posts       []envelope
	live        map[Handle]bool
	program     Program
	wakePending bool
	// closed says the instance has shut down: nothing will drain, so every
	// post reports false from here on.
	closed bool
	// beforeAttach counts posts made with no program attached, for the
	// diagnostic the next drain reports.
	beforeAttach int
}

// track records h as live, so a post to it succeeds until the unmount walk
// says otherwise.
func (in *inbox) track(h Handle) {
	in.mu.Lock()
	defer in.mu.Unlock()
	if in.live == nil {
		in.live = make(map[Handle]bool)
	}
	in.live[h] = true
}

// retire marks every handle in hs dead, in one critical section.
func (in *inbox) retire(hs []Handle) {
	in.mu.Lock()
	defer in.mu.Unlock()
	for _, h := range hs {
		delete(in.live, h)
	}
}

// close marks every handle dead at once, for Shutdown: a post made during the
// walk it runs has nothing left to drain it.
func (in *inbox) close() {
	in.mu.Lock()
	defer in.mu.Unlock()
	in.closed = true
	in.live = nil
}

// post appends under the lock and starts the wake if none is outstanding. It
// reports whether h was live at the moment of the append.
func (in *inbox) post(h Handle, msg tea.Msg) bool {
	in.mu.Lock()
	defer in.mu.Unlock()
	if in.closed || !in.live[h] {
		return false
	}
	in.posts = append(in.posts, envelope{to: h, msg: msg})
	if in.program == nil {
		in.beforeAttach++
	}
	in.wake()
	return true
}

// wake starts the goroutine that sends one wake, unless one is already owed.
// With no program attached the wake is owed but not sent; attach sends it.
// Called with the lock held.
func (in *inbox) wake() {
	if in.wakePending {
		return
	}
	in.wakePending = true
	if p := in.program; p != nil {
		go p.Send(wakeMsg{})
	}
}

// attach records the program and sends any wake owed to it.
func (in *inbox) attach(p Program) {
	in.mu.Lock()
	defer in.mu.Unlock()
	in.program = p
	if in.wakePending {
		go p.Send(wakeMsg{})
	}
}

// take empties the inbox. wake says the caller is servicing the wake message
// itself, so a later post may send another. It also reports how many of the
// posts it hands back were made before any program was attached.
func (in *inbox) take(wake bool) (posts []envelope, beforeAttach int) {
	in.mu.Lock()
	defer in.mu.Unlock()
	posts, in.posts = in.posts, nil
	beforeAttach, in.beforeAttach = in.beforeAttach, 0
	if wake {
		in.wakePending = false
	}
	return posts, beforeAttach
}

// Attach gives froth the program that runs it, so a PostFunc can wake the
// loop. Call it after tea.NewProgram and before Run:
//
//	f := froth.New(ctx, root)
//	p := tea.NewProgram(model{f: f})
//	f.Attach(p)
//
// A post made before this queues and is delivered once a program is attached
// and running, and the drain that delivers it reports DiagPostBeforeAttach.
func (f *Froth) Attach(p Program) {
	if p == nil {
		bug("froth: Attach called with a nil program")
	}
	f.inbox.attach(p)
}

// drainInbox moves what was posted from outside onto the ordinary queue. wake
// says this Update is the wake message itself.
func (f *Froth) drainInbox(wake bool) {
	posts, beforeAttach := f.inbox.take(wake)
	if beforeAttach > 0 {
		f.report(Diagnostic{
			Kind: DiagPostBeforeAttach,
			Detail: fmt.Sprintf("%d message(s) were posted from outside before "+
				"Attach was called; they were delivered now, but a post with no "+
				"program attached cannot wake the loop, so call Attach after "+
				"tea.NewProgram and before Run", beforeAttach),
		})
	}
	f.queue = append(f.queue, posts...)
}

// postFunc mints the post capability for h.
func (f *Froth) postFunc(h Handle) PostFunc {
	f.inbox.track(h)
	return func(msg tea.Msg) bool { return f.inbox.post(h, msg) }
}
