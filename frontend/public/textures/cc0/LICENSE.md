# CC0 texture sources

All texture images in this folder are derived from **ambientCG**, by Lennart Demes, and are released under **CC0 1.0 Universal**.

- Provider license: https://docs.ambientcg.com/license/
- CC0 legal text: https://creativecommons.org/publicdomain/zero/1.0/legalcode
- Metal032: https://ambientcg.com/a/Metal032 — equipment metal and reflective tent lining
- Plastic010: https://ambientcg.com/a/Plastic010 — reservoir, baskets, fan and equipment enclosures
- Fabric005: https://ambientcg.com/a/Fabric005 — tent fabric weave (normal/roughness; color is supplied by the material)
- Concrete034: https://ambientcg.com/a/Concrete034 — floor and clay pebble surface detail
- LeafSet001: https://ambientcg.com/a/LeafSet001 — leaf vein and surface detail

The leaf atlas is a tree-leaf source used for vein detail on authored lettuce geometry, not a botanically exact lettuce scan. UVs sample the interior of the lower middle leaf. No opacity cutout is used.

Source 1K JPG maps are resized/re-encoded into 1024px and 512px variants. Color maps use sRGB; OpenGL normal and roughness maps use linear data. Atlas images are otherwise unchanged. All modified texture images remain CC0. Source URLs, archive hashes, output hashes, and sizes are recorded in `manifest.json`.

Regenerate with `python scripts/fetch-twin-textures.py` from `frontend` (requires Pillow). Files are bundled locally and precached by the PWA; the running app does not contact ambientCG.

This dedication applies to these texture images, not to the application's source code or branding.
