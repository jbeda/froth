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
