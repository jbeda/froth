package producer_test

import (
	"context"
	"errors"
	"io"
	"slices"
	"sync"
	"testing"
	"testing/synctest"

	tea "charm.land/bubbletea/v2"
	"github.com/jbeda/froth"
	"github.com/jbeda/froth/producer"
)

type ready struct{}

// wakes records what a producer posted, standing in for the surface's post
// capability. Everything a producer posts is a wake, so the count is the
// assertion.
type wakes struct {
	mu     sync.Mutex
	posted []tea.Msg
	dead   bool
}

func (w *wakes) post(msg tea.Msg) bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.posted = append(w.posted, msg)
	return !w.dead
}

// count reports how many wakes were posted once every goroutine has settled.
func (w *wakes) count() int {
	synctest.Wait()
	w.mu.Lock()
	defer w.mu.Unlock()
	return len(w.posted)
}

// source is a step function over a channel: the items a test feeds, then
// the end it chooses.
type source struct {
	items chan int
	end   chan error
}

func newSource() *source {
	return &source{items: make(chan int), end: make(chan error)}
}

func (s *source) step(ctx context.Context) (int, error) {
	select {
	case <-ctx.Done():
		return 0, ctx.Err()
	case n := <-s.items:
		return n, nil
	case err := <-s.end:
		return 0, err
	}
}

func (s *source) feed(ns ...int) {
	for _, n := range ns {
		s.items <- n
	}
	synctest.Wait()
}

func TestCollectFoldsEverythingPendingIntoOneTake(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		w, s := &wakes{}, newSource()
		p := producer.Collect(t.Context(), froth.PostFunc(w.post), ready{}, s.step)

		s.feed(1, 2, 3)
		if got := w.count(); got != 1 {
			t.Fatalf("posted %d wakes for three items, want 1", got)
		}
		got, err := p.Take()
		if !slices.Equal(got, []int{1, 2, 3}) || err != nil {
			t.Errorf("Take = %v, %v; want [1 2 3], nil", got, err)
		}

		// The wake was serviced, so the next item owes another; a second
		// Take with nothing pending returns the zero value.
		s.feed(4)
		if got := w.count(); got != 2 {
			t.Fatalf("posted %d wakes after Take, want 2", got)
		}
		got, _ = p.Take()
		got2, _ := p.Take()
		if !slices.Equal(got, []int{4}) || got2 != nil {
			t.Errorf("Takes = %v then %v; want [4] then nil", got, got2)
		}
		p.Stop()
	})
}

func TestBoundedModesKeepWhatTheyPromise(t *testing.T) {
	cases := []struct {
		name  string
		start func(ctx context.Context, post froth.PostFunc, step func(context.Context) (int, error)) *producer.Producer[[]int]
		want  []int
	}{
		{"first", func(ctx context.Context, post froth.PostFunc, step func(context.Context) (int, error)) *producer.Producer[[]int] {
			return producer.First(ctx, post, ready{}, 2, step)
		}, []int{1, 2}},
		{"last", func(ctx context.Context, post froth.PostFunc, step func(context.Context) (int, error)) *producer.Producer[[]int] {
			return producer.Last(ctx, post, ready{}, 2, step)
		}, []int{4, 5}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				w, s := &wakes{}, newSource()
				p := c.start(t.Context(), w.post, s.step)

				s.feed(1, 2, 3, 4, 5)
				got, _ := p.Take()
				if !slices.Equal(got, c.want) {
					t.Errorf("Take = %v, want %v", got, c.want)
				}
				if got := w.count(); got != 1 {
					t.Errorf("posted %d wakes, want 1", got)
				}
				p.Stop()
			})
		})
	}
}

func TestLatestKeepsTheNewestItem(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		w, s := &wakes{}, newSource()
		p := producer.Latest(t.Context(), froth.PostFunc(w.post), ready{}, s.step)

		s.feed(10, 20, 30)
		if got, _ := p.Take(); got != 30 {
			t.Errorf("Take = %d, want 30", got)
		}
		p.Stop()
	})
}

func TestFoldRunsTheCallersMerge(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		w, s := &wakes{}, newSource()
		p := producer.Fold(t.Context(), froth.PostFunc(w.post), ready{}, s.step,
			func(sum int, n int) int { return sum + n })

		s.feed(1, 2, 3)
		if got, _ := p.Take(); got != 6 {
			t.Errorf("Take = %d, want 6", got)
		}
		p.Stop()
	})
}

