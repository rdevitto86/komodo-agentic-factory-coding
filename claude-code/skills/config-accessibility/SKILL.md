---
name: config-accessibility
description: ADHD-calibrated output formatting. Load before authoring any document, report, plan, audit, summary, README, or PR body — anything longer than one screen. Covers chunking, front-loaded bolding, emoji protocol, table shape, code-answer structure, and document typography.
---

# ADHD Format

**Every rule here exists to cut extraneous cognitive load.** The reader has ADHD: working memory is the bottleneck, not comprehension. Dense text does not read slower — it fails to read at all.

Grounded in Cognitive Load Theory (Sweller, 1988), Dual-Coding Theory (Paivio, 1986), Gestalt proximity, and WCAG 2.2 cognitive guidelines.

`AGENTS.md` § 2 carries the eight always-on rules. This skill is the depth behind them plus everything that only applies to authored documents.

---

## 1. Learning mode

**Hands-on and visual over textual and conceptual.** The user learns by doing and seeing, not by reading a description of the mechanism.

- **Show, don't tell.** A runnable snippet, a rendered diagram, or a before/after outranks a prose walkthrough of the same thing.
- **Concrete over abstract.** State the rule in one line, then show it working on a real example — never leave a concept to stand alone.
- **Prefer a demo to a description.** If a feature can be run, run it. If a mechanism can be diagrammed, diagram it (`artifact-diagramming`, `dataviz`).

---

## 2. Structure — the reader must never hold a map

**Two heading levels, three at absolute most.** `##` for major sections, `###` for sub-topics. Nested outlines (`1.1.2.a`) force the reader to maintain a mental position in a hierarchy, which is exactly the resource they lack.

**Horizontal rules mark context boundaries.** A `---` between major topics tells executive function it may discard the previous context. Without it, everything stays resident.

**Front-load every section.** The first sentence under a heading states the conclusion. Detail, caveats, and rationale come after. Never build to a point.

**Bold the first 1–3 words of every bullet.** The ADHD scan pattern is layer-cake, not F-shaped: the eye jumps between bold anchors and only commits to full sentences after semantic triage. A bullet with no bold lead-in is invisible on the first pass.

---

## 3. Turn-end change summary

**Use this exact three-bucket schema whenever a turn changed something and the turn is ending.** No other heading text substitutes for it — a status sentence never replaces the literal heading.

| Bucket | Heading | Contents |
|---|---|---|
| Landed and verified | `## ✅ Successful Changes` | Merged code, passing tests, a completed task |
| Attempted, could not complete | `## ❌ Blocked Changes` | A hard stop, an error, a missing input |
| Landed but needs attention | `## ⚠️ Flagged Changes` | A workaround, a risk, a follow-up |

- **Fixed heading text, always.** The heading is always the literal label above — never a summary sentence standing in for it.
- **Omit an empty bucket entirely.** Never print a bucket with nothing under it.
- **The verdict goes inside the bucket, not the heading.** Put it as the first bullet, table row, or list item under the heading.
- **Order is fixed:** ✅ before ❌ before ⚠️, whichever subset is present.
- **Contents are tables, bullets, or numbered lists** — never a paragraph standing in for structure.

**❌ Chaotic** (freeform per-heading verdicts, no fixed bucket)

> - 📌 Loop is untouched, skill is typed-only
> - ⚠️ Two contradictions the guard change created — both fixed
> - 📌 What's still not built
> - ✅ Scoped — 185/185, verify green

**✅ Fixed schema**

> ## ✅ Successful Changes
> - **Guard change** — two contradictions found and fixed
> - **Scope** — 185/185, verify green
>
> ## ⚠️ Flagged Changes
> - **Loop** — untouched; skill stays typed-only
>
> ## ❌ Blocked Changes
> - **Not built** — `<item>`, waiting on `<input>`

---

## 4. Density caps

| Unit | Hard cap |
|---|---|
| Paragraph | 3 sentences / 40 words |
| List | 5 bullets |
| Table | 3 cols × 6 rows |
| Table cell | 40 chars, one line |
| Options offered | 3 |

**Over a cap means restructure, not shrink.** A 9-bullet list becomes three groups of three under bold sub-headings. A 5-column table becomes a 3-column table plus bullets underneath.

**Decide, never enumerate.** "I'd change 4 of the 15 — say no to keep them", not "which of these 15?". Cap any option list at 3.

**Numbered lists only for sequential dependency.** If order does not matter, bullets — numbers imply a sequence the reader will try to hold.

---

## 5. Emoji protocol

**Emoji are functional category markers, never decoration.** A relevant icon is pre-attentive: it routes attention before reading. An irrelevant one is foveal noise that costs a fixation.

