// Package profile decides which machine runs each station, and how hard the line may push.
package profile

import (
	"encoding/json"
	"io/fs"
	"os"
	"path"
	"time"

	"komodo/internal/mount"
	"komodo/internal/toolkit"
)

// economyPlan is the plan decision 0016 drops into economy mode; every other plan runs full mode.
const economyPlan = "pro"

// Caps are the brief slot caps, in characters, that a profile may lower and never raise.
type Caps struct {
	RepoRules   int `json:"repo_rules"`
	RepoContext int `json:"repo_context"`
	PerFile     int `json:"per_file"`
	FilesTotal  int `json:"files_total"`
	Standard    int `json:"standard"`
	Failure     int `json:"failure"`
}

// Profile is one host's whole configuration: its machines, its caps, and its pacing.
type Profile struct {
	Name            string                 `json:"name"`
	Host            string                 `json:"host"`
	Plan            string                 `json:"plan"`
	Tiers           mount.Tiers            `json:"tiers"`
	Caps            Caps                   `json:"caps"`
	SeverityFloor   string                 `json:"severity_floor"`
	MaxParallel     int                    `json:"max_parallel"`
	Repairs         int                    `json:"repairs"`
	ReviewRepairs   int                    `json:"review_repairs"`
	ReviewSkipLines int                    `json:"review_skip_lines"`
	PauseAt         float64                `json:"pause_at"`
	WarnAt          float64                `json:"warn_at"`
	Labels          []string               `json:"labels"`
	Changelog       string                 `json:"changelog"`
	Base            string                 `json:"base"`
	CriticalRefs    []string               `json:"critical_refs,omitempty"`
	Utilization     float64                `json:"utilization"`
	ResetsAt        time.Time              `json:"resets_at,omitempty"`
	Why             string                 `json:"why"`
	Mode            string                 `json:"mode"`
	HostVersion     string                 `json:"host_version"`
	Roles           map[string]RoleProfile `json:"roles"`
}

// RoleProfile names one role's tier and effort; the host's mount pins the tier's full model ID (decision 0006).
type RoleProfile struct {
	Tier   string `json:"tier"`
	Effort string `json:"effort"`
}

// modeProfile is one mode's JSON shape under komodo/profiles: each role's tier and effort, naming no host.
type modeProfile struct {
	Roles map[string]RoleProfile `json:"roles"`
}

// mode picks full or economy from the plan (decision 0016): a Pro plan runs economy, everything else full.
func mode(plan string) string {
	if plan == economyPlan {
		return "economy"
	}
	return "full"
}

// loadMode reads one mode's profile JSON from komodo/profiles, disk first, the embed otherwise.
func loadMode(root, mode string) (modeProfile, error) {
	data, err := fs.ReadFile(toolkit.FS(root), path.Join("profiles", mode+".json"))
	if err != nil {
		return modeProfile{}, err
	}
	var out modeProfile
	if err := json.Unmarshal(data, &out); err != nil {
		return modeProfile{}, err
	}
	return out, nil
}

// base is what every profile starts from before a host, a plan, and an overlay narrow it.
func base() Profile {
	return Profile{
		Caps: Caps{RepoRules: 8000, RepoContext: 8000, PerFile: 10000,
			FilesTotal: 24000, Standard: 6000, Failure: 80000},
		SeverityFloor: "high",
		MaxParallel:   4,
		Repairs:       1,
		ReviewRepairs: 2,
		PauseAt:       0.9,
		WarnAt:        0.75,
		Labels:        []string{"agent"},
		Changelog:     "CHANGELOG.md",
	}
}

// planOverlay narrows a profile by what the account's plan allows.
func planOverlay(profile Profile, plan string) Profile {
	profile.Plan = plan
	switch plan {
	case "pro":
		profile.MaxParallel = 2
		profile.SeverityFloor = "medium"
		profile.ReviewSkipLines = 40
		profile.Repairs = 1
		profile.PauseAt = 0.75
		profile.WarnAt = 0.6
	case "max_5x":
		profile.MaxParallel = 4
		profile.PauseAt = 0.9
		profile.WarnAt = 0.75
	case "max_20x":
		profile.MaxParallel = 6
		profile.PauseAt = 0.9
		profile.WarnAt = 0.8
	default:
		profile.Plan = "unknown"
		profile.MaxParallel = 2
		profile.SeverityFloor = "medium"
		profile.ReviewSkipLines = 40
		profile.PauseAt = 0.7
		profile.WarnAt = 0.55
	}
	return profile
}

// Select picks the profile with no flag: the host installed, the plan probed, the local machine
// only when the overlay opts in with "local": true and it answers.
func Select(root string) Profile {
	localSwitch := mount.LoadOverlay().Local
	return SelectWith(root, mount.Active(), localSwitch, localSwitch && mount.LocalMachine().Up())
}

