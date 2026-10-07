package mount

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"komodo/internal/fsx"
	"komodo/internal/git"
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
	// Concurrency is how many groups or tasks the named plan runs at once; nil defers to a caller's own default.
	Concurrency func(plan string) int
	// LoggedIn reports whether the host CLI holds a login, by subscription or key; nil skips the check.
	LoggedIn   func() (bool, error)
	Usage      func(root, task string, since, until time.Time) (TaskUsage, bool)
	EventsPath func(root, task string) string
	// Leftovers names what a retired setup left in the host's user settings, such as a second agent hook.
	Leftovers func() []string
	// WritePaths names paths outside root this mount's session may still write, such as its own memory store.
	WritePaths func(root string) []string
	// ReviewerWhy says where review lands and why, when the overlay opts the reviewer onto the local machine.
	ReviewerWhy func(plan string) string
	// Deferred, when set, says why the mount is kept but unusable: nothing installs, selects, or renders it.
	Deferred string
	// Contract builds this mount's session driver for one worktree; nil when the mount cannot drive the conductor.
	Contract func(root, worktree string) Contract
	// FuzzTargets names this mount's own fuzz functions, which the gate runs alongside its own.
	FuzzTargets []FuzzTarget
}

// FuzzTarget is one mount's own fuzz function and the package that holds it.
type FuzzTarget struct {
	Name    string
	Package string
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

// Active returns every registered mount that is not deferred, sorted by name.
func Active() []Host {
	var out []Host
	for _, host := range Hosts() {
		if host.Deferred == "" {
			out = append(out, host)
		}
	}
	return out
}

// ConcurrencyFor is the named plan's lanes, from the first active mount that owns a Concurrency func.
func ConcurrencyFor(plan string) int {
	for _, host := range Active() {
		if host.Concurrency != nil {
			return host.Concurrency(plan)
		}
	}
	return 1
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

// WritePaths are every path outside root a registered mount's session may still write.
func WritePaths(root string) []string {
	var out []string
	for _, host := range Hosts() {
		if host.WritePaths != nil {
			out = append(out, host.WritePaths(root)...)
		}
	}
	return out
}

// FuzzTargets are every registered mount's own fuzz target, which the gate runs alongside its own.
func FuzzTargets() []FuzzTarget {
	var out []FuzzTarget
	for _, host := range Hosts() {
		out = append(out, host.FuzzTargets...)
	}
	return out
}

// ProjectPaths lists every path a registered mount renders under root as a project copy, relative and
// slash-separated; a deferred or uninstalled host is skipped, so neither is rendered just to be listed.
func ProjectPaths(root string) []string {
	seen := map[string]bool{}
	var out []string
	for _, host := range Active() {
		if host.Render == nil {
			continue
		}
		if host.Installed != nil && !host.Installed(root) {
			continue
		}
		plan, err := host.Render(root, BinaryPath())
		if err != nil {
			continue
		}
		for _, change := range plan.Project().Changes {
			rel, err := filepath.Rel(root, change.Path)
			if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
				continue
			}
			rel = filepath.ToSlash(rel)
			if !seen[rel] {
				seen[rel] = true
				out = append(out, rel)
			}
		}
	}
	sort.Strings(out)
	return out
}

// Executable resolves the running binary's own path with symlinks followed; a test swaps it.
var Executable = func() (string, error) {
	path, err := os.Executable()
	if err != nil {
		return "", err
	}
	return filepath.EvalSymlinks(path)
}

// BinaryPath is the running toolkit binary's absolute path, or bin/<name> relative to the main checkout under go run.
func BinaryPath() string {
	name := "komodo"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	if path, err := Executable(); err == nil && !goRunBuild(path) {
		if abs, err := filepath.Abs(path); err == nil {
			return abs
		}
	}
	return filepath.Join("bin", name)
}

// goRunBuild reports a binary the go tool built into a throwaway go-build directory or the build cache.
func goRunBuild(path string) bool {
	for _, segment := range strings.Split(filepath.ToSlash(path), "/") {
		if strings.HasPrefix(segment, "go-build") {
			return true
		}
	}
	cache := os.Getenv("GOCACHE")
	if cache == "" || cache == "off" {
		return false
	}
	rel, err := filepath.Rel(cache, path)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

// MainCheckout is the checkout that owns root's git directory, so a worktree resolves to the repo it came from.
func MainCheckout(root string) string {
	out, err := git.Run(root, "rev-parse", "--path-format=absolute", "--git-common-dir")
	if err != nil {
		return root
	}
	return filepath.Dir(out)
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

// localProvider names the provider that answers on the machine running the harness itself.
const localProvider = "ollama"

// LocalName is the local provider's name as a profile row and a step action carry it.
const LocalName = localProvider

// LocalResult is one local call's answer: the JSON the schema described and the tokens it cost.
type LocalResult struct {
	Value     map[string]any
	TokensIn  int
	TokensOut int
}

// Local is the local machine as the harness reaches it: probes, limits, and one call, all owned by its mount.
type Local struct {
	Env       string
	Up        func() bool
	ModelName func() string
	Fits      func(chars int) bool
	Allowed   func(tools []string) bool
	Post      func(model, brief string, schema []byte) (LocalResult, error)
}

var local Local

// RegisterLocal installs the local machine's mount, which is how the binary learns to reach it.
func RegisterLocal(machine Local) {
	lock.Lock()
	defer lock.Unlock()
	local = machine
}

// LocalMachine returns the registered local mount, with safe answers for anything it left unset.
func LocalMachine() Local {
	lock.Lock()
	defer lock.Unlock()
	out := local
	if out.Up == nil {
		out.Up = func() bool { return false }
	}
	if out.ModelName == nil {
		out.ModelName = func() string { return "" }
	}
	if out.Fits == nil {
		out.Fits = func(int) bool { return true }
	}
	if out.Allowed == nil {
		out.Allowed = func(tools []string) bool {
			for _, tool := range tools {
				if tool == "write" || tool == "edit" || tool == "shell" {
					return false
				}
			}
			return true
		}
	}
	if out.Post == nil {
		out.Post = func(string, string, []byte) (LocalResult, error) {
			return LocalResult{}, errors.New("no local machine is mounted")
		}
	}
	return out
}

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
	Plan        string    `json:"plan"`
	FiveHour    float64   `json:"five_hour"`
	ResetsAt    time.Time `json:"resets_at"`
	ExtraUsage  bool      `json:"extra_usage"`
	BillingType string    `json:"billing_type"`
}

// GuardTools is what the guard needs from one mount: its tool names, extra paths, and denial encoding.
type GuardTools struct {
	WriteTools   map[string]bool
	PathFields   []string
	ShellTool    string
	CommandField string
	// SpawnTools are this host's agent-spawn tool names, the ones that may start a second session.
	SpawnTools map[string]bool
	// IsolationField is the input key a spawn call uses to ask for a separate worktree.
	IsolationField string
	ConfigPaths    []string
	// PrivatePatterns are regular expressions for text the host considers private, such as a session link.
	PrivatePatterns []string
	// Deny renders the host's denial payload; blocked marks a refusal that must also stop the session.
	Deny func(reason string, blocked bool) []byte
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

// GuardPrivatePatterns are every private-text pattern, such as a session link, a registered mount's guard refuses.
func GuardPrivatePatterns() []string {
	var out []string
	for _, tools := range GuardHosts() {
		out = append(out, tools.PrivatePatterns...)
	}
	return out
}

// OverlayCaps lowers a profile's brief slot caps; the overlay may only shrink one, never grow it.
type OverlayCaps struct {
	RepoRules   int `json:"repo_rules"`
	RepoContext int `json:"repo_context"`
	PerFile     int `json:"per_file"`
	FilesTotal  int `json:"files_total"`
	Standard    int `json:"standard"`
	Failure     int `json:"failure"`
}

// Overlay is ~/.komodo/config.json, the one shape every reader across guard, mount, and profile decodes strictly.
type Overlay struct {
	Local               bool              `json:"local"`
	LocalURL            string            `json:"local_url"`
	LocalModel          string            `json:"local_model"`
	LocalWindow         int               `json:"local_window"`
	LocalReviewer       bool              `json:"local_reviewer"`
	LocalReviewerRecall float64           `json:"local_reviewer_recall"`
	LocalConcurrency    int               `json:"local_concurrency"`
	Models              map[string]string `json:"models"`
	// Sandbox runs every headless shell command in the host's OS sandbox, confined to the worktree and temp.
	Sandbox bool `json:"sandbox"`
	// SandboxWrite adds paths a sandboxed command may write, such as a build cache outside the worktree.
	SandboxWrite []string `json:"sandbox_write"`
	// SandboxDomains pre-allows network hosts, since a headless run cannot answer the sandbox's prompt.
	SandboxDomains []string `json:"sandbox_domains"`
	// CriticalRefs names a ref the guard protects beyond its own defaults.
	CriticalRefs []string `json:"critical_refs"`
	// Mode narrows or widens which critical-ref rules the guard enforces.
	Mode string `json:"mode"`
	// Caps lowers a profile's brief slot caps; nil leaves every cap at its default.
	Caps *OverlayCaps `json:"caps"`
	// MaxParallel lowers a profile's task concurrency; nil leaves it at its default.
	MaxParallel *int `json:"max_parallel"`
	// ReviewRepairs lowers how many fix rounds a blocking review earns; nil leaves it at its default.
	ReviewRepairs *int `json:"review_repairs"`
	// PauseAt lowers the usage fraction that pauses a wave; nil or non-positive leaves it at its default.
	PauseAt *float64 `json:"pause_at"`
	// WarnAt lowers the usage fraction that warns before a wave; nil or non-positive leaves it at its default.
	WarnAt *float64 `json:"warn_at"`
}

// OverlayPath is where a developer's own overlay lives, or empty when there is no home.
func OverlayPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".komodo", "config.json")
}

// DecodeOverlayFile reads one overlay file; an absent file decodes to a zero Overlay with no
// error, a present but malformed one returns an error naming path.
func DecodeOverlayFile(path string) (Overlay, error) {
	var overlay Overlay
	if _, err := fsx.ReadStrictJSON(path, &overlay); err != nil {
		return Overlay{}, err
	}
	return overlay, nil
}

// LoadOverlay reads the overlay, tolerating its absence as an empty one; a malformed file panics
// naming its path, since a developer's own corrupted config must never silently fall back.
func LoadOverlay() Overlay {
	overlay, err := DecodeOverlayFile(OverlayPath())
	if err != nil {
		panic(err)
	}
	return overlay
}

// OverlayModel is the model the overlay names for one tier, or empty.
func OverlayModel(tier string) string {
	return LoadOverlay().Models[tier]
}

// defaultReviewerRecallBar is the recall a local reviewer must clear before it takes review.
const defaultReviewerRecallBar = 0.6

// ReviewerRecallBar is the recall bar the local reviewer must clear, which an overlay may only raise.
func ReviewerRecallBar() float64 {
	if bar := LoadOverlay().LocalReviewerRecall; bar > defaultReviewerRecallBar {
		return bar
	}
	return defaultReviewerRecallBar
}

// RecallPath is where komodo recall writes its scores, beside OverlayPath, or empty with no home.
func RecallPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".komodo", "recall.json")
}

// ReviewerRecall reads model's recorded recall and case count from RecallPath, ok false when there is none.
func ReviewerRecall(model string) (recall float64, cases int, ok bool) {
	data, err := os.ReadFile(RecallPath())
	if err != nil {
		return 0, 0, false
	}
	var scores map[string]struct {
		Recall float64 `json:"recall"`
		Cases  int     `json:"cases"`
	}
	if json.Unmarshal(data, &scores) != nil {
		return 0, 0, false
	}
	score, found := scores[model]
	if !found {
		return 0, 0, false
	}
	return score.Recall, score.Cases, true
}
