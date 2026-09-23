package codex

import (
	"os"
	"path/filepath"

	"komodo/internal/mount"
	"komodo/internal/profile"
)

// Probe returns nothing: this host exposes no plan or window to read.
func Probe() (mount.Usage, bool) { return mount.Usage{}, false }

// Installed reports whether this host's project directory has been rendered here.
func Installed(root string) bool {
	_, err := os.Stat(filepath.Join(root, Dir, "hooks.json"))
	return err == nil
}

// Tiers is this host's profile row; with Ollama up every tier runs on the local provider,
// with the model the profile shares across every mount.
func Tiers(plan string, ollama bool) mount.Tiers {
	if ollama {
		local := mount.Machine{Provider: "ollama", Model: profile.OllamaModel}
		return mount.Tiers{Light: local, Standard: local, Heavy: local, Reviewer: local}
	}
	return mount.Tiers{
		Light:    mount.Machine{Provider: "codex", Model: models["light"], Effort: efforts["light"]},
		Standard: mount.Machine{Provider: "codex", Model: models["standard"], Effort: efforts["standard"]},
		Heavy:    mount.Machine{Provider: "codex", Model: models["heavy"], Effort: efforts["heavy"]},
		Reviewer: mount.Machine{Provider: "codex", Model: models["heavy"], Effort: efforts["heavy"]},
	}
}
