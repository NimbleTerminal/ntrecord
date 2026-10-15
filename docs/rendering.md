# Rendering and Kitty graphics

Terminal parsing is left entirely to [libghostty](https://github.com/ghostty-org/ghostty), through its [Go binding](https://go.mitchellh.com/libghostty). ntrecord reads the screen state from it and draws each frame in Go, in separate passes for backgrounds, text, and Kitty images. The geometry and layering follow [Ghostling](https://github.com/ghostty-org/ghostling).

## Fonts and text

The built-in text font is **Go Mono** in regular, bold, italic, and bold italic.
**Noto Sans Symbols 2** and **Noto Color Emoji** are bundled as fallbacks, so emoji,
box drawing, and braille need no font installation. Go Mono's license is in
[FONT-LICENSE](../FONT-LICENSE); Noto licenses, pinned sources, and hashes are in
the [font manifest](../render/fonts/README.md) and release archives.

`Set FontFamily` (or `-font`) selects an installed family or a `.ttf`, `.otf`, or
`.ttc` file. A collection path selects its first face. Missing text falls back to
Noto Symbols, then installed fonts covering the entire grapheme cluster, preferring
matching weight and italic style. CJK and other scripts need a suitable installed
outline font. Missing clusters use a replacement mark. Installed font coverage
can differ between machines.

System fallback searches the standard macOS/Linux font directories, including
user font directories and `$XDG_DATA_HOME/fonts`. `NTRECORD_FONT_PATH` replaces
those **fallback** roots with a platform-separated list of directories or font
files. Set it to an empty string for reproducible bundled-only rendering. It does
not change named primary-font lookup.

Box drawing (U+2500–257F), block elements (U+2580–259F), and braille (U+2800–28FF)
are drawn geometrically at the actual cell size, including letter and line
spacing. Borders reach adjacent cells; full blocks have no font-bearing gaps.
Light/heavy/double lines, mixed junctions, dashes, rounded corners, diagonal lines,
quadrants, shades, and all eight braille dots are supported. Very small cells
necessarily reduce line and dot detail.

The renderer shapes each terminal grapheme cluster with the pure-Go
`go-text/typesetting` OpenType shaper and normalizes text to NFC. Combining marks,
emoji skin tones, ZWJ sequences, flags, keycaps, and variation selectors are kept
together. Ghostty controls the grid width; shaped text cannot advance neighboring
cells. Grapheme-cluster mode (2027) defaults on. New `.cast` recordings begin with
its enable sequence for compatible viewers; applications can explicitly disable
it with `CSI ? 2027 l`. Older casts replay with this default too, unless they
specify their width mode, so legacy emoji spacing may change.

Emoji presentation uses the bundled color font; VS15 requests monochrome text
and VS16 requests emoji. The color bitmap keeps its aspect ratio inside its
terminal cells. Unsupported emoji sequences fall back to a replacement rather
than drawing overlapping component images. PNG and video preserve these colors;
GIF output still has the palette tradeoffs described in [GIF encoding](gif-encoding.md).

Ordinary single-cell primary-font glyphs retain the existing deterministic
rasterizer. Cluster and glyph image caches each retain at most 32 MiB of pixel
data, with entry limits of 4,096 and 8,192 respectively. Shaping is limited to 256
runes per cluster. System fallback discovery considers at most 1,024 files and
2,048 faces, with 32 MiB of accepted cmap source tables; individual font files
are limited to 64 MiB. At most 16 system faces are attempted per renderer, with
128 MiB of cumulative source bytes. Parsed font tables, temporary masks, and the
embedded fonts consume additional memory: these limits are not a total RSS cap.
Fonts are discovered and loaded lazily, then reused.

Remaining limits:

- Shaping stays within each terminal cluster. There are no cross-cell programming
  ligatures, paragraph bidi layout, or contextual shaping across separate cells.
- Installed fallback supports outline fonts. Color rendering uses the bundled
  Noto PNG bitmap font; arbitrary COLR/SVG fonts and platform emoji APIs are not
  supported. Emoji coverage follows the pinned font version.
- Text underlines, in every style, are drawn as a simple line.
- The cursor can be a block, bar, or underline, and can blink.

Try the bundled Unicode showcase from the repository root:

```sh
task build
./ntrecord examples/unicode.tape
./ntrecord play examples/unicode.cast
```

This produces GIF, PNG, and cast outputs showing accents, symbols, cell graphics,
and color emoji. The terminal player uses the same renderer and requires Kitty
image support in the host terminal.

## Kitty graphics

These parts of the protocol are supported:

- Direct data transfer (`t=d`) of PNG (`f=100`), RGB (`f=24`), and RGBA (`f=32`) images.
- Transmit and display (`a=T`), transmit then place (`a=t`, `a=p`), and delete.
- Images at their native pixel size or fitted to a grid, with cropping, sub-cell offsets, alpha, and clipping to the viewport. Placements scroll with the text.
- Z ordering: below cell backgrounds, below text, or above text.
- Unicode placeholders (`U=1`). Image and placement IDs are read from the cell colors, row and column diacritics are honored, and omitted coordinates are inherited from the previous cell. Each image fits its virtual grid with its aspect ratio kept and is clipped to the cells that name it. Erasing or scrolling those cells updates the image.
- Replies to device attribute, size, and Kitty graphics (`a=q`) queries, written back to the program.

The terminal gives Kitty images 64 MiB of storage. Image pixels and placement geometry are copied out of libghostty after every write, before the terminal changes again, and frames are composited only from those copies. Memory borrowed from C is never kept across writes.

Not supported: animation frames, relative placements, and shared-memory, regular-file, and temporary-file transfer. The file and shared-memory transports are turned off. libghostty may understand more of the protocol than this, but only what is listed is part of what ntrecord promises. Repeated PNG transfers do animate; what is missing is Kitty's separate animation-frame protocol.

### How programs detect it

Recorded programs run with `TERM=xterm-ghostty`, the name most programs check to learn that a terminal draws Kitty graphics. The matching terminfo entry is generated from the pinned Ghostty revision (`task terminfo`) and embedded in the binary. For the length of a recording it is installed in a private directory and exposed through `TERMINFO`, so terminfo readers work on machines without Ghostty. A tape's `Env TERM ...` overrides it, and `TERM_PROGRAM` is `ntrecord`.

`NO_COLOR` must be unset. Placeholder images keep their image ID in the foreground color of the cell, so a program that turns colors off cannot show them.

### Recording ntcharts

For the ntcharts shader example, ask for direct transfer. ntcharts recognizes the terminal with no further settings:

```sh
cd /path/to/ntcharts/examples/shaders
env -u NO_COLOR \
  /path/to/ntrecord/ntrecord record -o /tmp/shaders.gif -cols 80 -rows 24 -- \
  go run . -medium=direct -fullscreen -fps=20 -duration=8s
```

Use `/tmp/shaders.mp4` or `/tmp/shaders.webm` to record video the same way. `task demo:placeholder` draws the same kind of Unicode placement without ntcharts or a GPU.

For a repeatable eight-second shader recording that also checks for blank frames between images, point this task at your compiled ntcharts shader example:

```sh
task test:shaders SHADERS_BIN=/path/to/ntcharts/bin/ntcharts-shaders
# Optional: OUTPUT=/tmp/shaders.gif (default: examples/shaders.gif)
```

This test needs a hardware GPU, so it is opt-in. The normal test suite uses a small PNG client that pauses between upload chunks and checks GIF, MP4, and WebM for continuity without a GPU. A shader's startup and shutdown may legitimately show the plain background, so the check rejects only blank frames in the middle.

### Flicker while an image is replaced

The pinned libghostty removes an image as soon as a chunked replacement starts, before the new one is complete. While recording, if displayed placeholder cells lose their image, ntrecord keeps showing the last frame for up to 250 ms and waits for the replacement. Clearing the placeholder cells ends the wait at once.

libghostty does not report uploads in progress, so deleting an image on purpose while its placeholder cells stay on screen gets the same short delay. An abandoned upload can never hold the recording indefinitely, and a screenshot always shows the terminal's current state.

Synchronized output (mode 2026) is honored: a half-drawn frame is kept out of the recording, with a one-second timeout in case the program never finishes.
