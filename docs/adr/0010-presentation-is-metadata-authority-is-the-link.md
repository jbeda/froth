# Presentation is metadata; authority is the parent link

A surface's parent carries what the surface may do and what may be done to it —
lifetime, modality scope, routing chain, stacking context. Anything that changes only
how a surface is *drawn* is metadata on its handle instead.

The test, for any property somebody wants to vary per surface:

> Does overriding it change who may destroy this surface, who hears from it, or who may
> act on it? Then it stays on the parent link. Does it change only how the surface is
> drawn? Then it is metadata.

## Why this is worth a decision

[ADR 0006](./0006-one-parent-relation.md) says one link carries five properties, and
immediately carves out clipping as a per-surface flag. Two more carve-outs followed —
layout participation and placement space, both from `Float` — and a design with three
exceptions and no principle is a design that will accept a fourth without argument. The
web is the cautionary case: `z-index`, `position`, `contain` and `will-change` each
create or escape a stacking context, and no rule says which properties may.

The test earns its keep by refusing something that looked reasonable. A surface parented
to a pane cannot escape that pane's stacking context to draw above a neighbouring pane —
which reads like a presentation concern, and is not. Hit-testing resolves in draw order,
so what a click reaches is decided by stacking; letting stacking escape the link would
let a surface intercept input on behalf of a subtree it does not belong to. That is
authority wearing a presentation costume, and Win32's `owner` relation is where the same
pressure led.

## What sorts which way

**Metadata**, because they change only drawing: clipping; layout participation (`Float`
takes a child out of the list its parent's layout arranges); placement space (a float is
placed against the screen rather than its parent's rect, which costs nothing because
froth never translates coordinates — every rect is already absolute).

**On the link**, because they change authority: lifetime and the destruction cascade;
modality scope; the routing chain, including where intents walk; stacking context, per
the argument above.

## Consequences

A capability that wants to vary per surface must argue its way past the test rather than
arriving as a flag next to the other flags. A property that fails is not thereby
forbidden — it means the surface belongs somewhere else in the tree, which is a
structural answer to a structural question.

The known cost is on the table: a dropdown parented inside a pane is covered by a later
sibling wherever it overflows that pane. The alternative was a second stacking relation.
Modals are real today and dropdowns are theoretical, so the cost is accepted and this
gets revisited if real screens hit it.

## Amendment: input flags sort as metadata, and why that is not a loophole

Applied in M2, when `Create` grew its option list and the first three flags landed
(`TabStop`, `TextSink`, `FocusScope`). All three are about **input**, which is the one
word the test above does not use, so they are worth walking through rather than filing
next to clipping without comment.

The test asks whether overriding the property changes who may destroy this surface, who
hears from it, or who may act on it. None of the three does. Each changes only what
happens *to the surface carrying it*: whether the keyboard walk stops there, whether the
keys it is already receiving are matched as shortcuts first, whether a walk that entered
its subtree leaves again. A surface cannot use any of them to take input on behalf of a
subtree it does not belong to, which is the specific harm the stacking argument above
refuses — and it is why they are not the fourth exception the test was written to stop.

The line to hold if a fourth input flag is proposed: a flag that redirects input **away**
from where the tree says it goes — one surface claiming another's keys, or claiming them
for a subtree it is not in — is authority, and belongs on the link or nowhere. Modality is
exactly that. The sketch spells it as a create option, and it can stay one, because what
the flag says is only *that* the surface is modal: the subtree it takes input for is the
one its parent gives it, so the authority is in the link either way.

## Amendment: modality scope stays on the link, and Qt is the reason to watch it

The amendment above argues modality can be a create option because *"the subtree it takes
input for is the one its parent gives it, so the authority is in the link either way."*
That holds. It is also, precisely, `Qt::WindowModal` — and prior art gathered for M3
(`docs/research/modal-parenting.md`) found Qt is the **only** system of seven that derives
blocking scope from the parent chain.

Everyone else makes blocking a separate, explicitly-scoped mechanism: the web spends a
whole spec concept on `inert`, enumerated facility by facility; Flutter inserts a
`ModalBarrier` widget whose job is absorbing pointers; Android sets a window flag; GTK's
`set_modal` is application-wide regardless of `transient-for`. GTK is the counter-design
worth naming — `transient-for`, `set_modal` and `destroy-with-parent` are three
independent knobs, none implying another.

Qt is also the one whose blocking behaviour its own documentation undercuts: `exec()`
silently forces `ApplicationModal`, overriding a scope the caller configured.

froth keeps the fusion, deliberately. Its tree is one in-process tree with no window
manager and no nested loops, so Qt's failure modes are not froth's. But the cost is
concrete and should be stated rather than discovered: **froth cannot express "modal to this
pane, but outliving this pane."** Lifetime and blocking scope ride the same link, so
choosing subtree-scoped modality also chooses opener-coupled lifetime. If that combination
is ever wanted, this is the decision to reopen.

## Parked: `Modal()` implying `FocusScope()`

`CONTEXT.md` says *"Dialogs and modals are scopes"*, and the sketch passes both options on
the same create. They stay independent, and no case was found for the combination that
separating them permits.

A modal that is not a focus scope lets tab walk focus out of it into the region it blocks,
where every key is dropped until the user tabs back in. Every construction tried is neutral
or worse: a modal with nothing focusable inside is better off as a scope, where tab stays
put rather than escaping; the only shape where escaping is useful is a modal parented below
the root whose neighbour is outside the blocked subtree, which is the *"modal to this pane,
but outliving this pane"* the amendment above already records froth as unable to express.

The field couples them universally, but structurally rather than by decision: a modal is a
window or a screen everywhere else, so focus containment comes free. The web does it by
making everything outside the dialog inert — *"If new focus target is a focusable area and
its DOM anchor is inert, then return"*, with the mirror rule for unfocusing — and froth has
no inert or disabled surface.

They are kept apart on ordering rather than on a case. ADR 0019's reasoning for moving
`TabStop` onto the type does not reach here: that moved because the *component author* knew
the answer and the *creator* was the one who could forget, two different people. This is one
caller writing `froth.Modal()` and `froth.FocusScope()` on one line, and forgetting produces
a loud failure found in the first minute of use rather than a silent one that ships. Sugar
stays addable; an implication would not be removable.

Revive it on a case for a modal that is deliberately not a scope, or on the forgotten
`FocusScope` showing up as a real defect.
