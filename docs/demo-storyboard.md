# Demo GIF Storyboard (15 Seconds)

## Scene 1: Terminal Open (0-2s)
```
┌─────────────────────────────────────────┐
│ ● ● ●  terminal                        │
├─────────────────────────────────────────┤
│                                         │
│  $                                      │
│                                         │
│                                         │
│                                         │
│                                         │
│                                         │
│                                         │
└─────────────────────────────────────────┘
```
**Action:** Show terminal window opening, cursor blinking

---

## Scene 2: Install Command (2-4s)
```
┌─────────────────────────────────────────┐
│ ● ● ●  terminal                        │
├─────────────────────────────────────────┤
│                                         │
│  $ curl -sL .../install.sh | bash       │
│                                         │
│  Downloading llm-box...                 │
│  ✅ Installed successfully!              │
│                                         │
│  $                                      │
│                                         │
└─────────────────────────────────────────┘
```
**Action:** Type install command, show success message

---

## Scene 3: Create Workflow (4-7s)
```
┌─────────────────────────────────────────┐
│ ● ● ●  terminal                        │
├─────────────────────────────────────────┤
│                                         │
│  $ llm-box create "fetch HN stories     │
│    and save to file"                    │
│                                         │
│  ✅ Workflow created: hn_workflow.yaml  │
│                                         │
│  $ cat hn_workflow.yaml                 │
│  name: "HN Stories"                     │
│  steps:                                 │
│    - node: fetch_url                    │
│      params:                            │
│        url: news.ycombinator.com        │
│    - node: file_write                    │
│      params:                            │
│        path: stories.txt                │
│                                         │
└─────────────────────────────────────────┘
```
**Action:** Show natural language to YAML conversion

---

## Scene 4: Run Workflow (7-11s)
```
┌─────────────────────────────────────────┐
│ ● ● ●  terminal                        │
├─────────────────────────────────────────┤
│                                         │
│  $ llm-box run hn_workflow.yaml         │
│                                         │
│  ╔═══════════════════════════════════╗  │
│  ║  🚀 llm-box - hn_workflow.yaml   ║  │
│  ║                                   ║  │
│  ║  📁 examples/hn_workflow.yaml     ║  │
│  ║                                   ║  │
│  ║  📋 Steps:                        ║  │
│  ║  1. fetch_url          ✅ done    ║  │
│  ║  2. file_write         ✅ done    ║  │
│  ║                                   ║  │
│  ║  ✅ Workflow completed!           ║  │
│  ╚═══════════════════════════════════╝  │
│                                         │
└─────────────────────────────────────────┘
```
**Action:** Show TUI with animated progress

---

## Scene 5: Success Output (11-13s)
```
┌─────────────────────────────────────────┐
│ ● ● ●  terminal                        │
├─────────────────────────────────────────┤
│                                         │
│  ✅ Workflow completed in 2.3s          │
│                                         │
│  Output saved to: stories.txt          │
│                                         │
│  $ cat stories.txt                     │
│  1. "Open source AI startup raises     │
│      $100M Series A"                   │
│  2. "New Rust release with 40%        │
│      performance boost"                 │
│  3. "GitHub announces Copilot          │
│      Enterprise tier"                   │
│                                         │
│  $ _                                    │
│                                         │
└─────────────────────────────────────────┘
```
**Action:** Show final output with content

---

## Scene 6: Call to Action (13-15s)
```
┌─────────────────────────────────────────┐
│ ● ● ●  terminal                        │
├─────────────────────────────────────────┤
│                                         │
│  $                                     │
│                                         │
│        ┌─────────────────────┐          │
│        │   ⭐ Star on GitHub │          │
│        │  github.com/        │          │
│        │   alib8b8/llm-box   │          │
│        └─────────────────────┘          │
│                                         │
│  Questions? Open an issue!              │
│                                         │
│  $ _                                    │
│                                         │
└─────────────────────────────────────────┘
```
**Action:** Fade in star button, show support message

---

## Technical Notes for VHS

```bash
Output docs/demo.gif
Set FontSize 16
Set Width 1280
Set Height 720
Set Theme "Catppuccin Mocha"
Set TypingSpeed 50ms
Set Margin 20
Set Padding 20
Set BorderRadius 8

# Scene 1: Terminal open
Sleep 2s

# Scene 2: Install
Type "curl -sL https://raw.githubusercontent.com/alib8b8/llm-box/main/install.sh | bash"
Enter
Sleep 2s

# Scene 3: Create workflow
Type 'llm-box create "fetch HN stories and save to file"'
Enter
Sleep 2s

# Scene 4: Run workflow
Type "llm-box run hn_workflow.yaml"
Enter
Sleep 3s

# Scene 5: View output
Type "cat stories.txt"
Enter
Sleep 2s

# Scene 6: CTA
Type "echo '⭐ Star on GitHub!'"
Enter
```

---

## Estimated File Size
- GIF (optimized): ~500KB - 1MB
- Duration: 15 seconds
- Resolution: 1280x720
- Frame rate: 10-15 FPS
