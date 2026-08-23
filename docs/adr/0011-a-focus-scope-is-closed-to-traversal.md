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
