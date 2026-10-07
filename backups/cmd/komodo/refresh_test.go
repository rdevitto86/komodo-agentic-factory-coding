package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"komodo/internal/install"
	"komodo/internal/mount"
)

// refreshHost mounts a host whose global layer is one settings file under home, marked by marker.
func refreshHost(t *testing.T, name, home string) (settings, marker string) {
	t.Helper()
	settings = filepath.Join(home, "."+name, "settings.json")
	marker = filepath.Join(home, "."+name, "rendered")
	snapshot := mount.Snapshot()
	t.Cleanup(func() { mount.Restore(snapshot) })
	mount.Register(mount.Host{Name: name})
	install.RegisterGlobal(name, func(_, home, _ string) (install.Plan, error) {
		plan := install.Plan{Host: name, Root: home, Marker: marker}
		plan.Add(settings, []byte(`{"command": "komodo guard"}`), "the guard")
		return plan, nil
	})
	return settings, marker
}

func TestRefreshRewritesAnInstalledGlobalLayer(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	settings, marker := refreshHost(t, "refreshed", home)
	if err := os.MkdirAll(filepath.Dir(marker), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(marker, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(settings, []byte(`{"command": "/old/komodo-0123456789ab guard"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	done, err := refreshMachine(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if data, _ := os.ReadFile(settings); string(data) != `{"command": "komodo guard"}` {
		t.Fatalf("settings = %s; an installed global layer must be re-rendered", data)
	}
	if !strings.Contains(strings.Join(done, "\n"), settings) {
		t.Fatalf("done = %q, want the rewritten file named", done)
	}
}

func TestRefreshNeverInstallsAGlobalLayerUnasked(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	settings, _ := refreshHost(t, "unasked", home)
	if _, err := refreshMachine(t.TempDir()); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(settings); !os.IsNotExist(err) {
		t.Fatalf("settings written (%v); a machine with no global layer must stay that way", err)
	}
}
