// Copyright (c) 2026 Michael D Henderson. All rights reserved.

package worldgen

type Star struct {
	kind  starType
	color starColor
	size  int // 0..9

	numPlanets int // 1..9
	Planets    []*Planet
}

type starType int

const (
	starDwarf        starType = 1
	starDegenerate   starType = 2
	starMainSequence starType = 3
	starGiant        starType = 4
)

type starColor int

const (
	colorBlue        starColor = 1
	colorBlueWhite   starColor = 2
	colorWhite       starColor = 3
	colorYellowWhite starColor = 4
	colorYellow      starColor = 5
	colorOrange      starColor = 6
	colorRed         starColor = 7
)
