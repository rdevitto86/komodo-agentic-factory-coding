package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"komodo/internal/line"
	"komodo/internal/mount"
	"komodo/internal/profile"
)

func TestSplitTaskArgFindsTheTaskAfterTheRoleFlag(t *testing.T) {
	task, rest := splitTaskArg([]string{"--role", "reviewer", "TSK-12.1.1"})
	if task != "TSK-12.1.1" {
		t.Fatalf("task = %q", task)
	}
	if !reflect.DeepEqual(rest, []string{"--role", "reviewer"}) {
		t.Fatalf("rest = %v", rest)
	}
}

func TestSplitTaskArgFindsTheTaskBeforeTheRoleFlag(t *testing.T) {
	task, rest := splitTaskArg([]string{"TSK-12.1.1", "--role", "reviewer"})
	if task != "TSK-12.1.1" {
		t.Fatalf("task = %q", task)
	}
	if !reflect.DeepEqual(rest, []string{"--role", "reviewer"}) {
		t.Fatalf("rest = %v", rest)
	}
}

func TestLocalModelRejectsAHeavyTierWhenOnlyLightRunsLocally(t *testing.T) {
	tiers := mount.Tiers{
		Light:    mount.Machine{Provider: "ollama", Model: "llama3.2"},
		Standard: mount.Machine{Provider: "claude", Model: "sonnet"},
		Heavy:    mount.Machine{Provider: "claude", Model: "opus"},
	}
	_, err := localModel(tiers, "heavy")
	if err == nil {
		t.Fatal("want an error naming light as the fallback tier, got none")
	}
	if !strings.Contains(err.Error(), "light") {
		t.Fatalf("err = %q, want it to name light as the fallback tier", err.Error())
	}
}

func TestLocalModelResolvesTheTierThatMountsTheLocalMachine(t *testing.T) {
	tiers := mount.Tiers{
		Light:    mount.Machine{Provider: "ollama", Model: "llama3.2"},
		Standard: mount.Machine{Provider: "claude", Model: "sonnet"},
	}
	model, err := localModel(tiers, "light")
	if err != nil {
		t.Fatal(err)
	}
	if model != "llama3.2" {
		t.Fatalf("model = %q, want llama3.2", model)
	}
}

func TestLocalModelResolvesTheReviewerTierThroughTiersReviewer(t *testing.T) {
	tiers := mount.Tiers{
		Standard: mount.Machine{Provider: "claude", Model: "sonnet"},
		Heavy:    mount.Machine{Provider: "claude", Model: "opus"},
		Reviewer: mount.Machine{Provider: "ollama", Model: "llama3.2"},
	}
	model, err := localModel(tiers, "reviewer")
	if err != nil {
		t.Fatal(err)
	}
	if model != "llama3.2" {
		t.Fatalf("model = %q, want llama3.2", model)
	}
}

func TestLocalModelReviewerErrorNamesWhatActuallyHappens(t *testing.T) {
	tiers := mount.Tiers{
		Light:    mount.Machine{Provider: "ollama", Model: "llama3.2"},
		Standard: mount.Machine{Provider: "claude", Model: "sonnet"},
		Heavy:    mount.Machine{Provider: "claude", Model: "opus"},
		Reviewer: mount.Machine{Provider: "claude", Model: "opus"},
	}
	_, err := localModel(tiers, "reviewer")
	if err == nil {
		t.Fatal("want an error, got none")
	}
	if strings.Contains(err.Error(), "falls back") {
		t.Fatalf("err = %q, want it to say what actually happens, not claim a fallback", err.Error())
	}
	if !strings.Contains(err.Error(), "light") {
		t.Fatalf("err = %q, want it to name light as the tier that does mount", err.Error())
	}
}

func TestSplitFlagsKeepsFlagsAfterEveryPositional(t *testing.T) {
	positional, rest := splitFlags(
		[]string{"mygroup", "Add", "the", "feature", "--files", "a.go,b.go"},
		"files", "done-when", "priority", "status", "type",
	)
	if !reflect.DeepEqual(positional, []string{"mygroup", "Add", "the", "feature"}) {
		t.Fatalf("positional = %v", positional)
	}
	if !reflect.DeepEqual(rest, []string{"--files", "a.go,b.go"}) {
		t.Fatalf("rest = %v", rest)
	}
}

