# llm-box 推广素材包

> 所有文案都可以直接复制粘贴使用。根据不同平台的风格选择合适的版本。

---

## 一、GitHub 仓库优化

### 仓库描述（About）

```
Build terminal workflows using plain English. No YAML. No boilerplate. Just describe and execute.
```

### 网站（Website）
```
https://github.com/alib8b8/llm-box
```

### Topics（20个）
```
workflow, automation, terminal, cli, golang, developer-tools, productivity, workflow-engine, command-line, scripting, yaml-free, static-binary, open-source, devops, homebrew, linux, macos, windows, shell, tui
```

---

## 二、社交媒体文案

### 1. Twitter / X 线程（9 条）

#### 版本 A - 产品发布型
```
🧵 1/9 Just shipped llm-box - a terminal-first workflow automation tool!

After years of writing fragile bash scripts and maintaining YAML config hell, I wanted something simpler.

2/9 llm-box lets you:
✅ Describe workflows in plain English ("fetch HN and save")
✅ Single static binary - no dependencies
✅ Beautiful TUI with real-time progress
✅ 20+ built-in nodes
✅ 15+ LLM providers supported
✅ MIT licensed

3/9 Here's the magic:
llm-box create "fetch Hacker News and save to file"
→ generates a workflow YAML for you

Then just run it:
llm-box run hn-workflow.yaml

4/9 No YAML to write (unless you want to).
No drag-and-drop GUI.
No vendor lock-in.

All workflows are plain YAML, so you can edit them by hand too.

5/9 Built-in nodes include:
• fetch_url / http_request
• file_read / file_write
• execute (shell commands)
• json_parse / template_render
• 15+ LLMs (Ollama, DeepSeek, OpenAI-compatible)
• condition, call, combine, notify

6/9 Security is built-in:
• SSRF protection (URL validation, DNS checks, redirect validation)
• Path traversal protection (sandboxed paths, symlink resolution)
• Command injection prevention (metachar filtering, optional allowlist)
• Resource limits (file size, step count, timeouts)

7/9 Built with Go + Bubbletea.
Linux/macOS/Windows supported.
Single static binary - download and run.

8/9 Check out the repo for 10 ready-to-use workflow examples!
GitHub: https://github.com/alib8b8/llm-box

9/9 Would love your feedback! Try it out and let me know what you think.
⭐ if you find it interesting!

#golang #cli #automation #devtools #opensource #productivity #terminal #workflow #ai
```

#### 版本 B - 问题导向型
```
🧵 1/6 Tired of fragile bash scripts and YAML hell? I built something.

Meet llm-box - a terminal-first workflow tool.

2/6 The problem:
• Bash scripts are quick but unmaintainable
• Workflow tools require endless YAML
• GUI builders are slow and heavy
• I just want to get stuff done in my terminal

3/6 The solution:
Describe what you want in plain English
→ llm-box generates an executable workflow
→ Watch the beautiful TUI show progress

4/6 Example:
$ llm-box create "fetch HN top stories and save to file"
$ llm-box run hn_workflow.yaml

That's it. No YAML to write. No config files.

5/6 It's a single static binary built with Go.
20+ built-in nodes. 15+ LLM providers.
MIT licensed. Cross-platform.

6/6 Check it out: https://github.com/alib8b8/llm-box

⭐ if this looks useful!

#Go #CLI #Automation #DevTools #OpenSource
```

### 2. 微博 / 小红书（中文短文案）

```
做了个终端工作流工具，分享给大家 🛠️

llm-box - 用自然语言就能生成可执行的工作流

✨ 核心功能：
• 说一句"抓取 HN 热门并保存到文件"，自动生成工作流
• 单二进制文件，零依赖，下载即用
• 精美的终端 UI，实时展示进度
• 20+ 内置节点（网络/文件/命令/数据处理）
• 15+ LLM 支持（本地 Ollama + 各大云 API）
• 可扩展：用任意语言写自定义节点
• MIT 协议，完全开源

📦 GitHub: https://github.com/alib8b8/llm-box

程序员效率工具 +1 ⭐

#开源 #Go语言 #CLI #效率工具 #自动化 #开发者工具
```

### 3. LinkedIn / 领英

