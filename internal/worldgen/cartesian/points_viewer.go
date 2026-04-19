// Copyright (c) 2026 Michael D Henderson. All rights reserved.

package cartesian

import (
	"bytes"
	"cmp"
	"fmt"
	"html"
	"math"
	"os"
	"path/filepath"
	"slices"

	"github.com/mdhender/drynn/internal/prng"
)

func GalaxyToHTML(g *Galaxy, generatorName string) ([]byte, error) {
	set := make([]point, 0, len(g.Stars))
	for _, star := range g.Stars {
		set = append(set, point{
			x: float64(star.X) / g.Radius,
			y: float64(star.Y) / g.Radius,
			z: float64(star.Z) / g.Radius,
		})
	}
	return renderPointsHTML(set, generatorName, true), nil
}

func TestPointsGenerator(n int, g string, p string) error {
	var pg PointGenerator
	switch g {
	case "naiveDisk":
		pg = &NaiveDiskPointsGenerator{}
	case "naiveSphere":
		pg = &NaiveSpherePointsGenerator{}
	case "uniformDisk":
		pg = &UniformDiskPointsGenerator{}
	case "uniformSphere":
		pg = &UniformSpherePointsGenerator{}
	default:
		panic("unknown generator " + g)
	}
	r := prng.NewFromSeed(10, 10)
	set := pg.Generate(n, r)
	err := os.WriteFile(filepath.Join(p, g+".html"), renderPointsHTML(set, filepath.Join(p, g), true), 0o644)
	return err
}

// renderPointsHTML returns a self-contained HTML document embedding an SVG
// visualization of points. The main view is an oblique projection onto a
// tilted base plane with a vertical drop line from the plane to each point;
// a smaller top-down XY inset sits in the upper-right corner. If debug is
// true, a strip of XY/XZ/YZ panels is appended along the bottom for
// diagnosing flattening, banding, or clustering artifacts in generators.
//
// Points are assumed to lie in the closed unit ball (every coordinate in
// [-1, 1]). Coordinates outside that range are projected without clamping
// and may escape the plotting area.
func renderPointsHTML(points []point, title string, debug bool) []byte {
	const (
		width  = 1100
		height = 760
	)

	var buf bytes.Buffer
	escaped := html.EscapeString(title)

	fmt.Fprintf(&buf, "<!DOCTYPE html>\n<html lang=\"en\">\n<head>\n")
	fmt.Fprintf(&buf, "<meta charset=\"UTF-8\">\n<title>%s</title>\n", escaped)
	fmt.Fprintf(&buf, "<style>body{margin:0;background:#f6f6f7;font-family:system-ui,sans-serif}.wrap{display:flex;justify-content:center;padding:24px}</style>\n")
	fmt.Fprintf(&buf, "</head>\n<body>\n<div class=\"wrap\">\n")

	writePointsSVG(&buf, points, escaped, width, height, debug)

	fmt.Fprintf(&buf, "</div>\n</body>\n</html>\n")
	return buf.Bytes()
}

