# One parent relation, carrying five properties

A surface's parent determines its coordinate origin, its lifetime, its stacking
context, its modality scope, and its routing chain — one link carrying all five.
Clipping is the deliberate exception, a per-surface flag: a layout container wants
to clip its children, while a surface that is a parent purely for ownership or
modality does not.

Win32 splits this into two relations, parent and owner, which are mutually exclusive
and govern different things. That split exists because a top-level window
structurally cannot have a parent — a constraint froth does not inherit. Parenting a
modal to the root is precisely the case Win32 needs `owner` for, and here one
relation covers it. See `docs/research/win32-hierarchies.md`.
