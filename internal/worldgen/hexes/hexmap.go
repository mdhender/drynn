package hexmap

import (
	"fmt"

	"github.com/mdhender/drynn/internal/prng"
)

// Axial identifies a hex using axial coordinates (Q, R).
// The implied cube coordinate is S = -Q - R.
type Axial struct {
	Q int
	R int
}

// S returns the implied cube component.
func (h Axial) S() int {
	return -h.Q - h.R
}

// Distance returns the hex distance between two axial coordinates.
func (h Axial) Distance(other Axial) int {
	dq := h.Q - other.Q
	dr := h.R - other.R
	ds := h.S() - other.S()
	return max3(abs(dq), abs(dr), abs(ds))
}

// Neighbors returns the six adjacent hexes.
func (h Axial) Neighbors() [6]Axial {
	return [6]Axial{
		{Q: h.Q + 1, R: h.R},
		{Q: h.Q + 1, R: h.R - 1},
		{Q: h.Q, R: h.R - 1},
		{Q: h.Q - 1, R: h.R},
		{Q: h.Q - 1, R: h.R + 1},
		{Q: h.Q, R: h.R + 1},
	}
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

// NewGenerator returns a generator using math/rand/v2 and PCG seeded
// with the fixed values (20, 20).
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

	hexes := Disk(r)
	if n > len(hexes) && !merge {
		return nil, fmt.Errorf("number of systems %d exceeds disk capacity %d", n, len(hexes))
	}

	g.rng.Shuffle(len(hexes), func(i, j int) {
		hexes[i], hexes[j] = hexes[j], hexes[i]
	})

	systems := make([]System, 0, min(n, len(hexes)))

	for _, candidate := range hexes {
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

// Disk returns all axial hexes in a disk of radius r centered at (0,0).
//
// Ordering is deterministic: increasing q, then increasing r within q.
func Disk(r int) []Axial {
	if r < 0 {
		return nil
	}

	hexes := make([]Axial, 0, Capacity(r))
	for q := -r; q <= r; q++ {
		rMin := max(-r, -q-r)
		rMax := min(r, -q+r)
		for rr := rMin; rr <= rMax; rr++ {
			hexes = append(hexes, Axial{Q: q, R: rr})
		}
	}
	return hexes
}

// Capacity returns the number of hexes in a disk of radius r.
func Capacity(r int) int {
	if r < 0 {
		return 0
	}
	return 1 + 3*r*(r+1)
}

// Contains reports whether h lies inside the disk of radius r centered at (0,0).
func Contains(r int, h Axial) bool {
	return h.Distance(Axial{}) <= r
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

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func max3(a, b, c int) int {
	if a < b {
		a = b
	}
	if a < c {
		a = c
	}
	return a
}
