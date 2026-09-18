import unittest

from komodo import tasks

SAMPLE = """# Backlog

## [EPIC-01] Now
*Goal*

### [TG-01.1] Refunds
```yaml
type: feat
version: 1.1.0
```

#### [TSK-01.1.1] Add refund handler [P: H] [READY]
```yaml
files: [internal/refund/handler.go, internal/refund/handler_test.go]
done_when:
  - go test ./internal/refund/...
owner: agent
```

#### [TSK-01.1.2] Wire refund route [P: M] [READY]
```yaml
files: [cmd/api/routes.go]
done_when: [go build ./...]
depends_on: [TSK-01.1.1]
```

### [TG-01.2] Docs
```yaml
type: docs
version: 1.1.1
```

#### [TSK-01.2.1] Write runbook [P: L] [WIP]
```yaml
files: [docs/runbook.md]
done_when: [test -f docs/runbook.md]
owner: human
```
"""


class ParseTests(unittest.TestCase):
    def test_groups_and_tasks(self):
        backlog = tasks.parse(SAMPLE)
        self.assertEqual([g.id for g in backlog.groups], ["TG-01.1", "TG-01.2"])
        self.assertEqual(backlog.groups[0].type, "feat")
        self.assertEqual(backlog.groups[0].epic_id, "EPIC-01")
        first = backlog.task("TSK-01.1.1")
        self.assertEqual(first.files, ["internal/refund/handler.go", "internal/refund/handler_test.go"])
        self.assertEqual(first.dirs, ["internal/refund"])
        self.assertEqual(first.done_when, ["go test ./internal/refund/..."])
        self.assertEqual(backlog.task("TSK-01.1.2").depends_on, ["TSK-01.1.1"])

    def test_legacy_wip_maps_to_in_progress(self):
        self.assertEqual(tasks.parse(SAMPLE).task("TSK-01.2.1").status, "IN_PROGRESS")

    def test_lint_clean_sample(self):
        self.assertEqual(tasks.lint(tasks.parse(SAMPLE)), [])

    def test_lint_flags_missing_fields(self):
        text = SAMPLE.replace("files: [cmd/api/routes.go]\n", "")
        problems = tasks.lint(tasks.parse(text))
        self.assertTrue(any("declares no files" in p for p in problems))

    def test_lint_flags_prose_done_when(self):
        text = SAMPLE.replace("go build ./...", "the route works")
        problems = tasks.lint(tasks.parse(text))
        self.assertTrue(any("does not look like a command" in p for p in problems))

    def test_lint_flags_unknown_dependency(self):
        text = SAMPLE.replace("depends_on: [TSK-01.1.1]", "depends_on: [TSK-09.9.9]")
        self.assertTrue(any("unknown task" in p for p in tasks.lint(tasks.parse(text))))

    def test_next_group_skips_human_only(self):
        backlog = tasks.parse(SAMPLE)
        self.assertEqual(backlog.next_group().id, "TG-01.1")
        done = tasks.set_status(tasks.set_status(SAMPLE, "TSK-01.1.1", "DONE"), "TSK-01.1.2", "DONE")
        self.assertIsNone(tasks.parse(done).next_group())

    def test_group_lookup_by_substring(self):
        self.assertEqual(tasks.parse(SAMPLE).group("docs").id, "TG-01.2")


REFINING = """# Backlog

## [EPIC-01] Now

### [TG-01.1] Planning
```yaml
type: feat
version: 1.2.0
```

#### [TSK-01.1.1] Still being scoped [P: M] [REFINEMENT]

### [TG-01.2] Ready work
```yaml
type: feat
version: 1.2.1
```

#### [TSK-01.2.1] Do it [P: H] [READY]
```yaml
files: [a.py]
done_when: [test -d .]
```
"""


