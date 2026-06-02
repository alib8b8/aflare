# Hacker News Launch Content

---

## 20 Title Options for Show HN

1. Show HN: llm-box - Build terminal workflows using plain English
2. Show HN: A terminal workflow tool that doesn't require YAML
3. Show HN: Turn repetitive terminal tasks into reusable workflows
4. Show HN: llm-box – No YAML, no GUI, just terminal automation
5. Show HN: Workflow automation for people who love the terminal
6. Show HN: Build workflows in plain English and run them in your terminal
7. Show HN: A lightweight workflow engine for the command line
8. Show HN: llm-box – Escape YAML hell with plain English workflows
9. Show HN: From "fetch HN and save" to executable workflow in 10 seconds
10. Show HN: A single-binary terminal tool for workflow automation
11. Show HN: Extensible workflow automation that lives in your terminal
12. Show HN: llm-box – Describe it, run it, no boilerplate required
13. Show HN: Workflow automation for devs who hate configuration files
14. Show HN: llm-box – Turn English descriptions into terminal workflows
15. Show HN: Beautiful TUI for managing and running terminal workflows
16. Show HN: A Go tool for terminal workflow automation
17. Show HN: llm-box – No drag-and-drop, just plain English and the terminal
18. Show HN: llm-box – Save, share, and version control your terminal workflows
19. Show HN: Workflow automation without leaving your terminal
20. Show HN: llm-box – A simple alternative to bash scripts and Makefiles

---

## Show HN Post (Body)

### Primary (Pick 1 Title, Use This Body)
```
After years of writing bash scripts and maintaining YAML configs for my workflows, I decided to build something simpler.

llm-box is a terminal-first workflow automation tool that lets you describe what you want in plain English, then executes it with a beautiful TUI showing real-time progress.

Key features:
- Define workflows in plain English ("fetch Hacker News and save")
- Single static binary, no dependencies
- Beautiful TUI showing step-by-step progress
- Extensible node system (build custom nodes in any language)
- Cross-platform (Linux/macOS/Windows)
- MIT licensed

Here's how it works:
1. `llm-box create "fetch Hacker News and save to file"`
2. llm-box generates a workflow YAML
3. `llm-box run hn-workflow.yaml`

Repo: https://github.com/alib8b8/llm-box
Examples: https://github.com/alib8b8/llm-box/tree/main/examples

Would love to hear your feedback!
```

---

## Pre-Written Comments (Reply Templates)

### Comment 1: If Asked "Why Not Just Use Bash?"
```
Great question! I love bash too and still use it all the time for simple things.

llm-box adds:
- A beautiful TUI showing progress in real-time
- Workflow reusability and version control
- Better error handling and reporting
- A structured node system that's easier to reason about for complex workflows
- Built-in nodes for common tasks (fetching URLs, writing files, running commands, etc.)

Think of it as bash++ for structured, reusable workflows rather than a replacement for bash entirely.
```

### Comment 2: If Asked About the Tech Stack
```
It's built entirely with Go!

Libraries used:
- Bubbletea for the TUI
- Cobra for the CLI structure
- Standard library for everything else

The TUI updates in real-time as each step runs, which makes it really satisfying to watch your workflows execute.

The single static binary is one of my favorite parts - just download and run, no dependencies to install.
```

### Comment 3: If Asked About Extensibility
```
Yes! You can build custom nodes in any language.

A node just needs to:
1. Read input from stdin
2. Write output to stdout
3. Exit with 0 on success, non-zero on failure

The repo includes a template and docs for building custom nodes: https://github.com/alib8b8/llm-box/blob/main/docs/contributing.md

I'd love to see what the community builds!
```

### Comment 4: If Asked "How Is This Different from Make?"
```
Make is great for build systems, but less great for general-purpose workflows.

llm-box:
- Has a beautiful terminal UI showing progress
- Lets you define workflows in plain English
- Is designed for general-purpose automation, not just builds
- Has built-in nodes for fetching, transforming, writing files, etc.
- Generates readable, maintainable workflows

That said, they solve slightly different problems - I still use Make for builds!
```

### Comment 5: If Asked About Production Readiness
```
v0.1 is early access - I've been using it for my own workflows and it's been working great, but it's not quite production-ready for mission-critical stuff yet.

The roadmap has v1.0 (stable production release) planned for Q3 2026, which will include:
- Better error handling
- Scheduled workflows
- More built-in nodes
- Comprehensive docs

That said, try it out for your personal workflows and let me know what's missing!
```

---

## FAQ for HN Comments (Anticipated Questions)

### Q: Does this require any cloud services or external dependencies?
A: No! It's a single static binary with zero dependencies. Everything runs locally on your machine. No data leaves your computer.

### Q: Can I edit the generated workflows by hand?
A: Absolutely! All workflows are plain YAML that you can edit directly. The "plain English" feature is just a convenience for creating them.

### Q: What platforms are supported?
A: Linux, macOS, and Windows are all supported. Pre-built binaries are available on the releases page.

### Q: Is there a way to schedule workflows?
A: Not yet, but it's on the roadmap! For now, you can use cron or systemd timers to run your llm-box workflows on a schedule.

### Q: How do I contribute?
A: Check out the contributing guide here: https://github.com/alib8b8/llm-box/blob/main/docs/contributing.md

We welcome:
- Bug reports and feature requests
- Code contributions (new nodes, improvements to core)
- Documentation improvements
- Example workflows

### Q: Can workflows call external tools?
A: Yes! There's a built-in `execute` node that lets you run any shell command. You can also build custom nodes that integrate with other tools.

---

## HN Engagement Strategy

1. **Timing**: Submit between 6-8 AM Pacific on Tuesday or Wednesday (best time for HN traffic)
2. **First Comment**: Post a comment yourself adding more details about why you built it
3. **Monitoring**: Check comments every 30 minutes for the first 4 hours
4. **Engagement**: Reply to every comment thoughtfully - thank people for feedback, answer questions, don't get defensive
5. **Follow-Up**: Post an update comment in 24 hours about what you're fixing/improving from feedback
