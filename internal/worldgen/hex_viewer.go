// Copyright (c) 2026 Michael D Henderson. All rights reserved.

package worldgen

import (
	"bytes"
	"fmt"
	"math"
)

// GalaxyToHexHTML renders the galaxy as a flat-top, odd-q hexagonal grid map.
// Stars are projected onto the XY plane (Z is ignored) and snapped to the
// nearest hex center. The star coordinates are used directly (no un-scaling).
//
// cellSize is the hex size (center-to-vertex distance) in world-coordinate
// units; it controls the snapping granularity. If cellSize is 0, it is
// derived so the resulting grid fits within 1280×1280 pixels.
func GalaxyToHexHTML(g *Galaxy, cellSize float64) ([]byte, error) {
	if len(g.Stars) == 0 {
		return renderHexHTML(nil, 0, 0, 0, 0, 0), nil
	}

	// Collect star XY positions (Z ignored).
	type starXY struct{ x, y float64 }
	stars := make([]starXY, len(g.Stars))
	for i, s := range g.Stars {
		stars[i] = starXY{float64(s.X), float64(s.Y)}
	}

	// World bounding box.
	minX, maxX := stars[0].x, stars[0].x
	minY, maxY := stars[0].y, stars[0].y
	for _, s := range stars[1:] {
		if s.x < minX {
			minX = s.x
		}
		if s.x > maxX {
			maxX = s.x
		}
		if s.y < minY {
			minY = s.y
		}
		if s.y > maxY {
			maxY = s.y
		}
	}
	worldW := maxX - minX
	worldH := maxY - minY
	if worldW == 0 {
		worldW = 1
	}
	if worldH == 0 {
		worldH = 1
	}

	// Auto-derive cell size: aim for ~40 hexes across the longest axis.
	if cellSize <= 0 {
		cellSize = math.Max(worldW, worldH) / 40.0
		if cellSize <= 0 {
			cellSize = 1
		}
	}

	// Snap each star to its nearest hex cell (odd-q flat-top).
	type hexCoord struct{ q, r int }
	hexCounts := make(map[hexCoord]int)
	for _, s := range stars {
		h := pixelToOddQ(s.x, s.y, cellSize)
		hexCounts[hexCoord{h[0], h[1]}]++
	}

	// Hex bounding box.
	first := true
	var minQ, maxQ, minR, maxR int
	for h := range hexCounts {
		if first {
			minQ, maxQ = h.q, h.q
			minR, maxR = h.r, h.r
			first = false
			continue
		}
		if h.q < minQ {
			minQ = h.q
		}
		if h.q > maxQ {
			maxQ = h.q
		}
		if h.r < minR {
			minR = h.r
		}
		if h.r > maxR {
			maxR = h.r
		}
	}

	// Shift so the minimum hex is at (0, 0) and add a 1-cell border.
	type hc = struct{ q, r int }
	shifted := make(map[hc]int, len(hexCounts))
	for h, n := range hexCounts {
		shifted[hc{h.q - minQ + 1, h.r - minR + 1}] = n
	}
	numCols := maxQ - minQ + 3 // +2 for 1-cell border on each side
	numRows := maxR - minR + 3

	return renderHexHTML(shifted, numCols, numRows, 0, 0, 0), nil
}

