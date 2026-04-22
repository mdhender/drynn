// Copyright (c) 2026 Michael D Henderson. All rights reserved.

package worldgen

import (
	"fmt"

	"github.com/mdhender/drynn/internal/hexes"
	"github.com/mdhender/drynn/internal/prng"
)

// hexPlacement is an intermediate result from hex placement.
type hexPlacement struct {
	Hex   hexes.Axial
	Stars int
}

// placeHexSystems creates up to n systems inside a disk of radius r.
//
// Algorithm:
//   - Enumerate every hex in the disk.
//   - Shuffle once.
//   - Consume each candidate at most once.
//   - If the candidate is within minimumDistance of an existing placement:
//     merge into the nearest such placement if merge is true
//   - If multiple placements are tied for nearest:
//     choose one at random
//   - Otherwise discard the candidate
//   - Else create a new one-star placement.
//
// On merge, a placement's star count is incremented and hard-capped at
// 5. Candidates that would merge into a 5-star placement are consumed
// without changing the star count.
//
// Generation stops when either:
//   - exactly n placements have been created, or
//   - all candidates have been exhausted.
//
// If the candidate list is exhausted before n placements can be placed,
// placeHexSystems returns the placements created so far along with an error.
func placeHexSystems(rng *prng.PRNG, r, n, minimumDistance int, merge bool) ([]hexPlacement, error) {
	if n < 0 {
		return nil, fmt.Errorf("number of systems must be non-negative")
	} else if n == 0 {
		return nil, nil
	}
	if r < 0 {
		return nil, fmt.Errorf("radius must be non-negative")
	}
	if minimumDistance < 0 {
		return nil, fmt.Errorf("minimumDistance must be non-negative")
	}

	disk := hexes.Disk(r)
	if n > len(disk) && !merge {
		return nil, fmt.Errorf("number of systems %d exceeds disk capacity %d", n, len(disk))
	}

	rng.Shuffle(len(disk), func(i, j int) {
		disk[i], disk[j] = disk[j], disk[i]
	})

	placements := make([]hexPlacement, 0, min(n, len(disk)))

	for _, candidate := range disk {
		if idx := nearestPlacementWithinDistance(rng, placements, candidate, minimumDistance); idx >= 0 {
			if merge && placements[idx].Stars < 5 {
				placements[idx].Stars++
			}
			continue
		}

		placements = append(placements, hexPlacement{Hex: candidate, Stars: 1})
		if len(placements) == n {
			return placements, nil
		}
	}

	return placements, fmt.Errorf(
		"could only place %d systems in radius-%d disk with minimumDistance=%d merge=%t",
		len(placements), r, minimumDistance, merge,
	)
}

// nearestPlacementWithinDistance returns the index of the nearest placement whose hex
// distance from candidate is <= limit. If multiple placements are tied for the nearest
// distance, one is chosen uniformly at random using the provided PRNG.
func nearestPlacementWithinDistance(rng *prng.PRNG, placements []hexPlacement, candidate hexes.Axial, limit int) int {
	bestDist := -1
	tied := make([]int, 0, 4)

	for i := range placements {
		d := placements[i].Hex.Distance(candidate)
		if d > limit {
			continue
		}
		if bestDist < 0 || d < bestDist {
			bestDist = d
			tied = tied[:0]
			tied = append(tied, i)
			continue
		}
		if d == bestDist {
			tied = append(tied, i)
		}
	}

	if len(tied) == 0 {
		return -1
	}
	if len(tied) == 1 {
		return tied[0]
	}
	return tied[rng.Roll(0, len(tied)-1)]
}

// TotalStars returns the total number of stars across all systems.
func TotalStars(systems []*System) int {
	total := 0
	for _, s := range systems {
		total += len(s.Stars)
	}
	return total
}

// CountMultiStar returns the number of systems containing more than one star.
func CountMultiStar(systems []*System) int {
	total := 0
	for _, s := range systems {
		if len(s.Stars) > 1 {
			total++
		}
	}
	return total
}
