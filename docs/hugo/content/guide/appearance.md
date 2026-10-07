---
title: "How it looks"
weight: 40
---

# How it looks

Appearance is set at the top of a tape with `Set`. Everything has a default, so set only what you want to change.

```text
Output demo.gif
Set Theme "Catppuccin Mocha"
Set FontSize 20
Set Columns 70
Set Rows 14
Set Padding 20
Set Margin 30
Set MarginFill "#1e1e2e"
Set WindowBar Colorful
Set BorderRadius 10
```

## Colors

`Set Theme "Name"` picks one of 348 bundled themes (the [VHS](https://github.com/charmbracelet/vhs) set, such as `Dracula`, `Nord`, or `Catppuccin Mocha`). An unknown name is an error that tells you so. Or give your own colors as a JSON object:

```text
Set Theme { "background": "#101018", "foreground": "#e0e0f0", "red": "#ff6b6b", "green": "#7bd88f" }
```

A theme can also set `cursor` and the sixteen ANSI colors (`black` through `brightWhite`). In a GIF, a theme's background, foreground, and ANSI colors stay exact and flat rather than speckled.

## Text

| Setting | Does | Default |
| --- | --- | --- |
| `FontSize` | Text size in pixels, 6 to 96 | 18 |
| `FontFamily` | `Go Mono` (built in), an installed font's name, or a `.ttf`/`.otf` path | `Go Mono` |
| `LineHeight` | Line spacing multiplier | 1 |
| `LetterSpacing` | Extra pixels between letters | 0 |
| `CursorBlink` | `true` or `false` | `false` |

To use a font you have installed, name its family (`Set FontFamily "JetBrains Mono"`) or point to the file (`Set FontFamily ./fonts/Mono.ttf`). There is no fallback font: a character the font lacks is drawn as a replacement glyph, and color emoji are not supported.

## Size

Set the terminal's grid, or the picture's final pixel size, but not both for the same direction:

```text
Set Columns 80        # or Cols; with Rows: the grid
Set Rows 24

Set Width 1200        # or the output's total size in pixels, decorations included
Set Height 630
```

## Frame around the terminal

| Setting | Does |
| --- | --- |
| `Padding` | Pixels of space between the text and the window's edge |
| `Margin` | Pixels around the window |
| `MarginFill` | Color of the margin, as `#RRGGBB` (by default the theme background) |
| `WindowBar` | A title bar: `Colorful`, `ColorfulRight`, `Rings`, `RingsRight`, or `None` |
| `WindowBarSize` | Height of the bar in pixels (30) |
| `BorderRadius` | Rounded corners, in pixels |

## Speed and looping

```text
Set Framerate 20         # frames per second, 1 to 120
Set PlaybackSpeed 2      # play the recording twice as fast
Set TypingSpeed 40ms     # delay between typed keys
Set LoopOffset 50%       # a GIF starts halfway through its loop
```

`PlaybackSpeed` speeds up recorded time but not the waiting inside the session, so `Wait` still takes as long as the command does. `LoopOffset` applies to GIFs only.
