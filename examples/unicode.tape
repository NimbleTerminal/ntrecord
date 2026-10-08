# Portable: the symbols and emoji below use bundled fonts.
Output examples/unicode.gif
Output examples/unicode.cast
Set Shell /bin/sh
Set FontSize 24
Set Width 960
Set Height 390
Set TypingSpeed 0ms
Type `printf '\033[2J\033[H\033[?25lUnicode: é ä ñ  ♞ ⌘\n┌─────────┬─────────┐ ╭─────────╮ ╔════════╗\n│ borders │ no gaps │ │ rounded │ ║ double ║\n└─────────┴─────────┘ ╰─────────╯ ╚════════╝\nBraille: ⠁⠂⠄⠈⠐⠠⡀⢀ ⣿ ⢕⡪\nBlocks: ▁▂▃▄▅▆▇█ ▉▊▋▌▍▎▏ ░▒▓ ▖▗▘▙▚▛▜▝▞▟\nEmoji: 😀 👍🏽 👨‍👩‍👧‍👦 🇯🇵 1️⃣ ❤️ 👩🏽‍💻 END\n'; sleep 10`
Enter
Sleep 2s
Screenshot examples/unicode.png
