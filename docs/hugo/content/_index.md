---
title: "ntrecord"
type: docs
bookToC: false
---

# ntrecord

Record a terminal session to GIF, MP4, or WebM, including Kitty graphics. ntrecord runs your command in a real terminal, draws what it shows, and writes the result: no browser, no screen capture, no extra tools for GIFs.

## Quick start

```sh
brew install --cask nimbleterminal/tap/ntrecord

ntrecord record -o session.gif            # record a shell; exit it (Ctrl-D) to save
ntrecord record -o demo.gif -- ls -la     # record one command
ntrecord demo.tape                        # replay a script: a repeatable recording
```

A **tape** is a short script that types, waits, and saves, so the same recording can be made again whenever your program changes:

```text
Output demo.gif
Set Theme "Catppuccin Mocha"
Type "echo hello from ntrecord"
Enter
Sleep 1s
```

ntrecord reads the [VHS](https://github.com/charmbracelet/vhs) tape language, so existing tapes mostly just work.

## Where to look

- **[Guide](guide/)**: installing, your first recording, tapes, how a recording looks, the output formats, recording programs that show images, and fixing problems.
- **[Reference docs](https://github.com/NimbleTerminal/ntrecord/tree/main/docs)** on GitHub: every tape command and setting, the renderer, and building from source.

## Source

ntrecord is MIT-licensed and built in Go on [libghostty](https://ghostty.org). The source, issues, and releases are on [GitHub](https://github.com/NimbleTerminal/ntrecord).
