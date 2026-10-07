---
title: "Recording images"
weight: 60
---

# Recording images

sigmamax draws [Kitty graphics](https://sw.kovidgoyal.net/kitty/graphics-protocol/), so a program that shows pictures in a Kitty-capable terminal (charts, shader demos, image viewers) can be recorded to a GIF or video with the pictures in it.

```sh
sigmamax record -o plot.gif -cols 100 -rows 30 -- ./my-chart-program
```

The repository's `examples/kitty.tape` records a small program that shows an image; run it to see the result.

## What works

- PNG, RGB, and RGBA images sent directly in the terminal's data stream, shown at their own size or fitted to a number of cells, cropped, offset, and transparent.
- Images that sit behind text, behind cell backgrounds, or in front of everything.
- Pictures built from Unicode placeholder characters, as charting libraries such as [ntcharts](https://github.com/NimbleMarkets/ntcharts) draw them. They scroll and erase with the cells they occupy.
- The questions programs ask to find out whether graphics are available, which sigmamax answers.

## What does not

Animation frames, relative placements, and images sent through shared memory or files are not supported. Programs that offer a choice should be told to send the image data directly. An image sent over a shared-memory or file transport will not be shown.

## Make the program use graphics

Many programs decide whether to draw images by looking at the environment, and sigmamax does not look like Kitty or Ghostty to them. If nothing shows up, check the program's own setting for graphics, and these:

- **`NO_COLOR` must not be set.** Placeholder images carry their identity in text colors, so a program that turns colors off cannot show them. Run it with `env -u NO_COLOR`.
- **Turn off tmux passthrough** if the program has such a setting; there is no tmux between it and the recorder.
- **Force the capability.** Some programs have a switch for "assume Kitty graphics". For ntcharts' shader example:

```sh
env -u NO_COLOR NTCHARTS_KITTY=supported NTCHARTS_TMUX_PASSTHROUGH=false \
  sigmamax record -o shaders.gif -cols 80 -rows 24 -- \
  go run . -medium=direct -fullscreen -fps=20 -duration=8s
```

## Frames that flicker

When a program replaces an image by sending it in pieces, the terminal removes the old picture as the first piece arrives. sigmamax holds the last complete frame for up to a quarter of a second while it waits for the new one, so such updates do not show blank frames. A picture deleted on purpose while its placeholder cells stay on screen is shown for that same short extra time.

Programs that update the screen in synchronized batches are also respected: a half-drawn frame is held out of the recording.
