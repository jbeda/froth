# Measure may be called repeatedly; render is called once

`Measure` answers "how big would you be at this size" and must change nothing,
because a layout may ask at sizes it never uses. `Render` is called exactly once per
surface per frame, at final geometry, and *may* commit view state — clamping a
scroll offset to a new size being the motivating case. A surface that does not
implement `Measure` is assumed to accept whatever it is offered, so froth never
speculatively renders anything.

This keeps multi-pass layout reachable — flexbox and similar need to probe children
at several constraints — without demanding purity in the one call where resize
adjustments naturally belong. Declaring the contract now costs nothing. Retrofitting
it later would break every surface written against a once-per-frame assumption,
which is why it is recorded before any exist.

## Parked: layout properties will face the Environment's question

A layout reads handles and metadata. The moment any of that metadata is something *either*
a surface or its parent could reasonably set — a stretch weight, an alignment, a minimum
size — it faces the question [ADR 0017](./0017-a-surface-and-its-parent-each-get-a-say.md)
answers for ambient values: who writes it, and who wins when both do.

Measure and `Arrange` do not have this problem and are not what this is about. They are a
protocol with two distinct roles — the child answers how big it would be, the parent decides
where it goes — so there is nothing to contend. The prior art puts declarative layout
properties on the parent's side of that line: CSS declares `flex-grow` on the child but
writes it from a stylesheet the parent controls, Flutter wraps in `Expanded`, and Qt puts
stretch factors on the layout object outright.

Nothing is parked here yet because froth has no such metadata. Recorded so that whichever
change introduces the first piece of it asks the question deliberately, with ADR 0017's
two-slot answer already on the table, rather than picking a side by accident.
