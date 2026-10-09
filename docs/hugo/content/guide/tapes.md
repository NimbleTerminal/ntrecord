---
title: "Writing tapes"
weight: 30
---

# Writing tapes

A tape is a plain text file, one command after another. Lines starting with `#` are comments. If you have used [VHS](https://github.com/charmbracelet/vhs), this is the same language.

```text
# demo.tape
Output demo.gif          # where the recording goes
Set Shell /bin/sh
Set Columns 80
Set Rows 24

Type "echo hello"        # type text, one key at a time
Enter                    # press a key
Sleep 1s                 # let the viewer read it
```

Run it with `ntrecord demo.tape`. Mistakes are reported with the file, line, and column. To check a tape without recording it, run `ntrecord validate demo.tape`: it also confirms the font, theme, `Require` programs, and outputs (video needs FFmpeg) are all usable.

> [!WARNING]
> **Only run tapes you trust.** A tape runs real commands with your privileges and can write files wherever its paths point.

## Setup comes first

`Output`, `Set`, `Env`, and `Require` go at the top, before any typing or keys. (`Set TypingSpeed` is the one exception: it can change mid-tape.)

```text
Output demo.gif
Output demo.mp4                   # more than one output is fine
Require git                       # stop early if a program is missing
Env GREETING "Hello"              # set an environment variable for the session
Set Theme "Dracula"
```

## Typing and keys

```text
Type "git status"                 # text, at the current typing speed
Type@100ms "slowly"               # a per-command speed
Enter
Backspace 3                       # repeat a key
Down@200ms 2                      # key, interval, count
Ctrl+C                            # modifiers: Ctrl, Alt, Shift
Tab
Escape
```

Arrow keys, `Home`, `End`, `PageUp`, `PageDown`, and `F1` to `F25` work too. `Set TypingSpeed 50ms` changes how fast `Type` goes (the default is 25 ms between keys).

Quote text with `"…"`, `'…'`, or backticks. The inside is typed exactly as written, so pick a different quote character when the text contains one.

## Waiting

`Sleep` waits a fixed time. `Wait` waits for something to appear, which is steadier when a command takes an unpredictable time:

```text
Type "make test"
Enter
Wait+Screen /PASS/                # until PASS is on screen (up to 15 s)
Wait+Screen@60s /Build complete/  # with a longer limit
Wait                              # until the prompt returns
Sleep 2s
```

`Wait+Screen` looks at everything visible; `Wait+Line` (and plain `Wait`) looks at the line the cursor is on. The pattern is a [Go regular expression](https://pkg.go.dev/regexp/syntax). The default pattern, `[$>#]$`, matches a shell prompt, but it can also match text you typed, so use a specific pattern when it matters. If the wait times out, the error says what pattern it wanted and what was on screen.

For programs that emit OSC 7501, use structured status instead:

```tape
Wait+Status@60s blocked app=agent kind=question
Type "yes"
Enter
Wait+Status@60s done app=agent
```

States are `idle`, `working`, `done`, `blocked`, and `error`. Optional `app=`,
`id=`, and `kind=` filters match one current record, including status received
before the wait. `id=` selects the root; omitting it allows any record. `kind=`
applies only to `blocked` and accepts `permission`, `question`, or `auth`.
`WaitTimeout` and `@time` work as for screen waits, including in deterministic
mode. Programs must replace or clear old completion records before repeating
a task. Status remains invisible in GIF/video and is preserved in cast output.

## Keeping setup out of the picture

`Hide` stops recording while the session keeps running; `Show` resumes. The time in between is cut out. Use it to get ready without showing it:

```text
Hide
Type "cd ~/project && clear"
Enter
Wait
Show

Type "make"
Enter
```

Hide only pauses the recording. It does not wait for a running command, so put a `Wait` before `Hide` or `Show` when you need the command finished first.

## Reproducible recordings

Normally ntrecord records in real time, so two runs of a tape are close but not identical, and a slow encode can make the recording a little longer than the script. For a demo you regenerate and compare, or one that is a test, ask for a virtual clock:

```sh
ntrecord -mode deterministic demo.tape
```

Each step is held for exactly the time the tape says, once the program has settled, so the same tape gives the same file every time, and a `Sleep 5s` takes no real time. The one rule: a virtual clock does not wait for your program, so end any slow step with `Wait+Screen /text/` instead of guessing with `Sleep`. If a program is still drawing when the tape ends, ntrecord tells you. Programs that animate by themselves need the default real-time mode.

## More

```text
Copy "git log --oneline"          # an isolated clipboard, not yours
Paste
Screenshot title.png              # save the current frame as a PNG
Source common.tape                # reuse shared settings
```

Every command and setting is in the [tape reference](https://github.com/NimbleTerminal/ntrecord/blob/main/docs/tapes.md). To change how the recording looks, see [How it looks](../appearance/).
