// Package ledger appends one line per station event and aggregates what those lines hold.
package ledger

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// File names, both under .komodo and both gitignored.
const (
	RunFile    = "line.jsonl"
	AdhocFile  = "adhoc.jsonl"
	MetricsFile = "metrics.jsonl"
	EventsFile  = "events.jsonl"
)

// Limits at which the ad hoc file is truncated by its next writer.
const (
	AdhocMaxAge   = 24 * time.Hour
	AdhocMaxBytes = 1 << 20
)

// Entry is one station event. An empty field is never a guess.
type Entry struct {
	At           time.Time `json:"at"`
	Run          string    `json:"run,omitempty"`
	Group        string    `json:"group,omitempty"`
	Task         string    `json:"task,omitempty"`
	Wave         int       `json:"wave,omitempty"`
	Station      string    `json:"station"`
	Role         string    `json:"role,omitempty"`
	Tier         string    `json:"tier,omitempty"`
	Host         string    `json:"host,omitempty"`
	Provider     string    `json:"provider,omitempty"`
	Model        string    `json:"model,omitempty"`
	Seconds      float64   `json:"seconds,omitempty"`
	TokensIn     int       `json:"tokens_in,omitempty"`
	TokensOut    int       `json:"tokens_out,omitempty"`
	TokensCached int       `json:"tokens_cached,omitempty"`
	Turns        int       `json:"turns,omitempty"`
	Outcome      string    `json:"outcome,omitempty"`
	FailureClass string    `json:"failure_class,omitempty"`
	Findings     int       `json:"findings,omitempty"`
	Lines        int       `json:"lines,omitempty"`
	Checks       []Check   `json:"checks,omitempty"`
}

// Check is one command a station ran, its kind (compile or verify), and its exit code.
type Check struct {
	Kind     string  `json:"kind"`
	Command  string  `json:"command"`
	ExitCode int     `json:"exit_code"`
	Seconds  float64 `json:"seconds,omitempty"`
}

// Metric is one line of metrics.jsonl: aggregated data for a run/group/stage combination.
type Metric struct {
	Run          string    `json:"run"`
	Group        string    `json:"group"`
	Stage        string    `json:"stage"`
	Start        time.Time `json:"start"`
	Duration     float64   `json:"duration"`
	Turns        int       `json:"turns"`
	Input        int       `json:"input"`
	Output       int       `json:"output"`
	CachedTokens int       `json:"cached_tokens"`
	Cost         *float64  `json:"cost,omitempty"`
	Outcome      string    `json:"outcome"`
}

// Event is one line of events.jsonl: escalations, stops, pauses for usage limits, and resumes.
type Event struct {
	At      time.Time `json:"at"`
	Run     string    `json:"run,omitempty"`
	Group   string    `json:"group,omitempty"`
	Task    string    `json:"task,omitempty"`
	Station string    `json:"station,omitempty"`
	Type    string    `json:"type"`
	Outcome string    `json:"outcome,omitempty"`
}

// Ledger writes and reads one repo's two metric files.
type Ledger struct {
	Dir string
}

// New returns the ledger under a repo's state directory.
func New(stateDir string) *Ledger { return &Ledger{Dir: stateDir} }

// path is the full path of one ledger file.
func (l *Ledger) path(name string) string { return filepath.Join(l.Dir, name) }

