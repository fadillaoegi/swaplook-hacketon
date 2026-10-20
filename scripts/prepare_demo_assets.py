#!/usr/bin/env python3
"""Prepare only repository-supplied demo inputs; never generate images or overwrite files."""

import argparse
import json
from pathlib import Path
import shutil
import struct

ROOT = Path(__file__).resolve().parents[1]


def comparable_bytes(path):
    data = path.read_bytes()
    if not data.startswith(b"\x89PNG\r\n\x1a\n"):
        return data
    # Retain an existing PNG whose image bytes match but whose private text/EXIF
    # metadata differs from the sanitized repository export. Never rewrite it.
    result, offset = bytearray(data[:8]), 8
    while offset + 12 <= len(data):
        size = struct.unpack(">I", data[offset:offset + 4])[0]
        end = offset + size + 12
        if end > len(data):
            return data
        if data[offset + 4:offset + 8] not in (b"eXIf", b"tEXt", b"zTXt", b"iTXt"):
            result.extend(data[offset:end])
        offset = end
    return bytes(result) if offset == len(data) else data


parser = argparse.ArgumentParser(description=__doc__)
parser.add_argument("--output", type=Path, default=ROOT / ".data")
parser.add_argument("--check", action="store_true", help="Validate the plan without copying")
args = parser.parse_args()

sources = [(ROOT / f"swaplook_FE/public/catalog/SL-{i:03}.jpg", args.output / f"garments/SL-{i:03}.jpg") for i in range(1, 13)]
models = ROOT / "demo-assets/models"
entries = json.loads((models / "models.json").read_text())
assert len(entries) == 3 and {m["id"] for m in entries} == {"model-a", "model-b", "model-c"}, "Unexpected demo model manifest"
for model in entries:
    assert model["file"] == model["id"] + ".png" and "AI-generated fictional person" in model["note"], "Only the documented fictional models can be prepared"
sources += [(models / name, args.output / "models" / name) for name in ["models.json"] + [m["file"] for m in entries]]

# Validate every source and destination before making any changes.
for source, target in sources:
    assert source.is_file() and not source.is_symlink(), "A repository demo input is missing or is a symlink"
    assert not target.is_symlink(), "A destination is a symlink; no files were changed"
    if target.exists():
        assert target.is_file() and comparable_bytes(source) == comparable_bytes(target), "An existing demo input differs; no files were changed"

missing = [(source, target) for source, target in sources if not target.exists()]
if not args.check:
    for source, target in missing:
        target.parent.mkdir(parents=True, exist_ok=True)
        shutil.copyfile(source, target)
        target.chmod(0o644)
print(f"{'Validated' if args.check else 'Prepared'} {len(sources)} demo files: {len(missing)} {'to copy' if args.check else 'copied'}, {len(sources) - len(missing)} already matching. No API requests.")
