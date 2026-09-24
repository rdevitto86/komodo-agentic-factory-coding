package profile

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"komodo/internal/mount"
	"komodo/internal/mount/ollama"
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
	got := SelectWith(t.TempDir(), []mount.Host{fakeHost("h", false, mount.Usage{}, false)}, false, false)
	if got.Name != "none" || got.Plan != "unknown" {
		t.Fatalf("profile = %+v", got)
	}
	if got.MaxParallel != 2 || got.PauseAt != 0.7 {
		t.Fatalf("the conservative overlay did not apply: %+v", got)
	}
}

func TestTheInstalledMountPicksTheProfile(t *testing.T) {
	host := fakeHost("h", true, mount.Usage{Plan: "max_5x", FiveHour: 0.2}, true)
	got := SelectWith(t.TempDir(), []mount.Host{host}, false, false)
	if got.Name != "h" || got.Host != "h" || got.Plan != "max_5x" {
		t.Fatalf("profile = %+v", got)
	}
	if got.Tiers.Heavy.Model != "big" || got.MaxParallel != 4 {
		t.Fatalf("tiers or pacing are wrong: %+v", got)
	}
}

func TestOllamaUpMakesItHybrid(t *testing.T) {
	host := fakeHost("claude", true, mount.Usage{Plan: "max_5x"}, true)
	got := SelectWith(t.TempDir(), []mount.Host{host}, true, true)
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
	got := SelectWith(t.TempDir(), []mount.Host{fakeHost("codex", true, mount.Usage{}, false)}, true, true)
	if got.Name != "local" {
		t.Fatalf("name = %s", got.Name)
	}
}

func TestOllamaNotAnsweringReportsTheDegradeOnce(t *testing.T) {
	t.Setenv(ollama.Env, "http://127.0.0.1:1")
	host := fakeHost("claude", true, mount.Usage{Plan: "max_5x"}, true)
	got := SelectWith(t.TempDir(), []mount.Host{host}, true, false)
	if got.Name == "hybrid" {
		t.Fatal("the profile stayed hybrid with the local machine down")
	}
	if !strings.Contains(got.Why, "the local machine did not answer") {
		t.Fatalf("why = %q", got.Why)
	}
	if strings.Count(got.Why, "the local machine did not answer") != 1 {
		t.Fatalf("the degrade was reported more than once: %q", got.Why)
	}
}

func TestNoOllamaEnvIsSilentAboutTheLocalMachine(t *testing.T) {
	t.Setenv(ollama.Env, "")
	host := fakeHost("claude", true, mount.Usage{Plan: "max_5x"}, true)
	got := SelectWith(t.TempDir(), []mount.Host{host}, true, false)
	if strings.Contains(got.Why, "did not answer") {
		t.Fatalf("why = %q", got.Why)
	}
}

func TestProPlanLowersTheCeilingAndThePace(t *testing.T) {
	host := fakeHost("h", true, mount.Usage{Plan: "pro"}, true)
	got := SelectWith(t.TempDir(), []mount.Host{host}, false, false)
	if got.MaxParallel != 2 || got.PauseAt != 0.75 || got.ReviewSkipLines != 40 {
		t.Fatalf("pro overlay = %+v", got)
	}
	if got.Tiers.Heavy.Model != "mid" {
		t.Fatalf("the heavy ceiling did not drop: %+v", got.Tiers)
	}
}

func TestNoProbeIsUnknownNotAFailedRun(t *testing.T) {
	host := fakeHost("h", true, mount.Usage{}, false)
	got := SelectWith(t.TempDir(), []mount.Host{host}, false, false)
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
	got := Overlay(SelectWith(root, []mount.Host{fakeHost("h", true, mount.Usage{Plan: "max_20x"}, true)}, false, false), path)
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
	before := SelectWith(t.TempDir(), []mount.Host{fakeHost("h", true, mount.Usage{Plan: "max_5x"}, true)}, false, false)
	after := Overlay(before, filepath.Join(t.TempDir(), "absent.json"))
	if after.MaxParallel != before.MaxParallel || after.Caps.PerFile != before.Caps.PerFile {
		t.Fatal("an absent overlay changed the profile")
	}
}

