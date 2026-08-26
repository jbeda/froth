# froth

Window and surface management for Bubble Tea. froth owns a tree of independently
addressed surfaces, composites them to the screen, and routes input and messages
to them.

## Language

**Instance**:
The object an application's root model owns and forwards `Init`, `Update` and `View` to.
It holds one surface tree, its registry, and the phase machinery. A program may hold
more than one, and a handle names the instance that issued it, so a handle used against
the wrong instance is a defined drop rather than an accidental alias. Not the
application: the application owns an instance, not the other way round.
_Avoid_: app, screen, manager, context

**Surface**:
An independently addressed unit of screen and behaviour that renders itself and
handles messages sent to it. The unit of composition in froth.
_Avoid_: window, component, view, screen, panel

**Handle**:
The stable identity of a surface, valid to store and to name as a message
destination. A handle outlives the surface it names, so delivery to a dead
surface is a defined drop rather than an error. It is inert data and confers
nothing: acting on a surface means asking a phase context for a `Ctx` scoped to
that handle, and what may be done depends on the tier that mints.
_Avoid_: id, ref, pointer

**Parent**:
The surface a surface hangs off. Parenthood determines the space a surface is
given, its lifetime, its stacking context, its modality scope, and its routing
chain — one link carrying all five, and fixed for the surface's lifetime. It is
the space rather than a coordinate origin because froth never translates: every
rect is already the screen's.
_Avoid_: owner, container, host

**Stacking context**:
The ordering scope a surface's Z-order applies within. A surface's Z orders it
only among its siblings; it never orders it against a surface under a different
parent. Siblings order by their Z and then by creation, and a surface may not
escape the scope its parent gives it — what a click reaches is decided by what
draws on top, so escaping would be a claim on input, not a way of looking.
_Avoid_: layer, z-index

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
surface parented to the root is modal to the entire application. Its parent
decides what it blocks; its opener decides how long it lives, because a modal is
destroyed when the surface that opened it is.

**Cmd**:
Bubble Tea's unit of asynchronous work: a function run off the update loop whose
returned message is posted back into the program. Bubble Tea delivers every such
message to one global entry point with no record of who issued it, so froth
tags them itself.

**Environment**:
Ambient values a surface reads from its ancestors rather than being handed at
construction — theme, capabilities, locale, a service a subtree should use. The
application owns what is in it; froth puts nothing there. Not a surface's own
cancellation, which froth manages and hands over itself. Every surface has two slots,
one it writes on itself and one its parent writes on it, and resolution takes the
first of those before walking to the parent — so a surface always wins over what its
parent gave it, and a subtree always wins over what is above it. Resolved at read
time, so it is never a stale snapshot.
_Avoid_: deps, context, config, globals

**Rising**:
A message that keeps travelling after it arrives. froth offers it to the surface it
was posted to, then to that surface's parent, and on up through the root to the
instance, stopping at the first surface that calls `StopRising`. A message type
declares this by embedding `froth.Rising`, so how far a message travels is a
property of its type rather than of the call that sent it, and a reader of the type
knows its reach without finding the send site. Code that is generic over the message
type asks instead, with `froth.Rises` or `uc.Rises`. One that nothing takes
reaches the tail and is reported.
_Avoid_: bubbling, intent, event

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
into the rest of the tree. The boundary holds in both directions: the walk
outside a scope steps over the scope and everything under it, so focus crosses
only by being moved by name — which is what opening and closing a dialog are.
A scope remembers which child was last active, so being moved to it lands
where focus left. Dialogs and modals are scopes; plain layout containers are
not.
_Avoid_: focus trap, focus group

**Phase**:
Which part of an update froth is in — Deliver, Apply, or Render — and therefore
what a surface is allowed to do. Delivery may change only the receiving
surface; structure changes only in Apply; Render may not change structure at
all.

**Ctx**:
A limited-lifetime struct that lets you operate on something within a particular
situation, and expires when that situation ends. The phase contexts are one kind —
what a surface may do during Deliver, Apply or Render. The handle-scoped kind is
minted from those, and its type says what may be done to the handle it names: reading
anything, reaching a stranger, or restructuring yourself or an immediate child.
Retaining one past its callback panics.

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
The read-only value an application or a surface asks froth questions through,
which can change nothing. It walks the tree reading what froth has stored and
calls into no surface: shortcut resolution, the help view and the action listing
are all reads of the same stored declarations, so no two of them can describe
different worlds. froth makes no synchronous call into a surface at all.

