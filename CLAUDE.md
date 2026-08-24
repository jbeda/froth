# CLAUDE.md

froth is window and surface management for Bubble Tea, in Go. It is being built in
eight vertical milestones; each ships library code and an increment to the example
application.

## Start here

Read in this order before changing anything:

1. **`CONTEXT.md`** — the vocabulary. Use these terms; they are chosen deliberately and
   the `_Avoid_` lists are not decoration.
2. **`docs/adr/`** — the decisions that are hard to reverse, each with the alternatives
   that were rejected and why. Read before proposing something that contradicts one.
3. **`docs/design/kitchen-sink-sketch.md`** — its Reference section is the intended API
   surface, and its findings explain how several decisions were reached.
4. **`docs/testing.md`** — one seam, what to assert on, and why some tests should not
   exist.
5. **`docs/design/milestone-log.md`** — what has actually been built, what changed
   against the design while building it, and which design assumptions are still
   untested.
6. **Issue #1, with its comments** — the spec and the milestone breakdown. Read it as
   `gh issue view 1 --comments`: scope changes and decisions taken ahead of a milestone's
   slicing are recorded as comments, not by editing the body, so the body alone is out of
   date. Reparent being cut and M2's input decisions both live there.

The example application is real, current code and is often the fastest way to
understand the API.

## Precedence

When these disagree:

- **Code** is authoritative for what froth does.
- **ADRs** are authoritative for why, and for what was rejected.
- **The spec** is authoritative for scope.
- **The sketch** is authoritative only for intended ergonomics, and only where code has
  not superseded it.

Where reality has diverged from a document, fix the document in the same change — and
record it in the milestone log if the divergence was a decision rather than a typo.

## Explicit over magic

The base system is obvious and explicit, even when that makes it verbose. Sugar can be
layered on later; a convenience that hides where something came from cannot be removed
once applications depend on it.

In practice this decides a recurring shape of question. When froth could either remember
something on a caller's behalf or make the caller carry it, froth makes the caller carry
it: a message that needs a reply address holds the handle in a field rather than froth
recording who sent it, and a surface that depends on knowing who opened it takes that
handle as a constructor parameter rather than through a create option. The verbose version
says what the dependency is at the place a reader is already looking.

This is not an argument against convenience. It is an argument about order: build the
explicit mechanism, then add sugar over it if the ergonomics turn out to matter.

## Ideas with no case yet

An idea worth keeping but not worth building goes in **the ADR that governs its area**,
never in `CONTEXT.md` and never only in an issue or a commit message. A `CONTEXT.md` entry
names something that exists in code or is scoped in the spec; aspirational vocabulary gets
used as though it were real.

Mark it `Parked:` and say **what evidence would revive it** — a case, not a preference.
ADR 0004 set the form: *"if a case appears that Environment, Intent and Query genuinely
cannot serve, bring that case — not the mechanism."* The index is a grep:

```
grep -rn "Parked" docs/adr/
```

If no ADR governs the area, that absence is the finding: either the area needs a decision
record, or the idea is not ready to park.

## Working state

`HANDOFF.md` at the repo root holds working state across sessions — agreements, current
project state, anything a later session needs and the repo does not already record. It is
gitignored and per-machine. Read it at the start of a session and keep it current; keep it
under 200 lines. Record durable state there rather than in the agent memory system, and do
not create memory files.

Scratch and intermediate files go in `.scratch/` (gitignored), not `/tmp` or a session
temp dir, so they are easy to find and clean out.

## Standard actions

`Taskfile.yml` holds them; `task` with no argument lists them. Use these rather than
inventing a command line, so everyone runs the same thing.

`task check` is the gate before a commit: formatting, `go vet`, `golangci-lint`, tests.
`task build` compiles the example applications into `bin/` — froth itself is a library and
produces no binary. `task test -- -run TestFoo` passes arguments through to `go test`.

Lint settings are in `.golangci.yml`. `golangci-lint` refuses to start unless it was built
with a toolchain at least as new as the `go` directive in `go.mod`, and `go install` builds
it against *its own* `go.mod`, which lags. Force the toolchain when installing it:

```
GOTOOLCHAIN=go1.27.0 go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest
```

## Code comments

Comments document **the current state** and **how to use the thing**. Nothing else.

Two failures matter more than any style question, and both are common enough to name:

- **No implementation log.** A comment is not a changelog, a ticket reference, or a note
  about what a past version did. Never write milestone numbers, issue numbers, "shipped
  in", "was rejected", "arrives later", "inert until", or "M4 will". A reader wants to
  know what the code does now — not the order in which it came to be. Build-time history
  belongs in `docs/design/milestone-log.md`, and rejected alternatives belong in an ADR.
  Those artifacts exist precisely so comments do not have to carry this.
- **Don't restate the code.** If the comment and the line say the same thing, delete the
  comment. `// apply folds options into a meta` above `func (m *meta) apply(opts
  []CreateOption)` earns nothing.

What is worth writing: a non-obvious invariant, a constraint a caller must respect, a
sharp edge that looks like a bug and is not, or a design choice a reader would otherwise
try to "fix". Prefer one dense sentence to a paragraph. If a rationale runs longer than a
few lines it is an ADR, and the comment should be a pointer to it.

Package and exported-identifier doc comments say what the thing is and how to use it,
sized to what a caller needs.

Length is the tell: a 40-line comment over a 3-line function is wrong regardless of how
good the prose is. Err toward too little.

## Messages a human reads

Panic strings and `Diagnostic` details are built with `fmt.Sprintf`, and so is every
`String` method. `fmt` is already in froth's dependency graph through Bubble Tea and
lipgloss, so hand-assembling these with `strconv` and `+` buys a consumer nothing and
costs the reader the shape of the sentence. Wrapping a long literal across lines with `+`
is not the same thing and stays.

## Agent skills

### Working agreements

How work gets built here: closing tickets, the `gh` traps, running a design session, prior-art
surveys, and what verification means. See `docs/agents/working-agreements.md`.

### Issue tracker

Issues live as GitHub issues on `jbeda/froth`, managed via the `gh` CLI. See `docs/agents/issue-tracker.md`.

### Triage labels

The five canonical triage roles, each label string equal to its name. See `docs/agents/triage-labels.md`.

### Domain docs

Single-context: `CONTEXT.md` at the repo root, ADRs in `docs/adr/`. See `docs/agents/domain.md`.
