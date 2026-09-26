package claude

import (
	"bufio"
	"encoding/json"
	"io"
	"time"

	"komodo/internal/mount"
)

// streamLineCap bounds one stream-json line this mount reads, as sumTranscript does for a transcript line.
const streamLineCap = 8 << 20

// Event is one line the stream reports: the result event's totals and session ID, or a rate limit window.
type Event struct {
	Turns     int
	Usage     mount.TaskUsage
	CostUSD   float64
	RateLimit *mount.RateLimit
	SessionID string
}

// kindLine is the only field this mount reads before it knows which line it has.
type kindLine struct {
	Type string `json:"type"`
}

// resultLine is the result event's fields: the meter's totals and the ID a resume reuses (decision 0025).
type resultLine struct {
	NumTurns     int     `json:"num_turns"`
	SessionID    string  `json:"session_id"`
	TotalCostUSD float64 `json:"total_cost_usd"`
	Usage        struct {
		InputTokens         int `json:"input_tokens"`
		OutputTokens        int `json:"output_tokens"`
		CacheReadTokens     int `json:"cache_read_input_tokens"`
		CacheCreationTokens int `json:"cache_creation_input_tokens"`
	} `json:"usage"`
}

// rateWindow is one rate-limit window's utilisation and when it next resets.
type rateWindow struct {
	Utilization float64   `json:"utilization"`
	ResetsAt    time.Time `json:"resetsAt"`
}

// rateLimitLine is the rate_limit_event's fields: the five-hour and seven-day windows.
type rateLimitLine struct {
	FiveHour rateWindow `json:"five_hour"`
	SevenDay rateWindow `json:"seven_day"`
}

// Parse reads one stream-json session and reports the result event's totals and session ID, and
// every rate_limit_event, in order; any other line, such as a turn's message, is skipped.
func Parse(r io.Reader) <-chan Event {
	out := make(chan Event)
	go func() {
		defer close(out)
		scanner := bufio.NewScanner(r)
		scanner.Buffer(make([]byte, 0, 64*1024), streamLineCap)
		for scanner.Scan() {
			line := scanner.Bytes()
			if len(line) == 0 {
				continue
			}
			var kind kindLine
			if json.Unmarshal(line, &kind) != nil {
				continue
			}
			switch kind.Type {
			case "result":
				var parsed resultLine
				if json.Unmarshal(line, &parsed) == nil {
					out <- resultEvent(parsed)
				}
			case "rate_limit_event":
				var parsed rateLimitLine
				if json.Unmarshal(line, &parsed) == nil {
					out <- rateLimitEvent(parsed)
				}
			}
		}
	}()
	return out
}

// resultEvent turns a result line's totals into the stream's event.
func resultEvent(parsed resultLine) Event {
	return Event{
		Turns: parsed.NumTurns,
		Usage: mount.TaskUsage{
			TokensIn:     parsed.Usage.InputTokens + parsed.Usage.CacheCreationTokens,
			TokensOut:    parsed.Usage.OutputTokens,
			TokensCached: parsed.Usage.CacheReadTokens,
			Turns:        parsed.NumTurns,
		},
		CostUSD:   parsed.TotalCostUSD,
		SessionID: parsed.SessionID,
	}
}

// rateLimitEvent turns a rate_limit_event line into the stream's event, keyed to the five-hour reset,
// since the plan probe (limits.go) tracks only that window's reset too.
func rateLimitEvent(parsed rateLimitLine) Event {
	return Event{
		RateLimit: &mount.RateLimit{
			FiveHour: parsed.FiveHour.Utilization / 100,
			SevenDay: parsed.SevenDay.Utilization / 100,
			ResetsAt: parsed.FiveHour.ResetsAt,
		},
	}
}
