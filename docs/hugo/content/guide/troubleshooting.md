---
title: "When something goes wrong"
weight: 70
---

# When something goes wrong

Two commands answer most questions before you read further:

```sh
ntrecord doctor                 # can this machine record? engine, fonts, PTY, ffmpeg
ntrecord validate demo.tape     # is this tape sound? nothing is recorded
```

`doctor` prints one line per check and says plainly what is missing (an absent FFmpeg is a warning, since only MP4 and WebM need it). `validate` reads the tape the way a recording would and reports every problem at once: syntax errors with their line and column, an unknown theme or font, a `Require` program that is not installed, an `Output` that needs FFmpeg.

## A tape stops with "Wait timed out"

The error names the pattern ntrecord was waiting for and shows the last text on screen. Compare the two:

- The pattern may not match the way the text is written. Patterns are [Go regular expressions](https://pkg.go.dev/regexp/syntax) matched against what is shown on screen, not the raw program output.
- A plain `Wait` looks at the line the cursor is on, usually the prompt, and the default pattern `[$>#]$` expects one ending in `$`, `>`, or `#`. If your prompt ends differently, set it: `Set WaitPattern /%$/`. To wait for a command's *output*, name it: `Wait+Screen /your pattern/`.
- A slow command needs a longer limit: `Wait+Screen@90s /done/`, or `Set WaitTimeout 90s` for all waits.

## ".mp4 output requires ffmpeg"

Install [FFmpeg](../install/#for-mp4-and-webm). If it is installed, your build may lack the encoder: `.mp4` needs `libx264` and `.webm` needs `libvpx-vp9`. Check with `ffmpeg -encoders`.

## "recording has no visible frames"

Everything was hidden. A tape that ends while still hidden writes nothing; add `Show` before the end. For a `record` session, make sure you did not start with `Hide`.

## "Set … must precede actions"

Settings, `Output`, `Env`, and `Require` have to come before the first `Type`, key, `Sleep`, or other action. Move them to the top. Only `Set TypingSpeed` may change mid-tape.

## The picture shows my own shell prompt, not `$`

ntrecord starts the shell with a simple prompt, but a shell's startup files can replace it. Use the default `Set Shell /bin/sh`, or `Hide` the setup and `Show` once the prompt is as you want it.

## A character shows as a replacement mark or is missing

Go Mono is the primary font. Symbols, braille, box drawing, and color emoji come from built-in fallbacks, and other text from installed fonts that cover it. Icon-font (Nerd Font) symbols, CJK, and other scripts show a replacement mark unless a font that has them is installed. Name one with `Set FontFamily` (an installed family, or a `.ttf`/`.otf`/`.ttc` file); see [Using your own font](../fonts/). Fallback coverage can differ between machines; `NTRECORD_FONT_PATH=` (empty) restricts it to the bundled fonts.

## A font name is "not found"

`font family "X" not found` means no installed font has that family or full name in the standard font folders. Check the spelling, or point `Set FontFamily` at the `.ttf`/`.otf` file. `ntrecord validate` reports this without recording. See [Using your own font](../fonts/).

## ".cast output does not support Hide"

A cast stores terminal events, so it cannot leave out hidden output or reproduce viewport scrolling. Tapes that use `Hide`, `ScrollUp`, or `ScrollDown` cannot have a `.cast` output. Remove the `.cast` `Output` (a GIF or video can still use `Hide`), or remove those commands.

## "play needs an interactive terminal"

`ntrecord play` shows frames through Kitty graphics, so it needs a real terminal such as Kitty or Ghostty, run directly (not through tmux). To get a file instead, use `ntrecord render session.cast -o session.gif`.

## "Set Columns conflicts with Set Width"

Choose the picture size in pixels (`Width`, `Height`) or the grid (`Columns`, `Rows`) for each direction, not both. See [How it looks](../appearance/).

## `Wait+Status` times out

The program must emit OSC 7501 status; one that does not will never match, so wait on screen text instead. The timeout message summarizes the status records ntrecord has seen: compare their state, `app`, `id`, and `kind` with your filters. Status already received counts, so a stale `done` can satisfy a later wait; use a distinct `id` or have the program clear it. See [Writing tapes](../tapes/#waiting).

## The GIF is huge

Make the grid smaller (`Set Columns`, `Set Rows`, `Set FontSize`), shorten the session, lower `Set Framerate`, or write an `.mp4` or `.webm` instead. A GIF is limited to 512 MiB of distinct frames, and ntrecord stops with an error rather than running out of memory.

## Paths are wrong

Shell commands, outputs, fonts, screenshots, and `Source` files are found relative to the directory you ran ntrecord from, including inside sourced tapes. Run a tape from the directory it expects, or use absolute paths.

## The recording is cut off at the end

After a tape's last command, ntrecord waits only for the output to go quiet (100 ms, up to one second). If a command needs longer, finish the tape with `Wait` or a `Sleep`.

## Still stuck

Open an [issue](https://github.com/NimbleTerminal/ntrecord/issues) with the tape or command, the error text, and your ntrecord version (`ntrecord --version`).
