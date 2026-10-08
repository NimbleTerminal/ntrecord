---
name: ntrecord
description: Record, script, and render terminal sessions with ntrecord (a VHS-compatible recorder). Use when the user wants a GIF, MP4, WebM, asciicast (.cast), or screenshot of a CLI/TUI demo, a `.tape` file written or debugged, a deterministic or golden-file terminal recording, a recording that includes Kitty graphics, or a `.cast` converted to video.
---

# ntrecord: terminal recordings you can script and check

ntrecord runs a program on a real PTY, feeds its output to libghostty-vt, and
draws every frame itself. No browser, window, or screen capture, so it works
headless (CI, SSH, an agent with no terminal). It reads VHS `.tape` scripts.

Three jobs: **script** a demo (`.tape`), **record** a command, **render** a
`.cast` to video/GIF offline.

## Pick the mode

| You want | Run |
| --- | --- |
| A repeatable demo | write `demo.tape`, then `ntrecord -mode deterministic demo.tape` |
| A program that animates (spinners, TUIs, shaders) | `ntrecord demo.tape` (realtime, the default) |
| One command recorded as-is | `ntrecord record -o out.gif -cols 100 -rows 30 -- cmd args` |
| A cast turned into a picture | `ntrecord render in.cast -o out.gif` (also `.mp4`, `.webm`, `.txt`, `frames/`) |
| To check a tape without recording | `ntrecord validate demo.tape...` |
| To check the machine | `ntrecord doctor` |

**Never run `ntrecord record` without `-- command`.** With no command it opens
an interactive shell and waits for a person to type and exit. Likewise
`ntrecord play` needs a Kitty-capable terminal; it is for humans.

**Only run tapes you trust**: `Set Shell`, `Type`, `Env` run real programs
with the user's privileges, and `Output`/`Source`/`Screenshot` write and read
arbitrary paths. Read a tape before running one you did not write.

## The loop: validate, record, look

1. `ntrecord validate demo.tape` first. It reports line:column errors, unknown
   themes/fonts, a missing `Require` program, and an `Output` that needs
   FFmpeg, all without starting a PTY.
2. Run it. Add `Output demo.txt` next to the GIF: the text output holds the
   screen after each visible step, so **you can read what was recorded
   without looking at pixels**.
3. To see it, add `Screenshot check.png` and open the PNG (Read tool, or
   `gloss --output - --max-edge 1024 check.png`). Do not try to open a GIF.
4. Exit status 1 means failure; stderr has the reason. `Wrote <path>` lines on
   stderr list what succeeded. Existing outputs are replaced only after a
   successful encode.

## Tape essentials

```text
Output demo.gif              # settings, Output, Env, Require: before any action
Output demo.txt
Require sh
Set Shell /bin/sh            # default; started with a plain "$ " prompt
Set Columns 80
Set Rows 24
Set FontSize 18
Set Theme "Catppuccin Mocha"
Set TypingSpeed 30ms
Type "make test"             # Type@50ms "…" overrides speed for one command
Enter
Wait+Screen@60s /PASS|FAIL/  # wait for output, not for time
Sleep 2s
Screenshot demo.png
```

- Actions: `Type`, `Enter`/`Tab`/`Escape`/arrows/`F1`–`F25`, `Ctrl+C`, `Alt+x`,
  `Sleep`, `Wait`, `Copy`/`Paste`, `Hide`/`Show`, `Screenshot`, `Source file.tape`.
- Quotes (`"`, `'`, backtick) keep contents literally; use a different
  delimiter to embed a quote. `#` starts a comment.
- `Set Width/Height` is total pixels; `Columns/Rows` is the grid. Don't mix
  them on one axis.
- Outputs by extension: `.gif`, `.mp4`/`.webm` (need `ffmpeg`), `.cast`,
  `.txt`/`.ascii`/`.test`, or a path ending in `/` for numbered PNGs (the
  directory must be absent or empty). Repeat `Output` for several.
- `Hide` cuts hidden time; run `Wait` before it so the command has finished.
  A tape must not end hidden ("no visible frames").
- Paths (outputs, fonts, `Source`) resolve from the directory ntrecord was
  *run in*, not the tape's directory. `cd` first or use absolute paths.
- `Output` lines inside a `Source`d tape are ignored.

