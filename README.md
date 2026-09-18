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
python3 -m komodo install                    # renders the claude adapter into ~/.claude, generates settings.json
python3 -m komodo hooks install ~/komodo/*/* # points each repo's git hooks at komodo/hooks
```

Windows: same commands with `python` or `py -3`. No Git Bash, no symlinks, no `make`. See [docs/windows-install.md](docs/windows-install.md).

The install is a copy. After editing `komodo/rules`, `roles`, `standards`, or `adapters`, run `python3 -m komodo install` again.

## Usage

In any repo with a `BACKLOG.md` in the [task grammar](komodo/rules/backlog.md):

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

Roles declare a tier (`light`, `standard`, `heavy`). Profiles map tiers to a provider, model, and effort. Three ship as defaults in `komodo/config.py`; `komodo.json` and `.komodo/local.json` override them:

| Profile | light | standard | heavy | For |
|---|---|---|---|---|
| `fast` | Haiku low | Sonnet medium, $2 cap | Sonnet medium; review skipped under 150 diff lines | Pro plans, small groups |
| `thinking` | Haiku low | Sonnet high | Opus high | Max plans, hard groups |
| `local` | Ollama | Ollama | Ollama | Air-gapped use; summarize and review today, build once a tool-capable local runtime exists |

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
        H1[komodo-hooks precommit: no protected branch,<br/>no trailer, gofmt, comment lint]
        H2[komodo-hooks prepush: no protected ref,<br/>no force, repo verify gate]
        G[komodo-hooks guard: advisory PreToolUse<br/>in interactive sessions]
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
komodo/                the library and the orchestrator (stdlib only)
├── rules/             AGENTS.md (universal rules), backlog.md (grammar), cli.md (commands)
├── roles/             one file per role: tier, access, and the body every renderer uses
├── standards/         rule files per language and domain, injected by extension
├── briefs/            worker prompt template per role
├── adapters/claude/   renders ~/.claude from the above; owns its hooks and settings policy
│   └── hooks/         guard.py and context_injector.py, the Python fallbacks
├── hooks/             the sh stubs Git runs, the Python fallbacks, the Go source in src/, prebuilt binaries in bin/
├── workers/           claude (headless CLI) and ollama adapters
├── pipeline.py        the phases
├── gitops.py          the only git writer
└── __main__.py        CLI: run, status, tasks, comments, hooks, install, doctor, pr, release
tests/                 unittest suites
scripts/verify.py      the gate this repo runs
templates/project/     AGENTS.md, CLAUDE.md, BACKLOG.md, CHANGELOG.md, komodo.json templates, docs/spec starters
```

There is no hand-maintained Claude directory. `python3 -m komodo install` renders the adapter into `~/.claude`: `AGENTS.md`, one agent file per session role with model and effort from the active profile's tiers, two procedure skills built from `komodo/rules/`, a review skill from the reviewer role, one thin pointer skill per standard, the session hooks, and a settings policy merged into your personal `settings.json`. Another tool gets another adapter with the same inputs.

Every hook ships compiled. `komodo/hooks/src/` is one Go program, `komodo-hooks`, with a subcommand per hook: `guard` and `inject` for the session, `precommit` and `prepush` for Git. `scripts/build-hooks.py` cross-compiles it for darwin, linux, and windows on amd64 and arm64, and records a checksum manifest the verify gate rebuilds and compares.

`install` copies the session binary to `~/.claude` and points `settings.json` at it; the Git stubs pick the binary for the running machine out of `bin/`. Protected-branch matching, the trailer pattern, and repo-root discovery are written once in Go and shared by all four. The four Python hooks still ship as the fallback for a platform with no committed binary, and the test suite runs every hook case against both so they cannot drift.

## Comments

Every public function gets a one-line doc comment. A private function gets one only when long or non-obvious. Nothing else is commented unless the line cannot say it itself. The lint (`python3 -m komodo comments check`) flags a missing doc, a comment that restates its identifier, cites a version or ticket, uses first person or a hedge, narrates history, or runs past twenty words. It runs in `pre-commit` on staged lines and in the reviewer brief as a judgment class.

## Testing

```bash
python3 scripts/verify.py      # tests, validate, comment lint, doctor
```

There is no CI. The gate runs on the developer's machine, installed by `python3 -m komodo hooks install .`, and `pre-push` refuses a push whose verify run fails. Rebuilding the hook binaries and proving them against the manifest is part of that run, so it needs a Go toolchain; without one, verify checks the committed checksums and says so.

## References

- [docs/architecture.md](docs/architecture.md): phases, state, briefs, and what would change each decision
- [docs/design-decisions.md](docs/design-decisions.md): why each rule exists
- [komodo/rules/backlog.md](komodo/rules/backlog.md): the task grammar
- [SECURITY.md](SECURITY.md): how to report a problem
