// Copyright (c) 2026 Michael D Henderson. All rights reserved.

package cartesian

import (
	"cmp"
	"math"
	"slices"

	"github.com/mdhender/drynn/internal/prng"
)

// PointGenerator produces n points distributed within the closed unit ball
// centered at the origin.
//
// Invariant: every returned point p satisfies -1 ≤ p.x, p.y, p.z ≤ 1 and
// p.x² + p.y² + p.z² ≤ 1. The distribution (uniform, clustered, etc.) is
// defined by the implementation.
type PointGenerator interface {
	Generate(n int, r *prng.PRNG) []point
}

func GenerateSystems(set []point, radius float64) []Point {
	sortPoints(set)
	points := make([]Point, 0, len(set))
	for _, p := range set {
		points = append(points, Point{
			X: int(math.Round(p.x * radius)),
			Y: int(math.Round(p.y * radius)),
		})
	}
	return points
}

type Point struct {
	X, Y, Z int
}

// Distance returns the Euclidean distance between a and b.
func (p Point) Distance(b Point) int {
	return int(math.Ceil(math.Sqrt(float64(p.DistanceSquared(b)))))
}

// DistanceSquared returns the squared Euclidean distance between a and b.
// Prefer this over distance when only comparing magnitudes: it skips the
// sqrt and is monotonic with distance for non-negative values.
func (p Point) DistanceSquared(b Point) int {
	dx := p.X - b.X
	dy := p.X - b.Y
	dz := p.Z - b.Z
	return dx*dx + dy*dy + dz*dz
}

func (p Point) Less(q Point) bool {
	if p.X < q.X {
		return true
	}
	if p.X > q.X {
		return false
	}
	if p.Y < q.Y {
		return true
	}
	if p.Y > q.Y {
		return false
	}
	return p.Z <= q.Z
}

// NearestNeighbor returns the index of the point in set closest to set[n]
// by Euclidean distance, or -1 if set has fewer than two points. The
// comparison uses squared distance (monotonic with distance for non-negative
// values) to skip the sqrt. Ties resolve to the lowest index.
//
// Linear scan, O(len(set)). Fine for the 100–500 point sets this package
// works with; revisit with a spatial index if set sizes climb.
func NearestNeighbor(set []Point, n int) int {
	p := set[n]
	best := -1
	bestDistSq := -1
	for i, q := range set {
		if i == n {
			continue
		}
		if d := p.DistanceSquared(q); d < bestDistSq {
			bestDistSq = d
			best = i
		}
	}
	return best
}

// NoCloserThan reports whether every point in set other than set[n] lies
// at Euclidean distance ≥ d from set[p]. A non-positive d is vacuously
// satisfied (no point can be at a negative distance). Comparisons use
// squared distance to skip the sqrt.
func NoCloserThan(set []Point, n int, d int) bool {
	if d <= 0 {
		return true
	}
	dSq := d * d
	p := set[n]
	for i, q := range set {
		if i == n {
			continue
		}
		if p.DistanceSquared(q) < dSq {
			return false
		}
	}
	return true
}

// SortPoints sorts points in place by Euclidean distance from the origin,
// ascending. Ties are broken by x, then y, then z, all ascending.
//
// If two points agree on all four keys they are identical in every
// coordinate, so their relative order is incidental — not a bug and not
// nondeterminism. The comparison uses squared distance (monotonic with
// distance for non-negative values) to avoid sqrt.
func SortPoints(points []Point) {
	slices.SortFunc(points, func(a, b Point) int {
		da := a.X*a.X + a.Y*a.Y
		db := b.X*b.X + b.Y*b.Y
		if c := cmp.Compare(da, db); c != 0 {
			return c
		}
		if c := cmp.Compare(a.X, b.X); c != 0 {
			return c
		}
		return cmp.Compare(a.Y, b.Y)
	})
}

type point struct {
	x, y, z float64
}

