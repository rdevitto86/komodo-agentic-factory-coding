// Package ledger appends one line per station event and aggregates what those lines hold.
package ledger

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// File names, both under .komodo and both gitignored.
const (
	RunFile   = "line.jsonl"
	AdhocFile = "adhoc.jsonl"
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
	Turns        int       `json:"turns,omitempty"`
	Outcome      string    `json:"outcome,omitempty"`
	FailureClass string    `json:"failure_class,omitempty"`
	Findings     int       `json:"findings,omitempty"`
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

// TruncateRun empties the run's file, which intake does when a new run begins.
func (l *Ledger) TruncateRun() error {
	if err := os.MkdirAll(l.Dir, 0o755); err != nil {
		return err
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

// Metrics is what the two files aggregate to.
type Metrics struct {
	MedianSeconds map[string]float64 `json:"median_seconds_per_station"`
	FailuresBy    map[string]int     `json:"failures_by_class"`
	TokensByModel map[string]int     `json:"tokens_by_model"`
	FindingsBy    map[string]int     `json:"findings_by_group"`
	Tasks         int                `json:"tasks"`
	Repairs       int                `json:"repairs"`
	RepairRate    float64            `json:"repair_rate"`
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
		if entry.Model != "" {
			metrics.TokensByModel[entry.Model] += entry.TokensIn + entry.TokensOut
		}
		if entry.Findings > 0 && entry.Group != "" {
			metrics.FindingsBy[entry.Group] += entry.Findings
		}
		if entry.Task != "" {
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
	return metrics
}

// median returns the middle value, averaging the two middles of an even count.
func median(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	sorted := append([]float64(nil), values...)
	for index := 1; index < len(sorted); index++ {
		for back := index; back > 0 && sorted[back] < sorted[back-1]; back-- {
			sorted[back], sorted[back-1] = sorted[back-1], sorted[back]
		}
	}
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
	return strings.Join(out, "\n") + "\n"
}

// sortedKeys orders the keys of a float map.
func sortedKeys(values map[string]float64) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sortStrings(keys)
	return keys
}

// sortedIntKeys orders the keys of an integer map.
func sortedIntKeys(values map[string]int) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sortStrings(keys)
	return keys
}

// sortStrings orders a small slice in place.
func sortStrings(keys []string) {
	for index := 1; index < len(keys); index++ {
		for back := index; back > 0 && keys[back] < keys[back-1]; back-- {
			keys[back], keys[back-1] = keys[back-1], keys[back]
		}
	}
}
