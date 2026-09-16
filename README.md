# komodo-agentic-toolkit-coding

A software assembly line for Komodo repos. Feed it a task group from `BACKLOG.md`, and it plans waves, builds them in parallel worktrees, verifies, reviews once, and opens a pull request. A human merges.

Three ideas hold it together:

1. **The pipeline is code.** A stdlib-only Python orchestrator owns the state machine. Models are workers that receive one brief and return JSON. No model reads the queue or decides the order.
2. **Guarantees are structural.** Workers hold no push credential. One module pushes, and it refuses protected refs in code. Git hooks refuse the same on the developer's machine.
3. **Context is per worker.** A builder sees its task, its files, the standard for its language, and its acceptance commands. Nothing else. Fixed prompt text stays under 1.5k tokens; the orchestrator itself spends none.

## Setup

Requirements: Python 3.9+, git, the `claude` CLI on PATH, and `gh` authenticated. Ollama is optional.

```bash
git clone <this repo> ~/komodo/ai/komodo-agentic-toolkit-coding
cd ~/komodo/ai/komodo-agentic-toolkit-coding
python3 -m komodo install                    # copies claude-code/ into ~/.claude, generates settings.json
python3 -m komodo hooks install ~/komodo/*/* # points each repo's git hooks at komodo/hooks
```

Windows: same commands with `python` or `py -3`. No Git Bash, no symlinks, no `make`. See [docs/windows-install.md](docs/windows-install.md).

The install is a copy. After editing `claude-code/` or `komodo/standards/`, run `python3 -m komodo install` again.

## Usage

In any repo with a `BACKLOG.md` in the [task grammar](claude-code/skills/backlog/SKILL.md):

```bash
python3 -m komodo run --dry-run              # plan: waves, briefs, token estimates, no spend
python3 -m komodo run TG-01.2                # run one group, fast profile
python3 -m komodo run TG-01.2 --profile thinking
python3 -m komodo status --prune             # runs, stale worktrees, merged branches
python3 -m komodo pr respond                 # answer unresolved review threads
python3 -m komodo pr sync                    # merge the base in, resolve conflicts with a worker
```

A run reports to `.komodo/runs/<id>/report.md` and into the PR body: what landed, what blocked, per-phase time, per-role cost.

## The pipeline

```mermaid
flowchart LR
    A[Preflight<br/>lint backlog, build DAG,<br/>check tree, 0 tokens] --> B[Branch<br/>type/group-slug]
    B --> C[Build waves<br/>one builder per task,<br/>one worktree each,<br/>parallel by directory]
    C --> D{done_when<br/>rerun by<br/>orchestrator}
    D -->|pass| E[Commit + merge<br/>into run branch]
    D -->|fail| F[One repair pass] --> D
    E --> G[Compile gate<br/>per wave, 0 tokens]
    G --> C
    G -->|last wave| H[Verify once<br/>repo gate]
    H --> I[Review once<br/>bugs, security,<br/>tests, comments]
    I -->|floor findings| F2[Repair] --> H
    I -->|below floor| J[File to BACKLOG.md]
    J --> K[Changelog + statuses<br/>commit, push, PR]
    K --> L([Human merges])
```

| Phase | Runs as | Model calls |
|---|---|---|
| Preflight, branch, DAG | code | 0 |
| Build | one builder per task, parallel by directory | N |
| Verify, compile gates | code | 0, +1 repair on failure |
| Review | one reviewer over the group diff | 1 |
| Publish, report | code and `gh` | 0 |

Two profiles, set in `komodo.json` or `.komodo/local.json`, drive model and effort per role:

| Profile | Planner | Builder | Reviewer | For |
|---|---|---|---|---|
| `fast` | Sonnet medium | Sonnet medium, $2 cap | Sonnet medium, skipped under 150 diff lines | Pro plans, small groups |
| `thinking` | Opus high | Sonnet high | Opus high | Max plans, hard groups |

## Enforcement layers

```mermaid
flowchart TB
    subgraph Worker["Worker process (claude -p)"]
        W[Edits files in its worktree<br/>no GH_TOKEN, credential.helper empty,<br/>SSH batch mode, gh unauthenticated]
    end
    subgraph Orchestrator["Orchestrator (komodo/gitops.py)"]
        O[Only pusher.<br/>Refuses protected refs, force,<br/>amend, trailers, in code]
    end
    subgraph Machine["Developer machine"]
        H1[pre-commit: no protected branch,<br/>no trailer, gofmt, comment lint]
        H2[pre-push: no protected ref,<br/>no force, repo verify gate]
        G[guard.py: advisory PreToolUse<br/>in interactive sessions]
    end
    subgraph GitHub
        M([Merge button, human])
    end
    W -->|commits via| O -->|push branch| GitHub
    H1 & H2 & G -.->|same rules by hand| GitHub
```

The remote is the only place a change lands into `main`, and only a person presses that button.

## Layout

```
komodo/                the orchestrator (stdlib only)
├── __main__.py        CLI: run, status, tasks, comments, hooks, install, doctor, pr, release
├── pipeline.py        the phases
├── gitops.py          the only git writer
├── workers/           claude (headless CLI) and ollama adapters
├── briefs/            system + prompt template per role
├── standards/         rule files per language and domain, injected by extension
└── hooks/             pre-commit.py, pre-push.py, and the sh stubs Git runs
claude-code/           thin Claude Code adapter, installed by copy
├── AGENTS.md          ~40 lines: working, git, comments, writing for a human
├── agents/            builder, reviewer, planner, researcher, scout, architect, tester
├── skills/            komodo, backlog, review, and one thin pointer per standard
├── hooks/             guard.py (advisory), context_injector.py
└── settings.policy.json  permissions, hooks, skillOverrides; merged into ~/.claude/settings.json
tests/                 unittest suites
scripts/verify.py      the gate this repo runs
templates/project/     AGENTS.md, BACKLOG.md, CHANGELOG.md, README.md starters for a new repo
```

## Comments

Every public function gets a one-line doc comment. A private function gets one only when long or non-obvious. Nothing else is commented unless the line cannot say it itself. The lint (`python3 -m komodo comments check`) flags a missing doc, a comment that restates its identifier, cites a version or ticket, uses first person or a hedge, narrates history, or runs past twenty words. It runs in `pre-commit` on staged lines and in the reviewer brief as a judgment class.

## Testing

```bash
python3 scripts/verify.py      # tests, validate, comment lint, doctor
```

CI runs the same command on Ubuntu, macOS, and Windows, plus Python 3.9 on Ubuntu.

## References

- [docs/architecture.md](docs/architecture.md): phases, state, briefs, and what would change each decision
- [docs/design-decisions.md](docs/design-decisions.md): why each rule exists
- [claude-code/skills/backlog/SKILL.md](claude-code/skills/backlog/SKILL.md): the task grammar
- [SECURITY.md](SECURITY.md): what installing this grants, and how to report a problem
