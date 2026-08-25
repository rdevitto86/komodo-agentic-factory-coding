---
name: generate-pr-description
description: Fill the repo's PR template from the real diff — title, every declared section, and the reconciliation that keeps scope honest.
argument-hint: <the stories this PR closes> [--draft]
disable-model-invocation: true
---

# PR description

Closing: **$ARGUMENTS**

**Inside a git repository, on a non-default branch** — if either isn't true, say so and stop.

**It is output, not an action.** You return the title and body; the user opens the PR, applies the risk label, and posts the audit comment. **Typed only** — no phase of `/workflow-loop` reaches this skill, and none should: publishing is a deliberate act, not a loop step.

## The template on disk is the authority

**Read `.github/PULL_REQUEST_TEMPLATE.md` and fill exactly the sections it declares.** Never add one it doesn't carry, never drop one it does. **If it is absent, say so and stop** — scaffolding it is `generate-repo`'s job, and a body invented here would diverge from every human-authored PR in the repo.

The rules below govern *how* a section is filled. The template governs *which* sections exist, so it can change without this skill changing.

## Read the diff first

`git diff <base>...HEAD` and `git log <base>..HEAD`. **Every line you write comes from what actually changed**, never from the task list, the plan, or the conversation. A story that was attempted and abandoned is not a change.

## Title

**Owned by `generate-commit-message`** — same `<type>: <summary>` taxonomy, same 72-character cap, same imperative mood. Load it rather than restating the rules; a PR title and a squash-commit subject are the same string.

## Filling the sections

- **Summary** — one or two sentences on what the PR is for. Never a file list, never a restatement of the title.
- **Changes** — one bullet per area, never per file. A mechanical rename across ten files is one bullet; a hand-written function is its own. Append the PRD requirement ID where the closing story carried one.
- **Validation Evidence** — only what a green pipeline cannot show: a live-dependency happy path, a call against a deployed endpoint, a behaviour with no automated coverage. **Never paste unit, component, or contract output** — the merge gate already runs those. "Covered by CI" is a complete answer, and a failure is stated plainly rather than omitted.
- **A field with no answer gets its stated default**, not a blank and not "N/A". Where the template supplies one inline, use it verbatim.

## Release path — only when the template declares it

**A repo that deploys nothing carries no release section, and that is not a gap.** Global config, shared tooling, and documentation repos have no environment to name; never add the section back, and never note its absence as a finding.

**Where the template does declare it, read the repo's build config for the environments and regions it actually lists** — `standards-cicd` names the canonical path. Tick only what this change reaches, and only regions that config declares.

**Leave a box unticked when you cannot determine it from the repo**, and say which one in your return. A guessed environment is worse than an unticked one: it reads as a deployment decision nobody made.

**Blue/green is unconditional and is never a rollout answer.** The rollout field carries the flag-service shape — canary, A/B, flagged off — or the default. `standards-cicd` owns why.

## Reconciliation — the step that stops drift

**Every file in the diff must belong to a bullet you wrote.** Walk `git diff --name-only <base>...HEAD` against your own Changes list before returning. Anything unaccounted for is one of two findings, and both go in your return rather than being quietly folded in:

- **Scope that rode along** — a file changed outside the stories named in `$ARGUMENTS`.
- **A bullet with no diff behind it** — a change you described that never landed.

**Never widen a bullet to swallow an unexplained file.** That is the understatement this step exists to catch.

## Return

The title on one line, then the filled template body in a fenced block, then any reconciliation finding or unticked box beneath it. **No trailers** — no co-author line, no generated-by line, no issue footer unless the user asked for one.

**`--draft` changes nothing about the body.** It is a flag for the caller, which opens a stacked PR as a draft until its parent merges.
