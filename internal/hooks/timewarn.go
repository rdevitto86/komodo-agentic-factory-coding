package hooks

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// warnShare is the share of a session's time or turns at which the warning fires.
const warnShare = 0.8

// clockLockTimeout bounds how long a call waits on another tool call's lock on the same clock file.
const clockLockTimeout = 2 * time.Second

// errNoClock is a call whose session has no clock the hook can read.
var errNoClock = errors.New("no session clock")

// sessionClock is what the hook keeps per session: when it first saw the session, its turns, and whether it warned.
type sessionClock struct {
	Started time.Time `json:"started"`
	Turns   int       `json:"turns"`
	Warned  bool      `json:"warned"`
}

// warnTime counts the session's turns and warns once at 80% of its time or turns; a transcript's
// assistant entries name a turn, so parallel tool calls in one turn share a count.
func warnTime(_ context.Context, in Input) (Outcome, error) {
	if in.SessionID == "" || in.Root == "" || (in.Budget.Minutes <= 0 && in.Budget.Turns <= 0) {
		return Outcome{}, errNoClock
	}
	path := filepath.Join(in.Root, ".komodo", "runs", "hook-clock", filepath.Base(in.SessionID)+".json")
	unlock, err := clockLock(path)
	if err != nil {
		return Outcome{}, fmt.Errorf("%w: %v", errNoClock, err)
	}
	defer unlock()

	clock := sessionClock{Started: time.Now()}
	if data, err := os.ReadFile(path); err == nil {
		if err := json.Unmarshal(data, &clock); err != nil {
			return Outcome{}, fmt.Errorf("%w: %v", errNoClock, err)
		}
	} else if !os.IsNotExist(err) {
		return Outcome{}, fmt.Errorf("%w: %v", errNoClock, err)
	}
	if turns, ok := countTurns(in.TranscriptPath); ok {
		if turns > clock.Turns {
			clock.Turns = turns
		}
	} else {
		clock.Turns++
	}

	elapsed := time.Since(clock.Started)
	limit := time.Duration(in.Budget.Minutes) * time.Minute
	late := limit > 0 && elapsed.Seconds() >= warnShare*limit.Seconds()
	busy := in.Budget.Turns > 0 && float64(clock.Turns) >= warnShare*float64(in.Budget.Turns)
	out := Outcome{Verdict: Allow}
	if (late || busy) && !clock.Warned {
		clock.Warned = true
		out = Outcome{Verdict: Inform, Message: warnMessage(elapsed, clock.Turns, in.Budget)}
	}
	if err := writeClockAtomic(path, clock); err != nil {
		return Outcome{}, err
	}
	return out, nil
}

// warnMessage builds the warning from only the budgets that are set, so an unset one never reads
// as zero of zero.
func warnMessage(elapsed time.Duration, turns int, budget Budget) string {
	var used []string
	if budget.Minutes > 0 {
		used = append(used, fmt.Sprintf("%d of %d minutes", int(elapsed.Minutes()), budget.Minutes))
	}
	if budget.Turns > 0 {
		used = append(used, fmt.Sprintf("%d of %d turns", turns, budget.Turns))
	}
	return fmt.Sprintf("This session has used %s. Finish the task in hand, run its checks, write the result, and stop.",
		strings.Join(used, " and "))
}

// countTurns reports the assistant turns a transcript has recorded, and whether it could read
// one; a call with no readable transcript falls back to counting itself as one more turn.
func countTurns(path string) (int, bool) {
	if path == "" {
		return 0, false
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, false
	}
	turns := 0
	for _, line := range strings.Split(strings.TrimRight(string(data), "\n"), "\n") {
		if line == "" {
			continue
		}
		var entry struct {
			Type string `json:"type"`
		}
		if json.Unmarshal([]byte(line), &entry) == nil && entry.Type == "assistant" {
			turns++
		}
	}
	return turns, true
}

// clockLock creates path's own lock file as the exclusive holder, retrying until it succeeds or
// clockLockTimeout passes, and returns the func that releases it.
func clockLock(path string) (func(), error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	lock := path + ".lock"
	deadline := time.Now().Add(clockLockTimeout)
	for {
		file, err := os.OpenFile(lock, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
		if err == nil {
			file.Close()
			return func() { os.Remove(lock) }, nil
		}
		if !os.IsExist(err) {
			return nil, err
		}
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("timed out waiting for the clock lock on %s", path)
		}
		time.Sleep(time.Millisecond)
	}
}

// writeClockAtomic writes the clock as a temp file renamed into place, so a reader never sees a
// write still in progress.
func writeClockAtomic(path string, clock sessionClock) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	data, err := json.Marshal(clock)
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, "clock-*.tmp")
	if err != nil {
		return err
	}
	name := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(name)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(name)
		return err
	}
	return os.Rename(name, path)
}
