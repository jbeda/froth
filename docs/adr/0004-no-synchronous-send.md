# No synchronous send

Nothing calls into a surface synchronously except a read-only Query. Ambient values
— theme, capabilities, a service a subtree should use — are read from an Environment
resolved against the tree, rather than handed over at construction (which goes stale) or fetched by
message. A surface asking an ancestor to act returns an Intent, applied after its
handler has returned, so it can never be delivered re-entrantly. Collecting from
many surfaces at once, help bindings being the first case, is a read-only Query
walk.

## Considered options

A general synchronous `SendMessage`, for frame coherence: letting one surface query
another within a single frame instead of round-tripping the queue. Rejected on two
grounds. Its motivating case is already served — immediate-mode rendering builds
geometry and hit regions in the same pass, which is what frame coherence means here.
And it was the sole justification for three guards: send only from the pump
goroutine, a query-versus-command split enforced at registration, and a per-surface
reentrancy guard. Dropping it deletes the first two outright; the third survives,
scoped to Query alone.

Recorded so it is not re-proposed on general principle: if a case appears that
Environment, Intent and Query genuinely cannot serve, bring that case — not the
mechanism.

## Amendment: cancellation was never an ambient value

The rule above originally listed *cancellation* beside theme and capabilities. That was
wrong and is corrected in place. Environment is application-owned and subtree-overridable,
so a surface could be handed a context other than its own lifetime's — which defeats the
point of having one. A surface's context is froth-managed and reaches it through the phase
context. See [ADR 0015](./0015-results-come-back-to-whoever-asked.md).

The closing rule stands and has now been used once: `Topic` was proposed as a mechanism
without a case, and ADR 0015 parks it for exactly that reason.

## Amendment: Intent is gone, and the rule is unchanged

Settled while slicing M3. The rule above named three things a surface may use instead of a
synchronous send — Environment, Intent and Query. One of them no longer exists.

`Intent` was a parent-ward walk with a claim, applied after the raising handler returned.
The walk survives as ordinary message delivery: a message type that embeds `froth.Rising`
is offered to the surface it was posted to and then to each ancestor in turn. See
[ADR 0016](./0016-a-message-rises-until-something-takes-it.md).

**Nothing about this ADR's rule weakens.** A rising message is still posted rather than
delivered re-entrantly, still travels through the queue, and still cannot reach a surface
while that surface is inside its own handler. The sentence *"A surface asking an ancestor to
act returns an Intent, applied after its handler has returned"* should now read: a surface
asking an ancestor to act posts a rising message, delivered after its handler has returned.

Read the closing rule as **Environment, a rising message, and Query**. It has now been used
twice — `Topic` was parked by [ADR 0015](./0015-results-come-back-to-whoever-asked.md) for
being a mechanism without a case, and a root-first delivery pass is parked by
[ADR 0016](./0016-a-message-rises-until-something-takes-it.md) for the same reason.
