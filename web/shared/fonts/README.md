# Fonts

Self-hosted, subset, `font-display: swap`. All three are SIL OFL 1.1; the
licences sit next to the files.

| File | Source (github.com/google/fonts, `main`) | Cut |
|---|---|---|
| `archivo.woff2` | `ofl/archivo/Archivo[wdth,wght].ttf` | `wdth` pinned to 100, `wght` 400 to 700 |
| `jetbrains-mono.woff2` | `ofl/jetbrainsmono/JetBrainsMono[wght].ttf` | `wght` 400 to 700 |
| `instrument-serif.woff2` | `ofl/instrumentserif/InstrumentSerif-Regular.ttf` | static |
| `instrument-serif-italic.woff2` | `ofl/instrumentserif/InstrumentSerif-Italic.ttf` | static |
| `rupee.woff2` | Archivo, as above | U+20B9 only |

The design uses weights 500, 600 and 700, and one 800 that snaps to 700.

## Rebuild

Needs Python with `fonttools` and `brotli` (`pip install fonttools brotli`).

```sh
python -m fontTools.varLib.instancer "Archivo[wdth,wght].ttf" wdth=100 wght=400:700 -o archivo-inst.ttf
python -m fontTools.varLib.instancer "JetBrainsMono[wght].ttf" wght=400:700 -o jbm-inst.ttf

U="U+0020-007E,U+00A0-00FF,U+0131,U+0152-0153,U+02C6,U+02DA,U+02DC,U+2009,U+2013-2014,U+2018-201A,U+201C-201E,U+2022,U+2026,U+2030,U+2039-203A,U+20AC,U+2122,U+2190-2193,U+2212,U+2215,U+FEFF,U+FFFD"
F="--layout-features+=tnum,lnum,zero,case --flavor=woff2 --no-hinting --desubroutinize"

pyftsubset archivo-inst.ttf --unicodes="$U,U+20B9" $F --output-file=archivo.woff2
pyftsubset jbm-inst.ttf --unicodes="$U" $F --output-file=jetbrains-mono.woff2
pyftsubset InstrumentSerif-Regular.ttf --unicodes="$U" $F --output-file=instrument-serif.woff2
pyftsubset InstrumentSerif-Italic.ttf --unicodes="$U" $F --output-file=instrument-serif-italic.woff2
pyftsubset archivo-inst.ttf --unicodes="U+20B9" --flavor=woff2 --no-hinting --output-file=rupee.woff2
```

`tnum` is kept explicitly: pyftsubset drops it by default, and every number in
the product depends on it.
