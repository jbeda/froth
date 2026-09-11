# Results come back to whoever asked

When a surface starts something slow — an RPC, a file read, a timer — the answer is
delivered to that surface, not to the root. When the surface is destroyed, the work is
cancelled. And when an answer arrives that the surface no longer wants, froth delivers it
anyway, because froth has no way to know it is unwanted.

Those are three different questions, and it is worth keeping them apart:

- **Who gets the answer?** froth decides, because only froth knows the tree.
- **When does the work stop?** froth decides, for the same reason.
- **Is this answer still wanted?** The caller decides. froth stays out of it.

The third is the one worth arguing about, so it gets its own section below.

## Getting a message in from outside

froth only runs when the application's `Update` runs, so nothing outside that loop can
reach a surface. Bubble Tea's own way in is `Program.Send`, which is safe to call from any
goroutine and exists, in its own words, for *"allowing messages to be injected from outside
the program for interoperability purposes."*

froth wraps it, one per handle:

```go
type PostFunc func(tea.Msg) bool   // false: that surface is gone, stop posting
```

Whoever holds one can post to exactly one surface. They cannot name a different surface,
and they never see the instance or the Bubble Tea program at all.

The name follows the standard library, which uses two patterns that look alike. `CancelFunc`
and `WalkFunc` are named after a **verb**, and are capabilities handed to a caller.
`HandlerFunc` and `RoundTripperFunc` are named after an **interface that already exists**,
and are adapters that let a plain function satisfy it. There is no `Poster` interface here,
so this is the first kind, and `Post` is already froth's word for it.

**The bool is the part that matters.** A service can keep a surface's `PostFunc` forever
and never be told when that surface dies — posting just starts returning false, and the
service drops it. No registration protocol, no unsubscribe, no teardown that has to run in
the right order. Without the bool, anything long-lived slowly fills up with addresses for
surfaces that no longer exist.

One mechanism covers three things the spec describes separately: a command's result coming
back, a producer sending to the surface it runs for, and an outside service telling whoever
asked.

## Things that talk but do not draw

A model catalog, an RPC client, a cache — these take part in the message flow and render
nothing. They are **ordinary Go objects the application owns**. Not surfaces.

A surface reaches one through the Environment, which already exists for this: *"ambient
values a surface reads from its ancestors — theme, capabilities."* A service is a
capability. It answers by posting back.

Making it a surface instead means adding a node that draws nothing to the render, layout
and focus walks, purely so something can hold a pointer. That is the zero-area surface a
diagnostic ought to be complaining about.

**This is also what stops state going stale.** If a surface holds a *reference* to the
catalog, there is nothing to keep in sync. If it holds a *copy*, every place that writes the
real one has to remember to update the copy too — and the day someone forgets, a menu shows
the wrong default forever, with nothing to catch it. Environment resolves when you read it,
so it is *"never a stale snapshot."*

## Cancelling work when a surface dies

Each surface gets a `context.Context` that is cancelled when it is destroyed. Surface
authors write no teardown for it.

Two implementation notes, measured rather than assumed. On Go 1.27 a `context.WithCancel`
costs 93 ns and 96 bytes:

- **Do not mirror the tree.** Hanging each surface's context off its parent's costs about
  488 bytes per surface, against 120 flat, because every context with children allocates a
  map to hold them. The only thing the tree shape buys is automatic cancellation of a whole
  subtree, and unmount already walks the subtree leaf-up and can cancel as it goes.
- **Create them lazily.** Controls are surfaces, so a froth tree is deep and mostly
  structural, and most surfaces never start anything slow. Make the context the first time
  one is asked for.

The catch is that cancel has to actually run, every time, or the context stays registered
with its parent forever. That is froth's job now, which is the whole point — it is exactly
the teardown a surface used to have to write.

**A surface's context is not an Environment value**, though [ADR 0004](./0004-no-synchronous-send.md)
and `CONTEXT.md` both said it was, and are corrected. Environment belongs to the
application and a subtree can override it. If a surface could be handed a context other than
its own, cancellation would stop meaning anything. It comes from the phase context instead.

## Whether an answer is still wanted

froth does not track this, and will not.

Cancelling on destroy does not help here, because the surface is still alive. Picking a
model three times in a row leaves two answers in flight that nobody wants, and froth
delivers all three.

The reason for staying out is that any answer would be a guess. froth would need a notion of
request groups, somewhere to say "everything before this point is stale", and a rule for
what happens when some of the outstanding work is superseded and some is not. All of that
depends on what the requests *mean*, which only the caller knows.

**The pattern instead: let the service decide, not the surface.** A catalog knows which
fetch is the current one. It can simply not send the older answer. That replaces a check at
every call site with one place per service, and puts the decision where the knowledge
already is.

This is not theoretical. A real Bubble Tea codebase doing this by hand ended up with about
twenty generation counters and request tokens scattered through its UI, one of them with a
four-part condition, and its own design notes had already worked out the rule: *"cancellation
is not the correctness guard."*

## Consequences

**Three user stories need fixing rather than building as written.** Story 9 — a command's
result coming back to its issuer — is unbuilt, along with the command envelopes it needs, so
today every async result lands at the root. Story 21 asks for a per-surface context and the
code's own comment says the opposite; this settles it the story's way. Story 8 says handle
addressing means you need no request tokens: true for a surface that has *died*, false for
one that is alive and has moved on. Its real benefit is the no-unsubscribe property above,
which is bigger than what it claims.

**The root stops being a distributor.** Every message froth cannot address currently goes to
the root surface, which then has to pass it along — the hand-rolled router froth was built to
get rid of, surviving in one spot. Addressed results and service objects remove the need.

## Amended after the M5 design session

Three things the surveys run before building this moved, recorded here rather than by
rewriting the sections above:

- **"Any framework answer would be a guess" is too strong.** Textual's exclusive worker
  groups, SwiftUI's `task(id:)` and Kotlin's `collectLatest` all offer cancel-previous keyed
  by an id the caller names; the framework guesses nothing and supplies only the mechanism.
  Nobody tracks currency, which is what this ADR rejected, but six of thirteen systems ship
  that middle option. froth ships it too, in the producer subpackage over `Ctx` and the post
  capability, not in the core — see [ADR 0021](./0021-a-post-never-blocks-from-anywhere.md),
  whose own amendment records that the key is the field the surface holds the producer in.
- **Delivery to a dead handle drops silently.** Cancellation is cooperative, so a late
  result is the expected outcome of destroying a surface with work in flight. The diagnostic
  the spec asked for is not reported; ADR 0021 says why.
- **The "twenty generation counters" anecdote is uncited and stays that way.** A checkable
  version exists: the bubbles `timer` component declares a generation `tag` and never
  increments it, so its stale check is dead code in both current major versions
  ([survey](../research/async-results-addressing.md)).

**Parked: `Topic`** — a named channel that surfaces subscribe to and outside systems post
to. It sat in the vocabulary with no design, no code and no scope behind it, and it is not
needed: the outside object *is* the topic, and it is application code. Revive it only if
several surfaces have to receive the same event and no object can sensibly own sending it —
and bring that case, not the mechanism.
