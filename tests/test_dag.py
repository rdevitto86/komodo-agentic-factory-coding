import unittest

from komodo import dag
from komodo.tasks import Task


def task(task_id, files, deps=()):
    return Task(id=task_id, title=task_id, priority="M", status="TODO", fields={"files": list(files), "depends_on": list(deps)})


class WaveTests(unittest.TestCase):
    def test_disjoint_dirs_share_a_wave(self):
        result = dag.waves([task("A", ["x/a.go"]), task("B", ["y/b.go"])])
        self.assertEqual([[t.id for t in wave] for wave in result], [["A", "B"]])

    def test_same_dir_serializes(self):
        result = dag.waves([task("A", ["x/a.go"]), task("B", ["x/b.go"])])
        self.assertEqual([[t.id for t in wave] for wave in result], [["A"], ["B"]])

    def test_parent_dir_serializes(self):
        result = dag.waves([task("A", ["x/a.go"]), task("B", ["x/sub/b.go"])])
        self.assertEqual(len(result), 2)

    def test_dependency_orders_waves(self):
        result = dag.waves([task("B", ["y/b.go"], ["A"]), task("A", ["x/a.go"])])
        self.assertEqual([[t.id for t in wave] for wave in result], [["A"], ["B"]])

    def test_done_tasks_are_skipped_and_satisfy_deps(self):
        result = dag.waves([task("A", ["x/a.go"]), task("B", ["y/b.go"], ["A"])], done=["A"])
        self.assertEqual([[t.id for t in wave] for wave in result], [["B"]])

    def test_cycle_raises(self):
        with self.assertRaises(dag.CycleError):
            dag.waves([task("A", ["x/a.go"], ["B"]), task("B", ["y/b.go"], ["A"])])

    def test_blocked_by_is_transitive(self):
        tasks = [task("A", ["a/a.go"]), task("B", ["b/b.go"], ["A"]), task("C", ["c/c.go"], ["B"]), task("D", ["d/d.go"])]
        self.assertEqual(dag.blocked_by(tasks, "A"), ["B", "C"])


if __name__ == "__main__":
    unittest.main()
