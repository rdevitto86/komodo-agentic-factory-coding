package claude

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"

	"komodo/internal/mount"
	"komodo/internal/mount/ollama"
)

// configFile is the host's own config, which is the only place the plan is read from.
const configFile = ".claude.json"

// tierPlans maps the rate-limit tiers this host's config reports to the plan overlays.
var tierPlans = map[string]string{
	"default_claude_pro":    "pro",
	"default_claude_max_5x": "max_5x",
	"claude_max_5x":         "max_5x",
	"default_claude_max":    "max_20x",
	"claude_max_20x":        "max_20x",
	"max_20x":               "max_20x",
	"max_5x":                "max_5x",
	"pro":                   "pro",
}

// account is the part of the host config the probe reads. The email and the ids are never read.
type account struct {
	OAuth struct {
		OrganizationRateLimitTier string `json:"organizationRateLimitTier"`
		UserRateLimitTier         string `json:"userRateLimitTier"`
		SeatTier                  string `json:"seatTier"`
		SubscriptionType          string `json:"subscriptionType"`
	} `json:"oauthAccount"`
	Cached struct {
		Utilization struct {
			FiveHour struct {
				Utilization float64 `json:"utilization"`
				ResetsAt    string  `json:"resets_at"`
			} `json:"five_hour"`
		} `json:"utilization"`
	} `json:"cachedUsageUtilization"`
}

// Probe reads the plan and the window from this host's config file, never from its status line.
func Probe() (mount.Usage, bool) {
	home, err := os.UserHomeDir()
	if err != nil {
		return mount.Usage{}, false
	}
	data, err := os.ReadFile(filepath.Join(home, configFile))
	if err != nil {
		return mount.Usage{}, false
	}
	var parsed account
	if json.Unmarshal(data, &parsed) != nil {
		return mount.Usage{}, false
	}
	usage := mount.Usage{Plan: planName(parsed), FiveHour: parsed.Cached.Utilization.FiveHour.Utilization / 100}
	if stamp, err := time.Parse(time.RFC3339, parsed.Cached.Utilization.FiveHour.ResetsAt); err == nil {
		usage.ResetsAt = stamp
	}
	if usage.Plan == "" {
		return usage, false
	}
	return usage, true
}

// planName maps the first rate-limit tier the config names to a plan overlay.
func planName(parsed account) string {
	for _, raw := range []string{
		parsed.OAuth.OrganizationRateLimitTier,
		parsed.OAuth.UserRateLimitTier,
		parsed.OAuth.SeatTier,
		parsed.OAuth.SubscriptionType,
	} {
		if raw == "" {
			continue
		}
		if plan, ok := tierPlans[strings.ToLower(raw)]; ok {
			return plan
		}
	}
	return ""
}

// Installed reports whether this host's project directory has been rendered here.
func Installed(root string) bool {
	_, err := os.Stat(filepath.Join(root, Dir, "settings.json"))
	return err == nil
}

// Tiers is this host's profile row: a model per tier, lowered by the plan, renamed by the overlay,
// with the light tier on Ollama when it answers and the reviewer there only when the overlay opts in.
func Tiers(plan string, local bool) mount.Tiers {
	heavyTier := "heavy"
	if plan == "pro" {
		heavyTier = "standard"
	}
	tiers := mount.Tiers{
		Light:    mount.Machine{Provider: "claude", Model: modelFor("light")},
		Standard: mount.Machine{Provider: "claude", Model: modelFor("standard")},
		Heavy:    mount.Machine{Provider: "claude", Model: modelFor(heavyTier)},
		Reviewer: mount.Machine{Provider: "claude", Model: modelFor(heavyTier)},
	}
	if reviewer := mount.OverlayModel("reviewer"); reviewer != "" {
		tiers.Reviewer.Model = reviewer
	}
	if local {
		tiers.Light = mount.Machine{Provider: "ollama", Model: ollama.ModelName()}
		if mount.LoadOverlay().LocalReviewer {
			tiers.Reviewer = mount.Machine{Provider: "ollama", Model: ollama.ModelName()}
		}
	}
	return tiers
}

// modelFor is this host's model for a tier, unless the overlay names another.
func modelFor(tier string) string {
	if name := mount.OverlayModel(tier); name != "" {
		return name
	}
	return models[tier]
}
