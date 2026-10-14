# Render with: vhs docs/demo.tape   (or: make demo-gif)
Output docs/demo.gif

Require gtui

Set Shell "bash"
Set FontSize 16
Set Width 1200
Set Height 620
Set Theme "Catppuccin Mocha"
Set TypingSpeed 60ms

Hide
Type "export SHELL=/bin/bash PS1='$ '" Enter
Type "clear" Enter
Show

Type "gtui --demo" Sleep 300ms Enter
Sleep 1.5s
Enter Sleep 1.5s
Enter Sleep 3s
Type "o" Sleep 2s
Enter Sleep 2s
Type "v" Sleep 2s
Escape Sleep 600ms
Type "y" Sleep 1.5s
Type "t" Sleep 1.5s
Escape Sleep 800ms
Type "/" Sleep 300ms Type "web" Sleep 1s
Enter Sleep 500ms
Escape Sleep 500ms
Escape Sleep 800ms
Type "A" Sleep 1.5s
Type "!" Sleep 2s
Type "!" Sleep 800ms
Type "ooo" Sleep 800ms
Escape Sleep 800ms
Tab Sleep 1.5s
Type "?" Sleep 2.5s
Escape Sleep 800ms
Type "q"
Sleep 1s
