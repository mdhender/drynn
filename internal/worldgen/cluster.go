// Copyright (c) 2026 Michael D Henderson. All rights reserved.

package worldgen

// Cluster is the top-level worldgen output: a hex disk of Systems plus
// the stage-1 home-star-template library. Callers slice it up for
// database insertion.
type Cluster struct {
	Radius  int
	Systems []*System

	// HomeStarTemplates is the stage-1 output, indexed by planet count.
	// Length is always 10; indexes 0..2 are always nil and 3..9 are
	// always non-nil. A nil Template inside an outcome means the stage-1
	// driver exhausted its candidate budget without finding a viable
	// template for that planet count. See
	// reference/home-system-templates.md.
	HomeStarTemplates []*HomeStarTemplateOutcome
}
