"""Copy existing cat photos to unique storage keys used by demo_profiles.sql."""
from pathlib import Path
import shutil

photos = Path(__file__).resolve().parents[1] / "data" / "cats"
sources = sorted(photos.glob("*.jpg"))
if not sources:
    raise SystemExit(f"Add JPG photos to {photos} first")
output = photos / "demo"
output.mkdir(exist_ok=True)
for i in range(1, 16):
    target = output / f"profile_{i:02}.jpg"
    if not target.exists():
        shutil.copyfile(sources[(i - 1) % len(sources)], target)
print(f"Prepared 15 demo photo paths in {output}")
