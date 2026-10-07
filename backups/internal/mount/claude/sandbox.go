package claude

import (
	"encoding/json"
	"strings"

	"komodo/internal/mount"
)

// forgeDomains are the forge hosts, and every subdomain of each, that a sandboxed session never reaches.
var forgeDomains = []string{"github.com", "githubusercontent.com", "gitlab.com", "bitbucket.org"}

// platformSandbox reports whether goos has an OS sandbox the host starts: Seatbelt on macOS, bubblewrap on
// Linux and WSL2. Native Windows has none.
func platformSandbox(goos string) bool {
	return goos == "darwin" || goos == "linux"
}

// harnessSandbox is a harness session's inline sandbox settings on goos, or empty where it has none: no unsandboxed retry
// or start without it, credential paths unreadable, the forge off the network, ports bindable, tmp writable.
func harnessSandbox(overlay mount.Overlay, goos, tmp string) string {
	if !platformSandbox(goos) {
		return ""
	}
	filesystem := map[string]any{"denyRead": credentialPaths()}
	var writable []string
	if tmp != "" {
		writable = append(writable, tmp)
	}
	writable = append(writable, overlay.SandboxWrite...)
	if len(writable) > 0 {
		filesystem["allowWrite"] = writable
	}
	sandbox := map[string]any{
		"enabled":                  true,
		"failIfUnavailable":        true,
		"allowUnsandboxedCommands": false,
		"filesystem":               filesystem,
		"network": map[string]any{
			"allowedDomains":    offForge(overlay.SandboxDomains),
			"allowLocalBinding": true,
		},
	}
	data, _ := json.Marshal(map[string]any{"sandbox": sandbox})
	return string(data)
}

// offForge returns the domains with every forge host and its subdomains removed, never nil.
func offForge(domains []string) []string {
	out := make([]string, 0, len(domains))
	for _, domain := range domains {
		host := strings.ToLower(strings.TrimPrefix(domain, "*."))
		forge := false
		for _, name := range forgeDomains {
			forge = forge || host == name || strings.HasSuffix(host, "."+name)
		}
		if !forge {
			out = append(out, domain)
		}
	}
	return out
}
