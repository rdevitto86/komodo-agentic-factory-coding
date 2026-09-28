package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"komodo/internal/conductor"
	"komodo/internal/line"
	"komodo/internal/mount"
	"komodo/internal/review"
)

// stageRepo is a repo with one READY group and no mount; its own .git stops the root walk there.
func stageRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "BACKLOG.md"), []byte("# Backlog\n\n"+pendingGroup), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestStageRefusesAMalformedCommandAndAMissingMount(t *testing.T) {
	cases := []struct {
		name string
		args []string
		want string
	}{
		{"no stage", []string{"stage"}, "usage: komodo stage"},
		{"too many", []string{"stage", "build", "TG-90.2", "extra"}, "usage: komodo stage"},
		{"unknown stage", []string{"stage", "deploy", "TG-90.2"}, "build, review or ship"},
		{"unknown group", []string{"stage", "ship", "TG-99.9"}, "TG-99.9"},
		{"no mount", []string{"stage", "ship", "TG-90.2"}, "no mount is installed"},
	}
	for _, each := range cases {
		t.Run(each.name, func(t *testing.T) {
			got := runCLI(t, stageRepo(t), "", each.args...)
			if got.code != 1 || !strings.Contains(got.stderr, each.want) {
				t.Fatalf("komodo %v exited %d, stderr %q; want 1 and %q", each.args, got.code, got.stderr, each.want)
			}
		})
	}
}

// stageFakeHost answers every session with one scripted result.
type stageFakeHost struct {
	value map[string]any
}

func (f *stageFakeHost) Preflight() error { return nil }

func (f *stageFakeHost) Start(req mount.StartRequest) (mount.Handle, error) {
	return mount.Handle(req.Role), nil
}

func (f *stageFakeHost) Resume(handle mount.Handle, _ string) (mount.Handle, error) {
	return handle, nil
}

func (f *stageFakeHost) Stream(mount.Handle) (<-chan mount.Event, error) {
	out := make(chan mount.Event)
	close(out)
	return out, nil
}

func (f *stageFakeHost) Result(mount.Handle) (mount.Result, error) {
	return mount.Result{Value: f.value}, nil
}

func (f *stageFakeHost) Stop(mount.Handle) error { return nil }

func (f *stageFakeHost) Capabilities() mount.Capabilities { return mount.Capabilities{} }

// stageFakeStations run no git and push nothing; Ship returns shipErr.
type stageFakeStations struct {
	shipErr error
}

func (f *stageFakeStations) Snapshot() error                           { return nil }
func (f *stageFakeStations) Check(context.Context) ([]string, error)   { return nil, nil }
func (f *stageFakeStations) Prepare(context.Context) ([]string, error) { return nil, nil }
func (f *stageFakeStations) Ship(context.Context) error                { return f.shipErr }
func (f *stageFakeStations) Diff(string) (string, error)               { return "", nil }
func (f *stageFakeStations) Head() (string, error)                     { return "head", nil }
func (f *stageFakeStations) Merge() (bool, error)                      { return false, nil }

// fakeStage swaps the stage's host, stations and reviewer request for fakes until the test ends.
func fakeStage(t *testing.T, value map[string]any, shipErr error) {
	t.Helper()
	oldHost, oldStations, oldReviewer := stageHost, stageStations, stageReviewer
	t.Cleanup(func() { stageHost, stageStations, stageReviewer = oldHost, oldStations, oldReviewer })
	stageReviewer = func(string, *line.Plan, review.Lens) (mount.StartRequest, error) {
		return mount.StartRequest{Role: "reviewer", Brief: "review TG-90.2"}, nil
	}
	stageHost = func(string, string) (mount.Contract, error) { return &stageFakeHost{value: value}, nil }
	stageStations = func(string, string, *line.Plan) conductor.Stations { return &stageFakeStations{shipErr: shipErr} }
}

func TestStageRunsOneStageAndReportsItsOutcome(t *testing.T) {
	cases := []struct {
		name    string
		args    []string
		value   map[string]any
		shipErr error
		code    int
		want    string
	}{
		{"ship done", []string{"stage", "ship", "TG-90.2"}, nil, nil, 0, "TG-90.2 ship: done"},
		{"ship fails", []string{"stage", "ship", "TG-90.2"}, nil, errors.New("push refused"), 1, "push refused"},
		{"build blocks", []string{"stage", "build", "TG-90.2"}, map[string]any{"result": "BLOCKED"}, nil, 1,
			"TG-90.2 build: blocked"},
		{"build done", []string{"stage", "build", "TG-90.2"}, map[string]any{"result": "DONE"}, nil, 0,
			"TG-90.2 build: done"},
		{"review finds", []string{"stage", "review", "TG-90.2"}, map[string]any{"findings": []any{
			map[string]any{"severity": "critical", "file": "b/two.go", "line": 1, "title": "Nil map", "fix": "Make it"},
		}}, nil, 1, "b/two.go:1 Nil map: Make it"},
	}
	for _, each := range cases {
		t.Run(each.name, func(t *testing.T) {
			fakeStage(t, each.value, each.shipErr)
			root := stageRepo(t)
			got := runCLI(t, root, "", each.args...)
			if got.code != each.code || !strings.Contains(got.stdout+got.stderr, each.want) {
				t.Fatalf("komodo %v exited %d, printed %q%q; want %d and %q",
					each.args, got.code, got.stdout, got.stderr, each.code, each.want)
			}
			if _, err := os.Stat(filepath.Join(root, ".komodo", "adhoc.jsonl")); err != nil {
				t.Fatalf("the ad hoc ledger holds no row for the stage: %v", err)
			}
			if each.args[1] == "review" {
				saved, err := os.ReadFile(line.ResultPath(root, "TG-90.2-review"))
				if err != nil || !strings.Contains(string(saved), "Nil map") {
					t.Fatalf("review result = %q, %v; want the reviewer's findings where ship reads them", saved, err)
				}
			}
		})
	}
}
