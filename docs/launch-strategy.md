# Launch Strategy for llm-box

---

## Stage 1: 1 → 10 Stars (Days 1-7)

### Goal
Get initial traction with close connections and immediate network. Establish foundation.

### Communities to Engage
- Personal Twitter/X followers
- LinkedIn professional network
- Developer Discord servers you're active in
- Your company's internal Slack channels (if allowed)

### Content Strategy
- Post 3-4 times per week on Twitter/X
- 1 LinkedIn post with technical details
- 1 weekly update in relevant Discord servers

### Key Actions
1. **Day 1**:
   - Share with close dev friends, ask for quick feedback
   - Tweet the project with demo GIF
   - Post LinkedIn update with architecture diagram
2. **Days 2-4**:
   - Respond to all comments/feedback
   - Fix critical bugs quickly (first 48 hours)
   - Share a couple workflow examples on Twitter
3. **Days 5-7**:
   - Open 2-3 "good first issues" for new contributors
   - Post a progress update
   - Ask for specific feedback on 1 feature

### Launch Posts (Stage 1)

#### Twitter/X Thread (Day 1)
```
🧵 1/6 Just shipped llm-box - a terminal-first workflow automation tool!

After years of writing bash scripts that break and maintaining YAML config hell, I wanted something simpler.

2/6 llm-box lets you:
✅ Describe workflows in plain English ("fetch HN and save")
✅ Single static binary - no dependencies
✅ Beautiful TUI with real-time progress
✅ Extensible node system
✅ MIT licensed

3/6 Here's the magic:
llm-box create "fetch Hacker News and save to file"
→ generates a workflow YAML for you

Then just run it:
llm-box run hn-workflow.yaml

4/6 No YAML to write (unless you want to).
No drag-and-drop GUI.
No vendor lock-in.

All workflows are plain YAML, so you can edit them by hand too.

5/6 Built with Go + Bubbletea.
Linux/macOS/Windows supported.
Check out the repo for 10 workflow examples!

6/6 Would love your feedback!
⭐: https://github.com/alib8b8/llm-box

#golang #cli #automation #devtools
```

#### LinkedIn Post (Day 1)
```
Just released llm-box - a terminal-first workflow automation tool!

After years of juggling fragile bash scripts and complicated Makefiles/YAML configs, I decided to build something simpler.

llm-box lets you:
✅ Define what you want in plain English
✅ Execute workflows instantly
✅ See beautiful real-time progress in your terminal
✅ Extend with custom nodes in any language
✅ No dependencies - single static binary

MIT licensed, cross-platform, and ready for your feedback!

Check it out and let me know what you think:
https://github.com/alib8b8/llm-box

#DevTools #CLI #Automation #Golang #OpenSource #Productivity
```

### Metrics to Track
- Daily star count
- Issue/PR count
- Referrer traffic
- Downloads from releases
- Comments/feedback received

---

## Stage 2: 10 → 50 Stars (Weeks 2-4)

### Goal
Reach larger developer communities, hit Reddit and Hacker News, start building a small user base.

### Communities to Engage
- Reddit: r/SideProject, r/OpenSource, r/selfhosted, r/programming, r/golang
- Hacker News (Show HN)
- Dev.to (1 post per week)
- Lobste.rs
- Go Forum

### Content Strategy
- 1 major community post per week
- 2-3 smaller updates on Twitter/X
- 1 weekly workflow example on GitHub Discussions

### Key Actions
1. **Week 2**:
   - Post to r/SideProject (Day 8)
   - Post a "Show HN" draft to r/programming for feedback
   - Add 3 more example workflows
2. **Week 3**:
   - Submit Show HN (early Tuesday morning Pacific time)
   - Post to r/golang
   - Add 2-3 "help wanted" issues
3. **Week 4**:
   - Publish 1 tutorial blog post
   - Post to r/OpenSource and r/selfhosted
   - Ship v0.1.1 with bug fixes from feedback

### Launch Posts (Stage 2)
See separate `reddit-launch.md` and `hacker-news-launch.md` files.

### Hacker News Strategy
- **Timing**: Tuesday or Wednesday, 6-8 AM Pacific
- **Title**: "Show HN: llm-box - Build terminal workflows using plain English"
- **Body**:
  ```
  After years of writing bash scripts and maintaining YAML configs for my workflows, I built something simpler.

  llm-box:
  - Lets you describe what you want in plain English
  - Generates executable workflows
  - Has a beautiful terminal UI showing progress
  - Is a single static binary, no dependencies
  - MIT licensed, built with Go

  Would love to hear your feedback!

  https://github.com/alib8b8/llm-box
  ```
- **Prepare 5+ comments to reply with, addressing common questions**

### Metrics to Track
- Hourly star count during big launches
- Comments/upvotes on community posts
- New contributors
- Workflow examples shared by users

---

## Stage 3: 50 → 100 Stars (Months 2-3)

### Goal
Establish as a legitimate small open-source project, start getting contributions beyond issues.

### Communities to Engage
- Awesome lists (PR to awesome-go, awesome-cli-apps, etc.)
- Go podcast outreach
- Dev.to series (3-part tutorial)
- YouTube tutorial collaboration (optional)

### Content Strategy
- 1 blog post per 2 weeks
- 1 tutorial per month
- Weekly "workflow of the week" on Discussions

### Key Actions
1. **Month 2**:
   - PR to 3-5 awesome lists
   - Publish 3-part tutorial on Dev.to
   - Release v0.2 with plugin system
2. **Month 3**:
   - Reach out to 2-3 small dev podcasts
   - Host a "workflow contest" with small prizes
   - Ship v0.3 with template library
3. **Ongoing**:
   - Monthly contributor spotlight on Discussions
   - Respond to all issues within 48 hours
   - Review PRs within 72 hours

### PRs to Awesome Lists
```
## awesome-go
Add under "DevOps Tools" or "Command Line":
- [llm-box](https://github.com/alib8b8/llm-box) - Build terminal workflows using plain English.

## awesome-cli-apps
Add under "Automation":
- [llm-box](https://github.com/alib8b8/llm-box) - Terminal-first workflow automation.

## awesome-productivity
Add under "Tools":
- [llm-box](https://github.com/alib8b8/llm-box) - Turn repetitive terminal tasks into reusable workflows.
```

### Metrics to Track
- Weekly star growth rate
- Number of contributors
- PRs merged from non-maintainers
- Community-shared workflows
- Referrals from awesome lists

---

## Overall Timeline Summary

| Period | Target | Activities |
|--------|--------|------------|
| Days 1-7 | 10 stars | Personal network, initial feedback |
| Weeks 2-4 | 50 stars | Reddit, Show HN, initial examples |
| Months 2-3 | 100+ stars | Awesome lists, tutorials, v0.2/v0.3 |

---

## Crisis Management

### If Growth Stalls
- Double down on the platform that's working best
- Publish 3-5 more example workflows
- Ask existing users what's missing
- Consider a small pivot if feedback suggests it

### If Negative Feedback
- Always be thankful and respectful
- Address valid criticisms quickly
- Don't get defensive
- Turn negative comments into improvement opportunities

### If Competitor Launches
- Focus on your unique strengths
- Double down on community building
- Keep shipping improvements quickly
- Highlight your open-source nature

---

## Key Success Indicators

✅ 10 stars in 1 week  
✅ 50 stars in 1 month  
✅ 100 stars in 3 months  
✅ 3+ external contributors  
✅ 10+ community-shared workflows  
✅ 1-2 tutorial blog posts published
