---
title: "Output formats"
weight: 50
---

# Output formats

The file name's extension chooses the format, both for `sigmamax record -o` and for a tape's `Output`.

| Extension | What you get | Needs |
| --- | --- | --- |
| `.gif` | An animated GIF | nothing |
| `.mp4` | H.264 video | [FFmpeg](../install/#for-mp4-and-webm) |
| `.webm` | VP9 video | [FFmpeg](../install/#for-mp4-and-webm) |
| `.txt`, `.ascii`, `.test` | The screen's text, saved at each step | nothing |
| a path ending in `/` | One numbered PNG per frame | nothing |

An unsupported extension is rejected before the session starts, so you do not lose a recording to a typo.

## GIF or video?

GIFs play anywhere and need no tools, but they use at most 256 colors (a theme's own colors are kept exact; anti-aliased text edges and photos are dithered) and get large for long or big recordings. sigmamax holds a GIF's distinct frames in memory, up to 512 MiB, so keep GIFs short.

Video keeps full color and is far smaller, but it needs FFmpeg. It is silent, and small colored text can look a touch softer because video stores color at lower resolution than brightness. Video is streamed to FFmpeg as you record, so long videos do not use memory.

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

Files are written to a temporary file and moved into place only when encoding succeeds, so a failed or cancelled recording never overwrites a good one. A frame directory must be empty or not exist yet; sigmamax will not delete frames that are already there.
