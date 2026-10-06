# sigmamax

Record a terminal session to GIF, MP4, or WebM, including Kitty graphics. sigmamax is a new
Go module (`github.com/NimbleMarkets/sigmamax`), licensed under MIT. It runs a
real PTY, feeds its bytes into libghostty-vt, and composites pixels in Go.
There is no browser backend, ttyd, xterm.js, headless Chrome, or ghostty-web.
It is not a fork of VHS or a patch to EVP.

## Build

Requires Go 1.26+, [Task](https://taskfile.dev/), a C compiler/SDK, `make`,
`curl`, `tar` with xz support, and `shasum` or `sha256sum`. Supports macOS and
Linux on amd64/arm64. On macOS, install Command Line Tools with
`xcode-select --install` if needed, and Go/Task with `brew install go go-task`.
On Debian/Ubuntu, native prerequisites are
`sudo apt-get install build-essential curl xz-utils ca-certificates`; install
Go and Task separately.

**You do not need Zig or pkg-config installed globally.** Start with:

```sh
task build
task ci
task run -- examples/kitty.tape
task demo:text
```

The first build downloads SHA-256-verified **Zig 0.16.0** and **pkgconf 2.5.1**,
builds pkgconf and the pinned libghostty-vt, and writes `bin/sigmamax`. Everything
native is stored under ignored `.deps/`; no sudo, Homebrew Zig version, shell
profile changes, or `/tmp` installation is required. The first build needs
network access and can take a few minutes. Later builds reuse these tools.
`task deps` explicitly runs native setup (and forwards Zig build flags after
`--`); ordinary build/test/install tasks run setup when the library is missing.

Task sets `CGO_ENABLED=1`, uses `scripts/pkg-config.sh` to find the local
pkgconf, and adds the local native library to `PKG_CONFIG_PATH`, preserving
any existing search path. Without Task:

```sh
sh scripts/build-libghostty.sh
export PKG_CONFIG="$PWD/scripts/pkg-config.sh"
export PKG_CONFIG_PATH="$PWD/.deps/libghostty-vt/share/pkgconfig${PKG_CONFIG_PATH:+:$PKG_CONFIG_PATH}"
"$PKG_CONFIG" --static --cflags --libs libghostty-vt-static
CGO_ENABLED=1 go build -o bin/sigmamax .
go run . examples/kitty.tape
go run . examples/text.tape
```

Native setup downloads and builds Ghostty commit
`0e0ff282f72c92d930d41f4703547d3bc9618ba2` locally under `.deps/`. The Go binding
is pinned in go.mod to commit `183d2bc56a8a` (pseudo-version
`v0.0.0-20261004135024-183d2bc56a8a`). Its API is unstable; update these together.
An existing matching installation can be used by setting `PKG_CONFIG_PATH` to
its `share/pkgconfig` directory and `PKG_CONFIG` to your pkg-config executable.
`ZIG=/absolute/path/to/zig` overrides the downloaded compiler; overrides must
report version 0.16.0. Tool versions, source revision, and archive checksums are
pinned in `scripts/native-common.sh` and `scripts/bootstrap-tools.sh`.

```sh
task clean       # remove bin/ and .task/; keep dependencies and recordings
task build       # reuse the local native dependencies
task clean:all   # also remove all of .deps/ (downloads, tools, libraries, caches)
task build       # reproduce the first build from scratch
```

Clean tasks only affect this checkout. They never uninstall a Go-installed
binary, delete recordings, or remove an external `GHOSTTY_PREFIX`.

The unmodified `go.mitchellh.com/libghostty` binding uses **cgo and static
libghostty-vt linking** by default, through `libghostty-vt-static.pc`. There is
no purego rebinding. The resulting binary needs no Ghostty shared library,
external font file, or GIF encoder. GIFs use Go's encoder and a dithered palette;
image colors may be quantized.

For a Linux amd64 cross-build from a supported Zig host:

```sh
GHOSTTY_PREFIX="$PWD/.deps/ghostty-linux-amd64" \
  sh scripts/build-libghostty.sh -Dtarget=x86_64-linux-gnu
PKG_CONFIG_PATH="$PWD/.deps/ghostty-linux-amd64/share/pkgconfig" \
  PKG_CONFIG="$PWD/scripts/pkg-config.sh" \
  CGO_ENABLED=1 GOOS=linux GOARCH=amd64 \
  CC="$PWD/.deps/tools/bin/zig cc -target x86_64-linux-gnu" \
  CXX="$PWD/.deps/tools/bin/zig c++ -target x86_64-linux-gnu" \
  go build -o bin/sigmamax-linux-amd64 .
```

## Output formats

The filename extension selects the encoder in both tape `Output` and live
`record -o`. Unsupported extensions are rejected before launching the PTY.

| Extension | Encoder | Runtime requirement |
| --- | --- | --- |
| `.gif` | Go GIF encoder, dithered 256-color palette | None beyond sigmamax |
| `.mp4` | H.264 (`libx264`), CRF 18, fast-start MP4 | FFmpeg on `PATH` |
| `.webm` | VP9 (`libvpx-vp9`), CRF 30 | FFmpeg on `PATH` |

Install FFmpeg with `brew install ffmpeg` on macOS or
`sudo apt-get install ffmpeg` on Debian/Ubuntu. The binary must include the
selected encoder; sigmamax checks this before starting the session. FFmpeg is
optional and is not linked into sigmamax or required to build it or record GIFs.

```sh
task record -- -o session.mp4 -cols 100 -rows 30
task record -- -o session.webm -- sh -c 'printf "hello\n"; sleep 2'
task run -- examples/kitty-mp4.tape
task demo:video  # Kitty MP4 and WebM examples
```

Video uses the same Go compositor as GIF. Completed RGBA frames are piped
directly into FFmpeg's [raw-video input](https://ffmpeg.org/ffmpeg-formats.html#rawvideo),
with no intermediate images or GIF conversion. Output is silent, lossy video
using `yuv420p` for playback compatibility; it avoids GIF palette quantization,
though chroma subsampling can soften small colored text. Odd viewport dimensions
are padded by one pixel on the right/bottom as needed, without resizing text.
Hidden time is omitted. Video duration is rounded to the nearest 50 ms.

Output is written to a temporary file alongside the destination and renamed
only after successful encoding. Cancellation finalizes captured frames before
returning the cancellation error. Encoder failures leave any previous output
intact and include FFmpeg's diagnostics. Slow encoding can reduce the rate at
which the PTY is sampled; elapsed visible time is retained in the video.

## Tapes

Run tapes from the project directory. Command working directories and output
paths are relative to the caller's working directory, not the tape's location.
Existing output files are replaced atomically after successful encoding.

```text
Output demo.gif
Set Shell /bin/sh
Set FontSize 18
Set Cols 80
Set Rows 24
Type `printf 'Hello, terminal!\n'`
Enter
Sleep 2s
```

Supported commands:

| Command | Meaning |
| --- | --- |
| `Output path.gif` | Destination ending in `.gif`, `.mp4`, or `.webm`; default `recording.gif` |
| `Set Shell /bin/sh` | Shell executable, started with `-i`; no embedded arguments |
| `Set FontSize 18` | Font size in pixels, 6–96 |
| `Set Cols 80`, `Set Rows 24` | PTY grid size (`Columns` also works) |
| `Set Width 800`, `Set Height 480` | Pixel size, rounded down to whole cells; overrides that grid dimension |
| `Type "text"` | Type Unicode text with a 25 ms interval per rune |
| `Enter` | Send carriage return, then wait 100 ms |
| `Sleep 500ms` | Wait for a Go duration (`ms`, `s`, `m`, etc.) |
| `Hide`, `Show` | Pause/resume capture while the PTY continues running |

`Output` and `Set` must precede actions. Unknown commands/settings and malformed
arguments are errors with line numbers. Double-quoted strings accept Go escapes;
single-quoted and backtick strings are literal. `#` starts a comment outside
quotes. This is a small VHS-style subset, not full VHS compatibility.

Capture samples at 20 FPS. GIF merges identical adjacent bitmaps, preserving
their duration; video repeats held frames on its 20 FPS timeline and lets the
codec compress duplicates. Hidden time is omitted. Use `Sleep` to allow commands to finish
and to hold the last frame. Tape completion closes the PTY and stops its shell.
The Kitty demo runs a small Go program that first queries DA1 and Kitty support,
then emits a direct PNG APC. It waits four seconds for `go run`; increase that
Sleep on a slow machine or prebuild the example.

## Live recording

```sh
task record -- -o session.gif -cols 100 -rows 30
task record -- -o command.gif -- sh -c 'printf "hello\n"; sleep 2'
```

`record` attaches your input/output to a newly spawned PTY running the shell or
command. Exit the shell (or press Ctrl-D) to finish and write the recording. Ctrl-C
is passed to the foreground program when stdin is a terminal. SIGTERM cancels
recording and saves captured frames. The original terminal mode is restored on
exit. It does not attach to an unrelated, already-running process.

The recorded viewport remains fixed at the requested size. Child exit failures
are returned after saving the recording. GIF holds unique paletted frames in
memory with a 512 MiB limit; keep GIF recordings reasonably short. Video streams
frames to FFmpeg as they are captured, without accumulating them in Go memory. Viewports are
limited to 8192 pixels per side and 16 megapixels.

## Rendering and Kitty scope

The renderer uses bundled **Go Mono** regular/bold/italic/bold-italic TTFs from
`golang.org/x/image/font/gofont`. Fonts are parsed by `sfnt` and rasterized with
`opentype`; a bounded atlas caches each `(rune, foreground, bold, italic)` raster.
The font license is in [FONT-LICENSE](FONT-LICENSE). No custom rasterizer,
ligatures, text shaping, font fallback, or color emoji in v1. Combining marks
share the terminal cell origin. Cursor visibility/shape is rendered without
blinking; underline variants use a simple line.

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

For ntcharts' shader example, request direct transport and explicitly enable
Kitty capability (ntcharts' environment detection does not recognize sigmamax):

```sh
cd /path/to/ntcharts/examples/shaders
env -u NO_COLOR NTCHARTS_KITTY=supported NTCHARTS_TMUX_PASSTHROUGH=false \
  /path/to/sigmamax/bin/sigmamax record -o /tmp/shaders.gif -cols 80 -rows 24 -- \
  go run . -medium=direct -fullscreen -fps=20 -duration=8s
```

Change `/tmp/shaders.gif` to `/tmp/shaders.mp4` or `/tmp/shaders.webm` to record
video with the same invocation.

Repeated PNG transmissions animate normally; the deferred feature is Kitty's
separate animation-frame protocol. `task demo:placeholder` exercises the same
Unicode placement scheme without needing ntcharts or a GPU.
`NO_COLOR` must be unset for this scheme: the cell's foreground color carries
the image ID. Synchronized output (mode 2026) holds incomplete grid updates
out of the recording, with a one-second timeout if the application never finishes.

Geometry and layer handling follow the API used by
[Ghostling](https://github.com/ghostty-org/ghostling), with separate Go passes
for backgrounds, text, and Kitty layers. Terminal parsing is entirely delegated
to [libghostty](https://github.com/ghostty-org/ghostty) through the
[Go binding](https://go.mitchellh.com/libghostty).

## Tests

```sh
task test
task ci
task demos
task test:video  # requires ffmpeg and ffprobe; never silently skips video tests
task demo:video
```

`task` defaults to `task build`; `task list` shows available commands. The
workflow also provides `run`, `record`, `install`, `fmt`, `fmt-check`, `go-tidy`,
`test-race`, and `vet`. Arguments after `--` reach the CLI or test command, for
example `task test -- -run TestPNGFixture -v`. Builds use `-mod=readonly` and
Go's cache so native compiler settings and workspace dependencies are checked.
`task ci` runs formatting, module tidy/verification, race tests, vet, and build,
matching GitHub Actions. CI starts without `.deps/`, runs `task build`, `task ci`,
`task demos`, and `task demo:video` on macOS/Linux, then checks `task clean` followed by a rebuild.
It uses the same pinned tool bootstrap as local development. This workflow
installs FFmpeg, requires video tests with `SIGMAMAX_REQUIRE_FFMPEG=1`, and uploads
demo GIFs/MP4s/WebMs; release publishing is not configured. The Makefile
delegates to Task.

Set `GHOSTTY_PREFIX=/absolute/path` to use another native install prefix, or
set `PKG_CONFIG_PATH` to its pkg-config directory. `PKG_CONFIG` and `ZIG` can
select tool executables. For example, `task deps -- -Dtarget=x86_64-linux-gnu`
forwards a target to the native build (use a separate `GHOSTTY_PREFIX` when
cross-compiling, as shown above).

Tests feed a base64 PNG APC fixture into `VTWrite` and assert pixels inside its
placement, then check raw formats, replies, cropping, offsets, alpha, z-index,
scrolling, deletion, and copied-buffer lifetime. Integration tests build the
Kitty demo, run it through both a tape and live command recording, decode the
GIFs and videos, and assert image pixels for ordinary and Unicode placements. A text-only tape asserts colored glyphs, and
exit/cancellation tests verify that recordings are finalized. Video tests also
check codec, dimensions, padding, color, timing, failed-encode cleanup, and
preservation of existing outputs. Locally, tests needing FFmpeg/ffprobe skip if
the tools are missing; `task test:video` and CI require them. CI runs on Linux/macOS.