// Stamp appends one entry to the run's file, or to the ad hoc file when there is no run.
func (l *Ledger) Stamp(entry Entry) error {
	if entry.At.IsZero() {
		entry.At = time.Now().UTC()
	}
	name := RunFile
	if entry.Run == "" {
		name = AdhocFile
		if err := l.rotateAdhoc(); err != nil {
			return err
		}
	}
	if err := os.MkdirAll(l.Dir, 0o755); err != nil {
		return err
	}
	data, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	handle, err := os.OpenFile(l.path(name), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer handle.Close()
	_, err = handle.Write(append(data, '\n'))
	return err
}

// TruncateRun archives the previous run's file beside it as line.<run>.jsonl, then empties it,
// which intake does when a new run begins, so no run's stations are ever lost.
func (l *Ledger) TruncateRun() error {
	if err := os.MkdirAll(l.Dir, 0o755); err != nil {
		return err
	}
	if entries, err := l.Read(RunFile); err == nil && len(entries) > 0 && entries[0].Run != "" {
		archive := l.path("line." + entries[0].Run + ".jsonl")
		if _, err := os.Stat(archive); os.IsNotExist(err) {
			if err := os.Rename(l.path(RunFile), archive); err != nil {
				return err
			}
		}
	}
	return os.WriteFile(l.path(RunFile), nil, 0o644)
}

// rotateAdhoc empties the ad hoc file when its first line is stale or the file is too large.
func (l *Ledger) rotateAdhoc() error {
	info, err := os.Stat(l.path(AdhocFile))
	if err != nil {
		return nil
	}
	if info.Size() > AdhocMaxBytes {
		return os.WriteFile(l.path(AdhocFile), nil, 0o644)
	}
	entries, err := l.Read(AdhocFile)
	if err != nil || len(entries) == 0 {
		return nil
	}
	if time.Since(entries[0].At) > AdhocMaxAge {
		return os.WriteFile(l.path(AdhocFile), nil, 0o644)
	}
	return nil
}

// Read parses one ledger file, skipping any line that is not an entry.
func (l *Ledger) Read(name string) ([]Entry, error) {
	handle, err := os.Open(l.path(name))
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer handle.Close()
	var entries []Entry
	scanner := bufio.NewScanner(handle)
	scanner.Buffer(make([]byte, 0, 64*1024), 1<<20)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var entry Entry
		if json.Unmarshal([]byte(line), &entry) == nil {
			entries = append(entries, entry)
		}
	}
	return entries, scanner.Err()
}

// All reads both files, the run's first.
func (l *Ledger) All() ([]Entry, error) {
	run, err := l.Read(RunFile)
	if err != nil {
		return nil, err
	}
	adhoc, err := l.Read(AdhocFile)
	if err != nil {
		return nil, err
	}
	return append(run, adhoc...), nil
}

// WriteMetrics writes one metrics.jsonl line per stage and session, from qualifying entries.
func (l *Ledger) WriteMetrics(entries []Entry) error {
	if err := os.MkdirAll(l.Dir, 0o755); err != nil {
		return err
	}
	metrics := aggregateMetrics(entries)
	for _, m := range metrics {
		data, err := json.Marshal(m)
		if err != nil {
			return err
		}
		handle, err := os.OpenFile(l.path(MetricsFile), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
		if err != nil {
			return err
		}
		_, err = handle.Write(append(data, '\n'))
		handle.Close()
		if err != nil {
			return err
		}
	}
	return nil
}

// ReadMetrics parses metrics.jsonl, skipping any line that is not a metric.
func (l *Ledger) ReadMetrics() ([]Metric, error) {
	handle, err := os.Open(l.path(MetricsFile))
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer handle.Close()
	var metrics []Metric
	scanner := bufio.NewScanner(handle)
	scanner.Buffer(make([]byte, 0, 64*1024), 1<<20)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var m Metric
		if json.Unmarshal([]byte(line), &m) == nil {
			metrics = append(metrics, m)
		}
	}
	return metrics, scanner.Err()
}

// WriteEvents writes entries that represent events to events.jsonl.
func (l *Ledger) WriteEvents(entries []Entry) error {
	if err := os.MkdirAll(l.Dir, 0o755); err != nil {
		return err
	}
	events := extractEvents(entries)
	for _, e := range events {
		data, err := json.Marshal(e)
		if err != nil {
			return err
		}
		handle, err := os.OpenFile(l.path(EventsFile), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
		if err != nil {
			return err
		}
		_, err = handle.Write(append(data, '\n'))
		handle.Close()
		if err != nil {
			return err
		}
	}
	return nil
}

// ReadEvents parses events.jsonl, skipping any line that is not an event.
func (l *Ledger) ReadEvents() ([]Event, error) {
	handle, err := os.Open(l.path(EventsFile))
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer handle.Close()
	var events []Event
	scanner := bufio.NewScanner(handle)
	scanner.Buffer(make([]byte, 0, 64*1024), 1<<20)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var e Event
		if json.Unmarshal([]byte(line), &e) == nil {
			events = append(events, e)
		}
	}
	return events, scanner.Err()
}

