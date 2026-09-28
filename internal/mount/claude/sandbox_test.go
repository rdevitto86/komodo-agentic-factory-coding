package claude

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"komodo/internal/mount"
)

// sandboxBlock is the part of a session's inline settings the sandbox tests read.
type sandboxBlock struct {
	Enabled                  bool  `json:"enabled"`
	FailIfUnavailable        bool  `json:"failIfUnavailable"`
	AllowUnsandboxedCommands *bool `json:"allowUnsandboxedCommands"`
	Filesystem               struct {
		AllowWrite []string `json:"allowWrite"`
		DenyRead   []string `json:"denyRead"`
	} `json:"filesystem"`
	Network struct {
		AllowedDomains *[]string `json:"allowedDomains"`
	} `json:"network"`
}

// parseSandbox decodes the sandbox block out of inline settings, failing the test on malformed JSON.
func parseSandbox(t *testing.T, settings string) sandboxBlock {
	t.Helper()
	var parsed struct {
		Sandbox sandboxBlock `json:"sandbox"`
	}
	if err := json.Unmarshal([]byte(settings), &parsed); err != nil {
		t.Fatalf("settings %q: %v", settings, err)
	}
	return parsed.Sandbox
}

func TestTheSandboxIsOnByDefaultWhereThePlatformHasOne(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	for _, goos := range []string{"darwin", "linux"} {
		t.Run(goos, func(t *testing.T) {
			settings := lineSandbox(mount.Overlay{}, goos)
			if settings == "" {
				t.Fatalf("%s has a sandbox, but a session with no overlay runs without it", goos)
			}
			s := parseSandbox(t, settings)
			if !s.Enabled || !s.FailIfUnavailable || s.AllowUnsandboxedCommands == nil || *s.AllowUnsandboxedCommands {
				t.Fatalf("sandbox = %s; want enabled, failing if unavailable, with no unsandboxed retry", settings)
			}
		})
	}
}

func TestNativeWindowsCountsAsNoSandbox(t *testing.T) {
	if settings := lineSandbox(mount.Overlay{Sandbox: true}, "windows"); settings != "" {
		t.Fatalf("settings = %s; native Windows has no sandbox to start", settings)
	}
}

func TestASandboxedSessionMayWriteNothingOutsideTheWorktreeByDefault(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	s := parseSandbox(t, lineSandbox(mount.Overlay{}, "darwin"))
	if len(s.Filesystem.AllowWrite) != 0 {
		t.Fatalf("allowWrite = %v; with no overlay a write outside the worktree must fail", s.Filesystem.AllowWrite)
	}
	overlay := mount.Overlay{SandboxWrite: []string{"~/go/pkg/mod"}}
	s = parseSandbox(t, lineSandbox(overlay, "linux"))
	if len(s.Filesystem.AllowWrite) != 1 || s.Filesystem.AllowWrite[0] != "~/go/pkg/mod" {
		t.Fatalf("allowWrite = %v, want only the overlay's path", s.Filesystem.AllowWrite)
	}
}

func TestASandboxedSessionCannotReadAForgeCredential(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("GH_CONFIG_DIR", "")
	s := parseSandbox(t, lineSandbox(mount.Overlay{}, "darwin"))
	for _, want := range []string{filepath.Join(home, ".git-credentials"), filepath.Join(home, ".config", "gh")} {
		if !containsPath(s.Filesystem.DenyRead, want) {
			t.Fatalf("denyRead = %v, missing %s", s.Filesystem.DenyRead, want)
		}
	}
}

func TestTheNetworkAllowlistNeverHoldsTheForge(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	overlay := mount.Overlay{SandboxDomains: []string{
		"proxy.golang.org", "github.com", "api.github.com", "*.github.com", "raw.githubusercontent.com",
		"GitLab.com", "bitbucket.org", "notgithub.com",
	}}
	s := parseSandbox(t, lineSandbox(overlay, "darwin"))
	if s.Network.AllowedDomains == nil {
		t.Fatal("allowedDomains is unset; the allowlist must be explicit")
	}
	got := *s.Network.AllowedDomains
	if len(got) != 2 || got[0] != "proxy.golang.org" || got[1] != "notgithub.com" {
		t.Fatalf("allowedDomains = %v, want only proxy.golang.org and notgithub.com", got)
	}
	s = parseSandbox(t, lineSandbox(mount.Overlay{}, "linux"))
	if s.Network.AllowedDomains == nil || len(*s.Network.AllowedDomains) != 0 {
		t.Fatalf("allowedDomains = %v, want an explicit empty list", s.Network.AllowedDomains)
	}
}