Full command and setting tables: `docs/tapes.md` in the repo
(https://github.com/NimbleTerminal/ntrecord/blob/main/docs/tapes.md).

## Waiting correctly (most failures are here)

- `Wait+Screen /re/` matches anywhere visible; `Wait+Line /re/` only the
  cursor line. The pattern is a Go regex against *rendered cells*, not raw bytes.
- Typed text is echoed, so a loose pattern can match your own input. Use a
  pattern the *output* produces, not the command line.
- Bare `Wait` expects a prompt ending `$`, `>` or `#`. For other prompts:
  `Set WaitPattern /%$/`. Default timeout 15s: `Wait+Screen@90s` or
  `Set WaitTimeout 90s` for slow commands.
- A timeout error prints the pattern and the last screen text. Read it; the
  fix is almost always the pattern or the timeout.
- The tape ends when its last command does, after at most 1s of output
  quiet. Finish with `Wait` or `Sleep` if the tail matters.

## Deterministic vs realtime

`-mode deterministic` uses a virtual clock: same tape, byte-identical output,
`Sleep` costs no wall time. Use it for golden files and CI. The catch: it does
not wait for your program, so any step that takes real time (build, download)
**must** end in `Wait+Screen /…/`, never `Sleep`. Self-animating programs
(spinners, shaders) and live `record` sessions need realtime.

## Recording commands without a tape

```sh
ntrecord record -o demo.gif -cols 100 -rows 30 -font-size 20 -- ls -la
ntrecord record -o session.cast -- ./my-tui          # events, re-renderable
ntrecord render session.cast -o s.mp4 -fps 30 -speed 2 -theme "Dracula"
```

- The recording ends when the command does; add a trailing pause by wrapping
  it in a script that sleeps. `record` also echoes the program's output to
  stdout.
- `.cast` is asciicast v3 (`render` reads v2 and v3), so it is the cheap,
  tweakable master: re-render with a different font, theme, fps or speed
  without rerunning anything. A tape with `Hide`, `ScrollUp`, or `ScrollDown`
  cannot output `.cast`.
- Keystrokes, env vars and command metadata are not stored in a cast, but
  everything the program prints (including echoed secrets) is.

## Fonts, Unicode, Kitty graphics

- Built in: Go Mono. Coverage of symbols, emoji, braille and CJK depends on
  the ntrecord version (fallback fonts are evolving), so for anything beyond
  plain text, check `docs/rendering.md` (or the project's current docs) and
  record a quick `Screenshot` rather than assuming. If a glyph comes out as a
  replacement mark, choose a font that has it: `Set FontFamily "Name"` or a
  `.ttf`/`.otf`/`.ttc` path (`-font` for `render` and `play`; `record` has no
  `-font`, so record a `.cast` and `render` it). A path font is used for every
  style (no distinct bold/italic); a named family picks its own variants.
  `validate` loads the font, so a bad name or path fails before recording.
  Details: guide page "Using your own font"
  (https://nimbleterminal.github.io/ntrecord/guide/fonts/).
- Newer builds look up installed fallback fonts; `NTRECORD_FONT_PATH=""`
  restricts that to bundled fonts for machine-independent output.
- Programs see `TERM=xterm-ghostty`, `TERM_PROGRAM=ntrecord`. Kitty graphics
  work for **direct** transfer (PNG/RGB/RGBA, placements, Unicode
  placeholders). File and shared-memory transfer, animation frames, and
  relative placements do not. If images don't show: `env -u NO_COLOR …`,
  choose the program's "direct" transfer option, or force its Kitty switch.

## Common errors

| Message | Fix |
| --- | --- |
| `Set … must precede actions` | Move settings/`Output`/`Env`/`Require` above the first action |
| `Wait+Screen timed out … last text: "…"` | Fix pattern against the quoted text; raise timeout |
| `.mp4 output requires ffmpeg` | `brew install ffmpeg` / `apt-get install ffmpeg`; needs `libx264` / `libvpx-vp9` |
| `recording has no visible frames` | Tape ended while `Hide`den; add `Show` |
| `N of M tapes invalid` | See the `file:line:col` lines above it |
| GIF too large / hits 512 MiB | Smaller `Columns/Rows/FontSize`, lower `Framerate`, shorter, or use `.mp4`/`.webm` |
| Prompt isn't `$` | A shell rc file replaced it; use default `/bin/sh`, or `Hide` setup then `Show` |
| Replacement mark for a glyph | Font lacks it; set `FontFamily` (see Fonts above) |

If stuck: `ntrecord doctor`, then `ntrecord --version`, and point the user to
https://github.com/NimbleTerminal/ntrecord/issues with the tape and error text.

## Guardrails

- Don't hand-write ffmpeg/PTY/screenshot pipelines for a demo ntrecord can
  record; don't guess the screen contents when `.txt` output shows them.
- Don't use `Sleep` as a substitute for `Wait` on real work.
- Keep generated GIF/MP4 out of git unless the user wants them committed.
- Installed from Homebrew: `brew install --cask nimbleterminal/tap/ntrecord`.
  In a source checkout, `task build` makes `bin/ntrecord`.
