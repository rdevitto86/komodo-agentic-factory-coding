package line

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"komodo/internal/git"
	"komodo/internal/install"
	"komodo/internal/mount"
)

// renderedCopy is the project copy the fake host below renders, which no commit may carry.
const renderedCopy = ".fakehost-stage/skills/run/SKILL.md"

// registerRenderingHost mounts a fake host whose render marks renderedCopy as a project copy.
func registerRenderingHost(t *testing.T) {
	t.Helper()
	snapshot := mount.Snapshot()
	t.Cleanup(func() { mount.Restore(snapshot) })
	mount.Register(mount.Host{
		Name: "fakehost-stage",
		Render: func(root, binary string) (install.Plan, error) {
			plan := install.Plan{Host: "fakehost-stage", Root: root}
			plan.AddProject(filepath.Join(root, filepath.FromSlash(renderedCopy)), []byte("rendered\n"), "a rendered skill")
			plan.Add(filepath.Join(root, ".fakehost-stage", "settings.json"), []byte("{}\n"), "shared settings")
			return plan, nil
		},
	})
}

// writeFile writes body at root/name, making its directories.
func writeFile(t *testing.T, root, name, body string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestShipCommitsNeitherLineStateNorARenderedCopy proves the ship commit leaves out the state dir and a mount's project copy.
func TestShipCommitsNeitherLineStateNorARenderedCopy(t *testing.T) {
	registerRenderingHost(t)
	root, group := shipRepo(t)
	writeFile(t, group, filepath.ToSlash(filepath.Join(StateDir, "briefs", "TG-09.1-review.md")), "a brief\n")
	writeFile(t, group, renderedCopy, "rendered\n")
	writeFile(t, group, ".komodo/context/api.md", "declared context\n")
	writeFile(t, group, "one.go", "package a\n\nfunc One() {}\n")
	plan := &Plan{
		Group: "TG-09.1", Title: "A group", Type: "feat", Base: "main", Branch: "feat/a-group", Worktree: "group",
		Tasks: []PlanTask{{ID: "TSK-09.1.1", Title: "Do it", Files: []string{"one.go", ".komodo/context/api.md"}}},
	}
	if _, err := ShipGroup(root, plan, nil, nil); err != nil {
		t.Fatal(err)
	}
	files, err := git.Run(group, "show", "--name-only", "--format=", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	committed := strings.Split(files, "\n")
	for _, banned := range []string{"briefs", renderedCopy} {
		if strings.Contains(files, banned) {
			t.Fatalf("the ship commit carries %s: %v", banned, committed)
		}
	}
	for _, want := range []string{"one.go", ".komodo/context/api.md"} {
		if !strings.Contains(files, want) {
			t.Fatalf("the ship commit is missing %s: %v", want, committed)
		}
	}
	if status, _ := git.Run(group, "status", "--porcelain"); !strings.Contains(status, "?? .fakehost-stage/") {
		t.Fatalf("status = %q; the rendered copy stays untracked on disk", status)
	}
}

// TestCloseCommitsNeitherLineStateNorARenderedCopy proves a task close commit leaves out the same paths.
func TestCloseCommitsNeitherLineStateNorARenderedCopy(t *testing.T) {
	registerRenderingHost(t)
	root := gitRepo(t)
	commit(t, root, "a/one.go", "package a\n", "seed")
	writeFile(t, root, "a/one.go", "package a\n\nfunc One() {}\n")
	writeFile(t, root, filepath.ToSlash(filepath.Join(StateDir, "briefs", "TSK-09.1.1.md")), "a brief\n")
	writeFile(t, root, renderedCopy, "rendered\n")
	if err := stageWork(root, []string{"a/one.go"}); err != nil {
		t.Fatal(err)
	}
	staged, err := git.Run(root, "diff", "--cached", "--name-only")
	if err != nil {
		t.Fatal(err)
	}
	if staged != "a/one.go" {
		t.Fatalf("staged = %q; only the task's own file belongs in its commit", staged)
	}
}

// TestCloseStagesWhenTheStateDirIsIgnoredAndOnDisk proves an ignored state dir holding a brief never fails the stage.
func TestCloseStagesWhenTheStateDirIsIgnoredAndOnDisk(t *testing.T) {
	registerRenderingHost(t)
	root := gitRepo(t)
	commit(t, root, ".gitignore", "/"+StateDir+"/\n", "ignore state")
	commit(t, root, "a/one.go", "package a\n", "seed")
	writeFile(t, root, "a/one.go", "package a\n\nfunc One() {}\n")
	writeFile(t, root, filepath.ToSlash(filepath.Join(StateDir, "briefs", "TSK-09.1.1.md")), "a brief\n")
	writeFile(t, root, renderedCopy, "rendered\n")
	if err := stageWork(root, []string{"a/one.go"}); err != nil {
		t.Fatal(err)
	}
	staged, err := git.Run(root, "diff", "--cached", "--name-only")
	if err != nil {
		t.Fatal(err)
	}
	if staged != "a/one.go" {
		t.Fatalf("staged = %q; only the task's own file belongs in its commit", staged)
	}
}

// TestQCRunsGoTestInAGoWorktreeWithNoCommandsFile proves QC resolves go build and go test from detection on the worktree.
func TestQCRunsGoTestInAGoWorktreeWithNoCommandsFile(t *testing.T) {
	root := gitRepo(t)
	commit(t, root, "README.md", "root\n", "seed")
	worktree := filepath.Join(StateDir, "wt", "TG-13.1")
	group := filepath.Join(root, worktree)
	writeFile(t, group, "go.mod", "module example.com/qc\n\ngo 1.21\n")
	writeFile(t, group, "qc.go", "package qc\n\n// One is one.\nfunc One() int { return 1 }\n")
	writeFile(t, group, "qc_test.go", "package qc\n\nimport \"testing\"\n\nfunc TestOne(t *testing.T) {\n\tif One() != 1 {\n\t\tt.Fatal(\"one\")\n\t}\n}\n")
	if err := SaveRun(root, RunState{Run: "TG-13.1-1", Group: "TG-13.1", Worktree: worktree}); err != nil {
		t.Fatal(err)
	}
	plan := &Plan{Group: "TG-13.1", Mode: "single", Worktree: worktree, Waves: [][]string{{"TSK-13.1.1"}}}
	result, err := CloseWave(root, plan, 0)
	if err != nil {
		t.Fatal(err)
	}
	if !result.OK || result.Verify == nil || result.Verify.Command != "go test ./..." || !result.Verify.OK() {
		t.Fatalf("result = %+v; QC must run go test in a Go worktree", result)
	}
	if len(result.Gates) != 1 || !strings.Contains(result.Gates[0].Command, "go build ./...") {
		t.Fatalf("gates = %+v; QC must run go build in a Go worktree", result.Gates)
	}
	recorded := RecordedWaves(root, "TG-13.1")
	if len(recorded) != 1 || recorded[0].Verify == nil || recorded[0].Verify.Command != "go test ./..." {
		t.Fatalf("recorded = %+v; the QC ledger row must carry what ran", recorded)
	}
	body := ReportBody(plan, &ShipResult{}, recorded, BodyContext{})
	if strings.Contains(body, "no QC gate or verify command ran") || !strings.Contains(body, "go test ./...") {
		t.Fatalf("body = %s", body)
	}
}

// TestVerifyCommandFallsBackToTheRootsCommandsFile proves verify and compile only ever read the root's commands file.
func TestVerifyCommandFallsBackToTheRootsCommandsFile(t *testing.T) {
	root := t.TempDir()
	group := filepath.Join(root, StateDir, "wt", "TG-13.1")
	writeFile(t, group, "go.mod", "module x\n")
	writeFile(t, root, filepath.ToSlash(filepath.Join(StateDir, "commands.json")), `{"verify":"make check","compile":"make build"}`)
	if got := VerifyCommand(root, group); got != "make check" {
		t.Fatalf("verify = %q", got)
	}
	if got := CompileCommands(root, group); len(got) != 1 || got[0] != "make build" {
		t.Fatalf("compile = %v", got)
	}
	writeFile(t, group, filepath.ToSlash(filepath.Join(StateDir, "commands.json")), `{"verify":"make own"}`)
	if got := VerifyCommand(root, group); got != "make check" {
		t.Fatalf("verify = %q, want the root's command; a worktree's own commands.json must never override it", got)
	}
}
