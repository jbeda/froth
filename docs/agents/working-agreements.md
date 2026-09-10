# Working agreements

How work actually gets done in this repo, as opposed to what the code is — that is
`CONTEXT.md` and `docs/adr/`. Everything here was learned by getting it wrong once, and the
issue numbers are kept so a claim can be checked rather than taken on trust.

This is the canonical copy. `HANDOFF.md` holds per-machine working state and should not
duplicate any of it.

## Closing tickets

Once work is committed, reviewed, and **pushed to `main`**, close the issue without asking.
This is a narrow carve-out from the standing "confirm before outward-facing actions" rule
and does not generalise to other GitHub state changes.

Order is load-bearing: push, verify it landed (`git log origin/main..HEAD` is empty), then
close. A closed issue pointing at a local-only commit lies to everyone reading the tracker.
**#38 was closed before the push and sat that way for an hour**, its close comment citing a
SHA that existed on one machine — which is the failure the order prevents, and it happened
because closing felt like the end of the work. It is not; the push is.

**Never use both a `Closes #N` trailer and `gh issue close --comment`.** The trailer closes
on push, and `--comment` on an already-closed issue warns and **silently drops the comment**
— #22's verdict was nearly lost that way. House style (#2, #3, #14) is no trailer, plus an
explicit `gh issue close <n> --comment "…"` summarising what landed.

## The `gh` CLI here

**`gh issue view <n> --comments` prints the comments and nothing else, so an issue with
none produces no output at all and exits 0.** That reads exactly like a broken command and
is not one. Ask how many there are before concluding anything:

```
gh api repos/jbeda/froth/issues/<n> --jq .comments
```

Measured across four issues: 0 comments gives 0 bytes, 1 comment gives output. Prefer the
`gh api` form for reads generally, and `--body-file` for anything multi-line — a long
`--comment "$(cat …)"` puts the whole body on the command line.

**This section used to say `gh` needs the sandbox disabled, and that was wrong.** No
sandbox is installed on this machine — no `bwrap`, `sandbox-exec` or `firejail`, and no
`sandbox` key in any settings file — and the same `gh api` call returns byte-identical
output with the flag and without it. So `dangerouslyDisableSandbox` changes nothing about
how `gh` behaves and should not be passed. (Checked on the Linux box; the Mac is
unverified, so check there before assuming rather than re-deriving from this.)

The silent-empty result above is what the claim was built on. The correction is kept here
rather than deleted, so an agent that has met the old advice elsewhere finds the answer.

**Sub-issues** are `gh api -X POST repos/jbeda/froth/issues/<parent>/sub_issues -F
sub_issue_id=<id>`. Two traps: `-F` rather than `-f`, because the API wants a real integer
and `-f` sends a string (422); and `<id>` is the issue's **database id**, from `gh api
.../issues/<n> --jq .id`, not its number.

## How work gets built

Three routes. The third is the usual one for anything with design in it.

**`/mattpocock-skills:implement #N`.** Best single result so far (#13). The skill is
`disable-model-invocation`, so only the maintainer can start it; it commits to the current
branch and does not push. **Settle any real design decision before invoking it** — #13's
rule was settled in a grilling first, and the build followed it instead of inventing as it
went.

**One ticket to a subagent on Opus.** For mechanical or well-fenced work. A subagent cannot
load skills, so the prompt carries everything: the reading order, the central design call
named explicitly (an unattended agent otherwise builds around a wrong guess), the scope
fences (#9 drifted into #12's without them), the comment standard quoted (subagents drift
verbose), and "commit to `main`, do not push, do not close". Verify before believing the
report.

**Design settled in `/mattpocock-skills:grilling`, then built.** The default for anything
with design in it, because the answers reverse the design regularly and it is cheaper to be
reversed before the code exists. **Summarise the settled design for confirmation before
writing a line.**

