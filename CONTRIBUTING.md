# Contributing to llm-box

欢迎贡献代码！感谢你对 llm-box 的兴趣。

## 📋 贡献方式

### 1. 贡献代码

如果你想修复 bug 或添加新功能：

1. Fork 本仓库
2. 创建特性分支：`git checkout -b feature/my-feature`
3. 提交更改：`git commit -am 'Add some feature'`
4. 推送到分支：`git push origin feature/my-feature`
5. 创建 Pull Request

### 2. 贡献节点

这是最简单的贡献方式！创建自定义节点：

#### 步骤

1. 在 `nodes/` 目录下创建新文件夹，如 `nodes/my_node/`
2. 添加 `metadata.yaml`：
   ```yaml
   name: "my_node"
   description: "节点描述"
   entry: "main.py"
   ```
3. 编写入口脚本（支持 Python、Bash、Go、Rust 等任何语言）

#### 节点协议

节点通过 stdin/stdout 与主程序通信：

**输入（JSON）:**
```json
{
  "input": "上一个节点的输出文本",
  "params": {"key": "value"}
}
```

**输出:**
直接输出到 stdout 的纯文本

#### 示例

创建一个简单的 Python 节点：

```python
#!/usr/bin/env python3
import json
import sys

def main():
    data = json.load(sys.stdin)
    input_text = data.get('input', '')
    params = data.get('params', {})
    
    result = f"Processed: {input_text}"
    print(result)

if __name__ == "__main__":
    main()
```

### 3. 贡献文档

更新 README、添加教程或示例工作流。

## 📝 代码规范

- 遵循 Go 的标准代码风格
- 使用 `go fmt` 格式化代码
- 添加适当的注释
- 编写单元测试

## 🐛 报告 Bug

在 GitHub Issues 中报告 bug，请提供：

- 复现步骤
- 预期行为
- 实际行为
- 截图或日志（如适用）

## 💡 提交建议

欢迎提出新功能建议！在 GitHub Issues 中讨论。

---

感谢你的贡献！🎉