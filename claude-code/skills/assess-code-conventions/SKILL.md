---
name: assess-code-conventions
description: Read the changed code for judgment-call style/formatting violations a linter or formatter cannot mechanically decide — wrap-style choice, magic-number extraction placement/casing/grouping, guard-clause blank lines, single-call-site inline-vs-closure. Model-agnostic; never re-flags anything golangci-lint/gofmt/the repo's own formatter already gates.
argument-hint: <task text or band summary> [standards-* skills that apply]
context: fork
agent: reviewer
background: false
disable-model-invocation: true
---

# Code conventions assessment

Reviewing: **$ARGUMENTS**

Model-agnostic style pass — Read/Grep/Glob/Bash only, no host-specific tooling. Findings only, never fixes — same contract as `assess-bugs`/`assess-simplify`, just a different lens.

**Narrower than it sounds.** `/assess-code-quality` scores the diff's overall convention conformance (structure, naming, SDK reuse, performance) as one aggregate tier; `/assess-simplify` hunts reuse, duplication, and efficiency. This skill covers neither — it exists only for the style calls a `standards-<lang>` skill states as a rule but that no lint/format gate can mechanically enforce: which wrap step a long statement lands on, where a magic-number extraction belongs and how it's cased and grouped, blank lines around guard clauses, whether a single-call-site helper should be inline or a closure. If `golangci-lint`/`gofmt`/the repo's own formatter already catches it, it is out of scope here by definition — re-flagging a gated finding just duplicates a check that already runs on every commit.

## Process

1. **Load the `standards-<lang>` skill(s) named in `$ARGUMENTS`** and read their Conventions section — the judgment calls this skill checks are documented there, not invented per-review.
2. **Confirm the repo's lint/format gate is clean first** — run whatever `AGENTS.md` or the `standards-<lang>` skill names as the formatter/linter command. If it fails, stop and say so: a dirty gate means the mechanical layer hasn't run yet, and everything it would catch is noise here.
3. **Read `git diff` for the band.**
4. **For each changed block, check only the judgment calls the gate can't decide**: the wrap-style step chosen once a statement already needs wrapping, a magic number/string left inline or extracted to the wrong scope/casing/grouping, a guard-clause blank-line placement, a single-call-site function that should be a closure or vice versa.
5. **Skip anything a formatter or linter would flag or auto-fix** — that's already handled before this skill runs, and it is never this skill's finding to make twice.

## Report

| Sev | Where | What | Convention |
|---|---|---|---|
| L | `handler.go:84` | one-arg-per-line wrap when the grouped form fits the ~90-col soft threshold | `standards-go` wrap-style step |

**Diagram form**: a Mermaid `graph LR` — one node per drifting file, grouped under the convention each one breaks. Draw it when the table runs past 5 rows or one drift was copied from another; below both, the table alone is complete.

**Sev**: Medium (a judgment call that will mislead the next person copying the pattern) · Low (a single, contained drift with no compounding risk).

No findings: state that plainly, one line, and stop. **Never invent a finding to have something to report** — every row needs a `standards-<lang>` convention it violates, cited by name, not just a stylistic preference.

The caller files these rows to `BACKLOG.md`; you never write.