// distance returns the Euclidean distance between a and b.
func distance(a, b point) float64 {
	return math.Sqrt(distanceSquared(a, b))
}

// distanceSquared returns the squared Euclidean distance between a and b.
// Prefer this over distance when only comparing magnitudes: it skips the
// sqrt and is monotonic with distance for non-negative values.
func distanceSquared(a, b point) float64 {
	dx := a.x - b.x
	dy := a.y - b.y
	dz := a.z - b.z
	return dx*dx + dy*dy + dz*dz
}

// nearestNeighbor returns the point in set closest to p
// by Euclidean distance, or -1 if set has no points. The
// comparison uses squared distance (monotonic with distance for non-negative
// values) to skip the sqrt. Ties resolve to the lowest index.
//
// Linear scan, O(len(set)). Fine for the 100–500 point sets this package
// works with; revisit with a spatial index if set sizes climb.
func nearestNeighbor(p point, set []point) int {
	best := -1
	bestDistSq := math.Inf(1)
	for i, q := range set {
		if d := distanceSquared(p, q); d < bestDistSq {
			bestDistSq = d
			best = i
		}
	}
	return best
}

// noCloserThan reports whether every point in set lies
// at Euclidean distance ≥ d from p. A non-positive d is vacuously
// satisfied (no point can be at a negative distance). Comparisons use
// squared distance to skip the sqrt.
func noCloserThan(p point, set []point, d float64) bool {
	if d <= 0 {
		return true
	}
	dSq := d * d
	for _, q := range set {
		if distanceSquared(p, q) < dSq {
			return false
		}
	}
	return true
}

func (p point) quantize(scale float64) Point {
	p = p.scale(scale)
	return Point{
		X: int(p.x),
		Y: int(p.y),
		Z: int(p.z),
	}
}

func (p point) scale(scale float64) point {
	return point{
		x: p.x * scale,
		y: p.y * scale,
		z: p.z * scale,
	}
}

// sortPoints sorts points in place by Euclidean distance from the origin,
// ascending. Ties are broken by x, then y, then z, all ascending.
//
// If two points agree on all four keys they are identical in every
// coordinate, so their relative order is incidental — not a bug and not
// nondeterminism. The comparison uses squared distance (monotonic with
// distance for non-negative values) to avoid sqrt.
func sortPoints(points []point) {
	slices.SortFunc(points, func(a, b point) int {
		da := a.x*a.x + a.y*a.y + a.z*a.z
		db := b.x*b.x + b.y*b.y + b.z*b.z
		if c := cmp.Compare(da, db); c != 0 {
			return c
		}
		if c := cmp.Compare(a.x, b.x); c != 0 {
			return c
		}
		if c := cmp.Compare(a.y, b.y); c != 0 {
			return c
		}
		return cmp.Compare(a.z, b.z)
	})
}

type NaiveDiskPointsGenerator struct{}

// Generate returns n points produced by uniformSpherePointsGenerator with
// every z coordinate flattened to zero. Mimics the legacy engine's flat
// (2D disk) placement while reusing the 3D sampler.
//
// The output still satisfies the pointGenerator invariant — every point
// lies in the closed unit ball, specifically on the z=0 cross-section —
// but the resulting 2D distribution on the unit disk is NOT uniform. It
// inherits the ball's marginal density along z=0, which is proportional
// to the chord length 2·√(1 − x² − y²): highest at the origin, zero at
// the rim. For uniform-on-disk placement use uniformDiskPointsGenerator
// — do not "fix" this one, its bias is the point.
func (pg NaiveDiskPointsGenerator) Generate(n int, r *prng.PRNG) []point {
	points := UniformSpherePointsGenerator{}.Generate(n, r)
	for i := range points {
		points[i].z = 0
	}
	return points
}

type NaiveSpherePointsGenerator struct{}

