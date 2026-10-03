package preflight

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"komodo/internal/mount"
)

// fakeHostContract is a mock host contract for testing.
type fakeHostContract struct {
	preflightErr error
}

func (f *fakeHostContract) Preflight() error { return f.preflightErr }

// write puts one file into a fixture repo.
func write(t *testing.T, root, rel, body string) {
	t.Helper()
	path := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// clean builds a fixture repo that doctor passes.
func clean(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	write(t, root, "AGENTS.md", "# Rules\n")
	write(t, root, ".gitattributes", "* text=auto eol=lf\n")
	write(t, root, "komodo/AGENTS.md", "# Agent Rules\n")
	write(t, root, "komodo/roles/builder.md", "---\nname: builder\ndescription: Builds.\ntier: standard\n"+
		"tools: [read, edit, write]\nsession: true\nreturns: schema.json\n---\n\nBody.\n")
	write(t, root, "komodo/roles/schema.json", "{}")

	// Set up the profile to use the fake host.
	write(t, root, ".komodo/profile.json", `{"host":"fake"}`)

	// Each platform's sandbox tool is on PATH, so only a test that clears PATH fails the sandbox check.
	tools := t.TempDir()
	for _, name := range []string{"sandbox-exec", "bwrap"} {
		write(t, tools, name, "#!/bin/sh\n")
		if err := os.Chmod(filepath.Join(tools, name), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", tools+string(os.PathListSeparator)+os.Getenv("PATH"))

	return root
}

// registerHostContract sets up a host contract for testing.
func registerHostContract(t *testing.T, contract HostContract) {
	t.Helper()
	old := hostFallible
	t.Cleanup(func() { hostFallible = old })
	SetHostFallible(contract)
}

// TestHostLoginSuccess checks that a successful host preflight passes.
func TestHostLoginSuccess(t *testing.T) {
	root := clean(t)
	contract := &fakeHostContract{}
	registerHostContract(t, contract)

	failures, err := Run(root, Options{NoShip: true})
	if err != nil {
		t.Fatalf("Run = %v", err)
	}
	if len(failures) != 0 {
		t.Fatalf("expected no failures, got %v", failures)
	}
}

// TestHostLoginFailure checks that a failed host preflight is reported.
func TestHostLoginFailure(t *testing.T) {
	root := clean(t)
	contract := &fakeHostContract{preflightErr: errors.New("not logged in")}
	registerHostContract(t, contract)

	failures, err := Run(root, Options{NoShip: true})
	if err != nil {
		t.Fatalf("Run = %v", err)
	}
	if len(failures) == 0 {
		t.Fatal("expected a login failure")
	}
	var found bool
	for _, failure := range failures {
		if failure.Name == "host login" {
			found = true
			if failure.Fix != "not logged in" {
				t.Fatalf("fix = %q, want 'not logged in'", failure.Fix)
			}
		}
	}
	if !found {
		t.Fatalf("expected host login failure, got %v", failures)
	}
}

// TestForgeCredentialSkippedWithNoShip checks that --no-ship skips the forge credential check.
func TestForgeCredentialSkippedWithNoShip(t *testing.T) {
	root := clean(t)
	contract := &fakeHostContract{}
	registerHostContract(t, contract)

	failures, err := Run(root, Options{NoShip: true})
	if err != nil {
		t.Fatalf("Run = %v", err)
	}
	for _, failure := range failures {
		if failure.Name == "forge credential" {
			t.Fatalf("forge credential check should be skipped with NoShip, but got %v", failure)
		}
	}
}

// TestEachCheckNamesItsFixOnFailure checks that each check names its fix when it fails.
func TestEachCheckNamesItsFixOnFailure(t *testing.T) {
	root := clean(t)
	contract := &fakeHostContract{preflightErr: errors.New("claude not authenticated")}
	registerHostContract(t, contract)

	failures, err := Run(root, Options{NoShip: true})
	if err != nil {
		t.Fatalf("Run = %v", err)
	}

	// Check that each failure has a name and a fix.
	for _, failure := range failures {
		if failure.Name == "" {
			t.Fatal("failure has empty name")
		}
		if failure.Fix == "" {
			t.Fatalf("failure %q has empty fix", failure.Name)
		}
	}
}

// TestDoctorFailureIsReported checks that doctor failures are reported.
func TestDoctorFailureIsReported(t *testing.T) {
	root := clean(t)
	// Create an invalid role to trigger doctor failure.
	write(t, root, "komodo/roles/broken.md", "---\nname: broken\ndescription: x\ntier: invalid\n"+
		"tools: [read]\nsession: true\nreturns: schema.json\n---\n\nBody.\n")

	contract := &fakeHostContract{}
	registerHostContract(t, contract)

	failures, err := Run(root, Options{NoShip: true})
	if err != nil {
		t.Fatalf("Run = %v", err)
	}

	// Check that a doctor failure was reported.
	var found bool
	for _, failure := range failures {
		if failure.Name == "doctor: roles" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected doctor failure, got %v", failures)
	}
}

// hostLoginFailed runs preflight with one registered host and reports whether the login check failed.
func hostLoginFailed(t *testing.T, host mount.Host) bool {
	t.Helper()
	root := clean(t)
	snapshot := mount.Snapshot()
	t.Cleanup(func() { mount.Restore(snapshot) })
	mount.Register(host)
	failures, err := Run(root, Options{NoShip: true})
	if err != nil {
		t.Fatalf("Run = %v", err)
	}
	for _, failure := range failures {
		if failure.Name == "host login" {
			return true
		}
	}
	return false
}

func TestHostLoginFailsWhenTheHostReportsNoLogin(t *testing.T) {
	failed := hostLoginFailed(t, mount.Host{
		Name:      "loggedout",
		Installed: func(string) bool { return true },
		Probe:     func() (mount.Usage, bool) { return mount.Usage{Plan: "max_5x"}, true },
		LoggedIn:  func() (bool, error) { return false, nil },
	})
	if !failed {
		t.Fatal("a host that reports no login must fail the login check")
	}
}

func TestHostLoginPassesKeyBillingWhoseProbeFails(t *testing.T) {
	failed := hostLoginFailed(t, mount.Host{
		Name:      "keybilled",
		Installed: func(string) bool { return true },
		Probe:     func() (mount.Usage, bool) { return mount.Usage{}, false },
		LoggedIn:  func() (bool, error) { return true, nil },
	})
	if failed {
		t.Fatal("key billing has no plan to probe, but its login holds; the login check must pass")
	}
}

// sandboxFailed runs preflight and reports whether the sandbox check failed.
func sandboxFailed(t *testing.T, root string) bool {
	t.Helper()
	registerHostContract(t, &fakeHostContract{})
	failures, err := Run(root, Options{NoShip: true})
	if err != nil {
		t.Fatalf("Run = %v", err)
	}
	for _, failure := range failures {
		if failure.Name == "sandbox" {
			return true
		}
	}
	return false
}

// withPlatform runs the sandbox check as if on goos for one test.
func withPlatform(t *testing.T, platform string) {
	t.Helper()
	old := goos
	t.Cleanup(func() { goos = old })
	goos = platform
}

func TestSandboxPassesWhereThePlatformsToolIsOnPath(t *testing.T) {
	for _, platform := range []string{"darwin", "linux"} {
		t.Run(platform, func(t *testing.T) {
			withPlatform(t, platform)
			if sandboxFailed(t, clean(t)) {
				t.Fatalf("%s with its sandbox tool on PATH must pass", platform)
			}
		})
	}
}

func TestSandboxFailsWhenNoSandboxToolIsOnPath(t *testing.T) {
	for _, platform := range []string{"darwin", "linux"} {
		t.Run(platform, func(t *testing.T) {
			withPlatform(t, platform)
			root := clean(t)
			t.Setenv("PATH", t.TempDir())
			if !sandboxFailed(t, root) {
				t.Fatalf("%s with no sandbox tool must refuse the run", platform)
			}
		})
	}
}

func TestSandboxWarnsInsteadOfFailingOnNativeWindows(t *testing.T) {
	withPlatform(t, "windows")
	root := clean(t)
	registerHostContract(t, &fakeHostContract{})
	var notes []string
	failures, err := Run(root, Options{NoShip: true, Warn: func(note string) { notes = append(notes, note) }})
	if err != nil {
		t.Fatalf("Run = %v", err)
	}
	for _, failure := range failures {
		if failure.Name == "sandbox" {
			t.Fatalf("native Windows has no sandbox, but it must warn, not refuse: %v", failures)
		}
	}
	if len(notes) != 1 || !strings.Contains(notes[0], "Windows") {
		t.Fatalf("notes = %v, want one naming Windows", notes)
	}
}

// withProcVersion points the WSL check at a file holding body, in place of the real /proc/version.
func withProcVersion(t *testing.T, body string) {
	t.Helper()
	old := procVersionPath
	t.Cleanup(func() { procVersionPath = old })
	path := filepath.Join(t.TempDir(), "version")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	procVersionPath = path
}

func TestSandboxRefusesAWSL2RepoUnderMnt(t *testing.T) {
	withPlatform(t, "linux")
	withProcVersion(t, "Linux version 5.15.90.1-microsoft-standard-WSL2")
	err := checkSandbox("/mnt/c/src/repo", Options{})
	if err == nil || !strings.Contains(err.Error(), "Linux home") {
		t.Fatalf("err = %v, want it to refuse and name the Linux home", err)
	}
}

func TestSandboxPassesAWSL2RepoOffMnt(t *testing.T) {
	withPlatform(t, "linux")
	withProcVersion(t, "Linux version 5.15.90.1-microsoft-standard-WSL2")
	tools := t.TempDir()
	write(t, tools, "bwrap", "#!/bin/sh\n")
	if err := os.Chmod(filepath.Join(tools, "bwrap"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", tools+string(os.PathListSeparator)+os.Getenv("PATH"))
	if err := checkSandbox("/home/a/src/repo", Options{}); err != nil {
		t.Fatalf("err = %v; a WSL2 repo off /mnt, with bwrap on PATH, must pass", err)
	}
}

func TestAnOverlayCannotTurnTheSandboxOff(t *testing.T) {
	withPlatform(t, "linux")
	root := clean(t)
	home := t.TempDir()
	t.Setenv("HOME", home)
	write(t, home, ".komodo/config.json", `{"sandbox":false}`)
	t.Setenv("PATH", t.TempDir())
	if !sandboxFailed(t, root) {
		t.Fatal("an overlay with the sandbox off skipped the check; the run must still be refused")
	}
}

// TestCheckToolsNamesTheFixForEachMissingTool proves a missing git, gh or go fails preflight
// with the fix named, instead of a confusing failure further down the checks.
func TestCheckToolsNamesTheFixForEachMissingTool(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	failures := checkTools()
	if len(failures) != 3 {
		t.Fatalf("failures = %+v, want one per missing tool", failures)
	}
	for _, name := range []string{"git", "gh", "go"} {
		found := false
		for _, failure := range failures {
			if failure.Name == name && failure.Fix != "" {
				found = true
			}
		}
		if !found {
			t.Fatalf("failures = %+v, missing a named fix for %s", failures, name)
		}
	}
}

// TestCheckToolsPassesWhenEveryToolIsOnPath proves a host with git, gh and go on PATH has no failure.
func TestCheckToolsPassesWhenEveryToolIsOnPath(t *testing.T) {
	if len(checkTools()) != 0 {
		t.Skip("this host is missing a required tool")
	}
}

// TestCheckForgeCredentialKillsAHungGhAndNamesTheTimeout proves a gh auth status past toolTimeout
// is killed, process group included, and the error names the command and the timeout.
func TestCheckForgeCredentialKillsAHungGhAndNamesTheTimeout(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "gh"), []byte("#!/bin/sh\nsleep 30 &\nsleep 30\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	saved := toolTimeout
	toolTimeout = 200 * time.Millisecond
	t.Cleanup(func() { toolTimeout = saved })
	started := time.Now()
	err := checkForgeCredential()
	if err == nil || !strings.Contains(err.Error(), "timed out") {
		t.Fatalf("err = %v, want it to name the timeout", err)
	}
	if time.Since(started) > 5*time.Second {
		t.Fatalf("the kill took %s; the group was not killed", time.Since(started))
	}
}

// TestRunStopsOnFirstFailure implicitly checks that failures are returned; the test framework
// would fail if Run did not stop checking on the first failure.
func TestRunStopsOnFirstFailure(t *testing.T) {
	root := clean(t)
	contract := &fakeHostContract{preflightErr: errors.New("not logged in")}
	registerHostContract(t, contract)

	failures, err := Run(root, Options{NoShip: true})
	if err != nil {
		t.Fatalf("Run = %v", err)
	}

	// Checks that the failures include a host login entry.
	var found bool
	for _, failure := range failures {
		if failure.Name == "host login" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected host login failure in preflight checks")
	}
}
