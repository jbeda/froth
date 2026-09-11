# Testing froth

## One seam

Tests construct a `froth.Froth` with a root surface, post synthetic Bubble Tea messages
through its update entry point, and assert on what comes back. No terminal, no TTY, no
sleeping. That is exactly how an application uses froth, so tests exercise the real
public entry point and nothing else.

Subtree compositing is a narrower view through the same seam rather than a second one:
it returns the composited string for a subtree plus the resolved rect for each surface in
it, so a test can drive one surface and assert on just its region and position. It is
`froth.Composite(Handle) Frame`, and `View` is `Composite` of the root — tests and
production share one render path rather than two that can drift. "Is the
button where I think it is" is the assertion that catches layout regressions, and it
needs the rects as much as the string.

## What makes a good test here

Assert on rendered output and resolved rects. Never on registry internals, handle
values, phase bookkeeping, or the shape of the message queue.

A test that breaks when the registry's internal representation changes is testing the
wrong thing. A test that breaks when a modal stops receiving focus is testing the right
one.

The exception is a handle froth itself publishes as an answer: `Query.Focus` returns one,
so comparing it against a handle the test created asserts on documented behaviour rather
than on internals. What stays out of bounds is a handle's *contents* and any handle a test
reached rather than was given.

Almost everything is observable through the seam: shortcut precedence, focus movement and
restoration, rising delivery and untaken-message reporting, modality, capture and capture
loss, command tagging and dead-handle drops, runtime phase violations, chrome, and Z
ordering.

## Self-contained algorithms, tested directly

Some of what froth does is a **self-contained algorithm**: a pure function from values to
values, deciding something froth has written down as a rule, reaching for no tree, no
focus and no phase. Those are tested as the functions they are. This is the one sanctioned
departure from the single seam, and it is narrow on purpose.

Three things have to hold, or it is reaching inside:

- **Pure.** Values in, values out. No froth instance, no handles resolved against a
  registry, no state that a test has to arrange by driving froth first.
- **A written-down rule.** The behaviour is specified somewhere a reader can find — an
  ADR, the milestone log, a doc comment carrying the whole contract. A test asserting on
  something whose only specification is the code is a test of the code's current shape.
- **The wiring is still tested through the seam.** A direct test says the algorithm is
  right. It never says froth *calls* it, calls it with the right inputs, or does anything
  with the answer.

Three qualify so far:

- **`Layout`** — an interface users implement, so its space-and-measures to rects mapping
  is published API rather than an internal detail. Testing it directly is testing a
  contract.
- **Resolving a direction to a surface** — `pick` in `direction.go` takes candidate rects
  and returns the winner, and the tests it has to satisfy are the awkward geometric cases:
  exact ties, partial overlap, a candidate that grew around the source. Driving those
  through a surface tree means building a layout to produce each one and reading the
  answer back out of a composited string, which is a lot of fixture between the assertion
  and the thing asserted. The rule is in ADR 0013 and the milestone log; which surfaces
  are candidates at all, and what a key press does, stay at the seam.
- **Clipping what a surface drew** — `clip` in `clip.go` takes a string and the size the
  surface was offered and returns what reaches the compositor plus the drawn rect it
  makes. The rule is [ADR 0018](./adr/0018-a-surface-occupies-the-rectangle-its-output-makes.md):
  the string is truncated to the offer, padded out to a solid rectangle, and measured in
  columns rather than bytes. The cases that matter are ragged lines and wide graphemes
  straddling the edge, none of which a composited string can be read back for — padding is
  invisible in the frame, because the compositor clears a layer's box either way. That
  froth clips every surface's output, and what the drawn rect is then used for, stay at
  the seam.

What still does not qualify, and the test to apply: if the assertion would break when an
internal representation changed but froth's behaviour did not, it belongs at the seam
however pure the function looks. The registry, the message queue, phase bookkeeping and
the focus chain are all reachable as functions and none of them is a rule — they are how
froth happens to be built.

## Posting from outside, and the producer subpackage

**The post capability is tested with a fake program.** `Attach` takes an interface with
`Send` only, so a test hands froth a fake that records what was sent and then calls
`Update` itself with the wake. That covers ordering across posters, the dead-handle bool,
a post before attach, and the drain. The wake is the one thing that crosses a goroutine —
froth sends it from one of its own — so those tests run under `testing/synctest`, where
`synctest.Wait` makes "how many wakes have been sent" a deterministic question rather than
a sleep.

**Command envelopes are tested through the real wrapper.** Run the wrapped command in the
test, hand its result to `Update`, and assert where it landed. Quit, interrupt, batch and
sequence each get a case, since a wrong pass-through set fails silently in production.

**Producers live in the producer subpackage and are tested there, as one implementation.**
A step function is an ordinary function, tested by calling it with a pre-filled channel.
The loop that drives it is tested under `testing/synctest`, which makes cancellation
mid-stream, end-of-stream, a partial batch at error, and goroutine leaks deterministic
without an injected runner to keep faithful. It is the only place real goroutines appear
in the suite. `testing/synctest` is generally available from Go 1.25; froth requires 1.27.

## Panic quarantine needs a fixture

A deliberately panicking surface, asserting that the diagnostic fires, that the surface
stops receiving messages, and that the rest of the tree keeps working. This behaviour
regresses silently, so it needs a test rather than a paragraph.

## Compile-time is part of the strategy

Each phase has its own context type, so most illegal operations are compile errors rather
than test cases. Do not write tests asserting that structural change during render fails
— it does not compile. The runtime check covers only what types cannot: a surface
retaining a context across phases.

## Two conventions the seam needs

**Post a size before asserting on anything.** A froth instance draws nothing until a
`tea.WindowSizeMsg` arrives, so a test constructs the instance and posts one — which is
what Bubble Tea does on startup. One `newSized(t, root, w, h)` helper covers it.

**A fixture surface records its own handle.** `Create` returns a handle only to the
parent that called it, and froth has no lookup by name, so a fixture a test wants to
assert about stashes `ac.Self().Handle()` in its `Mount` and the test reads it from there. That is
[ADR 0001](./adr/0001-surfaces-addressed-by-handle.md) and the no-global-namespace
non-goal working as intended, not friction to remove.

## No prior art

M1 established these conventions rather than following them.
