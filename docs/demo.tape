# aflare demo recording script
# Install vhs: https://github.com/charmbracelet/vhs
# Run: vhs docs/demo.tape

Output docs/demo.gif

Require aflare

Set FontSize 16
Set Width 1200
Set Height 700
Set Theme "Dracula"
Set Shell "bash"
Set TypingSpeed 50ms

# Demo 1: aflare create
Type "aflare create \"check server CPU and alert if > 80%\""
Sleep 500ms
Enter
Sleep 2s

# Show generated YAML
Type "cat cpu-monitor.yaml"
Sleep 300ms
Enter
Sleep 2s

# Demo 2: aflare run
Type "aflare run cpu-monitor.yaml"
Sleep 300ms
Enter
Sleep 3s

# Demo 3: retry on failure
Type "aflare run api-health-check.yaml"
Sleep 300ms
Enter
Sleep 4s

# Show logs
Type "aflare logs --last"
Sleep 300ms
Enter
Sleep 2s

# End
Type "clear"
Sleep 200ms
Enter