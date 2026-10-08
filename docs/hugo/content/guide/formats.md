---
title: "Output formats"
weight: 50
---

# Output formats

The file name's extension chooses the format, both for `ntrecord record -o` and for a tape's `Output`.

| Extension | What you get | Needs |
| --- | --- | --- |
| `.cast` | Asciicast v3 terminal events with inline Kitty graphics | nothing |
| `.gif` | An animated GIF | nothing |
| `.mp4` | H.264 video | [FFmpeg](../install/#for-mp4-and-webm) |
| `.webm` | VP9 video | [FFmpeg](../install/#for-mp4-and-webm) |
| `.txt`, `.ascii`, `.test` | The screen's text, saved at each step | nothing |
| a path ending in `/` | One numbered PNG per frame | nothing |

An unsupported extension is rejected before the session starts, so you do not lose a recording to a typo.

## GIF or video?

GIFs play anywhere and need no tools, but they use at most 256 colors (a theme's own colors are kept exact; anti-aliased text edges and photos are dithered) and get large for long or big recordings. ntrecord holds a GIF's distinct frames in memory, up to 512 MiB, so keep GIFs short.

Video has no 256-color limit and is far smaller, but it needs FFmpeg. It is silent, and small colored text can look slightly softer because video stores color at a lower resolution than brightness. Frames are streamed to FFmpeg as you record, so long videos do not use memory.

## Asciicast

A cast saves terminal events for later rendering, including inline Kitty image
uploads. Record once and render again at a different font size or playback speed:

```sh
ntrecord record -o session.cast
ntrecord render session.cast -o session.gif
ntrecord render session.cast -o session.mp4 -font-size 24 -speed 2
```

`render` reads asciicast v2/v3 without running the recorded commands. It uses the
recording's grid and initial colors; `-theme` overrides those colors. `-font`,
`-font-size`, `-fps`, and `-cursor-blink` control its appearance. It can also export
PNG frame directories and the final screen as text. For interactive terminal playback:

```sh
ntrecord play session.cast
ntrecord play session.cast -paused
```

Use Space to pause/resume, Left/Right to seek five seconds, R to restart,
Home/End to jump to the beginning/end, +/- to change speed, and Q or Ctrl-C to
quit. Playback holds on the last frame when finished.

The player displays ntrecord-rendered images in a Kitty-capable terminal, such as
Kitty or Ghostty. Run directly in the terminal; tmux passthrough is not supported.
Text is part of the rendered image. Resizing the host terminal fits the display
without changing the recorded grid. Backward seeks replay from the beginning to
reconstruct terminal and image state.

Casts follow the tape's realtime or deterministic clock. `Hide`, `ScrollUp`, and
`ScrollDown` are not supported with cast output and fail before recording starts.
Rendering rejects changes to the initial grid size. It preserves idle time and
ignores input/unknown events. Fonts and decorations are not stored in the cast.
Kitty images require direct transport and a player that can render Kitty graphics.

Keystrokes, environment variables, and command metadata are not collected, but
anything the program prints is recorded. A single imported event can be up to
64 MiB; invalid UTF-8 is replaced to keep the file valid JSON.

## Several outputs at once

Repeat `Output` and every file is written from the same session:

```text
Output demo.gif
Output demo.mp4
Output demo.txt
```

## Text for tests

`.txt` saves what the screen showed after each visible step, which makes a readable record to compare against. It reads the terminal's cells, so erased text and escape codes do not appear. Put a `Wait` before a step whose output arrives late, or it may be captured too early.

## Screenshots

`Screenshot name.png` saves the current frame (images and window decoration included) without ending the recording:

```text
Type "make"
Enter
Wait+Screen /done/
Screenshot result.png
```

## Safe to re-run

Files are written to a temporary file and moved into place only when encoding succeeds, so an encoding failure leaves an existing file intact. Cancellation or a child
process failure attempts to save the output captured so far. A frame directory must be empty or not exist yet; ntrecord will not delete frames that are already there.
