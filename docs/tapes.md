# Tape language

ntrecord runs the VHS tape language on its own renderer. Scripts carry over, but fonts, anti-aliasing, window decoration, and encoded pixels are ntrecord's own and do not match VHS screenshots.

**Only run tapes you trust.** A tape runs real programs: `Set Shell`, `Type`, and `Env` execute commands with your privileges, and `Output`, `Source`, and `Screenshot` read and write files wherever the path points, as in VHS.

## Paths and sources

Run a tape from the directory it expects. Shell commands, output paths, font paths, and `Source` paths are all resolved from the directory you ran ntrecord in, including inside sourced tapes. A `Source` cycle, or nesting deeper than 32 files, is an error.

Following VHS, `Output` lines inside a sourced tape are ignored, so a shared tape cannot redirect the caller's recording. Repeat `Output` in the top-level tape to write several files from one session. Each file is replaced only after it encodes successfully.

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

## Commands

| Command | What it does |
| --- | --- |
| `Output path` | Write `.cast`, `.gif`, `.mp4`, `.webm`, `.txt`, `.ascii`, `.test`, or a PNG directory (a path ending in `/`). Repeat for several outputs. |
| `Require program` | Fail before starting if the program is not on the recording's `PATH`. |
| `Env KEY "value"` | Set an environment variable for the recorded program only. |
| `Source common.tape` | Include another tape with shared settings and actions. |
| `Type "text"`, `Type@50ms "text"` | Type text at the current typing speed, or at a speed for this command. |
| `Enter`, `Tab`, `Space`, `Backspace`, `Delete`, `Insert`, `Escape`, `Home`, `End`, `Up`, `Down`, `Left`, `Right`, `PageUp`, `PageDown`, `F1`–`F25` | Press a key. libghostty encodes it for the mode the program has set. |
| `Ctrl+C`, `Alt+x`, `Shift+Tab`, `Ctrl+Alt+Shift+R` | Press a key with modifiers. Chains such as `Ctrl+B+D` work too. |
| `Down@100ms 3` | Repeat a key, with an optional interval and a count. |
| `ScrollUp 5`, `ScrollDown@100ms 5` | Scroll the viewport. The program is not told. |
| `Sleep 0.5`, `Sleep 500ms`, `Sleep 500 ms` | Wait while recording continues. A bare number is seconds. |
| `Wait`, `Wait+Line /regex/`, `Wait+Screen@5s /regex/` | Wait for text on screen, with an optional scope and timeout. |
| `Copy "text"`, `Paste` | Use a clipboard private to the recording. Paste honors bracketed-paste mode. |
| `Hide`, `Show` | Stop and resume recording while the program keeps running. Hidden time is cut. |
| `Screenshot path.png` | Save the current frame, with Kitty images and decoration, as a PNG. |

`Output`, `Env`, `Require`, and settings must come before the first action. The one exception is `TypingSpeed`, which can change anywhere. A misplaced setting or an unknown command is an error, not silently ignored. Errors name the file, line, and column.

## Syntax

- Single quotes, double quotes, and backticks keep their contents literally, shell backslashes included. To put a quote inside, use a different delimiter. This matches VHS and replaces ntrecord's original Go-style escaping of double quotes.
- Regex literals may contain escaped slashes. Themes may be multiline JSON objects.
- `#` starts a comment outside a literal. Several commands may share a line.

## Waiting

`Wait+Line` checks the line the cursor is on. `Wait+Screen` checks everything visible. Both match a Go regular expression against the terminal's cells, so erased text and escape sequences never match by accident. With no arguments, `Wait` uses the `Line` scope, a 15 second timeout, and the pattern `[$>#]$`, which matches common shell prompts.

Use a specific pattern whenever you can. Text you type is echoed on screen, so a broad pattern can match your own input. A timeout error reports the pattern and the last text it saw.

Controls are ordered after any output already queued from the program, so a control cannot overtake output that arrived before it.

## Hide and Show

`Hide` records the last visible screen, even in the middle of synchronized output, then stops recording. `Show` resumes from the current screen. Neither waits for a running command, so put a `Wait` before them when the command must finish first.

At the end of a successful tape, ntrecord waits for the output to go quiet (100 ms of silence, one second at most). Use `Wait` or `Sleep` for commands that need longer. Finishing the tape closes the PTY and stops the shell.

## Settings

