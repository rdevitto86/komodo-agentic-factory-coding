package hooks

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// warnShare is the share of a session's time or turns at which the warning fires.
const warnShare = 0.8

// errNoClock is a call whose session has no clock the hook can read.
var errNoClock = errors.New("no session clock")

// sessionClock is what the hook keeps per session: when it first saw the session, its turns, and whether it warned.
type sessionClock struct {
	Started time.Time `json:"started"`
	Turns   int       `json:"turns"`
	Warned  bool      `json:"warned"`
}

// warnTime counts one more turn and warns once when the session has used 80% of its time or turns.
func warnTime(_ context.Context, in Input) (Outcome, error) {
	if in.SessionID == "" || in.Root == "" || (in.Budget.Minutes <= 0 && in.Budget.Turns <= 0) {
		return Outcome{}, errNoClock
	}
	path := filepath.Join(in.Root, ".komodo", "runs", "hook-clock", filepath.Base(in.SessionID)+".json")
	clock := sessionClock{Started: time.Now()}
	if data, err := os.ReadFile(path); err == nil {
		if err := json.Unmarshal(data, &clock); err != nil {
			return Outcome{}, fmt.Errorf("%w: %v", errNoClock, err)
		}
	} else if !os.IsNotExist(err) {
		return Outcome{}, fmt.Errorf("%w: %v", errNoClock, err)
	}
	clock.Turns++

	elapsed := time.Since(clock.Started)
	limit := time.Duration(in.Budget.Minutes) * time.Minute
	late := limit > 0 && elapsed.Seconds() >= warnShare*limit.Seconds()
	busy := in.Budget.Turns > 0 && float64(clock.Turns) >= warnShare*float64(in.Budget.Turns)
	out := Outcome{Verdict: Allow}
	if (late || busy) && !clock.Warned {
		clock.Warned = true
		out = Outcome{Verdict: Inform, Message: fmt.Sprintf(
			"This session has used %d of %d minutes and %d of %d turns. "+
				"Finish the task in hand, run its checks, write the result, and stop.",
			int(elapsed.Minutes()), in.Budget.Minutes, clock.Turns, in.Budget.Turns)}
	}
	if err := writeState(path, clock); err != nil {
		return Outcome{}, err
	}
	return out, nil
}
