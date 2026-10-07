# sigmamax

Record a terminal session to GIF, MP4, or WebM, including Kitty graphics. sigmamax is a new
Go module (`github.com/NimbleMarkets/sigmamax`), licensed under MIT. It runs a
real PTY, feeds its bytes into libghostty-vt, and composites pixels in Go.
There is no browser backend, ttyd, xterm.js, headless Chrome, or ghostty-web.
It is not a fork of VHS or a patch to EVP.

## Install

Release packaging uses the same Homebrew tap as gloss. Once the first tagged
release is published, install on macOS (13+) or Linux, Intel or ARM, with:

```sh
brew install --cask nimblemarkets/tap/sigmamax
sigmamax --version
```

The release binaries include libghostty-vt and the default font. GIF recording
needs no additional tools. For MP4/WebM, also install `ffmpeg`:

```sh
brew install ffmpeg
```

Release archives and SHA-256 checksums are published to
[GitHub Releases](https://github.com/NimbleMarkets/sigmamax/releases). Linux
binaries use static musl; macOS binaries use the system libraries. Neither
requires Zig, pkg-config, or a separate Ghostty installation at runtime.

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
task clean       # remove bin/, dist/, .task/; keep dependencies and recordings
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
Hidden time is omitted. Video duration is rounded to its configured frame interval (50 ms by default).

Output is written to a temporary file alongside the destination and renamed
only after successful encoding. Cancellation finalizes captured frames before
returning the cancellation error. Encoder failures leave any previous output
intact and include FFmpeg's diagnostics. Slow encoding can reduce the rate at
which the PTY is sampled; elapsed visible time is retained in the video.

## Tapes

sigmamax supports the VHS tape language on its native renderer. The goal is
useful script compatibility; font metrics, antialiasing, decorations, and encoded
pixels follow sigmamax rather than matching VHS screenshots.

Run tapes from the project directory. Shell commands, output paths, font paths,
and `Source` paths resolve from the invocation directory, including nested
sources. Source cycles and excessive nesting are errors. Outputs declared inside
sourced tapes are ignored, following VHS, so a shared tape cannot redirect the
caller's recording. Repeated top-level `Output` commands write every destination
from the same session. Files are replaced individually after successful encoding.

```text
Output demo.gif
Output demo.mp4
Require sh
Env MESSAGE "Hello, terminal!"
Set Shell /bin/sh
Set FontSize 18
Set Columns 80
Set Rows 24
Set Theme "Catppuccin Mocha"
Set TypingSpeed 30ms
Hide
Type `printf '\033[2J\033[H'; printf '%s\n' "$MESSAGE"`
Enter
Wait+Screen /Hello, terminal!/
Show
Sleep 2s
Screenshot demo.png
```

| Command | Behavior |
| --- | --- |
| `Output path` | `.gif`, `.mp4`, `.webm`, `.txt`, `.ascii`, `.test`, or a PNG directory ending in `/`; repeat for multiple outputs |
| `Require program` | Check the recording's `PATH` before starting the PTY |
| `Env KEY "value"` | Set a child environment variable without changing the recorder's environment |
| `Source common.tape` | Include another tape with shared settings and actions |
| `Type "text"`, `Type@50ms "text"` | Type text using the current typing interval or a per-command override |
| `Enter`, `Tab`, `Space`, `Backspace`, `Delete`, `Insert`, `Escape`, `Home`, `End`, `Up`, `Down`, `Left`, `Right`, `PageUp`, `PageDown`, `F1`–`F25` | Send keys through libghostty's mode-aware encoder |
| `Ctrl+C`, `Alt+x`, `Shift+Tab`, `Ctrl+Alt+Shift+R` | Combine modifiers; control chains such as `Ctrl+B+D` also work |
| `Down@100ms 3` | Repeat a key with an optional interval and count |
| `ScrollUp 5`, `ScrollDown@100ms 5` | Scroll the terminal viewport without sending keys to the application |
| `Sleep 0.5`, `Sleep 500ms`, `Sleep 500 ms` | Wait while capture continues; bare numbers are seconds |
| `Wait`, `Wait+Line /regex/`, `Wait+Screen@5s /regex/` | Wait for displayed text, with optional scope and timeout override |
| `Copy "text"`, `Paste` | Use a recording-local clipboard; paste respects bracketed-paste mode |
| `Hide`, `Show` | Pause/resume capture while the PTY continues running; hidden time is omitted |
| `Screenshot path.png` | Save the current composited frame, including Kitty images and decorations |

Single quotes, double quotes, and backticks preserve their contents literally,
including shell backslashes. Choose another delimiter to include a quote. This
matches VHS and replaces sigmamax's original Go-style double-quote escaping.
Regex literals accept escaped slashes, and themes accept multiline JSON objects.
`#` starts a comment outside literals. Multiple commands may share a line.
Errors include the source file, line, and column.

`Wait+Line` reads the cursor row; `Wait+Screen` reads the visible viewport. Both
use Go regular expressions against terminal cells, so erased text or escape
sequences do not produce false matches. Default scope is `Line`, timeout is
15 seconds, and pattern is `[$>#]$`, covering common shell prompts. Use a specific
pattern for reliable application readiness; output echoed while typing can also
match a broad regex. Timeout errors report the pattern and last observed text.

Tape controls are ordered after already-queued PTY output. `Hide` captures the
last visible viewport, including during synchronized output; `Show` resumes from
the current viewport. Neither waits for a running command to finish: use `Wait`
before a visibility change when command completion matters. At the end of a
successful tape, output drains until 100ms of silence, up to one second. Use
`Wait` or `Sleep` for commands that need longer.

| Setting | Values / default |
| --- | --- |
| `Shell` | Executable name or path, started with `-i`; default `/bin/sh` |
| `FontSize`, `FontFamily` | 6–96 pixels, default 18; bundled `Go Mono`, installed family name, or `.ttf`/`.otf` path |
| `Columns` / `Cols`, `Rows` | Grid dimensions, default 80×24 |
| `Width`, `Height` | Total output pixels including decorations; cannot combine with `Columns` or `Rows` on the same axis |
| `LetterSpacing`, `LineHeight` | Additional horizontal pixels and line-height multiplier; defaults 0 and 1 |
| `TypingSpeed` | Duration between keystrokes, default 25ms; can change during playback |
| `WaitTimeout`, `WaitPattern` | Default timeout and `/regex/` for `Wait` |
| `Framerate`, `PlaybackSpeed` | 1–120 FPS (default 20); playback multiplier (default 1) |
| `LoopOffset` | GIF starting frame index (fractional values accepted), or timeline percentage such as `50%` |
| `Theme` | One of 348 bundled VHS theme names, or inline JSON with foreground/background, cursor, and ANSI palette colors |
| `Padding`, `Margin`, `MarginFill` | Pixels around the grid/window and an optional `#RRGGBB` margin color |
| `WindowBar`, `WindowBarSize` | `Colorful`, `ColorfulRight`, `Rings`, `RingsRight`, or `None`; bar height defaults to 30 pixels |
| `BorderRadius`, `CursorBlink` | Corner radius in pixels; cursor blinking boolean (default false) |

Put `Output`, `Env`, `Require`, and settings before actions, except that
`TypingSpeed` may change anywhere. Misplaced settings and unknown commands are
errors rather than silently ignored. `Copy`/`Paste` intentionally use an isolated
clipboard, and do not read or overwrite your desktop clipboard. Font families
are looked up in standard macOS/Linux font directories; missing fonts produce
an actionable error. Missing bold/italic variants use the available regular face.
Theme selection colors are accepted but do not affect recording without a selection.

Capture uses the configured framerate. GIF merges identical adjacent bitmaps;
video repeats held frames on its fixed-rate timeline and lets the codec compress
duplicates. Playback speed changes recorded time without speeding up shell input
or readiness waits. Loop offsets apply to GIF only. PNG sequences are numbered
`000001.png`, `000002.png`, etc.; their destination must be absent or empty so
existing frame collections are never deleted. Text outputs contain changed
viewport checkpoints after visible tape actions and the final viewport, separated
by a rule. Use `Wait` before checkpoints that depend on asynchronous output.
They provide readable golden files, with sigmamax's own checkpoint formatting.

Try `task demo:language` for a sourced, themed tape with clipboard input, keyboard
editing, waits, multiple outputs, and a PNG screenshot. `task demo:kitty` exercises
direct PNG APCs; it allows four seconds for `go run`, which may need increasing
on a slow machine. Tape completion closes the PTY and stops its shell.

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
and output-forwarding errors (including a closed stdout pipe) are returned after
attempting to save every recording output. FFmpeg writes have a 30-second deadline
and are interrupted by cancellation. A failed or interrupted video stream cannot
always be saved; other output formats are finalized independently, and an existing
destination is replaced only after successful encoding. GIF holds unique paletted frames in
memory with a 512 MiB limit; keep GIF recordings reasonably short. Video streams
frames to FFmpeg as they are captured, without accumulating them in Go memory. Viewports are
limited to 8192 pixels per side and 16 megapixels.

## Rendering and Kitty scope

The renderer uses bundled **Go Mono** regular/bold/italic/bold-italic TTFs from
`golang.org/x/image/font/gofont`. Additional installed fonts or explicit TTF/OTF paths can be selected with
`Set FontFamily`. Fonts are parsed by `sfnt` and rasterized with
`opentype`; a bounded atlas caches each `(rune, foreground, bold, italic)` raster.
The font license is in [FONT-LICENSE](FONT-LICENSE). No custom rasterizer,
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

## Tests

```sh
task test
task ci
task demos
task test:video  # requires ffmpeg and ffprobe; never silently skips video tests
task demo:video
task demo:language
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
demo GIFs/MP4s/WebMs plus the language screenshot/text. The Makefile delegates to Task.

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

Compatibility tests parse the pinned upstream VHS command/settings examples
and the all-commands fixture. End-to-end tests exercise real PTY input, waits,
clipboard paste, screenshots, text checkpoints, and multiple outputs. Upstream
fixtures and theme data retain Charm's MIT license and theme author metadata.

## Releases and Homebrew

Like gloss, GoReleaser publishes archives/checksums to GitHub Releases and a
`sigmamax` cask to `NimbleMarkets/homebrew-tap`. Release builds run on macOS with
the Command Line Tools SDK. The pinned Zig compiler builds four separate static
libghostty-vt archives; cgo then links the matching archive into each macOS/Linux,
amd64/arm64 binary. Target-specific pkg-config paths prevent linking the host
library into a cross-compiled binary. Dependency licenses accompany each archive.

With [GoReleaser](https://goreleaser.com/) 2.16.0 installed:

```sh
task release-check
task release           # macOS: build an unsigned snapshot in dist/, publish nothing
```

The release workflow packages pull requests that change the packaging, and can
also be run manually as a snapshot. It extracts the archives on all four native
platforms and checks the version, text GIFs, and Kitty PNG GIFs without installing
libghostty. A pushed `v*` tag publishes only after those checks and `task ci` pass.
Snapshots and prereleases do not update Homebrew.

Configure the same repository secrets as gloss before publishing:

- `HOMEBREW_TAP_GITHUB_TOKEN`: token with contents write access to
  `NimbleMarkets/homebrew-tap`. GitHub supplies `GITHUB_TOKEN` for this repo's release.
- For signed/notarized macOS binaries: `MACOS_SIGN_P12`, `MACOS_SIGN_PASSWORD`,
  `MACOS_NOTARY_KEY`, `MACOS_NOTARY_KEY_ID`, and `MACOS_NOTARY_ISSUER_ID`.
  Signing is enabled when `MACOS_SIGN_P12` is set. Local snapshots skip it.

## Acknowledgements

Thank you to [Charm](https://charm.sh) for [VHS](https://github.com/charmbracelet/vhs),
which inspired sigmamax's tape format and terminal recording workflow, and to
[Ghostty](https://ghostty.org) for libghostty-vt, which powers sigmamax's terminal
emulation and Kitty graphics support. The bundled theme catalog and compatibility
fixtures come from VHS commit `24fa2254a9806091e6ee6a980e9f3bcfe0a9ba53`; see
[theme license](internal/theme/LICENSE) and [fixture license](internal/tape/testdata/vhs/LICENSE).

## License

[MIT](LICENSE). Bundled fonts are covered by [FONT-LICENSE](FONT-LICENSE).
