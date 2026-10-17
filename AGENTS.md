# AGENTS.md

This file provides guidance to AI coding agents working with code in this repository.

ntrecord is a headless terminal recorder: it runs a program on a real PTY, feeds the output to libghostty-vt, and paints every frame itself in Go (no browser, window, or screen capture). It reads VHS `.tape` scripts and writes GIF, MP4/WebM (via FFmpeg), asciicast, text, and PNG. Module: `nimbleterminal.dev/ntrecord`.

## Commands

Everything goes through [Task](https://taskfile.dev) (the Makefile only calls Task). The first build downloads pinned Zig and pkgconf and builds libghostty-vt into `.deps/` (network, a few minutes); later builds reuse it. Don't install Zig or pkg-config yourself.

```sh
task build                                  # writes ./ntrecord
task test                                   # go test ./...
task test -- -run TestName -v               # one test; flags after -- go before ./... in go test
task ci                                     # what CI runs: fmt-check, tidy/verify, race tests, test:library, vet, build
task fmt                                    # gofmt over the listed source dirs
task test:video                             # requires ffmpeg/ffprobe; video tests otherwise skip locally
task test:golden                            # renderer golden-frame hashes (see below)
task demos                                  # record example GIFs
```

To test one package, run `go` directly. It needs cgo and the local pkg-config, which Task otherwise sets: `PKG_CONFIG=$PWD/scripts/pkg-config.sh PKG_CONFIG_PATH=$PWD/.deps/libghostty-vt/share/pkgconfig CGO_ENABLED=1 go test -run TestName ./internal/tape`.

## Architecture

The pipeline for a tape: `internal/tape` parses the VHS language into a `Config` plus actions → `internal/record` starts the PTY (`pty.go`), drives actions, serializes VT access, and samples frames → `render` (libghostty terminal + Go painter) produces RGBA frames → `internal/encode` writes outputs (GIF via `gifencode`, video by streaming RGBA to FFmpeg, text, PNG frames), while `asciicast` writes `.cast` events alongside. `ntrecord record` uses the same path without a tape. `ntrecord render` (`internal/record/render_cast.go`) replays a cast through the renderer and encoders without a PTY; `ntrecord play` uses `internal/playback` (reconstructs cast state) and `internal/player` (draws it in a Kitty-capable terminal).

- `main.go` is the whole CLI (`flag` subcommands); `skill.go` embeds `skills/ntrecord/SKILL.md` and installs it for agents.
- `render`, `gifencode`, and `asciicast` are **public library packages**; `task test:library` builds them from an external module, with and without cgo. Keep their APIs deliberate.
- `internal/renderconfig` adapts tape/CLI settings to the public `render` API.
- Realtime mode samples on wall time; deterministic mode (`-mode deterministic`) uses a virtual clock and waits for the program to settle after each step (`settleQuiet`/`settleLimit` in `internal/record/record.go`).
- The recorded program's environment is built in `environment()` in `internal/record/record.go`: `TERM=xterm-ghostty` (embedded terminfo from `internal/terminfo`, installed to a private dir per recording), `TERM_PROGRAM=ntrecord`, and `NO_COLOR` removed.

### Package boundaries (enforced by a test)

`internal/boundaries/boundaries_test.go` holds an allow-list of what each package may import. A new package, or a new internal import, fails `go test` until its rule is added there. Invariants:

- Only `render` imports libghostty, so cgo stays behind one package.
- Only packages owning an external program use `os/exec`: `internal/encode` (FFmpeg), `internal/record` (the recorded program), `internal/doctor`.
- Leaf packages (`internal/theme`, `gifencode`, `internal/terminfo`, `asciicast`, `skills`) import nothing else from this module.

### Deterministic rendering

With `NTRECORD_FONT_PATH=` (bundled fonts only), a frame must be a pure function of terminal state, identical on macOS/Linux × amd64/arm64. `render/golden_frames_test.go` compares frame SHA-256s against `testdata/golden/frames.sha256` on four CI platforms. After an intended visual change: `NTRECORD_UPDATE_GOLDEN=1 task test:golden`, and inspect PNGs from `NTRECORD_GOLDEN_DUMP=/tmp/frames task test:golden` before committing hashes.

In the render path, wrap products that feed a sum as `float32(a*b)`: arm64 fuses multiply-add and amd64 does not, which changed glyph pixels (see `render/face.go`, `task test:fma`).

## Pins and generated files

- libghostty is pinned twice and must move together: the Ghostty commit in `scripts/native-common.sh` and the `go.mitchellh.com/libghostty` pseudo-version in `go.mod`. After a bump, run `task terminfo` (needs `tic`); a test fails if the embedded terminfo and its source disagree.
- `internal/tape/testdata/vhs` is the unmodified upstream VHS tape corpus, checked against a manifest of git blob SHAs; don't edit those files. Rejections are listed in `corpus_test.go`.

## Docs and skill

- `docs/tapes.md`, `docs/rendering.md`, `docs/development.md` are reference; `docs/hugo/content/guide/` is the user guide (Hugo, hugo-book theme submodule). A new guide page needs `title`, `weight`, and a link from `guide/_index.md`.
- `skills/ntrecord/SKILL.md` is the agent skill, embedded in the binary. When CLI flags, tape behavior, defaults, or error messages change, update it too — it quotes error text verbatim.

- `CHANGELOG.md`: add user-visible changes under `Unreleased`, briefly and from the user's side. Releasing is in `docs/development.md` ("Releases and Homebrew").

Commits use conventional prefixes (`feat:`, `fix:`, `docs:`, `build:`, `test:`, `ci:`).