// Metrics is what the two files aggregate to.
type Metrics struct {
	MedianSeconds map[string]float64 `json:"median_seconds_per_station"`
	FailuresBy    map[string]int     `json:"failures_by_class"`
	TokensByModel map[string]int     `json:"tokens_by_model"`
	FindingsBy    map[string]int     `json:"findings_by_group"`
	Tasks         int                `json:"tasks"`
	Repairs       int                `json:"repairs"`
	RepairRate    float64            `json:"repair_rate"`
	TasksPerHour  float64            `json:"tasks_per_hour"`
	MedianTaskSec float64            `json:"median_wall_seconds_per_task"`
	TokensPerLine map[string]float64 `json:"tokens_per_changed_line_by_model"`
	RepairByTier  map[string]float64 `json:"repair_rate_by_tier"`
}

// Aggregate reduces entries to the numbers the metrics command prints.
func Aggregate(entries []Entry) Metrics {
	metrics := Metrics{
		MedianSeconds: map[string]float64{}, FailuresBy: map[string]int{},
		TokensByModel: map[string]int{}, FindingsBy: map[string]int{},
	}
	seconds := map[string][]float64{}
	tasks := map[string]bool{}
	repaired := map[string]bool{}
	for _, entry := range entries {
		if entry.Seconds > 0 {
			seconds[entry.Station] = append(seconds[entry.Station], entry.Seconds)
		}
		if entry.FailureClass != "" {
			metrics.FailuresBy[entry.FailureClass]++
		}
		if key := modelKey(entry); key != "" {
			metrics.TokensByModel[key] += entry.TokensIn + entry.TokensOut
		}
		if entry.Findings > 0 && entry.Group != "" {
			metrics.FindingsBy[entry.Group] += entry.Findings
		}
		if entry.Station == "close" && entry.Task != "" {
			tasks[entry.Task] = true
			if entry.Outcome == "repair" {
				repaired[entry.Task] = true
			}
		}
	}
	for station, values := range seconds {
		metrics.MedianSeconds[station] = median(values)
	}
	metrics.Tasks = len(tasks)
	metrics.Repairs = len(repaired)
	if metrics.Tasks > 0 {
		metrics.RepairRate = float64(metrics.Repairs) / float64(metrics.Tasks)
	}
	metrics.TasksPerHour, metrics.MedianTaskSec = pace(entries)
	metrics.TokensPerLine = tokensPerLine(entries)
	metrics.RepairByTier = repairByTier(entries)
	return metrics
}

// shipKey names one group's ship within one run.
func shipKey(entry Entry) string { return entry.Run + "/" + entry.Group }

// shippedRow reports whether an entry is a group's ship that landed as done or a handoff.
func shippedRow(entry Entry) bool {
	return entry.Station == "ship" && entry.Group != "" && (entry.Outcome == "done" || entry.Outcome == "handoff")
}

