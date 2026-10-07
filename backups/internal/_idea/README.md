# Proposed per-role layout

Scaffold only. Each `.go` file holds a package clause; nothing is migrated. `go vet` passes on every package. Go skips `_idea/` in `./...`, so list the dirs.

Six top-level folders only. A path below without a prefix sits under the folder named in its table.

| Folder | Holds |
|---|---|
| `agents/` | The 9 roles, plus `roles.go` and `schema.go` |
| `skills/` | Skill loader, `render.go` (was `mount/skills.go`), `brief/` |
| `rules/` | Rule loader, same shape as skills |
| `hooks/` | Shared hook table and `timewarn`; role hooks stay in `agents/<role>/hooks/` |
| `guards/` | Guard engine, policy, `secrets/`, `mount/` (permissions, limits) |
| `commands/` | CLI verbs and packages behind them: `gate`, `build`, `doctor`, `install` (+`plugins/`), `detect` (+`profile/`), `facet`, `migrate`, `selfupdate`, `backlog`, `changelog`, `comments`, `git` (+`worktree/`), `eval`, `exec`, `proc` |

Rule: code only one role uses lives under that role. Shared code lives in one of the five folders beside `agents/`. Host adapters stay in `internal/mount`.

## builder: mechanically build code and tests
| Target | Source |
|---|---|
| `builder/guard.go` | `guard/builder.go` (commands `check task`, `check scope`) |
| `builder/hooks/format.go`, `taskchecks.go` | `hooks/format.go`, `hooks/taskchecks.go` |
| `builder/brief.go` | `line/brief.go` BuildBrief, FixBrief, fixTask |
| `builder/close.go` | `line/close.go` CommitBuild, Attempt, RepairText |
| `builder/drive.go` | `conductor/drive.go` work, build, repair, session |
| `builder/request.go` | `run/requests.go` BuilderRequest |
| `builder/mount.go` | `mount/claude/plugin.go` RenderBuilderPlugin |
| `builder/standards.go` | mechanical rules of skills `standards-go`, `standards-shell` |
| `builder/check/check.go`, `coverage.go`, `scope.go` | `check/check.go`, `check/coverage.go` |
| `builder/check/verify.go` | `line/verify.go` command compile |
| `builder/check/fuzz.go`, `comments.go`, `gofmt.go` | `gate/gate.go` fuzz and comments checks, `gate/gofmt.go` |
| `builder/test/` | tests that mirror the above |

## reviewer: mechanically review a diff
| Target | Source |
|---|---|
| `reviewer/guard.go` | `guard/reviewer.go` (command `check findings`) |
| `reviewer/hooks/evidence.go` | `hooks/evidence.go` |
| `reviewer/brief.go` | `line/brief.go` ReviewBrief, LensText |
| `reviewer/diff.go` | `line/diff.go` |
| `reviewer/findings.go` | `line/wave.go` findings funcs, `line/ship.go` ReviewSize, staleReview |
| `reviewer/round.go` | `conductor/drive.go` lenses, reviewRound, mergedReview |
| `reviewer/request.go` | `run/requests.go` ReviewerRequest, ReReviewInput |
| `reviewer/mount.go` | `mount/claude/plugin.go` RenderReviewerPlugin, `limits.go` reviewer funcs |
| `reviewer/lenses.go`, `rereview.go` | `review/lenses.go`, `review/rereview.go` |
| `reviewer/evidence/` | `review/evidence.go` |
| `reviewer/recall.go` | `recall/recall.go`, `mount/registry.go` ReviewerRecall |
| `reviewer/eval/` | `eval/cases.go`, `eval/confidence.go` (reviewer scoring) |
| `reviewer/standards.go` | review-lens rule IDs from `standards-go`, `standards-shell`, `standards-comments` |

Fix: dir `revewier` renamed to `reviewer`.