| Setting | Values and default |
| --- | --- |
| `Shell` | Executable name or path, started with `-i`. Default `/bin/sh`. |
| `FontSize`, `FontFamily` | 6 to 96 pixels (default 18). `Go Mono` is built in; use an installed family name or a `.ttf`/`.otf` path for another. |
| `Columns` (or `Cols`), `Rows` | Grid size. Default 80×24. |
| `Width`, `Height` | Total output size in pixels, decoration included. Cannot be combined with `Columns` or `Rows` on the same axis. |
| `LetterSpacing`, `LineHeight` | Extra pixels between letters (default 0), and a line-height multiplier (default 1). |
| `TypingSpeed` | Time between keystrokes. Default 25 ms. Can change during the tape. |
| `WaitTimeout`, `WaitPattern` | Default timeout and `/regex/` for `Wait`. |
| `Framerate`, `PlaybackSpeed` | 1 to 120 FPS (default 20), and a playback speed multiplier (default 1). |
| `LoopOffset` | Where a GIF starts: a frame index (fractions allowed) or a percentage such as `50%`. |
| `Theme` | One of 348 bundled VHS theme names, or inline JSON with foreground, background, cursor, and ANSI colors. |
| `Padding`, `Margin`, `MarginFill` | Pixels around the grid and around the window, and an optional `#RRGGBB` margin color. |
| `WindowBar`, `WindowBarSize` | `Colorful`, `ColorfulRight`, `Rings`, `RingsRight`, or `None`. Bar height defaults to 30 pixels. |
| `BorderRadius`, `CursorBlink` | Corner radius in pixels, and whether the cursor blinks (default false). |

A few details:

- `Copy` and `Paste` use their own clipboard. They never read or change your desktop clipboard.
- Fonts are looked up in the standard macOS and Linux font directories. A missing font gives an error that says what to do. If a bold or italic variant is missing, the regular face is used.
- A theme's selection colors are accepted but have no effect, since nothing is ever selected.

## Capture and outputs

Capture runs at the configured framerate. GIF merges identical neighboring frames. Video repeats held frames on a fixed timeline and lets the codec compress the duplicates. `PlaybackSpeed` changes recorded time, not how fast the shell receives input or how long a `Wait` takes. `LoopOffset` applies to GIF only.

PNG frames are numbered `000001.png`, `000002.png`, and so on. The destination must be absent or empty, so existing frames are never deleted.

Text outputs hold the screen's text after each visible action, but only when it changed, plus the final screen, separated by a rule. Put a `Wait` before any checkpoint that depends on late output. They make readable golden files, in ntrecord's own format.

To try the features, run `task demo:language` for a sourced, themed tape with clipboard use, keyboard editing, waits, several outputs, and a screenshot. `task demo:kitty` draws a PNG through direct Kitty transfer and waits for `go run` with `Wait+Screen`.

## Deterministic mode

By default a recording samples the screen on the wall clock (`realtime`). That captures whatever a program does, animations included, but two runs of one tape differ, and slow encoding can stretch the recording past the script. For a virtual clock, pass `-mode deterministic`:

```sh
ntrecord -mode deterministic demo.tape
```

After every step, ntrecord lets the program settle: 30 ms of silence, or one second at most. The first step also waits for any output, so a slow shell is not recorded before its prompt appears. Then the screen is held for exactly the time the tape names: the typing interval after each key, or the duration of a `Sleep`, divided by `PlaybackSpeed`.

The results:

- The same tape gives byte-identical output, so a recording can be a golden file.
- `Sleep 5s` costs no real time.
- Hidden time is left out, as usual, and a blinking cursor still blinks.

The cost is that a virtual clock does not wait for your program. A step that takes real time, such as a build, must end with `Wait+Screen /text/`, because a `Sleep` will not wait for it. If the screen is still changing when the tape ends, ntrecord says so on stderr.

Programs that animate on their own, such as spinners and shaders, need `realtime`, and so does a live `record` session, which follows a person's clock. Typing is limited by the settle window and by encoding each distinct frame, so a tape that mostly types is not much faster than real time. A tape that sleeps or waits is.

## Asciicast output

`Output session.cast` records standard asciicast v3 terminal events. It can be
combined with pixel outputs and follows the selected recording clock. Tapes with
`Hide`, `ScrollUp`, or `ScrollDown` are rejected before execution when any output
is a cast. See the [recording and rendering guide](../README.md#asciicast-recording-and-rendering)
for playback, timing, and portability limits.