// pace is tasks done per run hour, and median seconds from a task's first brief to its group's ship.
func pace(entries []Entry) (perHour, medianTask float64) {
	first, last := map[string]time.Time{}, map[string]time.Time{}
	shipped := map[string]time.Time{}
	briefed := map[string]Entry{}
	done := map[string]bool{}
	for _, entry := range entries {
		if entry.Run == "" || entry.At.IsZero() {
			continue
		}
		if at, ok := first[entry.Run]; !ok || entry.At.Before(at) {
			first[entry.Run] = entry.At
		}
		if entry.At.After(last[entry.Run]) {
			last[entry.Run] = entry.At
		}
		key := entry.Run + "/" + entry.Task
		switch {
		case shippedRow(entry):
			if entry.At.After(shipped[shipKey(entry)]) {
				shipped[shipKey(entry)] = entry.At
			}
		case entry.Station == "brief" && entry.Task != "" && entry.Group != "":
			if prior, ok := briefed[key]; !ok || entry.At.Before(prior.At) {
				briefed[key] = entry
			}
		case entry.Station == "close" && entry.Task != "" && entry.Outcome == "done":
			done[key] = true
		}
	}
	var hours float64
	for run, start := range first {
		hours += last[run].Sub(start).Hours()
	}
	if hours > 0 {
		perHour = float64(len(done)) / hours
	}
	var walls []float64
	for _, brief := range briefed {
		if at, ok := shipped[shipKey(brief)]; ok && at.After(brief.At) {
			walls = append(walls, at.Sub(brief.At).Seconds())
		}
	}
	return perHour, median(walls)
}

// tokensPerLine divides each model's tokens by the changed lines of the shipped groups it worked on.
func tokensPerLine(entries []Entry) map[string]float64 {
	lines := map[string]int{}
	for _, entry := range entries {
		if shippedRow(entry) && entry.Lines > 0 {
			lines[shipKey(entry)] = entry.Lines
		}
	}
	tokens := map[string]int{}
	spent := map[string]map[string]bool{}
	for _, entry := range entries {
		key := modelKey(entry)
		if key == "" || entry.TokensIn+entry.TokensOut == 0 || lines[shipKey(entry)] == 0 {
			continue
		}
		tokens[key] += entry.TokensIn + entry.TokensOut
		if spent[key] == nil {
			spent[key] = map[string]bool{}
		}
		spent[key][shipKey(entry)] = true
	}
	out := map[string]float64{}
	for model, total := range tokens {
		changed := 0
		for group := range spent[model] {
			changed += lines[group]
		}
		out[model] = float64(total) / float64(changed)
	}
	return out
}

// repairByTier is the share of closed tasks that needed a repair, by the tier their build ran on.
func repairByTier(entries []Entry) map[string]float64 {
	tier := map[string]string{}
	closed := map[string]bool{}
	repaired := map[string]bool{}
	for _, entry := range entries {
		if entry.Task == "" {
			continue
		}
		key := entry.Run + "/" + entry.Task
		switch {
		case entry.Station == "build" && entry.Tier != "" && tier[key] == "":
			tier[key] = entry.Tier
		case entry.Station == "close":
			closed[key] = true
			if entry.Outcome == "repair" {
				repaired[key] = true
			}
		}
	}
	counts, repairs := map[string]int{}, map[string]int{}
	for key := range closed {
		name := tier[key]
		if name == "" {
			continue
		}
		counts[name]++
		if repaired[key] {
			repairs[name]++
		}
	}
	out := map[string]float64{}
	for name, count := range counts {
		out[name] = float64(repairs[name]) / float64(count)
	}
	return out
}

// modelKey names who spent tokens: the model when a station set one, else the host, labelled as such.
func modelKey(entry Entry) string {
	if entry.Model != "" {
		return entry.Model
	}
	if entry.Host != "" {
		return entry.Host + " (host, no model)"
	}
	return ""
}

// median returns the middle value, averaging the two middles of an even count.
func median(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	sorted := append([]float64(nil), values...)
	sort.Float64s(sorted)
	middle := len(sorted) / 2
	if len(sorted)%2 == 1 {
		return sorted[middle]
	}
	return (sorted[middle-1] + sorted[middle]) / 2
}

