package burst

import (
	"testing"
	"time"
)

func TestGroup(t *testing.T) {
	base := time.Date(2026, 10, 26, 17, 0, 0, 0, time.Local)
	at := func(sec float64) time.Time { return base.Add(time.Duration(sec * float64(time.Second))) }
	// Two quick frames, a lone shot, an unknown time, then a chain of three
	// where each frame is within the gap of the one before.
	got := Group([]time.Time{at(0), at(0.17), at(30), {}, at(60), at(61.7), at(63.5), at(70)})
	want := []Member{{1, 1, 2}, {1, 2, 2}, {}, {}, {2, 1, 3}, {2, 2, 3}, {2, 3, 3}, {}}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("photo %d: got %+v, want %+v", i, got[i], want[i])
		}
	}
}
