# Controls are surfaces

Anything interactive or independently addressable is a surface, buttons included.
Hit-testing then becomes a walk of the tree froth already maintains rather than a
parallel registry of clickable regions; tab order becomes traversal of that same
tree; focus indication comes uniformly from chrome metadata. Non-interactive
decoration stays inline in its parent's body — this is "controls are surfaces", not
"every glyph is a surface".

## Consequences

Render stops returning the whole picture. A container returns its own background and
static content, and its interactive children are composited on top at rects its
layout assigned. This is the largest single consequence in the design and it is much
cheaper now than once there are consumers.

Layer count is the cost: the compositor flattens and sorts the whole set every
frame. A few hundred surfaces is comfortable; a few thousand would need measuring.
