---
name: repo-assess
description: Full-repo health assessment across every assess-* dimension — bugs, security, vulnerabilities, dependencies, performance, code quality, simplification, test coverage, and overall readiness — invoked as one command and rolled into a 1-100 score per category plus a composite total. Each sub-assessment files its own BACKLOG.md stories. Pass --report to skip every write.
argument-hint: [optional: path to scope the sweep] [--report]
disable-model-invocation: true
---

# Repo assessment

Scope: **$ARGUMENTS** (default: the whole tracked tree)

A periodic full health check, not a per-task gate — run it deliberately, the way you'd run `/assess-readiness` before a release, never per-edit. It invokes nine existing `assess-*` skills in sequence, each doing a genuine full-repo pass, and rolls their native severity tables and tier scores into one comparable 1-100 number per category. This skill adds no new review logic of its own — every finding traces back to the sub-skill that produced it — it only orchestrates and scores.

**Distinct from `/assess-readiness`.** Readiness answers one narrow question — a GO/NO-GO verdict against a stated mission brief, gated on four Blocker kinds. This skill answers a broader one — where does the repo stand across every dimension, scored, so it invokes `assess-readiness` itself as one of its nine inputs (using a generic brief when `$ARGUMENTS` names no decision) rather than duplicating its blocker logic.

**Excludes `/assess-change-risk` on purpose.** That skill scores the blast radius of *a change* — with no active band or diff to speak of, there is nothing for it to score. Every other `assess-*` skill either already operates at repo scope, or can be pointed at the whole tree without reinterpreting what it does (see below).

## Process

1. **Detect whether `$ARGUMENTS` narrows the sweep to a path.** Strip any `--report` token first; whatever remains is the scope note passed to every sub-invocation below. Empty means the whole tracked tree.
2. **Decide whether `--report` propagates.** If `$ARGUMENTS` carries `--report`, append it to every sub-invocation below and skip step 5 (this skill also writes nothing). Otherwise each sub-skill files under its own existing contract — this skill never files a story itself, only aggregates and scores.
3. **Give the four diff-scoped sub-skills the whole tree as their "diff."** They each read `git diff` for "the band" by design — point that band at git's empty-tree hash so every tracked file is in scope, without touching their own contracts:
   `git diff 4b825dc642cb6eb9a060e54bf8d69288fbee4904 HEAD`
   Pass this as context in each invocation's `$ARGUMENTS`, e.g. `"Full-repository sweep, no active band — diff base is the empty tree (git diff 4b825dc642cb6eb9a060e54bf8d69288fbee4904 HEAD), every tracked file is in scope. <scope note>"`.
4. **Invoke each of the nine, once, in this order** — capture its table/tier and let it file (or not) under its own rules:
   | # | Skill | Scope given |
   |---|---|---|
   | 1 | `assess-readiness` | Native full-repo. Brief: `$ARGUMENTS` if it reads as a mission brief, else `"General engineering health — no specific decision, evaluate current state."` |
   | 2 | `assess-bugs` | Empty-tree diff (step 3) |
   | 3 | `assess-security` | Empty-tree diff (step 3) |
   | 4 | `assess-vulnerabilities` | Native — manifest/CVE scan, not diff-scoped |
   | 5 | `assess-dependencies` | Native — manifest scan, not diff-scoped |
   | 6 | `assess-performance` | Empty-tree diff (step 3) |
   | 7 | `assess-simplify` | Empty-tree diff (step 3) |
   | 8 | `assess-code-quality` | Empty-tree diff (step 3) |
   | 9 | `assess-testing` | Native — default full-suite scope |
5. **File nothing centrally.** Each invocation above already filed its own stories (unless `--report` propagated in step 2) under its own Findings → backlog rules. This skill's only write is the report below, printed to the conversation, never to disk.
6. **Convert every result to a 1-100 score** using the table below, then emit the consolidated report.

## Scoring

**Severity-table categories** (`assess-bugs`, `assess-security`, `assess-vulnerabilities`, `assess-dependencies`, `assess-simplify`, `assess-testing`, and `assess-readiness`'s `📋 Findings` rows) — start at 100, subtract per finding, floor at 0:

| Sev | Deduction |
|---|---|
| Critical | −35 |
| High | −18 |
| Medium | −8 |
| Low | −3 |

`assess-readiness` additionally caps its category score at **15** if the verdict is `NO-GO` — a Blocker outweighs whatever the arithmetic alone would give.

**Tier-scale categories** (`assess-performance`, `assess-code-quality`) — the reported tier maps directly, no arithmetic:

| Tier | Score |
|---|---|
| Low | 97 |
| Low-Med | 85 |
| Med | 68 |
| Med-High | 48 |
| High | 28 |
| Critical | 8 |

**Composite** is the unweighted average of all nine category scores, rounded to the nearest integer. No category is weighted above another — a repo that's clean everywhere but one Critical dimension should read as exactly that, not smoothed over.

## Output

```markdown
## Repo assessment: <composite>/100

| Category | Score | Headline |
|---|---|---|
| Readiness | NN | GO/NO-GO — <deciding reason> |
| Bugs | NN | <count by sev, or "clear"> |
| Security | NN | <count by sev, or "clear"> |
| Vulnerabilities | NN | <count by sev, or "clear"> |
| Dependencies | NN | <count by sev, or "clear"> |
| Performance | NN | <tier> — <driving factor> |
| Simplification | NN | <count by sev, or "clear"> |
| Code quality | NN | <tier> — <driving factor> |
| Test coverage | NN | <count by sev, or "clear"> |

**Composite: <NN>/100**
```

- Each row's headline is one line — the sub-skill already printed its full table above this summary; don't repeat it here.
- **A clear pass still gets a row** — 100 with "clear", not an omission. An unbounded report where silence means "fine" is not an assessment.
- If `--report` was in `$ARGUMENTS`, add one line under the table: `Nothing written — --report was set.` Otherwise: `Findings above are filed as BACKLOG.md stories by each sub-assessment.`