// renderHexHTML produces a self-contained HTML page with an inline SVG
// hex grid. pixSize overrides the pixel hex size; if 0 it is derived to
// fit within 1280×1280.
func renderHexHTML(cells map[struct{ q, r int }]int, numCols, numRows int, pixSize, svgW, svgH float64) []byte {
	const (
		maxDim = 1280.0
		margin = 40.0
	)
	sqrt3 := math.Sqrt(3)

	if numCols <= 0 || numRows <= 0 {
		numCols, numRows = 1, 1
	}

	// Derive pixel hex size to fit within the target dimensions.
	if pixSize <= 0 {
		avail := maxDim - 2*margin
		sizeFromW := avail / (1.5*float64(numCols) + 0.5)
		sizeFromH := avail / (sqrt3 * (float64(numRows) + 0.5))
		pixSize = math.Min(sizeFromW, sizeFromH)
		if pixSize < 4 {
			pixSize = 4
		}
	}

	// Compute SVG canvas size.
	if svgW <= 0 {
		svgW = math.Ceil(pixSize*(1.5*float64(numCols)+0.5) + 2*margin)
	}
	if svgH <= 0 {
		svgH = math.Ceil(pixSize*sqrt3*(float64(numRows)+0.5) + 2*margin)
	}
	w := int(svgW)
	h := int(svgH)

	var buf bytes.Buffer
	fmt.Fprintf(&buf, "<!DOCTYPE html>\n<html lang=\"en\">\n<head>\n")
	fmt.Fprintf(&buf, "<meta charset=\"UTF-8\">\n<title>Hex Map</title>\n")
	fmt.Fprintf(&buf, "<style>body{margin:0;background:#f6f6f7;font-family:system-ui,sans-serif}.wrap{display:flex;justify-content:center;padding:24px}</style>\n")
	fmt.Fprintf(&buf, "</head>\n<body>\n<div class=\"wrap\">\n")

	fmt.Fprintf(&buf, `<svg xmlns="http://www.w3.org/2000/svg" width="%d" height="%d" viewBox="0 0 %d %d">`+"\n",
		w, h, w, h)
	fmt.Fprintf(&buf, `  <rect x="0" y="0" width="%d" height="%d" fill="white"/>`+"\n", w, h)

	type hexCoord = struct{ q, r int }

	// Draw every hex in the bounding rectangle.
	for col := 0; col < numCols; col++ {
		for row := 0; row < numRows; row++ {
			cx := margin + pixSize*(1.5*float64(col)+1)
			cy := margin + pixSize*sqrt3*(float64(row)+0.5*float64(col&1)+0.5)

			// Hex polygon (flat-top, 6 vertices).
			buf.WriteString(`  <polygon points="`)
			for i := range 6 {
				angle := math.Pi / 3.0 * float64(i)
				vx := cx + pixSize*math.Cos(angle)
				vy := cy + pixSize*math.Sin(angle)
				if i > 0 {
					buf.WriteByte(' ')
				}
				fmt.Fprintf(&buf, "%.1f,%.1f", vx, vy)
			}
			buf.WriteString(`" fill="none" stroke="#cccccc" stroke-width="1"/>` + "\n")

			// Star content.
			count := cells[hexCoord{col, row}]
			if count == 1 {
				fmt.Fprintf(&buf, `  <circle cx="%.1f" cy="%.1f" r="%.1f" fill="#222222"/>`+"\n",
					cx, cy, pixSize*0.27)
			} else if count > 1 {
				fmt.Fprintf(&buf, `  <circle cx="%.1f" cy="%.1f" r="%.1f" fill="#222222"/>`+"\n",
					cx, cy, pixSize*0.27)
				fmt.Fprintf(&buf, `  <text x="%.1f" y="%.1f" font-family="system-ui, sans-serif" font-size="%.0f" fill="#888888" text-anchor="middle">+%d</text>`+"\n",
					cx, cy+pixSize*0.48, pixSize*0.38, count-1)
			}
		}
	}

	fmt.Fprintln(&buf, `</svg>`)
	fmt.Fprintf(&buf, "</div>\n</body>\n</html>\n")
	return buf.Bytes()
}

// pixelToOddQ converts world coordinates (x, y) to odd-q offset hex
// coordinates for a flat-top hex grid with the given size (center-to-vertex).
//
// Algorithm: pixel → fractional axial → cube round → axial → odd-q offset.
// Reference: https://www.redblobgames.com/grids/hexagons/
func pixelToOddQ(x, y, size float64) [2]int {
	// Pixel to fractional axial (flat-top).
	q := (2.0 / 3.0 * x) / size
	r := (-1.0/3.0*x + math.Sqrt(3)/3.0*y) / size

	// Axial to cube.
	cx := q
	cz := r
	cy := -cx - cz

	// Cube round.
	rx := math.Round(cx)
	ry := math.Round(cy)
	rz := math.Round(cz)

	dx := math.Abs(rx - cx)
	dy := math.Abs(ry - cy)
	dz := math.Abs(rz - cz)

	if dx > dy && dx > dz {
		rx = -ry - rz
	} else if dy > dz {
		ry = -rx - rz
	} else {
		rz = -rx - ry
	}

	// Cube to axial.
	aq := int(rx)
	ar := int(rz)

	// Axial to odd-q offset.
	col := aq
	row := ar + (aq-(aq&1))/2

	return [2]int{col, row}
}


