// Copyright (c) 2026 Michael D Henderson. All rights reserved.

package worldgen

import (
	"github.com/mdhender/drynn/internal/hexes"
)

type System struct {
	Hex   hexes.Axial
	Stars []*Star

	HomeSystem bool
}