## orchestrator: run the line
| Target | Source |
|---|---|
| `orchestrator/guard.go`, `hooks/` | `guard/orchestrator.go`, `hooks/status.go`, `hooks/prune.go` |
| `orchestrator/mount.go` | `mount/claude/plugin.go` RenderOrchestratorPlugin, `claude.go` orchestratorSkills |
| `orchestrator/conductor/` | `conductor/*` except drive.go: state, registry, resume, abandon, stop (block), integrate, plugins (notify), schedule |
| `orchestrator/cut/` | `line/cut.go`, `line/collide.go` |
| `orchestrator/epic/` | `line/epic.go`, `line/rephase.go` |
| `orchestrator/ship/` | `line/ship.go` (split), `ship_body.go`, `labels.go`, `merge.go`, `pr/pr.go` open and refresh |
| `orchestrator/release/` | `release/release.go`, `publish.go` |
| `orchestrator/status/` | `line/status.go`, `line/report.go` |
| `orchestrator/run/` | `run/run.go`, `drive.go`, `pace.go`, `preflight/preflight.go`, `run/sync.go` worktree cleanup |
| `orchestrator/plan/` | `plan/plan.go`, `line/next.go`, `line/wave.go` TaskBranch, RecordedWaves |
| `orchestrator/lease/` | `lease/lease.go`, `line/lease.go` |
| `orchestrator/ledger/` | `ledger/ledger.go`, `line/stamp.go` |

## Other roles (little dedicated code today)
| Target | Source |
|---|---|
| `planner/guard.go`, `standards.go` | `guard/planner.go`, skill `standards-specs` |
| `planner/ingest/` | `ingest/card.go`, `ingest/checks.go` |
| `responder/guard.go`, `threads.go`, `hooks/` | none today: no guard suite exists; `pr/pr.go` thread answering, `cmd threads` |
| `researcher/`, `scout/`, `tester/`, `architect/` `guard.go` | `guard/<role>.go` rows only |

## Shared packages: used by two or more roles
| Target | Source |
|---|---|
| `guard/` | `guard/` engine: guard, policy, tokenize (shell), git, gh, paths, hook, table, global (suite); `check/output.go` (output) |
| `hooks/` | `hooks/hooks.go`, `timewarn.go` |
| `brief/` | `line/brief.go` LoadStandards, Fill, WriteBrief; `brief_slots.go`; `clip.go` |
| `roles`, `schema`, `secrets` | `line/roles.go`, `line/schema.go`, `check/secrets.go` + `line/ship.go` Scrub |
| `worktree/` | `line/worktree.go` git primitives and result files |
| `build`, `gate` | `gate/gate.go` build; `gate/gate.go`, `githook.go`, `checkout.go` |
| `exec`, `proc`, `profile` | `line/verify.go` RunCommand; `proc/*`; `profile/profile.go` |
| `selfupdate` | `run/sync.go` syncBinary, syncRelease |
| `eval` | `eval/suite.go`, `run.go`, `live.go`, `report.go` |
| `detect`, `facet`, `install`, `migrate` | `detect/`, `facet/`, `install/`, `backlog/import.go`, `legacy.go`, `cmd/migrate.go` |
| `mount/` | role-free parts of `mount/claude`: permissions, limits (tiers), shared render |
| `comments`, `changelog`, `backlog`, `doctor` | same-named `internal` packages |
| `git`, `git/pr` | `git/`, `repo/`, `pr/pr.go` client, `line/labels.go` |
| `plugins/` | `plugin/plugin.go` |

## Splits the migration must make first
1. `guard/policy.go`: `readOnlyRoles`, `RoleEnv`, `LineRefusedPaths` become per-suite attributes.
2. `guard/guard.go`: `komodoFindings` hard-codes `orchestrator`.
3. `guard/global.go`: `roleOrder` becomes registration-driven.
4. `guard/table.go`: orchestrator rows leave `globalRows`.
5. `hooks/hooks.go`: `Table()` becomes registry-driven with an exported register function.
6. `line/ship.go` (1378 lines), `conductor/drive.go` (967), `line/worktree.go`, `gate/gate.go`, `run/sync.go`.

