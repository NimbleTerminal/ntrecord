---
title: "Install"
weight: 10
bookToC: false
---

# Install

With [Homebrew](https://brew.sh), on macOS (13 or later) or Linux, Intel or ARM:

```sh
brew install --cask nimblemarkets/tap/sigmamax
sigmamax --version
```

The release is a single binary. It includes the terminal engine and a default font, so recording a GIF needs nothing else. Release archives and checksums are on the [releases page](https://github.com/NimbleMarkets/sigmamax/releases).

sigmamax runs on macOS and Linux. There is no Windows build.

## For MP4 and WebM

Video is encoded by [FFmpeg](https://ffmpeg.org), which you install yourself:

```sh
brew install ffmpeg                 # macOS
sudo apt-get install ffmpeg         # Debian, Ubuntu
```

The FFmpeg must include the `libx264` encoder (MP4) or `libvpx-vp9` (WebM). sigmamax checks before it starts recording, so a missing encoder is reported straight away rather than after a long session.

## From source

Building needs Go, [Task](https://taskfile.dev), and a C compiler. The first build downloads and builds its own pinned Zig and libghostty, with nothing installed globally. The steps are in the [development guide](https://github.com/NimbleMarkets/sigmamax/blob/main/docs/development.md).
