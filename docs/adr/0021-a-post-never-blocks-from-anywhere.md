# A post never blocks, from anywhere

A surface's post capability — the function it hands to a goroutine, a service or a
timer so that they can reach it — works the same whether it is called on the update
loop or off it, and it never blocks. It appends to an inbox froth owns, and the loop
comes round and drains it.

This is the mechanism under [ADR 0015](./0015-results-come-back-to-whoever-asked.md),
which decided *who* gets a result and *when* work stops but left *how a message gets in*
to the milestone that would build it. Building it turned up a footgun in the host that
the ADR had not seen, and the design here is shaped by it.

## The door Bubble Tea offers blocks

Bubble Tea runs one goroutine that reads a channel and calls `Update` for each message.
`Program.Send` is the public way to put a message on that channel from anywhere else,
and the channel is unbuffered (`make(chan Msg)`), so `Send` returns only when the loop
goroutine is at the read. Called from inside `Update` it deadlocks: the loop is busy
running `Update` and cannot read. Bubble Tea's own internals never call it bare from the
loop; they write `go p.Send(...)`. All of this is read from v2.0.9 source in
[the survey](../research/bubbletea-v2-commands-and-send.md).

froth cannot route around it by asking "am I on the loop?". Go exposes no goroutine
identity, and reading a phase-in-progress flag from another goroutine is a data race.
Whatever the post capability does, it has to do the same thing from anywhere.

## The inbox and one wake

froth owns a mutex-guarded inbox. A post appends to it under the lock. If no wake is
outstanding, it starts one goroutine whose only job is to `Send` a payload-free wake
message and exit. At the top of the next `Update`, froth drains the inbox into its
ordinary queue and settles as it does for everything else.

On the loop the append is instant and the wake goroutine blocks harmlessly until
`Update` returns. Off the loop it is the same code. Order is inbox order — FIFO across
every poster. Payloads are never collapsed; the wake is collapsed to at most one
outstanding, which is the split eleven of the systems in
[the stream survey](../research/streams-into-a-ui-loop.md) make without documenting it.

**Liveness is decided under the same lock.** The post reports false when its surface is
gone, and the unmount walk marks a surface dead under the inbox lock, so the answer is
not a race. Handle values are never reused — the id is monotonic per registry and the
registry id is part of the handle — so true means what it says.

**There is no back-pressure.** Bubble Tea's blocking `Send` was the only flow control in
the stack and it is removed on purpose. Every UI queue in the survey is unbounded except
Win32, which fails the post at ten thousand, and tcell, which silently drops at ten. A
cap is a decision about what to drop, and the loop does not know what a message means. A
producer that emits faster than the loop drains uses the producer subpackage's collapsing
modes; a raw poster that floods has the same problem it would have anywhere.

## Attaching the program

froth is built before the program that runs it, because the program takes the model that
holds froth, and nothing in `Init`, `Update` or `View` hands the model a sender. So the
application attaches after `tea.NewProgram` and before `Run`:

```go
f := froth.New(ctx, root)
p := tea.NewProgram(model{f: f})
f.Attach(p)
```

`Attach` takes an interface with `Send` only, which `*tea.Program` satisfies. Taking the
concrete program would force every test that touches a post to build one, and a real
program's `Send` blocks with nobody reading. Widening the interface later breaks only
test fakes. Holding the whole program would also let froth `Kill` from inside a phase
someday, which under [ADR 0012](./0012-dangerous-capability-is-named-and-imported.md) is
a deliberate widening, not a drift.

A post before attach queues in the inbox and the wake is owed rather than sent; `Attach`
sends it, and it blocks until the program is running. Forgetting to attach therefore fails
silently, so the drain that delivers those posts reports a diagnostic.

## Command envelopes

`Run(cmd)` wraps the command so its result comes back addressed to the issuing handle.
The wrapper runs on the command's goroutine after the function returns and inspects the
result. `QuitMsg` and `InterruptMsg` pass through untouched, because the loop consumes
them before `Update` and a wrapped one would silently stop the program from quitting.
`BatchMsg` and the unexported sequence message are rewrapped inner command by inner
command, so a batch still runs and its results come back addressed; the survey probed
that the sequence type converts through `reflect` into a value the loop still matches.
Everything else returns an addressed type that `route` unwraps. In v2.0.9 those four are
the whole pass-through set — nothing else skips `Update` — and the v1 mode messages the
spec worried about are `View` fields now.

An addressed result goes back through Bubble Tea's channel, not the inbox: one type, one
line in `route`, and the command path stays native.

## A result for a surface that has gone is dropped silently

