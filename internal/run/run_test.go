package run

import (
	"bytes"
	"os"
	"strings"
	"testing"
	"time"
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
