---
name: config-accessibility
description: ADHD-calibrated document formatting — turn-end change-summary schema, emoji protocol, density caps, code-answer structure, typography for rendered docs. Load before authoring anything longer than a screen. Everyday-turn rules (BLUF, prose caps, concrete numbers) live in CLAUDE.local.md.
---

# ADHD Format — long-document rules

Adds to CLAUDE.local.md's always-on rules: the turn-end summary schema, emoji protocol, density caps, code-answer order, and typography — specific to documents longer than a screen.

---

## 1. Structure

- **Two heading levels, three at most.** `##` major, `###` sub — deeper nesting forces a hierarchy the reader can't hold.
- **`---` marks a context boundary** — tells executive function it may discard the prior section.
- **Bold the first 1–3 words of every bullet.** The scan pattern is layer-cake: the eye jumps bold anchors first.
- **Show, don't tell.** A runnable snippet, diagram, or before/after beats a prose walkthrough (`artifact-diagramming`, `dataviz`).

---

## 2. Turn-end change summary

Use this exact three-bucket schema whenever a turn changed something and is ending. No status sentence substitutes for the heading.

| Bucket | Heading | Contents |
|---|---|---|
| Landed, verified | `## ✅ Successful Changes` | Merged code, passing tests |
| Attempted, blocked | `## ❌ Blocked Changes` | Hard stop, error, missing input |
| Needs attention | `## ⚠️ Flagged Changes` | Workaround, risk, follow-up |

- **Fixed heading text, always.** Omit an empty bucket. Order: ✅ then ❌ then ⚠️.
- **Verdict goes inside the bucket**, never the heading.

---

## 3. Density caps

| Unit | Hard cap |
|---|---|
| List | 5 bullets |
| Table | 3 cols × 6 rows |
| Table cell | 40 chars, one line |
| Options offered | 3 |

**Over a cap means restructure, not shrink.** Numbered lists only for sequential dependency.

---

## 4. Emoji protocol

- **Placement:** start of a `##` heading or alert callout only. Ceiling one per heading. Never mid-sentence, body text, tables, or bullets.

| Class | Set |
|---|---|
| Status | ⚠️ ❌ ✅ 🔴 |
| Action | 📌 🚀 🔧 |
| Concept | 💡 📊 ⚙️ 🔒 |

---

## 5. Code answers

Order fixed: what it does → the code → why it matters.

```
## Fix: <one-line statement>
<one sentence: what this achieves>
<smallest snippet showing the change>
**Why:** <1–2 bullets>
```

- **Show the diff, not the file.** Never annotate code with comments — explanation goes in `**Why:**`.
- **Language-tag every fence.**

---

## 6. Document typography

Applies to rendered output — HTML, artifact, README, slide — not terminal text.

- **Line length 50–75 chars, line height 1.5×**, left-aligned.
- **Never pure black on white.** `#1E1E1E`–`#222` on `#F8F9FA`–`#F4F4F0` light; `#E8E8E8` on `#121212` dark.
- **High-legibility sans-serif** — Lexend, Atkinson Hyperlegible, Inter, system sans.

---

## 7. Self-check

- [ ] Heading levels ≤ 3, `---` at each topic shift
- [ ] Turn-end summary uses fixed ✅/❌/⚠️ buckets, none freeform
- [ ] Density caps respected; emoji only in headings, ≤1 each
