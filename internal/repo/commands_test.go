package repo

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadCommandsWithNoFileReturnsZeroValue(t *testing.T) {
	commands := LoadCommands(t.TempDir())
	if commands != (Commands{}) {
		t.Fatalf("commands = %+v", commands)
	}
}

func TestLoadCommandsReadsEveryStation(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, CommandsFile)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	body := `{"verify":"make verify","compile":"make build","before_review":"make lint","after_publish":"make notify"}`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	commands := LoadCommands(root)
	want := Commands{Verify: "make verify", Compile: "make build", BeforeReview: "make lint", AfterPublish: "make notify"}
	if commands != want {
		t.Fatalf("commands = %+v, want %+v", commands, want)
	}
}
