// Package review holds the lenses, their findings, and the evidence check that decides which finding blocks.
package review

// Kind names the validator that took one measurement.
type Kind string

// KindCallers is the one validator kind a finding's evidence can cite: a caller count.
const KindCallers Kind = "callers"

// Measurement is what one validator found.
type Measurement struct {
	Kind    Kind   `json:"kind"`
	Command string `json:"command,omitempty"`
	Passed  bool   `json:"passed"`
	Detail  string `json:"detail"`
}

// Report is every measurement, in the order the validators ran.
type Report struct {
	Measurements []Measurement `json:"measurements"`
}
