# Rendering and Kitty graphics

Terminal parsing is left entirely to [libghostty](https://github.com/ghostty-org/ghostty), through its [Go binding](https://go.mitchellh.com/libghostty). sigmamax reads the screen state from it and draws each frame in Go, in separate passes for backgrounds, text, and Kitty images. The geometry and layering follow [Ghostling](https://github.com/ghostty-org/ghostling).

## Fonts and text

The built-in font is **Go Mono** in regular, bold, italic, and bold italic, from `golang.org/x/image/font/gofont`. Its license is in [FONT-LICENSE](../FONT-LICENSE). `Set FontFamily` selects an installed font or a `.ttf`/`.otf` file instead.

Glyphs are parsed with `sfnt` and drawn by sigmamax's own `face` (`internal/render/face.go`). It is the `opentype` face from `golang.org/x/image` with one change, so that frames come out identical on arm64 and amd64; see [Deterministic rendering](development.md#deterministic-rendering). A bounded cache holds each rendered glyph per `(rune, foreground, bold, italic)`.

Limits:

- There is no ligature support, text shaping, font fallback, or color emoji. Combining marks are drawn at the cell's origin.
- Without fallback, a character the font lacks is drawn as a replacement mark. Go Mono has no braille patterns, rounded box corners (`╭`), or CJK text. Choose a font that covers what your program prints.
- Text underlines, in every style, are drawn as a simple line.
- The cursor can be a block, bar, or underline, and can blink.

## Kitty graphics

These parts of the protocol are supported:

- Direct data transfer (`t=d`) of PNG (`f=100`), RGB (`f=24`), and RGBA (`f=32`) images.
- Transmit and display (`a=T`), transmit then place (`a=t`, `a=p`), and delete.
- Images at their native pixel size or fitted to a grid, with cropping, sub-cell offsets, alpha, and clipping to the viewport. Placements scroll with the text.
- Z ordering: below cell backgrounds, below text, or above text.
- Unicode placeholders (`U=1`). Image and placement IDs are read from the cell colors, row and column diacritics are honored, and omitted coordinates are inherited from the previous cell. Each image fits its virtual grid with its aspect ratio kept and is clipped to the cells that name it. Erasing or scrolling those cells updates the image.
- Replies to device attribute, size, and Kitty graphics (`a=q`) queries, written back to the program.

The terminal gives Kitty images 64 MiB of storage. Image pixels and placement geometry are copied out of libghostty after every write, before the terminal changes again, and frames are composited only from those copies. Memory borrowed from C is never kept across writes.

Not supported: animation frames, relative placements, and shared-memory, regular-file, and temporary-file transfer. The file and shared-memory transports are turned off. libghostty may understand more of the protocol than this, but only what is listed is part of what sigmamax promises. Repeated PNG transfers do animate; what is missing is Kitty's separate animation-frame protocol.

### How programs detect it

Recorded programs run with `TERM=xterm-ghostty`, the name most programs check to learn that a terminal draws Kitty graphics. The matching terminfo entry is generated from the pinned Ghostty revision (`task terminfo`) and embedded in the binary. For the length of a recording it is installed in a private directory and exposed through `TERMINFO`, so terminfo readers work on machines without Ghostty. A tape's `Env TERM ...` overrides it, and `TERM_PROGRAM` is `sigmamax`.

`NO_COLOR` must be unset. Placeholder images keep their image ID in the foreground color of the cell, so a program that turns colors off cannot show them.

### Recording ntcharts

For the ntcharts shader example, ask for direct transfer. ntcharts recognizes the terminal with no further settings:

```sh
cd /path/to/ntcharts/examples/shaders
env -u NO_COLOR \
  /path/to/sigmamax/bin/sigmamax record -o /tmp/shaders.gif -cols 80 -rows 24 -- \
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

The pinned libghostty removes an image as soon as a chunked replacement starts, before the new one is complete. While recording, if displayed placeholder cells lose their image, sigmamax keeps showing the last frame for up to 250 ms and waits for the replacement. Clearing the placeholder cells ends the wait at once.

libghostty does not report uploads in progress, so deleting an image on purpose while its placeholder cells stay on screen gets the same short delay. An abandoned upload can never hold the recording indefinitely, and a screenshot always shows the terminal's current state.

Synchronized output (mode 2026) is honored: a half-drawn frame is kept out of the recording, with a one-second timeout in case the program never finishes.
