# sigmamax

Record a terminal session to GIF, MP4, or WebM, including Kitty graphics. sigmamax is a new
Go module (`github.com/NimbleMarkets/sigmamax`), licensed under MIT. It runs a
real PTY, feeds its bytes into libghostty-vt, and composites pixels in Go.
There is no browser backend, ttyd, xterm.js, headless Chrome, or ghostty-web.
It is not a fork of VHS.

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

## Output formats

The filename extension selects the encoder in both tape `Output` and live
`record -o`. Unsupported extensions are rejected before launching the PTY. Text checkpoints
(`.txt`, `.ascii`, `.test`) and PNG frame directories (a path ending in `/`) are
also accepted; see [docs/tapes.md](docs/tapes.md).

| Extension | Encoder | Runtime requirement |
| --- | --- | --- |
| `.gif` | Go GIF encoder, 256-color palette with exact theme colors, dithered | None beyond sigmamax |
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
Hidden time is omitted. Video duration is rounded to its configured frame interval (`Framerate`; 50 ms at the default 20 FPS).

Output is written to a temporary file alongside the destination and renamed
only after successful encoding. Cancellation finalizes captured frames before
returning the cancellation error. Encoder failures leave any previous output
intact and include FFmpeg's diagnostics. Slow encoding can reduce the rate at
which the PTY is sampled; elapsed visible time is retained in the video.

## Tapes

sigmamax supports the VHS tape language on its native renderer. The goal is
useful script compatibility; font metrics, antialiasing, decorations, and encoded
pixels follow sigmamax rather than matching VHS screenshots.

**Only run tapes you trust.** A tape runs real programs: `Set Shell`, `Type`, and
`Env` execute commands with your privileges, and `Output`, `Source`, and
`Screenshot` read and write files wherever the path points, as in VHS.

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

See [docs/tapes.md](docs/tapes.md) for every command and setting, `Wait` semantics,
quoting rules, and output behavior.

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

## Documentation

- [docs/tapes.md](docs/tapes.md): the tape language, commands, settings, and capture timing.
- [docs/rendering.md](docs/rendering.md): fonts, the renderer, and the supported Kitty graphics slice.
- [docs/development.md](docs/development.md): building from source, tests and CI, and releases.

## Acknowledgements

Thank you to [Charm](https://charm.sh) for [VHS](https://github.com/charmbracelet/vhs),
which inspired sigmamax's tape format and terminal recording workflow, and to
[Ghostty](https://ghostty.org) for libghostty-vt, which powers sigmamax's terminal
emulation and Kitty graphics support. The bundled theme catalog and compatibility
fixtures come from VHS commit `24fa2254a9806091e6ee6a980e9f3bcfe0a9ba53`; see
[theme license](internal/theme/LICENSE) and [fixture license](internal/tape/testdata/vhs/LICENSE).

## License

[MIT](LICENSE). Bundled fonts are covered by [FONT-LICENSE](FONT-LICENSE).
