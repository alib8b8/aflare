# Contributing to llm-box

感谢你对 llm-box 的兴趣！🎉
llm-box 之所以强大，是因为它有一个充满活力的社区节点生态系统。
任何人，无需是 Go 开发者，都可以贡献节点。

## 📋 目录

- [贡献节点（最简单）](#贡献节点最简单)
- [贡献代码](#贡献代码)
- [贡献文档](#贡献文档)
- [报告 Bug](#报告-bug)
- [提交建议](#提交建议)
- [代码规范](#代码规范)

---

## 🧩 贡献节点（最简单）

贡献一个自定义节点 = 创建一个新文件夹，写个 `metadata.yaml` 和一个脚本（任何语言！）。
不需要懂 Go，不需要写 PR 改主程序。

### 步骤 1: 复制模板

```bash
cp -r nodes/_template nodes/my_node
cd nodes/my_node
```

模板中包含：

```
nodes/_template/
├── metadata.yaml     # 节点元数据
└── main.py           # 入口脚本（任何语言都行）
```

### 步骤 2: 编辑 `metadata.yaml`

```yaml
name: "my_node"
description: "我的第一个节点：提取数字"
entry: "main.py"
```

字段说明：
- `name`（必填）：节点在 YAML 中引用的名字，全局唯一
- `description`（必填）：一句话描述，做什么用
- `entry`（必填）：入口脚本名（`main.py`、`main.go`、`script.sh` 等）

### 步骤 3: 实现入口脚本

节点协议非常简单：stdin 进 JSON，stdout 出纯文本。

**输入（stdin 上的 JSON）：**
```json
{
  "input": "上一个节点的输出文本",
  "params": {"key": "value", ...}
}
```

**输出（stdout 上的纯文本）：**
直接输出结果文本，不需要 JSON 包裹。

Python 示例（`main.py`）：

```python
#!/usr/bin/env python3
import json
import sys
import re

def main():
    payload = json.load(sys.stdin)
    text = payload.get("input", "")
    nums = re.findall(r"\d+", text)
    print(" ".join(nums))

if __name__ == "__main__":
    main()
```

Bash 示例（`script.sh`）：

```bash
#!/usr/bin/env bash
INPUT=$(cat)
echo "$INPUT" | tr '[:upper:]' '[:lower:]'
```

> 完整示例可以参考 [nodes/echo/](../nodes/echo/)。

### 步骤 4: 本地测试

创建一个临时工作流来测试你的节点：

```yaml
# test_my_node.yaml
name: "Test my_node"
steps:
  - node: "echo"     # 任何节点来提供初始数据
    params: {}
  - node: "my_node"  # 你的节点
    params: {}
```

然后运行：

```bash
llm-box run test_my_node.yaml
```

测试 OK 后删除临时文件。

### 步骤 5: 添加一个示例

在 `examples/` 中放一个 `my_node_demo.yaml`，让其他人看到用法。

### 步骤 6: 提交 PR

```bash
git checkout -b feature/my-node
git add nodes/my_node examples/my_node_demo.yaml
git commit -m "feat(nodes): add my_node for extracting numbers"
git push origin feature/my-node
```

打开 GitHub → 发起 Pull Request。
按 PR 模板填写说明，maintainer 会 Review 并合并。

---

## 💻 贡献代码

修复 bug、改进 TUI、新增内置节点等，需要改 Go 代码。

1. Fork 本仓库
2. 创建特性分支：`git checkout -b feature/awesome-feature`
3. 编写代码并补测试：`go test ./...`
4. 确保 CI 通过：`go build ./... && go vet ./...`
5. 提交：`git commit -m "feat: do something"`
6. 推送并创建 PR

---

## 📖 贡献文档

- 改进 README、添加教程
- 给示例工作流写说明
- 翻译文档

直接改 `*.md` 文件，PR 即可。

---

## 🐛 报告 Bug

请使用 [Bug Report 模板](../../issues/new?template=bug_report.md)，
包含：复现步骤、预期行为、实际行为、版本、环境、日志。

---

## 💡 提交建议

使用 [Feature Request 模板](../../issues/new?template=feature_request.md)，
描述清楚：动机、提议方案、备选方案。

---

## 📝 代码规范

- Go 官方风格：`gofmt`、`go vet`
- 写单元测试，新代码覆盖率不低于 80%
- 提交信息用 [Conventional Commits](https://www.conventionalcommits.org/)
  - `feat:` 新功能
  - `fix:` 修 bug
  - `docs:` 文档
  - `chore:` 杂项

---

## 📜 行为准则

请友善、包容、专注技术。
我们欢迎所有技能水平的贡献者，从第一个 PR 开始。

---

🎉 感谢你的贡献！
