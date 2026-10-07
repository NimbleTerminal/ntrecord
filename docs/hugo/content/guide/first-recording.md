---
title: "Your first recording"
weight: 20
bookToC: false
---

# Your first recording

## Record what you do

```sh
sigmamax record -o session.gif
```

This opens a new shell inside the recorder. Type commands as you normally would; when you are done, exit the shell (type `exit` or press Ctrl-D). sigmamax then writes `session.gif` and prints `Wrote session.gif`.

The recording is a fixed-size window, 80 columns by 24 rows unless you say otherwise:

```sh
sigmamax record -o session.gif -cols 100 -rows 30
sigmamax record -o session.gif -font-size 22        # larger text, larger picture
sigmamax record -o session.gif -shell /bin/bash     # a different shell
```

To record one command instead of a shell, put it after `--`. The recording ends when the command does:

```sh
sigmamax record -o listing.gif -- ls -la
sigmamax record -o build.mp4 -- sh -c 'make; sleep 2'    # a short pause at the end
```

The format comes from the file name: `.gif`, `.mp4` (needs FFmpeg), or `.webm` (needs FFmpeg). See [Output formats](../formats/).

> [!NOTE]
> The shell is `sh` by default, started with a plain `$ ` prompt. Ctrl-C reaches the program you are running; it does not stop the recording.

## Make it repeatable with a tape

Typing live is fine for a one-off. For a demo you will redo, write the steps down. Save this as `demo.tape`:

```text
Output demo.gif
Set Columns 60
Set Rows 10
Type "echo hello from sigmamax"
Enter
Sleep 1s
```

and run it:

```sh
sigmamax demo.tape
```

sigmamax types the text, presses Enter, waits a second, and writes `demo.gif`. Change the tape and run it again whenever your program changes. Continue with [Writing tapes](../tapes/).
