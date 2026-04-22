// Copyright (c) 2026 Michael D Henderson. All rights reserved.

package worldgen

import (
	"github.com/mdhender/drynn/internal/hexes"
)

type System struct {
	// ID is a stable, sequential identifier assigned during Generate.
	// It exists to give callers (e.g. home-system template generation)
	// a deterministic sort key. It is not persisted to the database.
	ID    int
	Hex   hexes.Axial
	Stars []*Star

	HomeSystem bool
}
