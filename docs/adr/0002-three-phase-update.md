# An update runs Deliver, Apply and Render — and nothing else

Each Bubble Tea update runs three phases. **Deliver** hands one message to one
surface, which may change only itself and may return work. **Apply** commits
structural change: creation, destruction, reparenting, and intents raised to
ancestors. **Render** draws. Deliver and Apply loop until the message queue drains
and only then does Render run, so a modal can close and its opener handle the
result within the same frame, with no skew. The phase check is always on: every
froth call asserts its phase on entry.

## Considered options

A separate **Layout** phase was proposed and rejected. It exists in pixel toolkits
because measuring is expensive and text reflow is nonlinear; in a cell grid, layout
and render are the same top-down walk. The only thing it bought was hit-testing
against fresh geometry, and handle validation already covers the failure that would
have prevented.

**Direct mutation during Deliver** was rejected because froth is mid-walk over the
tree the handler would be splicing — the classic iterator-invalidation bug, and the
reason other implementations of this pattern need a separate sweep to remove closed
windows rather than removing them in place.

## Consequences

Structural calls made during Deliver record and commit at Apply; the same calls made
during Apply execute immediately. This is the subtlest thing in the design, and the
phase is always known, so the error can say precisely why a call queued rather than
executed. The drain loop needs an iteration cap so a message cycle fails loudly
instead of hanging.

## Amendment: the phase check is mostly static

Writing client code against this design showed the context wants to be *several*
types rather than one — `RenderCtx`, `UpdateCtx`, `ApplyCtx`, `MeasureCtx` — because
each phase permits a different set of operations. A `RenderCtx` has no `Create`
method, so structural change during render becomes a compile error rather than a
runtime one.

The runtime check stays for what types cannot catch, chiefly a surface stashing a
context and using it in a later phase, but it stops being the primary mechanism.

## Ordering within Apply

Apply commits in a fixed order: **intents first, then structural changes.** A handler
may both raise an intent and close itself in the same delivery, and if the destroy
committed first, the intent's sender would name a destroyed surface — so a modal
wired with `RepliesTo(sender)` would reply into a corpse.

An intent is delivered by walking parent-ward from the raiser, to each ancestor
implementing the intent-sink capability, until one claims it. Nearest ancestor wins.
An intent nothing claims is reported through the diagnostic hook rather than silently
dropped, because an unclaimed intent is nearly always a missing case.

An ancestor may raise a further intent while handling one. That appends to the intent
queue and is processed later in the same drain rather than recursing, so Apply never
nests — the same reentrancy exclusion that applies to message delivery, under the same
iteration cap.

## Amendment: the loop leads with Apply

Built in M1. The alternation is the one described above, entered one step earlier: an
update runs Apply, then Deliver, and repeats. The tree has to exist before anything can
be delivered into it — the root's own mount is the first Apply any instance runs — and
the Apply that a delivery asks for is then the one at the top of the next turn.

The cap is one budget that deliveries and mounts spend alike, because a cascade that will
not settle can be made of either and the two interleave. Tripping it reports through the
diagnostic hook and abandons what was being drained; the value it is set to, and why, is
in the milestone log.
