package line

import (
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
		if got := claimsOverlap(tasks[0], tasks[1]); got != c.want {
			t.Errorf("claimsOverlap(%s, %s) = %v, want %v", c.left, c.right, got, c.want)
		}
	}
}
