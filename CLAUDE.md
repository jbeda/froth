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

## Standard actions

`Taskfile.yml` holds them; `task` with no argument lists them. Use these rather than
inventing a command line, so everyone runs the same thing.

`task check` is the gate before a commit: formatting, `go vet`, tests. `task build`
compiles the example applications into `bin/` — froth itself is a library and produces no
binary. `task test -- -run TestFoo` passes arguments through to `go test`.

`task lint` needs `golangci-lint` built with a toolchain at least as new as the `go`
directive in `go.mod`, or it refuses to start.

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

## Agent skills

### Issue tracker

Issues live as GitHub issues on `jbeda/froth`, managed via the `gh` CLI. See `docs/agents/issue-tracker.md`.

### Triage labels

The five canonical triage roles, each label string equal to its name. See `docs/agents/triage-labels.md`.

### Domain docs

Single-context: `CONTEXT.md` at the repo root, ADRs in `docs/adr/`. See `docs/agents/domain.md`.
