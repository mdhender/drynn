// Copyright (c) 2026 Michael D Henderson. All rights reserved.

package worldgen

// Cluster is the top-level worldgen output: a hex disk of Systems and the
// radius bounding it. Callers slice it up for database insertion.
type Cluster struct {
	Radius  int
	Systems []*System
}
