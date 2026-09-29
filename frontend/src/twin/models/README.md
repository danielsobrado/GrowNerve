# Built-in grow equipment

## Blender refinements

The models now load locally authored geometry from Blender through `RefinedGeometry.tsx`. The seven editable `.blend` files, studio previews, and round-trip instructions are in [`assets/blender`](../../../../assets/blender/README.md). Geometry exchange preserves live materials, animation, picking, occupancy and reservoir controls. Desktop foliage retains its source vertex correspondence so health coloring still affects the older outer leaves. Smaller device profiles retain reduced procedural foliage; low-power mode skips the extra assets.

The digital twin builds these local Three.js models and loads bundled CC0 PBR textures. It makes no requests to external asset providers at runtime:

- Bibb lettuce with cupped, rippled leaves, vertex-colored veins, older outer leaves for attention state, clay pebbles, and slotted net baskets. Empty positions omit foliage.
- Cutaway tent with reflective lining, fabric backing, poles, corner joints, roof rails, stitching, zipper teeth, and a ventilation grille.
- DWC reservoir with a gasket, reinforced sides, handles, drain valve, calibrated sight glass, and air hoses. The optional cutaway exposes water, two air stones, and illustrative roots beneath occupied positions.
- Six-bar LED fixture with individual emitters, cooling fins, driver housing, wiring, and suspension cables.
- Circulation fan with five curved blades, concentric guard wires, radial spokes, motor housing, and bracket.
- Controller enclosure with fasteners, ventilation, status screen, cable glands, and attached air-sensor housing.
- Dual-outlet air pump with vents, rubber feet, fittings, and hoses.

`GrowModels.tsx` composes the objects; `Parts.tsx` provides reusable hardware and instanced details. `FineDetails.tsx` adds hex fasteners with recessed crosses, ribbed collars, creased foil, and merged branching roots. `lettuceGeometry.ts` generates deterministic foliage with UVs, up to 32 leaves and 55,296 triangles per desktop plant. Lower device profiles reduce that budget.

## CC0 materials

Five ambientCG sets supply color, OpenGL normal, and roughness maps for metal, molded plastic, fabric, concrete, and foliage. A Poly Haven studio HDRI supplies reflections and environment light. Sources, licenses, and hashes are bundled in [`public/textures/cc0`](../../../public/textures/cc0/LICENSE.md). The tree-leaf atlas supplies vein detail on the authored lettuce shapes; it is not an exact lettuce scan.

Desktop uses 1024px maps; other profiles use 512px maps. Texture sources are cached, material variants share GPU textures, and the last consumer releases its variant. Color maps use sRGB; normal/roughness maps stay linear. Materials show a plain fallback while loading and recompile when maps arrive. All images and the HDRI are precached for offline browser use. The existing PTL implementation remains available but these detailed models use the CC0 material layer. No additional runtime dependencies are introduced.

## Scene integration

Models retain the pilot's existing local pivots and illustrative scale conventions; these are not manufacturer CAD dimensions. `sceneBindings()` supplies missing controller/pump visuals from existing devices in rendered zones without changing persisted layouts or creating domain records. Explicit bindings take precedence.

Fan rotation follows reported power/output, LED emission follows power, the water gauge and cutaway follow reservoir fill, and plant color/occupancy follow the plant-position record. Roots illustrate a DWC system; they are not a measured or predicted root condition. All parts bubble picking to the existing entity selection and action flow.

Foliage subdivisions and LED counts follow the device quality profile. Repeated hardware uses instancing, and custom geometries are disposed on unmount. The cutaway interior is mounted only when requested. Materials support both existing WebGPU and WebGL render paths.

## Verification

Run `npm run build:browser`, `npm run lint`, `npm test`, and `npx playwright test e2e/detailed-twin.spec.ts --workers=1` from `frontend`. Browser tests verify the 15 local texture-map responses and save exterior/cutaway screenshots for desktop and mobile plus a desktop detail view under `test-results`.