```
Just released llm-box - a terminal-first workflow automation tool!

After years of juggling fragile bash scripts and complicated YAML configs, I decided to build something simpler.

llm-box lets you:
✅ Define workflows in plain English
✅ Execute instantly with a beautiful terminal UI
✅ 20+ built-in nodes for common tasks
✅ 15+ LLM providers (local + cloud)
✅ Single static binary - zero dependencies
✅ Extensible with custom nodes in any language
✅ MIT licensed, cross-platform

Built with Go and Bubbletea. Check it out and let me know what you think!

🔗 https://github.com/alib8b8/llm-box

#DevTools #CLI #Automation #Golang #OpenSource #Productivity #Workflow #DeveloperTools
```

---

## 三、技术社区发帖文案

### 1. V2EX
**节点：** 分享创造 / 程序员

**标题：**
```
[开源] llm-box - 用自然语言构建终端工作流
```

**正文：**
```
做了一个终端优先的工作流自动化工具，分享给大家。

## 项目简介

llm-box 是一个用 Go 写的终端工作流工具，可以用自然语言描述你想做什么，它会生成可执行的 YAML 工作流。

## 为什么做这个

- 写了太多脆弱的 bash 脚本，改起来头疼
- 现有工作流工具要么是重型 GUI，要么要写一堆 YAML
- 想要一个轻量、开箱即用的终端工具

## 核心特性

✅ 自然语言输入 - "抓取 HN 热门并保存到文件"，直接生成工作流
✅ 单二进制 - 零依赖，60 秒安装，下载就能用
✅ 精美 TUI - 实时展示每个步骤的执行进度
✅ 20+ 内置节点 - fetch_url、execute、文件操作、HTTP 请求、JSON 解析、模板渲染等
✅ 15+ LLM 支持 - Ollama（本地）、DeepSeek、通义千问、智谱、Kimi、OpenAI 兼容（200+ 模型）
✅ 可扩展 - 用任意语言（Python/Shell/JS 等）写自定义节点
✅ 跨平台 - Linux / macOS / Windows
✅ MIT 协议 - 完全开源

## 安全设计

- SSRF 防护（URL 校验、DNS 解析检查、重定向验证）
- 路径遍历防护（符号链接解析、沙箱路径限制）
- 命令注入防护（Shell 元字符过滤、可选命令白名单）
- 资源限制（文件大小、响应体大小、步骤数量限制）

## 快速上手

```bash
# 安装
curl -sL https://raw.githubusercontent.com/alib8b8/llm-box/main/install.sh | bash

# 生成工作流
llm-box create "抓取 Hacker News 热门帖子并保存到 stories.txt"

# 执行
llm-box run hn_workflow.yaml
```

## 项目地址

GitHub: https://github.com/alib8b8/llm-box

里面有 10 个现成的工作流示例，欢迎 star 和提 issue！
```

### 2. 掘金
**分类：** 后端 / 开源 / 工具

**标题：**
```
从 100 个 bash 脚本到一个工具：我用 Go 做了个终端工作流引擎
```

**正文：**
```
## 背景：被 bash 脚本淹没的日常

作为一个后端开发者，我每天都在写各种自动化脚本：
- 抓取数据 → 处理 → 存文件
- 跑测试 → 发通知 → 写报告
- 部署应用 → 检查日志 → 告警

写了几百个脚本之后，我发现几个痛点：

1. **不可维护**：三个月前的脚本，自己都看不懂
2. **不可复用**：每个项目都要复制粘贴改一改
3. **没有进度**：脚本一跑起来，天知道跑到哪了
4. **安全隐患**：随便 `curl | bash`，路径穿越、SSRF 想都没想过

试过 n8n、Airflow 这些工具，但对我来说太重了——我就想在终端里快速搭个工作流，不想开浏览器、不想装 Python 环境、不想写几百行 YAML。

于是我用 Go 写了 **llm-box**——一个终端优先的轻量工作流引擎。

## 它能干什么？

一句话：**说你想做什么，它生成可执行的工作流。**

```bash
# 用自然语言生成
llm-box create "抓取 Hacker News 热门帖子，总结后保存到 stories.md"