**Batching.** Small tickets batch fine in one session (#8, #20, #16) — one commit per
ticket, stacked on `main`, reviewed before pushing. Anything with design in it gets its own
run.

## Running a design session

**Ask one question at a time.** Asked for explicitly; long rounds scroll off screen.

**Look facts up rather than reasoning from them.** This has reversed recommendations in #14
and in every M3 design session — five times in one session alone, three from prior-art
surveys and two from the maintainer catching a claim that a two-minute check would have
settled. Concretely: read the code before saying froth withholds a capability, compile a
snippet before asserting what the Go version allows, and check both `CONTEXT.md`'s `_Avoid_`
lists **and the existing identifiers** before proposing any name.

**Do not invent vocabulary mid-session.** #13 coined "ladder"/"rung" and it reached code,
two ADRs, the milestone log, a GitHub comment and a commit message before anyone caught it.
A later session coined "claim" and used it thirty times before noticing `binding.go` already
had a `claim()` for a different concept. If a new term seems needed it goes in `CONTEXT.md`
or it does not get used.

**Vocabulary lands before the ADR; identifiers wait for call sites.** "Propose names last"
is right about identifiers and wrong about vocabulary, and the M4 design session found the
line. A term an ADR uses in prose — the three geometries — has to be settled *before* that
ADR is written, or the ADR becomes the one document using different words and every ticket
downstream coins its own. A name for a type or method — `LayoutFunc`, `SetBindings` —
waits, because the call sites judge it: #30 wrote `SetBindings` variadic before all seven
call sites showed a slice was better. Ask which kind it is before deferring it.

## Prior-art surveys

The highest-value thing a design session can spend tokens on. Every reversal that improved a
decision in M3 came from one.

Run them as subagents, one per axis, against **primary sources** — official docs and actual
source, not recollection — and require citations. **Never treat one system as prior art**: a
Win32-only reading recommended opener-parenting and a nine-system sweep reversed it.

Ask for an honest verdict on what transfers and what does not, and for anything unverifiable
to be flagged rather than asserted. Say explicitly that the agent should *not* design froth's
answer — a survey that arrives with a recommendation is harder to argue with than one that
arrives with facts.

Finished surveys go in `docs/research/`, not in a scratch directory.

**Survey froth's own record too — it contains recall dressed as citation.** The M4 focus-axes
session ran four surveys and three of them contradicted something froth had already written
down as though it were sourced: the `WM_GETDLGCODE` quotes on #1 describe Win32's arrow walk
and were recorded as the tab walk; "the prior art is unanimous" on container focus memory is
wrong in two independent surveys; and Textual #877 is a one-day-old closed report about a CSS
selector, not the focus-border bug that ADR 0009, `chrome.md` and `HANDOFF.md` all cited it as.

None of these came from a survey. Each was written during a session that had the citation
habit for *other* systems and relaxed it for a claim that felt already known. A parked comment
that says "verify before this enters an ADR" is doing its job — #47's did, and the check paid.
Treat an uncited claim in froth's own docs as a hypothesis, especially when a later document
leans on it: by the time #877 was checked it had propagated to three files.

**The same failure has a second origin: an invariant derived in-session and then stated as
general.** #48's ADR amendment claimed *"the only suppressed surface a rising message can
reach is the modal's own parent."* It was derived with focus inside the modal, holds only
there, and is false in the case the amendment existed to add. Nothing was misremembered — the
reasoning was sound for the case it was done on, and the sentence then outgrew it. This is the
harder one to catch, because the author has just proved it to themselves. The review found it;
re-reading did not. **Write which case an invariant was derived from, or check it against the
case you did not have in mind.**

## Verifying

**Review with `/mattpocock-skills:code-review` before pushing, and believe it.** Two Opus
agents, Standards and Spec. On #13, Standards caught an `_Avoid_` term in a field name and
Spec caught a real defect. Verify each finding rather than accepting it.

**Then break each rule and check that a test fails.** Two of #13's rules turned out to be
pinned by nothing. **Count test failures, not build failures** — a mutation that does not
compile scores as "caught" and is not. #39 ignored that sentence and had a mutation score
zero for exactly this reason.

**Mutate towards a plausible wrong implementation, not towards a disabled one.** A mutation
that switches a rule off is the easy one to write and the weakest one to pass: it usually
trips the first test that covers the feature at all, which says nothing about whether the
rule's *shape* is pinned. #48's suppression rule looked pinned under "give the suppressed
declaration no keys" and was not — a declaration with no keys claims none, so that mutation
missed the shadowing bug completely. The mutation that found the gap kept the claim and
dropped the listing, which is what a careless implementation would actually have done. Ask
what the wrong version someone would plausibly write looks like, and mutate to that.

**Restore the mutated files from a copy, never with `git checkout --`.** On #39 the harness
reverted the files under test to `HEAD` between mutations and destroyed three files of
uncommitted work, and its `cp` backup had silently failed for want of its destination
directory. Copy into `.scratch/` first, assert the copy exists, and restore from that — or
commit before mutating.

**Scaffolding tests are fine; shipping them was decided against.** A throwaway `_test.go`
driving froth through the public API and printing the frame is the fastest way to see an
example work. Delete it afterwards.

## File what building teaches, as it happens

Asked for explicitly, and it pays: #22 produced five issues, #14 two, and the M3 design
sessions several more. A problem noticed while doing something else is worth a ticket at the
moment it is noticed, not at the end.

Where it goes matters. An idea with no case yet belongs in the ADR governing its area,
marked `Parked:` with the evidence that would revive it — see `CLAUDE.md`. A defect or a
piece of scope belongs in the tracker.