func writePointsSVG(buf *bytes.Buffer, points []point, escapedTitle string, width, height int, debug bool) {
	const (
		margin  = 24.0
		headerH = 28.0
		insetW  = 180.0
		insetH  = 180.0
		debugH  = 180.0
	)

	pageW := float64(width)
	pageH := float64(height)

	contentTop := margin + headerH
	contentBottom := pageH - margin

	mainX := margin
	mainY := contentTop
	mainW := pageW - 2*margin
	mainH := contentBottom - contentTop
	if debug {
		mainH -= debugH + 16
	}

	insetX := mainX + mainW - insetW
	insetY := mainY

	// Shrink the main plotting width if the inset would crowd it.
	mainPlotW := mainW
	if insetX-12-mainX > 260 {
		mainPlotW = insetX - 12 - mainX
	}

	// Oblique-ish projection:
	//   base = (cx + (x-y)*sx, cy + (x+y)*sy)
	//   screen = base - (0, z*sz)
	cx := mainX + mainPlotW*0.52
	cy := mainY + mainH*0.62
	sx := mainPlotW * 0.23
	sy := mainH * 0.12
	sz := mainH * 0.22

	projectMain := func(p point) (bx, by, px, py float64) {
		bx = cx + (p.x-p.y)*sx
		by = cy + (p.x+p.y)*sy
		return bx, by, bx, by - p.z*sz
	}

	type projected struct {
		bx, by, px, py  float64
		radius, opacity float64
		sortKey         float64
	}
	proj := make([]projected, len(points))
	for i, p := range points {
		bx, by, px, py := projectMain(p)
		t := clamp01((p.z + 1.0) / 2.0)
		proj[i] = projected{
			bx: bx, by: by, px: px, py: py,
			radius:  3.0 + 1.8*t,
			opacity: 0.62 + 0.33*t,
			sortKey: py + 0.15*px,
		}
	}
	slices.SortFunc(proj, func(a, b projected) int {
		return cmp.Compare(a.sortKey, b.sortKey)
	})

	fmt.Fprintf(buf, `<svg xmlns="http://www.w3.org/2000/svg" width="%d" height="%d" viewBox="0 0 %d %d">`+"\n",
		width, height, width, height)
	fmt.Fprintf(buf, `  <rect x="0" y="0" width="%d" height="%d" fill="white"/>`+"\n", width, height)
	fmt.Fprintf(buf, `  <text x="24" y="22" font-family="system-ui, sans-serif" font-size="15" fill="#222">%s</text>`+"\n", escapedTitle)

	const circleSegments = 72
	fmt.Fprint(buf, `  <polygon points="`)
	for i := range circleSegments {
		theta := 2 * math.Pi * float64(i) / circleSegments
		bx, by, _, _ := projectMain(point{x: math.Cos(theta), y: math.Sin(theta)})
		if i > 0 {
			buf.WriteByte(' ')
		}
		fmt.Fprintf(buf, "%.1f,%.1f", bx, by)
	}
	fmt.Fprint(buf, `" fill="none" stroke="#cfcfcf" stroke-width="1.2"/>`+"\n")

	axes := [][2]point{
		{{x: -1}, {x: +1}},
		{{y: -1}, {y: +1}},
	}
	for _, axis := range axes {
		x1, y1, _, _ := projectMain(axis[0])
		x2, y2, _, _ := projectMain(axis[1])
		fmt.Fprintf(buf, `  <line x1="%.1f" y1="%.1f" x2="%.1f" y2="%.1f" stroke="#dddddd" stroke-width="1"/>`+"\n",
			x1, y1, x2, y2)
	}

	for _, pp := range proj {
		fmt.Fprintf(buf, `  <line x1="%.1f" y1="%.1f" x2="%.1f" y2="%.1f" stroke="#bbbbbb" stroke-width="0.8" opacity="0.55"/>`+"\n",
			pp.bx, pp.by, pp.px, pp.py)
	}
	for _, pp := range proj {
		fmt.Fprintf(buf, `  <circle cx="%.1f" cy="%.1f" r="%.2f" fill="#222222" opacity="%.2f"/>`+"\n",
			pp.px, pp.py, pp.radius, pp.opacity)
	}

	writeProjectionPanel(buf, points, "XY inset", insetX, insetY, insetW, insetH,
		func(p point) (u, v, t float64) { return p.x, p.y, p.z })

	if debug {
		debugY := mainY + mainH + 16
		debugW := pageW - 2*margin
		fmt.Fprintf(buf, `  <text x="%.1f" y="%.1f" font-family="system-ui, sans-serif" font-size="13" fill="#333">Debug projections</text>`+"\n",
			margin, debugY-4)
		gap := 14.0
		panelW := (debugW - 2*gap) / 3.0
		writeProjectionPanel(buf, points, "XY", margin, debugY+10, panelW, debugH-10,
			func(p point) (u, v, t float64) { return p.x, p.y, p.z })
		writeProjectionPanel(buf, points, "XZ", margin+panelW+gap, debugY+10, panelW, debugH-10,
			func(p point) (u, v, t float64) { return p.x, p.z, p.y })
		writeProjectionPanel(buf, points, "YZ", margin+2*(panelW+gap), debugY+10, panelW, debugH-10,
			func(p point) (u, v, t float64) { return p.y, p.z, p.x })
	}

	fmt.Fprintln(buf, `</svg>`)
}

func writeProjectionPanel(buf *bytes.Buffer, points []point, label string, x, y, w, h float64, proj func(point) (u, v, t float64)) {
	if w <= 0 || h <= 0 {
		return
	}
	fmt.Fprintf(buf, `  <rect x="%.1f" y="%.1f" width="%.1f" height="%.1f" fill="white" stroke="#cccccc" stroke-width="1"/>`+"\n",
		x, y, w, h)
	fmt.Fprintf(buf, `  <text x="%.1f" y="%.1f" font-family="system-ui, sans-serif" font-size="12" fill="#444">%s</text>`+"\n",
		x+8, y+16, html.EscapeString(label))

	plotPad := 18.0
	plotX := x + plotPad
	plotY := y + plotPad + 8
	plotW := w - 2*plotPad
	plotH := h - 2*plotPad - 8
	if plotW <= 0 || plotH <= 0 {
		return
	}

	fmt.Fprintf(buf, `  <rect x="%.1f" y="%.1f" width="%.1f" height="%.1f" fill="none" stroke="#e0e0e0" stroke-width="1"/>`+"\n",
		plotX, plotY, plotW, plotH)

	midX := plotX + plotW/2
	midY := plotY + plotH/2
	fmt.Fprintf(buf, `  <line x1="%.1f" y1="%.1f" x2="%.1f" y2="%.1f" stroke="#eeeeee" stroke-width="1"/>`+"\n",
		midX, plotY, midX, plotY+plotH)
	fmt.Fprintf(buf, `  <line x1="%.1f" y1="%.1f" x2="%.1f" y2="%.1f" stroke="#eeeeee" stroke-width="1"/>`+"\n",
		plotX, midY, plotX+plotW, midY)

	type item struct {
		x, y, r, opacity, order float64
	}
	items := make([]item, len(points))
	for i, p := range points {
		u, v, t := proj(p)
		ix := plotX + (u+1.0)/2.0*plotW
		iy := plotY + plotH - (v+1.0)/2.0*plotH
		k := clamp01((t + 1.0) / 2.0)
		items[i] = item{
			x:       ix,
			y:       iy,
			r:       2.7 + 0.9*k,
			opacity: 0.62 + 0.33*k,
			order:   iy + 0.10*ix,
		}
	}
	slices.SortFunc(items, func(a, b item) int {
		return cmp.Compare(a.order, b.order)
	})
	for _, it := range items {
		fmt.Fprintf(buf, `  <circle cx="%.1f" cy="%.1f" r="%.2f" fill="#333333" opacity="%.2f"/>`+"\n",
			it.x, it.y, it.r, it.opacity)
	}
}

func clamp01(t float64) float64 {
	if t < 0 {
		return 0
	}
	if t > 1 {
		return 1
	}
	return t
}
