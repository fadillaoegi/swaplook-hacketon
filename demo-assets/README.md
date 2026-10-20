# Repository demo inputs

`models/` contains only the three AI-generated fictional people used in the public demo gallery. They were generated with YouCam text-to-image on 19 October 2026; prompts are in `swaplook_BE/cmd/modelgen/main.go`. The manifest labels their provenance. No real person's portrait, API receipt, signed URL or try-on result is included.

These files are retained as functional demo inputs, not as proof of a new API request. See [catalog and model credits](../docs/CATALOG-CREDITS.md). Catalog garment photographs remain in `swaplook_FE/public/catalog` under their individual licenses; no duplicate garment collection is needed here.

Repository model PNGs have text/EXIF metadata removed; encoded image data is unchanged. The setup helper preserves existing local model PNGs when only that metadata differs.

Run `make prepare-demo-assets` to copy these inputs and the 12 licensed catalog photos to ignored `.data/models` and `.data/garments`. The command makes no API requests and refuses to overwrite different existing files. Enable the optional model gallery following the root README. Do not copy other `.data` contents into this folder.
