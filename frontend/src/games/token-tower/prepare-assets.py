"""Adapt the CC0 Kenney Tiny Dungeon archive into local TOKEN Tower assets."""
import sys
import zipfile
from io import BytesIO
from pathlib import Path

from PIL import Image, ImageDraw, ImageEnhance


root = Path(__file__).resolve().parents[4] / 'frontend/public/games/token-tower'
tiles = root / 'tiles'
tiles.mkdir(parents=True, exist_ok=True)
archive = zipfile.ZipFile(sys.argv[1])


def source(index):
    return Image.open(BytesIO(archive.read(f'Tiles/tile_{index:04}.png'))).convert('RGBA')


sprites = {
    'wall': 14, 'wall-carved': 28, 'floor': 48, 'floor-worn': 49,
    'hero': 97, 'merchant': 84, 'slime': 108, 'bat': 120,
    'skeleton': 121, 'guard': 96, 'mage': 111, 'warden': 122,
    'demon': 110, 'beast': 124, 'sword': 103, 'shield': 102,
    'potion': 115, 'chest': 89, 'stairs-up': 18, 'stairs-down': 38,
    'torch': 29, 'rune': 56, 'altar': 32, 'gate': 41, 'slash': 62,
}
for name, index in sprites.items():
    tile = source(index)
    if name.startswith('floor'):
        tile = ImageEnhance.Color(tile).enhance(0.16)
        tile = ImageEnhance.Brightness(tile).enhance(0.48)
    tile.save(tiles / f'{name}.png')

for color, rgb in [('red', (238, 100, 96)), ('blue', (90, 178, 238)), ('yellow', (244, 199, 101))]:
    door = source(45)
    pixels = door.load()
    for y in range(16):
        for x in range(16):
            r, g, b, a = pixels[x, y]
            if a and r > g * 1.1 and r > b * 1.1:
                factor = max(0.35, min(1, r / 180))
                pixels[x, y] = (*[round(c * factor) for c in rgb], a)
    door.save(tiles / f'door-{color}.png')
    key = Image.new('RGBA', (16, 16))
    draw = ImageDraw.Draw(key)
    draw.rectangle((3, 3, 8, 8), fill=(*rgb, 255))
    draw.rectangle((5, 5, 6, 6), fill=(0, 0, 0, 0))
    draw.rectangle((7, 8, 10, 12), fill=(*rgb, 255))
    draw.rectangle((10, 11, 12, 12), fill=(*rgb, 255))
    draw.rectangle((3, 3, 8, 3), fill=(255, 236, 198, 255))
    key.save(tiles / f'key-{color}.png')

for color, rgb in [('red', (243, 111, 125)), ('blue', (96, 198, 242))]:
    gem = Image.new('RGBA', (16, 16))
    draw = ImageDraw.Draw(gem)
    draw.polygon([(5, 3), (10, 3), (13, 7), (8, 13), (2, 7)], fill=(*rgb, 255))
    draw.polygon([(5, 3), (8, 6), (2, 7)], fill=(255, 237, 209, 255))
    draw.polygon([(8, 6), (13, 7), (8, 13)], fill=tuple(round(c * 0.65) for c in rgb) + (255,))
    gem.save(tiles / f'gem-{color}.png')

scene = Image.new('RGBA', (384, 240), '#161c27')
for y in range(15):
    for x in range(24):
        name = 'wall' if y < 2 or y > 12 or x < 2 or x > 21 or (x in [5, 18] and y % 4 != 3) else 'floor-worn' if (x * 3 + y) % 8 == 0 else 'floor'
        tile = Image.open(tiles / f'{name}.png').convert('RGBA')
        scene.alpha_composite(tile, (x * 16, y * 16))
for x, y, name in [(3, 3, 'torch'), (20, 3, 'torch'), (3, 11, 'torch'), (20, 11, 'torch'), (12, 3, 'altar'), (11, 3, 'gate'), (13, 3, 'gate'), (12, 6, 'demon'), (9, 7, 'warden'), (15, 7, 'mage'), (8, 10, 'slime'), (16, 10, 'skeleton'), (12, 10, 'hero'), (11, 10, 'sword'), (13, 10, 'shield'), (7, 4, 'chest'), (16, 4, 'chest'), (7, 11, 'gem-red'), (17, 11, 'gem-blue')]:
    scene.alpha_composite(Image.open(tiles / f'{name}.png').convert('RGBA'), (x * 16, y * 16))
scene.convert('RGB').resize((1536, 960), Image.Resampling.NEAREST).save(root / 'cover.webp', quality=92)
(root / 'License.txt').write_bytes(archive.read('License.txt'))
print(f'Wrote {len(list(tiles.glob("*.png")))} sprites and cover.webp')