func TestVerifyPathsFailsOnAPathThatDoesNotExist(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.go"), []byte("package a"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := verifyPaths(dir, []string{"a.go"}); err != nil {
		t.Fatal(err)
	}
	if err := verifyPaths(dir, []string{"missing.go"}); err == nil {
		t.Fatal("want an error naming the missing path, got none")
	}
}

func TestCommentsArgsStripsCheckAndKeepsFlagsSeparate(t *testing.T) {
	paths, rest := commentsArgs([]string{"check", "a.go", "b.go", "--require", "exported"}, "require")
	if !reflect.DeepEqual(paths, []string{"a.go", "b.go"}) {
		t.Fatalf("paths = %v", paths)
	}
	if !reflect.DeepEqual(rest, []string{"--require", "exported"}) {
		t.Fatalf("rest = %v", rest)
	}
}

func TestPrintCompactJSONWritesOneLineWithNoIndent(t *testing.T) {
	var buf bytes.Buffer
	printCompactJSON(&buf, map[string]any{"a": 1, "b": []int{1, 2}})
	got := buf.String()
	if strings.Count(got, "\n") != 1 {
		t.Fatalf("output = %q, want exactly one newline", got)
	}
	if strings.Contains(got, "  ") {
		t.Fatalf("output = %q, want no indentation", got)
	}
}

func TestPlanForJSONDropsRoleDescriptionsAndTheWholeProfile(t *testing.T) {
	plan := &line.Plan{
		Group: "TG-1",
		Roles: []line.Role{{
			Name: "builder", Tier: "standard", Machine: "claude/sonnet",
			Description: "a long description no station reads",
		}},
		Profile: profile.Profile{Name: "big-plan"},
	}
	data, err := json.Marshal(planForJSON(plan))
	if err != nil {
		t.Fatal(err)
	}
	got := string(data)
	if strings.Contains(got, "description") || strings.Contains(got, "profile") {
		t.Fatalf("output = %q, want no role description or profile", got)
	}
	if !strings.Contains(got, "claude/sonnet") {
		t.Fatalf("output = %q, want the resolved machine", got)
	}
}

func TestAFlagAfterTheTargetIsStillParsed(t *testing.T) {
	cases := []struct {
		name       string
		args       []string
		valueFlags []string
		positional string
		rest       []string
	}{
		{"flag after the target", []string{"TG-03.6", "--dry-run"}, nil, "TG-03.6", []string{"--dry-run"}},
		{"flag before the target", []string{"--dry-run", "TG-03.6"}, nil, "TG-03.6", []string{"--dry-run"}},
		{"target only", []string{"TG-03.6"}, nil, "TG-03.6", []string{}},
		{"no target", []string{"--dry-run"}, nil, "", []string{"--dry-run"}},
		{"value flag keeps its value", []string{"TSK-1", "--role", "reviewer"}, []string{"role"}, "TSK-1", []string{"--role", "reviewer"}},
		{"value flag before the target", []string{"--budget", "5m", "TG-03.6"}, []string{"budget"}, "TG-03.6", []string{"--budget", "5m"}},
		{"equals form is self contained", []string{"TG-03.6", "--budget=5m"}, []string{"budget"}, "TG-03.6", []string{"--budget=5m"}},
		{"only the first positional is taken", []string{"a", "b"}, nil, "a", []string{"b"}},
	}
	for _, each := range cases {
		got, rest := splitPositional(each.args, each.valueFlags...)
		if got != each.positional {
			t.Fatalf("%s: positional = %q, want %q", each.name, got, each.positional)
		}
		if len(rest) != len(each.rest) {
			t.Fatalf("%s: rest = %v, want %v", each.name, rest, each.rest)
		}
		for i := range rest {
			if rest[i] != each.rest[i] {
				t.Fatalf("%s: rest = %v, want %v", each.name, rest, each.rest)
			}
		}
	}
}

func TestARepairableCloseExitsZeroSoTheLoopContinues(t *testing.T) {
	for status, want := range map[string]int{"DONE": 0, "IN_PROGRESS": 0, "BLOCKED": 1} {
		if got := closeExitCode(status); got != want {
			t.Fatalf("closeExitCode(%s) = %d, want %d", status, got, want)
		}
	}
}