func TestSelectNeedsTheOverlaySwitchAsWellAsAnAnsweringServer(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, ".fake-select-marker"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	hostSnapshot := mount.Snapshot()
	t.Cleanup(func() { mount.Restore(hostSnapshot) })
	mount.Register(mount.Host{
		Name:       "fake-select",
		HybridName: "hybrid",
		Installed: func(r string) bool {
			_, err := os.Stat(filepath.Join(r, ".fake-select-marker"))
			return err == nil
		},
		Tiers: func(plan string, ollama bool) mount.Tiers {
			tiers := mount.Tiers{Standard: mount.Machine{Provider: "fake-select"}}
			if ollama {
				tiers.Light = mount.Machine{Provider: "ollama"}
			}
			return tiers
		},
	})
	previousLocal := mount.LocalMachine()
	mount.RegisterLocal(mount.Local{Env: ollama.Env, Up: func() bool { return true }})
	t.Cleanup(func() { mount.RegisterLocal(previousLocal) })
	t.Setenv(ollama.Env, "http://127.0.0.1:1")

	got := Select(root)
	if got.Name == "hybrid" {
		t.Fatalf("ollama answered without the overlay switch and every tier still went hybrid: %+v", got)
	}
	if strings.Contains(got.Why, "did not answer") {
		t.Fatalf("the switch-off case reported a failed probe instead of the switch: %q", got.Why)
	}

	if err := os.MkdirAll(filepath.Join(home, ".komodo"), 0o755); err != nil {
		t.Fatal(err)
	}
	overlay := filepath.Join(home, ".komodo", "config.json")
	if err := os.WriteFile(overlay, []byte(`{"local":true}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := Select(root); got.Name != "hybrid" {
		t.Fatalf("the overlay switch and an answering server did not return the hybrid profile: %+v", got)
	}
}

func TestPausedFollowsTheWindow(t *testing.T) {
	resets := time.Now().Add(time.Hour)
	host := fakeHost("h", true, mount.Usage{Plan: "max_5x", FiveHour: 0.95, ResetsAt: resets}, true)
	got := SelectWith(t.TempDir(), []mount.Host{host}, false, false)
	if !got.Paused() {
		t.Fatalf("utilization %v against pause_at %v did not pause", got.Utilization, got.PauseAt)
	}
	if !got.WaitUntil().Equal(resets) {
		t.Fatalf("wait until = %v", got.WaitUntil())
	}
	quiet := fakeHost("h", true, mount.Usage{Plan: "max_5x", FiveHour: 0.1}, true)
	if SelectWith(t.TempDir(), []mount.Host{quiet}, false, false).Paused() {
		t.Fatal("a quiet window paused the run")
	}
}

// TestWhyNamesWhereReviewLandsWhenTheOverlayOptsTheReviewerIn checks the mount's reviewer reason reaches the profile.
func TestWhyNamesWhereReviewLandsWhenTheOverlayOptsTheReviewerIn(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	if err := os.MkdirAll(filepath.Join(home, ".komodo"), 0o755); err != nil {
		t.Fatal(err)
	}
	host := fakeHost("h", true, mount.Usage{Plan: "max"}, true)
	host.ReviewerWhy = func(plan string) string { return "no recall on record for tiny; run komodo recall" }
	without := SelectWith(t.TempDir(), []mount.Host{host}, true, true)
	if strings.Contains(without.Why, "recall") {
		t.Fatalf("why = %q; with local_reviewer unset the reviewer reason stays out", without.Why)
	}
	if err := os.WriteFile(filepath.Join(home, ".komodo", "config.json"), []byte(`{"local": true, "local_reviewer": true}`), 0o644); err != nil {
		t.Fatal(err)
	}
	with := SelectWith(t.TempDir(), []mount.Host{host}, true, true)
	if !strings.Contains(with.Why, "no recall on record for tiny") {
		t.Fatalf("why = %q; the mount's reviewer reason must reach the profile", with.Why)
	}
}
