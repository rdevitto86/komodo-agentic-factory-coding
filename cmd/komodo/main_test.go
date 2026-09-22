package main

import (
	"reflect"
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
	if _, err := localModel(tiers, "heavy"); err == nil {
		t.Fatal("want an error naming light as the fallback tier, got none")
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
