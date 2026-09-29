# Blender models

## Hydroponic tower

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
