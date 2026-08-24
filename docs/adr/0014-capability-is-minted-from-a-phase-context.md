# Capability is minted from a phase context, not carried by a handle

A `Handle` names a surface and confers nothing. To act on one, a surface asks the phase
context it is already holding for an accessor scoped to that handle — `uc.At(h)`,
`ac.Child(h)`, `ac.Self()` — and the accessor's type decides what may be done. The
parentage the ownership rule turns on is checked **once, when the accessor is minted**.
After that the type carries the proof and every method on it is unconditionally safe.

The rule this puts teeth on is the one froth has stated in prose since the beginning: a
surface owns itself and its immediate children.

## Why a rule was needed

`Create` and `SetLayout` make ownership true by making the alternative **inexpressible** —
`Create` takes no parent, so creating a grandchild or a sibling cannot be written, and
`SetLayout` names no target, so a surface cannot rearrange somebody else's children.
`Destroy` is the first structural operation that cannot use that trick, because it has to
name which child.

At the same time the supply of handles widened. `Query.Focus()` hands out a handle to a
surface the caller did not create, and when focus names nothing it returns the root. So
"you can only reach what you made" stopped being true just as the operation that most
needs it arrived.

## What the split actually buys

Two different guarantees, and conflating them oversells the design:

- **Which operations exist is static.** `At(h)` has no `Destroy` method, so destroying a
  stranger does not compile.
- **Which target is legal is dynamic.** Whether `h` is your child is a map lookup. No type
  system reaches that.

So the runtime check does not disappear. It moves to one chokepoint that can be tested,
linted and reported from, instead of being repeated at every call site — and every method
downstream of it is then free of validation.

## Why a callback-scoped capability cannot leak here

An accessor holds the phase token, which already panics when a context outlives its
callback. That bounds its life to one callback, and
[ADR 0004](./0004-no-synchronous-send.md) bounds where it can go inside that callback:
there is no synchronous send, so a surface cannot call into another surface at all. A
capability that cannot outlive the callback and cannot reach anyone during it has nowhere
to escape to.

This is what makes the accessor stronger than a value handed back from `Create`. That
value would be storable, passable to a sibling, and usable three phases later.

## The three tiers

They are the ownership rule, one tier per clause, plus everyone else:

| Tier | Minted by | Checked | Carries |
|---|---|---|---|
| self | `Self()` | nothing — names no target, so it cannot fail | create, set layout, declare bindings, raise, reconcile, destroy |
| child | `Child(h)` | `h` is an immediate child | destroy, and the routing and reads below |
| open | `At(h)` | `h` is live | post, focus, and reads |

Self is a tier rather than a special case of child because their operation sets genuinely
differ: `SetLayout` arranges *your own* children, so putting it on an accessor mintable
for a child would be a surface rearranging its grandchildren.

Reads are available on every tier for any live handle. Restricting them would buy nothing
— a read changes nothing — and would block the surface-describing cases that the help
query already established as core API.

## Failure has two postures

Minting a child accessor for a handle that is not your child is a **programming error**
and panics, like a layout returning the wrong number of rects. Minting one for a handle
whose surface is already destroyed is a **data condition** and is a diagnostic and a drop,
like delivery to a dead handle. The same call, two answers, because the two mistakes are
not the same kind of mistake.

## Phases

Accessors are minted from Render, Update and Apply, and one rule decides which operations
each offers: **Update offers the operations that return nothing.** Focus, destroy, raise,
reconcile and binding declaration all record cleanly and commit at Apply. `Create` cannot,
because it must hand back a handle that does not exist yet — pre-assigning registry
identity to make it possible was rejected as paying real complexity for symmetry.

## Considered options

**A bearer handle with no check.** A handle is authority, full stop. Costs nothing and is
consistent with the accident-not-malice framing of
[ADR 0012](./0012-dangerous-capability-is-named-and-imported.md). Rejected because
`Query.Focus()` makes "destroy the focused surface" a two-line accident, and because it
leaves the ownership rule permanently aspirational — stated in prose, enforced nowhere.

**A distinct handle type returned by `Create`.** `Destroy` requires it; a one-way downgrade
yields a plain `Handle` for posting. Puts the rule in the type system with no runtime check
at all. Rejected because the value is a **bearer token with a long life**: storable,
passable to a sibling, usable in a later phase. It also adds a second identity for one
thing, against [ADR 0001](./0001-surfaces-addressed-by-handle.md), and every call site
holding one pays a conversion to reach the ordinary API.

**A runtime parent check on a flat `Destroy(h)`.** No new type, `==` untouched, and it
mirrors `Focus`, which already names a target and validates it. Rejected as *insufficient*
rather than wrong — it is the shape kept, but on its own the illegal call still compiles,
and there is nothing for review or a linter to see.

**`Destroy` with no target — a surface destroys only itself.** Removal becomes
self-relative like `Create` and `SetLayout`, so expressibility does the work again and no
new type appears. Rejected because destruction becomes asynchronous, an uncooperative
child cannot be removed, and `Reconcile`'s contract weakens from imperative to cooperative.

**One accessor type per tier, with the phase checked at runtime.** Fewest types. Rejected
because it discards the static phase guarantee the three context types exist to provide.

## Consequences

**The three phase contexts hollow out.** They become mint sites plus the tree-global read.
That is the intended direction — flat methods accumulating on a context is what made the
addressing question expensive to defer — but it reworks public API that already shipped,
along with the example that uses it.

