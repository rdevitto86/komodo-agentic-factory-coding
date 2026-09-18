"""Label derivation for a PR and the pr label command, including --auto."""

import argparse
import io
import os
import sys
import unittest
from contextlib import redirect_stdout, redirect_stderr
from unittest import mock

sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))

from komodo import pr, pr_actions
from komodo.config import Config

MAPPING = {"feat": "enhancement", "fix": "bug", "docs": "documentation", "refactor": "enhancement", "agent": "@agent"}
DEFINED = ["bug", "documentation", "enhancement", "@agent", "refactor"]


class KindOfTests(unittest.TestCase):
    def test_reads_the_type_from_a_conventional_title(self):
        self.assertEqual(pr.kind_of("feat: add a thing", "anything/here"), "feat")

    def test_reads_the_type_through_a_scope_and_a_breaking_bang(self):
        self.assertEqual(pr.kind_of("refactor(release): retire Unreleased", "x/y"), "refactor")
        self.assertEqual(pr.kind_of("fix(pr)!: drop the flag", "x/y"), "fix")

    def test_falls_back_to_the_branch_prefix(self):
        self.assertEqual(pr.kind_of("Retire Unreleased", "refactor/retire-unreleased"), "refactor")

    def test_no_type_anywhere_is_empty(self):
        self.assertEqual(pr.kind_of("Retire Unreleased", "patch-1"), "")


class PickLabelsTests(unittest.TestCase):
    def test_maps_the_type_and_always_adds_the_agent_label(self):
        self.assertEqual(pr.pick_labels("fix", MAPPING, DEFINED), ["bug", "@agent"])

    def test_never_invents_a_label_the_repo_does_not_define(self):
        self.assertEqual(pr.pick_labels("fix", MAPPING, ["@agent"]), ["@agent"])

    def test_an_unmapped_type_still_gets_the_agent_label(self):
        self.assertEqual(pr.pick_labels("chore", MAPPING, DEFINED), ["@agent"])


class LabelCommandTests(unittest.TestCase):
    def setUp(self):
        self.config = Config({"labels": MAPPING}, root=".")
        self.added = []

    def _run(self, wanted, auto, info):
        def edit(root, number, body=None, title=None, add_labels=()):
            self.added.append((number, list(add_labels)))

        out, err = io.StringIO(), io.StringIO()
        with mock.patch.object(pr, "view", return_value=info), \
             mock.patch.object(pr, "existing_labels", return_value=DEFINED), \
             mock.patch.object(pr, "edit", edit), \
             redirect_stdout(out), redirect_stderr(err):
            code = pr_actions.label(".", self.config, wanted, auto=auto)
        return code, out.getvalue() + err.getvalue()

    def test_auto_derives_the_labels_the_pipeline_would_have_used(self):
        info = {"number": 85, "title": "refactor(release): retire Unreleased", "headRefName": "refactor/x", "labels": []}
        code, text = self._run([], True, info)
        self.assertEqual(code, 0)
        self.assertEqual(self.added, [(85, ["enhancement", "@agent"])])
        self.assertIn("labelled #85", text)

    def test_auto_adds_only_what_is_missing(self):
        info = {"number": 85, "title": "fix: a thing", "headRefName": "fix/x", "labels": [{"name": "bug"}]}
        code, _ = self._run([], True, info)
        self.assertEqual(code, 0)
        self.assertEqual(self.added, [(85, ["@agent"])])

    def test_auto_on_an_already_labelled_pr_writes_nothing(self):
        info = {"number": 85, "title": "fix: a thing", "headRefName": "fix/x", "labels": [{"name": "bug"}, {"name": "@agent"}]}
        code, text = self._run([], True, info)
        self.assertEqual(code, 0)
        self.assertEqual(self.added, [], "an idempotent second run makes no API write")
        self.assertIn("already carries", text)

    def test_auto_with_no_type_anywhere_is_a_usage_error(self):
        info = {"number": 85, "title": "Retire Unreleased", "headRefName": "patch-1", "labels": []}
        code, text = self._run([], True, info)
        self.assertEqual(code, 2)
        self.assertEqual(self.added, [])
        self.assertIn("no commit type", text)

    def test_explicit_labels_still_work_and_reject_an_undefined_one(self):
        info = {"number": 85, "title": "fix: a thing", "headRefName": "fix/x", "labels": []}
        self.assertEqual(self._run(["hooks"], False, info)[0], 2)
        self.assertEqual(self.added, [])
        self.assertEqual(self._run(["bug"], False, info)[0], 0)
        self.assertEqual(self.added, [(85, ["bug"])])

    def test_no_labels_and_no_auto_is_a_usage_error(self):
        code, text = self._run([], False, {"number": 85, "title": "fix: x", "headRefName": "fix/x", "labels": []})
        self.assertEqual(code, 2)
        self.assertIn("--auto", text)


if __name__ == "__main__":
    unittest.main()
