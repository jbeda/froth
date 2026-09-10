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
at three heights, a large detail pane, two panels at two heights on the right, and a filter
bar along the bottom. The dots behind it all are the root, so the space the layouts leave is
visible rather than black.

Two things can be put over it, both floats — placed against the screen rather than by a
layout, and sized by what they draw. `g` opens the **Actions dialog**, which is modal: it
takes focus, takes the board's keys away while it is up, and hands focus back where it found
it as it goes. `f` shows the **twins**, a pair of one-line strips that draw the same width
and so land on the same rect.

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
| `g` | open the Actions dialog, and close it again from inside |
| `esc` | close the dialog |
| `enter` | pick the control focus is on, which also closes the dialog |
| `f` | show the twins, and hide them |
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
| 4 — reading order | `shift+right` from the detail pane | Metadata and Labels tie only once `]` has centred them on it — four presses with focus on Metadata; then the topmost wins |
| 5 — drawn last | `shift+right` from Drafts, with the twins up | the twins share one rect, so the four tests above it all tie; twin B was created second, drawn over twin A, and is the one you are looking at |

The fifth needs the twins to cross the row focus is in. They sit on the screen's middle line,
which at 100×30 is Drafts. Two screens where that press does not reach them, and what does:
on one taller than about 37 lines they fall clear below the sidebar, and `shift+down` from
Sent reaches them; at the 80×24 minimum the detail pane's centre sits too close to theirs, and
four presses of `]` from the detail pane — which shortens it — puts a twin ahead of it.

## Things worth having an opinion about

**Going somewhere and coming back does not return you where you were.** From Inbox,
`shift+right` lands on the detail pane and `shift+left` comes back to Sent — the row nearest
straight ahead, whichever row you left from. The rule is geometry, and geometry has no memory
of the trip. The same happens between the sidebar and the filter bar.

This is the board's first real finding and it is collected as **#28**: every toolkit that has
met it answers the same way, splitting the problem as *geometry picks the container, memory
picks the member*.

**An arrow and its opposite do not undo each other.** In the dialog, `shift+down` from Reply
lands on Archive and `shift+up` comes back to Forward. No rule change can fix this: Reply and
Forward both move down to Archive, so `up` has one answer for two callers. Collected as
**#29**, along with the two things that could deliver it — neighbours the arrangement
declares, or a memory of the edge focus crossed.

**A modal takes the keys away silently.** Inside the dialog the size keys do nothing and
nothing says why: a modal suppresses every binding that did not ask to survive it, which is
what a modal is for, and is indistinguishable from a key nobody ever bound. `q` survives
because it says so; the arrows keep working because froth services them itself.

**A float is offered the whole screen.** The banner reports the size the focused surface was
offered, so it reports a twin as the whole screen rather than the strip it drew. That is not
the banner being wrong: a float is handed the screen and takes the rect its output makes, and
the offer and the rect are two different things for exactly one kind of surface.

**Bare arrows are their own argument.** Press `m`, then `tab` to the filter bar: the field can
never have a cursor, and `m` cannot swap you back while it holds focus, because `m` is a
character there too. That is why froth's default is `shift` with the arrows.

**Focus never changes how much room a surface has.** The focus marker's column is reserved
whether or not it is drawn. A title that grew when focused wrapped inside a box that had not
moved, which reads as a layout bug and is not one.

## How it knows where focus is

It asks the surfaces. A surface learns it is focused during `Render` and froth tells nobody
else — a directional move is a froth action nobody here takes, so the next frame is what
announces it rather than a message. Each surface records itself as it draws, into the board they share, and the banner —
created last, so it renders last — reports what they wrote. The same record is what tells the
root which divider the size keys should move.

An application does not need this and should not copy it. A tool does, and it is collected on
**#24** (`froth/inspect`) rather than being worked around twice.

## Where the rule stands

#22 closed on this board with the tie-break order intact: beam, nearest ahead, straightest,
reading order, drawn last — probably as good as a purely geometric rule gets, and every one
of the five is now the first separator for a press you can make. What is missing is not a
better ordering but the systems beside it, filed as #29 and #28.

`shift`+arrow stays the default and stays provisional, which is why `m` is still here.

## What is not here

No tests, deliberately: a test pinning which surface a press lands on would pin a board that
is meant to keep changing through M3 and M4, and would turn every layout tweak into a failure.
The rule itself is tested in `direction_test.go`, where the board's geometry cannot reach it.

No library code either. Two frictions this example hit are filed rather than worked into froth
— **#25**, a layout cannot be told anything about a child, so `shell` reads its six children by
index; and **#26**, a surface can learn its geometry only during `Render` — and only its
size, never where it sits — so the layout has to stash the space it last divided for the key
handler to clamp against. Both are M7 (#27).
