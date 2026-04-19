package hexmap

import (
	"bytes"
	"fmt"
	"math"
)

type viewerCell struct{ q, r int }

// SystemsToHTML renders systems as a self-contained HTML page with an inline SVG
// hex map. The map uses the systems' axial coordinates directly; no projection or
// snapping is required because the generator already places systems on hexes.
//
// If systems is empty, a minimal empty map is returned.
func SystemsToHTML(systems []System) ([]byte, error) {
	if len(systems) == 0 {
		return renderHexHTML(nil, 1, 1, 0), nil
	}

	// Find the occupied-hex bounding box.
	minQ, maxQ := systems[0].Hex.Q, systems[0].Hex.Q
	minR, maxR := systems[0].Hex.R, systems[0].Hex.R
	for _, s := range systems[1:] {
		if s.Hex.Q < minQ {
			minQ = s.Hex.Q
		}
		if s.Hex.Q > maxQ {
			maxQ = s.Hex.Q
		}
		if s.Hex.R < minR {
			minR = s.Hex.R
		}
		if s.Hex.R > maxR {
			maxR = s.Hex.R
		}
	}

	// Shift the occupied hexes so the minimum occupied coordinate is at (1,1),
	// leaving a one-cell border around the rendered content.
	cells := make(map[viewerCell]int, len(systems))
	for _, s := range systems {
		cells[viewerCell{s.Hex.Q - minQ + 1, s.Hex.R - minR + 1}] = s.Stars
	}

	numCols := maxQ - minQ + 3 // +2 for one-cell border, +1 for inclusive range
	numRows := maxR - minR + 3

	return renderHexHTML(cells, numCols, numRows, 0), nil
}

// SystemsToHTMLWithPixelSize is like SystemsToHTML but allows the caller to
// override the pixel hex size used in the SVG. If pixSize <= 0, a suitable size
// is derived automatically to fit within roughly 1280x1280.
func SystemsToHTMLWithPixelSize(systems []System, pixSize float64) ([]byte, error) {
	if len(systems) == 0 {
		return renderHexHTML(nil, 1, 1, pixSize), nil
	}

	minQ, maxQ := systems[0].Hex.Q, systems[0].Hex.Q
	minR, maxR := systems[0].Hex.R, systems[0].Hex.R
	for _, s := range systems[1:] {
		if s.Hex.Q < minQ {
			minQ = s.Hex.Q
		}
		if s.Hex.Q > maxQ {
			maxQ = s.Hex.Q
		}
		if s.Hex.R < minR {
			minR = s.Hex.R
		}
		if s.Hex.R > maxR {
			maxR = s.Hex.R
		}
	}

	cells := make(map[viewerCell]int, len(systems))
	for _, s := range systems {
		cells[viewerCell{s.Hex.Q - minQ + 1, s.Hex.R - minR + 1}] = s.Stars
	}

	numCols := maxQ - minQ + 3
	numRows := maxR - minR + 3

	return renderHexHTML(cells, numCols, numRows, pixSize), nil
}

// RenderDiskHTML renders the full hex disk of the given radius and overlays any
// systems that fall within that disk. This is useful when you want to see the
// official galaxy boundary, not just the occupied bounding box.
func RenderDiskHTML(radius int, systems []System) ([]byte, error) {
	if radius < 0 {
		return nil, fmt.Errorf("radius must be non-negative")
	}

	return renderDiskHTML(radius, systems, 0), nil
}

// axialToPixel converts axial (q, r) to flat-top hex pixel center.
func axialToPixel(q, r int, size float64) (float64, float64) {
	sqrt3 := math.Sqrt(3)
	cx := size * 1.5 * float64(q)
	cy := size * sqrt3 * (float64(r) + 0.5*float64(q))
	return cx, cy
}

