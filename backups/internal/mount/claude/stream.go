package claude

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"time"

	"komodo/internal/mount"
)

// streamLineCap bounds one stream-json line this mount reads.
const streamLineCap = 8 << 20

// kindLine is the only field this mount reads before it knows which line it has.
type kindLine struct {
	Type string `json:"type"`
}

// resultLine is the result event's fields: the meter's totals and the ID a resume reuses.
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
	Utilization float64         `json:"utilization"`
	ResetsAt    json.RawMessage `json:"resetsAt"`
}

// rateLimitLine is the rate_limit_event's fields: the five-hour and seven-day windows.
type rateLimitLine struct {
	FiveHour rateWindow `json:"five_hour"`
	SevenDay rateWindow `json:"seven_day"`
}

// Parse reads one stream-json session and reports the result event's totals and every
// rate_limit_event; a cancelled ctx stops it mid-send, and a scanner error ends it as the final Err.
func Parse(ctx context.Context, r io.Reader) <-chan mount.Event {
	out := make(chan mount.Event)
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
			var event mount.Event
			switch kind.Type {
			case "result":
				var parsed resultLine
				if json.Unmarshal(line, &parsed) != nil {
					continue
				}
				event = resultEvent(parsed)
			case "rate_limit_event":
				var parsed rateLimitLine
				if json.Unmarshal(line, &parsed) != nil {
					continue
				}
				event = rateLimitEvent(parsed)
			default:
				continue
			}
			select {
			case out <- event:
			case <-ctx.Done():
				return
			}
		}
		if err := scanner.Err(); err != nil {
			select {
			case out <- mount.Event{Err: err}:
			case <-ctx.Done():
			}
		}
	}()
	return out
}

// resultEvent turns a result line's totals into the stream's event.
func resultEvent(parsed resultLine) mount.Event {
	return mount.Event{
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

// rateLimitEvent turns a rate_limit_event line into the stream's event, keyed to the five-hour reset.
func rateLimitEvent(parsed rateLimitLine) mount.Event {
	return mount.Event{
		RateLimit: &mount.RateLimit{
			FiveHour: parsed.FiveHour.Utilization / 100,
			SevenDay: parsed.SevenDay.Utilization / 100,
			ResetsAt: parseResetsAt(parsed.FiveHour.ResetsAt),
		},
	}
}

// parseResetsAt reads a reset time from an RFC3339 string or epoch seconds, zero when neither parses.
func parseResetsAt(raw json.RawMessage) time.Time {
	var text string
	if json.Unmarshal(raw, &text) == nil {
		parsed, _ := time.Parse(time.RFC3339, text)
		return parsed
	}
	var epoch int64
	if json.Unmarshal(raw, &epoch) == nil {
		return time.Unix(epoch, 0)
	}
	return time.Time{}
}
