"""Convert the MIT Magic-Tower-Qt campaign into browser-only static assets.

Usage: python frontend/src/games/token-tower/import-classic.py /path/to/Magic-Tower-Qt
No network access or credentials are used by this developer-time converter.
"""

import hashlib
import json
from pathlib import Path
import sqlite3
import subprocess
import sys

from PIL import Image


root = Path(__file__).resolve().parent
source = Path(sys.argv[1]).resolve()
revision = subprocess.check_output(["git", "-C", str(source), "rev-parse", "HEAD"], text=True).strip()
expected = "a32ee738485fa210e3a2b32cf57f506d51446c13"
if revision != expected:
    raise SystemExit(f"Expected audited revision {expected}, got {revision}")
database = source / "MagicTower/database/data.sql"
db = sqlite3.connect(":memory:")
db.executescript(database.read_text())
layers = db.execute("SELECT layer0, layer1, layer2, layer3, layer4, layer5, layer6, layer7, layer8, layer9 FROM map WHERE id = 1").fetchone()
floors = [[int(tile) for tile in layer.split()] for layer in layers]
assert len(floors) == 10 and all(len(floor) == 196 for floor in floors)
fields = ["id", "hp", "attack", "defense", "gold", "experience", "miss", "criticalThreshold"]
monsters = [dict(zip(fields, row)) for row in db.execute("SELECT id, hp, attack, defend, money, exp, miss, crit FROM monster ORDER BY id")]
output = root / "classic"
output.mkdir(exist_ok=True)
campaign = {"sourceRevision": revision, "sourceSha256": hashlib.sha256(database.read_bytes()).hexdigest(), "size": 14, "floors": floors, "monsters": monsters}
(output / "campaign.json").write_text(json.dumps(campaign, indent=2) + "\n")

public = root.parents[2] / "public/games/token-tower/classic"
public.mkdir(parents=True, exist_ok=True)
(public / "LICENSE.txt").write_bytes((source / "LICENSE").read_bytes())
atlas = Image.new("RGBA", (512, 256))
for path in (source / "MagicTower/assets").glob("*.png"):
    if not path.stem.isdecimal():
        continue
    tile = int(path.stem)
    sprite = Image.open(path).convert("RGBA")
    atlas.alpha_composite(sprite, ((tile % 16) * 32 + (32 - sprite.width) // 2, (tile // 16) * 32 + (32 - sprite.height) // 2))
atlas.save(public / "sprites.webp", lossless=True)
cover = Image.new("RGBA", (448, 448), "#20221e")
for index, tile in enumerate(floors[0]):
    x, y = index % 14, index // 14
    for number in [0, tile]:
        cover.alpha_composite(atlas.crop(((number % 16) * 32, (number // 16) * 32, (number % 16 + 1) * 32, (number // 16 + 1) * 32)), (x * 32, y * 32))
cover.alpha_composite(atlas.crop((13 * 32, 6 * 32, 14 * 32, 7 * 32)), (6 * 32, 12 * 32))
cover.convert("RGB").resize((896, 896), Image.Resampling.NEAREST).save(public / "cover.webp", quality=90)
print(json.dumps({"revision": revision, "floors": len(floors), "monsters": len(monsters), "spritesBytes": (public / "sprites.webp").stat().st_size}))
