// Copyright (c) 2026 Michael D Henderson. All rights reserved.

package worldgen

import (
	"bytes"
	"fmt"
	"sort"
	"strings"

	hexmap "github.com/mdhender/drynn/internal/worldgen/hexes"
)

// ToHTML renders a self-contained HTML page showing the galaxy's hex map.
// If showPlanets is true, a per-system planet report is appended below the
// map. If pixelSize <= 0, a size is derived to fit within roughly 1280x1280.
func (g *Galaxy) ToHTML(pixelSize float64, showCoords, showPlanets bool) []byte {
	systems := make([]hexmap.System, 0, len(g.Systems))
	for _, s := range g.Systems {
		systems = append(systems, hexmap.System{Hex: s.Hex, Stars: len(s.Stars)})
	}

	var buf bytes.Buffer
	buf.WriteString("<!DOCTYPE html>\n<html lang=\"en\">\n<head>\n")
	buf.WriteString("<meta charset=\"UTF-8\">\n<title>Galaxy</title>\n")
	buf.WriteString(galaxyPageCSS)
	buf.WriteString("</head>\n<body>\n")

	buf.WriteString("<div class=\"map\">\n")
	buf.Write(hexmap.RenderDiskSVG(g.Radius, systems, pixelSize, showCoords))
	buf.WriteString("</div>\n")

	if showPlanets {
		writePlanetReport(&buf, g)
	}

	buf.WriteString("</body>\n</html>\n")
	return buf.Bytes()
}

const galaxyPageCSS = `<style>
body{margin:0;background:#f6f6f7;font-family:system-ui,sans-serif;color:#222}
.map{display:flex;justify-content:center;padding:24px}
.report{max-width:960px;margin:0 auto;padding:0 24px 48px}
.report h1{font-size:22px;margin:0 0 16px}
.report h2{font-size:17px;margin:24px 0 6px;border-bottom:1px solid #ddd;padding-bottom:2px}
.report h3{font-size:14px;margin:10px 0 6px;color:#444}
.report table{width:100%;border-collapse:collapse;font-size:13px;margin:4px 0 12px}
.report th,.report td{padding:4px 8px;border-bottom:1px solid #eee;text-align:right}
.report th:first-child,.report td:first-child{text-align:left}
.report .empty{color:#888;font-style:italic;margin:4px 0 12px}
</style>
`

func writePlanetReport(buf *bytes.Buffer, g *Galaxy) {
	fmt.Fprintln(buf, `<section class="report">`)
	fmt.Fprintln(buf, `<h1>Planet Report</h1>`)
	for _, sys := range g.Systems {
		fmt.Fprintf(buf, "<h2>System %d,%d</h2>\n", sys.Hex.Q, sys.Hex.R)
		for i, star := range sys.Stars {
			fmt.Fprintf(buf, "<h3>Star %d — %s %s, size %d</h3>\n",
				i+1, starKindLabel(star.kind), starColorLabel(star.color), star.size)
			if len(star.Planets) == 0 {
				fmt.Fprintln(buf, `<p class="empty">No planets.</p>`)
				continue
			}
			fmt.Fprintln(buf, `<table><thead><tr><th>Orbit</th><th>Diameter (km)</th><th>Density</th><th>Gravity (g)</th><th>Temp</th><th>Pressure</th><th>Atmosphere</th><th>Mining</th></tr></thead><tbody>`)
			for orbit, p := range star.Planets {
				fmt.Fprintf(buf, "<tr><td>%d</td><td>%d</td><td>%.2f</td><td>%.2f</td><td>%d</td><td>%d</td><td>%s</td><td>%.0f</td></tr>\n",
					orbit+1, p.Diameter*1000, p.Density/100, p.Gravity/100,
					p.TemperatureClass, p.PressureClass,
					gasMixLabel(p.Gases), p.MiningDifficulty)
			}
			fmt.Fprintln(buf, `</tbody></table>`)
		}
	}
	fmt.Fprintln(buf, `</section>`)
}

func starKindLabel(k starType) string {
	switch k {
	case starDwarf:
		return "dwarf"
	case starDegenerate:
		return "degenerate"
	case starMainSequence:
		return "main-sequence"
	case starGiant:
		return "giant"
	}
	return "unknown"
}

func starColorLabel(c starColor) string {
	switch c {
	case colorBlue:
		return "blue"
	case colorBlueWhite:
		return "blue-white"
	case colorWhite:
		return "white"
	case colorYellowWhite:
		return "yellow-white"
	case colorYellow:
		return "yellow"
	case colorOrange:
		return "orange"
	case colorRed:
		return "red"
	}
	return "unknown"
}

func gasNameLabel(g AtmosphericGas) string {
	switch g {
	case GasH2:
		return "H2"
	case GasCH4:
		return "CH4"
	case GasHe:
		return "He"
	case GasNH3:
		return "NH3"
	case GasN2:
		return "N2"
	case GasCO2:
		return "CO2"
	case GasO2:
		return "O2"
	case GasHCl:
		return "HCl"
	case GasCl2:
		return "Cl2"
	case GasF2:
		return "F2"
	case GasH2O:
		return "H2O"
	case GasSO2:
		return "SO2"
	case GasH2S:
		return "H2S"
	}
	return "?"
}

func gasMixLabel(gases map[AtmosphericGas]int) string {
	if len(gases) == 0 {
		return "—"
	}
	keys := make([]AtmosphericGas, 0, len(gases))
	for k := range gases {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if gases[keys[i]] != gases[keys[j]] {
			return gases[keys[i]] > gases[keys[j]]
		}
		return keys[i] < keys[j]
	})
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%s %d%%", gasNameLabel(k), gases[k]))
	}
	return strings.Join(parts, ", ")
}
