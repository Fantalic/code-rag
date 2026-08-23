"""Generate the CodeRAG favicon.png from the favicon design.

Draws the same shapes as public/favicon.svg (rounded tile, code brackets,
graph triangle) with Pillow and saves a 64x64 PNG next to it. Re-run after
changing the SVG design to keep both in sync.

Usage: python scripts/dev/make_favicon.py
Requires: pillow (pip install pillow)
"""

from pathlib import Path

from PIL import Image, ImageDraw

# Render at 8x and downscale for smooth antialiased edges.
SCALE = 8
SIZE = 64 * SCALE

# Colors mirror the SVG palette (emerald on zinc).
BG = (24, 24, 27, 255)  # zinc-900
EMERALD = (52, 211, 153, 255)  # emerald-400
EDGE = (52, 211, 153, 140)  # emerald-400 @ ~55% opacity


def s(v: float) -> float:
    """Scale an SVG-space coordinate."""
    return v * SCALE


def main() -> None:
    img = Image.new("RGBA", (SIZE, SIZE), (0, 0, 0, 0))
    d = ImageDraw.Draw(img)

    # Rounded square tile (rx=14 in SVG space).
    d.rounded_rectangle((0, 0, SIZE - 1, SIZE - 1), radius=s(14), fill=BG)

    # Code brackets < > (SVG paths M21 18 L11 32 L21 46 / mirrored).
    w = max(1, round(s(7)))
    d.line([(s(21), s(18)), (s(11), s(32)), (s(21), s(46))],
           fill=EMERALD, width=w, joint="curve")
    d.line([(s(43), s(18)), (s(53), s(32)), (s(43), s(46))],
           fill=EMERALD, width=w, joint="curve")

    # Graph triangle edges + nodes.
    ew = max(1, round(s(3)))
    pts = [(s(32), s(25)), (s(27), s(39)), (s(37), s(39))]
    d.polygon(pts, outline=EDGE, width=ew)
    r = s(3.5)
    for x, y in pts:
        d.ellipse((x - r, y - r, x + r, y + r), fill=EMERALD)

    out = Path(__file__).resolve().parents[2] / (
        "coderag/lightrag/lightrag_webui/public/favicon.png"
    )
    img.resize((64, 64), Image.LANCZOS).save(out)
    print(f"wrote {out}")


if __name__ == "__main__":
    main()
