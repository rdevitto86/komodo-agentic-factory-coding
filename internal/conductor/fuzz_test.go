package conductor

import "testing"

// states is every GroupState Next can be given, in table order.
var states = []GroupState{
	Ready, Building, Checking, Reviewing, Repairing, Preparing, Shipping, Shipped, Escalated, Blocked,
}

// FuzzNext proves Next never panics and always answers with a known state, or removal from Shipped.
func FuzzNext(f *testing.F) {
	f.Add([]byte{0, 1, 2, 3, 4, 5, 6, 7})
	f.Add([]byte{255, 255, 255, 255, 255, 255, 255, 255, 255, 255})
	f.Add([]byte{})
	f.Fuzz(func(t *testing.T, data []byte) {
		bit := func(index int) bool { return index < len(data) && data[index]&1 == 1 }
		byteAt := func(index int) byte {
			if index < len(data) {
				return data[index]
			}
			return 0
		}
		s := State{
			Group:        "TG-1",
			Current:      states[int(byteAt(0))%len(states)],
			SlotFree:     bit(1),
			SessionDone:  bit(2),
			ChecksPassed: bit(3),
			Conflict:     bit(4),
			ShipDone:     bit(5),
			Merged:       bit(6),
			Escalate:     bit(7),
			Answered:     bit(8),
			Stop:         bit(9),
			Left:         states[int(byteAt(10))%len(states)],
			Edited:       bit(11),
		}
		if bit(12) {
			s.Findings = []Finding{{Severity: "high", Verified: bit(13)}}
		}
		next := Next(s)
		if next.Remove {
			return
		}
		for _, known := range states {
			if next.Move == known {
				return
			}
		}
		t.Fatalf("Next(%+v) = %+v, whose move is not a known state", s, next)
	})
}