// A clean end and a step error both arrive with whatever was pending, in one
// Take, and keep being reported afterwards.
func TestTheEndArrivesWithThePartialAccumulator(t *testing.T) {
	boom := errors.New("boom")
	for _, end := range []error{io.EOF, boom} {
		t.Run(end.Error(), func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				w, s := &wakes{}, newSource()
				p := producer.Collect(t.Context(), froth.PostFunc(w.post), ready{}, s.step)

				s.feed(1, 2)
				s.end <- end
				synctest.Wait()
				if got := w.count(); got != 1 {
					t.Fatalf("posted %d wakes for items then the end, want 1", got)
				}
				got, err := p.Take()
				if !slices.Equal(got, []int{1, 2}) || !errors.Is(err, end) {
					t.Errorf("Take = %v, %v; want [1 2], %v", got, err, end)
				}
				got, err = p.Take()
				if got != nil || !errors.Is(err, end) {
					t.Errorf("second Take = %v, %v; want nil, %v again", got, err, end)
				}
			})
		})
	}
}

func TestAnEndAfterATakeWakesAgain(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		w, s := &wakes{}, newSource()
		p := producer.Collect(t.Context(), froth.PostFunc(w.post), ready{}, s.step)

		s.feed(1)
		_, _ = p.Take()
		s.end <- io.EOF
		if got := w.count(); got != 2 {
			t.Fatalf("posted %d wakes, want a second for the end", got)
		}
		if got, err := p.Take(); got != nil || !errors.Is(err, io.EOF) {
			t.Errorf("Take = %v, %v; want nil, EOF", got, err)
		}
	})
}

func TestStopSuppressesTheWakeAndKeepsWhatArrived(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		w, s := &wakes{}, newSource()
		p := producer.Collect(t.Context(), froth.PostFunc(w.post), ready{}, s.step)

		s.feed(1)
		_, _ = p.Take()
		s.feed(2)
		// A wake is owed for item 2 and that one still finds the item; the
		// suppression itself is pinned by TestStopBeforeAnyWakePostsNothing.
		p.Stop()
		p.Stop()
		if got := w.count(); got != 2 {
			t.Fatalf("posted %d wakes, want 2", got)
		}
		got, err := p.Take()
		if !slices.Equal(got, []int{2}) || !errors.Is(err, context.Canceled) {
			t.Errorf("Take = %v, %v; want [2], context.Canceled", got, err)
		}
	})
}

func TestStopBeforeAnyWakePostsNothing(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		w, s := &wakes{}, newSource()
		p := producer.Collect(t.Context(), froth.PostFunc(w.post), ready{}, s.step)

		p.Stop()
		if got := w.count(); got != 0 {
			t.Errorf("posted %d wakes, want none", got)
		}
		if _, err := p.Take(); !errors.Is(err, context.Canceled) {
			t.Errorf("Take reported %v, want context.Canceled", err)
		}
	})
}

// The surface's context is cancelled as it leaves the tree; the loop ends
// and the wake is posted, which the post capability then drops.
func TestTheSurfacesContextEndsTheLoop(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		w, s := &wakes{}, newSource()
		ctx, cancel := context.WithCancel(t.Context())
		p := producer.Collect(ctx, froth.PostFunc(w.post), ready{}, s.step)

		s.feed(1)
		_, _ = p.Take()
		w.dead = true
		cancel()
		if got := w.count(); got != 2 {
			t.Errorf("posted %d wakes, want one for the cancel", got)
		}
		if _, err := p.Take(); !errors.Is(err, context.Canceled) {
			t.Errorf("Take reported %v, want context.Canceled", err)
		}
	})
}

// A step function that never looks at its context still stops, because the
// loop checks the context for it.
func TestTheLoopHonoursCancellationForAStepThatIgnoresIt(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		w := &wakes{}
		items := make(chan int)
		p := producer.Collect(t.Context(), froth.PostFunc(w.post), ready{},
			func(context.Context) (int, error) { return <-items, nil })

		items <- 1
		p.Stop()
		// The step is blocked on the channel and returns an item after the
		// cancel; the loop ends there rather than folding it.
		items <- 2
		synctest.Wait()
		got, err := p.Take()
		if !slices.Equal(got, []int{1}) || !errors.Is(err, context.Canceled) {
			t.Errorf("Take = %v, %v; want [1], context.Canceled", got, err)
		}
	})
}

// A step function is an ordinary function: a test calls it with a pre-filled
// channel and no producer at all.
func TestAStepFunctionIsCalledDirectly(t *testing.T) {
	s := newSource()
	go func() { s.items <- 7; s.end <- io.EOF }()
	if n, err := s.step(t.Context()); n != 7 || err != nil {
		t.Errorf("step = %d, %v; want 7, nil", n, err)
	}
	if _, err := s.step(t.Context()); !errors.Is(err, io.EOF) {
		t.Errorf("step reported %v, want EOF", err)
	}
}

func TestABoundOfZeroIsAContractViolation(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("First(0) did not panic")
		}
	}()
	producer.First[int](t.Context(), func(tea.Msg) bool { return true }, ready{}, 0, nil)
}
