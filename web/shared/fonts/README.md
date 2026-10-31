# Fonts

Self-hosted, subset, `font-display: swap`. All four are SIL OFL 1.1; the
licences sit next to the files.

| File | Source (github.com/google/fonts, `main`) | Cut |
|---|---|---|
| `fraunces.woff2` | `ofl/fraunces/Fraunces[SOFT,WONK,opsz,wght].ttf` | `wght` 400 to 700, `SOFT` 0, `WONK` 1, `opsz` left variable |
| `dm-sans.woff2` | `ofl/dmsans/DMSans[opsz,wght].ttf` | `wght` 400 to 700, `opsz` left variable |
| `caveat.woff2` | `ofl/caveat/Caveat[wght].ttf` | `wght` 400 to 600, ASCII only |
| `archivo.woff2` | `ofl/archivo/Archivo[wdth,wght].ttf` | `wdth` pinned to 100, `wght` 400 to 700 |
| `fraunces-600.woff2` | `fraunces.woff2` above | The guest's: `wght` 600, `opsz` pinned to 20. Static |
| `dm-sans-14.woff2` | `dm-sans.woff2` above | The guest's: `opsz` pinned to 14, `wght` 400 to 700 kept |

`fonts.css` declares the first four, for the venue's screens; `guest.css`
declares the guest's two cuts and Archivo, and no Caveat (GST-11, TECH-18,
ADR-397). The guest's fonts count against a budget of their own in
`scripts/check-size.mjs`.

Archivo is the numeral face. Fraunces and DM Sans, as Google ships them,
have no `tnum` feature: their digits are proportional, and
`font-variant-numeric: tabular-nums` does nothing on them. Archivo's `tnum`
is kept so prices align in a column (ADR-128).

`SOFT` 0 and `WONK` 1 are what Google Fonts serves when a page asks for
Fraunces by `opsz` and `wght` alone, which is how the design was drawn.

## Rebuild

Needs Python with `fonttools` and `brotli` (`pip install fonttools brotli`).

```sh
python -m fontTools.varLib.instancer "Fraunces[SOFT,WONK,opsz,wght].ttf" wght=400:700 SOFT=0 WONK=1 -o fraunces-inst.ttf
python -m fontTools.varLib.instancer "DMSans[opsz,wght].ttf" wght=400:700 -o dmsans-inst.ttf
python -m fontTools.varLib.instancer "Caveat[wght].ttf" wght=400:600 -o caveat-inst.ttf
python -m fontTools.varLib.instancer "Archivo[wdth,wght].ttf" wdth=100 wght=400:700 -o archivo-inst.ttf

U="U+0020-007E,U+00A0-00FF,U+0131,U+0152-0153,U+02C6,U+02DA,U+02DC,U+2009,U+2013-2014,U+2018-201A,U+201C-201E,U+2022,U+2026,U+2030,U+2039-203A,U+20AC,U+20B9,U+2122,U+2190-2193,U+2212,U+2215,U+FEFF,U+FFFD"
F="--flavor=woff2 --no-hinting --desubroutinize"

pyftsubset fraunces-inst.ttf --unicodes="$U" --layout-features+=case $F --output-file=fraunces.woff2
pyftsubset dmsans-inst.ttf --unicodes="$U" --layout-features+=case $F --output-file=dm-sans.woff2
pyftsubset caveat-inst.ttf --unicodes="U+0020-007E,U+2019" $F --output-file=caveat.woff2
pyftsubset archivo-inst.ttf --unicodes="$U" --layout-features+=tnum,lnum,zero,case $F --output-file=archivo.woff2
```

`tnum` is kept explicitly on Archivo: pyftsubset drops it by default, and
every number in the product depends on it.

The guest's two cuts are made from the files above, with the glyphs and
features they already have:

```sh
python -m fontTools.varLib.instancer fraunces.woff2 wght=600 opsz=20 -o fraunces-600.woff2
python -m fontTools.varLib.instancer dm-sans.woff2 opsz=14 -o dm-sans-14.woff2
```

Twenty is between the guest's dish names (15 and 17px) and its titles (22
and 32px); fourteen is the middle of its text (13 to 17px). Pinning an
axis drops its variation data, which is most of a variable face's weight.
