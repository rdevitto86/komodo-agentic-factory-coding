package mount

import (
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
)

// withRegistry swaps in an empty host registry for one test and restores the real one after.
func withRegistry(t *testing.T) {
	t.Helper()
	saved := Snapshot()
	Restore(map[string]Host{})
	t.Cleanup(func() { Restore(saved) })
}

func TestRegistryListsHostsSortedWithTheirNamesAndPaths(t *testing.T) {
	withRegistry(t)
	Register(Host{Name: "zeta", Vendors: []string{"z"}, ConfigPaths: []string{"~/.zeta/**"}})
	Register(Host{Name: "alpha", Vendors: []string{"a", "a2"}, ConfigPaths: []string{"~/.alpha/**"}})
	if got := Names(); !reflect.DeepEqual(got, []string{"alpha", "zeta"}) {
		t.Fatalf("names = %v", got)
	}
	if got := Vendors(); !reflect.DeepEqual(got, []string{"a", "a2", "z"}) {
		t.Fatalf("vendors = %v", got)
	}
	if got := ConfigPaths(); !reflect.DeepEqual(got, []string{"~/.alpha/**", "~/.zeta/**"}) {
		t.Fatalf("config paths = %v", got)
	}
	if host, ok := Get("zeta"); !ok || host.Name != "zeta" {
		t.Fatalf("get zeta = %+v, %v", host, ok)
	}
	if _, ok := Get("missing"); ok {
		t.Fatal("an unregistered host was found")
	}
}

func TestSnapshotIsACopyTheRegistryCannotChange(t *testing.T) {
	withRegistry(t)
	Register(Host{Name: "one"})
	snapshot := Snapshot()
	Register(Host{Name: "two"})
	if len(snapshot) != 1 {
		t.Fatalf("a later register changed the snapshot: %v", snapshot)
	}
	Restore(snapshot)
	if got := Names(); !reflect.DeepEqual(got, []string{"one"}) {
		t.Fatalf("restore = %v", got)
	}
}

func TestBinaryPathNamesThisPlatform(t *testing.T) {
	got := BinaryPath()
	if !strings.HasPrefix(got, "bin"+string(filepath.Separator)+"komodo-"+runtime.GOOS+"-"+runtime.GOARCH) {
		t.Fatalf("binary path = %s", got)
	}
}

func TestMainCheckoutFallsBackToRootOutsideGit(t *testing.T) {
	root := t.TempDir()
	if got := MainCheckout(root); got != root {
		t.Fatalf("main checkout = %s, want %s", got, root)
	}
}

func TestTiersResolveEachNameAndTheFirstRemote(t *testing.T) {
	tiers := Tiers{
		Light:    Machine{Provider: LocalName, Model: "small"},
		Standard: Machine{Provider: "remote", Model: "mid"},
		Heavy:    Machine{Provider: "remote", Model: "big"},
	}
	for tier, want := range map[string]string{"light": "small", "standard": "mid", "heavy": "big", "other": "mid"} {
		if got := tiers.Machine(tier).Model; got != want {
			t.Fatalf("tier %s = %s, want %s", tier, got, want)
		}
	}
	if !tiers.Light.Local() || tiers.Heavy.Local() {
		t.Fatal("local is read from the provider")
	}
	if remote, ok := tiers.FirstRemote(); !ok || remote.Model != "mid" {
		t.Fatalf("first remote = %+v, %v", remote, ok)
	}
	allLocal := Tiers{Light: tiers.Light, Standard: tiers.Light, Heavy: tiers.Light}
	if _, ok := allLocal.FirstRemote(); ok {
		t.Fatal("an all-local row has no remote")
	}
}

func TestLocalMachineFillsSafeDefaultsAndKeepsARegisteredCall(t *testing.T) {
	saved := LocalMachine()
	t.Cleanup(func() { RegisterLocal(saved) })
	RegisterLocal(Local{})
	empty := LocalMachine()
	if empty.Up() || empty.ModelName() != "" || !empty.Fits(1<<30) {
		t.Fatal("an unset local machine must be down, unnamed, and unbounded")
	}
	if empty.Allowed([]string{"read", "write"}) || !empty.Allowed([]string{"read", "search"}) {
		t.Fatal("the default allow list must refuse a write role and pass a read role")
	}
	if _, err := empty.Post("m", "b", nil); err == nil {
		t.Fatal("an unset local machine must refuse a call")
	}
	RegisterLocal(Local{Env: "X", Up: func() bool { return true }, ModelName: func() string { return "m" }})
	if got := LocalMachine(); !got.Up() || got.ModelName() != "m" || got.Env != "X" {
		t.Fatalf("a registered machine lost its fields: %+v", got)
	}
}

func TestGuardHostsAreSortedAndCarryTheirPaths(t *testing.T) {
	guardLock.Lock()
	saved := guards
	guards = map[string]GuardTools{}
	guardLock.Unlock()
	t.Cleanup(func() {
		guardLock.Lock()
		guards = saved
		guardLock.Unlock()
	})
	RegisterGuard("zeta", GuardTools{ShellTool: "Z", ConfigPaths: []string{".z"}})
	RegisterGuard("alpha", GuardTools{ShellTool: "A", ConfigPaths: []string{".a"}})
	hosts := GuardHosts()
	if len(hosts) != 2 || hosts[0].ShellTool != "A" || hosts[1].ShellTool != "Z" {
		t.Fatalf("guard hosts = %+v", hosts)
	}
	if got := GuardConfigPaths(); !reflect.DeepEqual(got, []string{".a", ".z"}) {
		t.Fatalf("guard config paths = %v", got)
	}
}

func TestOverlayReadsTheHomeFileAndToleratesItsAbsence(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	if got := LoadOverlay(); got.LocalURL != "" || got.Models != nil {
		t.Fatalf("a missing overlay = %+v", got)
	}
	if got := OverlayPath(); got != filepath.Join(home, ".komodo", "config.json") {
		t.Fatalf("overlay path = %s", got)
	}
	if err := os.MkdirAll(filepath.Join(home, ".komodo"), 0o755); err != nil {
		t.Fatal(err)
	}
	body := `{"local_url":"http://x:1","local_reviewer":true,"models":{"heavy":"mid"}}`
	if err := os.WriteFile(OverlayPath(), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	got := LoadOverlay()
	if got.LocalURL != "http://x:1" || !got.LocalReviewer || OverlayModel("heavy") != "mid" || OverlayModel("light") != "" {
		t.Fatalf("overlay = %+v", got)
	}
	if err := os.WriteFile(OverlayPath(), []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := LoadOverlay(); got.LocalURL != "" {
		t.Fatalf("a malformed overlay = %+v", got)
	}
}
