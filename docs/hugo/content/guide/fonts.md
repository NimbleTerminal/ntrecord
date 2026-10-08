---
title: "Using your own font"
weight: 45
---

# Using your own font

The built-in font is Go Mono. To use another, set `FontFamily` in a tape, or pass `-font` to `ntrecord render` and `ntrecord play`. (`ntrecord record` has no `-font` flag; record a `.cast` and `render` it with the font you want.)

```text
Set FontFamily "JetBrains Mono"       # an installed family, by name
Set FontFamily ./fonts/MyMono.ttf     # or a font file
```

```sh
ntrecord render session.cast -o session.gif -font "JetBrains Mono"
```

## By name or by file

| Value | What happens |
| --- | --- |
| `Go Mono` (or `monospace`) | The built-in font, with regular, bold, italic, and bold italic. |
| An installed family name | Matched ignoring case and spaces against the font's family or full name. Bold and italic faces are picked from the family; a style it lacks uses the regular face. |
| A `.ttf`, `.otf`, or `.ttc` path | That one file, used for every style, so bold and italic text will not look different. A `.ttc` collection uses its first face. |

A name is searched for in the standard macOS and Linux font folders: `/Library/Fonts`, `/System/Library/Fonts`, `~/Library/Fonts`, `/usr/share/fonts`, `/usr/local/share/fonts`, `~/.fonts`, `~/.local/share/fonts`, and `$XDG_DATA_HOME/fonts`. Font files larger than 32 MiB are skipped in that search; a path can be up to 64 MiB.

Prefer a path when the tape must work on other machines, since an installed font may not exist there. Relative paths are taken from the directory you ran ntrecord in, not from the tape's folder.

## Check it before recording

`ntrecord validate demo.tape` loads the font, so a wrong name or path is reported without recording anything:

```text
demo.tape: font family "Nope Font" not found; install it, use a .ttf/.otf path, or Set FontFamily "Go Mono"
```

Use a monospaced font. The grid is sized from the font, so the same `Columns` and `Rows` give a different picture size with a different font. To fix the picture's size instead, use `Width` and `Height` (see [How it looks](../appearance/)).

## Characters the font lacks

Text the chosen font cannot draw is looked up in fallback fonts before a replacement mark is shown. Which fallbacks exist, and how to control them (including `NTRECORD_FONT_PATH`), is described in [rendering](https://github.com/NimbleTerminal/ntrecord/blob/main/docs/rendering.md#fonts-and-text). CJK and other scripts need a suitable font installed, or one you name here.