// SelectWith is Select over a given set of mounts, which is what a test drives.
// localSwitch is the overlay's own opt-in; local is that switch on and the probe answering.
func SelectWith(root string, hosts []mount.Host, localSwitch, local bool) Profile {
	profile := base()
	host, found := installed(root, hosts)
	if !found {
		profile.Name, profile.Why = "none", "no mount is installed here; run komodo install"
		return withMode(root, planOverlay(profile, ""))
	}
	profile.Host = host.Name
	profile.HostVersion = host.Version
	plan := ""
	if host.Probe != nil {
		if usage, ok := host.Probe(); ok {
			plan = usage.Plan
			profile.Utilization = usage.FiveHour
			profile.ResetsAt = usage.ResetsAt
		}
	}
	profile = planOverlay(profile, plan)
	if host.Tiers != nil {
		profile.Tiers = host.Tiers(profile.Plan, local)
	}
	profile.Name = host.Name
	profile.Why = "the " + host.Name + " mount is installed"
	if local {
		if host.HybridName != "" {
			profile.Name = host.HybridName
		}
		profile.Why += " and the local machine answers"
	} else if endpoint := os.Getenv(mount.LocalMachine().Env); endpoint != "" && host.HybridName != "" {
		if localSwitch {
			profile.Why += "; the local machine did not answer at " + endpoint + ", so the light tier stays on " + host.Name + "'s own light tier"
		} else {
			profile.Why += "; local tiers are off until the overlay sets local"
		}
	}
	if local && host.ReviewerWhy != nil && mount.LoadOverlay().LocalReviewer {
		profile.Why += "; " + host.ReviewerWhy(profile.Plan)
	}
	if plan == "" {
		profile.Why += "; no plan probe, so the conservative overlay applies"
	}
	return withMode(root, profile)
}

// withMode sets the profile's mode from its plan and loads that mode's role tiers and efforts.
func withMode(root string, profile Profile) Profile {
	profile.Mode = mode(profile.Plan)
	loaded, err := loadMode(root, profile.Mode)
	if err != nil {
		return profile
	}
	profile.Roles = loaded.Roles
	return profile
}

// installed returns the first mount rendered into this repo.
func installed(root string, hosts []mount.Host) (mount.Host, bool) {
	for _, host := range hosts {
		if host.Installed != nil && host.Installed(root) {
			return host, true
		}
	}
	return mount.Host{}, false
}

// overlayFile is the machine overlay, which may only lower a cap or add a critical ref.
type overlayFile struct {
	Caps          *Caps    `json:"caps"`
	MaxParallel   *int     `json:"max_parallel"`
	ReviewRepairs *int     `json:"review_repairs"`
	PauseAt       *float64 `json:"pause_at"`
	WarnAt        *float64 `json:"warn_at"`
	CriticalRefs  []string `json:"critical_refs"`
}

// Overlay applies ~/.komodo/config.json, which can only tighten what the profile allows.
func Overlay(profile Profile, path string) Profile {
	data, err := os.ReadFile(path)
	if err != nil {
		return profile
	}
	var overlay overlayFile
	if json.Unmarshal(data, &overlay) != nil {
		return profile
	}
	if overlay.Caps != nil {
		profile.Caps.RepoRules = lower(profile.Caps.RepoRules, overlay.Caps.RepoRules)
		profile.Caps.RepoContext = lower(profile.Caps.RepoContext, overlay.Caps.RepoContext)
		profile.Caps.PerFile = lower(profile.Caps.PerFile, overlay.Caps.PerFile)
		profile.Caps.FilesTotal = lower(profile.Caps.FilesTotal, overlay.Caps.FilesTotal)
		profile.Caps.Standard = lower(profile.Caps.Standard, overlay.Caps.Standard)
		profile.Caps.Failure = lower(profile.Caps.Failure, overlay.Caps.Failure)
	}
	if overlay.MaxParallel != nil {
		profile.MaxParallel = lower(profile.MaxParallel, *overlay.MaxParallel)
	}
	if overlay.ReviewRepairs != nil {
		profile.ReviewRepairs = lower(profile.ReviewRepairs, *overlay.ReviewRepairs)
	}
	if overlay.PauseAt != nil && *overlay.PauseAt > 0 && *overlay.PauseAt < profile.PauseAt {
		profile.PauseAt = *overlay.PauseAt
	}
	if overlay.WarnAt != nil && *overlay.WarnAt > 0 && *overlay.WarnAt < profile.WarnAt {
		profile.WarnAt = *overlay.WarnAt
	}
	profile.CriticalRefs = append(profile.CriticalRefs, overlay.CriticalRefs...)
	return profile
}

// lower keeps the smaller of the two, so an overlay never raises a cap.
func lower(current, proposed int) int {
	if proposed > 0 && proposed < current {
		return proposed
	}
	return current
}

// Paused reports whether the window is too far spent to start another wave.
func (p Profile) Paused() bool { return p.Utilization >= p.PauseAt }

// WaitUntil is when the window resets, which is what intake prints instead of a wave.
func (p Profile) WaitUntil() time.Time { return p.ResetsAt }

// Machine is the model and effort a role runs at: its tier's machine from the host, at the role's effort.
func (p Profile) Machine(role string) (mount.Machine, bool) {
	settings, ok := p.Roles[role]
	if !ok {
		return mount.Machine{}, false
	}
	machine := p.Tiers.Machine(settings.Tier)
	machine.Effort = settings.Effort
	return machine, true
}
