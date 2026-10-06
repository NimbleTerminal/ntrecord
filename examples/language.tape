Source examples/language-settings.tape
Output examples/language.gif
Output examples/language.txt
Require sh
Env DEMO_MESSAGE "VHS language, libghostty terminal, Go pixels."

Hide
Copy `printf '\033[2J\033[H\033[?25l'; printf 'VHS tapes, native pixels\n'; printf '%s\n' "$DEMO_MESSAGE"`
Paste
Enter
Wait+Screen /^VHS tapes, native pixels/
Show
Sleep 0.5
Type "printf 'Keyboard editing worksZ"
Backspace
Type `\n'`
Enter
Sleep 0.5
Type "discard this input"
Ctrl+U
Type@10ms `printf '\nReady to record.\n'`
Enter
Wait+Screen /Ready to record\./
Sleep 1s
Screenshot examples/language.png
