---
title: "Recording images"
weight: 60
---

# Recording images

ntrecord draws [Kitty graphics](https://sw.kovidgoyal.net/kitty/graphics-protocol/), so a program that shows pictures in a Kitty-capable terminal (charts, shader demos, image viewers) can be recorded to a GIF or video with the pictures in it.

```sh
ntrecord record -o plot.gif -cols 100 -rows 30 -- ./my-chart-program
```

The repository's `examples/kitty.tape` records a small program that shows an image; run it to see the result.

## What works

- PNG, RGB, and RGBA images sent directly in the terminal's data stream, shown at their own size or fitted to a number of cells, cropped, offset, and transparent.
- Images that sit behind text, behind cell backgrounds, or in front of everything.
- Pictures built from Unicode placeholder characters, as charting libraries such as [ntcharts](https://github.com/NimbleMarkets/ntcharts) draw them. They scroll and erase with the cells they occupy.
- The questions programs ask to find out whether graphics are available, which ntrecord answers.

## What does not

Animation frames, relative placements, and images sent through shared memory or files are not supported. If a program lets you choose how it sends images, choose direct transfer; an image sent through shared memory or a file will not be shown.

Braille, box drawing, block characters, symbols, and color emoji are drawn without any font setup, so charts that use them render. CJK and other scripts need an installed font; pick one with `Set FontFamily` (see [Using your own font](../fonts/)).

## Make the program use graphics

ntrecord tells the program it is running in Ghostty (`TERM=xterm-ghostty`), which is how most programs learn that a terminal can draw images, so most need no setup. If nothing shows up:

- **Don't set `NO_COLOR` with a tape's `Env`.** Placeholder images carry their identity in text colors, so a program that turns colors off cannot show them. ntrecord already removes your own `NO_COLOR` from the program's environment.
- **Ask for direct image transfer** if the program has a choice of how to send images. For ntcharts' shader example:

```sh
ntrecord record -o shaders.gif -cols 80 -rows 24 -- \
  go run . -medium=direct -fullscreen -fps=20 -duration=8s
```

- **Check the program's own graphics setting.** Some programs have a switch to force or disable Kitty graphics. If one only looks at `TERM_PROGRAM`, which is `ntrecord`, set that switch.
- **A tape can change the terminal name** with `Env TERM xterm-256color` for a program that misbehaves under `xterm-ghostty`.

## Frames that flicker

When a program replaces an image by sending it in pieces, the terminal removes the old picture as the first piece arrives. ntrecord holds the last complete frame for up to a quarter of a second while it waits for the new one, so such updates do not show blank frames. A picture deleted on purpose while its placeholder cells stay on screen is shown for that same short extra time.

Programs that update the screen in synchronized batches are also respected: a half-drawn frame is held out of the recording.
