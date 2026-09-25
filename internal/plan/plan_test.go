package plan

import (
	"fmt"
	"sort"
	"strings"
	"testing"

	"komodo/internal/backlog"
)

// twoTasks parses a group of two READY tasks listing the given files.
func twoTasks(t *testing.T, first, second string) []backlog.Task {
	t.Helper()
	text := "### [TG-21.1] A group\n```yaml\ntype: feat\nversion: 1.0.0\n```\n\n" +
		"#### [TSK-21.1.1] First [P: C] [READY]\n```yaml\nfiles: [" + first + "]\ndone_when: [\"true\"]\n```\n\n" +
		"#### [TSK-21.1.2] Second [P: C] [READY]\n```yaml\nfiles: [" + second + "]\ndone_when: [\"true\"]\n```\n"
	return backlog.Parse(text).Groups[0].Tasks
}

func TestTwoTasksOwningNestedTreesNeverShareAWave(t *testing.T) {
	waves, err := Waves(twoTasks(t, "internal", "internal/profile"), nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(waves) != 2 {
		t.Fatalf("waves = %v; a task owning internal cannot run beside one owning internal/profile", waves)
	}
}

func TestTwoFilesInOneDirectoryShareAWave(t *testing.T) {
	waves, err := Waves(twoTasks(t, "web/a/x.ts", "web/a/y.ts"), nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(waves) != 1 || len(waves[0]) != 2 {
		t.Fatalf("waves = %v; different files in one directory must share wave 1", waves)
	}
}

func TestTwoGoFilesInOnePackageSerialize(t *testing.T) {
	waves, err := Waves(twoTasks(t, "internal/backlog/lint.go", "internal/backlog/backlog.go"), nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(waves) != 2 {
		t.Fatalf("waves = %v; both builders may edit the package's shared test file", waves)
	}
}

func TestAFileAndItsDirectorySerialize(t *testing.T) {
	waves, err := Waves(twoTasks(t, "internal/a/x.go", "internal/a"), nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(waves) != 2 {
		t.Fatalf("waves = %v; a directory claim holds every file under it", waves)
	}
}

func TestOneSharedFileSerializes(t *testing.T) {
	waves, err := Waves(twoTasks(t, "internal/a/x.go, README.md", "./README.md"), nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(waves) != 2 {
		t.Fatalf("waves = %v; two tasks listing README.md cannot build at once", waves)
	}
}

func TestClaimsOverlapReadsPathsNotPrefixes(t *testing.T) {
	cases := []struct {
		left, right string
		want        bool
	}{
		{"internal/a/x.go", "internal/a/y.go", true},
		{"web/a/x.ts", "web/a/y.ts", false},
		{"web/a/x.ts", "web/a/x.test.ts", true},
		{".", "internal/a/x.go", true},
		{"./", "internal/a/x.go", true},
		{"internal/a/x.go", "internal/a", true},
		{"internal/a/", "internal/a/b/c.go", true},
		{"internal/ab", "internal/a", false},
		{"internal/a/x.go", "internal/a/x.go", true},
		{"cmd/komodo/main.go", "internal/line", false},
	}
	for _, c := range cases {
		tasks := twoTasks(t, c.left, c.right)
		if got := Overlap(tasks[0], tasks[1]); got != c.want {
			t.Errorf("Overlap(%s, %s) = %v, want %v", c.left, c.right, got, c.want)
		}
	}
}

// chain parses four tasks: .2 depends on .1, .3 on .2, and .4 stands alone, each on its own file.
func chain(t *testing.T, cycle bool) []backlog.Task {
	t.Helper()
	first := "[]"
	if cycle {
		first = "[TSK-22.1.3]"
	}
	task := func(n, deps string) string {
		return "#### [TSK-22.1." + n + "] Task " + n + " [P: C] [READY]\n```yaml\nfiles: [d" + n + "/x.go]\ndone_when: [\"true\"]\ndepends_on: " + deps + "\n```\n\n"
	}
	text := "### [TG-22.1] A group\n```yaml\ntype: feat\nversion: 1.0.0\n```\n\n" +
		task("1", first) + task("2", "[TSK-22.1.1]") + task("3", "[TSK-22.1.2]") + task("4", "[]")
	return backlog.Parse(text).Groups[0].Tasks
}

// ids lists each wave's task IDs, for a readable comparison.
func ids(waves [][]backlog.Task) [][]string {
	var out [][]string
	for _, wave := range waves {
		var row []string
		for _, task := range wave {
			row = append(row, task.ID)
		}
		out = append(out, row)
	}
	return out
}

func TestADependentWaitsForItsDependencyAndADoneTaskIsSkipped(t *testing.T) {
	waves, err := Waves(chain(t, false), nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	if got := fmt.Sprint(ids(waves)); got != "[[TSK-22.1.1 TSK-22.1.4] [TSK-22.1.2] [TSK-22.1.3]]" {
		t.Fatalf("waves = %s; each dependent waits a wave for what it depends on", got)
	}
	waves, err = Waves(chain(t, false), []string{"TSK-22.1.1", "TSK-22.1.4"}, 0)
	if err != nil {
		t.Fatal(err)
	}
	if got := fmt.Sprint(ids(waves)); got != "[[TSK-22.1.2] [TSK-22.1.3]]" {
		t.Fatalf("waves = %s; a done task never runs again and frees its dependent", got)
	}
}

func TestCapacityCapsAWave(t *testing.T) {
	waves, err := Waves(twoTasks(t, "web/a/x.ts", "web/b/y.ts"), nil, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(waves) != 2 {
		t.Fatalf("waves = %v; capacity 1 runs one task per wave", ids(waves))
	}
}

func TestADependencyCycleIsNamed(t *testing.T) {
	if _, err := Topological(chain(t, true)); err == nil || !strings.Contains(err.Error(), "dependency cycle") {
		t.Fatalf("err = %v; a cycle must be named, not ordered", err)
	}
	if _, err := Waves(chain(t, true), nil, 0); err == nil {
		t.Fatal("waves planned a group whose dependencies form a cycle")
	}
}

func TestBlockedByFollowsEveryDependent(t *testing.T) {
	blocked := BlockedBy(chain(t, false), "TSK-22.1.1")
	sort.Strings(blocked)
	if got := strings.Join(blocked, " "); got != "TSK-22.1.2 TSK-22.1.3" {
		t.Fatalf("blocked = %q; a failure blocks its dependents and theirs, never an unrelated task", got)
	}
	if blocked := BlockedBy(chain(t, false), "TSK-22.1.4"); len(blocked) != 0 {
		t.Fatalf("blocked = %v; nothing depends on TSK-22.1.4", blocked)
	}
}
