# worldgen burndown

Cleanup items from code review. Items marked ✅ are done.

## ✅ 1. Collapse `hexes` sub-package

The `internal/worldgen/hexes` sub-package was mostly pass-through after the
`internal/hexes` extraction. Moved remaining code (placement generator, viewer)
into `internal/worldgen` and deleted the sub-package.

## ✅ 2. Merge duplicate `System` type

`hexmap.System` (flat `Stars int`) and `worldgen.System` (`Stars []*Star`) merged
into a single `worldgen.System`. The placement generator now returns placement
structs internally; callers use `worldgen.System` everywhere.

## 3. Galaxy is intentionally thin

`Galaxy` exists to bag `Radius` + `[]*System` for return from `Generate`. Once it
leaves the package the caller slices it up for database insertion. No changes needed.

## ✅ 4–5. Planet generation in `rollPlanet`

Moved the inline planet generation loop from `rollStar` into `rollPlanet`.
Removed the old no-op stub.

## ✅ 6. Export `Star` fields

Exported `Kind`, `Color`, `Size`, `NumPlanets` on `Star` and the `StarType` /
`StarColor` enum types. Added `String()` methods on both enums.

## ✅ 7. Document `-2` bias on `numPlanets`

Added comment explaining the `-2` is a bias to offset the multiple die rolls so
the average lands in a reasonable range.

## 8. Clamping loops can theoretically infinite-loop (known defect)

The `for value < min` / `for value > max` clamping patterns use random increments.
With a deterministic PRNG they terminate in practice, but they burn an
unpredictable number of RNG calls. This makes reproducibility fragile if roll
ranges change. Documented in code; needs a design review to replace with
bounded clamping.

## 9. Gas selection loop can spin (known defect)

The outer `for len(p.Gases) == 0` retry loop re-attempts the entire gas window
when every candidate is randomly skipped. With a narrow window (5 gases, each
~2/3 chance of skip) this occasionally burns many iterations. Documented in code;
needs a design review.

## 10. Half-exported fields on `Star`

Crud left over from iterating blindly. We aim to export all fields. Done in item 6.

## Remaining (not yet addressed)

- `Planet.Special` struct is unused by the generator but referenced in the
  reference docs for home-system and planet generation. Keep for now; implement
  when `rollPlanet` gets the full planet-special logic.
- ✅ `SystemsToHTML` / `SystemsToHTMLWithPixelSize` collapsed into a single
  `SystemsToHTML(systems, pixSize)` function.
- Label functions (`starKindLabel`, `gasNameLabel`) can become `String()` methods
  once all types are exported.