## Skills: condense or keep
| Skill | Verdict |
|---|---|
| `standards-go`, `standards-shell` | Mechanical rules become `builder/standards.go` and `reviewer/standards.go`; keep a short design-guidance prompt. |
| `standards-comments` | Already backed by `comments/`; shrink to a rule table. |
| `standards-specs` | Keep as a prompt, owned by planner. |
| `release` | Mechanical steps into `orchestrator/release`; keep a short prompt. |
| `run`, `adhoc` | Thin wrappers over `komodo run` and `komodo stage`; prompts. |
| `plan`, `respond` | Judgment prompts; keep. |
| `komodo` | Generated by `komodo help --skill`; stays global. |

## Commands (`cmd/komodo`) by role
| Role | Subcommands |
|---|---|
| builder | `check task`, `check scope`, `comments` |
| reviewer | `check findings`, `recall`, `eval` |
| orchestrator | `run`, `resume`, `abandon`, `status`, `diff`, `ship`, `sync`, `worktree`, `ingest`, `release`, `metrics` |
| planner | `lint`, `add`, `backlog`, `list` |
| responder | `threads`, `pr`, `rephase` |
| global | `gate`, `git-hook`, `guard`, `hook`, `install`, `detect`, `init`, `migrate`, `help`, `doctor`, `tag` |

## Skill loader (mechanical, on demand)

The harness picks skills from facts; the model never chooses. Files are placeholders.

| File | Job |
|---|---|
| `skills/manifest.go` | Index of embedded skills: name, role, triggers, token cost |
| `skills/context.go` | Facts for one call: role, stage, task files, failure kind, findings rule IDs |
| `skills/trigger.go` | Match rules: file glob, role, stage, failure kind, rule ID |
| `skills/budget.go` | Token cap per call, priority order, dedupe, drop report |
| `skills/loader.go` | `Select(ctx) []Skill`: pure, deterministic, no I/O after embed |
| `skills/fetch.go` | Optional host tool: model asks for a skill by name; same budget applies |
| `mount/skills.go` | Host render: inline text (local, Claude) or native skill file (Claude) |
| `rules/{manifest,context,loader}.go` | Same shape for rules: universal rules always load, role slices selected; Ollama only, Claude and Codex read rules natively |
| `<role>/skills/`, `<role>/rules/` | Role-owned `.md` files with frontmatter (`globs`, `stage`, `tokens`); lookup order role, shared, repo override |
| `rules/{manifest,context,loader}.go` | Same shape for rules: universal rules always load, role slices selected; Ollama only, since Claude and Codex read rules natively |
| `<role>/skills/`, `<role>/rules/` | Role-owned `.md` files with frontmatter (`globs`, `stage`, `tokens`); lookup order role, shared, repo override |
| `<role>/skills.go` | Each role's manifest entries, `//go:embed` of its `standards/` and `prompt.md` |

### Plan
1. **Manifest:** each role registers entries at `init()`, as guard suites do. Repo overrides from `.komodo/standards` layer on top.
2. **Context:** build it once per brief from the task card, stage and last failure. Reuse `line/brief.go` StandardsFor logic.
3. **Select:** match triggers, sort by priority, stop at the budget, record what was dropped.
4. **Render:** the host adapter inlines the text; Claude may also write native skill files.
5. **Repair rounds:** a failure kind or finding rule ID pulls the matching skill in for that retry only.
6. **Fetch tool (optional):** wired only on hosts with tool calls. Counts against the same budget.
7. **Proof:** table tests on `Select`, plus a token-count golden per role (`eval` suite).

Open decision: the per-call token cap (suggest 2,000 for local, 4,000 for Claude).
