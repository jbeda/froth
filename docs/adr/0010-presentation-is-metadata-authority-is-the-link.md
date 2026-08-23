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