**Five concrete types, and the count grows with phases, not with operations.** Render
collapses to one, because with no render-time mutation a child and a stranger permit
identical reads; Update and Apply share the child and open types, because their method sets
agree and whether `Destroy` records or acts is the phase token's business.

**One interface ships: `HandleReader`, sealed.** An interface earns its place when
somebody writes a function parameter of that type, and only the read set has a plausible
one. The others are deferred because adding an exported interface later is never a breaking
change — any concrete type whose method set already covers it satisfies it retroactively.

Sealing follows the standard library's own reasoning, stated at `testing.TB`: *"A private
method to prevent users implementing the interface and so future additions to it will not
violate Go 1 compatibility."* `go/ast`'s `Expr`, `Stmt` and `Decl` and `reflect.Type` seal
the same way. The rule to apply: **seal an interface that is a closed set froth defines;
leave open an interface that is an extension point.** `Surface`, `Layout` and `Mounter` are
the second kind and stay open. The marker method goes on all five concrete types from the
start, so any interface added later inherits the seal — unsealing later is harmless,
sealing later is breaking.

**Where a future operation goes is decided by the tiers, not case by case.** Ask who the
operation is *about*. A property of the surface itself, or of how its own children are
arranged, is self. Something done *to* a child is the child tier. Reaching a surface
without restructuring it is open. A title is the worked example: reading it belongs on the
reader, and setting it is self or child depending on whether a surface titles itself or its
parent titles it — a question this ADR does not answer, but does locate.

## Amendment: the self tier carries the reach set, and Apply carries Post

Settled while building it. The five types shipped as `ReadCtx` (Render, all three tiers),
`SelfUpdateCtx`, `SelfApplyCtx`, `ChildCtx` and `AnyCtx` — "accessor" throughout this ADR
is a `Ctx` in the code, per `CONTEXT.md`'s entry for the word. Two corrections to the
table above, both forced by call sites.

**Self carries post and focus as well as what the table lists.** A surface posting to
itself to continue in a later delivery is an ordinary pattern, and the table as written
sends it through `At(Self().Handle())` to reach a permission the self tier obviously has.
So the tiers nest by *capability* — self ⊇ child ⊇ open — while still not nesting by
*target*, which is why self remains a tier rather than a special case of child:
`SetLayout` arranges your own children and has nowhere else to sit.

**Apply mints contexts that can post.** The five-type count in the consequences depends on
the child and open types being shared across Deliver and Apply, and that requires their
method sets to agree. It is not a concession to the type count: #17's session-B comment
had already given `ApplyCtx.Post` a justification — a surface being torn down telling
something outside froth, which `Unmount` runs in Apply to do.

**"A diagnostic and a drop, like delivery to a dead handle" was wrong about the simile,
not the rule.** Delivery to a dead handle did not report anything; it dropped silently.
The rule shipped as written, so posting to a stored handle whose surface has gone now
reports `DiagDeadHandle` where it used to be silent. `DiagInvalidFocus` correspondingly
has no reachable path until a surface can leave the tree between a recorded focus move and
the Apply that commits it.

## Amendment: what `Destroy` shipped as, and the panic that is not a surface's

Settled while building the operation the tiers exist for.

**`Destroy` is on all three tiers and takes no target.** `Self().Destroy()`,
`Child(h).Destroy()`, and nothing on `At(h)` — so destroying a stranger does not compile,
which is what the table above promised. The self tier carries it because a surface
removing itself is ordinary; there is no `Close`, and the sketch's `uc.Close()` is
superseded. One verb with one spelling was preferred to a second name for the same
operation, per `CLAUDE.md` on building the explicit mechanism before sugaring it.

**The root refuses by panicking, not by construction.** The ADR said root is undestroyable
because nobody can mint an accessor naming it as a child, and that is true of the child
tier. It is not true of the self tier: the root's own `Self()` is mintable like anyone
else's. So the rule needed a mechanism after all, and it is a panic — a surface asking to
destroy the tree it heads is a programming error, not a condition.

**Quarantine is the one route out that the root has.** A surface asking to destroy the root
panics, as above. Quarantine is not a surface asking: a root that failed has to leave, or
froth goes on calling it. The tree empties and the instance is inert, which is where
`Shutdown` also ends up.

**A panic quarantine has to tell a surface's failure from froth's own.** Panic quarantine
recovers a panic out of `Update`, `Render`, `Mount`, `Unmount` or `Bindings`, reports it and
takes the surface out of the tree. Every contract this ADR puts teeth on is enforced by a
panic raised from inside one of those same callbacks, so a blanket `recover` would demote
all of them to a diagnostic and a removal — the illegal call would be reported far from its
cause and the program would carry on around it.

froth therefore raises its own panics as an unexported error type and quarantine re-panics
when it sees one. The rule: **a panic froth raised says the program is wrong, and a panic a
surface raised says the surface is wrong.** The first must reach the developer; the second
must not cost the terminal. The cost is that every panic site inside froth goes through one
constructor, which is also the thing that makes the set greppable.

**Considered and rejected: quarantining everything uniformly.** One code path, nothing to
keep in sync, and every contract violation becomes survivable. Rejected because the
guarantees this ADR spent a design on would then be advisory: an illegal call would produce
a report and a missing surface rather than a stack trace at the call site. Reporting the two
under different diagnostic kinds was rejected for the same reason — it distinguishes them in
the hook while still not stopping the program.
