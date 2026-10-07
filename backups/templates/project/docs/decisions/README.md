# Decisions — <Repo Name>

Why the system is this way and not another. One file per decision, named `NNNN-<slug>.md` from its title, numbered in order and never rewritten. A superseded entry keeps its text and gains a status line naming its successor. An open technical question is a Proposed entry; a product question belongs in `prd.md`. `komodo lint` refuses two files with one number.

A new entry copies this shape into the next free number, such as `0001-<slug>.md`:

```markdown
# 0001. <What was decided, as a sentence>

**Status:** Proposed, {{DATE}}.

**Context.** <What forces the decision.>

**Decision.** <What was chosen.>

**Alternatives.** <What was rejected, and why.>

**Consequences.** <What follows, good and bad.>
```
