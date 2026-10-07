---
title: "When something goes wrong"
weight: 70
---

# When something goes wrong

Two commands answer most questions before you read further:

```sh
sigmamax doctor                 # can this machine record? engine, fonts, PTY, ffmpeg
sigmamax validate demo.tape     # is this tape sound? nothing is recorded
```

`doctor` prints one line per check and says plainly what is missing (an absent FFmpeg is a warning, since only MP4 and WebM need it). `validate` reads the tape the way a recording would and reports every problem at once: syntax errors with their line and column, an unknown theme or font, a `Require` program that is not installed, an `Output` that needs FFmpeg.

## A tape stops with "Wait timed out"

The error names the pattern sigmamax was waiting for and shows the last text on screen. Compare the two:

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

sigmamax starts the shell with a simple prompt, but a shell's startup files can replace it. Use the default `Set Shell /bin/sh`, or `Hide` the setup and `Show` once the prompt is as you want it.

## A character shows as a replacement mark or is missing

The built-in font, Go Mono, has no symbols for icon fonts, CJK text, or emoji, and sigmamax does not substitute another font. Use `Set FontFamily` with a font that has them (an installed family, or a `.ttf`/`.otf` file).

## The GIF is huge

Make the grid smaller (`Set Columns`, `Set Rows`, `Set FontSize`), shorten the session, lower `Set Framerate`, or write an `.mp4` or `.webm` instead. A GIF is limited to 512 MiB of distinct frames, and sigmamax stops with an error rather than running out of memory.

## Paths are wrong

Shell commands, outputs, fonts, screenshots, and `Source` files are found relative to the directory you ran sigmamax from, including inside sourced tapes. Run a tape from the directory it expects, or use absolute paths.

## The recording is cut off at the end

After a tape's last command, sigmamax waits only for the output to go quiet (100 ms, up to one second). If a command needs longer, finish the tape with `Wait` or a `Sleep`.

## Still stuck

Open an [issue](https://github.com/NimbleMarkets/sigmamax/issues) with the tape or command, the error text, and your sigmamax version (`sigmamax --version`).
