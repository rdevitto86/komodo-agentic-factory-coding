package main

import (
	"reflect"
	"strings"
	"testing"

	"komodo/internal/mount"
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
