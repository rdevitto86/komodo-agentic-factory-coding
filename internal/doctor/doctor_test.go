package doctor

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"komodo/internal/detect"
	"komodo/internal/install"
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
		seen = ollama.Up()
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
