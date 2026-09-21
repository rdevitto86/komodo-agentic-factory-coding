package profile

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"komodo/internal/mount"
)

// fakeHost builds a mount that reports what a test wants.
func fakeHost(name string, installed bool, usage mount.Usage, probed bool) mount.Host {
	hybrid := "local"
	if name == "claude" {
		hybrid = "hybrid"
	}
	return mount.Host{
		Name:       name,
		HybridName: hybrid,
		Installed:  func(string) bool { return installed },
		Probe:      func() (mount.Usage, bool) { return usage, probed },
		Tiers: func(plan string, ollama bool) mount.Tiers {
			tiers := mount.Tiers{
				Light:    mount.Machine{Provider: name, Model: "small"},
				Standard: mount.Machine{Provider: name, Model: "mid"},
				Heavy:    mount.Machine{Provider: name, Model: "big"},
				Reviewer: mount.Machine{Provider: name, Model: "big"},
			}
			if plan == "pro" {
				tiers.Heavy = mount.Machine{Provider: name, Model: "mid"}
			}
			if ollama {
				tiers.Light = mount.Machine{Provider: "ollama"}
				tiers.Reviewer = mount.Machine{Provider: "ollama"}
			}
			return tiers
		},
	}
}

func TestNoMountInstalledIsTheConservativeOverlay(t *testing.T) {
	got := SelectWith(t.TempDir(), []mount.Host{fakeHost("h", false, mount.Usage{}, false)}, false)
	if got.Name != "none" || got.Plan != "unknown" {
		t.Fatalf("profile = %+v", got)
	}
	if got.MaxParallel != 2 || got.PauseAt != 0.7 {
		t.Fatalf("the conservative overlay did not apply: %+v", got)
	}
}

func TestTheInstalledMountPicksTheProfile(t *testing.T) {
	host := fakeHost("h", true, mount.Usage{Plan: "max_5x", FiveHour: 0.2}, true)
	got := SelectWith(t.TempDir(), []mount.Host{host}, false)
	if got.Name != "h" || got.Host != "h" || got.Plan != "max_5x" {
		t.Fatalf("profile = %+v", got)
	}
	if got.Tiers.Heavy.Model != "big" || got.MaxParallel != 4 {
		t.Fatalf("tiers or pacing are wrong: %+v", got)
	}
}

func TestOllamaUpMakesItHybrid(t *testing.T) {
	host := fakeHost("claude", true, mount.Usage{Plan: "max_5x"}, true)
	got := SelectWith(t.TempDir(), []mount.Host{host}, true)
	if got.Name != "hybrid" {
		t.Fatalf("name = %s", got.Name)
	}
	if got.Tiers.Reviewer.Provider != "ollama" || got.Tiers.Light.Provider != "ollama" {
		t.Fatalf("the local machine did not take the light tier and the reviewer: %+v", got.Tiers)
	}
	if got.Tiers.Standard.Provider != "claude" {
		t.Fatal("the builder left the host's standard tier")
	}
}

func TestAnotherHostWithOllamaIsLocal(t *testing.T) {
	got := SelectWith(t.TempDir(), []mount.Host{fakeHost("codex", true, mount.Usage{}, false)}, true)
	if got.Name != "local" {
		t.Fatalf("name = %s", got.Name)
	}
}

func TestProPlanLowersTheCeilingAndThePace(t *testing.T) {
	host := fakeHost("h", true, mount.Usage{Plan: "pro"}, true)
	got := SelectWith(t.TempDir(), []mount.Host{host}, false)
	if got.MaxParallel != 2 || got.PauseAt != 0.75 || got.ReviewSkipLines != 40 {
		t.Fatalf("pro overlay = %+v", got)
	}
	if got.Tiers.Heavy.Model != "mid" {
		t.Fatalf("the heavy ceiling did not drop: %+v", got.Tiers)
	}
}

func TestNoProbeIsUnknownNotAFailedRun(t *testing.T) {
	host := fakeHost("h", true, mount.Usage{}, false)
	got := SelectWith(t.TempDir(), []mount.Host{host}, false)
	if got.Plan != "unknown" || got.Host != "h" {
		t.Fatalf("profile = %+v", got)
	}
	if got.Tiers.Standard.Model == "" {
		t.Fatal("a missing probe left the profile without machines")
	}
}

func TestAnOverlayOnlyLowersACapAndAddsARef(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "config.json")
	body := `{"caps":{"per_file":2000,"failure":999999},"max_parallel":1,"pause_at":0.5,"warn_at":0.99,"critical_refs":["release"]}`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	got := Overlay(SelectWith(root, []mount.Host{fakeHost("h", true, mount.Usage{Plan: "max_20x"}, true)}, false), path)
	if got.Caps.PerFile != 2000 {
		t.Fatalf("the overlay did not lower the cap: %d", got.Caps.PerFile)
	}
	if got.Caps.Failure != 80000 {
		t.Fatalf("the overlay raised a cap: %d", got.Caps.Failure)
	}
	if got.MaxParallel != 1 {
		t.Fatalf("max_parallel = %d", got.MaxParallel)
	}
	if got.PauseAt != 0.5 {
		t.Fatalf("pause_at = %v", got.PauseAt)
	}
	if got.WarnAt == 0.99 {
		t.Fatal("the overlay raised warn_at")
	}
	if len(got.CriticalRefs) != 1 || got.CriticalRefs[0] != "release" {
		t.Fatalf("critical refs = %v", got.CriticalRefs)
	}
}

func TestAMissingOverlayChangesNothing(t *testing.T) {
	before := SelectWith(t.TempDir(), []mount.Host{fakeHost("h", true, mount.Usage{Plan: "max_5x"}, true)}, false)
	after := Overlay(before, filepath.Join(t.TempDir(), "absent.json"))
	if after.MaxParallel != before.MaxParallel || after.Caps.PerFile != before.Caps.PerFile {
		t.Fatal("an absent overlay changed the profile")
	}
}

func TestPausedFollowsTheWindow(t *testing.T) {
	resets := time.Now().Add(time.Hour)
	host := fakeHost("h", true, mount.Usage{Plan: "max_5x", FiveHour: 0.95, ResetsAt: resets}, true)
	got := SelectWith(t.TempDir(), []mount.Host{host}, false)
	if !got.Paused() {
		t.Fatalf("utilization %v against pause_at %v did not pause", got.Utilization, got.PauseAt)
	}
	if !got.WaitUntil().Equal(resets) {
		t.Fatalf("wait until = %v", got.WaitUntil())
	}
	quiet := fakeHost("h", true, mount.Usage{Plan: "max_5x", FiveHour: 0.1}, true)
	if SelectWith(t.TempDir(), []mount.Host{quiet}, false).Paused() {
		t.Fatal("a quiet window paused the run")
	}
}
