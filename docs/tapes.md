# Tape language

sigmamax supports the VHS tape language on its native renderer. The goal is
useful script compatibility; font metrics, antialiasing, decorations, and encoded
pixels follow sigmamax rather than matching VHS screenshots.

**Only run tapes you trust.** A tape runs real programs: `Set Shell`, `Type`, and
`Env` execute commands with your privileges, and `Output`, `Source`, and
`Screenshot` read and write files wherever the path points, as in VHS.

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
direct PNG APCs, waiting for `go run` to finish with `Wait+Screen`. Tape completion
closes the PTY and stops its shell.

## Deterministic mode

By default a recording samples the screen on the wall clock (`realtime`), so it
captures whatever a program does, animations included, but two runs of one tape
differ, and slow encoding can stretch the recording past the script. Pass `-mode
deterministic` to run a virtual clock instead:

```sh
sigmamax -mode deterministic demo.tape
```

After every step the program is given time to settle (30 ms of silence, 1 s at
most; the first step also waits for any output, so a slow shell is not recorded
before its prompt). Then the screen is held for exactly the time the tape names:
the typing interval after each key, the duration of a `Sleep`, divided by
`PlaybackSpeed`. `Sleep 5s` costs no real time, hidden time is omitted as usual, a
blinking cursor still blinks, and the same tape gives byte-identical output, so a
recording can be a golden file.

The trade is that a virtual clock does not wait for the program. A step that takes
real time, such as a build, must end with `Wait+Screen /text/`; a `Sleep` will not
wait for it. If the screen is still changing when the tape ends, sigmamax says so
on stderr. Programs that animate on their own (spinners, shaders) need `realtime`,
and so does a live `record` session, which follows a person's clock. Typing is
bound by the settle window and by encoding each distinct frame, so a tape that
mostly types is not much faster than real time; one that sleeps or waits is.