**Binding**:
A surface's declaration that it answers to some keys, under one action, with help
text. A surface pushes its whole set with `SetBindings` — from Mount, from
Reconcile, or from Update where its state changed — and froth stores it against
the handle; nothing asks a surface what its bindings are. A declaration therefore
stands until it is replaced, so a surface whose state moves on from what it last
said declares again. `Disabled` is the field that most needs it.
_Avoid_: keymap, accelerator, hotkey table

**Stock binding**:
A binding froth declares for an action it services itself, matched after every
surface on the focus chain. Declaring the same action anywhere on the chain
replaces it; there is no second override table.
_Avoid_: default binding, built-in, global

**Capture**:
A claim on all mouse input regardless of what lies under the cursor. Held by at
most one surface, and lost unilaterally — a surface that loses capture must
abandon what it was doing, never finish it.

**Producer**:
A long-lived goroutine froth runs on a surface's behalf, emitting messages to
that surface's handle until the surface dies. Producers may not touch surface
state.
_Avoid_: subscription, worker, stream

**Anchor**:
A scroll position expressed as an item plus an offset within it, rather than an
absolute line. Lets a surface scroll unbounded content without ever measuring
all of it.

**Action**:
The stable identity of something the application can be asked to do. A binding is
one way to ask — a shortcut resolves to an action and nothing else, and who acts on
it is decided by delivery, which rises from the active surface — but an action is
not defined by having a key, and invoking one by identity is ordinary. It is a
string because that identity has to survive in a settings file and in front of a
user.
_Avoid_: command, shortcut, hotkey

**Action listing**:
Every action the focus chain currently offers, each with the surface that declared
it — what a command palette shows. It and the help view are two questions asked of
one set of declarations, so neither can name an action the other does not: the
listing keeps an action nobody bound a key to, and a text sink holding focus does
not change it, because a palette's own filter field is one.
_Avoid_: catalog, command list, registry

**Text sink**:
A surface that consumes ordinary typing. While one is focused, bare printable keys
are never matched as shortcuts; modified chords still are.

**Float**:
A surface placed against the screen rather than by its parent's layout, which
never sees it among the children it arranges. Everything else about it is
ordinary: its parent still governs its lifetime, its modality scope, its routing
and the stacking scope it sits in. A dropdown or a popover is a float.
_Avoid_: overlay, popup, absolute

**Leaf**:
A surface with no children. Adapted third-party components are always leafs,
because a component that cannot report what it consumed can have nothing beneath
it and nothing above it that trusts its answer.

**Post**:
To enqueue a message for later delivery to a surface. All message delivery in
froth is posted; nothing is delivered re-entrantly. It is the only verb: a message
whose type is Rising is posted the same way and continues past its destination,
which is why the queue drains completely before structure changes again.
_Avoid_: send, dispatch, emit

**Destroy**:
To take a surface out of the tree, along with everything below it. A surface may
destroy itself or one of its immediate children and no one else; removing a deep
branch means destroying your own child and letting the cascade reach the rest.
The root cannot be destroyed. Recorded during Deliver and committed at the next
Apply, immediate during Apply.
_Avoid_: remove, delete, close, dispose

**Reconcile**:
A surface re-expressing the children it owns, part-way through an Apply, because
its own state has moved on from the tree it built. Asked for with
`Self().Reconcile()` from Deliver or Apply and serviced during Apply; asking more
than once before froth gets there reconciles once. It is written as a function of
desired state rather than as a list of operations, which is what removes the need
for a surface to remember what it has already asked for. Mount is the one-time
half of the same capability and stays a separate method, because Mount also does
setup that is not structural.
_Avoid_: rebuild, refresh, update, sync, invalidate

**Unmount**:
A surface's chance to release what froth does not own, run once when it leaves
the tree by any route. It runs leaf-up, so a child tears down before its parent
and can still reach it while doing so. The mirror of Mount, and a surface that
never mounted is never unmounted.
_Avoid_: teardown, dispose, cleanup, destructor

**Shutdown**:
Tearing the instance down: the unmount walk run once from the root, when the
program is quitting. Afterwards the tree is empty and the instance is spent. It
exists because a teardown hook that only ran on destroy would not run at the one
moment every application expects it to.
_Avoid_: close, dispose, quit

**Quarantine**:
What froth does with a surface that panics inside one of its callbacks: report
it, take it out of the tree by the ordinary destroy path, and never call it
again. A panic froth raised itself is not quarantined — that says the program
violated froth's contract, and burying it would leave the bug in place.
_Avoid_: recover, catch, isolate

**Diagnostic**:
Something froth detected and should not decide about — a message cycle, a rising
message nothing took, a surface that panicked. All of them go to one hook
registered at construction, because none of them is froth's to escalate.
_Avoid_: error, warning
