# froth

Surface management for [Bubble Tea](https://github.com/charmbracelet/bubbletea).

**Status: under construction.** Milestone 1 has landed. froth builds a tree of
surfaces addressed by handle, places them with a layout, composites them through Lip
Gloss with its own Z ordinals, drains Deliver and Apply before drawing a frame, and
delivers raw keys to the surface that holds focus. Everything after that — bindings,
reconcile and destroy, modals and mouse, async, environment and chrome, tiling and
scrolling, bubbles adapters — is still design.

[`examples/panes`](./examples/panes) is the fastest way to understand the API: it is
real, current code, and it grows with each milestone. [`examples/keys`](./examples/keys)
is a tool rather than a demonstration: it prints what a key press actually arrives as, so
a binding is written from what the terminal sends rather than from what the keyboard
says. The design is written down:
[`CONTEXT.md`](./CONTEXT.md) defines the vocabulary,
[`docs/adr/`](./docs/adr/) records the decisions that were hard to reverse,
[`docs/research/`](./docs/research/) holds the primary-source findings those
decisions rest on, and
[`docs/design/milestone-log.md`](./docs/design/milestone-log.md) records what building
each milestone taught. This README is the overview; where it disagrees with those,
they win.

## What it's for

Bubble Tea gives you one model, one `Update`, one `View`. Composing anything
non-trivial means hand-rolling a router in your root model — the community's
standing answer to nested components. froth makes surfaces first-class:

- **Surfaces** that own their state and rendering, addressed independently rather
  than reached by walking a model tree.
- **Composition** without every parent knowing how to forward messages to every
  descendant.
- **Layout as a pluggable concern**, including tiling, rather than something
  hardcoded into each application's `View`.
- **Modals**, as one case of surface management rather than the headline.

Light inspiration from the Win32 messaging model — addressed messages, a message
pump, per-surface handlers. This is not a Win32 port and not a retro-aesthetic
project; the interaction model is the interesting part, not the API surface or the
look. Where Win32's design turned out to rest on constraints froth does not have, it
was dropped: see [ADR 0006](./docs/adr/0006-one-parent-relation.md).

## The shape of it

A tree of surfaces, each addressed by a **handle** that stays valid to hold and to
name as a destination. A surface's parent determines its coordinate origin,
lifetime, stacking context, modality scope, and routing chain. Z-order applies
within a parent and never across parents, so a modal that wants the whole screen
parents itself to the root.

Every update runs three phases — **Deliver** a message to one surface, **Apply**
structural change, **Render** — looping over the first two until the queue drains,
so a modal can close and its opener handle the result in the same frame. Nothing is
delivered re-entrantly, and there is no nested message pump. The one synchronous
call into a surface is a read-only query.

Controls are surfaces too, buttons included, which makes hit-testing and tab order
walks of a tree that already exists.

## Deliberate non-goals

- **Not an operating system.** This is for complex TUIs. Where a general mechanism
  and a simple one both work, froth takes the simple one.
- **No synchronous send.** Rejected with reasons in
  [ADR 0004](./docs/adr/0004-no-synchronous-send.md), which also records what would
  reopen it.
- **No global name lookup for surfaces.** Handles are the identity; a string
  namespace with no lifetime story becomes the way everything talks to everything.

## Not yet, but not foreclosed

- **Multi-pass layout** is unimplemented, but the render contract already permits it —
  `Measure` may be called repeatedly, so flexbox-style layouts can land without
  breaking surfaces written today.
- **Drag.** Mouse capture ships from the start; synthesized drag messages — dragging
  tabs and panels — do not. They are additive, because routing already accounts for
  capture.
- **A dialog helper library.** Stock confirm, prompt and pick dialogs, each with a
  helper that opens it and returns its answer (`confirm.Ask(...)`). Sugar over the
  modal-opening primitive rather than new mechanism.
- **Chorded shortcuts** (`ctrl-k ctrl-s`) need pending state in the key resolver.
- **User remapping** of key bindings is not built, but bindings carry stable action
  identity so an override layer can be added without touching surfaces.
- **Grouping surfaces** that exist only to carry a layout, with no rendering of their
  own — the equivalent of a `div`. They would change what "immediate children" means,
  which is what surface ownership is currently defined in terms of.
- **Transparent and non-rectangular surfaces.** Lip Gloss's layers are opaque
  rectangles; going further means implementing `ultraviolet`'s drawable interface
  directly and taking a direct dependency on a module with no tagged releases.

## Rendering

froth does not implement a compositor. Lip Gloss v2 ships `Canvas`, `Layer` and
`Compositor` — cell buffer, absolute positioning, clipping, hit-testing by layer ID
— and froth drives them, assigning the global Z ordinals itself because Lip Gloss
sorts Z globally and unstably
([ADR 0007](./docs/adr/0007-froth-owns-the-stacking-math.md)).

This was an open question until it was checked; the finding is in
[`docs/research/bubbletea-v2-compositing.md`](./docs/research/bubbletea-v2-compositing.md),
and M1 ran it: surfaces nested three deep with siblings at each level composite in the
order the ordinals say they should.
Note that [bubbletea#642](https://github.com/charmbracelet/bubbletea/issues/642) is
stale as a reference — it was closed in January 2023 as a duplicate of
[#79](https://github.com/charmbracelet/bubbletea/issues/79), and the capability
landed in Lip Gloss v2.

## Prior art

Nothing found implements addressed messaging or surface management on Bubble Tea.
The adjacent pieces:

- [bubbletea-nav](https://github.com/pgavlin/bubbletea-nav) — screen stack and
  sequential focus management.
- [bubblezone](https://github.com/lrstanley/bubblezone) — resolves mouse events to
  screen regions. The hit-testing half of the problem.
- [bubbletea-overlay](https://github.com/rmhubbert/bubbletea-overlay) — composites
  two models with positioning. Visual only, no dispatch.
- [vtui](https://github.com/unxed/vtui) — Go, Turbo Vision inspired. Modality via a
  Z-ordered frame stack with a single unified dispatcher — the approach that ports.
  Its modal results are delivered by a callback invoked synchronously inside the
  input dispatch, which froth does not do; see
  [`docs/research/vtui-framemanager.md`](./docs/research/vtui-framemanager.md).
- [turbo-tui](https://github.com/four-bytes/turbo-tui) — same idea for
  Rust/Ratatui. Uses a nested modal loop, the approach that does *not* port.

Background on why nested models are painful today:
[bubbletea#751](https://github.com/charmbracelet/bubbletea/discussions/751),
[bubbletea#176](https://github.com/charmbracelet/bubbletea/discussions/176).

## Name

Cheese foam — the light layer that sits on top of the tea.

## License

Apache 2.0. See [LICENSE](LICENSE).
