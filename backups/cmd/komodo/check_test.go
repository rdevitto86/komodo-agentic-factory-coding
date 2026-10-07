package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"komodo/internal/backlog/backlogtest"
)

// checkBacklog is one group whose first task passes its check and whose second fails it.
const checkBacklog = "# Backlog\n\n### [TG-91.1] A checked group\n```yaml\ntype: feat\nversion: 3.0.0\n```\n\n" +
	"#### [TSK-91.1.1] Passes [P: C] [READY]\n```yaml\nfiles: [a/one.go]\ndone_when: [\"test -f a/one.go\"]\n```\n\n" +
	"#### [TSK-91.1.2] Fails [P: C] [READY]\n```yaml\nfiles: [b/two.go]\ndone_when: [\"test -f b/missing.go\"]\n```\n"

// checkRepo commits the group's own file on main, then edits a/one.go and c/stray.go in the
// working tree.
func checkRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	runGit(t, root, "init", "-b", "main")
	runGit(t, root, "config", "user.email", "a@example.com")
	runGit(t, root, "config", "user.name", "a")
	backlogtest.SeedText(t, root, checkBacklog)
	runGit(t, root, "add", "-A")
	runGit(t, root, "commit", "-m", "backlog")
	writeCheckFile(t, root, "a/one.go", "package a\n\nfunc One() {}\n")
	writeCheckFile(t, root, "c/stray.go", "package c\n")
	return root
}

// writeCheckFile writes one file under root, creating its directory.
func writeCheckFile(t *testing.T, root, name, body string) {
	t.Helper()
	path := filepath.Join(root, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestCheckRunsEachKindThroughOneEntryPoint(t *testing.T) {
	root := checkRepo(t)
	onChanged := `{"findings":[{"severity":"high","class":"bug","file":"a/one.go","line":3,"title":"on it","detail":"d"}]}`
	offChanged := `{"findings":[{"severity":"high","class":"bug","file":"docs/backlog/epic-91/tg-91.1/TG.md","line":1,"title":"off it","detail":"d"}]}`
	noLineOnChanged := `{"findings":[{"severity":"high","class":"test-gap","file":"a/one.go","title":"whole file","detail":"d"}]}`
	noLineOffChanged := `{"findings":[{"severity":"high","class":"test-gap",` +
		`"file":"docs/backlog/epic-91/tg-91.1/TG.md","title":"whole file off","detail":"d"}]}`
	results := t.TempDir()
	writeCheckFile(t, results, "on.json", onChanged)
	writeCheckFile(t, results, "off.json", offChanged)
	writeCheckFile(t, results, "noline-on.json", noLineOnChanged)
	writeCheckFile(t, results, "noline-off.json", noLineOffChanged)
	cases := []struct {
		name string
		args []string
		code int
		want string
	}{
		{"no kind", nil, 2, "usage: komodo check"},
		{"no target", []string{"task"}, 2, "usage: komodo check"},
		{"unknown kind", []string{"nonsense", "x"}, 2, `unknown check "nonsense"`},
		{"unknown task", []string{"task", "TSK-99.9.9"}, 1, "no task TSK-99.9.9"},
		{"task out of scope", []string{"task", "TSK-91.1.1", "--base", "main"}, 1, "c/stray.go is edited outside"},
		{"failing done_when", []string{"task", "TSK-91.1.2", "--base", "main"}, 1, "test -f b/missing.go"},
		{"task scope", []string{"scope", "TSK-91.1.1"}, 1, "c/stray.go is edited outside"},
		{"group scope", []string{"scope", "TG-91.1"}, 1, "1 problem(s)"},
		{"finding on a changed line", []string{"findings", filepath.Join(results, "on.json"), "--base", "main"}, 0, "0 problem(s)"},
		{"finding off the diff", []string{"findings", filepath.Join(results, "off.json")}, 1, "docs/backlog/epic-91/tg-91.1/TG.md:1 off it"},
		{
			"finding with no line on a changed file",
			[]string{"findings", filepath.Join(results, "noline-on.json"), "--base", "main"}, 0, "0 problem(s)",
		},
		{
			"finding with no line off the diff",
			[]string{"findings", filepath.Join(results, "noline-off.json")}, 1, "is not a file the diff changes",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := runCLI(t, root, "", append([]string{"check"}, tc.args...)...)
			if got.code != tc.code || !strings.Contains(got.stdout+got.stderr, tc.want) {
				t.Fatalf("code = %d, want %d; output lacks %q:\n%s%s", got.code, tc.code, tc.want, got.stdout, got.stderr)
			}
		})
	}
}

func TestCheckTaskPassesWhenItsCommandsAndScopeHold(t *testing.T) {
	root := checkRepo(t)
	if err := os.RemoveAll(filepath.Join(root, "c")); err != nil {
		t.Fatal(err)
	}
	got := runCLI(t, root, "", "check", "task", "TSK-91.1.1")
	if got.code != 0 || !strings.Contains(got.stdout, "0 problem(s)") {
		t.Fatalf("code = %d:\n%s%s", got.code, got.stdout, got.stderr)
	}
}
