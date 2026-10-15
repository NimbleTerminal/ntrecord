# ntrecord -- Nimble Terminal Recorder

Record a terminal session to GIF, MP4, WebM, or asciicast, including Kitty graphics.

`ntrecord` runs your program on a real PTY, passes its output to `libghostty-vt` (the terminal engine from [Ghostty](https://ghostty.org/)), and draws every frame itself in Go.  No browser, screen capture, or window is involved. It reads [Charmbracelet VHS](https://github.com/charmbracelet/vhs) tape scripts, but it is not a fork of VHS.

## Install

Once the first tagged release is published, install it from the same Homebrew tap as gloss, on macOS (13 or later) or Linux, Intel or ARM:

```sh
brew install --cask nimbleterminal/tap/ntrecord
ntrecord --version
```

The release is a single binary with `libghostty-vt` and default text, symbol, and color emoji fonts built in, so recording a GIF needs nothing else. MP4 and WebM also need `ffmpeg`:

```sh
brew install ffmpeg
```

Archives and SHA-256 checksums are on [GitHub Releases](https://github.com/NimbleTerminal/ntrecord/releases). Linux binaries are static (musl) and macOS binaries use the system libraries, so none of them needs Zig, pkg-config, or Ghostty at runtime. To build from source, see [docs/development.md](docs/development.md).

ntrecord also carries a skill that teaches an AI agent to script, record, and render with it. Install it for the agents on your machine, from the binary or from the repository:

```sh
ntrecord skill install                        # from the binary, so it matches its flags
npx skills add NimbleTerminal/ntrecord         # or from the repository, for every agent it finds
```

Or read [`SKILL.md`](skills/ntrecord/SKILL.md).

## Quick start

```sh
ntrecord record -o session.gif           # record a shell; exit it (Ctrl-D) to save
ntrecord record -o demo.gif -- ls -la    # record one command
ntrecord demo.tape                       # run a script
ntrecord validate demo.tape              # check a script without recording it
ntrecord doctor                          # check that this machine can record
```

## Output formats

The file extension picks the format, for both a tape's `Output` and `record -o`. An unsupported extension is rejected before the PTY starts.

| Extension | Result | Needs |
| --- | --- | --- |
| `.cast` | Asciicast v3 terminal events, including inline Kitty graphics | None beyond ntrecord |
| `.gif` | Animated GIF (up to 256 colors, one adaptive palette, exact theme colors) | nothing |
| `.mp4` | H.264 video (`libx264`, CRF 18, fast-start) | FFmpeg |
| `.webm` | VP9 video (`libvpx-vp9`, CRF 30) | FFmpeg |
| `.txt`, `.ascii`, `.test` | The screen's text at each step | nothing |
| a path ending in `/` | One numbered PNG per frame | nothing |

FFmpeg must be on `PATH` and built with the encoder for the format. ntrecord checks this before it starts recording. On Debian or Ubuntu, `sudo apt-get install ffmpeg`.

Video is made from the same frames as GIF, piped straight into FFmpeg's [raw-video input](https://ffmpeg.org/ffmpeg-formats.html#rawvideo) with no intermediate files. It is silent and uses `yuv420p` for wide playback support, so small colored text can look slightly softer. Odd sizes are padded by one pixel on the right and bottom. Hidden time is left out, and the duration is rounded to the frame interval (`Framerate`; 50 ms at the default 20 FPS).

Output goes to a temporary file next to the destination and is renamed only after encoding succeeds, so a failed recording never replaces an existing file. Encoder errors include FFmpeg's own message. If the encoder is slow, the PTY is sampled less often, but the elapsed visible time is kept.

## Asciicast recording and rendering

Save terminal events to a standard `.cast` file, then render it without rerunning
the recorded commands:

```sh
ntrecord record -o session.cast -- sh -c 'printf "hello\n"; sleep 2'
ntrecord render session.cast -o session.gif
ntrecord render session.cast -o session.mp4 -fps 30 -speed 2
ntrecord render session.cast -o frames/ -font-size 24
```

The writer uses [asciicast v3](https://docs.asciinema.org/manual/asciicast/v3/);
`render` reads v2 and v3. Casts preserve timed terminal output, grid size, initial
colors, inline Kitty image data, and trailing silence. They stream to disk rather
than storing frames. A tape may combine `Output session.cast` with other outputs;
its realtime or deterministic clock and `PlaybackSpeed` also apply to the cast.

`render` uses the existing terminal renderer and supports GIF, MP4, WebM, PNG
frame directories, and a final text screen. It runs offline without a PTY or shell.
Use `-font`, `-font-size`, `-theme`, `-fps`, `-speed`, and `-cursor-blink` to control
the result. The default is Go Mono at 18 pixels and the cast's initial colors.
Timing is sampled at the selected frame rate; final output gets at least one frame.

Play a cast directly inside a Kitty-capable terminal:

```sh
ntrecord play session.cast
ntrecord play session.cast -paused -speed 2
```

The player uses ntrecord's renderer and displays the resulting frames through
Kitty graphics. It checks graphics support before switching to an alternate
screen. Run it directly in Kitty or Ghostty; tmux passthrough and terminals
without Kitty graphics are not supported in this version. Text is rendered into
the image, so it is not selectable as terminal text.

- `Space` or `P`: pause/resume; playback starts automatically unless `-paused`.
- `Left`/`Right` (or `H`/`L`): seek backward/forward five seconds.
- `Home`/`End`: seek to the beginning/end. `R` restarts and plays.
- `+`/`-`: change speed (0.1×–16×); `0` restores 1×.
- `Q`, Ctrl-C, or Ctrl-D: quit and restore the terminal.

At the end the final frame stays on screen until you restart or quit. Resize
your terminal to fit the display; the recording's grid stays fixed. `-font`,
`-font-size`, `-theme`, `-fps`, and `-cursor-blink` also work with `play`.
Backward seeking reconstructs state from the beginning, including earlier image
uploads; long recordings can take longer to seek. Cast events stream from disk.

Initial limits:

- Cast output rejects tapes with `Hide`, `ScrollUp`, or `ScrollDown` before
  execution. Dropping hidden bytes would lose terminal state; saving them would
  expose the hidden content. Viewport scrolling is a renderer operation.
- Rendering uses the initial fixed grid and rejects size-changing resize events.
  Input and unknown events are ignored while retaining their timing. Recorded
  idle time is preserved; the optional `idle_time_limit` hint is not applied.
- Kitty support is the same [direct-transport subset](docs/rendering.md) as video
  recording. File/shared-memory image transfers are not portable cast assets.
  Other asciicast players need their own Kitty renderer to display these images.
- Casts do not preserve fonts or window decorations. UTF-8 split across PTY reads
  is joined; invalid UTF-8 is replaced with U+FFFD to satisfy the JSON format.
  A single imported header/event line is limited to 64 MiB.

Input keystrokes, environment variables, and command metadata are not collected.
Anything printed by the program (including echoed commands) is still recorded.
Cancellation and child failures attempt to finalize captured output. Failed cast
writes and failed offline renders leave an existing destination intact.

## Tapes

A tape is a script of settings, keystrokes, and waits. ntrecord runs VHS tapes on its own renderer, so scripts carry over, but fonts, anti-aliasing, and window decoration are ntrecord's own and do not match VHS pixel for pixel.

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

`ntrecord validate demo.tape` checks a tape without recording it, and `ntrecord doctor` checks the machine. `ntrecord -mode deterministic demo.tape` runs a virtual clock: the same tape gives byte-identical output, and `Sleep` costs no real time ([details](docs/tapes.md#deterministic-mode)). Every command and setting is in [docs/tapes.md](docs/tapes.md).

## Live recording

```sh
ntrecord record -o session.gif -cols 100 -rows 30
ntrecord record -o command.gif -- sh -c 'printf "hello\n"; sleep 2'
```

`record` starts a new PTY running your shell, or the command you give, and connects your terminal to it. Exit the shell (or press Ctrl-D) to finish and write the recording. Ctrl-C goes to the program you are running when stdin is a terminal. SIGTERM stops the recording and saves the frames captured so far. Your terminal mode is restored on exit. It cannot attach to a process that is already running.

The recorded window keeps the size you asked for. Limits and failure behavior:

- GIF retains compressed source frames to build one stable palette for the whole recording. Its 512 MiB working-set budget includes sources, indexed frames, palettes and temporary buffers; long or noisy recordings can reach it sooner than simple terminal sessions. Video is streamed to FFmpeg and does not accumulate. See [GIF encoding and benchmarks](docs/gif-encoding.md) for details.
- The window is limited to 8192 pixels per side and 16 megapixels.
- If the program fails, or output cannot be forwarded (a closed stdout pipe, for example), ntrecord tries to save every output first and then returns the error.
- FFmpeg writes have a 30-second deadline and are interrupted on cancellation. A video stream that fails or is cut off cannot always be saved. Other formats finish independently, and an existing file is replaced only after a successful encode.

## Go libraries

Use `nimbleterminal.dev/ntrecord/gifencode` for animated GIF encoding,
`nimbleterminal.dev/ntrecord/asciicast` for terminal-event streams, and
`nimbleterminal.dev/ntrecord/render` for terminal-to-image rendering. The two
codecs use only the standard library; the renderer requires cgo/libghostty-vt.
See [library APIs and build instructions](docs/libraries.md).

## Documentation

- The [user guide](https://nimbleterminal.github.io/ntrecord/): installing, a first recording, tapes, appearance, output formats, recording images, and troubleshooting.
- [docs/tapes.md](docs/tapes.md): the tape language, commands, settings, and capture timing.
- [docs/rendering.md](docs/rendering.md): fonts, the renderer, and the supported Kitty graphics.
- [docs/libraries.md](docs/libraries.md): the public Go packages for GIF encoding, asciicast streams, and rendering.
- [docs/gif-encoding.md](docs/gif-encoding.md): how GIFs are optimized, with limits and benchmarks.
- [docs/development.md](docs/development.md): building from source, tests and CI, and releases.

## Acknowledgements

Thank you to [Charm](https://charm.sh) for [VHS](https://github.com/charmbracelet/vhs), which inspired ntrecord's tape format and workflow, and to [Ghostty](https://ghostty.org) for libghostty-vt, which provides the terminal emulation and Kitty graphics support. The bundled theme catalog and compatibility fixtures come from VHS commit `24fa2254a9806091e6ee6a980e9f3bcfe0a9ba53`; see the [theme license](internal/theme/LICENSE) and [fixture license](internal/tape/testdata/vhs/LICENSE).

[foley](https://github.com/GH-Jaider/foley) takes the same approach and came first, but before [`go-libghostty`](https://github.com/mitchellh/go-libghostty) existed (thanks for that too!).  We borrowed some ideas from `foley`, notably the deterministic clock and the `xterm-ghostty` terminal identity.

## License

This project is released under the [MIT License](https://en.wikipedia.org/wiki/MIT_License), see [`LICENSE`](./LICENSE).  Go Mono is covered by [`FONT-LICENSE`](FONT-LICENSE); bundled Noto fonts have [OFL notices and source information](render/fonts/README.md).

Copyright (c) 2026 [Neomantra Corp](https://www.neomantra.com).

----
Made with :heart: and :fire: by the team behind [NimbleTerminal](https://NimbleTerminal.com).
