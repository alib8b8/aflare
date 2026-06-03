# llm-box Demo Recording Script
# Install vhs: https://github.com/charmbracelet/vhs
# Run: vhs docs/demo.tape

Output docs/demo.gif

Set FontSize 18
Set Width 1200
Set Height 720
Set Theme "Dracula"
Set Padding 20
Set Margin 10
Set TypingSpeed 80ms
Set CursorBlink false

# Set dark background
Set WindowBar Colorful
Set BorderRadius 8

# === START RECORDING ===

Type "# Build workflows with plain English"
Sleep 1s
Enter

Type ./llm-box create "fetch example.com and save to output.txt"
Sleep 500ms
Enter

Sleep 2s

Type "# Now run the workflow!"
Sleep 1s
Enter

Type "./llm-box run example.com_output.txt.yaml"
Sleep 500ms
Enter

Sleep 5s

# Show final output
Type "# Check what we saved"
Sleep 1s
Enter

Type "cat output.txt"
Sleep 500ms
Enter

Sleep 4s

