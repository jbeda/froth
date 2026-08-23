# Dangerous capability is named and imported, not gated

froth withholds capabilities on purpose: there is no child enumeration, a surface owns
only itself and its immediate children, and each phase has its own context type so most
illegal operations do not compile. Real cases will push on those — a UI debugger that
shows the surface tree needs to enumerate it, and needs handles, because a debugger that
cannot map a node back to a handle cannot focus or highlight what you clicked.

Where such a case is genuine, froth provides the capability **in a separate package** —
`froth/inspect` — rather than behind a flag on a context or a method on the ordinary one.

The rule underneath: **the threat model is accident, not malice.** These protections exist
to stop a developer or an agent doing the wrong thing by reflex. They are not security
boundaries; nobody is attacking a TUI library from inside its own process. So the goal is
to make the wrong thing hard and *visible*, not impossible — and hardening them as though
someone were attacking costs expressiveness to buy nothing.

Two corollaries decide most cases:

- **Prefer compile-time shape to a runtime flag.** A capability nobody should hold
  casually is absent from the type they normally have, rather than present and refused.
- **The hatch is for tools, not applications.** Anything an application legitimately needs
  gets a real API. An application reaching for `inspect` is a missing feature in core — a
  design bug to fix, not a hatch to widen.

## Considered options

**A boolean on a context — `allowDangerous`.** Runtime-checked, so it needs a diagnostic
path for the refusal; invisible at the call site, so review cannot see it; and set once at
construction, then forgotten. Strictly weaker than a type for every property that matters.

**A method on the ordinary context — `uc.Unsafe()`.** Better: compile-gated, and
`\.Unsafe\(\)` greps. But it ships in every consumer's core import, so it offers nothing at
the dependency level, and the deterrent is one identifier deep in a function body.

**A second, weaker identity for reads** — enumeration returns descriptors rather than
handles. This is the only option that would make enumeration genuinely safe, and it costs
two identities for one thing, against [ADR 0001](./0001-surfaces-addressed-by-handle.md).
It also does not serve the motivating case, which needs to act on what it found.

**Widening the core API and trusting callers.** Loses what the no-enumeration rule was
buying — which was never safety. It was that nobody reaches for it by accident.

A separate package gets the compile gate of the second option plus a signal the others
cannot give: an import is visible in review, in `go.mod`, in an SBOM and in a dependency
scan. That is the argument
[the milestone log records for keeping bubbles out of `froth`](../design/milestone-log.md)
— the cost of a dependency is what it imposes on consumers — turned around and used
deliberately.

Prior art agrees about where the deterrent lives: Go's `unsafe`, Rust's `unsafe` blocks,
React's `dangerouslySetInnerHTML`. In each, **the name is the lint**.

## Consequences

**The hatch does not make it safe. It makes it visible.** A handle from `inspect` is an
ordinary handle, so a root that enumerated a modal can destroy it — the modal is its
immediate child, which is exactly what the ownership rule permits. Nothing here contains
that. The import is the protection, and pretending otherwise would be worse than having no
hatch at all.

**Linting is configuration, not an analyzer.** `golangci-lint`'s `depguard` forbids an
import outside named paths, so "no `froth/inspect` except under `cmd/` or a debug build
tag" is a few lines in `.golangci.yml`. The rule lands with the package rather than ahead
of it. A build tag can compile it out of a release binary entirely.

**`froth/inspect` starts read-only** — tree shape, handle metadata, resolved rects.
Focusing what a debugger found needs `Focus`, which is ordinary public API, so read-only
already serves the motivating case. Anything beyond reading is argued on its own.

**Hatches accumulate, and that is the standing risk.** Once the package exists, the
pressure is to put the next awkward requirement behind it instead of designing an API, and
in two years `inspect` *is* the API. The tools-not-applications rule is the whole
mitigation, and it only works if it is enforced when the pull request arrives.

**What this deliberately does not decide:** whether enumeration hides the surfaces froth
parented on a caller's behalf — modals and floats — or shows the tree honestly. That
belongs to the milestone that builds it, once there is a real consumer to argue from. This
ADR sets the shape, not the contents.
