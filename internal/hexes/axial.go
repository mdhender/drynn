package hexes

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
	return max(abs(dq), abs(dr), abs(ds))
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

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}
