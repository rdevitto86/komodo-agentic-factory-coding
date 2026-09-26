package doctor

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"komodo/internal/git"
	"komodo/internal/mount"
)

// pinnedHost registers a fake, installed host with full model IDs on every tier and restores the
// registry and the version probe after the test.
func pinnedHost(t *testing.T, name, version, reportedVersion string) {
	t.Helper()
	registerHost(t, mount.Host{
		Name:      name,
		Version:   version,
		Installed: func(string) bool { return true },
		Tiers: func(string, bool) mount.Tiers {
			machine := mount.Machine{Provider: name, Model: "claude-sonnet-5"}
			return mount.Tiers{Light: machine, Standard: machine, Heavy: machine, Reviewer: machine}
		},
	})
	swapCLIVersion(t, func(got string) (string, error) {
		if got != name {
			t.Fatalf("cliVersion probed %q, not %q", got, name)
		}
		return reportedVersion, nil
	})
}

// swapCLIVersion overrides the CLI version probe for one test and restores it after.
func swapCLIVersion(t *testing.T, fn func(string) (string, error)) {
	t.Helper()
	previous := cliVersion
	cliVersion = fn
	t.Cleanup(func() { cliVersion = previous })
}

// swapGoToolchain overrides the running Go toolchain for one test and restores it after.
func swapGoToolchain(t *testing.T, version string) {
	t.Helper()
	previous := goToolchain
	goToolchain = func() string { return version }
	t.Cleanup(func() { goToolchain = previous })
}

func TestACleanRepoHasNoPins(t *testing.T) {
	if got := problemsFrom(t, clean(t))["pins"]; len(got) != 0 {
		t.Fatalf("pins = %+v", got)
	}
}

func TestAHostVersionThatDiffersIsFound(t *testing.T) {
	root := clean(t)
	pinnedHost(t, "testhost", "9.9.9", "1.0.0")
	got := problemsFrom(t, root)["pins"]
	found := false
	for _, problem := range got {
		if problem.Where == "testhost" && strings.Contains(problem.Detail, "reports 1.0.0") &&
			strings.Contains(problem.Detail, "pinned to 9.9.9") {
			found = true
		}
	}
	if !found {
		t.Fatalf("pins = %+v", got)
	}
}

func TestAHostVersionProbeFailureIsFound(t *testing.T) {
	root := clean(t)
	registerHost(t, mount.Host{
		Name:      "testhost",
		Version:   "9.9.9",
		Installed: func(string) bool { return true },
		Tiers: func(string, bool) mount.Tiers {
			machine := mount.Machine{Provider: "testhost", Model: "claude-sonnet-5"}
			return mount.Tiers{Light: machine, Standard: machine, Heavy: machine, Reviewer: machine}
		},
	})
	swapCLIVersion(t, func(string) (string, error) { return "", os.ErrNotExist })
	got := problemsFrom(t, root)["pins"]
	if len(got) != 1 || !strings.Contains(got[0].Detail, "could not read its version") {
		t.Fatalf("pins = %+v", got)
	}
}

func TestABareModelAliasIsFound(t *testing.T) {
	root := clean(t)
	write(t, root, "komodo/profiles/full.json", `{"roles":{"builder":{"tier":"standard","effort":"medium"}}}`)
	registerHost(t, mount.Host{
		Name:      "testhost",
		Version:   "1.0.0",
		Installed: func(string) bool { return true },
		Tiers: func(string, bool) mount.Tiers {
			machine := mount.Machine{Provider: "testhost", Model: "latest"}
			return mount.Tiers{Light: machine, Standard: machine, Heavy: machine, Reviewer: machine}
		},
	})
	swapCLIVersion(t, func(string) (string, error) { return "1.0.0", nil })
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