- **Placement:** start of a `##` heading or an alert callout. Nowhere else.
- **Ceiling:** one per heading. Never two.
- **Never mid-sentence.** Substituting an emoji for a word forces a decode-and-resume, which is strictly worse than the word.
- **Never in body text, tables, or bullets.**

| Class | Set |
|---|---|
| Status | ⚠️ ❌ ✅ 🔴 |
| Action | 📌 🚀 🔧 |
| Concept | 💡 📊 ⚙️ 🔒 |

---

## 6. Prose tone

**Imperative, active voice.** "Check log files daily", never "it is recommended that log files should be checked". Passive voice adds a clause the reader must unwind to find the actor.

**Zero conversational scaffolding.** Cut "In order to understand this, it's important to remember that…". Jump to the mechanism.

**Concrete quantities always.** "3 files", "40ms", "20 minutes", "2 of 15". Never "a bit", "several", "some work" — vague magnitude forces the reader to invent a number and hold it.

**No hedging on findings.** State it, or state that you could not determine it. "It may possibly be the case that" is a full clause carrying no information.

**Judge familiarity by what the user has already used correctly**, never by assumed seniority. A term they have not been given in this conversation gets one plain clause of definition before its first use, or it gets cut. Seniority predicts nothing about whether they have seen *this* mechanism.

**An error states cause and fix, nothing else.** No apology, no restatement of what was attempted, no speculation about what else might be wrong.

**Never attach an unrequested caution.** A warning nobody asked for reads as hedging and costs the reader a paragraph to discard.

**Disagreement is one sentence with evidence** — no hedge, and no apology folded into it. Then do it their way and never re-argue.

---

## 7. Code answers

**Order is fixed:** what it does → the code → why it matters. Never prose-then-code-then-explanation.

```
## Fix: <one-line statement of the change>

<one sentence: what this achieves>

<the smallest snippet that shows the change>

**Why:** <1–2 bullets, front-loaded bold>
```

- **Show the diff, not the file.** Only the changed block plus the minimum surrounding context to place it. Never dump an unchanged file.
- **Never annotate the code with comments.** The `rules-commenting` skill overrides every "add explanatory comments" instinct, including the one in most formatting guides. Explanation goes in the `**Why:**` block underneath, outside the code fence.
- **Language-tag every fence.** Untagged fences lose syntax colour, which is a free pre-attentive channel.

---

## 8. Document typography

Applies when the output is a rendered document — HTML, an artifact, a README, a slide — not terminal text.

- **Line length 50–75 characters** (≈600–700px). Wide measure multiplies saccades and accelerates fatigue.
- **Line height 1.5×** body size, minimum.
- **Left-align everything.** Justified text opens rivers of white space that break vertical tracking; centred text moves the line-start target every line.
- **Never pure black on pure white.** Use `#1E1E1E`–`#222222` on `#F8F9FA`–`#F4F4F0`, or `#E8E8E8` on `#121212` for dark. `#000` on `#FFF` glares.
- **High-legibility sans-serif** — Lexend, Atkinson Hyperlegible, Inter, system sans.
- **Space above a heading is 1.5–2× the space below it.** Gestalt proximity then binds the heading to its own section rather than the one above.

---

## 9. Self-check

Run this before sending anything longer than five lines.

- [ ] **Line 1 is the verdict**, not context or preamble
- [ ] **Every bullet opens bold**
- [ ] **No paragraph exceeds 3 sentences**
- [ ] **No table exceeds 3×6**, no cell wraps
- [ ] **`---` separates each major topic shift**
- [ ] **Options capped at 3**, with a stated recommendation
- [ ] **One open question**, maximum
- [ ] **Zero emoji outside `##` headings**
- [ ] **A turn-end change summary uses the fixed ✅/❌/⚠️ buckets**, never a freeform heading

---

## 10. Worked example

**❌ Uncalibrated**

> The authentication service handles sign-in using OAuth 2.0 with JWT tokens. When a request arrives, the API gateway verifies the bearer token against the public key stored in Redis. If the token is valid, the request proceeds downstream with user claims attached in the headers. If expired, the gateway returns 401, triggering the client to call /auth/refresh using an HTTP-only cookie.

**✅ Calibrated**

> ## 🔒 Auth token flow
>
> **Gateway validates every request against a Redis-cached public key; expiry returns 401 and the client refreshes.**
>
> | Attribute | Value |
> |---|---|
> | Protocol | OAuth 2.0 + JWT |
> | Validation | API gateway, Redis PKI |
> | Failure | HTTP 401 |
>
> 1. **Ingest** — gateway receives `Authorization: Bearer <JWT>`.
> 2. **Verify** — signature checked against cached public key.
> 3. **Route** — valid requests forward with claims in headers.
> 4. **Refresh** — 401 sends the client to `/auth/refresh`.
>
> **⚠️ Never retry a 401 before refreshing** — it cascades into an auth loop.
