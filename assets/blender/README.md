# Blender models

## Tent layout editor

**3D Twin → Tent layout** provides a centimetre-based layout for every catalog model, including six individual sensor modules exported by `export_layout_sensors.py`. Set tent width/height/depth, add equipment, select it in 3D or the object list, and resize its three dimensions. Drag translation arrows or use **Place on grid** and tap the floor. Position and elevation snap to 5, 10 or 25 cm; rotation uses 90° increments. Bounds include fittings and attached tubing. Objects stay inside the tent; a tent resize that cannot contain existing equipment is rejected.

The editor supports undo/redo and automatically saves a versioned layout in local browser storage, keyed by facility. These planning layouts are separate from farm records and are not currently included in `.grownerve.json` archives or synchronized to the server. Adding equipment searches for vacant space; subsequent placement allows intentional overlaps (for example probes within a reservoir). Scaling adjusts the complete model, including fittings.

## Fans, humidifiers and irrigation

`climate-systems.blend` contains clip and inline duct fans with separate rotors, guards, supports and power leads; ultrasonic and evaporative humidifiers with controls, tank details, intake/exhaust vents; and two-zone irrigation assemblies with drip stakes or watering rings. The irrigation hardware includes a supply tank, pump, filter, manifold, solenoid valves, controller and tubing. Materials reuse bundled CC0 metal and polymer maps.

Open **3D Twin → Climate & irrigation** to select all six variants. Fan rotation and ultrasonic mist have pause/intensity controls. The evaporative unit has no visible mist. These are model previews, not physical equipment control or connected irrigation schedules. `build_climate_systems.py` exports compact modules, and `prepare_previews.py` can render `preview('climate-systems')`. Dimensions and routing are illustrative.

## LED fixtures and soil pots

`grow-options.blend` and `grow-options.glb` contain panel, linear-bar and six-bar LED fixtures, with circuit boards, individual diodes, heat-sink fins, drivers and suspension eyes. Pots include ribbed nursery plastic, fabric with lifting handles, and terracotta ceramic with a saucer. Each has a hollow interior and central drainage opening. A separate soil insert has modeled soil aggregates and perlite. Hardware and fabric reuse bundled CC0 maps; soil granules are original geometry.

Open **3D Twin → Lights & pots** to select styles, diameter and height (20–50 cm), soil fill, and visual LED brightness. These settings affect the preview only. They do not create farm records, control physical equipment, or estimate light output. `build_grow_options.py` rebuilds the seven compact modules; `prepare_previews.py` can render `preview('grow-options')`.

## Hydroponic tower

The sensor package includes a vented air humidity/temperature housing, PAR/light sensor, blue pH probe with guarded tip, amber EC probe with electrode rings, stainless water-temperature probe, and float-level sensor. Reservoir probes have bulkhead glands, strain reliefs and leads into a labeled hub. Crown sensors follow the chosen tower height. These are illustrative visual assets without live telemetry. `tower_sensors.py` runs during the tower build. After the studio pass, `sensor_preview.py` saves `sensor-probes.blend` and a separate close-up showing the four immersed probe designs outside the reservoir.

`hydroponic-tower.blend` adds a reference-inspired tower with six staggered levels and 18 angled planting sites. It includes a tapered reservoir, lime lid, separate service caps, drain, tier seams, hollow planting necks, slotted baskets, locking nubs and a top feed fitting. Metal and polymer materials use bundled ambientCG CC0 roughness and normal maps.

In **3D Twin → Hydroponic tower**, choose 3, 4, 6 or 8 levels (9–24 sites). This is a model preview and does not create farm records. Existing scene layouts can render it with the `hydroponic_tower` profile on a zone; child zones of type `level` determine the count, defaulting to six. The supported range is three to eight.

`build_tower.py` creates the editable assembled model and exports `hydroponic-tower.glb` as three reusable modules: base, tier and crown. Meshes are grouped by module/material for fewer draw calls. Unlike the seven geometry libraries below, this GLB includes full materials. The reference image guides appearance; dimensions and planting capacity are illustrative, not fabrication or growing-system specifications.

The detail pass adds socket gaskets, collar edge beads, coupler ribs, captive lid screws, a protective bumper and a decorative reservoir sight gauge. The gauge is static geometry, not a live water reading. After building, execute `prepare_previews.py`, call `preview('hydroponic-tower', render=False)`, then execute `tower_studio.py` for the Cycles studio render with a matte floor and contact shadows. The studio stays in the Blender file; the browser GLB contains only the three equipment modules.

The seven `.blend` files contain the existing GrowNerve objects imported individually, refined in Blender 5.2.1, and supplied with CC0 textures. Each has a studio camera, lights, and a packed Poly Haven HDRI. `previews/` contains renders of each object.

| File | Changes |
| --- | --- |
| `lettuce.blend` | Wider asymmetric leaves, a lower crown, scalloped edges, raised midribs and secondary veins; beveled basket details |
| `tent.blend` | Additional lining folds; beveled poles, joints, vent and zipper hardware |
| `reservoir.blend` | Beveled reinforcement, hardware and fittings |
| `light.blend` | Beveled LED bars, cooling fins, fasteners and cable hardware |
| `fan.blend` | Progressive blade pitch, corrected normals and beveled hardware |
| `controller.blend` | Beveled vent, display, gland and fastener details |
| `pump.blend` | Beveled vent and outlet details |

These remain illustrative models, not manufacturer CAD or photogrammetry. The lettuce material uses a tree-leaf atlas for surface detail; it is not a scanned lettuce plant. See [CC0 sources and license](../../frontend/public/textures/cc0/LICENSE.md).

## Round trip

1. Start the frontend with `npm run dev:browser` and run `node scripts/export-for-blender.mjs` from `frontend`. The authoring page exports the original components, not the refined geometry wrapper. `imports/` retains those textured GLBs as source references.
2. In Blender, execute `refine_models.py`, then call `refine("lettuce")` (or another name from the table). It creates a separate scene, saves its editable `.blend`, and writes a compact geometry library to `frontend/public/models/blender/`.
3. Execute `prepare_previews.py`, then `preview("lettuce")` to add a studio setup and render a preview. The preview applies the app's illustrative proportions, particularly the flattened grow light.
4. Run `node scripts/validate-blender-assets.mjs`, the frontend checks, and the detailed-twin browser test.

The scripts currently set `ROOT` to this checkout; update it when moving the project. The source imports are retained to keep the first Blender pass reproducible.

## Browser assets

The GLBs in `frontend/public/models/blender/` are **libraries of revised local mesh geometry**, not assembled stand-alone models. Each mesh is keyed to its original geometry. Use the `.blend` files for complete editable objects.

The browser exchanges matching geometry while preserving the existing object transforms, instances, CC0 materials, fan animation, LEDs, picking, water-level display and reservoir cutaway. A vertex correspondence attribute preserves the lettuce's live outer-leaf health colors even when Blender splits or reorders vertices.

Desktop foliage uses the refined mesh. Lower geometry profiles keep their smaller procedural foliage; matching equipment details still use refinements. Low-power mode uses the procedural models throughout. Assets load locally, show the existing geometry while loading, and are precached for offline use.
