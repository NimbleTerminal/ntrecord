# sigmamax

Record a terminal session to GIF, MP4, or WebM, including Kitty graphics.

sigmamax runs your program on a real PTY, passes its output to libghostty-vt (the terminal engine from Ghostty), and draws every frame itself in Go. No browser, screen capture, or window is involved. It reads [VHS](https://github.com/charmbracelet/vhs) tape scripts, but it is not a fork of VHS.

## Install

Once the first tagged release is published, install it from the same Homebrew tap as gloss, on macOS (13 or later) or Linux, Intel or ARM:

```sh
brew install --cask nimblemarkets/tap/sigmamax
sigmamax --version
```

The release is a single binary with libghostty-vt and a default font built in, so recording a GIF needs nothing else. MP4 and WebM also need `ffmpeg`:

```sh
brew install ffmpeg
```

Archives and SHA-256 checksums are on [GitHub Releases](https://github.com/NimbleMarkets/sigmamax/releases). Linux binaries are static (musl) and macOS binaries use the system libraries, so none of them needs Zig, pkg-config, or Ghostty at runtime. To build from source, see [docs/development.md](docs/development.md).

## Quick start

```sh
sigmamax record -o session.gif           # record a shell; exit it (Ctrl-D) to save
sigmamax record -o demo.gif -- ls -la    # record one command
sigmamax demo.tape                       # run a script
sigmamax validate demo.tape              # check a script without recording it
sigmamax doctor                          # check that this machine can record
```

## Output formats

The file extension picks the format, for both a tape's `Output` and `record -o`. An unsupported extension is rejected before the PTY starts.

| Extension | Result | Needs |
| --- | --- | --- |
| `.gif` | Animated GIF (256 colors, with the theme's own colors kept exact) | nothing |
| `.mp4` | H.264 video (`libx264`, CRF 18, fast-start) | FFmpeg |
| `.webm` | VP9 video (`libvpx-vp9`, CRF 30) | FFmpeg |
| `.txt`, `.ascii`, `.test` | The screen's text at each step | nothing |
| a path ending in `/` | One numbered PNG per frame | nothing |

FFmpeg must be on `PATH` and built with the encoder for the format. sigmamax checks this before it starts recording. On Debian or Ubuntu, `sudo apt-get install ffmpeg`.

Video is made from the same frames as GIF, piped straight into FFmpeg's [raw-video input](https://ffmpeg.org/ffmpeg-formats.html#rawvideo) with no intermediate files. It is silent and uses `yuv420p` for wide playback support, so small colored text can look slightly softer. Odd sizes are padded by one pixel on the right and bottom. Hidden time is left out, and the duration is rounded to the frame interval (`Framerate`; 50 ms at the default 20 FPS).

Output goes to a temporary file next to the destination and is renamed only after encoding succeeds, so a failed recording never replaces an existing file. Encoder errors include FFmpeg's own message. If the encoder is slow, the PTY is sampled less often, but the elapsed visible time is kept.

## Tapes

A tape is a script of settings, keystrokes, and waits. sigmamax runs VHS tapes on its own renderer, so scripts carry over, but fonts, anti-aliasing, and window decoration are sigmamax's own and do not match VHS pixel for pixel.

**Only run tapes you trust.** A tape runs real programs: `Set Shell`, `Type`, and `Env` execute commands with your privileges, and `Output`, `Source`, and `Screenshot` read and write files wherever the path points, as in VHS.

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

`sigmamax validate demo.tape` checks a tape without recording it, and `sigmamax doctor` checks the machine. `sigmamax -mode deterministic demo.tape` runs a virtual clock: the same tape gives byte-identical output, and `Sleep` costs no real time ([details](docs/tapes.md#deterministic-mode)). Every command and setting is in [docs/tapes.md](docs/tapes.md).

## Live recording

```sh
sigmamax record -o session.gif -cols 100 -rows 30
sigmamax record -o command.gif -- sh -c 'printf "hello\n"; sleep 2'
```

`record` starts a new PTY running your shell, or the command you give, and connects your terminal to it. Exit the shell (or press Ctrl-D) to finish and write the recording. Ctrl-C goes to the program you are running when stdin is a terminal. SIGTERM stops the recording and saves the frames captured so far. Your terminal mode is restored on exit. It cannot attach to a process that is already running.

The recorded window keeps the size you asked for. Limits and failure behavior:

- GIF keeps each distinct frame in memory, up to 512 MiB, so keep GIF recordings fairly short. Video is streamed to FFmpeg and does not accumulate.
- The window is limited to 8192 pixels per side and 16 megapixels.
- If the program fails, or output cannot be forwarded (a closed stdout pipe, for example), sigmamax tries to save every output first and then returns the error.
- FFmpeg writes have a 30-second deadline and are interrupted on cancellation. A video stream that fails or is cut off cannot always be saved. Other formats finish independently, and an existing file is replaced only after a successful encode.

## Documentation

- The [user guide](https://nimblemarkets.github.io/sigmamax/): installing, a first recording, tapes, appearance, output formats, recording images, and troubleshooting.
- [docs/tapes.md](docs/tapes.md): the tape language, commands, settings, and capture timing.
- [docs/rendering.md](docs/rendering.md): fonts, the renderer, and the supported Kitty graphics.
- [docs/development.md](docs/development.md): building from source, tests and CI, and releases.

## Acknowledgements

Thank you to [Charm](https://charm.sh) for [VHS](https://github.com/charmbracelet/vhs), which inspired sigmamax's tape format and workflow, and to [Ghostty](https://ghostty.org) for libghostty-vt, which provides the terminal emulation and Kitty graphics support. The bundled theme catalog and compatibility fixtures come from VHS commit `24fa2254a9806091e6ee6a980e9f3bcfe0a9ba53`; see the [theme license](internal/theme/LICENSE) and [fixture license](internal/tape/testdata/vhs/LICENSE).

[foley](https://github.com/GH-Jaider/foley) takes the same approach and came first. We borrowed these ideas from it, without its code: the deterministic clock, the `xterm-ghostty` terminal identity, golden frames checked on every platform, upstream VHS tapes as conformance tests, and the `validate` and `doctor` commands.

## License

[MIT](LICENSE). Bundled fonts are covered by [FONT-LICENSE](FONT-LICENSE).
