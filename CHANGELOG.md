# `ntrecord` CHANGELOG

## Unreleased

- Update the terminal engine to Ghostty 1.4.0, including upstream security fixes
  for Kitty graphics and scrollback handling. Native builds now use Zig 0.17.0.
- OSC 7501 capability replies now advertise supported status states and kinds.

- The Homebrew cask uses the current macOS requirement syntax, avoiding a
  deprecation warning during Homebrew upgrades.
- `ntrecord skill install` prints each path once on a terminal; the bare paths
  go to stdout only when it is redirected, for scripts.

## `v0.2.0` (2026-10-17)

### New

- `ntrecord skill show` prints the agent skill built into the binary.
  `ntrecord skill install [FOLDER]` installs it for Claude Code, Kimi Code, and
  the shared `~/.agents/skills` folder, or under FOLDER alone. Run it again
  after an upgrade to refresh the skill.

### Changed

- The recorded program no longer inherits `NO_COLOR`, so colors are recorded
  even when your shell sets it. `ntrecord doctor` no longer warns about it.
- When a recording ends, background jobs the shell started end with it.
- A key the tape types after the program has exited now fails the tape instead
  of being dropped.
- Deterministic mode holds synchronized updates and Kitty image replacements on
  the recorded clock, as realtime does on the wall clock. `Hide` records the
  screen it closes on if it changed since the last frame.
- `ntrecord render` holds a cast's last frame for a full frame period, so a
  change near the end of a cast is no longer dropped.

### Fixed

- Underlines no longer paint into the row below at small font sizes or line
  heights.
- A font family's Bold face is chosen over Semibold or ExtraBold, and Regular
  over Light or Medium.
- A long `Sleep` in deterministic mode no longer times out a healthy FFmpeg.
- A slow-starting FFmpeg keeps the video when a recording is cancelled.
- Ctrl-C ends a recording even while the terminal it forwards to is blocked.
- Output already read from the program is no longer lost when a step or the
  end of the tape hits its time limit.
- `Wait` and `WaitPattern` start a regular expression only as a command or
  setting, so `Type WaitPattern /usr/bin/env` types what it says.
- `Ctrl+@` accepts a timing, as in `Ctrl+@100ms`.
- `ntrecord validate --help` and `ntrecord doctor --help` print help.
- Escape in `ntrecord play` no longer swallows the next key.
- The asciicast writer rejects a timestamp earlier than output it is still
  holding.
- `ntrecord skill install` replaces a broken link at the skill folder, keeps a
  link to a real folder, treats `auto` as a folder name, and refuses `~` when
  no home folder is set.

### Development

- `task build` writes `./ntrecord` instead of `bin/ntrecord`.
- `AGENTS.md` describes the repository for coding agents.
