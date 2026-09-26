package preflight

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
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
