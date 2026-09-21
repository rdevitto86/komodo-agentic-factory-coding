package codex

import (
	"os"
	"path/filepath"

	"komodo/internal/mount"
)

// Probe returns nothing: this host exposes no plan or window to read.
func Probe() (mount.Usage, bool) { return mount.Usage{}, false }

// Installed reports whether this host's project directory has been rendered here.
func Installed(root string) bool {
	_, err := os.Stat(filepath.Join(root, Dir, "hooks.json"))
	return err == nil
}

// Tiers is this host's profile row; with Ollama up every tier runs on the local provider.
func Tiers(plan string, ollama bool) mount.Tiers {
	provider := "codex"
	if ollama {
		provider = "ollama"
	}
	return mount.Tiers{
		Light:    mount.Machine{Provider: provider, Model: models["light"], Effort: efforts["light"]},
		Standard: mount.Machine{Provider: provider, Model: models["standard"], Effort: efforts["standard"]},
		Heavy:    mount.Machine{Provider: provider, Model: models["heavy"], Effort: efforts["heavy"]},
		Reviewer: mount.Machine{Provider: provider, Model: models["heavy"], Effort: efforts["heavy"]},
	}
}
