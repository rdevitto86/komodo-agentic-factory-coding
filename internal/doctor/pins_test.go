package doctor

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"komodo/internal/gate"
	"komodo/internal/git"
	"komodo/internal/mount"
)

// swapGoToolchain overrides the running Go toolchain for one test and restores it after.
func swapGoToolchain(t *testing.T, version string) {
	t.Helper()
	previous := goToolchain
	goToolchain = func(string) (string, error) { return version, nil }
	t.Cleanup(func() { goToolchain = previous })
}

func TestACleanRepoHasNoPins(t *testing.T) {
	if got := problemsFrom(t, clean(t))["pins"]; len(got) != 0 {
		t.Fatalf("pins = %+v", got)
	}
}

func TestABareModelAliasIsFound(t *testing.T) {
	root := clean(t)
	write(t, root, "komodo/profiles/full.json", `{"roles":{"builder":{"tier":"standard","effort":"medium"}}}`)
	registerHost(t, mount.Host{
		Name:      "testhost",
		Installed: func(string) bool { return true },
		Tiers: func(string, bool) mount.Tiers {
			machine := mount.Machine{Provider: "testhost", Model: "latest"}
			return mount.Tiers{Light: machine, Standard: machine, Heavy: machine, Reviewer: machine}
		},
	})
	got := problemsFrom(t, root)["pins"]
	found := false
	for _, problem := range got {
		if strings.Contains(problem.Detail, `"latest" is not a full ID`) {
			found = true
		}
	}
	if !found {
		t.Fatalf("pins = %+v", got)
	}
}

func TestAReviewerAliasIsFoundWithNoRoleOnThatTier(t *testing.T) {
	root := clean(t)
	write(t, root, "komodo/profiles/full.json", `{"roles":{"builder":{"tier":"standard","effort":"medium"}}}`)
	registerHost(t, mount.Host{
		Name:      "testhost",
		Installed: func(string) bool { return true },
		Tiers: func(string, bool) mount.Tiers {
			full := mount.Machine{Provider: "testhost", Model: "testhost-1"}
			return mount.Tiers{Light: full, Standard: full, Heavy: full,
				Reviewer: mount.Machine{Provider: "testhost", Model: "opus"}}
		},
	})
	got := problemsFrom(t, root)["pins"]
	found := false
	for _, problem := range got {
		if problem.Where == "reviewer" && strings.Contains(problem.Detail, `"opus" is not a full ID`) {
			found = true
		}
	}
	if !found {
		t.Fatalf("pins = %+v", got)
	}
}

func TestAToolchainThatDiffersIsFound(t *testing.T) {
	root := clean(t)
	write(t, root, "go.mod", "module fixture\n\ngo 1.22\n\ntoolchain go1.27.1\n")
	swapGoToolchain(t, "go1.20.0")
	got := problemsFrom(t, root)["pins"]
	found := false
	for _, problem := range got {
		if problem.Where == "go.mod" && strings.Contains(problem.Detail, "pinned to go1.27.1") &&
			strings.Contains(problem.Detail, "runs go1.20.0") {
			found = true
		}
	}
	if !found {
		t.Fatalf("pins = %+v", got)
	}
}

// TestAToolchainReadFailureIsFoundNotSilentlySkipped proves checkToolchain reports goToolchain's
// error, instead of goToolchain returning "" and the problem going unseen.
func TestAToolchainReadFailureIsFoundNotSilentlySkipped(t *testing.T) {
	root := clean(t)
	write(t, root, "go.mod", "module fixture\n\ngo 1.22\n\ntoolchain go1.27.1\n")
	previous := goToolchain
	goToolchain = func(string) (string, error) { return "", fmt.Errorf("go env GOVERSION: boom") }
	t.Cleanup(func() { goToolchain = previous })
	got := problemsFrom(t, root)["pins"]
	found := false
	for _, problem := range got {
		if problem.Where == "go.mod" && strings.Contains(problem.Detail, "could not read this machine's Go toolchain") &&
			strings.Contains(problem.Detail, "boom") {
			found = true
		}
	}
	if !found {
		t.Fatalf("pins = %+v", got)
	}
}

func TestAMatchingToolchainHasNoPin(t *testing.T) {
	root := clean(t)
	write(t, root, "go.mod", "module fixture\n\ngo 1.22\n\ntoolchain go1.27.1\n")
	swapGoToolchain(t, "go1.27.1")
	if got := problemsFrom(t, root)["pins"]; len(got) != 0 {
		t.Fatalf("pins = %+v", got)
	}
}

