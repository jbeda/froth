# Testing froth

## One seam

Tests construct a `froth.Froth` with a root surface, post synthetic Bubble Tea messages
through its update entry point, and assert on what comes back. No terminal, no TTY, no
sleeping. That is exactly how an application uses froth, so tests exercise the real
public entry point and nothing else.

Subtree compositing is a narrower view through the same seam rather than a second one:
it returns the composited string for a subtree plus the resolved rect for each surface in
it, so a test can drive one surface and assert on just its region and position. "Is the
button where I think it is" is the assertion that catches layout regressions, and it
needs the rects as much as the string.

## What makes a good test here

Assert on rendered output and resolved rects. Never on registry internals, handle
values, phase bookkeeping, or the shape of the message queue.

A test that breaks when the registry's internal representation changes is testing the
wrong thing. A test that breaks when a modal stops receiving focus is testing the right
one.

Almost everything is observable through the seam: shortcut precedence, focus movement and
restoration, intent routing and unclaimed-intent reporting, modality, capture and capture
loss, command tagging and dead-handle drops, runtime phase violations, chrome, and Z
ordering.

## Layout, tested directly

A `Layout` is an interface users implement, so its space-and-measures to rects mapping is
published API rather than an internal detail. Testing it directly as a pure function is
testing a contract, not reaching inside. This is the only sanctioned second attachment
point.

## Producers: two levels and a fidelity check

Never with real goroutines or sleeps, except where stated.

**The step function is an ordinary function.** A producer is `func(ctx) (T, error)`, so
it is tested by calling it with a pre-filled channel — no goroutine, no froth, no timing.
That covers the body a surface author actually writes.

**Delivery is tested through the injectable runner.** The production runner spawns a
goroutine per producer and drives its step function; the test runner calls the step
function only when the test says so and delivers synchronously into froth's queue.
Because froth owns the only loop, the test runner has almost nothing left to fake.

That makes the edge cases deterministic: an emission arriving after its surface is
destroyed, several pending within one frame under each delivery mode, cancellation
mid-stream, end-of-stream termination, and a producer returning an error.

**A small set of tests uses `testing/synctest` against the real runner**, checking that
it cancels on destroy, terminates on end-of-stream, and leaks no goroutines. This is the
standing risk of any injected double — every test passing against a fake that behaves
differently from the real thing — and it is the only place real concurrency appears in
the suite. `testing/synctest` is generally available from Go 1.25; froth requires 1.27.

## Panic quarantine needs a fixture

A deliberately panicking surface, asserting that the diagnostic fires, that the surface
stops receiving messages, and that the rest of the tree keeps working. This behaviour
regresses silently, so it needs a test rather than a paragraph.

## Compile-time is part of the strategy

Each phase has its own context type, so most illegal operations are compile errors rather
than test cases. Do not write tests asserting that structural change during render fails
— it does not compile. The runtime check covers only what types cannot: a surface
retaining a context across phases.

## No prior art

There is no code yet. These tests establish the conventions rather than following them.
