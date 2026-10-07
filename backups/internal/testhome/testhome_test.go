package testhome

import (
	"os"
	"testing"
)

// TestIsolatePointsHomeAtAFreshRootAndCleansItUp proves Isolate moves HOME and the temp
// variables into a new directory, and its cleanup removes it.
func TestIsolatePointsHomeAtAFreshRootAndCleansItUp(t *testing.T) {
	oldHome, oldTemp := os.Getenv("HOME"), os.Getenv("TMPDIR")
	cleanup := Isolate()
	defer func() {
		_ = os.Setenv("HOME", oldHome)
		_ = os.Setenv("TMPDIR", oldTemp)
	}()

	home := os.Getenv("HOME")
	if home == "" || home == oldHome {
		t.Fatalf("HOME = %q, want a fresh root distinct from %q", home, oldHome)
	}
	if _, err := os.Stat(home); err != nil {
		t.Fatalf("home dir does not exist: %v", err)
	}
	if os.Getenv("TMPDIR") == oldTemp {
		t.Fatal("TMPDIR was not pointed at the fresh root")
	}

	cleanup()
	if _, err := os.Stat(home); !os.IsNotExist(err) {
		t.Fatalf("cleanup left the home dir behind: err = %v", err)
	}
}