# 执行
llm-box run hn_workflow.yaml
```

执行的时候，终端会显示一个精美的 TUI，每个步骤的状态和耗时一目了然。

## 架构设计

整个项目的架构其实很简单，核心就三层：

```
┌─────────────────────────────────────────┐
│              TUI (Bubbletea)            │  ← 用户交互层
├─────────────────────────────────────────┤
│           Workflow Engine               │  ← 执行引擎
│  ┌─────────┐  ┌──────────┐  ┌────────┐ │
│  │ Parser  │→│ Executor │→│  Expr  │ │
│  └─────────┘  └──────────┘  └────────┘ │
├─────────────────────────────────────────┤
│             Node Registry               │  ← 节点层
│  fetch_url / execute / file_* / LLM...  │
└─────────────────────────────────────────┘
```

### 1. 节点系统：一切皆是节点

工作流由一个个节点串联而成，每个节点只做一件事：
- 接收输入（string）
- 处理
- 返回输出（string）

接口非常简洁：

```go
type Node interface {
    Name() string
    Description() string
    Schema() NodeSchema
    Execute(ctx context.Context, input string, params map[string]interface{}) (string, error)
}
```

为什么用 string 而不是 interface{}？因为简单、可序列化、好调试。所有节点之间传的都是文本，想看中间结果直接打印就行。

### 2. 表达式引擎：在步骤间传数据

工作流不能只是线性执行，还需要在前一个步骤的输出里取数据，传给下一个步骤。

我实现了一个轻量的表达式引擎，支持这样的语法：

```yaml
steps:
  - name: fetch
    node: fetch_url
    params:
      url: "https://api.example.com/data"

  - name: parse
    node: json_parse
    params:
      path: "data.items.[0].name"
    input: "{{steps.fetch.output}}"   # ← 引用上一步的输出
```

表达式引擎支持变量、函数调用、字符串操作，完全自己实现的，没用第三方依赖。

### 3. 自然语言生成：不是 AI，是关键词匹配

很多人以为 `llm-box create` 是用 LLM 生成工作流——其实不是，至少现在不是。

目前是**规则匹配 + 关键词识别**：
- 看到 "fetch" / "抓取" → 加 fetch_url 节点
- 看到 "save" / "保存" → 加 file_write 节点
- 看到 "summarize" / "总结" → 加 LLM 节点
- 识别 URL、文件名、提供商名称

为什么不用 LLM？两个原因：
1. **确定性**：同样的输入应该得到同样的输出，不能今天生成的和明天不一样
2. **零依赖**：用户不想配置 API key 也能用这个功能

当然，复杂的工作流还是建议直接写 YAML。关键词匹配覆盖的是 80% 的简单场景。

## 安全设计：从第一天就重视

因为工作流会执行命令、发网络请求、读写文件，安全是头等大事。

### SSRF 防护

`fetch_url` 和 `http_request` 节点做了三层防护：

1. **URL 校验**：只允许 http/https scheme，拒绝 file://、gopher:// 这些危险协议
2. **DNS 重绑定防护**：解析 IP 后检查是否是内网 IP（10.x、172.16-31.x、192.168.x、127.x 全部拒绝）
3. **重定向验证**：每次重定向都重新校验目标 URL，防止跳转到内网

关键代码大致是这样：

```go
func validateURL(rawURL string) error {
    u, err := url.Parse(rawURL)
    if err != nil {
        return err
    }
    
    // 只允许 http/https
    if u.Scheme != "http" && u.Scheme != "https" {
        return fmt.Errorf("unsupported scheme: %s", u.Scheme)
    }
    
    // 解析 IP，检查是否内网
    ips, err := net.LookupIP(u.Hostname())
    if err != nil {
        return err
    }
    for _, ip := range ips {
        if ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() {
            return fmt.Errorf("internal IP not allowed: %s", ip)
        }
    }
    
    return nil
}
```

### 路径遍历防护

文件操作节点限制在工作目录内：

1. **禁止绝对路径**：只能用相对路径
2. **解析后检查**：把路径解析成绝对路径后，确认它在 baseDir 内
3. **符号链接解析**：用 `filepath.EvalSymlinks` 解析真实路径，防止通过 symlink 绕过

这部分踩了不少坑——一开始只做了字符串前缀匹配，后来发现符号链接可以绕过，又加了一层。

### 命令注入防护

`execute` 节点默认过滤 shell 元字符（`;` `|` `&&` `$` 反引号等），如果需要完全控制，可以在配置里开启 allowlist 模式。

### 资源限制

内置了各种上限，防止恶意工作流把机器搞挂：
- 最大步骤数：1000
- 单步超时：30 分钟
- 文件大小上限：10 MB
- 最大重试次数：10 次

## TUI 实现：Bubbletea 的魅力

终端 UI 用的是 Bubbletea，The Elm Architecture 写起来真的很舒服。

每个步骤一个状态机：
```
Pending → Running → Success
               ↘ Failed → Retrying → ...
