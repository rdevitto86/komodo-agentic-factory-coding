package plugin

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fixture writes a toolkit root holding each manifest under komodo/plugins/<folder>/plugin.json.
func fixture(t *testing.T, manifests map[string]string) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "komodo"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "komodo", "AGENTS.md"), []byte("# rules\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for folder, body := range manifests {
		dir := filepath.Join(root, "komodo", "plugins", folder)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, ManifestName), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

var shipped = map[string]string{
	"chat":  `{"name":"chat","type":"notifier","command":"true","settings":{"channel":"line"}}`,
	"cloud": `{"name":"cloud","type":"tool-pack","roles":["builder"],"tools":["aws s3 ls"]}`,
	"lint":  `{"name":"lint","type":"stage-hook","stages":["ship"],"when":"before","command":"true"}`,
}

func TestAllThreeTypesLoadDisabled(t *testing.T) {
	plugins, problems := Load(fixture(t, shipped), map[string]bool{})
	if len(problems) != 0 {
		t.Fatalf("problems: %v", problems)
	}
	if len(plugins) != 3 {
		t.Fatalf("loaded %d plugins, want 3", len(plugins))
	}
	types := map[Type]bool{}
	for _, loaded := range plugins {
		types[loaded.Type] = true
		if loaded.Enabled {
			t.Errorf("%s loaded enabled with nothing enabling it", loaded.Name)
		}
	}
	for _, kind := range Types {
		if !types[kind] {
			t.Errorf("no %s loaded", kind)
		}
	}
	if plugins[0].Settings["channel"] != "line" {
		t.Errorf("settings = %v", plugins[0].Settings)
	}
}

func TestOnlyTheNamedPluginIsEnabled(t *testing.T) {
	plugins, _ := Load(fixture(t, shipped), map[string]bool{"cloud": true})
	for _, loaded := range plugins {
		if loaded.Enabled != (loaded.Name == "cloud") {
			t.Errorf("%s enabled = %v", loaded.Name, loaded.Enabled)
		}
	}
}

func TestEnabledReadsTheMachineFile(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	enabled, err := Enabled()
	if err != nil || len(enabled) != 0 {
		t.Fatalf("no file: %v, %v", enabled, err)
	}
	if err := os.MkdirAll(filepath.Join(home, ".komodo"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(EnabledPath(), []byte(`{"enabled":["chat"]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	enabled, err = Enabled()
	if err != nil || !enabled["chat"] || len(enabled) != 1 {
		t.Fatalf("enabled = %v, %v", enabled, err)
	}
	if err := os.WriteFile(EnabledPath(), []byte(`{`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Enabled(); err == nil {
		t.Fatal("a malformed machine file read as none enabled")
	}
}

func TestAMalformedManifestIsAProblem(t *testing.T) {
	cases := []struct {
		name, folder, body, want string
	}{
		{"not JSON", "bad", `{`, "is not JSON"},
		{"no name", "bad", `{"type":"notifier","command":"true"}`, "names no plugin"},
		{"wrong folder", "bad", `{"name":"other","type":"notifier","command":"true"}`, "not its folder"},
		{"unknown type", "bad", `{"name":"bad","type":"webhook"}`, "is not notifier"},
		{"notifier without command", "bad", `{"name":"bad","type":"notifier"}`, "needs a command"},
		{"tool pack without roles", "bad", `{"name":"bad","type":"tool-pack","tools":["gcloud"]}`, "needs tools and roles"},
		{"hook without stages", "bad", `{"name":"bad","type":"stage-hook","command":"true","when":"after"}`, "needs a command"},
		{"hook without when", "bad", `{"name":"bad","type":"stage-hook","command":"true","stages":["ship"]}`, "before or after"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			plugins, problems := Load(fixture(t, map[string]string{tc.folder: tc.body}), nil)
			if len(plugins) != 0 {
				t.Fatalf("a malformed manifest loaded: %v", plugins)
			}
			if len(problems) != 1 || !strings.Contains(problems[0].Detail, tc.want) {
				t.Fatalf("problems = %v, want one naming %q", problems, tc.want)
			}
			if problems[0].Where != "komodo/plugins/bad/plugin.json" {
				t.Errorf("where = %s", problems[0].Where)
			}
		})
	}
}
