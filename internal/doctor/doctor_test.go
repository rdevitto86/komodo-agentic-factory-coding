package doctor

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"komodo/internal/detect"
	"komodo/internal/mount"
)

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

// clean builds a fixture repo that every check passes.
func clean(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	write(t, root, "AGENTS.md", "# Rules\n\nSee `komodo/AGENTS.md`.\n")
	write(t, root, "komodo/AGENTS.md", "# Agent Rules\n\n{{accessibility}}\n")
	write(t, root, "komodo/rules/accessibility.md", "## Writing for a human\n- **Answer first.**\n")
	write(t, root, "komodo/roles/builder.md", "---\nname: builder\ndescription: Writes code.\ntier: standard\n"+
		"tools: [read, edit, write, shell, search]\nsession: true\nreturns: builder.schema.json\n---\n\nBody.\n")
	write(t, root, "komodo/roles/builder.schema.json", `{"type":"object","required":["result"]}`)
	write(t, root, "komodo/skills/run/SKILL.md", "---\nname: run\n---\n\nCall komodo step, do what it says, repeat.\n")
	write(t, root, "komodo/skills/standards-go/SKILL.md", "---\nname: standards-go\nglobs: [\"**/*.go\"]\n---\n\n# Go\n")
	return root
}

// problemsFrom returns the checks that fired.
func problemsFrom(t *testing.T, root string) map[string][]Problem {
	t.Helper()
	found, err := Run(root, Options{NoGit: true})
	if err != nil {
		t.Fatal(err)
	}
	out := map[string][]Problem{}
	for _, problem := range found {
		out[problem.Check] = append(out[problem.Check], problem)
	}
	return out
}

func TestACleanRepoHasNoProblems(t *testing.T) {
	if got := problemsFrom(t, clean(t)); len(got) != 0 {
		t.Fatalf("problems = %+v", got)
	}
}

func TestAReferenceThatResolvesToNothingIsFound(t *testing.T) {
	root := clean(t)
	write(t, root, "AGENTS.md", "# Rules\n\nSee `komodo/missing.md`.\n")
	got := problemsFrom(t, root)["references"]
	if len(got) != 1 || !strings.Contains(got[0].Detail, "resolves to nothing") {
		t.Fatalf("references = %+v", got)
	}
}

func TestAStandardsSkillMayNameALanguagesManifests(t *testing.T) {
	root := clean(t)
	write(t, root, "komodo/skills/standards-rust/SKILL.md",
		"---\nname: standards-rust\n---\n\n# Rust\n\nEvery crate has a `Cargo.toml`.\n")
	if got := problemsFrom(t, root)["references"]; len(got) != 0 {
		t.Fatalf("a language manifest was read as a repo path: %+v", got)
	}
}

func TestAMalformedRoleIsFound(t *testing.T) {
	root := clean(t)
	write(t, root, "komodo/roles/broken.md", "---\nname: broken\ndescription: x\ntier: enormous\n"+
		"tools: [read, telepathy]\nsession: true\nreturns: broken.schema.json\n---\n\nBody.\n")
	got := problemsFrom(t, root)["roles"]
	joined := ""
	for _, problem := range got {
		joined += problem.Detail + "\n"
	}
	for _, want := range []string{"is not light, standard, or heavy", "not one of the five Komodo verbs", "does not exist"} {
		if !strings.Contains(joined, want) {
			t.Errorf("roles do not report %q: %s", want, joined)
		}
	}
}

func TestAVendorNameOutsideTheMountsIsFound(t *testing.T) {
	mount.Register(mount.Host{Name: "testhost", Vendors: []string{"testvendor"}})
	root := clean(t)
	write(t, root, "internal/line/thing.go", "package line\n\n// uses testvendor directly\nvar x = 1\n")
	got := problemsFrom(t, root)["leaks"]
	if len(got) != 1 || !strings.Contains(got[0].Detail, "belongs inside internal/mount") {
		t.Fatalf("leaks = %+v", got)
	}
}

func TestAMountMayNameItsOwnVendor(t *testing.T) {
	mount.Register(mount.Host{Name: "testhost", Vendors: []string{"testvendor"}})
	root := clean(t)
	write(t, root, "internal/mount/testhost/testhost.go", "package testhost\n\n// testvendor lives here\nvar x = 1\n")
	if got := problemsFrom(t, root)["leaks"]; len(got) != 0 {
		t.Fatalf("a mount was reported for naming its own host: %+v", got)
	}
}

func TestATestFixtureIsNotALeak(t *testing.T) {
	mount.Register(mount.Host{Name: "testhost", Vendors: []string{"testvendor"}})
	root := clean(t)
	write(t, root, "internal/line/thing_test.go", "package line\n\n// testvendor as a fixture\nvar x = 1\n")
	if got := problemsFrom(t, root)["leaks"]; len(got) != 0 {
		t.Fatalf("a test fixture was reported: %+v", got)
	}
}

func TestAnOversizedStandardIsFound(t *testing.T) {
	root := clean(t)
	write(t, root, "komodo/skills/standards-big/SKILL.md",
		"---\nname: standards-big\n---\n\n"+strings.Repeat("rule. ", 2000))
	got := problemsFrom(t, root)["budgets"]
	if len(got) != 1 || !strings.Contains(got[0].Detail, "the cap is 8192") {
		t.Fatalf("budgets = %+v", got)
	}
}

func TestAnOversizedRunSkillIsFound(t *testing.T) {
	root := clean(t)
	write(t, root, "komodo/skills/run/SKILL.md", "---\nname: run\n---\n\n"+strings.Repeat("step. ", 1000))
	got := problemsFrom(t, root)["budgets"]
	if len(got) == 0 || !strings.Contains(got[0].Detail, "the cap is 800") {
		t.Fatalf("budgets = %+v", got)
	}
}

func TestAProfileThatDriftsFromTheTreeIsFound(t *testing.T) {
	root := clean(t)
	detect.Load(root)
	write(t, root, "go.mod", "module example\n")
	got := problemsFrom(t, root)["profile"]
	if len(got) != 1 || !strings.Contains(got[0].Detail, "differs from a fresh detection") {
		t.Fatalf("profile = %+v", got)
	}
}

func TestAProfileWithNoCacheYetIsNotFound(t *testing.T) {
	root := clean(t)
	if got := problemsFrom(t, root)["profile"]; len(got) != 0 {
		t.Fatalf("profile = %+v, want none before a detection has ever run", got)
	}
}

func TestOversizedAlwaysOnContextIsFound(t *testing.T) {
	root := clean(t)
	write(t, root, "AGENTS.md", strings.Repeat("rule. ", 2000))
	found := false
	for _, problem := range problemsFrom(t, root)["budgets"] {
		if problem.Where == "always-on context" {
			found = true
		}
	}
	if !found {
		t.Fatal("the always-on budget did not fire")
	}
}

func TestTokensCountFourCharacters(t *testing.T) {
	if tokens(400) != 100 {
		t.Fatalf("tokens = %d", tokens(400))
	}
}
