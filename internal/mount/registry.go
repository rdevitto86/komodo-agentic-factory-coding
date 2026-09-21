package mount

import (
	"sort"
	"sync"

	"komodo/internal/install"
)

// Host is one mount: its name, the paths it owns, and how it renders a repo.
type Host struct {
	Name        string
	ConfigPaths []string
	Render      func(root, binary string) (install.Plan, error)
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

// ConfigPaths are every path the registered hosts own, which the guard refuses writes to.
func ConfigPaths() []string {
	var out []string
	for _, host := range Hosts() {
		out = append(out, host.ConfigPaths...)
	}
	return out
}