// Generate returns n points uniformly distributed in the closed unit ball
// via rejection sampling from the enclosing cube [-1, 1]³. Roughly 48% of
// candidates are rejected (the cube has volume 8 vs. the ball's 4π/3 ≈ 4.19),
// so expect ~1.91·n RNG draws per point on average.
func (pg NaiveSpherePointsGenerator) Generate(n int, r *prng.PRNG) []point {
	if n <= 0 {
		return nil
	}
	points := make([]point, 0, n)
	for len(points) < n {
		x := 2*r.Float64() - 1
		y := 2*r.Float64() - 1
		z := 2*r.Float64() - 1
		if x*x+y*y+z*z <= 1 {
			points = append(points, point{x: x, y: y, z: z})
		}
	}
	return points
}

type UniformDiskPointsGenerator struct{}

// Generate returns n points uniformly distributed on the closed unit disk
// (the z=0 cross-section of the unit ball) by direct sampling. Each point
// consumes exactly two RNG draws.
//
// Method: pick φ uniformly in [0, 2π) and draw r from an inverse CDF that
// makes the area density constant. The polar area element dA = r · dr · dφ
// means equal slices of r correspond to rings whose area grows linearly
// with r, so sampling r uniformly would over-sample the origin. Set
// P(R ≤ r) = r² (the disk's area fraction) and invert: r = √U with U
// uniform in [0, 1). This is the 2D analogue of uniformSpherePointsGenerator
// — same trick, different exponent (√ for disk area, ∛ for ball volume).
//
// Preferred over NaiveDiskPointsGenerator for two-dimensional maps:
//
//   - Uniform density. Every equal-area region of the disk is equally
//     likely to contain a point. naiveDisk's density falls off as
//     √(1 − x² − y²), producing a visible dome-shaped clump at the origin
//     and thin coverage near the rim — rarely what a 2D map wants.
//   - Cheaper. Two RNG draws per point vs. naiveDisk's three (one of
//     which is spent on a z coordinate that is immediately discarded).
//   - Honest. The output matches the stated intent directly, without
//     sampling a 3D ball and then squishing the result.
func (pg UniformDiskPointsGenerator) Generate(n int, r *prng.PRNG) []point {
	if n <= 0 {
		return nil
	}
	points := make([]point, 0, n)
	for range n {
		radius := math.Sqrt(r.Float64())
		phi := 2 * math.Pi * r.Float64()
		points = append(points, point{
			x: radius * math.Cos(phi),
			y: radius * math.Sin(phi),
			z: 0,
		})
	}
	return points
}

type UniformSpherePointsGenerator struct{}

// Generate returns n points uniformly distributed in the closed unit ball
// by direct sampling. Each point consumes exactly three RNG draws.
//
// Method: sample a direction uniformly on S², then a radius whose CDF makes
// the volume density constant.
//
//   - Direction: draw cosθ uniformly in [-1, 1] and φ uniformly in [0, 2π).
//     The sinθ factor in the spherical-area element dA = sinθ dθ dφ is
//     exactly the Jacobian of the substitution u = cosθ, so uniform cosθ
//     gives area-uniform coverage of the unit sphere.
//   - Radius: the ball's volume grows as r³, so P(R ≤ r) = r³ and the
//     inverse CDF is r = U^(1/3) with U uniform in [0, 1). Sampling r
//     uniformly would over-sample the origin.
func (pg UniformSpherePointsGenerator) Generate(n int, r *prng.PRNG) []point {
	if n <= 0 {
		return nil
	}
	points := make([]point, 0, n)
	for range n {
		cosTheta := 1 - 2*r.Float64()
		sinTheta := math.Sqrt(1 - cosTheta*cosTheta)
		phi := 2 * math.Pi * r.Float64()
		radius := math.Cbrt(r.Float64())
		points = append(points, point{
			x: radius * sinTheta * math.Cos(phi),
			y: radius * sinTheta * math.Sin(phi),
			z: radius * cosTheta,
		})
	}
	return points
}
