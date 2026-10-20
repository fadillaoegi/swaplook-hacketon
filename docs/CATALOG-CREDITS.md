# Catalog photo credits

SwapLook's demo catalog uses openly licensed photographs from Wikimedia Commons. Product names, prices, colors, materials and stock are fictional demo data and do not describe the original items or their makers.

**Changes made:** cropped, background normalized or padded, resized to 1024×1024 JPEG (`swaplook_FE/public/catalog/SL-*.jpg`, also used as the private YouCam garment references in `.data/garments/`). Transparent cut-outs (`hero-SL-001.png`, `cut-SL-003.png`, `cut-SL-005.png`, `cut-SL-009.png`) are derived from the same sources. Adapted files are shared under the same license as their source; CC BY-SA adaptations remain CC BY-SA.

| SKU | Demo name | Source file | Author / source | License | Source page |
|---|---|---|---|---|---|
| SL-001 | Oxford Blue Shirt | Blue Business Shirt.jpg | ChristianGlaeser (own work) | CC BY-SA 3.0 | https://commons.wikimedia.org/wiki/File:Blue_Business_Shirt.jpg |
| SL-002 | Harbor Stripe Popover | Fischerhemd 002.jpg | Peng | CC BY-SA 3.0 | https://commons.wikimedia.org/wiki/File:Fischerhemd_002.jpg |
| SL-003 | Graphite Check Shirt | 와이셔츠 01 02.jpg | 국립국어원 | CC BY-SA 2.0 kr | https://commons.wikimedia.org/wiki/File:%EC%99%80%EC%9D%B4%EC%85%94%EC%B8%A0_01_02.jpg |
| SL-004 | Midnight Flannel Shirt | Flanell 1.jpg | Ryj | CC BY-SA 3.0 | https://commons.wikimedia.org/wiki/File:Flanell_1.jpg |
| SL-005 | Picnic Check Camp Shirt | Blus för kvinna av diagonalkypertvävd bomull, rutig i blått, rött, vitt och grått - Nordiska museet - NM.0305557 (1).jpg | Elisabeth Eriksson | CC BY 4.0 | https://commons.wikimedia.org/wiki/File:Blus_f%C3%B6r_kvinna_av_diagonalkypertv%C3%A4vd_bomull,_rutig_i_bl%C3%A5tt,_r%C3%B6tt,_vitt_och_gr%C3%A5tt_-_Nordiska_museet_-_NM.0305557_(1).jpg |
| SL-006 | Bubble Logo Tee | Banfu.jpg | Guangxqs | CC BY-SA 4.0 | https://commons.wikimedia.org/wiki/File:Banfu.jpg |
| SL-007 | Cloud Outline Tee | Banfu5.jpg | Guangxqs | CC BY-SA 4.0 | https://commons.wikimedia.org/wiki/File:Banfu5.jpg |
| SL-008 | Number One Tee | Banfu3.jpg | Guangxqs | CC BY-SA 4.0 | https://commons.wikimedia.org/wiki/File:Banfu3.jpg |
| SL-009 | Pitch Raglan Polo | Erie Raptors Jersey.jpeg | Erie Rugby Club | CC BY-SA 4.0 | https://commons.wikimedia.org/wiki/File:Erie_Raptors_Jersey.jpeg |
| SL-010 | Sailor Middy Top | Middy blouse from Jack Tar Togs (7680582534).jpg | Center for Jewish History, NYC | No restrictions | https://commons.wikimedia.org/wiki/File:Middy_blouse_from_Jack_Tar_Togs_(7680582534).jpg |
| SL-011 | Matchday Stripe Jersey | Maglia as accettura 07-11.jpg | Marchi.leo | Public domain | https://commons.wikimedia.org/wiki/File:Maglia_as_accettura_07-11.jpg |
| SL-012 | Ivory French Cuff Shirt | Camisade puño doble.jpg | Incal | CC BY-SA 3.0 | https://commons.wikimedia.org/wiki/File:Camisade_pu%C3%B1o_doble.jpg |

Selection rules: single garment, front-facing, no people, no character prints. A poster-print tee candidate was rejected for possible third-party artwork. Small maker/club marks remain on some garments (SL-009, SL-011) and are not endorsements.

These licenses cover the photographs. They do not grant trademark rights in any visible marks.

## Model gallery

Models A–C are **AI-generated fictional people**, created on 19 October 2026 with the YouCam AI Image Generator (`youcam-image-v2`, text-to-image, 1104×1472) by `swaplook_BE/cmd/modelgen` (`make models-generate`). The exact prompts are in that file. They do not depict real individuals; they are labelled as AI-generated in the app. No photo of a real person is used in the public model gallery.

The three gallery inputs and their provenance manifest are included in `demo-assets/models` for reproducible setup. Run `make prepare-demo-assets` to reuse them without a new generation. Private user uploads, provider receipts and try-on results remain excluded from Git.
