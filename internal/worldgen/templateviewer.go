// Copyright (c) 2026 Michael D Henderson. All rights reserved.

package worldgen

import (
	"bytes"
	"fmt"
	"strings"
)

// ToHTML renders a self-contained HTML page describing a single home-system
// template. Layout mirrors the per-system section of Galaxy.ToHTML so both
// reports feel like the same document family.
func (t *HomeSystemTemplate) ToHTML() []byte {
	var buf bytes.Buffer
	buf.WriteString("<!DOCTYPE html>\n<html lang=\"en\">\n<head>\n")
	fmt.Fprintf(&buf, "<meta charset=\"UTF-8\">\n<title>Home System Template (%d planets)</title>\n", t.NumPlanets)
	buf.WriteString(galaxyPageCSS)
	buf.WriteString("</head>\n<body>\n")

	fmt.Fprintln(&buf, `<section class="report">`)
	fmt.Fprintf(&buf, "<h1>Home System Template — %d planets</h1>\n", t.NumPlanets)
	fmt.Fprintln(&buf, `<table><tbody>`)
	fmt.Fprintf(&buf, "<tr><th>Source Star ID</th><td>%d</td></tr>\n", t.SourceStarID)
	fmt.Fprintf(&buf, "<tr><th>Viability Score</th><td>%d</td></tr>\n", t.ViabilityScore)
	fmt.Fprintln(&buf, `</tbody></table>`)

	fmt.Fprintln(&buf, `<h2>Planets</h2>`)
	fmt.Fprintln(&buf, `<table><thead><tr><th>Orbit</th><th>Diameter (km)</th><th>Gravity (g)</th><th>Temp</th><th>Pressure</th><th>Atmosphere</th><th>Mining</th><th>Home?</th></tr></thead><tbody>`)
	for i, p := range t.Planets {
		home := ""
		if p.Special == 1 {
			home = "yes"
		}
		fmt.Fprintf(&buf,
			"<tr><td>%d</td><td>%d</td><td>%.2f</td><td>%d</td><td>%d</td><td>%s</td><td>%.2f</td><td>%s</td></tr>\n",
			i+1,
			p.Diameter*1000,
			float64(p.Gravity)/100.0,
			p.TemperatureClass,
			p.PressureClass,
			templateAtmosphereLabel(p.Atmosphere),
			float64(p.MiningDifficulty)/100.0,
			home,
		)
	}
	fmt.Fprintln(&buf, `</tbody></table>`)
	fmt.Fprintln(&buf, `</section>`)

	buf.WriteString("</body>\n</html>\n")
	return buf.Bytes()
}

// HomeSystemTemplateUnavailableHTML renders a report explaining that no
// viable template could be produced for the requested planet count. This
// keeps the simulation's output complete even when the galaxy does not
// happen to yield a candidate.
func HomeSystemTemplateUnavailableHTML(numPlanets, candidateCount int) []byte {
	var buf bytes.Buffer
	buf.WriteString("<!DOCTYPE html>\n<html lang=\"en\">\n<head>\n")
	fmt.Fprintf(&buf, "<meta charset=\"UTF-8\">\n<title>Home System Template (%d planets) — unavailable</title>\n", numPlanets)
	buf.WriteString(galaxyPageCSS)
	buf.WriteString("</head>\n<body>\n")

	fmt.Fprintln(&buf, `<section class="report">`)
	fmt.Fprintf(&buf, "<h1>Home System Template — %d planets</h1>\n", numPlanets)
	fmt.Fprintln(&buf, `<p class="empty">No viable template found.</p>`)
	fmt.Fprintf(&buf, "<p>Attempted %d candidate star(s) with exactly %d planet(s); none produced a template in the (53, 57) viability window.</p>\n",
		candidateCount, numPlanets)
	fmt.Fprintln(&buf, `</section>`)

	buf.WriteString("</body>\n</html>\n")
	return buf.Bytes()
}

func templateAtmosphereLabel(atm []TemplateGas) string {
	if len(atm) == 0 {
		return "—"
	}
	parts := make([]string, 0, len(atm))
	for _, e := range atm {
		parts = append(parts, fmt.Sprintf("%s %d%%", e.Gas, e.Percent))
	}
	return strings.Join(parts, ", ")
}
