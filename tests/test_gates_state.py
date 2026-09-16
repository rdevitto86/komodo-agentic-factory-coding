import os
import shlex
import sys
import tempfile
import time
import unittest

from komodo import gates, state

PY = sys.executable if " " not in sys.executable else (shlex.quote(sys.executable) if os.name != "nt" else '"%s"' % sys.executable)
OK = '%s -c "import sys; sys.exit(0)"' % PY
FAIL = '%s -c "import sys; sys.exit(1)"' % PY
SLOW = '%s -c "import time; time.sleep(3)"' % PY


class GateTests(unittest.TestCase):
    def test_resolve_verify_order(self):
        with tempfile.TemporaryDirectory() as root:
            self.assertIsNone(gates.resolve_verify(root))
            with open(os.path.join(root, "Makefile"), "w") as handle:
                handle.write("verify:\n\ttrue\n")
            self.assertEqual(gates.resolve_verify(root), "make verify")
            os.makedirs(os.path.join(root, "scripts"))
            with open(os.path.join(root, "scripts", "verify.py"), "w") as handle:
                handle.write("print('ok')\n")
            self.assertTrue(gates.resolve_verify(root).endswith("scripts/verify.py"))

    def test_run_gate_stops_at_first_failure(self):
        with tempfile.TemporaryDirectory() as root:
            gate = gates.run_gate("done_when", [OK, FAIL, OK], root, timeout=10)
            self.assertFalse(gate.ok)
            self.assertEqual(len(gate.results), 2)
            self.assertEqual(gate.failures()[0].command, FAIL)

    def test_run_gate_skips_empty(self):
        with tempfile.TemporaryDirectory() as root:
            gate = gates.run_gate("compile", [], root, timeout=10)
            self.assertTrue(gate.ok)
            self.assertEqual(gate.skipped, "nothing to run")

    def test_timeout_is_a_failure(self):
        with tempfile.TemporaryDirectory() as root:
            result = gates.run_command(SLOW, root, timeout=1)
            self.assertEqual(result.returncode, 124)
            self.assertIn("timed out", result.output)

    def test_compile_commands_from_manifests(self):
        with tempfile.TemporaryDirectory() as root:
            self.assertEqual(gates.compile_commands(root), [])
            with open(os.path.join(root, "go.mod"), "w") as handle:
                handle.write("module x\n")
            self.assertEqual(gates.compile_commands(root), ["go build ./... && go vet ./..."])


class StateTests(unittest.TestCase):
    def test_roundtrip_and_resume(self):
        with tempfile.TemporaryDirectory() as root:
            store = state.Store(root)
            run = state.RunState(run_id=store.new_id("TG-01.1"), group_id="TG-01.1", branch="feat/x", base="main", profile="fast", started=time.time())
            run.task("TSK-01.1.1").status = "DONE"
            run.workers.append(state.WorkerRecord(role="builder", task_id="TSK-01.1.1", provider="claude", model="sonnet", ok=True, seconds=3.2, cost_usd=0.12))
            run.mark_phase("build")
            store.save(run)
            loaded = store.load(run.run_id)
            self.assertEqual(loaded.tasks["TSK-01.1.1"].status, "DONE")
            self.assertEqual(loaded.cost_usd, 0.12)
            self.assertEqual(loaded.phase, "build")
            self.assertEqual(store.latest_for_group("TG-01.1").run_id, run.run_id)
            loaded.finished = time.time()
            store.save(loaded)
            self.assertIsNone(store.latest_for_group("TG-01.1"))


if __name__ == "__main__":
    unittest.main()