// Render writes the metrics as the text the command prints.
func Render(metrics Metrics) string {
	var out []string
	out = append(out, fmt.Sprintf("%d task(s), %d repaired, repair rate %.0f%%.",
		metrics.Tasks, metrics.Repairs, metrics.RepairRate*100))
	out = append(out, "", "## Median seconds per station")
	for _, station := range sortedKeys(metrics.MedianSeconds) {
		out = append(out, fmt.Sprintf("- **%s** %.1fs", station, metrics.MedianSeconds[station]))
	}
	if len(metrics.FailuresBy) > 0 {
		out = append(out, "", "## Failures by class")
		for _, class := range sortedIntKeys(metrics.FailuresBy) {
			out = append(out, fmt.Sprintf("- **%s** %d", class, metrics.FailuresBy[class]))
		}
	}
	if len(metrics.TokensByModel) > 0 {
		out = append(out, "", "## Tokens by model")
		for _, model := range sortedIntKeys(metrics.TokensByModel) {
			out = append(out, fmt.Sprintf("- **%s** %d", model, metrics.TokensByModel[model]))
		}
	}
	if len(metrics.FindingsBy) > 0 {
		out = append(out, "", "## Findings by group")
		for _, group := range sortedIntKeys(metrics.FindingsBy) {
			out = append(out, fmt.Sprintf("- **%s** %d", group, metrics.FindingsBy[group]))
		}
	}
	out = append(out, "", "## Tasks per hour")
	if metrics.TasksPerHour > 0 {
		out = append(out, fmt.Sprintf("- **line time** %.2f", metrics.TasksPerHour))
	} else {
		out = append(out, "- **none yet** no shipped group is on record yet")
	}
	if metrics.MedianTaskSec > 0 {
		out = append(out, "", "## Median wall seconds per task", fmt.Sprintf("- **brief to ship** %.0fs", metrics.MedianTaskSec))
	}
	if len(metrics.TokensPerLine) > 0 {
		out = append(out, "", "## Tokens per changed line")
		for _, model := range sortedKeys(metrics.TokensPerLine) {
			out = append(out, fmt.Sprintf("- **%s** %.1f", model, metrics.TokensPerLine[model]))
		}
	}
	if len(metrics.RepairByTier) > 0 {
		out = append(out, "", "## Repair rate by tier")
		for _, tier := range sortedKeys(metrics.RepairByTier) {
			out = append(out, fmt.Sprintf("- **%s** %.0f%%", tier, metrics.RepairByTier[tier]*100))
		}
	}
	return strings.Join(out, "\n") + "\n"
}

// aggregateMetrics converts each qualifying entry to its own metric, one line per stage and session.
func aggregateMetrics(entries []Entry) []Metric {
	var metrics []Metric
	for _, e := range entries {
		if e.Run == "" || e.Group == "" || e.Station == "" {
			continue
		}
		metrics = append(metrics, Metric{
			Run:          e.Run,
			Group:        e.Group,
			Stage:        e.Station,
			Start:        e.At,
			Duration:     e.Seconds,
			Turns:        e.Turns,
			Input:        e.TokensIn,
			Output:       e.TokensOut,
			CachedTokens: e.TokensCached,
			Outcome:      e.Outcome,
		})
	}
	return metrics
}

// extractEvents returns entries that represent notable events.
func extractEvents(entries []Entry) []Event {
	var events []Event
	for _, e := range entries {
		if isEvent(e) {
			ev := Event{
				At:      e.At,
				Run:     e.Run,
				Group:   e.Group,
				Task:    e.Task,
				Station: e.Station,
				Outcome: e.Outcome,
			}
			if e.Outcome == "escalated" {
				ev.Type = "escalation"
			} else if e.Outcome == "paused" {
				ev.Type = "pause"
			} else if e.Outcome == "resumed" {
				ev.Type = "resume"
			}
			events = append(events, ev)
		}
	}
	return events
}

// isEvent returns true if an entry represents a notable event.
func isEvent(e Entry) bool {
	return e.Outcome == "escalated" || e.Outcome == "paused" || e.Outcome == "resumed"
}

// sortedKeys orders the keys of a float map.
func sortedKeys(values map[string]float64) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

// sortedIntKeys orders the keys of an integer map.
func sortedIntKeys(values map[string]int) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
