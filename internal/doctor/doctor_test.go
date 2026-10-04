package doctor

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"komodo/internal/backlog"
	"komodo/internal/backlog/backlogtest"
	"komodo/internal/detect"
	"komodo/internal/install"
	"komodo/internal/ledger"
	"komodo/internal/line"
	"komodo/internal/mount"
	"komodo/internal/mount/ollama"
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
	t.Setenv("HOME", t.TempDir())
	root := t.TempDir()
	write(t, root, "AGENTS.md", "# Rules\n\nSee `komodo/AGENTS.md`.\n")
	write(t, root, ".gitattributes", "* text=auto eol=lf\n")
	write(t, root, "komodo/AGENTS.md", "# Agent Rules\n\n{{accessibility}}\n")
	write(t, root, "komodo/rules/accessibility.md", "## Writing for a human\n- **Answer first.**\n")
	write(t, root, "komodo/roles/builder.md", "---\nname: builder\ndescription: Writes code.\ntier: standard\n"+
		"tools: [read, edit, write, shell, search]\nsession: true\nreturns: builder.schema.json\n---\n\nBody.\n")
	write(t, root, "komodo/roles/builder.schema.json", `{"type":"object","required":["result"]}`)
	write(t, root, "komodo/skills/run/SKILL.md", "---\nname: run\n---\n\nCall komodo step, do what it says, repeat.\n")
	write(t, root, "komodo/skills/standards-go/SKILL.md", "---\nname: standards-go\nglobs: [\"**/*.go\"]\n---\n\n# Go\n")
	return root
}