class RefinementTests(unittest.TestCase):
    """REFINEMENT is a task still being planned: open work the harness will not pick up."""

    def test_legacy_todo_maps_to_ready(self):
        text = SAMPLE.replace("[P: H] [READY]", "[P: H] [TODO]")
        self.assertEqual(tasks.parse(text).task("TSK-01.1.1").status, "READY")

    def test_a_refinement_task_needs_no_block(self):
        self.assertEqual(tasks.lint(tasks.parse(REFINING)), [])

    def test_a_refinement_task_is_open_but_not_ready(self):
        task = tasks.parse(REFINING).task("TSK-01.1.1")
        self.assertTrue(task.open)
        self.assertFalse(task.ready)

    def test_next_group_skips_a_group_still_in_refinement(self):
        backlog = tasks.parse(REFINING)
        self.assertEqual(backlog.next_group().id, "TG-01.2")
        self.assertEqual(backlog.group("TG-01.1").ready_tasks, [])

    def test_promoting_to_ready_makes_the_group_next(self):
        promoted = tasks.set_status(REFINING, "TSK-01.1.1", "READY")
        self.assertEqual(tasks.parse(promoted).next_group().id, "TG-01.1")

    def test_todo_is_no_longer_a_writable_status(self):
        with self.assertRaises(ValueError):
            tasks.set_status(SAMPLE, "TSK-01.1.1", "TODO")


class GroupVersionTests(unittest.TestCase):
    """A group declares the version it ships; the lint is what stops a changelog and a tag drifting apart."""

    def _backlog(self, block):
        return tasks.parse("# Backlog\n\n## [EPIC-01] Now\n\n### [TG-01.1] Refunds\n```yaml\n%s\n```\n\n#### [TSK-01.1.1] Do it [P: H] [READY]\n```yaml\nfiles: [a.py]\ndone_when: [test -d .]\n```\n" % block)

    def test_a_declared_version_is_read_off_the_group(self):
        self.assertEqual(self._backlog("type: feat\nversion: 2.3.4").group("TG-01.1").version, "2.3.4")

    def test_a_group_with_no_version_fails_lint(self):
        problems = tasks.lint(self._backlog("type: feat"))
        self.assertEqual(len(problems), 1, problems)
        self.assertIn("no version", problems[0])

    def test_a_malformed_version_fails_lint(self):
        for bad in ("1.2", "v1.2.3", "1.2.3-rc1", "latest"):
            problems = tasks.lint(self._backlog("type: feat\nversion: %s" % bad))
            self.assertEqual(len(problems), 1, (bad, problems))
            self.assertIn("is not x.y.z", problems[0])

class RewriteTests(unittest.TestCase):
    def test_set_status_touches_only_the_token(self):
        updated = tasks.set_status(SAMPLE, "TSK-01.1.2", "DONE")
        self.assertIn("#### [TSK-01.1.2] Wire refund route [P: M] [DONE]", updated)
        self.assertEqual(len(updated.splitlines()), len(SAMPLE.splitlines()))
        self.assertIn("[TSK-01.1.1] Add refund handler [P: H] [READY]", updated)

    def test_append_task_lands_at_group_end(self):
        updated, task_id = tasks.append_task(SAMPLE, "TG-01.1", "Add refund metrics", {"files": ["internal/refund/metrics.go"], "done_when": ["go test ./internal/refund/..."]})
        self.assertEqual(task_id, "TSK-01.1.3")
        backlog = tasks.parse(updated)
        self.assertEqual([t.id for t in backlog.group("TG-01.1").tasks], ["TSK-01.1.1", "TSK-01.1.2", "TSK-01.1.3"])
        self.assertEqual(tasks.lint(backlog), [])
        self.assertLess(updated.index("TSK-01.1.3"), updated.index("### [TG-01.2]"))

    def test_append_to_last_group(self):
        updated, task_id = tasks.append_task(SAMPLE, "TG-01.2", "Another doc", {"files": ["docs/x.md"], "done_when": ["test -f docs/x.md"]})
        self.assertEqual(task_id, "TSK-01.2.2")
        self.assertEqual(tasks.lint(tasks.parse(updated)), [])


LEGACY = """### [TG-01.1] Cross-Cutting
* **Target Release:** V1

#### [TSK-01.1.1] Old shape [P: M] [TODO]
| Field | Value |
|---|---|
| Depends on | `TSK-01.1.0` |

| Subtask | Work | Done when |
|---|---|---|
| `SUB-01.1.1.1` | do a thing | `go test ./...` |
| `SUB-01.1.1.2` | do another | the doc reads well |
"""


