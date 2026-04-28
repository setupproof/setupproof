Output docs/demo/setupproof.gif

Require git
Require go
Require bash

Set Shell "bash"
Set FontFamily "Menlo"
Set FontSize 18
Set LetterSpacing 0
Set LineHeight 1.12
Set Width 1040
Set Height 640
Set Padding 16
Set Margin 12
Set MarginFill "#f6f8fa"
Set BorderRadius 8
Set WindowBar Colorful
Set Framerate 24
Set TypingSpeed 26ms

Hide
Type@0ms "source docs/demo/prepare-recording.sh && clear"
Enter
Show

Sleep 400ms
Type "sed -n '1,10p' README.md"
Enter
Sleep 900ms

Type "setupproof review README.md"
Enter
Sleep 1300ms

Type "setupproof --require-blocks --no-color --no-glyphs README.md"
Enter
Sleep 1800ms
