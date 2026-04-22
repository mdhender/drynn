// Copyright (c) 2026 Michael D Henderson. All rights reserved.

package worldgen

type Star struct {
	// ID is a stable, sequential identifier assigned during Generate.
	// It exists to give callers (e.g. home-system template generation)
	// a deterministic sort key. It is not persisted to the database.
	ID    int
	Kind  StarType
	Color StarColor
	Size  int // 0..9

	NumPlanets int // 1..9
	Planets    []*Planet
}

type StarType int

const (
	StarDwarf        StarType = 1
	StarDegenerate   StarType = 2
	StarMainSequence StarType = 3
	StarGiant        StarType = 4
)

func (t StarType) String() string {
	switch t {
	case StarDwarf:
		return "dwarf"
	case StarDegenerate:
		return "degenerate"
	case StarMainSequence:
		return "main-sequence"
	case StarGiant:
		return "giant"
	default:
		return "unknown"
	}
}

type StarColor int

const (
	ColorBlue        StarColor = 1
	ColorBlueWhite   StarColor = 2
	ColorWhite       StarColor = 3
	ColorYellowWhite StarColor = 4
	ColorYellow      StarColor = 5
	ColorOrange      StarColor = 6
	ColorRed         StarColor = 7
)

func (c StarColor) String() string {
	switch c {
	case ColorBlue:
		return "blue"
	case ColorBlueWhite:
		return "blue-white"
	case ColorWhite:
		return "white"
	case ColorYellowWhite:
		return "yellow-white"
	case ColorYellow:
		return "yellow"
	case ColorOrange:
		return "orange"
	case ColorRed:
		return "red"
	default:
		return "unknown"
	}
}
