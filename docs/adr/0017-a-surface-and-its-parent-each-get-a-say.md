# A surface and its parent each get a say about its Environment

Every surface has **two Environment slots**: one it writes on itself, and one its parent
writes on it. Resolution at a surface checks its own slot, then the slot its parent gave
it, then walks to the parent and repeats. Both slots cover the surface and its whole
subtree, because resolution starts at the reader.

The Environment holds values the **application** owns. froth writes none of them.

## Why two slots

A parent needs some way to say "this child's subtree sees a different theme". Every
declarative system expresses that by **wrapping** the child in a differently-valued
provider — React's `Provider`, Flutter's `InheritedWidget`, SwiftUI's `.environment`
modifier, which looks like a per-child write at the use site and is actually a
`ModifiedContent` node. In all of them the parent's value sits at a position *above* the
child and the child's own value sits *at* it, so when they disagree the child wins **by
position**, decidable by reading the tree.

[ADR 0009](./0009-one-tree-no-implicit-surfaces.md) denies froth that move: there are no
wrapper nodes, and froth may not insert one. Collapsing both writes onto a single binding
would leave **recency** as the only tiebreak — and recency is what no surveyed system uses.
CSS resolves by encapsulation context then specificity, Qt by which value was explicitly
set, Android by which side defined it. All are decidable by inspection; "whoever ran last"
is decidable only by replaying the program.

Two slots put the two positions back at one node. Nothing is inserted, nothing is hidden,
and the ordering is a property of the data structure rather than of execution history.

## What this dissolves

**There is no dumb-surface concept to express.** The question that produced this design was
whether a parent may meddle with a child's ambient values — plainly fine for a layout-only
container it created, and plainly not for a surface that owns the value itself. froth cannot
tell those apart, and does not need to: **the parent may always write the given slot, and a
child that cares writes its own and wins.** A container that writes nothing lets its
parent's value stand. A surface that owns a value is never silently overridden.

That is Qt's behaviour — a child wins only if it explicitly set the value — arrived at
deliberately rather than through an undocumented resolve mask, and it closes the one failure
the prior art actually documents. Android shipped `MaterialThemeOverlay` as a whole utility
class so components could defensively re-wrap against a client's `android:theme` silently
defeating their own; under two slots there is nothing to defend against.

## Reads are broad, writes are narrow

Reading changes nothing, so it is available everywhere:

```go
th := rc.Env(ThemeKey)              // flat on every phase context: this surface's view
th := rc.At(h).Env(ThemeKey)        // another handle's view, on the reader accessor
```

Writing is policed by [ADR 0014](./0014-capability-is-minted-from-a-phase-context.md)'s
tiers, because a write is about a subject:

```go
uc.Self().SetEnv(ThemeKey, t)       // this surface's own slot
uc.Child(h).SetEnv(ThemeKey, t)     // the slot this surface gave that child
```

The child tier's parentage check is what enforces "only the immediate parent", so there is
no `SetEnvOn` and no documented restriction to remember — an illegal write panics at the
mint like any other.

**Setting a value on an ancestor is not expressible**, which is unanimous in the prior art:
nothing surveyed lets a node write upward, and SwiftUI needed a separate feature entirely
(`PreferenceKey`) for the other direction.

`Env` stays flat rather than moving to the self accessor, against ADR 0014's usual rule.
The reason is the one that ADR already gives for keeping reads unrestricted — a read changes
nothing — and the cost of the alternative is a mint per read in every `View` method.

## Writes take effect immediately

`SetEnv` means the same thing on `UpdateCtx` and `ApplyCtx`. It is froth's only call whose
recorded and immediate forms collapse, and the reason is one line: **an Environment binding
is not structure.** The Deliver-records-Apply-commits rule exists because froth is mid-walk
over the tree a handler would be splicing; changing a value alters what descendants
*resolve*, not what the tree *is*.

The frame guarantee people actually want survives untouched, and is better stated than it
was: **froth never renders mid-drain, so a frame never shows a torn view of the
Environment.** That comes from the phase order, not from deferring the write. Deferral would
have bought only that every delivery within one drain resolves the same binding, which
nobody has a case for — and it would have bought it while protecting a promise that is
already best-effort, since a surface can put a pointer in the Environment and mutate what it
points at, and Go has no way to stop it.

**What is given up:** two related keys written in sequence are no longer atomic, so a
delivery landing between them sees one updated and the other not. The mitigation is
ordinary — put related values in one struct under one key — and it is the discipline the
Environment wants anyway, since a subtree overriding half a pair is incoherent regardless.

