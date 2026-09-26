package conductor

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"komodo/internal/line"
	"komodo/internal/mount"
)

// stateFile is state.json's name under a group's run directory.
const stateFile = "state.json"

// StatePath is where one group's state.json lives, under its run directory.
func StatePath(root, group string) string {
	return filepath.Join(line.RunDir(root, group), stateFile)
}

// LoadState reads one group's state.json.
func LoadState(path string) (State, error) {
	var s State
	data, err := os.ReadFile(path)
	if err != nil {
		return s, err
	}
	if err := json.Unmarshal(data, &s); err != nil {
		return s, fmt.Errorf("reading %s: %w", path, err)
	}
	return s, nil
}

// SaveState writes one group's state.json, making its run directory when it does not exist.
func SaveState(path string, s State) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

// Resume continues a group from its saved state.json, resuming an unfinished Building or
// Repairing session when the host can and starting fresh otherwise, then lets Drive carry it on.
func (d *Driver) Resume(ctx context.Context, s State) (State, error) {
	if d.Host == nil || d.Stations == nil || d.Ledger == nil || d.Save == nil {
		return s, errNotWired
	}
	station, req, waiting := pendingSession(d, s)
	if waiting {
		handle, err := d.startOrResume(s, req)
		if err != nil {
			return s, fmt.Errorf("resuming %s at %s: %w", s.Group, s.Current, err)
		}
		if err := d.build(ctx, &s, station, req, handle); err != nil {
			return s, fmt.Errorf("resuming %s at %s: %w", s.Group, s.Current, err)
		}
	}
	return d.Drive(ctx, s)
}

// pendingSession names the station and request a stopped Building or Repairing group was
// running, so Resume knows what to restart; every other state has nothing left to resume.
func pendingSession(d *Driver, s State) (station string, req mount.StartRequest, waiting bool) {
	if s.SessionDone {
		return "", mount.StartRequest{}, false
	}
	switch s.Current {
	case Building:
		return StationBuild, d.Builder, true
	case Repairing:
		return StationRepair, d.Builder, true
	default:
		return "", mount.StartRequest{}, false
	}
}

// startOrResume resumes the group's last recorded session when the host supports it, else
// starts a fresh one from the same request, so a killed run never repeats a finished session.
func (d *Driver) startOrResume(s State, req mount.StartRequest) (mount.Handle, error) {
	if last := lastSession(s); last != "" && d.Host.Capabilities().Resume {
		return d.Host.Resume(last, "")
	}
	return d.Host.Start(req)
}

// lastSession returns the most recent session handle state.json recorded, or none.
func lastSession(s State) mount.Handle {
	if len(s.Sessions) == 0 {
		return ""
	}
	return mount.Handle(s.Sessions[len(s.Sessions)-1])
}