Cancellation is cooperative here as in every system the
[addressing survey](../research/async-results-addressing.md) found that has it. A result
that arrives after destroy is the expected outcome of destroying a surface with work in
flight, and diagnosing the expected path is what React shipped and then removed in 18.0.
So an addressed result or an inbox post for a dead handle is dropped with no diagnostic.
The poster already has the bool; a command result has nobody to tell.

`DiagDeadHandle` stays for its existing case: a phase context minted on the loop for a
dead handle, which is a surface holding a stale handle and worth a warning.

## Where the capability is minted

`Ctx` and the post capability are on `UpdateCtx` and `ApplyCtx`, since a goroutine starts
in `Mount` and that is Apply. Not on `RenderCtx`: render starts no work. Not through
`At(h)`: a capability tied to another surface's lifetime is odd to hold, and a surface
that wants work done on another's behalf posts it a message and lets it own the outcome.

## Producers leave the core

With `Ctx` and a non-blocking post, everything the sketch's `Produce` did in the core —
address results, cancel on destroy, drop for the dead — comes free. What is left is the
loop over a step function and collapsing pending messages, and neither needs froth's
queue: a helper can buffer on its own side and post one wake, exactly as the inbox does.
Batching saves `Update` calls and nothing else, because the renderer already coalesces
frames at its own rate, so it did not earn a place in the core.

A producer subpackage owns the step-function loop, batching, coalescing, and
cancel-previous by key, as **one implementation** tested under `testing/synctest`. The
injectable runner with a test double is not built: the stream survey found no UI
framework that ships two runners, and gpui and Flutter avoided the fidelity problem by
having one implementation driven step by step.

## Considered and not built

- **Bare `Send`.** Deadlocks from inside `Update`; see above.
- **Detecting the phase and enqueueing directly on the loop.** No goroutine identity;
  a data race. Would also give one call two orderings depending on where it ran.
- **A bounded inbox.** A cap chooses what to drop, and nothing in the loop can.
- **froth building the program.** Cleaner call site, but it takes over Bubble Tea's
  program options and makes froth a framework around the app rather than a library in
  it. Sugar that can be layered on `Attach` later.
- **Passing the batch through untagged.** A wrapped batch does not lose its tags — it
  never runs, because the loop matches by concrete type. Rewrapping is the only option
  that keeps batches working.

## Costs

An unbounded inbox. One goroutine per wake, short-lived. A mutex in a codebase that has
needed none, and the discipline that liveness is read under it. Every application gains
a third bootstrap line, with a diagnostic rather than a compile error if it is forgotten.

## Amended when the producer subpackage was built

Building `producer` settled the shape the section above left open, recorded here rather
than by rewriting it:

- **The surface pulls.** A producer cannot see when `Update` runs, so it cannot know how
  much to deliver; it folds items into an accumulator under its own lock and posts one
  wake the caller named, the same split as the inbox. The surface receives the wake and
  calls `Take`, which returns the accumulator and resets it. The wake is caller-named
  rather than a package type carrying the producer, because a `case` on the caller's own
  message type reads like every other Bubble Tea `Update` and tells two producers of one
  accumulator type apart.
- **One primitive, a fold.** Collect-all, keep-first-n, keep-last-n and latest-wins are
  each a merge function over the same loop; the sketch's `BatchedFunc` was the same idea
  done late over a list. The unbuffered "one `Update` per item" mode is not built: a
  goroutine calling the post capability already is that, and `Collect` with a range in
  the surface delivers the same items in the same order in fewer `Update` calls.
- **`Take` reports the end like a reader.** `io.EOF` is the clean end, the step's error
  otherwise, `ctx.Err()` after cancellation. A partial accumulator arrives in the same
  call as the error, which answers the Rx `buffer` hazard in the stream survey without a
  special case, and once non-nil the error stays.
- **The field is the key.** Cancel-previous is `Stop` on the producer the surface holds
  and a new one assigned to the field. `Stop` suppresses any wake not yet posted and
  leaves the accumulator for a `Take`. No keyed registry: the frameworks that key by an id
  do so because they hold the registry, and here the surface does.

**Parked: a keyed holder** (`Start(key, …)` that stops what ran under `key`). Sugar over
`Stop`; revive it for a call site whose producers are not fields on the surface, and bring
that call site.

**Parked: merge in the inbox.** A posted message that says how to merge with the pending
post before it, Textual's `can_replace` and Qt's `compressEvent`, one branch in
`inbox.post`. It contradicts "payloads are never collapsed" above and would need an
amendment; revive it for a poster that is not a producer and needs collapse.

