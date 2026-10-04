package gate

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"komodo/internal/install"
	"komodo/internal/mount"
)

// freshClone copies the working tree's tracked and addable files into a new repo, so the gate proves this commit.
func freshClone(t *testing.T) string {
	t.Helper()
	source, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	clone, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	for _, rel := range strings.Split(strings.TrimSpace(gitCommand(t, source, "ls-files", "--cached", "--others", "--exclude-standard")), "\n") {
		data, err := os.ReadFile(filepath.Join(source, rel))
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			t.Fatal(err)
		}
		info, err := os.Stat(filepath.Join(source, rel))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(filepath.Dir(filepath.Join(clone, rel)), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(clone, rel), data, info.Mode().Perm()); err != nil {
			t.Fatal(err)
		}
	}
	gitCommand(t, clone, "init", "-q", "-b", "feat/out-of-box")
	gitCommand(t, clone, "add", "-A")
	gitCommand(t, clone, "-c", "user.email=a@example.com", "-c", "user.name=a", "commit", "-q", "--no-verify", "-m", "clone")
	return clone
}

// runIn runs name with args in dir under the test's environment, failing the test on a non-zero exit.
func runIn(t *testing.T, dir, name string, args ...string) string {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s %v: %v\n%s", name, args, err, out)
	}
	return string(out)
}

// hookCommands lists every hook binary named by a JSON file under dir, keyed by the file that names it.
func hookCommands(t *testing.T, dir string) map[string][]string {
	t.Helper()
	found := map[string][]string{}
	_ = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		// Test fixtures, golden renders among them, are not files a host reads.
		if err == nil && info.IsDir() && info.Name() == "testdata" {
			return filepath.SkipDir
		}
		if err != nil || info.IsDir() || !strings.HasSuffix(path, ".json") {
			return nil
		}
		if data, err := os.ReadFile(path); err == nil {
			if binaries := install.HookBinaries(data); len(binaries) > 0 {
				found[path] = binaries
			}
		}
		return nil
	})
	return found
}

// TestAFreshCloneWorksOutOfTheBox proves the whole first-run path with an empty home: one install from source,
// every hook naming the one published binary exactly once per file, and no refusal of normal work.
func TestAFreshCloneWorksOutOfTheBox(t *testing.T) {
	if testing.Short() {
		t.Skip("builds the binary from a fresh clone")
	}
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("KOMODO_BIN_DIR", filepath.Join(home, ".local", "bin"))
	clone := freshClone(t)
	runIn(t, clone, "go", "run", "komodo/cmd/komodo", "install")

	fixed, err := mount.HookPath()
	if err != nil {
		t.Fatal(err)
	}
	if info, err := os.Stat(fixed); err != nil || info.Mode().Perm()&0o111 == 0 {
		t.Fatalf("published binary %s: %v; install must put the hooks' binary in place", fixed, err)
	}
	hook, err := os.ReadFile(filepath.Join(clone, ".git", "hooks", "pre-commit"))
	if err != nil || !strings.Contains(string(hook), filepath.ToSlash(fixed)+`" git-hook`) {
		t.Fatalf("pre-commit = %q, %v; want one exec into the published binary", hook, err)
	}

	// Every rendered hook, global or repo, names the published binary, and no file registers the guard twice.
	named := map[string][]string{}
	for path, binaries := range hookCommands(t, home) {
		named[path] = binaries
	}
	for path, binaries := range hookCommands(t, clone) {
		named[path] = binaries
	}
	if len(named) == 0 {
		t.Fatal("no rendered file registers the guard")
	}
	for path, binaries := range named {
		if len(binaries) != 1 || binaries[0] != fixed {
			t.Fatalf("%s names %q; every hook must name %s once", path, binaries, fixed)
		}
	}

	// The guard lets normal work through: edits, a commit, and a push of the feature branch.
	calls := []map[string]any{
		{"tool_name": "Edit", "tool_input": map[string]any{"file_path": filepath.Join(clone, "README.md")}},
		{"tool_name": "Write", "tool_input": map[string]any{"file_path": filepath.Join(clone, "notes.md")}},
		{"tool_name": "Bash", "tool_input": map[string]any{"command": "git commit -m 'docs: a note'"}},
		{"tool_name": "Bash", "tool_input": map[string]any{"command": "git push origin feat/out-of-box"}},
	}
	for _, call := range calls {
		call["hook_event_name"], call["cwd"], call["session_id"] = "PreToolUse", clone, "out-of-box"
		payload, err := json.Marshal(call)
		if err != nil {
			t.Fatal(err)
		}
		guard := exec.Command(fixed, "guard")
		guard.Dir, guard.Stdin = clone, strings.NewReader(string(payload))
		out, err := guard.CombinedOutput()
		if err != nil || strings.Contains(string(out), "deny") {
			t.Fatalf("the guard refused %s: %v\n%s", payload, err, out)
		}
	}

	// The same check still refuses what it must, so a pass above is the guard allowing, not failing open.
	refused, err := json.Marshal(map[string]any{"hook_event_name": "PreToolUse", "tool_name": "Bash", "cwd": clone,
		"session_id": "out-of-box", "tool_input": map[string]any{"command": "git push origin main"}})
	if err != nil {
		t.Fatal(err)
	}
	guard := exec.Command(fixed, "guard")
	guard.Dir, guard.Stdin = clone, strings.NewReader(string(refused))
	if out, _ := guard.CombinedOutput(); !strings.Contains(string(out), "deny") {
		t.Fatalf("a push to main was allowed: %s", out)
	}

	// Nothing is left behind: no stash, no extra worktree, and only ignored files beside the clone's source.
	if stashes := strings.TrimSpace(runIn(t, clone, "git", "stash", "list")); stashes != "" {
		t.Fatalf("stashes = %q, want none", stashes)
	}
	if worktrees := strings.Count(strings.TrimSpace(runIn(t, clone, "git", "worktree", "list")), "\n"); worktrees != 0 {
		t.Fatalf("install cut %d worktrees, want none", worktrees)
	}
	if status := strings.TrimSpace(runIn(t, clone, "git", "status", "--porcelain")); status != "" {
		t.Fatalf("status = %q; install must leave the clone clean but for ignored files", status)
	}
}

// TestSessionStartStaysUnderItsBudget proves the status hook and one guard call each run in under 100 ms.
func TestSessionStartStaysUnderItsBudget(t *testing.T) {
	if testing.Short() {
		t.Skip("times a built binary")
	}
	binary := gateBinary(t)
	root := t.TempDir()
	gitCommand(t, root, "init", "-q", "-b", "main")
	measure := func(stdin string, args ...string) time.Duration {
		var runs []time.Duration
		for range 5 {
			cmd := exec.Command(binary, args...)
			cmd.Dir, cmd.Stdin = root, strings.NewReader(stdin)
			started := time.Now()
			_ = cmd.Run()
			runs = append(runs, time.Since(started))
		}
		slices.Sort(runs)
		return runs[len(runs)/2]
	}
	read := `{"hook_event_name":"PreToolUse","tool_name":"Read","tool_input":{"file_path":"x"},"cwd":"` + root + `"}`
	for name, took := range map[string]time.Duration{
		"status hook": measure(`{"hook_event_name":"SessionStart","cwd":"`+root+`"}`, "hook", "status", "--host", "claude"),
		"guard":       measure(read, "guard"),
	} {
		if took > 100*time.Millisecond {
			t.Errorf("%s took %v at the median of 5; the budget is 100ms", name, took)
		}
	}
}
