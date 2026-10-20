#!/usr/bin/env python3
"""Export the original SwapLook vector mark and its browser/app icon sizes."""

from pathlib import Path
import subprocess
import xml.etree.ElementTree as ET


ROOT = Path(__file__).resolve().parents[1]
PUBLIC = ROOT / "swaplook_FE/public"
BRAND = PUBLIC / "brand"
BRAND.mkdir(parents=True, exist_ok=True)


def mark(primary="#4f46e5", secondary="#c7d2fe"):
    return f'''<rect width="72" height="72" rx="20" fill="{primary}"/>
  <path d="M19 15H39L51 27L39 39H19Z" fill="#ffffff" stroke="#ffffff" stroke-width="4" stroke-linejoin="round"/>
  <circle cx="39" cy="27" r="2.5" fill="{primary}"/>
  <path d="M53 33H33L21 45L33 57H53Z" fill="{secondary}" stroke="{primary}" stroke-width="6" stroke-linejoin="round"/>
  <circle cx="33" cy="45" r="2.5" fill="{primary}"/>'''


# Custom outlined lettering: no external font or font installation required.
LETTERING = '''<g fill="none" stroke="#202232" stroke-width="4" stroke-linecap="round" stroke-linejoin="round">
  <path d="M107 27C102 23 92 23 92 29C92 36 107 32 107 40C107 47 95 47 91 43"/>
  <path d="M116 25L122 45L130 29L138 45L144 25"/>
  <ellipse cx="159" cy="35" rx="9" ry="10"/><path d="M168 25V45"/>
  <path d="M178 25V55"/><ellipse cx="187" cy="35" rx="9" ry="10"/>
  <path d="M207 16V41Q207 45 211 45"/>
  <ellipse cx="227" cy="35" rx="9" ry="10"/>
  <ellipse cx="254" cy="35" rx="9" ry="10"/>
  <path d="M275 16V45M290 25L276 35L291 45"/>
</g>'''


def svg(viewbox, content, title):
    return f'''<svg xmlns="http://www.w3.org/2000/svg" viewBox="{viewbox}" fill="none" role="img" aria-labelledby="title">
<title id="title">{title}</title>
{content}
</svg>
'''


icon = svg("0 0 72 72", mark(), "SwapLook — two exchanging clothing tags")
logo = svg("0 0 310 72", mark() + LETTERING + '<circle cx="304" cy="44" r="3" fill="#4f46e5"/>', "SwapLook")
mono = svg("0 0 310 72", mark("#202232", "#ffffff") + LETTERING + '<circle cx="304" cy="44" r="3" fill="#202232"/>', "SwapLook monochrome")
(BRAND / "swaplook-icon.svg").write_text(icon)
(BRAND / "swaplook-logo.svg").write_text(logo)
(BRAND / "swaplook-logo-monochrome.svg").write_text(mono)
(PUBLIC / "favicon.svg").write_text(icon)


def raster(source, destination, width):
    # Draw the small SVG shape vocabulary explicitly: ImageMagick's built-in
    # SVG delegate can drop inherited strokes on some local installations.
    root = ET.fromstring(source.read_text())
    _, _, vw, vh = map(float, root.attrib["viewBox"].split())
    scale = width / vw
    commands = [f"scale {scale},{scale}"]

    def draw(element, inherited):
        attrs = inherited | element.attrib
        tag = element.tag.split("}")[-1]
        if tag in ("svg", "g"):
            for child in element:
                draw(child, attrs)
            return
        shape = None
        if tag == "path":
            shape = f"path '{attrs['d']}'"
        elif tag in ("circle", "ellipse"):
            rx = attrs.get("r", attrs.get("rx"))
            ry = attrs.get("r", attrs.get("ry"))
            shape = f"ellipse {attrs['cx']},{attrs['cy']} {rx},{ry} 0,360"
        elif tag == "rect":
            x, y = float(attrs.get("x", 0)), float(attrs.get("y", 0))
            right, bottom = x + float(attrs["width"]), y + float(attrs["height"])
            radius = attrs.get("rx", "0")
            shape = f"roundrectangle {x},{y} {right},{bottom} {radius},{radius}"
        if shape:
            commands.extend(["push graphic-context", f"fill '{attrs.get('fill', 'none')}'", f"stroke '{attrs.get('stroke', 'none')}'", f"stroke-width {attrs.get('stroke-width', '1')}", f"stroke-linecap {attrs.get('stroke-linecap', 'butt')}", f"stroke-linejoin {attrs.get('stroke-linejoin', 'miter')}", shape, "pop graphic-context"])

    draw(root, {})
    height = round(vh * scale)
    subprocess.run(["magick", "-size", f"{width}x{height}", "xc:none", "-draw", "\n".join(commands), str(destination)], check=True)


for size in (192, 512, 1024):
    raster(BRAND / "swaplook-icon.svg", BRAND / f"swaplook-icon-{size}.png", size)
raster(BRAND / "swaplook-logo.svg", BRAND / "swaplook-logo.png", 1240)
raster(BRAND / "swaplook-icon.svg", PUBLIC / "apple-touch-icon.png", 180)
subprocess.run(["magick", str(BRAND / "swaplook-icon-512.png"), "-define", "icon:auto-resize=64,48,32,16", str(PUBLIC / "favicon.ico")], check=True)
print("Exported SwapLook SVG logos, PNG icons, Apple touch icon and multi-size favicon.")
