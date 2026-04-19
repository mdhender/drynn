// Copyright (c) 2026 Michael D Henderson. All rights reserved.

package worldgen

import (
	hexmap "github.com/mdhender/drynn/internal/worldgen/hexes"
)

type System struct {
	Hex   hexmap.Axial
	Stars []*Star

	homeSystem bool
}