// registerHost adds a fake mount for one test and restores the registry after it.
func registerHost(t *testing.T, host mount.Host) {
	t.Helper()
	snapshot := mount.Snapshot()
	t.Cleanup(func() { mount.Restore(snapshot) })
	mount.Register(host)
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

// TestAMalformedRepoConfigIsFound proves doctor names a present policy or labels file its reader
// refuses, and stays quiet when both are absent.
func TestAMalformedRepoConfigIsFound(t *testing.T) {
	root := t.TempDir()
	if got := checkConfig(root); len(got) != 0 {
		t.Fatalf("absent configs = %+v, want nothing", got)
	}
	if err := os.MkdirAll(filepath.Join(root, ".komodo"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, rel := range []string{filepath.Join(".komodo", "policy.json"), filepath.Join(".komodo", "labels.json")} {
		if err := os.WriteFile(filepath.Join(root, rel), []byte(`{"not_a_field": 1}`), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	got := checkConfig(root)
	if len(got) != 2 || got[0].Check != "config" || got[1].Check != "config" {
		t.Fatalf("problems = %+v, want one config problem per file", got)
	}
}

func TestAMalformedOverlayIsFound(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	if got := checkOverlay(path); len(got) != 0 {
		t.Fatalf("an absent overlay = %+v, want nothing", got)
	}
	for _, body := range []string{"", " \n"} {
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		if got := checkOverlay(path); len(got) != 0 {
			t.Fatalf("an empty overlay %q = %+v, want nothing", body, got)
		}
	}
	for _, body := range []string{`{"critical_refs": ["prod"],}`, `{"critical_refs": "prod"}`, `[]`, `{"max_parallel": "2"}`, `{"not_a_field": 1}`, `{} junk`} {
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		if got := checkOverlay(path); len(got) != 1 || got[0].Check != "overlay" {
			t.Fatalf("overlay %s = %+v, want one overlay problem", body, got)
		}
	}
	if err := os.WriteFile(path, []byte(`{"critical_refs": ["prod"], "sandbox": true}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := checkOverlay(path); len(got) != 0 {
		t.Fatalf("a well-formed overlay = %+v, want nothing", got)
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
	registerHost(t, mount.Host{Name: "testhost", Vendors: []string{"testvendor"}})
	root := clean(t)
	write(t, root, "internal/line/thing.go", "package line\n\n// uses testvendor directly\nvar x = 1\n")
	got := problemsFrom(t, root)["leaks"]
	if len(got) != 1 || !strings.Contains(got[0].Detail, "belongs inside internal/mount") {
		t.Fatalf("leaks = %+v", got)
	}
}

func TestAMountMayNameItsOwnVendor(t *testing.T) {
	registerHost(t, mount.Host{Name: "testhost", Vendors: []string{"testvendor"}})
	root := clean(t)
	write(t, root, "internal/mount/testhost/testhost.go", "package testhost\n\n// testvendor lives here\nvar x = 1\n")
	if got := problemsFrom(t, root)["leaks"]; len(got) != 0 {
		t.Fatalf("a mount was reported for naming its own host: %+v", got)
	}
}

func TestATestFixtureIsNotALeak(t *testing.T) {
	registerHost(t, mount.Host{Name: "testhost", Vendors: []string{"testvendor"}})
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
	if len(got) != 1 || !strings.Contains(got[0].Detail, "the cap is 6000") {
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
	write(t, root, "go.mod", "module example\n")
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

// alwaysOnFired reports whether the always-on context problem is among those found.
func alwaysOnFired(problems []Problem) bool {
	for _, problem := range problems {
		if problem.Where == "always-on context" {
			return true
		}
	}
	return false
}

// hostRenderingSkills fakes an installed host whose render carries exactly the given skills.
func hostRenderingSkills(root string, skills map[string]string) mount.Host {
	return mount.Host{Name: "testhost", Installed: func(string) bool { return true },
		Render: func(root, binary string) (install.Plan, error) {
			plan := install.Plan{Host: "testhost", Root: root}
			for name, body := range skills {
				plan.AddProject(filepath.Join(root, ".testhost", "skills", name, "SKILL.md"), []byte(body), "the "+name+" skill")
			}
			return plan, nil
		}}
}

// TestTheAlwaysOnBudgetTracksTheRenderedSkillsNotTheShippedOnes fails against the pre-fix
// checkBudgets, which sums every shipped skill and ignores what a host actually renders.
func TestTheAlwaysOnBudgetTracksTheRenderedSkillsNotTheShippedOnes(t *testing.T) {
	root := clean(t)
	huge := "---\nname: standards-huge\ndescription: " + strings.Repeat("word ", 1600) + "\n---\n\n# Huge\n"
	write(t, root, "komodo/skills/standards-huge/SKILL.md", huge)
	small := "---\nname: run\ndescription: three words here\n---\n\nBody.\n"

	registerHost(t, hostRenderingSkills(root, map[string]string{"run": small}))
	if got := problemsFrom(t, root)["budgets"]; alwaysOnFired(got) {
		t.Fatalf("budgets = %+v; a shipped skill the host never rendered must not count", got)
	}

	registerHost(t, hostRenderingSkills(root, map[string]string{"run": small, "standards-huge": huge}))
	if got := problemsFrom(t, root)["budgets"]; !alwaysOnFired(got) {
		t.Fatalf("budgets = %+v; a rendered skill's description must join the always-on total", got)
	}
}

func TestARolesScopedSkillIsNoPartOfTheAlwaysOnBudget(t *testing.T) {
	root := clean(t)
	huge := "---\nname: standards-huge\ndescription: " + strings.Repeat("word ", 1600) + "\n---\n\n# Huge\n"
	registerHost(t, mount.Host{Name: "testhost", Installed: func(string) bool { return true },
		Render: func(root, binary string) (install.Plan, error) {
			plan := install.Plan{Host: "testhost", Root: root}
			plan.AddScoped(filepath.Join(root, ".testhost", "plugins", "builder", "skills", "standards-huge", "SKILL.md"),
				[]byte(huge), "the builder's standard")
			return plan, nil
		}})
	if got := problemsFrom(t, root)["budgets"]; alwaysOnFired(got) {
		t.Fatalf("budgets = %+v; a skill only the builder's session loads must not count as always-on", got)
	}
}

func TestACreateAgainstAnAlreadyRenderedHostIsDrift(t *testing.T) {
	root := clean(t)
	rendered := filepath.Join(root, "existing.txt")
	write(t, root, "existing.txt", "old\n")
	registerHost(t, mount.Host{Name: "testhost", Render: func(root, binary string) (install.Plan, error) {
		plan := install.Plan{Host: "testhost", Root: root}
		plan.Add(rendered, []byte("old\n"), "kept in sync")
		plan.Add(filepath.Join(root, "missing.txt"), []byte("new\n"), "never rendered")
		return plan, nil
	}})
	got := problemsFrom(t, root)["drift"]
	if len(got) != 1 || got[0].Where != "missing.txt" || !strings.Contains(got[0].Detail, "run komodo install") {
		t.Fatalf("drift = %+v", got)
	}
}

func TestAHookNamingAMissingBinaryIsFoundAndIsDrift(t *testing.T) {
	root := clean(t)
	gone, _ := json.Marshal(filepath.Join(root, "gone", "komodo") + " guard")
	write(t, root, "settings.json", `{"command": `+string(gone)+`}`)
	registerHost(t, mount.Host{Name: "testhost", Render: func(root, binary string) (install.Plan, error) {
		plan := install.Plan{Host: "testhost", Root: root}
		plan.Add(filepath.Join(root, "settings.json"), []byte(`{"command": "/elsewhere/komodo-linux-amd64 guard"}`), "the guard")
		return plan, nil
	}})
	found := problemsFrom(t, root)
	if got := found["drift"]; len(got) != 1 || got[0].Where != "settings.json" {
		t.Fatalf("drift = %+v; a hook naming any binary but the rendered one is drift", got)
	}
	if got := found["hook"]; len(got) != 1 || got[0].Where != "settings.json" || !strings.Contains(got[0].Detail, "does not exist") {
		t.Fatalf("hook = %+v", got)
	}
}

func TestADeletedSeedFileIsNotDrift(t *testing.T) {
	root := clean(t)
	registerHost(t, mount.Host{Name: "testhost", Render: func(root, binary string) (install.Plan, error) {
		plan := install.Plan{Host: "testhost", Root: root}
		plan.AddSeed(filepath.Join(root, "seeded.local.json"), []byte("{}"), "the personal overlay")
		return plan, nil
	}})
	if got := problemsFrom(t, root); len(got) != 0 {
		t.Fatalf("problems = %+v; a seed file the user deleted must never count as drift", got)
	}
}

// TestCheckStaleSkillsNamesATrackedSkillThatDiffersAndSaysTheFix proves a skill git tracks, which
// install leaves alone, is still named when its body differs from the binary's own render.
func TestCheckStaleSkillsNamesATrackedSkillThatDiffersAndSaysTheFix(t *testing.T) {
	root := gitRepo(t)
	skillPath := filepath.Join(root, ".testhost", "skills", "run", "SKILL.md")
	write(t, root, filepath.Join(".testhost", "skills", "run", "SKILL.md"), "old body\n")
	commitAll(t, root, "init")
	registerHost(t, mount.Host{Name: "testhost", Installed: func(string) bool { return true },
		Render: func(root, binary string) (install.Plan, error) {
			plan := install.Plan{Host: "testhost", Root: root}
			plan.AddProject(skillPath, []byte("new body\n"), "the run skill")
			return plan, nil
		}})
	got := checkStaleSkills(renderInstalled(root, pinLocalDown))
	if len(got) != 1 || got[0].Where != filepath.Join(".testhost", "skills", "run", "SKILL.md") {
		t.Fatalf("checkStaleSkills = %+v", got)
	}
	if !strings.Contains(got[0].Detail, "git rm --cached") {
		t.Fatalf("detail = %q, want the fix command", got[0].Detail)
	}
}

// TestCheckStaleSkillsIgnoresAnUntrackedSkill proves checkDrift, not checkStaleSkills, names a skill
// that is merely stale because the repo never tracked it.
func TestCheckStaleSkillsIgnoresAnUntrackedSkill(t *testing.T) {
	root := clean(t)
	skillPath := filepath.Join(root, ".testhost", "skills", "run", "SKILL.md")
	write(t, root, filepath.Join(".testhost", "skills", "run", "SKILL.md"), "old body\n")
	registerHost(t, mount.Host{Name: "testhost", Installed: func(string) bool { return true },
		Render: func(root, binary string) (install.Plan, error) {
			plan := install.Plan{Host: "testhost", Root: root}
			plan.AddProject(skillPath, []byte("new body\n"), "the run skill")
			return plan, nil
		}})
	if got := checkStaleSkills(renderInstalled(root, pinLocalDown)); len(got) != 0 {
		t.Fatalf("checkStaleSkills = %+v, want none for an untracked skill; checkDrift covers that", got)
	}
}

func TestAPromisedAccessorWithNoRealCallerIsFound(t *testing.T) {
	root := clean(t)
	write(t, root, "komodo/rules/backlog.md", "# Backlog grammar\n\n## Rules\n"+
		"- **`severity_floor`** is how low a finding may sink before a review blocks it.\n")
	write(t, root, "internal/profile/floor.go", "package profile\n\nfunc SeverityFloor() int { return 0 }\n")
	write(t, root, "internal/profile/floor_test.go",
		"package profile\n\nimport \"testing\"\n\nfunc TestSeverityFloor(t *testing.T) { SeverityFloor() }\n")
	got := problemsFrom(t, root)["promises"]
	if len(got) != 1 || !strings.Contains(got[0].Detail, "SeverityFloor") ||
		!strings.Contains(got[0].Detail, "nothing outside its own tests calls it") {
		t.Fatalf("promises = %+v", got)
	}
}

func TestAPromisedAccessorWithARealCallerPasses(t *testing.T) {
	root := clean(t)
	write(t, root, "komodo/rules/backlog.md", "# Backlog grammar\n\n## Rules\n"+
		"- **`severity_floor`** is how low a finding may sink before a review blocks it.\n")
	write(t, root, "internal/profile/floor.go", "package profile\n\nfunc SeverityFloor() int { return 0 }\n")
	write(t, root, "internal/review/gate.go",
		"package review\n\nimport \"komodo/internal/profile\"\n\nfunc Gate() int { return profile.SeverityFloor() }\n")
	if got := problemsFrom(t, root)["promises"]; len(got) != 0 {
		t.Fatalf("promises = %+v, want none: a real caller reads the accessor", got)
	}
}

func TestAGrammarKeyWithNoAccessorIsNotFound(t *testing.T) {
	root := clean(t)
	write(t, root, "komodo/rules/backlog.md", "# Backlog grammar\n\n## Rules\n"+
		"- **`unwritten_key`** describes a key no accessor exists for yet.\n")
	if got := problemsFrom(t, root)["promises"]; len(got) != 0 {
		t.Fatalf("promises = %+v, want none: no accessor exists to call", got)
	}
}

func TestAMissingGitattributesIsFound(t *testing.T) {
	root := clean(t)
	os.Remove(filepath.Join(root, ".gitattributes"))
	got := problemsFrom(t, root)["gitattributes"]
	if len(got) != 1 || !strings.Contains(got[0].Detail, "eol=lf") {
		t.Fatalf("gitattributes = %+v", got)
	}
}

func TestAGitattributesWithoutEollfIsFound(t *testing.T) {
	root := clean(t)
	write(t, root, ".gitattributes", "* text=auto\n")
	got := problemsFrom(t, root)["gitattributes"]
	if len(got) != 1 || !strings.Contains(got[0].Detail, "eol=lf") {
		t.Fatalf("gitattributes = %+v", got)
	}
}

func TestAGitattributesWithProperEollfPasses(t *testing.T) {
	root := clean(t)
	write(t, root, ".gitattributes", "* text=auto eol=lf\n")
	got := problemsFrom(t, root)["gitattributes"]
	if len(got) != 0 {
		t.Fatalf("gitattributes = %+v, want none", got)
	}
}

func TestAGitattributesWithATabBeforeEollfPasses(t *testing.T) {
	root := clean(t)
	write(t, root, ".gitattributes", "*\ttext=auto eol=lf\n")
	got := problemsFrom(t, root)["gitattributes"]
	if len(got) != 0 {
		t.Fatalf("gitattributes = %+v, want none", got)
	}
}

func TestTokensCountFourCharacters(t *testing.T) {
	if tokens(400) != 100 {
		t.Fatalf("tokens = %d", tokens(400))
	}
}

func TestAHostThatSaysItIsNotInstalledHereIsSkipped(t *testing.T) {
	root := clean(t)
	registerHost(t, mount.Host{Name: "absenthost",
		Installed: func(string) bool { return false },
		Render: func(root, binary string) (install.Plan, error) {
			plan := install.Plan{Host: "absenthost", Root: root}
			plan.Add(filepath.Join(root, "absent.txt"), []byte("new\n"), "never rendered")
			return plan, nil
		}})
	for _, problem := range problemsFrom(t, root)["drift"] {
		if problem.Where == "absent.txt" {
			t.Fatal("a host that says it is not installed here has nothing to drift from")
		}
	}
}

// TestAnUncalledConfigFieldIsFoundThenClearsWithARealCaller proves the new field-promise scan
// fires on a dead config field, and that a real selector read on the same field quiets it.
func TestAnUncalledConfigFieldIsFoundThenClearsWithARealCaller(t *testing.T) {
	root := clean(t)
	write(t, root, "internal/profile/extra.go",
		"package profile\n\ntype Extra struct {\n\tMaxParallel int `json:\"max_parallel_extra\"`\n}\n")
	got := problemsFrom(t, root)["promises"]
	if len(got) != 1 || !strings.Contains(got[0].Detail, "MaxParallel") ||
		!strings.Contains(got[0].Detail, "nothing outside its own tests calls it") {
		t.Fatalf("promises = %+v", got)
	}
	write(t, root, "internal/line/reader.go",
		"package line\n\nimport \"komodo/internal/profile\"\n\nfunc Read(e profile.Extra) int { return e.MaxParallel }\n")
	if got := problemsFrom(t, root)["promises"]; len(got) != 0 {
		t.Fatalf("promises = %+v, want none: a real selector reads the field", got)
	}
}

func TestACommentMentionIsNotARealCall(t *testing.T) {
	root := clean(t)
	write(t, root, "komodo/rules/backlog.md", "# Backlog grammar\n\n## Rules\n"+
		"- **`severity_ceiling`** is a key only a comment ever mentions.\n")
	write(t, root, "internal/profile/ceiling.go", "package profile\n\nfunc SeverityCeiling() int { return 0 }\n")
	write(t, root, "internal/other/thing.go",
		"package other\n\n// SeverityCeiling is not actually called here\nfunc Noop() {}\n")
	got := problemsFrom(t, root)["promises"]
	if len(got) != 1 || !strings.Contains(got[0].Detail, "SeverityCeiling") {
		t.Fatalf("promises = %+v, want one: a comment naming a symbol is not a real call", got)
	}
}

func TestCheckDriftDoesNotEraseProfileDriftOrWriteTheCache(t *testing.T) {
	root := clean(t)
	detect.Load(root)
	write(t, root, "go.mod", "module example\n")
	before, err := os.ReadFile(filepath.Join(root, ".komodo", "profile.json"))
	if err != nil {
		t.Fatal(err)
	}
	registerHost(t, mount.Host{Name: "testhost", Render: func(root, binary string) (install.Plan, error) {
		detect.Load(root)
		return install.Plan{Host: "testhost", Root: root}, nil
	}})
	got := problemsFrom(t, root)
	if len(got["profile"]) != 1 {
		t.Fatalf("profile = %+v, want one: a render must not erase real drift", got["profile"])
	}
	after, err := os.ReadFile(filepath.Join(root, ".komodo", "profile.json"))
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Fatal("the profile cache changed during an audit")
	}
}

func TestCheckDriftIgnoresTheLiveOllamaEndpoint(t *testing.T) {
	root := clean(t)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	t.Setenv(ollama.Env, "http://"+listener.Addr().String())
	seen := false
	registerHost(t, mount.Host{Name: "testhost", Render: func(root, binary string) (install.Plan, error) {
		seen = seen || os.Getenv(ollama.Env) == "http://"+listener.Addr().String()
		return install.Plan{Host: "testhost", Root: root}, nil
	}})
	problemsFrom(t, root)
	if seen {
		t.Fatal("a render saw the live Ollama endpoint during an audit")
	}
}

func TestALeakInATrackedMarkdownFileIsFound(t *testing.T) {
	registerHost(t, mount.Host{Name: "testhost", Vendors: []string{"testvendor"}})
	root := clean(t)
	write(t, root, "komodo/policy.json", "{\n  \"config_paths\": [\"~/.testvendor/**\"]\n}\n")
	got := problemsFrom(t, root)["leaks"]
	if len(got) != 1 || !strings.Contains(got[0].Detail, "belongs inside internal/mount") {
		t.Fatalf("leaks = %+v", got)
	}
}

func TestAProfileWhoseBuilderIsLightIsRejected(t *testing.T) {
	root := clean(t)
	write(t, root, "komodo/profiles/full.json", `{"roles":{"builder":{"tier":"heavy","effort":"medium"}}}`)
	write(t, root, "komodo/profiles/economy.json", `{"roles":{"builder":{"tier":"light","effort":"medium"}}}`)
	got := problemsFrom(t, root)["profiles"]
	if len(got) != 1 || got[0].Where != "komodo/profiles/economy.json" || !strings.Contains(got[0].Detail, "light tier") {
		t.Fatalf("profiles = %+v, want only the economy profile's light builder", got)
	}
}

func TestARoleFileThatFailsToLoadIsFound(t *testing.T) {
	root := clean(t)
	write(t, root, "komodo/roles/broken2.md",
		"---\r\nname: broken2\r\ndescription: x\r\ntier: standard\r\n---\r\n\r\nBody.\r\n")
	got := problemsFrom(t, root)["roles"]
	found := false
	for _, problem := range got {
		if problem.Where == filepath.Join("komodo", "roles", "broken2.md") {
			found = true
		}
	}
	if !found {
		t.Fatalf("roles = %+v, want the CRLF file reported as not loaded", got)
	}
}

func TestAStandardsCapMeasuresTheBodyNotTheWholeFile(t *testing.T) {
	root := clean(t)
	filler := strings.Repeat("x", 9000)
	body := strings.Repeat("rule. ", 800)
	write(t, root, "komodo/skills/standards-huge/SKILL.md",
		"---\nname: standards-huge\nnotes: "+filler+"\n---\n\n"+body)
	if got := problemsFrom(t, root)["budgets"]; len(got) != 0 {
		t.Fatalf("budgets = %+v, want none: only the body counts against the cap", got)
	}
}

func TestAlwaysOnBudgetCountsSkillAndAgentDescriptions(t *testing.T) {
	root := clean(t)
	skills := map[string]string{}
	for i := 0; i < 40; i++ {
		name := fmt.Sprintf("standards-x%02d", i)
		skills[name] = "---\nname: " + name + "\ndescription: " + strings.Repeat("word ", 40) + "\n---\n\n# X\n"
		write(t, root, "komodo/skills/"+name+"/SKILL.md", skills[name])
	}
	registerHost(t, hostRenderingSkills(root, skills))
	found := false
	for _, problem := range problemsFrom(t, root)["budgets"] {
		if problem.Where == "always-on context" {
			found = true
		}
	}
	if !found {
		t.Fatal("the always-on budget did not count the skill descriptions")
	}
}

// gitRepo builds a throwaway repository with one commit on main.
func gitRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	for _, args := range [][]string{
		{"init", "-q", "-b", "main"},
		{"config", "user.email", "test@example.com"},
		{"config", "user.name", "test"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = root
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
	}
	return root
}

// commitAll stages and commits every change in the fixture.
func commitAll(t *testing.T, root, message string) {
	t.Helper()
	for _, args := range [][]string{{"add", "-A"}, {"commit", "-q", "-m", message}} {
		cmd := exec.Command("git", args...)
		cmd.Dir = root
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
	}
}

func TestAConflictMarkerOnTheFirstLineIsFound(t *testing.T) {
	root := gitRepo(t)
	write(t, root, "AGENTS.md", "# Rules\n")
	commitAll(t, root, "init")
	write(t, root, "komodo/rules/broken.md", "<<<<<<< HEAD\nours\n=======\ntheirs\n>>>>>>> branch\n")
	got, err := checkGit(root)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, problem := range got {
		if problem.Check == "git" && problem.Where == filepath.Join("komodo", "rules", "broken.md") {
			found = true
		}
	}
	if !found {
		t.Fatalf("git = %+v, want a conflict marker on the first line to be found", got)
	}
}

// TestAnUntrackedAGENTSFileFailsTheCheck proves a line worktree, cut from a branch, never gets a
// file git never committed, so an AGENTS.md git does not track fails the check.
func TestAnUntrackedAGENTSFileFailsTheCheck(t *testing.T) {
	root := gitRepo(t)
	write(t, root, "AGENTS.md", "# Rules\n")
	if got := checkAGENTSTracked(root); len(got) != 1 || got[0].Where != "AGENTS.md" {
		t.Fatalf("checkAGENTSTracked = %v, want one problem naming AGENTS.md", got)
	}
	commitAll(t, root, "init")
	if got := checkAGENTSTracked(root); len(got) != 0 {
		t.Fatalf("checkAGENTSTracked = %v, want none once AGENTS.md is committed", got)
	}
}

// TestNoAGENTSFileIsNotAProblem proves a repo with no AGENTS.md at all fails no check of its own.
func TestNoAGENTSFileIsNotAProblem(t *testing.T) {
	root := gitRepo(t)
	if got := checkAGENTSTracked(root); len(got) != 0 {
		t.Fatalf("checkAGENTSTracked = %v, want none with no AGENTS.md", got)
	}
}

// TestCheckLegacyBacklogFailsOnARootBacklogFile proves a legacy backlog file at the repo's root or
// under docs/ fails doctor, naming `komodo migrate` as the fix.
func TestCheckLegacyBacklogFailsOnARootBacklogFile(t *testing.T) {
	root := gitRepo(t)
	if got := checkLegacyBacklog(root); len(got) != 0 {
		t.Fatalf("checkLegacyBacklog = %v, want none with no legacy backlog file", got)
	}
	write(t, root, backlog.LegacyName, "# Backlog\n")
	got := checkLegacyBacklog(root)
	if len(got) != 1 || got[0].Where != backlog.LegacyName || !strings.Contains(got[0].Detail, "komodo migrate") {
		t.Fatalf("checkLegacyBacklog = %+v, want one problem naming the legacy file and komodo migrate", got)
	}
}

func TestAWorktreeOutsideTheStateDirectoryYieldsANoteNotAProblem(t *testing.T) {
	root := gitRepo(t)
	write(t, root, "AGENTS.md", "# Rules\n")
	commitAll(t, root, "init")
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = root
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
	}
	outside := filepath.Join(t.TempDir(), "elsewhere")
	run("worktree", "add", "-q", "-b", "feat/outside", outside, "main")
	if resolved, err := filepath.EvalSymlinks(outside); err == nil {
		outside = resolved
	}
	inside := filepath.Join(root, ".komodo", "wt", "TG-01.1")
	run("worktree", "add", "-q", "-b", "feat/inside", inside, "main")

	notes := StrayWorktrees(root)
	joined := strings.Join(notes, "\n")
	if len(notes) != 2 || !strings.Contains(joined, outside+" pins feat/outside; git -C "+outside+" switch --detach frees it") ||
		!strings.Contains(joined, " pins feat/inside; git -C ") {
		t.Fatalf("StrayWorktrees = %+v, want a pin note and its fix for each attached worktree", notes)
	}

	problems, err := checkGit(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, problem := range problems {
		if problem.Check == "git" && (problem.Where == outside || strings.Contains(problem.Detail, "worktree")) {
			t.Fatalf("checkGit = %+v, want a stray worktree to never add a problem", problems)
		}
	}
}

func TestPruneNeverDeletesACriticalRef(t *testing.T) {
	root := gitRepo(t)
	write(t, root, "AGENTS.md", "# Rules\n")
	commitAll(t, root, "init")
	for _, args := range [][]string{
		{"checkout", "-q", "-b", "docs/v2-plan"},
		{"checkout", "-q", "-b", "task/temp"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = root
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
	}
	done, err := Prune(root, "docs/v2-plan", true)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range done {
		if strings.Contains(entry, "deleted merged branch main") {
			t.Fatalf("done = %v, want main never deleted", done)
		}
	}
	out, err := exec.Command("git", "-C", root, "branch", "--list", "main").CombinedOutput()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "main") {
		t.Fatalf("main was deleted: %s", out)
	}
}

func TestAFileInstalledWithOllamaUpIsNotDrift(t *testing.T) {
	root := clean(t)
	write(t, root, "rendered.md", "up\n")
	registerHost(t, mount.Host{Name: "testhost", Render: func(root, binary string) (install.Plan, error) {
		plan := install.Plan{Host: "testhost", Root: root}
		state := "down\n"
		if ollama.Up() {
			state = "up\n"
		}
		plan.Add(filepath.Join(root, "rendered.md"), []byte(state), "a file the local machine shapes")
		return plan, nil
	}})
	if got := problemsFrom(t, root)["drift"]; len(got) != 0 {
		t.Fatalf("drift = %+v", got)
	}
}

func TestCheckRulesetsFlagsARuleThatReachesEveryBranch(t *testing.T) {
	run := func(_ string, args ...string) (string, error) {
		if args[1] == "repos/{owner}/{repo}/rulesets" {
			return `[{"id":1,"name":"Default","target":"branch","enforcement":"active"},{"id":2,"name":"Tags","target":"tag","enforcement":"active"}]`, nil
		}
		return `{"id":1,"name":"Default","target":"branch","enforcement":"active","conditions":{"ref_name":{"include":["~ALL"]}}}`, nil
	}
	problems := CheckRulesets(t.TempDir(), "main", run)
	if len(problems) != 1 || !strings.Contains(problems[0].Detail, "~ALL") {
		t.Fatalf("problems = %+v", problems)
	}
	scoped := func(_ string, args ...string) (string, error) {
		if args[1] == "repos/{owner}/{repo}/rulesets" {
			return `[{"id":1,"name":"Default","target":"branch","enforcement":"active"}]`, nil
		}
		return `{"id":1,"conditions":{"ref_name":{"include":["~DEFAULT_BRANCH"]}}}`, nil
	}
	if problems := CheckRulesets(t.TempDir(), "main", scoped); len(problems) != 0 {
		t.Fatalf("a rule scoped to the default branch was flagged: %+v", problems)
	}
}

func TestCheckRulesetsFlagsADefaultBranchNothingProtects(t *testing.T) {
	unprotected := func(_ string, args ...string) (string, error) {
		if args[1] == "repos/{owner}/{repo}/rulesets" {
			return `[]`, nil
		}
		return "", errors.New("HTTP 404: Branch not protected")
	}
	problems := CheckRulesets(t.TempDir(), "main", unprotected)
	if len(problems) != 1 || !strings.Contains(problems[0].Detail, "no active ruleset without bypass actors") {
		t.Fatalf("problems = %+v", problems)
	}
	classic := func(_ string, args ...string) (string, error) {
		if args[1] == "repos/{owner}/{repo}/rulesets" {
			return `[]`, nil
		}
		return `{"url":"x"}`, nil
	}
	problems = CheckRulesets(t.TempDir(), "main", classic)
	if len(problems) != 1 || !strings.Contains(problems[0].Detail, "no active ruleset") {
		t.Fatalf("classic protection alone passed where the forge offers rulesets: %+v", problems)
	}
}

func TestCheckRulesetsFlagsARulesetOnTheDefaultBranchWithBypassActors(t *testing.T) {
	run := func(_ string, args ...string) (string, error) {
		if args[1] == "repos/{owner}/{repo}/rulesets" {
			return `[{"id":1,"name":"Main","target":"branch","enforcement":"active"}]`, nil
		}
		return `{"id":1,"name":"Main","conditions":{"ref_name":{"include":["~DEFAULT_BRANCH"]}},` +
			`"bypass_actors":[{"actor_id":5,"actor_type":"RepositoryRole","bypass_mode":"always"}]}`, nil
	}
	problems := CheckRulesets(t.TempDir(), "main", run)
	joined := ""
	for _, problem := range problems {
		joined += problem.Detail + "\n"
	}
	if len(problems) != 2 || !strings.Contains(joined, "1 bypass actor(s)") || !strings.Contains(joined, "no active ruleset") {
		t.Fatalf("problems = %+v; a bypassable ruleset must be flagged and must not count as protection", problems)
	}
}

func TestAForgeThatOffersNoRulesetsIsANoteNotAProblem(t *testing.T) {
	unoffered := func(_ string, args ...string) (string, error) {
		return "", errors.New("gh api: exit status 1: Upgrade to GitHub Pro or make this repository public to enable this feature. (HTTP 403)")
	}
	if problems := CheckRulesets(t.TempDir(), "main", unoffered); len(problems) != 0 {
		t.Fatalf("problems = %+v; a forge with no rulesets to offer is a warning", problems)
	}
	notes := ForgeNotes(t.TempDir(), unoffered)
	if len(notes) != 2 || !strings.Contains(notes[0], "no rulesets") || !strings.Contains(notes[1], "status: wip") {
		t.Fatalf("notes = %v, want a warning for rulesets and one for drafts", notes)
	}
	offered := func(_ string, args ...string) (string, error) { return `[]`, nil }
	if notes := ForgeNotes(t.TempDir(), offered); len(notes) != 0 {
		t.Fatalf("notes = %v; a forge that offers rulesets and drafts has nothing to warn of", notes)
	}
}

func TestABareHTTP403StillReportsAProblem(t *testing.T) {
	scopeless := func(_ string, args ...string) (string, error) {
		return "", errors.New("gh: Resource not accessible by integration (HTTP 403)")
	}
	problems := CheckRulesets(t.TempDir(), "main", scopeless)
	if len(problems) != 1 || !strings.Contains(problems[0].Detail, "could not list rulesets") {
		t.Fatalf("problems = %+v; a scopeless or unauthorised-SSO 403 must still be a problem", problems)
	}
	notes := ForgeNotes(t.TempDir(), scopeless)
	if len(notes) != 0 {
		t.Fatalf("notes = %v; a bare 403 is a problem, not a plan note", notes)
	}
}

func TestARemoteAuditWarnsOfAForgeWithNoRulesets(t *testing.T) {
	tools := t.TempDir()
	gh := "#!/bin/sh\n" +
		"case \"$2\" in\n" +
		"  */rulesets) echo 'HTTP 403: Upgrade to GitHub Pro' >&2; exit 1 ;;\n" +
		"  'repos/{owner}/{repo}') echo '{\"delete_branch_on_merge\":true}' ;;\n" +
		"  *) echo '[]' ;;\n" +
		"esac\n"
	write(t, tools, "gh", gh)
	if err := os.Chmod(filepath.Join(tools, "gh"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", tools+string(os.PathListSeparator)+os.Getenv("PATH"))
	var notes []string
	options := Options{NoGit: true, Remote: true, Warn: func(note string) { notes = append(notes, note) }}
	if _, err := Run(clean(t), options); err != nil {
		t.Fatal(err)
	}
	if len(notes) != 2 || !strings.Contains(notes[0], "no rulesets") || !strings.Contains(notes[1], "status: wip") {
		t.Fatalf("notes = %v; a remote audit must warn of a forge with no rulesets or drafts", notes)
	}
}

func TestCheckHeadBranchesFlagsAForgeThatKeepsThemAfterAMerge(t *testing.T) {
	tests := []struct {
		name string
		repo string
		want int
	}{
		{"kept", `{"delete_branch_on_merge":false}`, 1},
		{"deleted", `{"delete_branch_on_merge":true}`, 0},
		{"not shown to this caller", `{"name":"repo"}`, 0},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			run := func(_ string, args ...string) (string, error) { return test.repo, nil }
			problems := CheckHeadBranches(t.TempDir(), run)
			if len(problems) != test.want {
				t.Fatalf("problems = %+v, want %d", problems, test.want)
			}
		})
	}
	failing := func(_ string, args ...string) (string, error) { return "", errors.New("HTTP 404") }
	if problems := CheckHeadBranches(t.TempDir(), failing); len(problems) != 1 {
		t.Fatalf("problems = %+v; an unreadable repo must be reported", problems)
	}
}

func TestPruneSettlesAShippedRunOnceOriginHoldsItsBranch(t *testing.T) {
	const ready = "### [TG-01.1] G\n```yaml\ntype: feat\nversion: 1.0.0\n```\n\n" +
		"#### [TSK-01.1.1] Do it [P: C] [READY]\n```yaml\nfiles: [a.go]\ndone_when:\n  - true\n```\n"
	done := strings.Replace(ready, "[READY]", "[DONE]", 1)
	root := gitRepo(t)
	backlogtest.SeedText(t, root, ready)
	write(t, root, ".gitignore", "/.komodo/\n")
	commitAll(t, root, "init")
	bare := filepath.Join(t.TempDir(), "origin.git")
	worktree := filepath.Join(root, ".komodo", "wt", "TG-01.1")
	run := func(dir string, args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
	}
	run(root, "init", "-q", "--bare", bare)
	run(root, "remote", "add", "origin", bare)
	run(root, "push", "-q", "origin", "main")
	run(root, "worktree", "add", "-q", "-b", "feat/g", worktree, "main")
	backlogtest.SeedText(t, worktree, done)
	commitAll(t, worktree, "ship")
	state := line.RunState{Run: "r", Group: "TG-01.1", Base: "main", Branch: "feat/g", Worktree: worktree}
	if err := line.SaveRun(root, state); err != nil {
		t.Fatal(err)
	}
	line.Stamp(root, ledger.Entry{Station: "ship", Outcome: "done"})

	if _, err := Prune(root, "main", true); err != nil {
		t.Fatal(err)
	}
	rootGroupFile := filepath.Join(root, "docs", "backlog", "TG-01.1-g.md")
	if data, _ := os.ReadFile(rootGroupFile); !strings.Contains(string(data), "[READY]") || !exists(worktree) {
		t.Fatal("prune settled a run whose branch origin does not hold yet")
	}

	run(worktree, "push", "-q", "origin", "feat/g:main", "feat/g")
	got, err := Prune(root, "main", true)
	if err != nil {
		t.Fatal(err)
	}
	if data, _ := os.ReadFile(rootGroupFile); !strings.Contains(string(data), "[READY]") {
		t.Fatalf("prune rewrote the group file; no status flip is left uncommitted to restore; done = %v", got)
	}
	if exists(worktree) {
		t.Fatalf("the shipped worktree survived; done = %v", got)
	}
	if out, _ := exec.Command("git", "-C", root, "branch", "--list", "feat/g").Output(); strings.TrimSpace(string(out)) == "" {
		t.Fatalf("feat/g, a person's branch, was deleted; done = %v", got)
	}
}

func TestPruneSweepsAnEarlierRunsMergedWorktreeWhileTheCurrentRunIsStillOpen(t *testing.T) {
	root := gitRepo(t)
	write(t, root, "README.md", "# Readme\n")
	write(t, root, ".gitignore", "/.komodo/\n")
	commitAll(t, root, "init")
	bare := filepath.Join(t.TempDir(), "origin.git")
	run := func(dir string, args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
	}
	run(root, "init", "-q", "--bare", bare)
	run(root, "remote", "add", "origin", bare)
	run(root, "push", "-q", "origin", "main")

	// branch builds a clean worktree off start.
	branch := func(group, name, start string) string {
		worktree := filepath.Join(root, ".komodo", "wt", group)
		run(root, "worktree", "add", "-q", "-b", name, worktree, start)
		write(t, worktree, group+".txt", "done\n")
		commitAll(t, worktree, "ship "+group)
		return worktree
	}

	// the first run ships and origin's main already holds it.
	first := branch("TG-01.1", "feat/g", "main")
	run(first, "push", "-q", "origin", "feat/g:main", "feat/g")
	run(root, "fetch", "-q", "origin", "main")

	// the second run is still open: its branch has not reached origin.
	second := branch("TG-01.2", "feat/h", "origin/main")
	state := line.RunState{Run: "r2", Group: "TG-01.2", Base: "main", Branch: "feat/h", Worktree: second}
	if err := line.SaveRun(root, state); err != nil {
		t.Fatal(err)
	}

	got, err := Prune(root, "main", true)
	if err != nil {
		t.Fatal(err)
	}
	if exists(first) {
		t.Fatalf("the first run's merged worktree survived because the second run is still open; done = %v", got)
	}
	if out, _ := exec.Command("git", "-C", root, "branch", "--list", "feat/g").Output(); strings.TrimSpace(string(out)) == "" {
		t.Fatalf("feat/g, a person's branch, was deleted; done = %v", got)
	}
	if !exists(second) {
		t.Fatalf("the second run's own unmerged worktree was removed; done = %v", got)
	}
	if out, _ := exec.Command("git", "-C", root, "branch", "--list", "feat/h").Output(); strings.TrimSpace(string(out)) == "" {
		t.Fatalf("feat/h was deleted though origin does not hold it; done = %v", got)
	}
}

func TestSettleShippedRunSkipsTheSweepWhileARunIsOpen(t *testing.T) {
	const ready = "### [TG-01.1] G\n```yaml\ntype: feat\nversion: 1.0.0\n```\n\n" +
		"#### [TSK-01.1.1] Do it [P: C] [READY]\n```yaml\nfiles: [a.go]\ndone_when:\n  - true\n```\n"
	root := gitRepo(t)
	backlogtest.SeedText(t, root, ready)
	write(t, root, ".gitignore", "/.komodo/\n")
	commitAll(t, root, "init")
	bare := filepath.Join(t.TempDir(), "origin.git")
	run := func(dir string, args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
	}
	run(root, "init", "-q", "--bare", bare)
	run(root, "remote", "add", "origin", bare)
	run(root, "push", "-q", "origin", "main")

	worktree := filepath.Join(root, ".komodo", "wt", "TG-01.1")
	run(root, "worktree", "add", "-q", "-b", "feat/g", worktree, "main")
	write(t, worktree, "done.txt", "done\n")
	commitAll(t, worktree, "ship")
	run(worktree, "push", "-q", "origin", "feat/g:main")

	state := line.RunState{Run: "r", Group: "TG-01.1", Base: "main", Branch: "feat/g", Worktree: worktree}
	if err := line.SaveRun(root, state); err != nil {
		t.Fatal(err)
	}

	got, err := Prune(root, "main", true)
	if err != nil {
		t.Fatal(err)
	}
	if !exists(worktree) {
		t.Fatalf("a clean, merged worktree was swept while its own run's group is still open; done = %v", got)
	}
}

func TestPruneRemovesBothRunsWorktreesWhenOnlyTheLatestIsRecorded(t *testing.T) {
	root := gitRepo(t)
	write(t, root, "README.md", "# Readme\n")
	write(t, root, ".gitignore", "/.komodo/\n")
	commitAll(t, root, "init")
	bare := filepath.Join(t.TempDir(), "origin.git")
	run := func(dir string, args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
	}
	run(root, "init", "-q", "--bare", bare)
	run(root, "remote", "add", "origin", bare)
	run(root, "push", "-q", "origin", "main")

	// ship builds a worktree off start, commits, and merges it straight into origin's main.
	ship := func(group, branch, start string) string {
		worktree := filepath.Join(root, ".komodo", "wt", group)
		run(root, "worktree", "add", "-q", "-b", branch, worktree, start)
		write(t, worktree, group+".txt", "done\n")
		commitAll(t, worktree, "ship "+group)
		run(worktree, "push", "-q", "origin", branch+":main", branch)
		return worktree
	}

	first := ship("TG-01.1", "feat/g", "main")
	run(root, "fetch", "-q", "origin", "main")
	second := ship("TG-01.2", "feat/h", "origin/main")
	state := line.RunState{Run: "r2", Group: "TG-01.2", Base: "main", Branch: "feat/h", Worktree: second}
	if err := line.SaveRun(root, state); err != nil {
		t.Fatal(err)
	}

	got, err := Prune(root, "main", true)
	if err != nil {
		t.Fatal(err)
	}
	if exists(first) || exists(second) {
		t.Fatalf("a shipped worktree from an earlier run survived one prune; done = %v", got)
	}
	for _, branch := range []string{"feat/g", "feat/h"} {
		if out, _ := exec.Command("git", "-C", root, "branch", "--list", branch).Output(); strings.TrimSpace(string(out)) == "" {
			t.Fatalf("%s, a person's branch, was deleted; done = %v", branch, got)
		}
	}
}

func TestAMalformedPluginManifestIsFound(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	root := clean(t)
	write(t, root, "komodo/plugins/chat/plugin.json", `{"name":"chat","type":"webhook"}`)
	got := problemsFrom(t, root)["plugins"]
	if len(got) != 1 || got[0].Where != "komodo/plugins/chat/plugin.json" {
		t.Fatalf("plugins = %+v", got)
	}
}

func TestDoctorListsEveryPluginTypeDisabled(t *testing.T) {
	root := clean(t)
	home := t.TempDir()
	t.Setenv("HOME", home)
	write(t, root, "komodo/plugins/cloud/plugin.json",
		`{"name":"cloud","type":"tool-pack","roles":["builder"],"tools":["aws s3 ls"]}`)
	want := []string{
		"plugin notifier: disabled, none installed",
		"plugin tool-pack cloud: disabled",
		"plugin stage-hook: disabled, none installed",
	}
	if got := PluginStates(root); strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("states = %q", got)
	}
	write(t, home, ".komodo/plugins.json", `{"enabled":["cloud"]}`)
	if got := PluginStates(root); got[1] != "plugin tool-pack cloud: enabled" {
		t.Fatalf("states = %q", got)
	}
}

func TestAStaleGlobalLayerFailsDoctorAndWarnsTheGate(t *testing.T) {
	root := clean(t)
	home := t.TempDir()
	t.Setenv("HOME", home)
	marker := filepath.Join(home, ".globalhost", "rendered")
	settings := filepath.Join(home, ".globalhost", "settings.json")
	write(t, home, ".globalhost/rendered", "")
	write(t, home, ".globalhost/settings.json", `{"command": "/old/komodo-0123456789ab guard"}`)
	registerHost(t, mount.Host{Name: "globalhost", Render: func(root, _ string) (install.Plan, error) {
		return install.Plan{Host: "globalhost", Root: root}, nil
	}})
	install.RegisterGlobal("globalhost", func(_, home, _ string) (install.Plan, error) {
		plan := install.Plan{Host: "globalhost", Root: home, Marker: marker, Fix: "komodo install --global"}
		plan.Add(settings, []byte(`{"command": "/home/.komodo/bin/komodo guard"}`), "the guard")
		return plan, nil
	})
	found := problemsFrom(t, root)
	if got := found["global"]; len(got) != 1 || got[0].Where != settings || !strings.Contains(got[0].Detail, "komodo install --global") {
		t.Fatalf("global = %+v; a stale global hook must fail doctor and name the fix", got)
	}
	var warned []string
	problems, err := Run(root, Options{NoGit: true, RepoOnly: true, Warn: func(note string) { warned = append(warned, note) }})
	if err != nil {
		t.Fatal(err)
	}
	for _, problem := range problems {
		if problem.Check == "global" {
			t.Fatalf("problems = %+v; the gate must never fail on the machine's global layer", problems)
		}
	}
	if len(warned) != 1 || !strings.Contains(warned[0], settings) {
		t.Fatalf("warned = %q, want the stale global settings named", warned)
	}
}