## The Environment is the application's

froth seeds nothing. Terminal capabilities and size reach the Environment because the
application puts them there — it owns the Bubble Tea program and sees the messages.

This keeps [ADR 0004](./0004-no-synchronous-send.md)'s invariant load-bearing rather than
nearly-true. That ADR decided a surface's `context.Context` is *not* an Environment value
precisely because *"Environment is application-owned and subtree-overridable"* — and a
froth-written value would make that qualified. It also keeps every binding traceable to a
line somebody wrote, which a froth-seeded value would not be.

**Not foreclosed.** The standard library does exactly the thing declined here: `net/http`
places `ServerContextKey` and `LocalAddrContextKey` in every request context. So the
precedent for a small fixed framework-owned set exists and is respectable. Revive it if a
value appears that only froth can know, that many surfaces across a subtree read, and that
changes during their lifetime — bring that value, not the tier.

## Mechanics

Keys carry their own type, so neither end asserts:

```go
var ThemeKey = froth.NewKey[theme.Theme]("theme")
var DensityKey = froth.NewKeyDefault("density", layout.Comfortable)
```

Go 1.27 allows **type parameters on methods of concrete types**, verified by compiling one,
which is what makes `SetEnv[T]` and `Env[T]` methods rather than free functions. Interface
methods still cannot have them — `interface method must have no type parameters` — so a
helper that takes a reader needs a free `froth.Env[T](r HandleReader, k *Key[T]) T` layered
over an unexported untyped lookup. ADR 0014's seal is what makes that safe: only froth
implements the interface, so an unexported method on it is not a hole.

Two slots means two maps per surface rather than one, created lazily. Most surfaces write
neither.

## Amendment: keys are opaque identities, and absence is an error

A key is identified by the value `NewKey` or `NewKeyDefault` returned, not by its name. Two
calls with the same name create distinct keys. The name is for diagnostics only: it makes a
missing binding actionable without introducing a global namespace or coupling packages that
happen to use the same word. Passing a nil key to a read or write is also a contract
panic, with an explicit diagnostic rather than a nil-pointer failure.

Reading an unbound key without a default panics. Returning the value type's zero value would
make an absent binding indistinguishable from an explicit zero, and leave a missing
application-owned value silently in use. A key made with `NewKeyDefault` instead resolves to
its declared default when no binding exists. A read through a Ctx naming a dead handle also
returns that default when one exists; otherwise it panics as a missing binding. The Ctx still
reports the dead handle when it is minted, as every reader does.

## Considered options

**One slot, last write wins.** Simplest, and unprecedented — no surveyed system resolves
ambient writes by recency. Its failure is the one the prior art documents: the loser gets no
signal, and which value stands depends on execution order rather than on anything readable.

**A create-time option only**, `Create(s, WithEnv(k, v))`, with no live per-child write. This
is where the genuine per-child writes in the prior art live — Angular's
`createComponent({injector})`, Android's `android:theme` read during inflation — and it was
rejected because changing a child's ambient value would then mean **destroying and
recreating the child**. Under two slots it needs no separate mechanism anyway: a parent
writing the given slot during the Apply that creates the child *is* the create-time form.

**A constructor parameter; the child sets its own value.** The most explicit option, and
dead as the general mechanism: it only works for surfaces you wrote. A third-party surface
whose constructor takes no theme could never be overridden.

**A parent-created grouping surface as the wrapper.** froth's imperative equivalent of what
the declarative systems do, and legitimate — [ADR 0009](./0009-one-tree-no-implicit-surfaces.md)
permits explicitly created grouping surfaces. Rejected as the *primary* mechanism because it
charges a node on the render, layout and focus walks, and one more hop on every rising
message ([ADR 0016](./0016-a-message-rises-until-something-takes-it.md)), for what two slots
give free. It remains available to an application that wants one.

## Parked: removing a binding

M6 has no `UnsetEnv`. A surface can restore a value it replaced by writing the value it held
before, and no case needs an override to expose an inherited value or a key's default again.

Revive this when a real surface needs a temporary binding to disappear rather than be replaced
with its prior value — bring that lifetime and the value it must reveal, not a removal method.

## Parked: a value that skips the parent's slot

If layout-only containers turn out to stack several deep, a surface may want to set a value
that its immediate parent's slot cannot shadow, or to reach past a chain of containers that
write nothing. Neither is expressible.

Not built, because the shape of the need is unknown and guessing at it would add a third
slot nothing has asked for. Revive it on a real tree with containers several layers deep and
a value that genuinely cannot be placed at either end of them — bring the tree, not the
mechanism.
