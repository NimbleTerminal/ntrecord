# Development

Building from source, testing, and releasing.

## Build

You need Go 1.26 or later, [Task](https://taskfile.dev/), a C compiler and SDK, `make`, `curl`, `tar` with xz support, and `shasum` or `sha256sum`. sigmamax builds on macOS and Linux, amd64 and arm64.

- **macOS:** `xcode-select --install` for the Command Line Tools, then `brew install go go-task`.
- **Debian or Ubuntu:** `sudo apt-get install build-essential curl xz-utils ca-certificates`, plus Go and Task.

**You do not need Zig or pkg-config installed.** Start here:

```sh
task build
task ci
task run -- examples/kitty.tape
task demo:text
```

The first build downloads **Zig 0.16.0** and **pkgconf 2.5.1** (both SHA-256 verified), builds pkgconf and the pinned libghostty-vt, and writes `bin/sigmamax`. Everything native lives in `.deps/`, which git ignores. Nothing needs sudo, a Homebrew Zig, shell profile changes, or `/tmp`. The first build needs network access and takes a few minutes; later builds reuse what it made. `task deps` runs the native setup on its own and passes Zig build flags after `--`. The build, test, and install tasks run it for you when the library is missing.

Task sets `CGO_ENABLED=1`, finds the local pkgconf through `scripts/pkg-config.sh`, and adds the local library to `PKG_CONFIG_PATH`, keeping any path you already have. Without Task:

```sh
sh scripts/build-libghostty.sh
export PKG_CONFIG="$PWD/scripts/pkg-config.sh"
export PKG_CONFIG_PATH="$PWD/.deps/libghostty-vt/share/pkgconfig${PKG_CONFIG_PATH:+:$PKG_CONFIG_PATH}"
"$PKG_CONFIG" --static --cflags --libs libghostty-vt-static
CGO_ENABLED=1 go build -o bin/sigmamax .
go run . examples/kitty.tape
go run . examples/text.tape
```

### Pins

Native setup downloads and builds Ghostty commit `0e0ff282f72c92d930d41f4703547d3bc9618ba2` under `.deps/`. The Go binding is pinned in `go.mod` to commit `183d2bc56a8a` (pseudo-version `v0.0.0-20261004135024-183d2bc56a8a`). Its API is unstable, so update the two together. After a bump, run `task terminfo` to regenerate the embedded `xterm-ghostty` terminfo entry from the same Ghostty revision. It needs `tic` from ncurses, and a test fails if the entry and its source disagree.

Tool versions, the source revision, and archive checksums are pinned in `scripts/native-common.sh` and `scripts/bootstrap-tools.sh`.

To use an existing matching installation, set `PKG_CONFIG_PATH` to its `share/pkgconfig` directory and `PKG_CONFIG` to your pkg-config executable. `GHOSTTY_PREFIX=/absolute/path` chooses another install prefix. `ZIG=/absolute/path/to/zig` overrides the downloaded compiler, which must report version 0.16.0.

### Cleaning

```sh
task clean       # remove bin/, dist/, .task/; keep dependencies and recordings
task build       # reuse the local native dependencies
task clean:all   # also remove all of .deps/ (downloads, tools, libraries, caches)
task build       # reproduce the first build from scratch
```

Clean tasks touch only this checkout. They never uninstall a Go-installed binary, delete recordings, or remove an external `GHOSTTY_PREFIX`.

### Linking

The unmodified `go.mitchellh.com/libghostty` binding uses cgo and links libghostty-vt statically through `libghostty-vt-static.pc`. There is no purego rebinding. The binary needs no Ghostty shared library, font file, or GIF encoder.

To cross-build for Linux amd64 from a supported Zig host:

```sh
GHOSTTY_PREFIX="$PWD/.deps/ghostty-linux-amd64" \
  sh scripts/build-libghostty.sh -Dtarget=x86_64-linux-gnu
PKG_CONFIG_PATH="$PWD/.deps/ghostty-linux-amd64/share/pkgconfig" \
  PKG_CONFIG="$PWD/scripts/pkg-config.sh" \
  CGO_ENABLED=1 GOOS=linux GOARCH=amd64 \
  CC="$PWD/.deps/tools/bin/zig cc -target x86_64-linux-gnu" \
  CXX="$PWD/.deps/tools/bin/zig c++ -target x86_64-linux-gnu" \
  go build -o bin/sigmamax-linux-amd64 .
```

Or run `task deps -- -Dtarget=x86_64-linux-gnu`, with a separate `GHOSTTY_PREFIX` so it does not overwrite the host build.

## Tests

```sh
task test
task ci
task demos
task test:video  # needs ffmpeg and ffprobe; never skips video tests silently
task demo:video
task demo:language
```

`task` alone runs `task build`, and `task list` shows everything. Arguments after `--` go to the program, for example `task test -- -run TestPNGFixture -v`. Builds use `-mod=readonly` and Go's cache, so changes to the native compiler settings are noticed.

`task ci` runs the same checks as GitHub Actions: formatting, module tidy and verify, race tests, vet, and build. The workflow starts with no `.deps/`, then runs `task build`, `task ci`, `task demos`, and `task demo:video` on macOS and Linux, and finally checks that `task clean` followed by a rebuild works. It installs FFmpeg, sets `SIGMAMAX_REQUIRE_FFMPEG=1` so video tests cannot be skipped, and uploads the demo GIFs, MP4s, WebMs, and the language screenshot and text. The Makefile just calls Task.

What the tests cover:

- **Rendering:** a base64 PNG fixture is written to the terminal and pixels are checked inside its placement. Further tests cover raw formats, replies, cropping, offsets, alpha, z-index, scrolling, deletion, and the lifetime of copied buffers.
- **End to end:** the Kitty demo runs through a tape and through live recording, and the GIFs and videos are decoded and checked for image pixels, for ordinary and Unicode placements. A text-only tape checks colored glyphs. Exit and cancellation tests check that recordings are finalized. Real PTY input, waits, paste, screenshots, text checkpoints, and several outputs are exercised.
- **Video:** codec, dimensions, padding, color, timing, cleanup after a failed encode, and that existing outputs survive. Without FFmpeg and ffprobe these skip locally; `task test:video` and CI require them.
- **VHS compatibility:** every tape from upstream VHS (109, in `internal/tape/testdata/vhs`) must parse, checked against a manifest of git blob SHAs that proves the files are unmodified. The few tapes sigmamax rejects are listed with reasons in `corpus_test.go`. The fixtures and theme data keep Charm's MIT license and theme author metadata.

## Deterministic rendering

A frame is a pure function of the terminal state and the embedded font, so the same state must draw the same pixels on macOS and Linux, amd64 and arm64. `internal/render/golden_frames_test.go` checks this. It draws 13 scenarios (text styles, box drawing, themes and window decoration, fractional and 96 px fonts, cursors, Kitty images and placeholders) and compares each frame's SHA-256 with `testdata/golden/frames.sha256`. The `golden` CI job runs it on four platforms, so all four must produce the committed hashes.

```sh
task test:golden                                   # check the hashes
SIGMAMAX_UPDATE_GOLDEN=1 task test:golden          # rewrite them after an intended change
SIGMAMAX_GOLDEN_DUMP=/tmp/frames task test:golden  # also write the frames as PNGs
task test:fma                                      # arm64 only: fusion off, imitating amd64
```

Review the PNGs before committing new hashes. A changed hash means a changed picture.

The check has already found a bug. arm64 fuses `a*b + c` into a single rounding and amd64 does not. The curve flattening in `golang.org/x/image/vector` (`lerp`) does not guard against this, so large glyphs differed by one alpha step in a few pixels. `render/face.go` therefore flattens curves itself with `float32(a*b)` conversions, and `TestFaceMatchesOpentype` holds it to the original. Keep that conversion wherever a product feeds a sum in the render path. To find which fusion site matters, bisect it with `bisect -compile=fma task test:golden` (from `golang.org/x/tools/cmd/bisect`).

Deterministic recording (`-mode deterministic`) has its own tests in `internal/record/deterministic_test.go`: three runs of one tape must produce the same GIF bytes while 5 s of `Sleep` costs almost no real time. If one ever fails intermittently, suspect the settle window (`settleQuiet` in `record.go`) first.

## Package boundaries

`internal/boundaries` is a test that fails when a package imports more than its rule in `boundaries_test.go` allows. It keeps three seams narrow:

- libghostty, and so cgo, is reached only through `internal/render`;
- `os/exec` is used only by the packages that own an external program: `encode` for FFmpeg, `record` for the recorded program, and `doctor`;
- the leaf packages (`theme`, `gifencode`, `terminfo`) import nothing else from this module.

A new package needs a rule, which is the moment to decide what it may depend on. Tools and examples are exempt.

## User guide site

The user guide is a [Hugo](https://gohugo.io) site in `docs/hugo`. It uses the [hugo-book](https://github.com/alex-shpak/hugo-book) theme as a git submodule and the Nimble brand styling shared with the gloss docs. The pages in `docs/hugo/content/guide` are for people using sigmamax. The files beside this one (`tapes.md`, `rendering.md`, and this page) are reference.

```sh
git submodule update --init   # once: fetch the theme
task docs:hugo:serve          # live preview (needs hugo, extended)
task docs:hugo:build          # build into docs/hugo/public
```

A new guide page needs a `title` and a `weight` in its front matter, and a link from `docs/hugo/content/guide/_index.md`. The `pages` workflow builds the site on pull requests that touch it and deploys it to GitHub Pages from `main`. Keep the guide to what a user needs, and put detail in the reference pages.

## Releases and Homebrew

As with gloss, GoReleaser publishes archives and checksums to GitHub Releases and a `sigmamax` cask to `NimbleMarkets/homebrew-tap`. Release builds run on macOS with the Command Line Tools SDK. The pinned Zig compiler builds four separate static libghostty-vt archives, one per platform, and cgo links the matching one into each macOS and Linux binary, amd64 and arm64. Target-specific pkg-config paths keep the host library from being linked into a cross-compiled binary. Dependency licenses ship with each archive.

With [GoReleaser](https://goreleaser.com/) 2.16.0 installed:

```sh
task release-check
task release           # macOS: build an unsigned snapshot in dist/, publish nothing
```

The release workflow packages pull requests that change the packaging, and it can be run by hand as a snapshot. It unpacks the archives on all four native platforms and checks the version, text GIFs, and Kitty PNG GIFs without installing libghostty. Pushing a `v*` tag publishes only after those checks and `task ci` pass. Snapshots and prereleases do not update Homebrew.

Set the same repository secrets as gloss before publishing:

- `HOMEBREW_TAP_GITHUB_TOKEN`: a token with contents write access to `NimbleMarkets/homebrew-tap`. GitHub supplies `GITHUB_TOKEN` for this repository's release.
- For signed and notarized macOS binaries: `MACOS_SIGN_P12`, `MACOS_SIGN_PASSWORD`, `MACOS_NOTARY_KEY`, `MACOS_NOTARY_KEY_ID`, and `MACOS_NOTARY_ISSUER_ID`. Signing is on when `MACOS_SIGN_P12` is set, and local snapshots skip it.

## Asciicast

`internal/asciicast` is a standard-library-only streaming v2/v3 reader and v3
writer. `internal/record` connects it to the PTY event stream and owns atomic
publication. Casts are event outputs alongside the frame encoders, not pixel
encoders. `RenderCast` reuses the renderer and encoders without starting a PTY.
Replay uses recorded time for cursor blinking, synchronized updates, and the
Kitty replacement grace period. The package boundary test enforces that the
codec cannot depend on rendering or launch programs.

Tests cover split UTF-8, v2/v3 clocks, large inline events, malformed input,
deterministic recording, cancellation, finalization after child failure,
unsupported tape edits, destination preservation, and PTY-to-cast-to-PNG Kitty
pixels. CLI tests cover both flag orders accepted by `render`.
