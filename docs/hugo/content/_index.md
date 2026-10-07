---
title: "sigmamax"
type: docs
bookToC: false
---

# sigmamax

Record a terminal session to GIF, MP4, or WebM, including Kitty graphics. sigmamax runs your command in a real terminal, draws what it shows, and writes the result: no browser, no screen capture, no extra tools for GIFs.

## Quick start

```sh
brew install --cask nimblemarkets/tap/sigmamax

sigmamax record -o session.gif            # record a shell; exit it (Ctrl-D) to save
sigmamax record -o demo.gif -- ls -la     # record one command
sigmamax demo.tape                        # replay a script: a repeatable recording
```

A **tape** is a short script that types, waits, and saves, so the same recording can be made again whenever your program changes:

```text
Output demo.gif
Set Theme "Catppuccin Mocha"
Type "echo hello from sigmamax"
Enter
Sleep 1s
```

sigmamax reads the [VHS](https://github.com/charmbracelet/vhs) tape language, so existing tapes mostly just work.

## Where to look

- **[Guide](guide/)**: installing, your first recording, tapes, how a recording looks, the output formats, recording programs that show images, and fixing problems.
- **[Reference docs](https://github.com/NimbleMarkets/sigmamax/tree/main/docs)** on GitHub: every tape command and setting, the renderer, and building from source.

## Source

sigmamax is MIT-licensed and built in Go on [libghostty](https://ghostty.org). The source, issues, and releases are on [GitHub](https://github.com/NimbleMarkets/sigmamax).
