# Built-in grow equipment

The digital twin builds these local Three.js models without downloading models, textures, or fonts:

- Bibb lettuce with cupped, rippled leaves, vertex-colored veins, older outer leaves for attention state, clay pebbles, and slotted net baskets. Empty positions omit foliage.
- Cutaway tent with reflective lining, fabric backing, poles, corner joints, roof rails, stitching, zipper teeth, and a ventilation grille.
- DWC reservoir with a gasket, reinforced sides, handles, drain valve, calibrated sight glass, and air hoses. The optional cutaway exposes water, two air stones, and illustrative roots beneath occupied positions.
- Six-bar LED fixture with individual emitters, cooling fins, driver housing, wiring, and suspension cables.
- Circulation fan with five curved blades, concentric guard wires, radial spokes, motor housing, and bracket.
- Controller enclosure with fasteners, ventilation, status screen, cable glands, and attached air-sensor housing.
- Dual-outlet air pump with vents, rubber feet, fittings, and hoses.

`GrowModels.tsx` composes the objects; `Parts.tsx` provides reusable hardware and instanced details. `lettuceGeometry.ts` generates deterministic foliage. Existing PTL surface recipes supply the reservoir, fixture, tent fabric, and floor materials. No additional runtime dependencies are introduced.

## Scene integration

Models retain the pilot's existing local pivots and illustrative scale conventions; these are not manufacturer CAD dimensions. `sceneBindings()` supplies missing controller/pump visuals from existing devices in rendered zones without changing persisted layouts or creating domain records. Explicit bindings take precedence.

Fan rotation follows reported power/output, LED emission follows power, the water gauge and cutaway follow reservoir fill, and plant color/occupancy follow the plant-position record. Roots illustrate a DWC system; they are not a measured or predicted root condition. All parts bubble picking to the existing entity selection and action flow.

Foliage subdivisions and LED counts follow the device quality profile. Repeated hardware uses instancing, and custom geometries are disposed on unmount. The cutaway interior is mounted only when requested. Materials support both existing WebGPU and WebGL render paths.

## Verification

Run `npm run build:browser`, `npm run lint`, `npm test`, and `npx playwright test e2e/detailed-twin.spec.ts --workers=1` from `frontend`. Browser tests save exterior/cutaway screenshots for desktop and mobile under `test-results` for visual inspection.
