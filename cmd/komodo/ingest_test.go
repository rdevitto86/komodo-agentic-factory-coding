package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const ingestSample = "## [EPIC-01] Epic\n\n### [TG-01.1] A card\n```yaml\ntype: feat\nversion: 1.0.0\n```\n\n" +
	"#### [TSK-01.1.1] First task [P: C] [READY]\n```yaml\nfiles: [a/one.go]\n```\n"

// writeBacklog writes text as the repo's BACKLOG.md.
func writeBacklog(t *testing.T, root, text string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(root, "BACKLOG.md"), []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestRunIngestWritesACardForEveryReadyGroup proves ingest compiles every ready group with no name given.
func TestRunIngestWritesACardForEveryReadyGroup(t *testing.T) {
	root := t.TempDir()
	writeBacklog(t, root, ingestSample)
	out := captureStdout(t, func() { runIngest(root, nil) })
	if !strings.Contains(out, "TG-01.1") || !strings.Contains(out, "1 card(s)") {
		t.Fatalf("ingest output = %q", out)
	}
	if _, err := os.Stat(filepath.Join(root, ".komodo", "queue", "TG-01.1.json")); err != nil {
		t.Fatalf("card file not written: %v", err)
	}
}

// TestRunIngestNamesOneGroup proves a positional group id narrows ingest to just that group.
func TestRunIngestNamesOneGroup(t *testing.T) {
	root := t.TempDir()
	writeBacklog(t, root, ingestSample+"\n### [TG-01.2] Another group\n```yaml\ntype: feat\nversion: 1.0.0\n```\n\n"+
		"#### [TSK-01.2.1] Second task [P: C] [READY]\n```yaml\n```\n")
	out := captureStdout(t, func() { runIngest(root, []string{"TG-01.2"}) })
	if !strings.Contains(out, "TG-01.2") || strings.Contains(out, "TG-01.1") {
		t.Fatalf("ingest output = %q", out)
	}
	if _, err := os.Stat(filepath.Join(root, ".komodo", "queue", "TG-01.1.json")); err == nil {
		t.Fatal("ingest wrote a card for a group it wasn't asked for")
	}
}

// TestRunIngestJSONPrintsTheCompiledCards proves --json prints the cards rather than one summary line each.
func TestRunIngestJSONPrintsTheCompiledCards(t *testing.T) {
	root := t.TempDir()
	writeBacklog(t, root, ingestSample)
	out := captureStdout(t, func() { runIngest(root, []string{"--json"}) })
	if !strings.Contains(out, `"group": "TG-01.1"`) || !strings.Contains(out, `"hash"`) {
		t.Fatalf("ingest --json output = %q", out)
	}
}

// TestRunIngestFailsOnAnUnknownGroup proves a missing group name exits non-zero instead of writing nothing quietly.
func TestRunIngestFailsOnAnUnknownGroup(t *testing.T) {
	root := t.TempDir()
	writeBacklog(t, root, ingestSample)
	oldExit := exit
	defer func() { exit = oldExit }()
	var code int
	exit = func(c int) { code = c; panic("exit") }
	defer func() {
		if recover() == nil {
			t.Fatal("want a panic from exit")
		}
		if code != 1 {
			t.Fatalf("exit code = %d, want 1", code)
		}
	}()
	runIngest(root, []string{"TG-99.9"})
}
