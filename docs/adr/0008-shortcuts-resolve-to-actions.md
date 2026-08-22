# Shortcuts resolve to actions before input is delivered

A key press is first matched against bindings declared by surfaces on the focus
chain, **deepest first**. The surface whose binding matches receives a typed
**Action** — not the key. Only when nothing matches does the raw key reach the
focused surface. A surface marked a **text sink** suppresses matching for bare
printable keys, so ordinary typing is never stolen, while modified chords still
resolve.

## Considered options

**Deliver to the focused surface, then bubble root-ward when unhandled.** This is
what most Bubble Tea code does and it was the original decision here. It requires
every surface to report truthfully whether it consumed a key, and adapted
third-party components cannot: bubbles' `Update(tea.Msg) (Model, tea.Cmd)` returns
no such signal. A focused text input therefore swallows `ctrl-c`.

**Match root-ward first, then deliver.** Fixes the swallowing, but inverts
precedence: a global binding beats a dialog-local one for the same chord, which is
backwards. Escape inside a dialog belongs to the dialog.

Matching declaratively, deepest first, before delivery gets the precedence of the
first and the timing of the second, and requires nobody to report anything.

## Consequences

Bindings need **stable action identity** so keys can be remapped without reaching
inside surfaces. A surface declares `(ActionID, default keys, help text)`; froth
owns the key-to-action table and the override layer on top of it. Any sophisticated
TUI will want user remapping, and retrofitting it onto bindings with no identity is
unpleasant, so identity is required from the start even though the override layer
is not built yet.

The same declarations feed the help query, so help can never drift from what will
actually fire — including greying out bindings whose `Enabled()` is currently false.

Adapted components are necessarily **leafs**: nothing can sit beneath a surface that
cannot say what it consumed, and nothing above can trust its answer. They keep their
own internal keymaps, which froth cannot see or remap; rebinding those means
mutating the component's keymap directly.

Chorded sequences need pending state in the resolver. Not built, not foreclosed.
