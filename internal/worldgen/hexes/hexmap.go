package hexmap

import (
	"fmt"

	"github.com/mdhender/drynn/internal/hexes"
	"github.com/mdhender/drynn/internal/prng"
)

// Axial is an alias so callers that already reference hexmap.Axial keep working.
type Axial = hexes.Axial

// Disk delegates to the canonical implementation in internal/hexes.
func Disk(r int) []Axial {
	return hexes.Disk(r)
}

// Capacity delegates to the canonical implementation in internal/hexes.
func Capacity(r int) int {
	return hexes.Capacity(r)
}

// Contains delegates to the canonical implementation in internal/hexes.
func Contains(r int, h Axial) bool {
	return hexes.Contains(r, h)
}

// System is a star system located at a hex, with one or more stars.
type System struct {
	Hex   Axial
	Stars int
}

// Generator creates systems inside a hex disk.
type Generator struct {
	rng *prng.PRNG
}

// NewGenerator returns a generator using the provided PRNG.
func NewGenerator(r *prng.PRNG) *Generator {
	return &Generator{rng: r}
}

// Generate creates up to n systems inside a disk of radius r.
//
// Algorithm:
//   - Enumerate every hex in the disk.
//   - Shuffle once.
//   - Consume each candidate at most once.
//   - If the candidate is within minimumDistance of an existing system:
//     merge into the nearest such system if merge is true
//   - If multiple systems are tied for nearest:
//     choose one at random
//   - Otherwise discard the candidate
//   - Else create a new one-star system.
//
// Generation stops when either:
//   - exactly n systems have been created, or
//   - all candidates have been exhausted.
//
// If the candidate list is exhausted before n systems can be placed,
// Generate returns the systems created so far along with an error.
func (g *Generator) Generate(r, n, minimumDistance int, merge bool) ([]System, error) {
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

	disk := Disk(r)
	if n > len(disk) && !merge {
		return nil, fmt.Errorf("number of systems %d exceeds disk capacity %d", n, len(disk))
	}

	g.rng.Shuffle(len(disk), func(i, j int) {
		disk[i], disk[j] = disk[j], disk[i]
	})

	systems := make([]System, 0, min(n, len(disk)))

	for _, candidate := range disk {
		if idx := g.nearestWithinDistance(systems, candidate, minimumDistance); idx >= 0 {
			if merge {
				systems[idx].Stars++
				if systems[idx].Stars > 5 {
					systems[idx].Stars = g.rng.Roll(2, 5)
				}
			}
			continue
		}

		systems = append(systems, System{Hex: candidate, Stars: 1})
		if len(systems) == n {
			return systems, nil
		}
	}

	return systems, fmt.Errorf(
		"could only place %d systems in radius-%d disk with minimumDistance=%d merge=%t",
		len(systems), r, minimumDistance, merge,
	)
}

// MustGenerate is like Generate but panics on error.
func (g *Generator) MustGenerate(r, n, minimumDistance int, merge bool) []System {
	systems, err := g.Generate(r, n, minimumDistance, merge)
	if err != nil {
		panic(err)
	}
	return systems
}

// TotalStars returns the total number of stars across all systems.
func TotalStars(systems []System) int {
	total := 0
	for _, s := range systems {
		total += s.Stars
	}
	return total
}

// CountMultiStar returns the number of systems containing more than one star.
func CountMultiStar(systems []System) int {
	total := 0
	for _, s := range systems {
		if s.Stars > 1 {
			total++
		}
	}
	return total
}

// nearestWithinDistance returns the index of the nearest system whose hex distance
// from candidate is <= limit. If multiple systems are tied for the nearest distance,
// one is chosen uniformly at random using the generator RNG.
func (g *Generator) nearestWithinDistance(systems []System, candidate Axial, limit int) int {
	bestDist := -1
	tied := make([]int, 0, 4)

	for i := range systems {
		d := systems[i].Hex.Distance(candidate)
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
	return tied[g.rng.Roll(0, len(tied)-1)]
}
