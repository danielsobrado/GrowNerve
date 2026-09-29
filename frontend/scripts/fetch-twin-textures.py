"""Fetch pinned ambientCG CC0 sets and prepare local web-sized PBR maps.

Run from frontend: python scripts/fetch-twin-textures.py
Requires Pillow. Original ZIPs are cached outside the repository.
"""
import concurrent.futures
import hashlib
import io
import json
from pathlib import Path
import tempfile
import urllib.request
import zipfile
from PIL import Image

ROOT = Path(__file__).resolve().parents[1] / "public" / "textures" / "cc0"
CACHE = Path(tempfile.gettempdir()) / "grownerve-cc0"
ASSETS = {"metal": "Metal032", "plastic": "Plastic010", "fabric": "Fabric005", "concrete": "Concrete034", "leaf": "LeafSet001"}


def prepare(entry):
    name, asset = entry
    url = f"https://ambientcg.com/get?file={asset}_1K-JPG.zip"
    archive = CACHE / f"{asset}_1K-JPG.zip"
    if not archive.exists():
        request = urllib.request.Request(url, headers={"User-Agent": "GrowNerve asset preparation"})
        with urllib.request.urlopen(request, timeout=120) as response:
            archive.write_bytes(response.read())
    output = ROOT / name
    output.mkdir(parents=True, exist_ok=True)
    records = []
    with zipfile.ZipFile(archive) as source:
        print(asset, source.namelist(), flush=True)
        for channel in ["Color", "NormalGL", "Roughness"]:
            matches = [f for f in source.namelist() if f.endswith(f"_{channel}.jpg")]
            if len(matches) != 1:
                raise ValueError(f"Missing {asset} {channel}")
            original = Image.open(io.BytesIO(source.read(matches[0]))).convert("RGB")
            for size in [512, 1024]:
                image = original.resize((size, size), Image.Resampling.LANCZOS)
                target = output / f"{channel.lower()}-{size}.jpg"
                image.save(target, quality=90 if channel == "NormalGL" else 86, subsampling=0, optimize=True)
                records.append({"file": str(target.relative_to(ROOT)).replace("\\", "/"), "bytes": target.stat().st_size, "sha256": hashlib.sha256(target.read_bytes()).hexdigest()})
    return {"key": name, "asset": asset, "source": f"https://ambientcg.com/a/{asset}", "download": url, "license": "CC0-1.0", "archiveSha256": hashlib.sha256(archive.read_bytes()).hexdigest(), "files": records}


if __name__ == "__main__":
    CACHE.mkdir(exist_ok=True)
    ROOT.mkdir(parents=True, exist_ok=True)
    with concurrent.futures.ThreadPoolExecutor(max_workers=3) as pool:
        assets = list(pool.map(prepare, ASSETS.items()))
    hdr_url = "https://dl.polyhaven.org/file/ph-assets/HDRIs/hdr/1k/studio_small_09_1k.hdr"
    hdr = ROOT / "studio_small_09_1k.hdr"
    if not hdr.exists():
        with urllib.request.urlopen(hdr_url, timeout=120) as response:
            hdr.write_bytes(response.read())
    environment = {"provider": "Poly Haven / Sergej Majboroda", "source": "https://polyhaven.com/a/studio_small_09", "license": "CC0-1.0", "licenseUrl": "https://polyhaven.com/license", "download": hdr_url, "file": hdr.name, "bytes": hdr.stat().st_size, "sha256": hashlib.sha256(hdr.read_bytes()).hexdigest()}
    (ROOT / "manifest.json").write_text(json.dumps({"provider": "ambientCG / Lennart Demes", "licenseUrl": "https://docs.ambientcg.com/license/", "processing": "1K source maps resized and JPEG encoded; OpenGL normals kept in linear color space. HDRI bundled unchanged.", "assets": assets, "environment": environment}, indent=2) + "\n", encoding="utf-8")
    print("Texture bytes:", sum(f["bytes"] for a in assets for f in a["files"]))
