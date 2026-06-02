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

Type "# Watch llm-box run a workflow with a beautiful TUI"
Sleep 1s
Enter

Type "llm-box run examples/basic_summary.yaml"
Sleep 500ms
Enter

Sleep 3s

# Capture TUI showing steps
Type "# See the steps progress in real-time"
Sleep 1s

# Wait for steps to complete
Sleep 8s

# Show final output
Type "# The summarized content was saved to ai_summary.txt"
Sleep 2s
Enter

Type "cat ai_summary.txt"
Sleep 500ms
Enter

Sleep 4s
