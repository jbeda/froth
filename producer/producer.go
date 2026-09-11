// Package producer runs a step function in a goroutine and folds what it
// produces into one value a surface pulls on its own update.
//
// A surface starts a producer in Mount, handing it the surface's context and
// post capability, and names the message it wants to be woken with:
//
//	type linesReadyMsg struct{}
//
//	func (l *logPane) Mount(ac *froth.ApplyCtx) {
//	    l.lines = producer.Collect(ac.Ctx(), ac.PostFunc(), linesReadyMsg{}, l.nextLine)
//	}
//
//	func (l *logPane) Update(uc *froth.UpdateCtx, msg tea.Msg) {
//	    switch msg.(type) {
//	    case linesReadyMsg:
//	        lines, err := l.lines.Take()
//	        l.buf = append(l.buf, lines...)
//	        if err != nil { ... }
//	    }
//	}
//
// The step function is an ordinary function a test calls directly; the
// producer owns the loop, the goroutine and the cancellation. Items that
// arrive between two updates are folded into one accumulator, and the wake is
// posted once for however many arrive, so the loop is never woken more than it
// drains. The surface reads the accumulator with Take, which also reports how
// the producer ended.
//
// To replace a running producer with a new one, Stop the old one and assign the
// new: the field that holds it is the key. A surface that wants one Update per
// item needs no producer; a goroutine calling the post capability is that.
package producer

import (
	"context"
	"fmt"
	"sync"

	tea "charm.land/bubbletea/v2"
	"github.com/jbeda/froth"
)

// A Producer is a running step loop and the accumulator it fills. The
// surface that started it holds it and pulls from it with Take.
type Producer[M any] struct {
	stop context.CancelFunc
	post froth.PostFunc
	wake tea.Msg

	mu sync.Mutex
	// acc is what has been folded since the last Take.
	acc M
	// err is how the loop ended, nil while it runs. Once set it stays.
	err error
	// wakeOwed says the wake has been posted and not yet serviced by a
	// Take, so another item posts nothing. stopped says Stop suppressed
	// every wake from here on.
	wakeOwed bool
	stopped  bool
}

// Fold starts a producer that runs step until it returns an error, folding
// each item into the accumulator with merge. io.EOF is the clean end; any
// other error stops the loop the same way and Take reports it.
//
// wake is posted to the surface through post when an item arrives and when
// the loop ends, but never while a wake is already owed: at most one between
// two Takes. The loop stops when ctx is cancelled, which the surface's own
// context does as it leaves the tree.
func Fold[T, M any](ctx context.Context, post froth.PostFunc, wake tea.Msg,
	step func(context.Context) (T, error), merge func(M, T) M) *Producer[M] {
	if post == nil {
		panic("producer: Fold was given a nil post capability")
	}
	if wake == nil {
		panic("producer: Fold was given a nil wake message")
	}
	ctx, cancel := context.WithCancel(ctx)
	p := &Producer[M]{stop: cancel, post: post, wake: wake}
	go run(ctx, p, step, merge)
	return p
}

// Collect folds every item into a slice, in order. It is unbounded between
// two Takes; First and Last are the bounded forms.
func Collect[T any](ctx context.Context, post froth.PostFunc, wake tea.Msg,
	step func(context.Context) (T, error)) *Producer[[]T] {
	return Fold(ctx, post, wake, step, func(acc []T, item T) []T {
		return append(acc, item)
	})
}

// First keeps the first n items between two Takes and drops the rest
// silently, so it suits a source whose later items can be missed, such as a
// preview; a source whose every item matters wants Collect. n must be
// positive.
func First[T any](ctx context.Context, post froth.PostFunc, wake tea.Msg, n int,
	step func(context.Context) (T, error)) *Producer[[]T] {
	checkBound("First", n)
	return Fold(ctx, post, wake, step, func(acc []T, item T) []T {
		if len(acc) >= n {
			return acc
		}
		return append(acc, item)
	})
}

// Last keeps the newest n items between two Takes, in order, and drops the
// older ones silently, so it suits a tail whose history can be missed; a
// source whose every item matters wants Collect. n must be positive.
func Last[T any](ctx context.Context, post froth.PostFunc, wake tea.Msg, n int,
	step func(context.Context) (T, error)) *Producer[[]T] {
	checkBound("Last", n)
	return Fold(ctx, post, wake, step, func(acc []T, item T) []T {
		if len(acc) >= n {
			acc = append(acc[:0], acc[1:]...)
		}
		return append(acc, item)
	})
}

// Latest keeps only the newest item: Last(1) without the slice. It is the
// right shape for a value that supersedes itself, such as a progress figure,
// and the wrong one for deltas, where every item matters.
func Latest[T any](ctx context.Context, post froth.PostFunc, wake tea.Msg,
	step func(context.Context) (T, error)) *Producer[T] {
	return Fold(ctx, post, wake, step, func(_ T, item T) T { return item })
}

func checkBound(fn string, n int) {
	if n <= 0 {
		panic(fmt.Sprintf("producer: %s was given a bound of %d, which keeps nothing", fn, n))
	}
}

// Take returns what has accumulated since the last Take and resets the
// accumulator, so a second call in the same Update returns the zero value.
// The error is nil while the loop runs, io.EOF after a clean end, the step's
// error otherwise, and ctx.Err() after cancellation or Stop. Like a reader, a
// partial accumulator and the error that ended the loop arrive in the same
// call, and once the error is non-nil it stays so.
func (p *Producer[M]) Take() (M, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	var zero M
	acc := p.acc
	p.acc = zero
	p.wakeOwed = false
	return acc, p.err
}

// Stop cancels the loop and suppresses any wake not yet posted. What had
// accumulated stays for a Take, which then reports context.Canceled, unless
// the loop had already ended, in which case that end stands. Stopping a
// stopped producer does nothing.
func (p *Producer[M]) Stop() {
	p.mu.Lock()
	p.stopped = true
	p.mu.Unlock()
	p.stop()
}

// run is a function rather than a method because it needs the item type and
// a Go method cannot add a type parameter. It ends on the first error step
// returns, waking the surface once more so the error is seen.
func run[T, M any](ctx context.Context, p *Producer[M],
	step func(context.Context) (T, error), merge func(M, T) M) {
	defer p.stop()
	for {
		item, err := step(ctx)
		if err == nil {
			// A step that ignores its context would otherwise run on past
			// Stop; the loop honours the cancellation for it.
			err = ctx.Err()
		}
		if err != nil {
			p.mu.Lock()
			p.err = err
			p.postWake()
			p.mu.Unlock()
			return
		}
		p.mu.Lock()
		p.acc = merge(p.acc, item)
		p.postWake()
		p.mu.Unlock()
	}
}

// postWake posts the wake unless one is already owed or Stop has been called.
// Called with the lock held; post never blocks, so holding it is safe.
func (p *Producer[M]) postWake() {
	if p.wakeOwed || p.stopped {
		return
	}
	p.wakeOwed = true
	p.post(p.wake)
}
