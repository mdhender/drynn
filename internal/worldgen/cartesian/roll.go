// Copyright (c) 2026 Michael D Henderson. All rights reserved.

package cartesian

import "math/rand/v2"

type Roller struct {
	r *rand.Rand
}

// roll(low, high) returns a uniformly distributed random integer in the range [low, high] (inclusive on both ends).
// intentionally ignores over and under flow.
func (r Roller) roll(low, high int) int {
	if high < low {
		low, high = high, low
	}
	delta := high - low + 1
	return low + r.r.IntN(delta)
}
