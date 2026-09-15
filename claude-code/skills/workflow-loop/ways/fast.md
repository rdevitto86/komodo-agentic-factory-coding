# Way of working — Fast path

**`/workflow-loop fast <task>` skips P1 decompose, P2.0 align, and P3 consolidate.** P2.1, P2.2, P2.3, and P2.4 run unchanged — fast means less planning, never less verification or review.

Load this alongside `sdlc.md` or `debugging.md`, never instead of one — those own the gates the surviving phases still run.

---

## Entry is checked, not judged

Run both against the change's own diff — before P2.1 on the paths the task names, and again before P2.2's commit:

```bash
git diff --shortstat     # ≤ 1 file changed and ≤ 10 lines changed
git diff --name-only     # no path on the exclusion list below
```

**Exclusion list — any match refuses the fast path regardless of diff size.** Match `git diff --name-only` against these paths, no judgement required:

- **Agent config** — `**/hooks/**`, `**/agents/**`, `settings.json`
- **The verify gate and every script it runs** — `.claude/verify.sh`, `Makefile`, `Taskfile.yml`, `justfile`, `scripts/**`
- **Git-hook dispatchers and installers** — `**/pre-commit*`, `**/pre-push*`, `install.sh`, `install.py`
- **Always-loaded directives and ownership** — `**/AGENTS.md`, `**/CLAUDE.md`, `CODEOWNERS`

A change able to weaken its own verification is not a fast path — the gate's own scripts are the check, so editing one and then running it proves nothing. **The list is a floor, not a ceiling:** a path it misses that you nonetheless read as enforcement also refuses. That judgement may only refuse, never admit — erring into the full machine costs time and nothing else.

**Either check failing sends the change back to P1** — the full machine, from the top.
