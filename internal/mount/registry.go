package mount

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"komodo/internal/install"
)

// Host is one mount: its name, the paths it owns, the names only it may use, and its render.
type Host struct {
	Name        string
	ConfigPaths []string
	Vendors     []string
	HybridName  string
	Render      func(root, binary string) (install.Plan, error)
	Installed   func(root string) bool
	Tiers       func(plan string, ollama bool) Tiers
	Probe       func() (Usage, bool)
	Usage       func(root, task string, since, until time.Time) (TaskUsage, bool)
	Headless    func(skill, target string) (string, []string)
}

// TaskUsage is what one machine spent on one task, filled after the fact or left empty.
type TaskUsage struct {
	TokensIn     int `json:"tokens_in"`
	TokensOut    int `json:"tokens_out"`
	TokensCached int `json:"tokens_cached"`
	Turns        int `json:"turns"`
}

var (
	lock  sync.Mutex
	hosts = map[string]Host{}
)

// Register adds a mount to the registry, which is how the binary learns a host exists.
func Register(host Host) {
	lock.Lock()
	defer lock.Unlock()
	hosts[host.Name] = host
}

// Hosts returns every registered mount, sorted by name.
func Hosts() []Host {
	lock.Lock()
	defer lock.Unlock()
	out := make([]Host, 0, len(hosts))
	for _, host := range hosts {
		out = append(out, host)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// Snapshot copies the registry, so a test can restore it after registering a fake host.
func Snapshot() map[string]Host {
	lock.Lock()
	defer lock.Unlock()
	out := make(map[string]Host, len(hosts))
	for name, host := range hosts {
		out[name] = host
	}
	return out
}

// Restore replaces the registry with a copy Snapshot returned.
func Restore(snapshot map[string]Host) {
	lock.Lock()
	defer lock.Unlock()
	hosts = snapshot
}

// Get returns one mount by name.
func Get(name string) (Host, bool) {
	lock.Lock()
	defer lock.Unlock()
	host, ok := hosts[name]
	return host, ok
}

// Names lists every registered mount.
func Names() []string {
	var out []string
	for _, host := range Hosts() {
		out = append(out, host.Name)
	}
	return out
}

// Vendors are every name that may appear only inside a mount.
func Vendors() []string {
	var out []string
	for _, host := range Hosts() {
		out = append(out, host.Vendors...)
	}
	return out
}

// ConfigPaths are every path the registered hosts own, which the guard refuses writes to.
func ConfigPaths() []string {
	var out []string
	for _, host := range Hosts() {
		out = append(out, host.ConfigPaths...)
	}
	return out
}

// BinaryPath is where gate --install builds this machine's binary, relative to the main checkout.
func BinaryPath() string {
	name := "komodo-" + runtime.GOOS + "-" + runtime.GOARCH
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	return filepath.Join("bin", name)
}

// MainCheckout is the checkout that owns root's git directory, so a worktree resolves to the repo it came from.
func MainCheckout(root string) string {
	out, err := exec.Command("git", "-C", root, "rev-parse", "--path-format=absolute", "--git-common-dir").Output()
	if err != nil {
		return root
	}
	return filepath.Dir(strings.TrimSpace(string(out)))
}

// Machine is one model behind a tier: who serves it, which model, and at what effort.
type Machine struct {
	Provider string `json:"provider"`
	Model    string `json:"model"`
	Effort   string `json:"effort,omitempty"`
}

// Tiers is a host's whole profile row: a machine per tier, plus the one the reviewer uses.
type Tiers struct {
	Light    Machine `json:"light"`
	Standard Machine `json:"standard"`
	Heavy    Machine `json:"heavy"`
	Reviewer Machine `json:"reviewer"`
}

// Machine returns the machine for one tier name.
func (t Tiers) Machine(tier string) Machine {
	switch tier {
	case "light":
		return t.Light
	case "heavy":
		return t.Heavy
	default:
		return t.Standard
	}
}

// localProvider names the provider that answers on the machine running the line itself.
const localProvider = "ollama"

// Local reports whether a machine mounts the provider running on this host.
func (m Machine) Local() bool {
	return m.Provider == localProvider
}

// FirstRemote returns the first mounted tier, standard first, whose machine is not local.
func (t Tiers) FirstRemote() (Machine, bool) {
	for _, machine := range []Machine{t.Standard, t.Heavy, t.Light} {
		if machine.Provider != "" && machine.Provider != localProvider {
			return machine, true
		}
	}
	return Machine{}, false
}

// Usage is what a host's own config says about the account's plan and window.
type Usage struct {
	Plan     string    `json:"plan"`
	FiveHour float64   `json:"five_hour"`
	ResetsAt time.Time `json:"resets_at"`
}

// GuardTools is what the guard needs from one mount: its tool names, extra paths, and denial encoding.
type GuardTools struct {
	WriteTools   map[string]bool
	PathFields   []string
	ShellTool    string
	CommandField string
	ConfigPaths  []string
	Deny         func(reason string) []byte
}

var (
	guardLock sync.Mutex
	guards    = map[string]GuardTools{}
)

// RegisterGuard adds one mount's tool names and denial encoding, keyed by its host name.
func RegisterGuard(host string, tools GuardTools) {
	guardLock.Lock()
	defer guardLock.Unlock()
	guards[host] = tools
}

// GuardHosts returns every registered mount's guard tools, sorted by host name.
func GuardHosts() []GuardTools {
	guardLock.Lock()
	defer guardLock.Unlock()
	names := make([]string, 0, len(guards))
	for name := range guards {
		names = append(names, name)
	}
	sort.Strings(names)
	out := make([]GuardTools, 0, len(names))
	for _, name := range names {
		out = append(out, guards[name])
	}
	return out
}

// GuardConfigPaths are every extra path a registered mount's guard protects.
func GuardConfigPaths() []string {
	var out []string
	for _, tools := range GuardHosts() {
		out = append(out, tools.ConfigPaths...)
	}
	return out
}

// Overlay is the developer's own ~/.komodo/config.json as the mounts read it.
type Overlay struct {
	LocalModel    string            `json:"local_model"`
	LocalWindow   int               `json:"local_window"`
	LocalReviewer bool              `json:"local_reviewer"`
	Models        map[string]string `json:"models"`
}

// OverlayPath is where a developer's own overlay lives, or empty when there is no home.
func OverlayPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".komodo", "config.json")
}

// LoadOverlay reads the overlay, tolerating a missing or malformed file as an empty one.
func LoadOverlay() Overlay {
	var overlay Overlay
	data, err := os.ReadFile(OverlayPath())
	if err != nil {
		return overlay
	}
	_ = json.Unmarshal(data, &overlay)
	return overlay
}

// OverlayModel is the model the overlay names for one tier, or empty.
func OverlayModel(tier string) string {
	return LoadOverlay().Models[tier]
}
