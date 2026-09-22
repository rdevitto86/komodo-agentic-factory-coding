package main

import (
	"reflect"
	"testing"
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
