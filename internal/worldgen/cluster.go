// Copyright (c) 2026 Michael D Henderson. All rights reserved.

package worldgen

// Cluster is the top-level worldgen output. It stores the hex-disk
// systems, stars, and planets as three parallel flat slices linked by
// integer IDs (Star.SystemID, Planet.StarID), plus the stage-1 home-
// star-template library. Callers can walk the slices or fetch a
// parent-scoped subset via the StarsForSystem / PlanetsForStar /
// PlanetsForSystem helpers.
//
// Flat layout matches the eventual relational schema one-to-one and
// makes staged generators (templates, deposits) trivially composable.
// See internal/worldgen/staged-generator-plan.md.
type Cluster struct {
	Radius  int
	Systems []*System
	Stars   []*Star
	Planets []*Planet

	// HomeStarTemplates is the stage-1 output, indexed by planet count.
	// Length is always 10; indexes 0..2 are always nil and 3..9 are
	// always non-nil. A nil Template inside an outcome means the stage-1
	// driver exhausted its candidate budget without finding a viable
	// template for that planet count. See
	// reference/home-system-templates.md.
	HomeStarTemplates []*HomeStarTemplateOutcome
}

// StarsForSystem returns the stars owned by the given system, preserving
// the cluster's generation order. Linear scan; fine at worldgen scale.
func (c *Cluster) StarsForSystem(sysID int) []*Star {
	var out []*Star
	for _, s := range c.Stars {
		if s.SystemID == sysID {
			out = append(out, s)
		}
	}
	return out
}

// PlanetsForStar returns the planets orbiting the given star in orbit
// order (innermost first), per the cluster's generation ordering.
func (c *Cluster) PlanetsForStar(starID int) []*Planet {
	var out []*Planet
	for _, p := range c.Planets {
		if p.StarID == starID {
			out = append(out, p)
		}
	}
	return out
}

// PlanetsForSystem returns every planet under the given system,
// regardless of which star they orbit.
func (c *Cluster) PlanetsForSystem(sysID int) []*Planet {
	starIDs := make(map[int]bool)
	for _, s := range c.Stars {
		if s.SystemID == sysID {
			starIDs[s.ID] = true
		}
	}
	var out []*Planet
	for _, p := range c.Planets {
		if starIDs[p.StarID] {
			out = append(out, p)
		}
	}
	return out
}

// TotalStars returns the number of stars in the cluster.
func (c *Cluster) TotalStars() int { return len(c.Stars) }

// TotalPlanets returns the number of planets in the cluster.
func (c *Cluster) TotalPlanets() int { return len(c.Planets) }

// CountMultiStarSystems returns the number of systems containing more
// than one star.
func (c *Cluster) CountMultiStarSystems() int {
	counts := make(map[int]int, len(c.Systems))
	for _, s := range c.Stars {
		counts[s.SystemID]++
	}
	n := 0
	for _, v := range counts {
		if v > 1 {
			n++
		}
	}
	return n
}
