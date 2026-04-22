// Copyright (c) 2026 Michael D Henderson. All rights reserved.

package worldgen

import (
	"github.com/mdhender/drynn/internal/hexes"
)

type System struct {
	// ID is a stable, sequential identifier assigned during Generate.
	// Stars that belong to this system reference it via Star.SystemID.
	ID         int
	Hex        hexes.Axial
	HomeSystem bool
}
