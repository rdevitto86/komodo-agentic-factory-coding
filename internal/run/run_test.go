package run

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"komodo/internal/line"
	"komodo/internal/mount"
	"komodo/internal/pr"
)

// env reads one key back out of a scrubbed environment.
func env(list []string, key string) (string, bool) {
	for _, entry := range list {
		if name, value, found := strings.Cut(entry, "="); found && name == key {
			return value, true
		}
	}
	return "", false
}

func TestScrubDropsEveryPushCredential(t *testing.T) {
	base := []string{
		"GH_TOKEN=secret", "GITHUB_TOKEN=secret", "GH_ENTERPRISE_TOKEN=secret",
		"GIT_ASKPASS=/bin/askpass", "SSH_AUTH_SOCK=/tmp/agent.sock", "PATH=/usr/bin",
	}
	scrubbed := Scrub(base)
	for _, key := range dropped {
		if _, found := env(scrubbed, key); found {
			t.Fatalf("%s survived the scrub", key)
		}
	}
	if path, _ := env(scrubbed, "PATH"); path != "/usr/bin" {
		t.Fatalf("PATH = %q, want the inherited value", path)
	}
}

func TestScrubLeavesGitUnableToPromptOrAuthenticate(t *testing.T) {
	scrubbed := Scrub([]string{"PATH=/usr/bin"})
	want := map[string]string{
		"GIT_TERMINAL_PROMPT": "0",
		"GIT_CONFIG_COUNT":    "1",
		"GIT_CONFIG_KEY_0":    "credential.helper",
		"GIT_CONFIG_VALUE_0":  "",
	}
	for key, value := range want {
		if got, found := env(scrubbed, key); !found || got != value {
			t.Fatalf("%s = %q, %v; want %q", key, got, found, value)
		}
	}
	if ssh, _ := env(scrubbed, "GIT_SSH_COMMAND"); !strings.Contains(ssh, "IdentitiesOnly=yes") {
		t.Fatalf("GIT_SSH_COMMAND = %q", ssh)
	}
}

func TestScrubIgnoresAnIdentityFileConfiguredOutsideTheEnvironment(t *testing.T) {
	scrubbed := Scrub([]string{"PATH=/usr/bin"})
	ssh, _ := env(scrubbed, "GIT_SSH_COMMAND")
	if !strings.Contains(ssh, "-F "+os.DevNull) {
		t.Fatalf("GIT_SSH_COMMAND = %q, want -F %s so ~/.ssh/config never applies", ssh, os.DevNull)
	}
}

func TestScrubDropsATokenByItsNameShapeNotJustAnExactSpelling(t *testing.T) {
	base := []string{
		"GITHUB_PAT=secret", "HOMEBREW_GITHUB_API_TOKEN=secret", "GIT_CONFIG_PARAMETERS=secret",
		"PATH=/usr/bin",
	}
	scrubbed := Scrub(base)
	for _, key := range []string{"GITHUB_PAT", "HOMEBREW_GITHUB_API_TOKEN", "GIT_CONFIG_PARAMETERS"} {
		if _, found := env(scrubbed, key); found {
			t.Fatalf("%s survived the scrub", key)
		}
	}
}

func TestScrubKeepsTheModelHostsOwnLoginAndDropsEveryForgeSecret(t *testing.T) {
	base := []string{"MODELHOST_OAUTH_TOKEN=login", "PATH=/usr/bin", "GITLAB_TOKEN=secret", "BITBUCKET_PASSWORD=secret"}
	scrubbed := Scrub(base)
	for _, key := range []string{"MODELHOST_OAUTH_TOKEN", "PATH"} {
		if _, found := env(scrubbed, key); !found {
			t.Fatalf("%s was scrubbed; only a forge's push credential may be", key)
		}
	}
	for _, key := range []string{"GITLAB_TOKEN", "BITBUCKET_PASSWORD"} {
		if _, found := env(scrubbed, key); found {
			t.Fatalf("%s survived the scrub", key)
		}
	}
}