LEGACY_FULL = """### [TG-01.1] Cross-Cutting

#### [TSK-01.1.1] Old shape [P: M] [TODO]
**Acceptance Criteria**
- the refund lands in the ledger

| Field | Value |
|---|---|
| Depends on | `TSK-01.1.0` |

| Subtask | Category | Work | Done when |
|---|---|---|---|
| `SUB-01.1.1.1` | build | do a thing | `go build ./...` |
| `SUB-01.1.1.2` | test | do another | `TEST_TIER=component go test ./...` |
| `SUB-01.1.1.3` | test | do a third | `grep -q x f && go test ./...` |
| `SUB-01.1.1.4` | docs | write it up | the doc reads well |
"""


class MigrateTests(unittest.TestCase):
    def test_migrate_builds_blocks_and_reports(self):
        text, report = tasks.migrate(LEGACY)
        backlog = tasks.parse(text)
        task = backlog.task("TSK-01.1.1")
        self.assertEqual(task.done_when, ["go test ./..."])
        self.assertEqual(task.depends_on, ["TSK-01.1.0"])
        self.assertEqual(task.fields["done_when_prose"], ["the doc reads well"])
        self.assertTrue(any("prose" in line for line in report))
        self.assertTrue(any("files list is empty" in line for line in report))

    def test_migrate_keeps_the_task_body(self):
        text, _ = tasks.migrate(LEGACY_FULL)
        self.assertIn("**Acceptance Criteria**", text)
        self.assertIn("- the refund lands in the ledger", text)
        self.assertIn("| `SUB-01.1.1.1` | build | do a thing | `go build ./...` |", text)

    def test_migrate_takes_the_last_cell_of_a_four_column_row(self):
        text, _ = tasks.migrate(LEGACY_FULL)
        task = tasks.parse(text).task("TSK-01.1.1")
        self.assertEqual(
            task.done_when,
            ["go build ./...", "TEST_TIER=component go test ./...", "grep -q x f && go test ./..."],
        )

    def test_migrate_keeps_prose_out_of_done_when(self):
        text, _ = tasks.migrate(LEGACY_FULL)
        task = tasks.parse(text).task("TSK-01.1.1")
        self.assertEqual(task.fields["done_when_prose"], ["the doc reads well"])

    def test_migrate_leaves_new_shape_alone(self):
        text, report = tasks.migrate(SAMPLE)
        self.assertEqual(text, SAMPLE)
        self.assertEqual(report, [])


class RemoveTaskTests(unittest.TestCase):
    TEXT = (
        "# Backlog\n\n"
        "## [EPIC-01] Now\n*Goal*\n\n"
        "### [TG-01.1] Refunds\n```yaml\ntype: feat\nversion: 1.1.0\n```\n\n"
        "#### [TSK-01.1.1] First [P: H] [DONE]\n```yaml\nfiles: [a.py]\ndone_when: ['python3 -c pass']\n```\n\n"
        "#### [TSK-01.1.2] Second [P: M] [READY]\n```yaml\nfiles: [b.py]\ndone_when: ['python3 -c pass']\ndepends_on: [TSK-01.1.1]\n```\n\n"
        "### [TG-01.2] Solo\n```yaml\ntype: fix\nversion: 1.1.1\n```\n\n"
        "#### [TSK-01.2.1] Only one [P: L] [DONE]\n```yaml\nfiles: [c.py]\ndone_when: ['python3 -c pass']\n```\n"
    )

    def test_task_and_its_block_are_gone(self):
        out = tasks.remove_task(self.TEXT, "TSK-01.1.1")
        self.assertNotIn("TSK-01.1.1", out)
        self.assertNotIn("a.py", out)
        self.assertIn("TSK-01.1.2", out)

    def test_group_survives_while_it_holds_another_task(self):
        out = tasks.remove_task(self.TEXT, "TSK-01.1.1")
        self.assertIn("### [TG-01.1] Refunds", out)

    def test_dependency_on_the_removed_task_is_stripped(self):
        out = tasks.remove_task(self.TEXT, "TSK-01.1.1")
        self.assertEqual(tasks.lint(tasks.parse(out)), [])
        self.assertNotIn("depends_on", out)

    def test_group_left_with_no_tasks_is_dropped(self):
        out = tasks.remove_task(self.TEXT, "TSK-01.2.1")
        self.assertNotIn("TG-01.2", out)
        self.assertIn("### [TG-01.1] Refunds", out)

    def test_unknown_task_raises(self):
        with self.assertRaises(KeyError):
            tasks.remove_task(self.TEXT, "TSK-09.9.9")


if __name__ == "__main__":
    unittest.main()
