package line

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"komodo/internal/detect"
	"komodo/internal/facet"
	"komodo/internal/mount"
	repopkg "komodo/internal/repo"
)

// placeholderRole is a role file whose body only carries the placeholders one test needs filled.
func placeholderRole(root, name, tier, body string) error {
	text := "---\nname: " + name + "\ndescription: Test role.\ntier: " + tier +
		"\ntools: [read, edit, write, shell, search]\nsession: true\nreturns: builder.schema.json\n---\n\n" + body + "\n"
	return os.WriteFile(filepath.Join(root, RolesDir, name+".md"), []byte(text), 0o644)
}

// writeFacet ships a minimal facet under komodo/facets: its appendix, its skill, and its detect markers.
func writeFacet(t *testing.T, root, name, facetMD string, detectJSON string) {
	t.Helper()
	dir := filepath.Join(root, facet.FacetsDir, name)
	if err := os.MkdirAll(filepath.Join(dir, "skill"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "facet.md"), []byte(facetMD), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "skill", "SKILL.md"), []byte("---\nname: facet-"+name+"\n---\n\nsetup\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if detectJSON != "" {
		if err := os.WriteFile(filepath.Join(dir, "detect.json"), []byte(detectJSON), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// TestProfileRowSwapMovesTheStationsMachine proves a profile row change moves a station to
// another machine and the next step names it, with no code change and no restart.
func TestProfileRowSwapMovesTheStationsMachine(t *testing.T) {
	root := stepRepo(t)
	startRun(t, root)
	briefPath := filepath.Join(root, StateDir, "briefs", "TSK-12.1.1.md")
	if err := os.MkdirAll(filepath.Dir(briefPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(briefPath, []byte("brief"), 0o644); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(root, ".fakehost-marker")
	if err := os.WriteFile(marker, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	var current mount.Machine
	mount.Register(mount.Host{
		Name: "fakehost-machine-swap",
		Installed: func(r string) bool {
			_, err := os.Stat(filepath.Join(r, ".fakehost-marker"))
			return err == nil
		},
		Tiers: func(string, bool) mount.Tiers {
			return mount.Tiers{Standard: current}
		},
	})

	current = mount.Machine{Provider: "vendora", Model: "model-a"}
	before, err := Step(root, "")
	if err != nil {
		t.Fatal(err)
	}
	if before.Machine != "vendora/model-a" {
		t.Fatalf("machine = %q, want vendora/model-a before the swap", before.Machine)
	}

	current = mount.Machine{Provider: "vendorb", Model: "model-b"}
	after, err := Step(root, "")
	if err != nil {
		t.Fatal(err)
	}
	if after.Machine != "vendorb/model-b" {
		t.Fatalf("machine = %q; a profile row change must move the station with no code change and no restart", after.Machine)
	}
}

// TestSkillSwapReachesTheBriefAndTheRender proves a skill body change reaches the next brief
// under komodo/skills, and the next render when it overrides a shipped skill.
func TestSkillSwapReachesTheBriefAndTheRender(t *testing.T) {
	t.Run("a standards skill under komodo/skills reaches the next brief", func(t *testing.T) {
		root := repo(t, groupText)
		if err := placeholderRole(root, "builder", "standard", "Standards:\n{{standards}}\n"); err != nil {
			t.Fatal(err)
		}
		skillDir := filepath.Join(root, SkillsDir, "standards-go")
		if err := os.MkdirAll(skillDir, 0o755); err != nil {
			t.Fatal(err)
		}
		skill := "---\nname: standards-go\nglobs: [\"**/*.go\"]\nroles: []\n---\n\nversion one of the go standard\n"
		skillPath := filepath.Join(skillDir, "SKILL.md")
		if err := os.WriteFile(skillPath, []byte(skill), 0o644); err != nil {
			t.Fatal(err)
		}

		before, err := BuildBrief(root, root, "TSK-05.1.1", "builder", "")
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(before.Text, "version one of the go standard") {
			t.Fatalf("brief = %s, want the skill's first body", before.Text)
		}

		updated := strings.Replace(skill, "version one", "version two", 1)
		if err := os.WriteFile(skillPath, []byte(updated), 0o644); err != nil {
			t.Fatal(err)
		}

		after, err := BuildBrief(root, root, "TSK-05.1.1", "builder", "")
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(after.Text, "version one of the go standard") || !strings.Contains(after.Text, "version two of the go standard") {
			t.Fatalf("brief = %s; a skill body change must reach the next brief with no code change and no restart", after.Text)
		}
	})

	t.Run("a repo override under .komodo/skills reaches the next project render", func(t *testing.T) {
		root := t.TempDir()
		shipped := filepath.Join(root, "komodo", "skills", "demo")
		if err := os.MkdirAll(shipped, 0o755); err != nil {
			t.Fatal(err)
		}
		body := "---\nname: demo\n---\n\nshipped body\n"
		if err := os.WriteFile(filepath.Join(shipped, "SKILL.md"), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}

		render := func() string {
			skills, err := mount.LoadSkills(root)
			if err != nil {
				t.Fatal(err)
			}
			byName := map[string]int{}
			for i, skill := range skills {
				byName[skill.Name] = i
			}
			overrides, _ := repopkg.LoadSkills(root)
			for _, override := range overrides {
				mount.MergeOverride(&skills, byName, override.Name, override.New, override.Body)
			}
			return skills[byName["demo"]].Body
		}

		before := render()
		if !strings.Contains(before, "shipped body") || strings.Contains(before, "extra rule") {
			t.Fatalf("render = %q, want the shipped body with no override yet", before)
		}

		overrideDir := filepath.Join(root, repopkg.SkillsDir, "demo")
		if err := os.MkdirAll(overrideDir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(overrideDir, "SKILL.md"), []byte("extra rule for this repo\n"), 0o644); err != nil {
			t.Fatal(err)
		}

		after := render()
		if !strings.Contains(after, "extra rule for this repo") {
			t.Fatalf("render = %q; a .komodo/skills override must reach the next project render with no code change and no restart", after)
		}
	})
}

// TestFacetSwapReachesTheStandardsSlotTheProfileSlotAndTheRender proves a facet added any of the
// three ways reaches the brief's standards slot, its profile slot, and the render.
func TestFacetSwapReachesTheStandardsSlotTheProfileSlotAndTheRender(t *testing.T) {
	t.Run("a task's facets key reaches the standards slot", func(t *testing.T) {
		root := repo(t, groupText)
		if err := placeholderRole(root, "builder", "standard", "{{standards}}\n"); err != nil {
			t.Fatal(err)
		}
		writeFacet(t, root, "aws", "# AWS\n\n## Builder appendix\nUse least privilege IAM.\n", "")

		before, err := BuildBrief(root, root, "TSK-05.1.1", "builder", "")
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(before.Text, "least privilege") {
			t.Fatalf("brief = %s, want no aws appendix before the task carries the facet", before.Text)
		}

		path := filepath.Join(root, "BACKLOG.md")
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		swapped := strings.Replace(string(data),
			"#### [TSK-05.1.1] One [P: C] [READY]\n```yaml\nfiles: [a/one.go]\ndone_when: [\"go test ./a/...\"]\n```",
			"#### [TSK-05.1.1] One [P: C] [READY]\n```yaml\nfiles: [a/one.go]\ndone_when: [\"go test ./a/...\"]\nfacets: [aws]\n```", 1)
		if swapped == string(data) {
			t.Fatal("the task block was not found to swap")
		}
		if err := os.WriteFile(path, []byte(swapped), 0o644); err != nil {
			t.Fatal(err)
		}

		after, err := BuildBrief(root, root, "TSK-05.1.1", "builder", "")
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(after.Text, "least privilege") {
			t.Fatalf("brief = %s; a task's facets key must reach the standards slot with no code change and no restart", after.Text)
		}
	})

	t.Run("an external dependency file reaches the profile slot", func(t *testing.T) {
		root := repo(t, groupText)
		if err := placeholderRole(root, "builder", "standard", "Profile: {{repo_profile}}\n"); err != nil {
			t.Fatal(err)
		}

		before, err := BuildBrief(root, root, "TSK-05.1.1", "builder", "")
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(before.Text, "cloud: none") {
			t.Fatalf("brief = %s, want no cloud detected yet", before.Text)
		}

		if err := os.WriteFile(filepath.Join(root, "cdk.json"), []byte("{}"), 0o644); err != nil {
			t.Fatal(err)
		}

		after, err := BuildBrief(root, root, "TSK-05.1.1", "builder", "")
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(after.Text, "cloud: aws") {
			t.Fatalf("brief = %s; an infra file must reach the profile slot with no code change and no restart", after.Text)
		}
	})

	t.Run(".komodo/facets reaches what the render adds", func(t *testing.T) {
		root := t.TempDir()
		writeFacet(t, root, "aws", "# AWS\n\n## Builder appendix\nUse least privilege IAM.\n", "")

		before, err := facet.Select(root, detect.Profile{}, nil)
		if err != nil {
			t.Fatal(err)
		}
		if contains(before, "aws") {
			t.Fatal("aws must not be selected before the override names it")
		}

		if err := os.MkdirAll(filepath.Dir(filepath.Join(root, facet.OverrideFile)), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, facet.OverrideFile), []byte("aws\n"), 0o644); err != nil {
			t.Fatal(err)
		}

		after, err := facet.Select(root, detect.Profile{}, nil)
		if err != nil {
			t.Fatal(err)
		}
		if !contains(after, "aws") {
			t.Fatal(".komodo/facets must select aws with no code change and no restart")
		}
		loaded, err := facet.Load(root, "aws")
		if err != nil {
			t.Fatal(err)
		}
		if loaded.Skill == "" {
			t.Fatal("the render adds the facet's own setup skill, which must not be empty")
		}
	})
}

// TestFacetMCPJSONChangesNothing proves a facet's mcp.json, when present, changes nothing
// in V2: MCP is the fifth swap point and stays deferred.
func TestFacetMCPJSONChangesNothing(t *testing.T) {
	root := t.TempDir()
	writeFacet(t, root, "aws", "# AWS\n\n## Builder appendix\nUse least privilege IAM.\n", "")

	before, err := facet.Load(root, "aws")
	if err != nil {
		t.Fatal(err)
	}

	mcpPath := filepath.Join(root, facet.FacetsDir, "aws", "mcp.json")
	if err := os.WriteFile(mcpPath, []byte(`{"mcpServers":{"aws":{"command":"aws-mcp"}}}`), 0o644); err != nil {
		t.Fatal(err)
	}

	after, err := facet.Load(root, "aws")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before, after) {
		t.Fatalf("facet = %+v; a facet's mcp.json must change nothing in V2 while MCP is deferred", after)
	}
}

// TestCommandsJSONSwapReplacesVerifyAtQC proves a commands.json change replaces the
// verify command QC runs, with no code change and no restart.
func TestCommandsJSONSwapReplacesVerifyAtQC(t *testing.T) {
	root := gitRepo(t)
	commit(t, root, "Makefile", "verify:\n\t@true\n", "seed")
	if _, err := git(root, "branch", "task/tsk-14.1.1"); err != nil {
		t.Fatal(err)
	}
	plan := &Plan{Group: "TG-14.1", Waves: [][]string{{"TSK-14.1.1"}}}

	before, err := CloseWave(root, plan, 0)
	if err != nil {
		t.Fatal(err)
	}
	if before.Verify == nil || before.Verify.Command != "make verify" {
		t.Fatalf("verify = %+v, want the Makefile's own command before any override", before.Verify)
	}

	if err := os.MkdirAll(filepath.Join(root, ".komodo"), 0o755); err != nil {
		t.Fatal(err)
	}
	body := []byte(`{"verify":"echo swapped"}`)
	if err := os.WriteFile(filepath.Join(root, repopkg.CommandsFile), body, 0o644); err != nil {
		t.Fatal(err)
	}

	after, err := CloseWave(root, plan, 0)
	if err != nil {
		t.Fatal(err)
	}
	if after.Verify == nil || after.Verify.Command != "echo swapped" {
		t.Fatalf("verify = %+v; a commands.json change must replace verify at QC with no code change and no restart", after.Verify)
	}
}
