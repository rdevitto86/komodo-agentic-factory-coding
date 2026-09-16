# Project Backlog

Priority `[P: C|H|M|L]`. Status `[TODO|IN_PROGRESS|BLOCKED|DONE]`. Ids `EPIC-XX` > `TG-XX.Y` > `TSK-XX.Y.Z`. The grammar the harness parses is in the `backlog` skill; `python3 -m komodo tasks lint` checks it. Items tied to the pre-1.0 machinery were dropped in the 1.0.0 release; the PR that shipped it lists them.

---

## [EPIC-02] V1.1, harden the line
*Goal: prove the harness on real repos and close the gaps the first runs expose.*

### [TG-02.1] Worker providers
```yaml
type: feat
```

#### [TSK-02.1.1] Ollama first-pass review before the Claude reviewer in the fast profile [P: M] [TODO]
```yaml
files: [komodo/pipeline.py, komodo/workers/ollama.py, tests/test_pipeline.py]
done_when:
  - python3 -m unittest tests.test_pipeline -q
context: [docs/architecture.md#review]
```

#### [TSK-02.1.2] Per-worker wall-clock and cost caps surfaced as report warnings when a role runs over its spec [P: M] [TODO]
```yaml
files: [komodo/pipeline.py, komodo/render.py, tests/test_pipeline.py]
done_when:
  - python3 -m unittest tests.test_pipeline -q
```

#### [TSK-02.1.3] Bridge: set num_ctx on generate requests so a large summarizer payload does not truncate silently [P: M] [BLOCKED]
```yaml
files: [bridges/komodo-bridge/README.md]
done_when:
  - test -f bridges/komodo-bridge/README.md
owner: human
context: [bridges/komodo-bridge/README.md#limits]
```

### [TG-02.2] PR actions
```yaml
type: feat
```

#### [TSK-02.2.1] Unit tests for pr_actions.sync and pr_actions.respond with a mocked gh and a fake worker [P: H] [TODO]
```yaml
files: [tests/test_pr_actions.py, komodo/pr_actions.py]
done_when:
  - python3 -m unittest tests.test_pr_actions -q
```

#### [TSK-02.2.2] pr respond marks a thread resolved through GraphQL after replying [P: L] [TODO]
```yaml
files: [komodo/pr.py, komodo/pr_actions.py]
done_when:
  - python3 -m unittest tests.test_pr_actions -q
depends_on: [TSK-02.2.1]
```

### [TG-02.3] CI and security gates
```yaml
type: ci
```

#### [TSK-02.3.1] Secret scan and dependency scan in the verify workflow, blocking on a verified credential or a High advisory [P: M] [TODO]
```yaml
files: [.github/workflows/verify.yml]
done_when:
  - python3 -c "import yaml" 2>/dev/null || python3 -c "print('yaml parse skipped')"
  - test -f .github/workflows/verify.yml
context: [komodo/standards/cicd.md#ci]
```

### [TG-02.4] Planner quality
```yaml
type: feat
```

#### [TSK-02.4.1] komodo tasks plan validates every proposed done_when by running it once in a scratch worktree before appending [P: M] [TODO]
```yaml
files: [komodo/__main__.py, tests/test_cli.py]
done_when:
  - python3 -m unittest tests.test_cli -q
```