func TestAStaleBuiltBinaryIsFound(t *testing.T) {
	root := gitRepo(t)
	write(t, root, "AGENTS.md", "# Rules\n")
	commitAll(t, root, "init")
	head, err := git.Run(root, "rev-parse", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	write(t, root, filepath.Join("bin", ".built-from"), "0000000000000000000000000000000000000000\n")
	got := checkRelease(root)
	if len(got) != 1 || !strings.Contains(got[0].Detail, head) {
		t.Fatalf("release pin = %+v, HEAD = %s", got, head)
	}
}

func TestAGoChangeSinceTheBuiltCommitIsFound(t *testing.T) {
	root := gitRepo(t)
	write(t, root, "AGENTS.md", "# Rules\n")
	commitAll(t, root, "init")
	built, err := git.Run(root, "rev-parse", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	write(t, root, filepath.Join("bin", gate.BuiltFrom), built+"\n")
	write(t, root, "main.go", "package main\n\nfunc main() {}\n")
	commitAll(t, root, "add a go file")
	head, err := git.Run(root, "rev-parse", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	got := checkRelease(root)
	if len(got) != 1 || !strings.Contains(got[0].Detail, head) {
		t.Fatalf("release pin = %+v, HEAD = %s", got, head)
	}
}

func TestADocsOnlyChangeSinceTheBuiltCommitHasNoPin(t *testing.T) {
	root := gitRepo(t)
	write(t, root, "AGENTS.md", "# Rules\n")
	commitAll(t, root, "init")
	built, err := git.Run(root, "rev-parse", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	write(t, root, filepath.Join("bin", gate.BuiltFrom), built+"\n")
	write(t, root, "docs/notes.md", "notes\n")
	commitAll(t, root, "docs only")
	if got := checkRelease(root); len(got) != 0 {
		t.Fatalf("release pin = %+v, want none for a docs-only commit", got)
	}
}

func TestABuiltBinaryFromHeadHasNoPin(t *testing.T) {
	root := gitRepo(t)
	write(t, root, "AGENTS.md", "# Rules\n")
	commitAll(t, root, "init")
	head, err := git.Run(root, "rev-parse", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	write(t, root, filepath.Join("bin", ".built-from"), head+"\n")
	if got := checkRelease(root); len(got) != 0 {
		t.Fatalf("release pin = %+v", got)
	}
}

func TestThePinnedReleaseCheck(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the installed stand-in is a shell script")
	}
	cases := []struct {
		name      string
		pin       string
		installed string
		toolkit   bool
		want      string
	}{
		{name: "no pin"},
		{name: "the pinned release installed", pin: "v1.0.0-beta.2", installed: "echo 'komodo 1.0.0-beta.2 (abc)'"},
		{name: "another release installed", pin: "1.0.0-beta.2", installed: "echo 'komodo 1.0.0-alpha.8 (abc)'",
			want: "runs komodo 1.0.0-alpha.8; the profile pins release 1.0.0-beta.2"},
		{name: "a binary that names no version", pin: "1.0.0-beta.2", installed: "echo komodo", want: "printed no version"},
		{name: "nothing installed", pin: "1.0.0-beta.2", want: "could not read its version"},
		{name: "the toolkit's own checkout", pin: "1.0.0-beta.2", toolkit: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := clean(t)
			write(t, root, "komodo/profiles/full.json", `{"release": "`+tc.pin+`", "roles": {}}`)
			if tc.toolkit {
				write(t, root, "cmd/komodo/main.go", "package main\n")
			}
			t.Setenv("HOME", t.TempDir())
			path, err := ReleaseBinary()
			if err != nil {
				t.Fatal(err)
			}
			if tc.installed != "" {
				write(t, filepath.Dir(path), filepath.Base(path), "#!/bin/sh\n"+tc.installed+"\n")
				if err := os.Chmod(path, 0o755); err != nil {
					t.Fatal(err)
				}
			}
			got := checkPinnedRelease(root, "full")
			if tc.want == "" {
				if len(got) != 0 {
					t.Fatalf("pins = %+v", got)
				}
				return
			}
			if len(got) != 1 || !strings.Contains(got[0].Detail, tc.want) {
				t.Fatalf("pins = %+v, want %q", got, tc.want)
			}
		})
	}
}

// TestReleaseVersionKillsAHungBinaryAndNamesTheTimeout proves a released binary past toolTimeout
// is killed, process group included, and the error names the command and the timeout.
func TestReleaseVersionKillsAHungBinaryAndNamesTheTimeout(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the stand-in is a shell script")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "komodo-fake")
	if err := os.WriteFile(path, []byte("#!/bin/sh\nsleep 30 &\nsleep 30\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	saved := toolTimeout
	toolTimeout = 200 * time.Millisecond
	t.Cleanup(func() { toolTimeout = saved })
	started := time.Now()
	if _, err := ReleaseVersion(path); err == nil || !strings.Contains(err.Error(), "timed out") {
		t.Fatalf("err = %v, want it to name the timeout", err)
	}
	if time.Since(started) > 5*time.Second {
		t.Fatalf("the kill took %s; the group was not killed", time.Since(started))
	}
}

func TestThePinnedReleaseCheckNamesAMissingHome(t *testing.T) {
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		t.Skip("only a Unix home comes from HOME alone")
	}
	root := clean(t)
	write(t, root, "komodo/profiles/full.json", `{"release": "1.0.0-beta.2", "roles": {}}`)
	t.Setenv("HOME", "")
	got := checkPinnedRelease(root, "full")
	if len(got) != 1 || !strings.Contains(got[0].Detail, "could not find the release binary") {
		t.Fatalf("pins = %+v", got)
	}
}

func TestPinnedReleaseIgnoresAMissingOrMalformedProfile(t *testing.T) {
	root := clean(t)
	if got := PinnedRelease(root, "full"); got != "" {
		t.Fatalf("pin = %q with no profile file", got)
	}
	write(t, root, "komodo/profiles/full.json", "{not json")
	if got := PinnedRelease(root, "full"); got != "" {
		t.Fatalf("pin = %q from a malformed profile", got)
	}
}
