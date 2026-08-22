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
