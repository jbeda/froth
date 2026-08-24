# Async work is addressed and lifetime-scoped; its currency is the caller's

Three separable questions hide inside "how does froth handle async", and froth answers
them differently:

- **Addressing** — a result comes back to the surface that asked for it, never to the root.
- **Lifetime** — a surface's in-flight work is cancelled when the surface is destroyed.
- **Currency** — whether a result that arrives is still *wanted* is not froth's problem.

The first two are froth's because only froth knows the tree. The third is not, and saying
so is the substance of this decision.

## Outside systems reach in through an addressed post

froth is driven by an application's `Update`, so nothing outside the update loop can
address a surface. Bubble Tea's own escape hatch is `Program.Send`, which is goroutine-safe
and documented as *"allowing messages to be injected from outside the program for
interoperability purposes."* froth wraps that per handle:

```go
type PostFunc func(tea.Msg) bool   // false: the surface is gone; stop posting
```

minted by the instance for one handle and handed to whatever needs it. A caller holding one
cannot reach anything else, cannot name a different surface, and never sees the instance or
the program.

Naming follows the standard library's split: `CancelFunc`, `WalkFunc` and `SplitFunc` are
capabilities handed to a caller and take their name from the **verb**; `HandlerFunc`,
`ReaderFunc` and `RoundTripperFunc` are adapters and take theirs from an interface that
already exists. froth has no `Poster` interface, so this is the first case, and `Post` is
already the vocabulary's verb.

**The bool is what makes handle addressing pay off.** A long-lived service can hold a
surface's `PostFunc` forever with no registration protocol and no teardown coupling: when
the surface dies, posting reports false and the service prunes itself. Without it a service
accumulates dead handles it can never learn about.

This one mechanism serves three things the spec treats separately: a command's result
returning to its issuer, a producer emitting to the surface it runs for, and an outside
system notifying whoever asked.

## A service is not a surface

A model catalog, an RPC client, a cache — things that participate in the message flow but
draw nothing — are **ordinary objects the application owns**, not surfaces. Surfaces reach
them through the Environment, which already exists for exactly this: *"ambient values a
surface reads from its ancestors — theme, capabilities."* A service handle is a capability.
The service posts back through a `PostFunc`.

The alternative — a surface that renders nothing — puts a node in the render, layout and
focus walks to hold a pointer, and is the zero-area surface a diagnostic should be
complaining about.

**This is what dissolves the copies.** A surface that holds a *reference* to a catalog has
nothing to keep in sync. A surface that holds a *copy* needs a mirror at every write site,
and the mirror is what rots — one missed line and a picker shows a stale default forever.
Environment resolves at read time and is *"never a stale snapshot"*, so the whole class of
bug goes away rather than being managed.

## Lifetime: a context per surface, created lazily

Each surface gets a `context.Context` cancelled when it is destroyed, so in-flight work
stops without the surface writing teardown. Two implementation constraints, measured rather
than assumed (Go 1.27, `context.WithCancel`: 93 ns, 96 B, 2 allocs):

- **Flat, not mirrored.** Chaining each surface's context off its parent's costs ~488 B per
  surface against ~120 B flat, because every node with a child allocates a children map.
  The only thing chaining buys is cascade on subtree destroy, and unmount already walks the
  subtree leaf-up.
- **Lazily.** Controls are surfaces, so a froth tree is deep and mostly structural, and
  most surfaces never start async work. Create on first ask, cache on the node.

The obligation this creates is that **cancel is always called**, which is precisely the
teardown a surface no longer has to write.

**Cancellation is not an Environment value.** Environment is application-owned and
subtree-overridable; a surface handed a context other than its own lifetime's would break
the guarantee entirely. It stays on the phase context. This corrects
[ADR 0004](./0004-no-synchronous-send.md) and `CONTEXT.md`, which both list cancellation
among ambient values.

## Currency is the caller's problem

froth does not track whether a result is still wanted. A surface that issues three requests
and cares only about the last gets no help from lifetime cancellation — it is alive, and
*"cancellation is not the correctness guard."*

froth deliberately stops short, because the alternative is worse. Any framework answer
needs request groups, invalidation points, and a policy for partially-superseded work — and
it would be guessing at semantics only the caller knows.

**The pattern instead: the service holds currency, not the surface.** A catalog that knows
which fetch is current simply does not post the superseded one. That collapses what would
otherwise be a generation guard at every call site into one place per service, and it is
where the knowledge already is. A real codebase carrying this by hand accumulated roughly
twenty generation and token guards across its UI, with a four-clause guard at its worst,
and its own design notes had already reached the same rule.

## Consequences

**Three user stories need correcting rather than implementing as written.** Story 9's
addressed command results and the command-envelope decision are unbuilt — today every
async result lands at the root. Story 21 asks for a lifetime-scoped context and the code's
own doc comment decides the opposite; this ADR settles it in the story's favour. Story 8
claims handle addressing removes the need for per-surface request tokens: that is true for
a **dead** surface and false for a live one that has moved on. Its real payoff is the
no-deregistration property above, which is larger than the claim it makes.

**The root stops being a distributor.** Every unaddressed message currently routes to the
root surface, which then has to hand it on — the router froth exists to remove, surviving in
one place. Addressed results and service objects remove the reason for it.

**Parked: a `Topic` — a named channel surfaces subscribe to and outside systems post to.**
It was in the vocabulary with no design, no code and no scope behind it. It is not needed:
the outside object *is* the topic, and it is application code. Revive it only if a case
appears where several surfaces must receive the same event and no object can reasonably own
the fan-out — bring that case, not the mechanism.
