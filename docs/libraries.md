# Go libraries

Three public packages share the `nimbleterminal.dev/ntrecord` module:

| Import | Purpose | Native dependencies |
| --- | --- | --- |
| `nimbleterminal.dev/ntrecord/gifencode` | Timed RGBA images to optimized animated GIFs | None; standard library only |
| `nimbleterminal.dev/ntrecord/asciicast` | Streaming v2/v3 reader and v3 output writer | None; standard library only |
| `nimbleterminal.dev/ntrecord/render` | Terminal bytes to RGBA snapshots, including Kitty graphics and Unicode | cgo and the pinned libghostty-vt static library |

They do not execute recorded commands or open a PTY. Recording commands, tape
execution, FFmpeg orchestration, and the interactive terminal player remain
internal to the CLI. No public signature requires an internal type.

## GIF encoding

```go
var encoder gifencode.Encoder // ready to use; defaults to a 512 MiB budget
if err := encoder.Add(frame, 100*time.Millisecond); err != nil {
    return err
}
return encoder.Encode(output) // output is an io.Writer
```

Inputs are opaque, zero-origin `*image.RGBA` frames with constant dimensions.
Composite transparent input onto a background first. `Add` copies the pixels, so
the caller can reuse the input buffer. Configure `Seed`, `FixedPalette`, and
`MaxBytes` before the first `Add`; do not mutate those options afterward.
Durations accumulate and round to centiseconds. `Rotate` changes the starting
point within the loop; deferred errors are returned by `Encode`. The GIF loops
forever. `Encode` can write the accumulated animation more than once and does not
close its writer. The encoder retains its frames until it becomes unreachable.

This is a buffered encoder, not constant-memory streaming: palette construction
uses all retained frames. Its working-set budget is not a total process RSS cap.
See [GIF encoding](gif-encoding.md) for palette and resource tradeoffs.
A [runnable example](../gifencode/example_test.go) shows frame-buffer reuse.

## Asciicast streams

`asciicast.NewReader(io.Reader)` validates the header; `Next()` returns events
with absolute `time.Duration` timestamps and preserves unknown event codes.
`Header.Size()` and `Header.Colors()` work with both v2 and v3. Input lines are
limited to `asciicast.MaxLine` (64 MiB).

`asciicast.NewWriter(io.Writer, asciicast.Terminal)` writes a v3 header.
`Output(at, bytes)` consumes terminal output, retaining incomplete UTF-8 until the
next call. Call `End(at)` once after the final output to flush the last bytes and
preserve trailing silence. It deliberately does not record input, environment,
command metadata, or invented exit statuses. The writer does not impose the
reader's line-size limit; keep individual events below it for round trips.
Readers and writers do not close their underlying streams. See the
[runnable round-trip example](../asciicast/example_test.go).

## Rendering

```go
config := render.DefaultConfig()
config.Cols, config.Rows = 80, 24
renderer, err := render.New(config)
if err != nil {
    return err
}
defer renderer.Close()
if err := renderer.Write([]byte("\x1b[?25lHello 👍🏽\r\n")); err != nil {
    return err
}
frame, err := renderer.FrameAfter(0)
if err != nil {
    return err
}
return png.Encode(output, frame)
```

Start with `DefaultConfig`; the zero configuration is intentionally invalid.
Configuration is copied at construction. It controls the grid, font, theme,
spacing, decorations, cursor, and advertised version, independently of tape
settings. Width and height are fixed for a renderer's lifetime. Exported dimension
fields are read-only; `OutputSize()` includes decorations.

Frame images are cached and **must not be modified**. They remain valid after
later writes and after `Close`; copy pixels before editing. `Write` consumes its
input synchronously. `TakeReplies` drains terminal replies into a caller-owned
slice; a caller connecting a live process can forward those bytes to its PTY.
`FrameAfter` uses a relative clock for reproducible cursor timing. `FrameAt` uses
an explicit time. `CaptureFrameAfter` additionally applies image-replacement
smoothing; `RenderHeldAfter` reports synchronized-output holds for a recorder to
honor. Raw `Frame`/`FrameAt` snapshots can show an in-progress update.

`ProgramStatuses()` returns an independent OSC 7501 snapshot sorted by record ID.
Each `render.ProgramStatus` contains `ID`, `State`, `Kind`, `App`, `Title`,
`Message`, and `Progress` (-1 when unspecified). App names include ancestor
inheritance. Records replace earlier reports in full; subtree clears, shell
prompts, and terminal resets follow the protocol. At most 256 records are stored,
evicting the least recently updated when full. Status never alters frame pixels.

Forward `TakeReplies()` to the live process to answer OSC 7501 capability queries.
The reply lists the supported states and kinds, as provided by Ghostty 1.4.
After feeding its final output, call `ProgramExited()` to discard working and
blocked records; idle, done, and error remain. Keep all calls on the renderer's
own goroutine. Snapshots contain Go-owned values that survive further writes and
closing the renderer. Display titles and messages as plain text; strip invisible
formatting characters if presenting them outside the terminal grid.

Serialize calls on each renderer, including `Close`. Separate instances may run
concurrently. `Close` is idempotent; do not call other methods after closing.
Codec instances also require serialized access. Retained immutable frame images
can be read by other goroutines.

See [rendering](rendering.md) for font fallback, environment controls, color emoji,
protocol support, and limits. The renderer installs a process-wide libghostty PNG
decoder once on first construction. Redistribution of binaries using the bundled
fonts must include their [notices](../render/fonts/README.md), [Go Mono notice](../FONT-LICENSE),
and relevant native/dependency licenses.

## Building a consumer

The codecs build with `CGO_ENABLED=0`; importing them does not link the renderer.
The renderer needs Go 1.26 or later, cgo, and the same native library as the CLI.
Build it from a checkout of the same ntrecord version you depend on:

```sh
cd /path/to/ntrecord
task deps
export CGO_ENABLED=1
export PKG_CONFIG="$PWD/scripts/pkg-config.sh"
export PKG_CONFIG_PATH="$PWD/.deps/libghostty-vt/share/pkgconfig${PKG_CONFIG_PATH:+:$PKG_CONFIG_PATH}"
cd /path/to/your/application
go test ./...
```

Depend on a published module version once the vanity endpoint and release are
available. During local development, use a Go `replace` directive pointing to the
checkout. See [development](development.md) for native prerequisites and module
hosting setup. This does not require a separate Go module for each package.

`task test:library` creates an unrelated temporary module, tests both codecs with
cgo disabled, then tests rendering and GIF composition with cgo enabled. It checks
UTF-8 event round trips, frame ownership after input reuse and renderer shutdown,
terminal replies, and GIF timing. `task ci` includes it. Existing renderer golden
frames and GIF composition tests continue to test the implementation in its new
public location.
