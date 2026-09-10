# A focus scope is closed to traversal in both directions

Keyboard traversal runs within one focus scope and never crosses its boundary. A
surface's scope is the nearest `FocusScope` **at or above** it — itself included — and
the root when nothing above it is one. The walk within a scope covers that scope's
surfaces alone: a nested scope and everything under it are absent from it, and the scope
surface itself is present, so `TabStop` on a scope admits it to its own cycle and to no
other.

Focus crosses a boundary only by being moved by name. Such a move **enters** the scope: it
lands on the child the scope last had focus on, or on the scope's first tab stop. This is
the one place in froth where a focus move does not land on the handle it names.

## Considered options

**A scope belongs to the scope above it.** Then focus resting on a dialog and pressing tab
walks out into the application behind it, which is the one thing a dialog exists to
prevent.

**`tab` enters a scope as one unit.** Reads well — the walk above treats the scope as a
single stop and entering resolves to a control inside — until you notice traversal cannot
bring focus back out. Every stop *after* the scope in child order becomes unreachable, so
a scope halfway down a tree eats the rest of the tab order. The two real cases already
move focus explicitly: a dialog is focused when it opens and focus is put back when it
closes.

**`Focus(scope)` lands on the scope surface, literally.** Consistent, and useless: what a
dialog wants focused is one of its controls, so every caller writes the same descent by
hand. Entering is exempted for a move that starts *inside* the scope — otherwise a walk
reaching a scope marked `TabStop` would be sent straight back to the child it just came
from.

## Consequences

A scope holding no tab stop at all keeps focus on its own surface. That is a defined place
to be, not a rejected move and not a diagnostic.

Restoration state is a `lastActive` handle on **every** node, written for the whole chain
each time focus commits, and read only when a scope is entered. It is never consulted to
answer where focus is: that is one handle and the chain derived from it. A remembered
surface that has died, or that its creator kept out of traversal, falls back to the
scope's first stop.

Modality is a separate flag and arrives later. A modal will be a scope, but a scope is not
a modal: this decides where the keyboard walks, not who receives input.

## Amendment: the boundary bounds directional movement too

A directional move is bounded by the focus scope exactly as traversal is, and by the same
implementation: candidates come from `focusScopeOrder` of the source's scope, so a nested scope
and everything under it are absent, and the scope surface itself is present. There is one
answer to "where can the keyboard go" rather than two that can disagree.

**When the geometrically obvious target is outside the scope, nothing moves.** This is the
question traversal never had to answer, because tab's wall is invisible and nobody expects
to cross it. A spatial wall is *visible*: the user is aiming at something they can see and
pressing the key that points at it.

The alternative — clamp to the nearest candidate inside the scope — was rejected. It is
never silent, which is its whole appeal, but it can move focus away from the direction
pressed, and moving left when the user pressed right teaches that the rule is
unpredictable. Nothing-moved teaches that there is a wall, which is true, and is re-learned
consistently at every edge of every scope.

Precedent agrees on the bound: Android's `FocusFinder` and the WICG spatial-navigation
proposal both search within a container rather than the whole screen. Neither has a
satisfying answer to what happens at the edge, which is why this one is chosen on purpose
rather than left to fall out.

## Amendment: the redirect is keyed on admission, not on direction of approach

The rule above made entering conditional on where focus already was: a move to a scope from
outside entered it, and the same move from inside was literal. That is superseded. Focus moved
to a surface **admitting focus by no route** redirects into it; a surface admitting focus by
any route — `TabStop` or `ClickFocus` — takes the move literally. See
[ADR 0019](./0019-focus-is-admitted-on-two-axes-and-a-click-must-find-you-first.md).

One predicate, no dependence on where focus was, and the "only from outside" exemption is
gone. This rule was proposed in review while building #12 and rejected then for a reason that
no longer holds: keyed on `TabStop` alone it left an unwalkable container permanently
unfocusable. A surface can now admit clicks without joining the walk, so naming such a
container is a move that can land.

What a dialog sees is unchanged — a scope declaring no admissions still redirects onto what it
remembers, or its first tab stop. What changes is `Focus(scope)` from inside, which no longer
lands on a scope surface that admits nothing at all.

## Amendment: a modal is a scope, and opening one moves focus into it

Settled in the M4 design session for #49 and #51. *"A modal will be a scope"* above was a
sentence in the vocabulary and not a rule in the code: `Modal()` set one flag, every
traversal predicate tested `FocusScope` alone, and nothing moved focus when a modal was
created. Three things follow from what a modal is — a claim on all input for its parent's
subtree, the keyboard's counterpart to Capture — and each was missing.

**`modal` is a scope boundary in the predicates.** `focusScopeOf`, `focusScopeOrder` and the
redirect test `FocusScope || Modal`. An unscoped modal is a trap rather than a choice:
traversal is froth-serviced and survives modality, so tab from the last control walks out to
a surface whose every key is then dropped. If all input in the subtree belongs to the modal,
a walk leaving it is handing focus to surfaces that cannot receive. `FocusScope()` remains
for the non-modal case.

**Opening a modal moves focus into it.** Through the redirect, so it lands where the modal
remembers or on its first tab stop, and on the modal surface itself when it holds nothing
focusable. The move runs at the end of the Apply that created it, after the mount drain,
because at `Create` the modal has no children yet; and it runs only if focus is not already
inside the modal, so a `Mount` that focuses its own child wins — the ordering Apply already
promises. Leaving this to the caller means every opener writes the same line and a
forgotten one strands focus behind the dialog with no diagnostic.

**Closing a modal restores focus.** Where focus was when the modal opened is saved on the
modal's node at `Create`, and put back when the modal leaves the tree, by any route, if focus
is inside it then. Focus moved behind the modal by name while it was open is a choice the
application made, and froth leaves it. Nested modals need no stack: each saves its own and
restores its own.

**A saved position that is dead or leaving resolves through the removal.** Two modals can
leave together — an opener dying takes both its dialog and the dialog that one opened — and
the inner's saved position is inside the outer. Before the registry is cut, while ancestry
is still readable, a saved target that is leaving is replaced by the saved target of the
leaving modal it sits in, repeatedly, and only then does the ordinary rehome fall back. This
is what keeps a cancel from two dialogs deep landing on the row the first dialog was opened
from rather than wherever the root's chain happens to point.
