## 📝 变更说明

<!-- 简述本 PR 做了什么、为什么。关联 issue：Closes #123 -->

## 🔍 变更类型

<!-- 勾选所有适用项 -->

- [ ] feat — 新功能
- [ ] fix — Bug 修复
- [ ] refactor — 重构（无行为变化）
- [ ] perf — 性能优化
- [ ] security — 安全修复
- [ ] docs — 文档
- [ ] test — 测试
- [ ] chore — 维护 / CI / 工具

## ✅ 自检清单

<!-- 提交前逐项确认。CI 会自动检查前 8 项，但请先本地自测。 -->

### 自动化检查（CI 必过）

- [ ] `gofmt -l .` 无输出
- [ ] `go vet ./...` 通过
- [ ] `golangci-lint run ./...` 通过（errcheck / staticcheck / funlen）
- [ ] `go test ./... -short` 通过
- [ ] `go test -cover` ≥ 60%
- [ ] `go test -race ./internal/agent/... ./internal/memory/...` 通过
- [ ] `govulncheck ./...` 无漏洞
- [ ] Commit message 符合格式：`<type>: <description>`

### 代码质量

- [ ] 无 `_ =` 丢弃错误（os.Remove 等 best-effort 清理需加 `// best-effort` 注释）
- [ ] 函数 ≤ 250 行（超出需加 `//nolint:funlen` 并关联 issue）
- [ ] 无死代码（unused 变量 / 函数 / 导入）
- [ ] 新增公开 API 有 GoDoc 注释

### 安全

- [ ] 用户输入未直接拼入文件路径（path traversal）
- [ ] 用户输入未直接传给 `exec.Command`（command injection）
- [ ] 用户提供的 URL 经过校验（SSRF）
- [ ] 并发访问的 map / slice 已加锁

### 测试

- [ ] 新功能有对应的 `*_test.go`
- [ ] 流式路径（onChunk / onToolCall）已测试
- [ ] 并发场景用 `go test -race` 验证（goroutine ≥ 10）

## 📸 验证截图 / 日志

<!-- 如果是 UI / CLI 变更，贴出前后对比。 -->

## 📎 附加信息

<!-- Breaking changes、迁移说明、相关讨论链接等。无则留空。 -->

## 📜 授权确认

- [ ] 我确认本人有权提交本贡献，并同意 [CONTRIBUTING.md](../CONTRIBUTING.md) 的贡献条款（AGPL v3.0 出站许可 + 项目所有者平行商业授权）。引入的第三方代码为 MIT / Apache-2.0 / BSD 等宽松许可且保留版权声明。

