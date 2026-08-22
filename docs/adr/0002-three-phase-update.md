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
