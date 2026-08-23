# dir-nav

A board for judging directional focus movement.

Every froth feature on this screen is already demonstrated by
[`examples/panes`](../panes). What this adds is a layout worth navigating: a screen shaped
like a real application, arranged so that nothing lines up across columns, with keys that
slide the dividers around underneath a fixed focus.

It exists to be sat in front of and pressed at. The question is not what the rule does —
[`direction.go`](../../direction.go) says that, and
[ADR 0013](../../docs/adr/0013-direction-is-resolved-geometrically.md) says why — but
whether what it does is what your fingers expected.

```
task run:dir-nav
```

It needs a window of at least 80×24 and says so below that.

## The board

A banner along the top, a toolbar of three buttons at three widths, a sidebar of four rows
at three heights, a large detail pane, two panels stacked on the right above a focus scope,
and a filter bar along the bottom. The dots behind it all are the root, so the space the
layouts leave is visible rather than black.

The sidebar rows are separate surfaces rather than lines inside one, because controls are
surfaces. That is what makes leaving the list and coming back a question about geometry
rather than about a remembered selection — see below, it is the sharpest thing here.

## Keys

| key | what it does |
| --- | --- |
| `shift` + arrow | move focus by where the surfaces actually are |
| arrow | whatever holds focus gets it — until `m` |
| `tab` / `shift+tab` | every stop, in declaration order |
| `m` | swap froth's `shift`+arrow defaults for the bare arrows, and back |
| `g` | into the focus scope, and back out |
| `-` `=` | narrow and widen the column focus is in |
| `[` `]` | shorten and heighten it |
| `q` / `ctrl+c` | quit |

The size keys move the divider beside whatever holds focus — there is no divider to select,
because the one you want is always the one next to what you are looking at. They are what
makes this a board rather than a screenshot: the geometry can be walked one cell at a time
to exactly where the rule changes its mind, without focus moving and without a rebuild.

## Which rule decides

`better()` in `direction.go` applies five tests in order, and the first that separates two
candidates picks the winner. With the board at its default splits, each of them is the first
separator for at least one press:

| test | press | what happens |
| --- | --- | --- |
| 1 — in the beam | `shift+down` from Compose | Inbox and the detail pane are both flush below; Inbox is in the beam |
| 2 — nearest ahead | `shift+down` from Inbox | every row below is in the beam, so the closest wins |
| 3 — straightest | `shift+left` from the detail pane | all four rows are flush against it; Sent is nearest straight ahead |
| 4 — reading order | `shift+right` from the detail pane | Metadata and Labels tie only once `[` and `]` have centred them on it; then the topmost wins |
| 5 — drawn last | unreachable here | it needs two surfaces with the same rect, which a tiling board never produces |

The fifth closes with floats and modals in M4 (#18).

## Things worth having an opinion about

**Going somewhere and coming back does not return you where you were.** From Inbox,
`shift+right` lands on the detail pane and `shift+left` comes back to Sent — the row nearest
straight ahead, whichever row you left from. The rule is geometry, and geometry has no memory
of the trip. The same happens between the sidebar and the filter bar.

This is the board's first real finding and it is collected as **#28**: every toolkit that has
met it answers the same way, splitting the problem as *geometry picks the container, memory
picks the member*.

**An arrow and its opposite do not undo each other.** In the scope, `shift+down` from Reply
lands on Archive and `shift+up` comes back to Forward. No rule change can fix this: Reply and
Forward both move down to Archive, so `up` has one answer for two callers. Collected as
**#29**, along with the two things that could deliver it — neighbours the arrangement
declares, or a memory of the edge focus crossed.

**A move into the scope does nothing, and nothing says why.** Arrows cross a focus scope in
neither direction; `g` is the only way in or out. Pressing `shift+right` at the detail pane's
lower edge is indistinguishable from pressing it at the edge of the screen.

**Bare arrows are their own argument.** Press `m`, then `tab` to the filter bar: the field can
never have a cursor, and `m` cannot swap you back while it holds focus, because `m` is a
character there too. That is why froth's default is `shift` with the arrows.

**Focus never changes how much room a surface has.** The focus marker's column is reserved
whether or not it is drawn. A title that grew when focused wrapped inside a box that had not
moved, which reads as a layout bug and is not one.

## How it knows where focus is

It asks the surfaces. A surface learns it is focused during `Render` and froth tells nobody
else — a directional move is serviced by froth and delivered nowhere, so no message announces
it. Each surface records itself as it draws, into the board they share, and the banner —
created last, so it renders last — reports what they wrote. The same record is what tells the
root which divider the size keys should move.

An application does not need this and should not copy it. A tool does, and it is collected on
**#24** (`froth/inspect`) rather than being worked around twice.

## What is not here

No tests, deliberately: a test pinning which surface a press lands on would pin a board that
is meant to keep changing through M3 and M4, and would turn every layout tweak into a failure.
The rule itself is tested in `direction_test.go`, where the board's geometry cannot reach it.

No library code either. Two frictions this example hit are filed rather than worked into froth
— **#25**, a layout cannot be told anything about a child, so `shell` reads its six children by
index; and **#26**, a surface can learn its rect only during `Render`, so the layout has to
stash the space it last divided for the key handler to clamp against. Both are M7 (#27).
