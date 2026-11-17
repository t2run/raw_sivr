// Package burst groups photos that were shot in quick succession, so the
// best frame of each group can be kept and the rest rejected together.
package burst

import (
	"sort"
	"time"
)

// Gap is the longest pause between two frames of the same burst.
const Gap = 2 * time.Second

// Member places one photo in a burst.
type Member struct {
	Group int // 1-based burst number; 0 when the photo is on its own
	Pos   int // 1-based position inside the burst
	Size  int // number of photos in the burst
}

// Group takes the capture time of each photo (zero when unknown) and
// returns, for the same indexes, where each photo sits in a burst.
func Group(times []time.Time) []Member {
	out := make([]Member, len(times))
	var order []int
	for i, t := range times {
		if !t.IsZero() {
			order = append(order, i)
		}
	}
	sort.SliceStable(order, func(a, b int) bool { return times[order[a]].Before(times[order[b]]) })

	group := 0
	for start := 0; start < len(order); {
		end := start + 1
		for end < len(order) && times[order[end]].Sub(times[order[end-1]]) <= Gap {
			end++
		}
		if n := end - start; n > 1 {
			group++
			for k := start; k < end; k++ {
				out[order[k]] = Member{Group: group, Pos: k - start + 1, Size: n}
			}
		}
		start = end
	}
	return out
}