// renderDiskHTML produces a self-contained HTML page with an inline SVG hex
// grid shaped as a hexagonal disk. It iterates the actual Disk() hexes in
// axial coordinates, so the outline is always a proper hexagon.
func renderDiskHTML(radius int, systems []System, pixSize float64) []byte {
	const (
		maxDim = 1280.0
		margin = 40.0
	)
	sqrt3 := math.Sqrt(3)

	hexes := Disk(radius)

	// Build a lookup of system star counts keyed by axial coords.
	starMap := make(map[Axial]int, len(systems))
	for _, s := range systems {
		if Contains(radius, s.Hex) {
			starMap[s.Hex] = s.Stars
		}
	}

	// Auto-size: derive pixSize to fit within maxDim.
	if pixSize <= 0 {
		// The flat-top axial layout spans roughly:
		//   width  ≈ size * (3*radius + 1)       (from q = -radius to +radius)
		//   height ≈ size * sqrt3 * (2*radius + 1)
		avail := maxDim - 2*margin
		sizeFromW := avail / (3.0*float64(radius) + 1.0)
		sizeFromH := avail / (sqrt3 * (2.0*float64(radius) + 1.0))
		pixSize = math.Min(sizeFromW, sizeFromH)
		if pixSize < 4 {
			pixSize = 4
		}
	}

	// Compute pixel centers for every hex and track the bounding box.
	type hexPix struct {
		ax     Axial
		cx, cy float64
	}
	pixels := make([]hexPix, len(hexes))
	minX, minY := math.Inf(1), math.Inf(1)
	maxX, maxY := math.Inf(-1), math.Inf(-1)
	for i, h := range hexes {
		cx, cy := axialToPixel(h.Q, h.R, pixSize)
		pixels[i] = hexPix{ax: h, cx: cx, cy: cy}
		if cx < minX {
			minX = cx
		}
		if cx > maxX {
			maxX = cx
		}
		if cy < minY {
			minY = cy
		}
		if cy > maxY {
			maxY = cy
		}
	}

	// viewBox with margin around the hex centers (account for hex radius).
	vbX := minX - pixSize - margin
	vbY := minY - pixSize*sqrt3/2 - margin
	vbW := (maxX - minX) + 2*pixSize + 2*margin
	vbH := (maxY - minY) + pixSize*sqrt3 + 2*margin
	svgW := int(math.Ceil(vbW))
	svgH := int(math.Ceil(vbH))

	var buf bytes.Buffer
	fmt.Fprintf(&buf, "<!DOCTYPE html>\n<html lang=\"en\">\n<head>\n")
	fmt.Fprintf(&buf, "<meta charset=\"UTF-8\">\n<title>Hex Map</title>\n")
	fmt.Fprintf(&buf, "<style>body{margin:0;background:#f6f6f7;font-family:system-ui,sans-serif}.wrap{display:flex;justify-content:center;padding:24px}</style>\n")
	fmt.Fprintf(&buf, "</head>\n<body>\n<div class=\"wrap\">\n")

	fmt.Fprintf(&buf, `<svg xmlns="http://www.w3.org/2000/svg" width="%d" height="%d" viewBox="%.1f %.1f %.1f %.1f">`+"\n",
		svgW, svgH, vbX, vbY, vbW, vbH)
	fmt.Fprintf(&buf, `  <rect x="%.1f" y="%.1f" width="%.1f" height="%.1f" fill="white"/>`+"\n", vbX, vbY, vbW, vbH)

	// Draw hex outlines.
	for _, hp := range pixels {
		buf.WriteString(`  <polygon points="`)
		for i := 0; i < 6; i++ {
			angle := math.Pi / 3.0 * float64(i)
			vx := hp.cx + pixSize*math.Cos(angle)
			vy := hp.cy + pixSize*math.Sin(angle)
			if i > 0 {
				buf.WriteByte(' ')
			}
			fmt.Fprintf(&buf, "%.1f,%.1f", vx, vy)
		}
		buf.WriteString(`" fill="none" stroke="#cccccc" stroke-width="1"/>` + "\n")
	}

	// Overlay star systems as die-face dot patterns.
	for _, hp := range pixels {
		if count := starMap[hp.ax]; count > 0 {
			renderStarDots(&buf, hp.cx, hp.cy, pixSize, count)
		}
	}

	fmt.Fprintln(&buf, `</svg>`)
	fmt.Fprintf(&buf, "</div>\n</body>\n</html>\n")
	return buf.Bytes()
}

// renderHexHTML produces a self-contained HTML page with an inline SVG
// rectangular hex grid. Used by SystemsToHTML / SystemsToHTMLWithPixelSize.
func renderHexHTML(cells map[viewerCell]int, numCols, numRows int, pixSize float64) []byte {
	const (
		maxDim = 1280.0
		margin = 40.0
	)
	sqrt3 := math.Sqrt(3)

	if numCols <= 0 || numRows <= 0 {
		numCols, numRows = 1, 1
	}

	if pixSize <= 0 {
		avail := maxDim - 2*margin
		sizeFromW := avail / (1.5*float64(numCols) + 0.5)
		sizeFromH := avail / (sqrt3 * (float64(numRows) + 0.5))
		pixSize = math.Min(sizeFromW, sizeFromH)
		if pixSize < 4 {
			pixSize = 4
		}
	}

	svgW := math.Ceil(pixSize*(1.5*float64(numCols)+0.5) + 2*margin)
	svgH := math.Ceil(pixSize*sqrt3*(float64(numRows)+0.5) + 2*margin)
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

	for col := 0; col < numCols; col++ {
		for row := 0; row < numRows; row++ {
			cx := margin + pixSize*(1.5*float64(col)+1)
			cy := margin + pixSize*sqrt3*(float64(row)+0.5*float64(col&1)+0.5)

			buf.WriteString(`  <polygon points="`)
			for i := 0; i < 6; i++ {
				angle := math.Pi / 3.0 * float64(i)
				vx := cx + pixSize*math.Cos(angle)
				vy := cy + pixSize*math.Sin(angle)
				if i > 0 {
					buf.WriteByte(' ')
				}
				fmt.Fprintf(&buf, "%.1f,%.1f", vx, vy)
			}
			buf.WriteString(`" fill="none" stroke="#cccccc" stroke-width="1"/>` + "\n")

			if count := cells[viewerCell{col, row}]; count > 0 {
				renderStarDots(&buf, cx, cy, pixSize, count)
			}
		}
	}

	fmt.Fprintln(&buf, `</svg>`)
	fmt.Fprintf(&buf, "</div>\n</body>\n</html>\n")
	return buf.Bytes()
}

// renderStarDots writes SVG circles in a die-face pattern for 1–5 stars.
func renderStarDots(buf *bytes.Buffer, cx, cy, size float64, count int) {
	r := size * 0.10
	d := size * 0.28

	type pos struct{ x, y float64 }
	var dots []pos
	switch count {
	case 1:
		dots = []pos{{0, 0}}
	case 2:
		dots = []pos{{-d, -d}, {d, d}}
	case 3:
		dots = []pos{{-d, -d}, {0, 0}, {d, d}}
	case 4:
		dots = []pos{{-d, -d}, {d, -d}, {-d, d}, {d, d}}
	default: // 5
		dots = []pos{{-d, -d}, {d, -d}, {0, 0}, {-d, d}, {d, d}}
	}

	for _, p := range dots {
		fmt.Fprintf(buf, `  <circle cx="%.1f" cy="%.1f" r="%.1f" fill="#222222"/>`+"\n",
			cx+p.x, cy+p.y, r)
	}
}