```

进度条、耗时统计、错误信息，都是实时更新的。执行完还会输出一个汇总表格，告诉你每个步骤花了多久。

## 为什么选 Go？

几个原因：

1. **单二进制**：编译出来一个文件，丢到哪都能跑，用户不用装运行时
2. **性能好**：工作流引擎本身几乎零开销，瓶颈都在 IO 和 LLM 上
3. **并发方便**：以后加并行执行节点，goroutine 直接上
4. **依赖少**：除了 Bubbletea 和 YAML 解析，几乎全是标准库

## 项目现状

目前已经实现的：
- ✅ 20+ 内置节点
- ✅ 15+ LLM 提供商（Ollama、DeepSeek、通义、智谱、Kimi、OpenAI 兼容...）
- ✅ 精美的 TUI
- ✅ 表达式引擎
- ✅ 工作流嵌套（call 节点）
- ✅ 条件执行（condition 节点）
- ✅ 自定义节点（任意语言，只要能读写 stdin/stdout）
- ✅ 完整的安全防护
- ✅ 9 语言 i18n
- ✅ 10 个示例工作流

## 一些思考

做这个项目最大的体会是：**简单比功能多更难。**

一开始我想加很多功能：并行执行、错误重试、依赖图、循环、子流程... 但后来发现，大多数人需要的只是一个简单的工具——能把几个步骤串起来，看得见进度，出了问题知道哪错了。

所以现在的策略是：**核心保持简单，扩展通过节点实现。** 引擎本身只负责任务调度和状态管理，复杂的逻辑都放在节点里。

## 项目地址

📦 **GitHub**: https://github.com/alib8b8/llm-box

如果觉得有意思，欢迎 star ⭐
有任何想法或建议，评论区聊聊～

---

**你有哪些自动化需求是 bash 脚本搞不定的？欢迎在评论区说说**

#Go #开源 #CLI #自动化 #工作流 #开发者工具 #后端 #架构设计
```

### 3. 知乎回答
**适用问题：** "有哪些命令行的软件堪称神器？" / "有哪些好用的开源工作流工具？"

```
推荐一个我自己做的终端工作流工具——**llm-box**。

## 它解决了什么问题？

不知道你有没有这种感觉：
- 简单的任务写 bash 脚本，写着写着就越来越长
- 过几个月回头看，完全忘了当初写的啥
- 想分享给同事用，还得解释半天
- 重型工作流工具（n8n、Airflow 之类的）又太重量级了

llm-box 走的是**轻量终端路线**：

> 用自然语言描述你想做什么 → 自动生成可执行的 YAML 工作流 → 终端 UI 实时展示进度

## 怎么用？

三行命令搞定：

```bash
# 安装（单二进制，60秒）
curl -sL https://raw.githubusercontent.com/alib8b8/llm-box/main/install.sh | bash

# 生成工作流
llm-box create "抓取 HN 热门并保存到文件"

# 执行
llm-box run hn_workflow.yaml
```

## 为什么值得试试？

1. **真的快** - 单二进制，下载即用，不需要装一堆依赖
2. **体验好** - TUI 实时展示进度，比黑盒跑脚本舒服多了
3. **够灵活** - 20+ 内置节点，还能自己扩展
4. **LLM 多** - 支持 Ollama 本地模型，也支持 DeepSeek/通义/Kimi 等国内 API
5. **全开源** - MIT 协议，随便用

## 适合什么场景？

- 日常开发中的重复性任务
- 数据抓取和处理流水线
- 简单的 DevOps 自动化
- 把你现有的 bash 脚本整理成结构化的工作流

## 项目地址

https://github.com/alib8b8/llm-box

觉得有用的话点个 star 支持一下～有问题随时提 issue。
```

### 4. Dev.to
**标签：** `#go` `#cli` `#automation` `#devtools` `#opensource` `#productivity` `#terminal`

**标题：**
```
Show DEV: I built llm-box - a terminal-first workflow tool that uses plain English instead of YAML
```

