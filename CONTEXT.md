# froth

Window and surface management for Bubble Tea. froth owns a tree of independently
addressed surfaces, composites them to the screen, and routes input and messages
to them.

## Language

**Surface**:
An independently addressed unit of screen and behaviour that renders itself and
handles messages sent to it. The unit of composition in froth.
_Avoid_: window, component, view, screen, panel

**Handle**:
The stable identity of a surface, valid to store and to name as a message
destination. A handle outlives the surface it names, so delivery to a dead
surface is a defined drop rather than an error.
_Avoid_: id, ref, pointer

**Parent**:
The surface a surface hangs off. Parenthood determines coordinate origin,
lifetime, stacking context, modality scope, and the routing chain — one link
carrying all five.
_Avoid_: owner, container, host

**Stacking context**:
The ordering scope a surface's Z-order applies within. A surface's Z orders it
only among its siblings; it never orders it against a surface under a different
parent.

**Focus chain**:
The path from the root to the deepest active surface. Focus itself is one
authoritative handle and the chain is derived from it by walking parents, so
the two can never disagree. A parent remembering which child was last active is
restoration state — where focus lands when it re-enters that subtree — not what
the chain is made of. Input enters at the deep end.
_Avoid_: focus stack, selection path

**Active surface**:
The deepest surface on the focus chain — the one that sees input first.
_Avoid_: focused window, current surface

**Modal**:
A surface that takes input for its parent's whole subtree while it is open. A
surface parented to the root is modal to the entire application.

**Cmd**:
Bubble Tea's unit of asynchronous work: a function run off the update loop whose
returned message is posted back into the program. Bubble Tea delivers every such
message to one global entry point with no record of who issued it, so froth
tags them itself.

**Environment**:
Ambient values a surface reads from its ancestors rather than being handed at
construction — theme, capabilities, locale. Bound to a surface and scoped to its
subtree: resolution walks root-ward and takes the nearest binding, so a subtree
can override what is above it. Resolved at read time, so it is never a stale
snapshot.
_Avoid_: deps, context, config, globals

**Intent**:
A request a surface returns to its ancestors, applied by froth after the
surface's handler has returned. It is a return value, not a message, so it can
never be delivered re-entrantly.
_Avoid_: action, event, callback

**Live tree**:
The surface hierarchy as it logically stands, including changes made during the
current update. What structural queries answer by default.
_Avoid_: current tree, model tree

**Rendered frame**:
The structure and geometry as of the last render. The only place geometry
exists, and what a click resolves against. Read-only.
_Avoid_: last frame, screen state

**Focus scope**:
A surface whose subtree keyboard navigation cycles within, rather than escaping
into the rest of the tree. Dialogs and modals are scopes; plain layout
containers are not.
_Avoid_: focus trap, focus group

**Phase**:
Which part of an update froth is in — Deliver, Apply, or Render — and therefore
what a surface is allowed to do. Delivery may change only the receiving
surface; structure changes only in Apply; Render may not change structure at
all.

**Measure**:
A surface's answer to "how big would you be at this size", asked by a layout as
many times as it needs. Answering must never change anything, because most
answers are discarded.

**Chrome**:
The border, title, and focus indication froth draws around a surface from its
handle metadata. A surface renders only its client area and never its own
chrome.
_Avoid_: decoration, frame, border

**Layout**:
An object attached to a container that turns the container's rect and its
children's measures into a rect per child. Layouts see handles and metadata,
never surface state.

**Query**:
A read-only walk over the surface tree collecting from surfaces that implement
a given capability — help bindings being the first. The only synchronous call
into a surface.

**Capture**:
A claim on all mouse input regardless of what lies under the cursor. Held by at
most one surface, and lost unilaterally — a surface that loses capture must
abandon what it was doing, never finish it.

**Producer**:
A long-lived goroutine froth runs on a surface's behalf, emitting messages to
that surface's handle until the surface dies. Producers may not touch surface
state.
_Avoid_: subscription, worker, stream

**Topic**:
A named channel a surface subscribes to and an outside system posts to, for
events with several interested surfaces or a lifetime longer than any of them.

**Anchor**:
A scroll position expressed as an item plus an offset within it, rather than an
absolute line. Lets a surface scroll unbounded content without ever measuring
all of it.

**Action**:
What a shortcut resolves to: a stable identity a surface declares along with its
default keys and help text. Surfaces handle actions rather than keys, so keys can
be remapped without touching them.
_Avoid_: command, shortcut, hotkey

**Text sink**:
A surface that consumes ordinary typing. While one is focused, bare printable keys
are never matched as shortcuts; modified chords still are.

**Leaf**:
A surface with no children. Adapted third-party components are always leafs,
because a component that cannot report what it consumed can have nothing beneath
it and nothing above it that trusts its answer.

**Post**:
To enqueue a message for later delivery to a surface. All message delivery in
froth is posted; nothing is delivered re-entrantly.
_Avoid_: send, dispatch, emit

**Diagnostic**:
Something froth detected and should not decide about — a message cycle, an
intent nobody claimed, a surface that panicked. All of them go to one hook
registered at construction, because none of them is froth's to escalate.
_Avoid_: error, warning
