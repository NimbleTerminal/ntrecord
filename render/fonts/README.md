# Bundled fallback fonts

These unmodified fonts are embedded in the binary. Both use the SIL Open Font
License 1.1; their upstream notices are alongside the files and included in
release archives. No installed system fonts are redistributed.

| File | Upstream revision and path | SHA-256 |
| --- | --- | --- |
| NotoColorEmoji.ttf | [googlefonts/noto-emoji](https://github.com/googlefonts/noto-emoji/tree/f3ae03f5e9b3b8516fa151f7168159ca1a3e7515/fonts), `fonts/NotoColorEmoji.ttf` (v2.051) | `72a635cb3d2f3524c51620cdde406b217204e8a6a06c6a096ff8ed4b5fd6e27b` |
| NotoSansSymbols2-Regular.ttf | [google/fonts](https://github.com/google/fonts/tree/5e8a3ba899557829a76cfdac30fa512bda91d7ca/ofl/notosanssymbols2), `ofl/notosanssymbols2/NotoSansSymbols2-Regular.ttf` | `7d5fb73b7ca67a6798101741f5d280a3d016a56a197afcd4199dbb57b4b82a21` |

To reproduce, download the raw files from these exact revisions. The emoji notice
comes from `fonts/LICENSE`; the symbols notice from `ofl/notosanssymbols2/OFL.txt`.
Verify the SHA-256 hashes before updating. Font changes require review of the
renderer golden PNGs, including emoji sequences and fractional font sizes.

The emoji font uses CBDT/CBLC PNG bitmap strikes, scaled to terminal cells after
OpenType shaping. It adds about 10 MiB to the binary; Symbols 2 adds about 1.2 MiB.
Coverage follows these pinned fonts, not every future Unicode release.

The shaper and OpenType parser are `github.com/go-text/typesetting v0.3.5`, a
pure-Go implementation used by Go GUI projects. It is dual-licensed under
Unlicense/BSD-3-Clause (see the module's license files, collected by the existing
release license script). Its HarfBuzz subpackage carries a separate MIT notice,
also collected for release archives. It adds no external shaping process or native library.
The existing deterministic outline rasterizer remains in use. Neither font
requires a new Go dependency; both are embedded data.
