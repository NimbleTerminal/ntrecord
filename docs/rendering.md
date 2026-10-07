# Rendering and Kitty scope

The renderer uses bundled **Go Mono** regular/bold/italic/bold-italic TTFs from
`golang.org/x/image/font/gofont`. Additional installed fonts or explicit TTF/OTF paths can be selected with
`Set FontFamily`. Fonts are parsed by `sfnt` and rasterized with
`opentype`; a bounded atlas caches each `(rune, foreground, bold, italic)` raster.
The font license is in [FONT-LICENSE](../FONT-LICENSE). No custom rasterizer,
ligatures, text shaping, font fallback, or color emoji in v1. Combining marks
share the terminal cell origin. Cursor visibility/shape is rendered with optional blinking; underline variants use a simple line.

Supported Kitty slice:

- Direct data transport (`t=d`): PNG (`f=100`), RGB (`f=24`), RGBA (`f=32`).
- Transmit-and-display (`a=T`), transmit then place (`a=t`, `a=p`), and delete.
- Native pixel sizing, grid sizing, source crop, sub-cell offsets, alpha, and
  viewport clipping. Placements scroll with libghostty's grid anchors.
- Z ordering: below explicit cell backgrounds, below text, and above text.
- Unicode placeholders (`U=1`), including image/placement IDs encoded in cell
  colors, row/column diacritics and omitted-coordinate inheritance. Images fit
  their virtual grid with preserved aspect ratio and are clipped to the cells
  that reference them. Erasing and scrolling those cells updates the image.
- Primary device attributes, size queries, and Kitty graphics queries (`a=q`)
  are answered through the binding's effect callbacks and written to the PTY.

The terminal enables 64 MiB of Kitty image storage and installs the binding's
Go PNG decoder. After every `VTWrite`, image pixels and placement geometry are
copied before any further terminal mutation. Frame compositing only uses those
owned copies; borrowed C slices are never retained across writes.

Animation frames, relative placements, shared-memory transport, regular-file
transport, and temporary-file transport are deferred. The file/shared-memory
transports remain disabled. libghostty may parse a broader protocol, but these
features are not part of sigmamax's v1 rendering contract.

Recorded programs run with `TERM=xterm-ghostty`, the identity programs check to
learn that a terminal draws Kitty graphics. The matching terminfo entry, generated
from the pinned Ghostty revision (`task terminfo`), is embedded in the binary and
made available to the program through `TERMINFO` for the length of the recording,
so terminfo readers work on hosts without Ghostty. A tape's `Env TERM ...` still
overrides it. `TERM_PROGRAM` is `sigmamax`.

For ntcharts' shader example, request direct transport; ntcharts recognizes the
terminal without any override:

```sh
cd /path/to/ntcharts/examples/shaders
env -u NO_COLOR \
  /path/to/sigmamax/bin/sigmamax record -o /tmp/shaders.gif -cols 80 -rows 24 -- \
  go run . -medium=direct -fullscreen -fps=20 -duration=8s
```

Change `/tmp/shaders.gif` to `/tmp/shaders.mp4` or `/tmp/shaders.webm` to record
video with the same invocation.

Repeated PNG transmissions animate normally; the deferred feature is Kitty's
separate animation-frame protocol. `task demo:placeholder` exercises the same
Unicode placement scheme without needing ntcharts or a GPU.
For a repeatable eight-second shader recording with a check for blank frames
between image frames, point this task at your compiled ntcharts shader example:

```sh
task test:shaders SHADERS_BIN=/path/to/ntcharts/bin/ntcharts-shaders
# Optional: OUTPUT=/tmp/shaders.gif (default: examples/shaders.gif)
```

This opt-in test needs a hardware GPU. The normal test suite uses a small PNG
client that deliberately pauses between upload chunks and checks GIF, MP4, and
WebM frame continuity without a GPU. Shader startup and shutdown may legitimately
show the terminal background; the shader check only rejects interior blank frames.

The pinned libghostty removes an existing image when a chunked replacement
starts, before the replacement is complete. During recording, if displayed
Unicode placeholders lose their image, sigmamax holds the last presented bitmap
for up to 250ms while waiting for a replacement. Clearing the placeholder cells
ends this hold immediately. The binding does not expose uploads in progress, so
an explicit image deletion that leaves its placeholder cells on screen has the
same bounded delay. Abandoned uploads cannot hold capture indefinitely; immediate
screenshots still reflect the terminal's current state.

`NO_COLOR` must be unset for this scheme: the cell's foreground color carries
the image ID. Synchronized output (mode 2026) holds incomplete grid updates
out of the recording, with a one-second timeout if the application never finishes.

Geometry and layer handling follow the API used by
[Ghostling](https://github.com/ghostty-org/ghostling), with separate Go passes
for backgrounds, text, and Kitty layers. Terminal parsing is entirely delegated
to [libghostty](https://github.com/ghostty-org/ghostty) through the
[Go binding](https://go.mitchellh.com/libghostty).
