package review

// Lens names one review lens: the checklist skill its session loads, and the key its state is kept under.
type Lens string

// The lenses full mode runs in parallel, and the one combined lens economy mode runs alone.
const (
	Correctness Lens = "correctness"
	Security    Lens = "security"
	Quality     Lens = "quality"
	Economy     Lens = "economy"
)

// economyMode is the profile mode that folds every lens into one session.
const economyMode = "economy"

// ForMode returns the lenses a profile mode runs: the combined lens in economy mode, else all three.
func ForMode(mode string) []Lens {
	if mode == economyMode {
		return []Lens{Economy}
	}
	return []Lens{Correctness, Security, Quality}
}

// Skill names the checklist skill a lens's session loads.
func (l Lens) Skill() string {
	return "review-" + string(l)
}
