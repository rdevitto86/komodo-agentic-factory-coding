package conductor

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"komodo/internal/plugin"
)

// enabled wraps manifests as plugins this machine enabled.
func enabled(manifests ...plugin.Manifest) []plugin.Plugin {
	out := make([]plugin.Plugin, 0, len(manifests))
	for _, manifest := range manifests {
		out = append(out, plugin.Plugin{Manifest: manifest, Enabled: true})
	}
	return out
}

func TestANotifierGetsTheNoteAndDecidesNothing(t *testing.T) {
	dir := t.TempDir()
	plugins := Plugins{Dir: dir, Enabled: enabled(
		plugin.Manifest{
			Name: "chat", Type: plugin.Notifier, Settings: map[string]string{"channel": "line"},
			Command: `printf '%s %s %s %s' "$KOMODO_EVENT" "$KOMODO_GROUP" "$KOMODO_SETTING_CHANNEL" "$KOMODO_TEXT" > sent`,
		},
		plugin.Manifest{Name: "broken", Type: plugin.Notifier, Command: "exit 3"},
	)}
	failures := plugins.Notify(EventBlocker, "TG-01.1", "needs a person")
	data, err := os.ReadFile(filepath.Join(dir, "sent"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "blocker TG-01.1 line needs a person" {
		t.Fatalf("sent = %q", data)
	}
	if len(failures) != 1 || !strings.Contains(failures[0], "broken") {
		t.Fatalf("failures = %v", failures)
	}
}

func TestAToolPackAddsItsCommandsToItsRolesOnly(t *testing.T) {
	plugins := Plugins{Enabled: enabled(
		plugin.Manifest{Name: "aws", Type: plugin.ToolPack, Roles: []string{"builder"}, Tools: []string{"aws s3 ls"}},
		plugin.Manifest{Name: "gcp", Type: plugin.ToolPack, Roles: []string{"reviewer"}, Tools: []string{"gcloud"}},
	)}
	cases := map[string][]string{"builder": {"aws s3 ls"}, "reviewer": {"gcloud"}, "planner": nil}
	for role, want := range cases {
		if got := plugins.Tools(role); !slices.Equal(got, want) {
			t.Errorf("%s tools = %v, want %v", role, got, want)
		}
	}
}

func TestAStageHookStopsTheGroupWithItsReason(t *testing.T) {
	dir := t.TempDir()
	plugins := Plugins{Dir: dir, Enabled: enabled(
		plugin.Manifest{Name: "ran", Type: plugin.StageHook, Stages: []string{"ship"}, When: plugin.After,
			Command: `touch "$KOMODO_WHEN-$KOMODO_STAGE"`},
		plugin.Manifest{Name: "freeze", Type: plugin.StageHook, Stages: []string{"ship"}, When: plugin.Before,
			Command: "echo release freeze; exit 1"},
	)}
	if err := plugins.Hook(plugin.Before, "build", "TG-01.1"); err != nil {
		t.Fatalf("a hook for another stage ran: %v", err)
	}
	err := plugins.Hook(plugin.Before, "ship", "TG-01.1")
	if !errors.Is(err, ErrHookStopped) || !strings.Contains(err.Error(), "release freeze") {
		t.Fatalf("err = %v", err)
	}
	if err := plugins.Hook(plugin.After, "ship", "TG-01.1"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "after-ship")); err != nil {
		t.Fatalf("the after hook did not run: %v", err)
	}
}

func TestOnlyEnabledPluginsAreCalled(t *testing.T) {
	root, home := t.TempDir(), t.TempDir()
	t.Setenv("HOME", home)
	files := map[string]string{
		"komodo/AGENTS.md":                "# rules\n",
		"komodo/plugins/aws/plugin.json":  `{"name":"aws","type":"tool-pack","roles":["builder"],"tools":["aws"]}`,
		"komodo/plugins/gcp/plugin.json":  `{"name":"gcp","type":"tool-pack","roles":["builder"],"tools":["gcloud"]}`,
		"komodo/plugins/chat/plugin.json": `{"name":"chat","type":"notifier","command":"touch sent"}`,
	}
	for name, body := range files {
		if err := os.MkdirAll(filepath.Dir(filepath.Join(root, name)), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	plugins, err := LoadPlugins(root, root)
	if err != nil || len(plugins.Enabled) != 0 {
		t.Fatalf("with nothing enabled: %v, %v", plugins.Enabled, err)
	}
	if err := os.MkdirAll(filepath.Join(home, ".komodo"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(plugin.EnabledPath(), []byte(`{"enabled":["gcp"]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	plugins, err = LoadPlugins(root, root)
	if err != nil {
		t.Fatal(err)
	}
	if got := plugins.Tools("builder"); !slices.Equal(got, []string{"gcloud"}) {
		t.Fatalf("tools = %v", got)
	}
	plugins.Notify(EventSummary, "", "done")
	if _, err := os.Stat(filepath.Join(root, "sent")); !os.IsNotExist(err) {
		t.Fatalf("a disabled notifier ran: %v", err)
	}
}