**正文：**
```
Hey DEV community! 👋

I've been working on **llm-box** - a terminal-first workflow automation tool, and I'd love to get your feedback.

## The Problem

After years of writing:
- Fragile bash scripts that break when you look at them wrong
- Endless YAML config files for workflow tools
- Heavy GUI builders that feel slow and opaque

I wanted something simpler that lives in my terminal.

## What is llm-box?

**Describe what you want in plain English → get an executable workflow.**

```bash
# Generate a workflow
llm-box create "fetch Hacker News top stories and save to file"

# Run it
llm-box run hn_workflow.yaml
```

It generates a YAML workflow you can run immediately - or edit by hand if you want to tweak things.

## Key Features

✅ **Plain English input** - No YAML boilerplate to write
✅ **Single static binary** - Zero dependencies, download and run
✅ **Beautiful TUI** - Real-time progress tracking with Bubbletea
✅ **20+ built-in nodes** - fetch_url, execute, file_*, http_request, json_parse, template_render, and more
✅ **15+ LLM providers** - Ollama (local), DeepSeek, OpenAI-compatible (200+ models), etc.
✅ **Extensible** - Build custom nodes in any language (Python, Shell, JS, whatever)
✅ **Cross-platform** - Linux, macOS, Windows
✅ **MIT Licensed** - Open source, no vendor lock-in

## Security First

I take security seriously:
- SSRF protection (URL validation, DNS rebinding, redirect checks)
- Path traversal protection (sandboxed paths, symlink resolution)
- Command injection prevention (shell metacharacter filtering, optional allowlist)
- Resource limits (file size, response body, step count, timeouts)

## Tech Stack

- **Language**: Go 1.25
- **TUI**: Bubbletea + Lipgloss
- **Minimal dependencies** - mostly standard library

## Check It Out

📦 **GitHub**: https://github.com/alib8b8/llm-box

The repo includes 10 ready-to-use workflow examples and comprehensive docs.

I'd love to hear what you think! What features would you want to see? What workflow would you build with this?

Star if you find it interesting ⭐
```

---

## 四、发布时间线（第一周）

### 第 1 天（周二）
- [ ] 更新 GitHub 仓库 About 描述和 Topics
- [ ] 发 Twitter/X 线程
- [ ] 发 LinkedIn 帖子

### 第 2 天（周三）
- [ ] V2EX 发帖（分享创造节点）
- [ ] 掘金发文

### 第 3 天（周四）
- [ ] 开源中国发布
- [ ] 思否发文

### 第 4 天（周五）
- [ ] 知乎回答 3-5 个相关问题
- [ ] 发微博/小红书（如果有的话）

### 第 5-7 天（周末）
- [ ] 回复所有评论和反馈
- [ ] 整理高频问题，更新 FAQ
- [ ] 发一条"根据大家反馈更新了 XX"的帖子

---

## 五、互动回复模板

### 1. 感谢支持型
```
Thanks for the kind words! Means a lot 🙌

Let me know if you try it out and have any feedback!
```

### 2. 中文感谢
```
谢谢支持！有问题随时提 issue，或者在评论区交流～
```

### 3. 回答"和 XX 有什么区别？"
```
Great question!

The main difference is [具体区别，比如：llm-box is terminal-first, not GUI / it's a single binary / it uses plain English input instead of YAML].

Think of it as [类比，比如：a lighter, simpler alternative that focuses on the terminal experience].

That said, different tools for different use cases - use whatever works best for you!
```

### 4. 收到功能建议
```
Great idea! That's actually on the roadmap - [link to roadmap issue if exists].

I've added it to my list. Would you mind opening an issue on GitHub so I can track it properly?
```

### 5. 收到负面反馈
```
Thanks for the honest feedback - I really appreciate it.

[回应具体问题，承认不足]

You're right that [承认的点]. That's something I'm planning to improve in [vX.X / the next release].

Thanks for taking the time to share your thoughts!
```

---

## 六、常用 Hashtags

### 英文
```
#golang #go #cli #automation #devtools #opensource #productivity #terminal #workflow #developertools #commandline #scripting #ai #llm
```

### 中文
```
#开源 #Go语言 #CLI #效率工具 #自动化 #开发者工具 #工作流 #终端 #编程 #程序员
```