func TestScrubDoesNotLetAnInheritedOverrideSurvive(t *testing.T) {
	scrubbed := Scrub([]string{"GIT_TERMINAL_PROMPT=1", "GIT_SSH_COMMAND=ssh -i /home/me/.ssh/id_ed25519"})
	if got, _ := env(scrubbed, "GIT_TERMINAL_PROMPT"); got != "0" {
		t.Fatalf("GIT_TERMINAL_PROMPT = %q, want the launcher's own value", got)
	}
	if ssh, _ := env(scrubbed, "GIT_SSH_COMMAND"); strings.Contains(ssh, "id_ed25519") {
		t.Fatalf("an inherited SSH identity survived: %q", ssh)
	}
	count := 0
	for _, entry := range scrubbed {
		if strings.HasPrefix(entry, "GIT_SSH_COMMAND=") {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("GIT_SSH_COMMAND appears %d times", count)
	}
}

func TestLaunchWithNoMountSaysToInstall(t *testing.T) {
	code, err := Launch(Options{Root: t.TempDir(), Target: "TG-01.1", DryRun: true})
	if code == 0 || err == nil {
		t.Fatalf("code = %d, err = %v; want a refusal", code, err)
	}
	if !strings.Contains(err.Error(), "komodo install") {
		t.Fatalf("err = %v", err)
	}
}

func TestLaunchKillsARunThatPassesItsBudget(t *testing.T) {
	if _, err := os.Stat("/bin/sleep"); err != nil {
		t.Skip("no sleep on this machine")
	}
	var out bytes.Buffer
	code, err := launch(Options{
		Root: t.TempDir(), Budget: 50 * time.Millisecond,
		Stdout: &out, Stderr: &out, Env: []string{"PATH=/usr/bin:/bin"},
	}, "/bin/sleep", []string{"30"})
	if code != 124 || err == nil {
		t.Fatalf("code = %d, err = %v; want the budget kill", code, err)
	}
}

func TestLaunchReturnsTheHostsExitCode(t *testing.T) {
	var out bytes.Buffer
	code, err := launch(Options{
		Root: t.TempDir(), Stdout: &out, Stderr: &out, Env: []string{"PATH=/usr/bin:/bin"},
	}, "/bin/sh", []string{"-c", "exit 7"})
	if err != nil {
		t.Fatal(err)
	}
	if code != 7 {
		t.Fatalf("code = %d, want 7", code)
	}
}

func TestLaunchTeesTheHeadlessJSONToTheHostsUsageEventsFile(t *testing.T) {
	if _, err := os.Stat("/bin/sh"); err != nil {
		t.Skip("no /bin/sh on this machine")
	}
	saved := mount.Snapshot()
	defer mount.Restore(saved)
	mount.Register(mount.Host{
		Name:      "codex",
		Installed: func(string) bool { return true },
		Headless: func(skill, target string) (string, []string) {
			return "/bin/sh", []string{"-c", `echo '{"type":"turn.completed"}'`}
		},
	})
	root := t.TempDir()
	var out, errOut bytes.Buffer
	code, err := Launch(Options{
		Root: root, Target: "TSK-01.1.1", Stdout: &out, Stderr: &errOut, Env: []string{"PATH=/usr/bin:/bin"},
	})
	if err != nil || code != 0 {
		t.Fatalf("code = %d, err = %v", code, err)
	}
	events, err := os.ReadFile(filepath.Join(root, line.StateDir, "codex", "TSK-01.1.1.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(events), "turn.completed") {
		t.Fatalf("events file = %q", events)
	}
	if !strings.Contains(out.String(), "turn.completed") {
		t.Fatalf("stdout lost the tee: %q", out.String())
	}
}

// runGit runs one git command in dir, failing the test on error.
func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v: %s", args, err, out)
	}
}

// writeHandoff writes a ship handoff file the way a scrubbed ship leaves it.
func writeHandoff(t *testing.T, root string, handoff line.ShipHandoff) {
	t.Helper()
	dir := filepath.Join(root, line.StateDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(handoff)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "ship.json"), data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestFinishShipPushesOpensThePullRequestThenStampsAndClearsTheHandoff(t *testing.T) {
	bare := filepath.Join(t.TempDir(), "origin.git")
	runGit(t, "", "init", "--bare", bare)
	root := t.TempDir()
	runGit(t, root, "init")
	runGit(t, root, "config", "user.email", "a@example.com")
	runGit(t, root, "config", "user.name", "a")
	runGit(t, root, "remote", "add", "origin", bare)
	runGit(t, root, "checkout", "-b", "feat/a-group")
	if err := os.WriteFile(filepath.Join(root, "one.go"), []byte("package a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, root, "add", "-A")
	runGit(t, root, "commit", "-m", "seed")
	writeHandoff(t, root, line.ShipHandoff{
		Branch: "feat/a-group", Base: "main", Title: "t", Body: "b", Labels: []string{"agent"},
	})
	created := false
	client := &pr.Client{Dir: root, Run: func(_ string, args ...string) (string, error) {
		switch {
		case len(args) > 1 && args[0] == "pr" && args[1] == "create":
			created = true
			return "https://example.invalid/pr/1", nil
		case len(args) > 0 && args[0] == "label":
			return "[]", nil
		}
		t.Fatalf("gh must not run any other command: %v", args)
		return "", nil
	}}
	if err := finishShip(Options{Root: root, PR: client}); err != nil {
		t.Fatal(err)
	}
	if !created {
		t.Fatal("gh pr create never ran")
	}
	if _, err := os.Stat(filepath.Join(root, line.StateDir, "ship.json")); !os.IsNotExist(err) {
		t.Fatalf("ship.json survived a finished ship: %v", err)
	}
	entries, err := line.Book(root).All()
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, entry := range entries {
		if entry.Station == "ship" && entry.Outcome == "done" {
			found = true
		}
	}
	if !found {
		t.Fatal("the ledger never recorded ship done")
	}
	out, err := exec.Command("git", "ls-remote", "--heads", bare, "feat/a-group").Output()
	if err != nil || len(out) == 0 {
		t.Fatalf("the branch never reached origin: %v %q", err, out)
	}
}

func TestFinishShipDoesNothingWithNoHandoff(t *testing.T) {
	if err := finishShip(Options{Root: t.TempDir()}); err != nil {
		t.Fatal(err)
	}
}
