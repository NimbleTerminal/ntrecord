---
title: "Install"
weight: 10
bookToC: false
---

# Install

With [Homebrew](https://brew.sh), on macOS (13 or later) or Linux, Intel or ARM:

```sh
brew install --cask nimbleterminal/tap/ntrecord
ntrecord --version
```

The release is a single binary. It includes the terminal engine and default text, symbol, and color emoji fonts, so recording a GIF needs nothing else. Release archives and checksums are on the [releases page](https://github.com/NimbleTerminal/ntrecord/releases).

ntrecord runs on macOS and Linux. There is no Windows build.

## For MP4 and WebM

Video is encoded by [FFmpeg](https://ffmpeg.org), which you install yourself:

```sh
brew install ffmpeg                 # macOS
sudo apt-get install ffmpeg         # Debian, Ubuntu
```

The FFmpeg must include the `libx264` encoder (MP4) or `libvpx-vp9` (WebM). ntrecord checks before it starts recording, so a missing encoder is reported straight away rather than after a long session.

## For agents

If you are installing ntrecord for an AI agent, also install its skill, which teaches the agent to write tapes, record commands, and render casts headlessly:

```sh
ntrecord skill install                    # for the agents on this machine
ntrecord skill install ~/.claude/skills   # or into the skills folder named
ntrecord skill                            # print it, to place it by hand
```

Installation always writes the shared `~/.agents/skills` folder, which Codex and other agents read but do not create, and the folder of each agent found by its home: Claude Code (`~/.claude/skills`) and Kimi Code (`~/.kimi-code/skills`, or `$KIMI_CODE_HOME`). The skill comes from the binary, so it always matches the flags it was built with; run the install again after upgrading. Stdout carries just the paths written, one per line; progress goes to stderr.

With Node, the [skills CLI](https://github.com/vercel-labs/skills) installs it from the repository instead:

```sh
npx skills add NimbleTerminal/ntrecord
```

## From source

Building needs Go, [Task](https://taskfile.dev), and a C compiler. The first build downloads and builds its own pinned Zig and libghostty, with nothing installed globally. The steps are in the [development guide](https://github.com/NimbleTerminal/ntrecord/blob/main/docs/development.md).
