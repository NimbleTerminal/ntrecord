# Development

Building from source, running tests and CI, and cutting releases.

## Build

Requires Go 1.26+, [Task](https://taskfile.dev/), a C compiler/SDK, `make`,
`curl`, `tar` with xz support, and `shasum` or `sha256sum`. Supports macOS and
Linux on amd64/arm64. On macOS, install Command Line Tools with
`xcode-select --install` if needed, and Go/Task with `brew install go go-task`.
On Debian/Ubuntu, native prerequisites are
`sudo apt-get install build-essential curl xz-utils ca-certificates`; install
Go and Task separately.

**You do not need Zig or pkg-config installed globally.** Start with:

```sh
task build
task ci
task run -- examples/kitty.tape
task demo:text
```

The first build downloads SHA-256-verified **Zig 0.16.0** and **pkgconf 2.5.1**,
builds pkgconf and the pinned libghostty-vt, and writes `bin/sigmamax`. Everything
native is stored under ignored `.deps/`; no sudo, Homebrew Zig version, shell
profile changes, or `/tmp` installation is required. The first build needs
network access and can take a few minutes. Later builds reuse these tools.
`task deps` explicitly runs native setup (and forwards Zig build flags after
`--`); ordinary build/test/install tasks run setup when the library is missing.

Task sets `CGO_ENABLED=1`, uses `scripts/pkg-config.sh` to find the local
pkgconf, and adds the local native library to `PKG_CONFIG_PATH`, preserving
any existing search path. Without Task:

```sh
sh scripts/build-libghostty.sh
export PKG_CONFIG="$PWD/scripts/pkg-config.sh"
export PKG_CONFIG_PATH="$PWD/.deps/libghostty-vt/share/pkgconfig${PKG_CONFIG_PATH:+:$PKG_CONFIG_PATH}"
"$PKG_CONFIG" --static --cflags --libs libghostty-vt-static
CGO_ENABLED=1 go build -o bin/sigmamax .
go run . examples/kitty.tape
go run . examples/text.tape
```

Native setup downloads and builds Ghostty commit
`0e0ff282f72c92d930d41f4703547d3bc9618ba2` locally under `.deps/`. The Go binding
is pinned in go.mod to commit `183d2bc56a8a` (pseudo-version
`v0.0.0-20261004135024-183d2bc56a8a`). Its API is unstable; update these together, then run `task terminfo` to
regenerate the embedded `xterm-ghostty` terminfo entry from the same Ghostty
revision (needs `tic` from ncurses; a test fails if the entry and its source
diverge).
An existing matching installation can be used by setting `PKG_CONFIG_PATH` to
its `share/pkgconfig` directory and `PKG_CONFIG` to your pkg-config executable.
`ZIG=/absolute/path/to/zig` overrides the downloaded compiler; overrides must
report version 0.16.0. Tool versions, source revision, and archive checksums are
pinned in `scripts/native-common.sh` and `scripts/bootstrap-tools.sh`.

```sh
task clean       # remove bin/, dist/, .task/; keep dependencies and recordings
task build       # reuse the local native dependencies
task clean:all   # also remove all of .deps/ (downloads, tools, libraries, caches)
task build       # reproduce the first build from scratch
```

Clean tasks only affect this checkout. They never uninstall a Go-installed
binary, delete recordings, or remove an external `GHOSTTY_PREFIX`.

The unmodified `go.mitchellh.com/libghostty` binding uses **cgo and static
libghostty-vt linking** by default, through `libghostty-vt-static.pc`. There is
no purego rebinding. The resulting binary needs no Ghostty shared library,
external font file, or GIF encoder. GIFs use Go's encoder and a 256-color palette that reserves the theme's
background, foreground, cursor, ANSI, margin and window-bar colors, so they stay
exact and flat; other colors (antialiasing, images) are dithered and quantized.

For a Linux amd64 cross-build from a supported Zig host:

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

## Tests

```sh
task test
task ci
task demos
task test:video  # requires ffmpeg and ffprobe; never silently skips video tests
task demo:video
task demo:language
```

`task` defaults to `task build`; `task list` shows available commands. The
workflow also provides `run`, `record`, `install`, `fmt`, `fmt-check`, `go-tidy`,
`test-race`, and `vet`. Arguments after `--` reach the CLI or test command, for
example `task test -- -run TestPNGFixture -v`. Builds use `-mod=readonly` and
Go's cache so native compiler settings and workspace dependencies are checked.
`task ci` runs formatting, module tidy/verification, race tests, vet, and build,
matching GitHub Actions. CI starts without `.deps/`, runs `task build`, `task ci`,
`task demos`, and `task demo:video` on macOS/Linux, then checks `task clean` followed by a rebuild.
It uses the same pinned tool bootstrap as local development. This workflow
installs FFmpeg, requires video tests with `SIGMAMAX_REQUIRE_FFMPEG=1`, and uploads
demo GIFs/MP4s/WebMs plus the language screenshot/text. The Makefile delegates to Task.

Set `GHOSTTY_PREFIX=/absolute/path` to use another native install prefix, or
set `PKG_CONFIG_PATH` to its pkg-config directory. `PKG_CONFIG` and `ZIG` can
select tool executables. For example, `task deps -- -Dtarget=x86_64-linux-gnu`
forwards a target to the native build (use a separate `GHOSTTY_PREFIX` when
cross-compiling, as shown above).

Tests feed a base64 PNG APC fixture into `VTWrite` and assert pixels inside its
placement, then check raw formats, replies, cropping, offsets, alpha, z-index,
scrolling, deletion, and copied-buffer lifetime. Integration tests build the
Kitty demo, run it through both a tape and live command recording, decode the
GIFs and videos, and assert image pixels for ordinary and Unicode placements. A text-only tape asserts colored glyphs, and
exit/cancellation tests verify that recordings are finalized. Video tests also
check codec, dimensions, padding, color, timing, failed-encode cleanup, and
preservation of existing outputs. Locally, tests needing FFmpeg/ffprobe skip if
the tools are missing; `task test:video` and CI require them. CI runs on Linux/macOS.

Compatibility tests parse the pinned upstream VHS command/settings examples
and the all-commands fixture. End-to-end tests exercise real PTY input, waits,
clipboard paste, screenshots, text checkpoints, and multiple outputs. Upstream
fixtures and theme data retain Charm's MIT license and theme author metadata.

## Package boundaries

`internal/boundaries` is a test that fails when a package imports more than its
rule allows (`rules` in `boundaries_test.go`). It keeps three seams narrow:
libghostty, and so cgo, is reached only through `internal/render`; `os/exec` is
used only by the packages that own an external program (`encode` for FFmpeg,
`record` for the recorded program, `doctor`); and the leaf packages (`theme`,
`gifencode`, `terminfo`) import nothing of ours. A new package needs a rule,
which is the moment to decide what it may depend on. Tools and examples are
exempt.

## User guide site

The user guide is a [Hugo](https://gohugo.io) site in `docs/hugo`, using the
[hugo-book](https://github.com/alex-shpak/hugo-book) theme as a git submodule and
the Nimble brand styling shared with the gloss docs. The pages in
`docs/hugo/content/guide` are written for people using sigmamax; the files
beside this one (`tapes.md`, `rendering.md`, this page) are the reference.

```sh
git submodule update --init   # once: fetch the theme
task docs:hugo:serve          # live preview (needs hugo, extended)
task docs:hugo:build          # build into docs/hugo/public
```

A new guide page needs a `title` and `weight` in its front matter and a link
from `docs/hugo/content/guide/_index.md`. The `pages` workflow builds the site on
pull requests that touch it and deploys it to GitHub Pages from `main`.
Keep the guide to what a user needs; put detail in the reference pages and link
to it.

## Releases and Homebrew

Like gloss, GoReleaser publishes archives/checksums to GitHub Releases and a
`sigmamax` cask to `NimbleMarkets/homebrew-tap`. Release builds run on macOS with
the Command Line Tools SDK. The pinned Zig compiler builds four separate static
libghostty-vt archives; cgo then links the matching archive into each macOS/Linux,
amd64/arm64 binary. Target-specific pkg-config paths prevent linking the host
library into a cross-compiled binary. Dependency licenses accompany each archive.

With [GoReleaser](https://goreleaser.com/) 2.16.0 installed:

```sh
task release-check
task release           # macOS: build an unsigned snapshot in dist/, publish nothing
```

The release workflow packages pull requests that change the packaging, and can
also be run manually as a snapshot. It extracts the archives on all four native
platforms and checks the version, text GIFs, and Kitty PNG GIFs without installing
libghostty. A pushed `v*` tag publishes only after those checks and `task ci` pass.
Snapshots and prereleases do not update Homebrew.

Configure the same repository secrets as gloss before publishing:

- `HOMEBREW_TAP_GITHUB_TOKEN`: token with contents write access to
  `NimbleMarkets/homebrew-tap`. GitHub supplies `GITHUB_TOKEN` for this repo's release.
- For signed/notarized macOS binaries: `MACOS_SIGN_P12`, `MACOS_SIGN_PASSWORD`,
  `MACOS_NOTARY_KEY`, `MACOS_NOTARY_KEY_ID`, and `MACOS_NOTARY_ISSUER_ID`.
  Signing is enabled when `MACOS_SIGN_P12` is set. Local snapshots skip it.
