# Delegating

Who to send work to, and what every brief must carry. **Read this once per run, at P2.0** — the phases above reference it rather than restating it, and the brief-slot table below is the authority every invocation site defers to.

---

## Who

| Need | Send to |
|---|---|
| Read-only research across many files | `researcher` |
| "Where is X" — a path list | `scout` |
| A design question with more than one answer | `architect` — weighed options, never a decision |
| Tests against an existing interface | `tester` — test paths only, and that is prose, not a lock |
| Grading work this session produced | Fresh subagent — **never `subagent_type: fork`.** A `context: fork` *skill* (`/assess-bugs`, `/assess-security`, `/assess-simplify`) runs `reviewer`, not this session — how P2.3 grades the diff. |

**Parallel writers need `isolation: "worktree"`** — `builder` writes tests too, so `tester` beside it is two writers. P2.0 owns the manifest proof that gates it.

**Set `model`/`effort` in the delegate's own frontmatter.** `opus`/`high` for architecture and hard debugging, `sonnet`/`medium` for research and routine code, `haiku`/`low` for path lookup.

---

## What every brief carries

**Brief every delegate with every slot its role requires** — never "see above":

| Role | Required slots |
|---|---|
| `builder` · `tester` | `Task` `Files` `Context` `Done when` `Out of scope` |
| `reviewer` | `Task` `Files` `Context` `Round` `Standards` `Out of scope` |
| `pm` · `architect` · `researcher` | `Task` `Context` `Out of scope` |
| `scout` | `Task` |

**An absent slot and a present-but-empty one are the same thing** — the fork stops and returns the gap instead of doing the work. Delete an optional slot you have nothing for rather than leaving it blank. Ask for the verdict, not the transcript.

The per-role templates under `templates/briefs/` are the fuller authoring reference, available when working in the toolkit repo itself; this file is what a session in any other repo has.
