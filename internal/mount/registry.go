package mount

import (
	"path/filepath"
	"runtime"
	"sort"
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

// BinaryPath is the prebuilt binary for the platform the toolkit runs on.
func BinaryPath() string {
	name := "komodo-linux-amd64"
	switch runtime.GOOS {
	case "darwin":
		name = "komodo-darwin-arm64"
	case "windows":
		name = "komodo-windows-amd64.exe"
	}
	return filepath.Join("bin", name)
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

// Usage is what a host's own config says about the account's plan and window.
type Usage struct {
	Plan     string    `json:"plan"`
	FiveHour float64   `json:"five_hour"`
	ResetsAt time.Time `json:"resets_at"`
}
