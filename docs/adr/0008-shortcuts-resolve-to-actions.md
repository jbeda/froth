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

## Amendment: modality truncates the chain

Settled before M2 was sliced. A modal takes input for its parent's whole subtree, and
that has to mean bindings as well as raw keys: with focus inside a modal the chain still
runs modal → … → root, so on the rule above every ancestor's bindings would keep firing
underneath it. **Resolution stops at the modal.** Nothing above it matches.

One exception, declared rather than built in: a binding carries a `SurvivesModal` flag,
and those still resolve from above. It survives modality only — not focus scopes, not
anything else — and any surface may declare one.

froth owns no bindings of its own and hardcodes no quit key. The consequence is
deliberate and worth stating plainly: an application that marks nothing as surviving can
write a modal it cannot quit past, and froth will not save it. The alternative is froth
deciding what quit means, which is worse — a double-tap `ctrl-c` is a common and
perfectly reasonable policy, and it belongs to the application. Such a policy needs no
new mechanism either: the surface receives its action twice and compares timestamps.
