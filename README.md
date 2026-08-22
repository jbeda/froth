# froth

Window and surface management for [Bubble Tea](https://github.com/charmbracelet/bubbletea).

**Status: nothing here yet.** This README records the design intent so work can start
from a shared picture. Everything below is a decision or an open question, not a
description of code that exists.

## What it's for

Bubble Tea gives you one model, one `Update`, one `View`. Composing anything
non-trivial means hand-rolling a router in your root model — the community's
standing answer to nested components. `froth` aims to make surfaces first-class:

- **Window / surface management** — surfaces that own their own state and rendering,
  addressed independently rather than reached by walking a model tree.
- **Componentization** — components that compose without every parent needing to know
  how to forward messages to every descendant.
- **Flexible layout, including tiling** — layout as a pluggable concern, not something
  hardcoded into each application's `View`.
- **Modals** — supported, but as one case of surface management, not the headline.

## Design decisions so far

Light inspiration from the Win32 messaging model — addressed messages, a message
pump, per-window handlers. Light. This is not a Win32 port and not a retro-aesthetic
project; the interaction model is the interesting part, not the API surface or the
look.

**No reentrant message delivery. No nested message pumps.** This is the firm one, and
several other decisions follow from it.

**Modals are asynchronous.** Since there's no nested pump, a modal cannot block and
return a result. Opening one pushes a surface with a continuation; closing it pops and
delivers the result to that continuation. Callers get an ergonomic close to
"call and handle the result" without recursion.

**Synchronous send, for frame coherence.** A `SendMessage` analogue is wanted — not for
control flow, but so a surface can query another within a single frame and avoid frame
skew from round-tripping through the queue. Note this is a direct call into the target's
handler, which is not the same thing as nested pumping.

Guards worth having, given synchronous send:

- Send only from the pump goroutine; everything else posts.
- A per-surface reentrancy guard, so a send into a surface already on the call stack is
  a defined error rather than a stack overflow.
- Distinguish query messages (may be sent, must not mutate) from command messages
  (must be posted), and enforce it at registration.

**Message reordering** is under consideration — priority tiers rather than strict FIFO,
so that paint and timer-ish messages can be coalesced and deprioritized. Undecided.

## Open questions

- **Where does surface state live?** Synchronous send is hard to reconcile with surfaces
  held as values inside a parent's model — it likely pushes toward a flat registry of
  surfaces addressed by handle, held outside any value tree. That is a departure from
  idiomatic Elm-architecture Bubble Tea and is the load-bearing decision; everything
  else follows from it.
- **Reordering:** worth the complexity, or does strict FIFO plus explicit invalidation
  get most of the benefit?
- **Rendering:** does Bubble Tea v2 / Lipgloss v2 provide a compositing or layer
  primitive that covers the rendering half? Unverified — check before building a
  compositor. See [bubbletea#642](https://github.com/charmbracelet/bubbletea/issues/642).

## Prior art

Nothing found implements addressed messaging or surface management on Bubble Tea.
The adjacent pieces:

- [bubbletea-nav](https://github.com/pgavlin/bubbletea-nav) — screen stack and
  sequential focus management.
- [bubblezone](https://github.com/lrstanley/bubblezone) — resolves mouse events to
  screen regions. The hit-testing half of the problem.
- [bubbletea-overlay](https://github.com/rmhubbert/bubbletea-overlay) — composites two
  models with positioning. Visual only, no dispatch.
- [vtui](https://github.com/unxed/vtui) — Go, Turbo Vision inspired. Modality via a
  Z-ordered frame stack with a single unified dispatcher — the approach that ports to
  Bubble Tea. Worth reading the `FrameManager` source.
- [turbo-tui](https://github.com/four-bytes/turbo-tui) — same idea for Rust/Ratatui.
  Uses a nested modal loop, which is the approach that does *not* port.

Background on why nested models are painful today:
[bubbletea#751](https://github.com/charmbracelet/bubbletea/discussions/751),
[bubbletea#176](https://github.com/charmbracelet/bubbletea/discussions/176).

## Name

Cheese foam — the light layer that sits on top of the tea.

## License

Apache 2.0. See [LICENSE](LICENSE).
