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

func TestAHookNamingAMissingBinaryIsFoundAndAnotherKomodoCopyIsNotDrift(t *testing.T) {
	root := clean(t)
	gone, _ := json.Marshal(filepath.Join(root, "gone", "komodo") + " guard")
	write(t, root, "settings.json", `{"command": `+string(gone)+`}`)
	registerHost(t, mount.Host{Name: "testhost", Render: func(root, binary string) (install.Plan, error) {
		plan := install.Plan{Host: "testhost", Root: root}
		plan.Add(filepath.Join(root, "settings.json"), []byte(`{"command": "/elsewhere/komodo-linux-amd64 guard"}`), "the guard")
		return plan, nil
	}})
	found := problemsFrom(t, root)
	if len(found["drift"]) != 0 {
		t.Fatalf("drift = %+v; another komodo copy is not drift", found["drift"])
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
	if len(notes) != 1 || !strings.Contains(notes[0], outside) || !strings.Contains(notes[0], "feat/outside") {
		t.Fatalf("StrayWorktrees = %+v, want one note naming the outside path and branch", notes)
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
	done, err := Prune(root, "docs/v2-plan")
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
	if len(problems) != 1 || !strings.Contains(problems[0].Detail, "no active ruleset or branch protection") {
		t.Fatalf("problems = %+v", problems)
	}
	classic := func(_ string, args ...string) (string, error) {
		if args[1] == "repos/{owner}/{repo}/rulesets" {
			return `[]`, nil
		}
		return `{"url":"x"}`, nil
	}
	if problems := CheckRulesets(t.TempDir(), "main", classic); len(problems) != 0 {
		t.Fatalf("a branch under classic protection was flagged: %+v", problems)
	}
}

func TestPruneSettlesAShippedRunOnceOriginHoldsItsBranch(t *testing.T) {
	const ready = "### [TG-01.1] G\n```yaml\ntype: feat\nversion: 1.0.0\n```\n\n" +
		"#### [TSK-01.1.1] Do it [P: C] [READY]\n```yaml\nfiles: [a.go]\ndone_when:\n  - true\n```\n"
	done := strings.Replace(ready, "[READY]", "[DONE]", 1)
	root := gitRepo(t)
	write(t, root, "BACKLOG.md", ready)
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
	write(t, worktree, "BACKLOG.md", done)
	commitAll(t, worktree, "ship")
	state := line.RunState{Run: "r", Group: "TG-01.1", Base: "main", Branch: "feat/g", Worktree: worktree}
	if err := line.SaveRun(root, state); err != nil {
		t.Fatal(err)
	}
	line.Stamp(root, ledger.Entry{Station: "ship", Outcome: "done"})

	if _, err := Prune(root, "main"); err != nil {
		t.Fatal(err)
	}
	if data, _ := os.ReadFile(filepath.Join(root, "BACKLOG.md")); string(data) != ready || !exists(worktree) {
		t.Fatal("prune settled a run whose branch origin does not hold yet")
	}

	run(worktree, "push", "-q", "origin", "feat/g:main")
	got, err := Prune(root, "main")
	if err != nil {
		t.Fatal(err)
	}
	if data, _ := os.ReadFile(filepath.Join(root, "BACKLOG.md")); string(data) != ready {
		t.Fatalf("prune rewrote BACKLOG.md; no status flip is left uncommitted to restore; done = %v", got)
	}
	if exists(worktree) {
		t.Fatalf("the shipped worktree survived; done = %v", got)
	}
	if out, _ := exec.Command("git", "-C", root, "branch", "--list", "feat/g").Output(); strings.TrimSpace(string(out)) != "" {
		t.Fatalf("feat/g survived; done = %v", got)
	}
}

func TestPruneSweepsAnEarlierRunsMergedWorktreeWhileTheCurrentRunIsStillOpen(t *testing.T) {
	root := gitRepo(t)
	write(t, root, "BACKLOG.md", "# Backlog\n")
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
	run(first, "push", "-q", "origin", "feat/g:main")
	run(root, "fetch", "-q", "origin", "main")

	// the second run is still open: its branch has not reached origin.
	second := branch("TG-01.2", "feat/h", "origin/main")
	state := line.RunState{Run: "r2", Group: "TG-01.2", Base: "main", Branch: "feat/h", Worktree: second}
	if err := line.SaveRun(root, state); err != nil {
		t.Fatal(err)
	}

	got, err := Prune(root, "main")
	if err != nil {
		t.Fatal(err)
	}
	if exists(first) {
		t.Fatalf("the first run's merged worktree survived because the second run is still open; done = %v", got)
	}
	if out, _ := exec.Command("git", "-C", root, "branch", "--list", "feat/g").Output(); strings.TrimSpace(string(out)) != "" {
		t.Fatalf("feat/g survived; done = %v", got)
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
	write(t, root, "BACKLOG.md", ready)
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

	got, err := Prune(root, "main")
	if err != nil {
		t.Fatal(err)
	}
	if !exists(worktree) {
		t.Fatalf("a clean, merged worktree was swept while its own run's group is still open; done = %v", got)
	}
}

func TestPruneRemovesBothRunsWorktreesWhenOnlyTheLatestIsRecorded(t *testing.T) {
	root := gitRepo(t)
	write(t, root, "BACKLOG.md", "# Backlog\n")
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
		run(worktree, "push", "-q", "origin", branch+":main")
		return worktree
	}

	first := ship("TG-01.1", "feat/g", "main")
	run(root, "fetch", "-q", "origin", "main")
	second := ship("TG-01.2", "feat/h", "origin/main")
	state := line.RunState{Run: "r2", Group: "TG-01.2", Base: "main", Branch: "feat/h", Worktree: second}
	if err := line.SaveRun(root, state); err != nil {
		t.Fatal(err)
	}

	got, err := Prune(root, "main")
	if err != nil {
		t.Fatal(err)
	}
	if exists(first) || exists(second) {
		t.Fatalf("a shipped worktree from an earlier run survived one prune; done = %v", got)
	}
	for _, branch := range []string{"feat/g", "feat/h"} {
		if out, _ := exec.Command("git", "-C", root, "branch", "--list", branch).Output(); strings.TrimSpace(string(out)) != "" {
			t.Fatalf("%s survived; done = %v", branch, got)
		}
	}
}
