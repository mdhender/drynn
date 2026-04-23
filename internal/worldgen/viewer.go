// Copyright (c) 2026 Michael D Henderson. All rights reserved.

package worldgen

import (
	"bytes"
	"fmt"
	"sort"
	"strings"
)

// ToHTML renders a self-contained HTML page showing the cluster's hex map.
// If showPlanets is true, a per-system planet report is appended below the
// map. If showDeposits is true, a collapsible <details> block per planet
// is appended inside the planet report — showDeposits implies showPlanets
// at the call site. If pixelSize <= 0, a size is derived to fit within
// roughly 1280x1280.
func (g *Cluster) ToHTML(pixelSize float64, showCoords, showPlanets, showDeposits bool) []byte {
	starCounts := make(map[int]int, len(g.Systems))
	for _, s := range g.Stars {
		starCounts[s.SystemID]++
	}
	systems := make([]viewerSystem, 0, len(g.Systems))
	for _, s := range g.Systems {
		systems = append(systems, viewerSystem{Hex: s.Hex, Stars: starCounts[s.ID]})
	}

	var buf bytes.Buffer
	buf.WriteString("<!DOCTYPE html>\n<html lang=\"en\">\n<head>\n")
	buf.WriteString("<meta charset=\"UTF-8\">\n<title>Cluster</title>\n")
	buf.WriteString(clusterPageCSS)
	buf.WriteString("</head>\n<body>\n")

	buf.WriteString("<div class=\"map\">\n")
	buf.Write(RenderDiskSVG(g.Radius, systems, pixelSize, showCoords))
	buf.WriteString("</div>\n")

	if showPlanets {
		writePlanetReport(&buf, g, showDeposits)
	}

	buf.WriteString("</body>\n</html>\n")
	return buf.Bytes()
}

const clusterPageCSS = `<style>
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
.report details.deposits{margin:0 0 6px;padding:4px 8px;background:#fafafa;border:1px solid #eee;border-radius:3px;font-size:12px}
.report details.deposits summary{cursor:pointer;color:#555;padding:2px 0}
.report details.deposits[open] summary{color:#222;font-weight:500;border-bottom:1px solid #ddd;margin-bottom:4px}
.report details.deposits table{margin:4px 0 0}
</style>
`

func writePlanetReport(buf *bytes.Buffer, g *Cluster, showDeposits bool) {
	starsBySys := make(map[int][]*Star, len(g.Systems))
	for _, s := range g.Stars {
		starsBySys[s.SystemID] = append(starsBySys[s.SystemID], s)
	}
	planetsByStar := make(map[int][]*Planet, len(g.Stars))
	for _, p := range g.Planets {
		planetsByStar[p.StarID] = append(planetsByStar[p.StarID], p)
	}
	var depositsByPlanet map[int][]*Deposit
	if showDeposits {
		depositsByPlanet = make(map[int][]*Deposit, len(g.Planets))
		for _, d := range g.Deposits {
			depositsByPlanet[d.PlanetID] = append(depositsByPlanet[d.PlanetID], d)
		}
	}

	fmt.Fprintln(buf, `<section class="report">`)
	fmt.Fprintln(buf, `<h1>Planet Report</h1>`)
	for _, sys := range g.Systems {
		fmt.Fprintf(buf, "<h2>System %d,%d</h2>\n", sys.Hex.Q, sys.Hex.R)
		for i, star := range starsBySys[sys.ID] {
			fmt.Fprintf(buf, "<h3>Star %d — %s %s, size %d</h3>\n",
				i+1, star.Kind, star.Color, star.Size)
			planets := planetsByStar[star.ID]
			if len(planets) == 0 {
				fmt.Fprintln(buf, `<p class="empty">No planets.</p>`)
				continue
			}
			fmt.Fprintln(buf, `<table><thead><tr><th>Orbit</th><th>Kind</th><th>Diameter (km)</th><th>Density</th><th>Gravity (g)</th><th>Temp</th><th>Pressure</th><th>Atmosphere</th><th>Mining</th></tr></thead><tbody>`)
			for _, p := range planets {
				fmt.Fprintf(buf, "<tr><td>%d</td><td>%s</td><td>%d</td><td>%.2f</td><td>%.2f</td><td>%d</td><td>%d</td><td>%s</td><td>%.0f</td></tr>\n",
					p.Orbit, p.Kind, p.Diameter*1000, p.Density, p.Gravity,
					p.TemperatureClass, p.PressureClass,
					gasMixLabel(p.Gases), p.MiningDifficulty)
			}
			fmt.Fprintln(buf, `</tbody></table>`)
			if showDeposits {
				for _, p := range planets {
					writeDepositsDetails(buf, p, depositsByPlanet[p.ID])
				}
			}
		}
	}
	fmt.Fprintln(buf, `</section>`)
}

func writeDepositsDetails(buf *bytes.Buffer, p *Planet, deposits []*Deposit) {
	if len(deposits) == 0 {
		fmt.Fprintf(buf, `<details class="deposits"><summary>Orbit %d (%s) — no deposits</summary></details>`+"\n",
			p.Orbit, p.Kind)
		return
	}
	fmt.Fprintf(buf, `<details class="deposits"><summary>Orbit %d (%s) — %d deposits</summary>`+"\n",
		p.Orbit, p.Kind, len(deposits))
	fmt.Fprintln(buf, `<table><thead><tr><th>ID</th><th>Resource</th><th>Unit</th><th>Quantity</th><th>Yield %</th><th>Mining</th></tr></thead><tbody>`)
	for _, d := range deposits {
		fmt.Fprintf(buf, "<tr><td>%d</td><td>%s</td><td>%s</td><td>%s</td><td>%.0f</td><td>%.0f</td></tr>\n",
			d.ID, d.Resource.Label(), d.Resource.UnitCode(),
			formatQuantity(d.Quantity), d.YieldPct*100, d.MiningDifficulty)
	}
	fmt.Fprintln(buf, `</tbody></table></details>`)
}

func formatQuantity(q int) string {
	s := fmt.Sprintf("%d", q)
	n := len(s)
	if n <= 3 {
		return s
	}
	var out []byte
	for i, c := range s {
		if i > 0 && (n-i)%3 == 0 {
			out = append(out, ',')
		}
		out = append(out, byte(c))
	}
	return string(out)
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
		parts = append(parts, fmt.Sprintf("%s %d%%", k, gases[k]))
	}
	return strings.Join(parts, ", ")
}
