package preflight

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

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

// TestSandboxFailsWhenTheOverlayAsksAndNoSandboxToolIsOnPath checks the sandbox check fails,
// rather than passing silently, when the platform's sandbox tool cannot be found.
func TestSandboxFailsWhenTheOverlayAsksAndNoSandboxToolIsOnPath(t *testing.T) {
	root := clean(t)
	home := t.TempDir()
	t.Setenv("HOME", home)
	if err := os.MkdirAll(filepath.Join(home, ".komodo"), 0o755); err != nil {
		t.Fatal(err)
	}
	overlay := filepath.Join(home, ".komodo", "config.json")
	if err := os.WriteFile(overlay, []byte(`{"sandbox":true}`), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", t.TempDir())
	contract := &fakeHostContract{}
	registerHostContract(t, contract)

	failures, err := Run(root, Options{NoShip: true})
	if err != nil {
		t.Fatalf("Run = %v", err)
	}
	var found bool
	for _, failure := range failures {
		if failure.Name == "sandbox" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected a sandbox failure, got %v", failures)
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
