package gate

import (
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"komodo/internal/mount"
)

// hookRepo is a committed repo, with a linked worktree beside it; toolkit makes it the toolkit's own checkout.
func hookRepo(t *testing.T, toolkit bool) (main, worktree string) {
	t.Helper()
	main = t.TempDir()
	gitCommand(t, main, "init", "-q", "-b", "main")
	module := "module other\n"
	if toolkit {
		module = "module komodo\n"
	}
	for path, body := range map[string]string{"go.mod": module, "cmd/komodo/main.go": "package main\n"} {
		if err := os.MkdirAll(filepath.Dir(filepath.Join(main, path)), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(main, path), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	gitCommand(t, main, "add", "-A")
	gitCommand(t, main, "-c", "user.email=a@example.com", "-c", "user.name=a", "commit", "-q", "-m", "seed")
	gitCommand(t, main, "-c", "user.email=a@example.com", "-c", "user.name=a", "commit", "-q", "--allow-empty", "-m", "two")
	worktree = filepath.Join(t.TempDir(), "wt")
	gitCommand(t, main, "worktree", "add", "-q", "--detach", worktree)
	return main, worktree
}

// argsOf flattens each step to its args, marking a rule check with a leading "rules".
func argsOf(steps []HookStep) [][]string {
	var out [][]string
	for _, step := range steps {
		args := step.Args
		if step.Rules {
			args = append([]string{"rules"}, args...)
		}
		out = append(out, args)
	}
	return out
}

func TestHookScriptIsOneExecIntoTheInstalledBinary(t *testing.T) {
	script := hookScript("/home/a/.komodo/bin/komodo")
	var commands []string
	for _, line := range strings.Split(strings.TrimSpace(script), "\n") {
		if !strings.HasPrefix(line, "#") {
			commands = append(commands, line)
		}
	}
	want := `exec "/home/a/.komodo/bin/komodo" git-hook "$(basename "$0")" "$@"`
	if len(commands) != 1 || commands[0] != want {
		t.Fatalf("commands = %q, want the one line %q", commands, want)
	}
	if got := hookScript(`C:\Users\a\.komodo\bin\komodo.exe`); runtime.GOOS == "windows" && !strings.Contains(got, "C:/Users/a/.komodo/bin/komodo.exe") {
		t.Fatalf("script = %q, want a forward-slash path git's sh can run", got)
	}
}

func TestAnInstalledHookExecsGitHookWithItsNameAndArguments(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the fake binary is a shell script")
	}
	binary, err := mount.HookPath()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(binary), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(binary, []byte("#!/bin/sh\necho ran \"$@\"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	gitDir := filepath.Join(t.TempDir(), ".git")
	written, err := Install(gitDir)
	if err != nil || len(written) != 7 {
		t.Fatalf("written = %v, %v; want the seven hooks", written, err)
	}
	out, err := exec.Command(filepath.Join(gitDir, "hooks", "commit-msg"), "MSG").CombinedOutput()
	if err != nil || strings.TrimSpace(string(out)) != "ran git-hook commit-msg MSG" {
		t.Fatalf("out = %q, %v; want the hook's name and argument passed through", out, err)
	}
}

func TestHookStepsPlanEveryHook(t *testing.T) {
	main, worktree := hookRepo(t, false)
	head := gitCommand(t, main, "rev-parse", "HEAD")
	parent := gitCommand(t, main, "rev-parse", "HEAD^1")
	twoRefs := "refs/heads/feat/a abc refs/heads/feat/a def\nrefs/heads/feat/b abc refs/heads/feat/b def\n"
	cases := []struct {
		name, hook, dir, stdin string
		args                   []string
		want                   [][]string
	}{
		{"commit-msg checks the trailer in the installed binary", "commit-msg", main, "", []string{"MSG"},
			[][]string{{"rules", "gate", "--commit-msg", "MSG"}}},
		{"pre-commit checks the branch, then gates without the tests", "pre-commit", main, "", nil,
			[][]string{{"rules", "gate", "--check-branch"}, {"gate", "--commit"}}},
		{"pre-push checks every pushed ref and scopes the gate to the first", "pre-push", main, twoRefs, nil,
			[][]string{{"rules", "gate", "--check-push", "refs/heads/feat/a"}, {"rules", "gate", "--check-push", "refs/heads/feat/b"},
				{"gate", "--fuzz", "10s", "--from", "def", "--to", "abc", "--at", "abc"}}},
		{"pre-push with no stdin fuzzes unscoped", "pre-push", main, "", nil, [][]string{{"gate", "--fuzz", "10s"}}},
		{"pre-push of a branch delete checks the ref and runs no gate", "pre-push", main,
			"(delete) " + zeroOID + " refs/heads/feat/gone abc\n", nil,
			[][]string{{"rules", "gate", "--check-push", "refs/heads/feat/gone"}}},
		{"post-commit rebuilds from the parent", "post-commit", main, "", nil,
			[][]string{{"gate", "--rebuild", "--from", parent, "--to", head}}},
		{"post-checkout of a branch rebuilds", "post-checkout", main, "", []string{"old", "new", "1"},
			[][]string{{"gate", "--rebuild", "--from", "old", "--to", "new"}}},
		{"post-checkout of a file never rebuilds", "post-checkout", main, "", []string{"old", "new", "0"}, nil},
		{"post-rewrite rebuilds across the whole span", "post-rewrite", main, "old1 new1 extra\nold2 new2\n", []string{"amend"},
			[][]string{{"gate", "--rebuild", "--from", "old1", "--to", "new2"}}},
		{"a worktree the harness cut never rebuilds on checkout", "post-checkout", worktree, "", []string{"old", "new", "1"}, nil},
		{"a worktree the harness cut never rebuilds on commit", "post-commit", worktree, "", nil, nil},
		{"a worktree the harness cut never rebuilds on rewrite", "post-rewrite", worktree, "old new\n", nil, nil},
		{"a worktree the harness cut never rebuilds on merge", "post-merge", worktree, "", nil, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			steps, err := HookSteps(tc.dir, tc.hook, tc.args, tc.stdin)
			if err != nil {
				t.Fatal(err)
			}
			if got := argsOf(steps); !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("steps = %q, want %q", got, tc.want)
			}
		})
	}
	if _, err := HookSteps(main, "commit-msg", nil, ""); err == nil {
		t.Fatal("a commit-msg hook with no message file must fail, not pass silently")
	}
}

func TestPostMergeRebuildsFromOrigHead(t *testing.T) {
	main, _ := hookRepo(t, false)
	head := gitCommand(t, main, "rev-parse", "HEAD")
	gitCommand(t, main, "update-ref", "ORIG_HEAD", "HEAD^1")
	parent := gitCommand(t, main, "rev-parse", "HEAD^1")
	steps, err := HookSteps(main, "post-merge", nil, "")
	if err != nil {
		t.Fatal(err)
	}
	if got, want := argsOf(steps), [][]string{{"gate", "--rebuild", "--from", parent, "--to", head}}; !reflect.DeepEqual(got, want) {
		t.Fatalf("steps = %q, want %q", got, want)
	}
}

func TestGateRunnerRunsTheToolkitsOwnSourceElseThisBinary(t *testing.T) {
	toolkit, worktree := hookRepo(t, true)
	top, err := filepath.EvalSymlinks(toolkit)
	if err != nil {
		t.Fatal(err)
	}
	if argv, dir := GateRunner(toolkit, "/bin/komodo"); !reflect.DeepEqual(argv, []string{"go", "run", "./cmd/komodo"}) || dir != top {
		t.Fatalf("toolkit runner = %q in %s, want go run in the checkout being committed", argv, dir)
	}
	if argv, _ := GateRunner(worktree, "/bin/komodo"); argv[0] != "go" {
		t.Fatalf("a toolkit worktree runner = %q, want its own source too", argv)
	}
	other, _ := hookRepo(t, false)
	if argv, dir := GateRunner(other, "/bin/komodo"); !reflect.DeepEqual(argv, []string{"/bin/komodo"}) || dir != other {
		t.Fatalf("a repo with its own cmd/komodo but another module ran %q in %s; want the installed binary", argv, dir)
	}
}
