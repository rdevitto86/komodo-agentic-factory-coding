package line

import (
	"os"
	"path/filepath"
	"testing"

	"komodo/internal/install"
	"komodo/internal/mount"
)

// renderHost registers a host installed on every root that renders one agent file and one project copy.
func renderHost(t *testing.T) {
	t.Helper()
	snapshot := mount.Snapshot()
	t.Cleanup(func() { mount.Restore(snapshot) })
	mount.Register(mount.Host{
		Name:      "fakehost-render",
		Installed: func(string) bool { return true },
		Render: func(root, _ string) (install.Plan, error) {
			plan := install.Plan{Host: "fakehost-render", Root: root}
			plan.Add(filepath.Join(root, "agents", "reviewer.md"), []byte("new role\n"), "an agent")
			plan.AddProject(filepath.Join(root, "rules.md"), []byte("rules\n"), "the rules")
			return plan, nil
		},
	})
}

func TestRenderRootRewritesAnAgentTheProjectRenderSkips(t *testing.T) {
	renderHost(t)
	root := t.TempDir()
	agent := filepath.Join(root, "agents", "reviewer.md")
	if err := os.MkdirAll(filepath.Dir(agent), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(agent, []byte("old role\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := RenderProject(root, root); err != nil {
		t.Fatal(err)
	}
	if data, _ := os.ReadFile(agent); string(data) != "old role\n" {
		t.Fatalf("the project render wrote the agent: %q", data)
	}
	if err := RenderRoot(root); err != nil {
		t.Fatal(err)
	}
	if data, _ := os.ReadFile(agent); string(data) != "new role\n" {
		t.Fatalf("the root render left the stale agent: %q", data)
	}
	if _, err := os.Stat(filepath.Join(root, "rules.md")); err != nil {
		t.Fatalf("the root render skipped the project copy: %v", err)
	}
}
