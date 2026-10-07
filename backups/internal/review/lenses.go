package review

// Lens names one review lens: the brief section it reads, and the key its state is kept under.
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
