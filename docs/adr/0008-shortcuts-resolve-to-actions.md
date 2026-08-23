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

froth hardcodes no quit key. The consequence is deliberate and worth stating plainly: an
application that marks nothing as surviving can write a modal it cannot quit past, and
froth will not save it. The alternative is froth deciding what quit means, which is worse
— a double-tap `ctrl-c` is a common and perfectly reasonable policy, and it belongs to the
application. Such a policy needs no new mechanism either: the surface receives its action
twice and compares timestamps.

## Amendment: froth binds the keys for the actions it services itself

The amendment above once read "froth owns no bindings of its own and hardcodes no quit
key". The first half was overreach: it was written about quit and generalised to every
binding, and it contradicts both the sketch's `froth.ActionCancel, // esc, per froth's
stock bindings` and the user story that wants tab to move focus without an application
implementing it.

**The line is what froth services.** Moving focus is froth's job — no surface implements
traversal — so the action that asks for it is froth's, and so is the key. Quit is not:
froth would only forward it to a surface, so it stays the application's to declare.
`ActionFocusNext` and `ActionFocusPrev` are froth's with `tab` and `shift+tab` bound to
them; `ActionCancel` is a froth identity with no key, because froth services nothing for
it until modals exist.

**An application takes one over through this same machinery.** froth's stock bindings are
matched after every surface on the focus chain, so a surface that declares `tab` for
something else simply wins. Declaring a `Binding` for a froth action anywhere on the chain
replaces froth's keys for it, and a declaration marked `Disabled` leaves the action with
no keys at all — as inert as any other disabled binding. There is no second override
table, and there is nothing an application can do to a binding of its own that it cannot
do to one of froth's.

**A serviced action is delivered nowhere.** The surface that declared the binding asked
froth to move focus, not to be told that it did.
