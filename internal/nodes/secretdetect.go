package nodes

import (
	"regexp"
	"strings"
	"sync"
	"time"
)

// secretdetect.go — 密钥脱敏与出站数据量监控
//
// 借鉴 Grok Build 隐私丑闻的教训：默认上传整个仓库且 .env 不脱敏。
// 本模块实现"隐私优先"原则：
//   1. 文件读取时自动识别并脱敏常见密钥格式（API Key / Token / 密码）
//   2. .env / .env.* / 含密钥特征的文件按整文件脱敏
//   3. 出站数据量监控：追踪累计发送字节，异常倍数告警

// ------------------------------------------------------------
// 密钥模式识别
// ------------------------------------------------------------

// secretPattern 描述一类密钥的识别规则
type secretPattern struct {
	name    string
	pattern *regexp.Regexp
	mask    string // 脱敏后的占位符
}

// 常见密钥格式（高置信度，避免误伤普通文本）
// 设计原则：仅匹配明显具备密钥特征的字符串（长随机串 + 前缀/结构），
// 不做宽泛匹配以免破坏正常代码内容。
var secretPatterns = []secretPattern{
	// AWS Access Key ID：20位大写字母数字，AKIA 开头
	{name: "aws_access_key", pattern: regexp.MustCompile(`AKIA[0-9A-Z]{16}`), mask: "AKIA[REDACTED]"},
	// AWS Secret Access Key：40位 base64，通常跟在 = 后
	{name: "aws_secret", pattern: regexp.MustCompile(`(?i)aws_secret_access_key["'\s:=]+([A-Za-z0-9/+=]{40})`), mask: "aws_secret_access_key=[REDACTED]"},
	// GitHub Personal Access Token：ghp_/gho_/ghu_/ghs_/ghr_ + 36位
	{name: "github_token", pattern: regexp.MustCompile(`gh[posur]_[A-Za-z0-9]{36}`), mask: "ghp_[REDACTED]"},
	// GitLab Token：glpat- + 20位
	{name: "gitlab_token", pattern: regexp.MustCompile(`glpat-[A-Za-z0-9_-]{20}`), mask: "glpat-[REDACTED]"},
	// Slack Token：xox[baprs]- + 10+位
	{name: "slack_token", pattern: regexp.MustCompile(`xox[baprs]-[A-Za-z0-9-]{10,}`), mask: "xox-[REDACTED]"},
	// Generic API Key：key/api_key 后跟 32+ 位十六进制或 base64
	{name: "generic_api_key", pattern: regexp.MustCompile(`(?i)(api[_-]?key|secret[_-]?key)["'\s:=]+([A-Za-z0-9+/=_-]{32,})`), mask: "${1}=[REDACTED]"},
	// Bearer Token
	{name: "bearer_token", pattern: regexp.MustCompile(`(?i)bearer\s+[A-Za-z0-9+/=_-]{20,}`), mask: "bearer [REDACTED]"},
	// 私钥头
	{name: "private_key", pattern: regexp.MustCompile(`-----BEGIN (RSA |EC |OPENSSH |DSA )?PRIVATE KEY-----`), mask: "-----BEGIN [REDACTED PRIVATE KEY]-----"},
	// JWT（三段式）
	{name: "jwt", pattern: regexp.MustCompile(`eyJ[A-Za-z0-9_-]{10,}\.eyJ[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{10,}`), mask: "eyJ[REDACTED].eyJ[REDACTED].[REDACTED]"},
	// 数据库连接串中的密码 password=xxx
	{name: "db_password", pattern: regexp.MustCompile(`(?i)(postgres|mysql|mongodb|redis)://[^:]+:([^@]{3,})@`), mask: "${1}://[REDACTED]:[REDACTED]@"},
}

// 整文件脱敏的文件名特征（不区分大小写）
var sensitiveFileNamePatterns = []string{
	".env",
	".env.local",
	".env.production",
	".env.development",
	".env.staging",
	"credentials",
	"credentials.json",
	"id_rsa",
	"id_ecdsa",
	"id_ed25519",
	".npmrc",
	".pypirc",
	".netrc",
	".pgpass",
	".htpasswd",
}

// sensitiveFileNamePatternsLower 预计算小写集合，便于快速查找
var sensitiveFileNameSet = func() map[string]bool {
	m := make(map[string]bool, len(sensitiveFileNamePatterns))
	for _, n := range sensitiveFileNamePatterns {
		m[strings.ToLower(n)] = true
	}
	return m
}()

// IsSensitiveFile 根据文件名判断是否为敏感文件（需整文件脱敏）
func IsSensitiveFile(filename string) bool {
	if filename == "" {
		return false
	}
	lower := strings.ToLower(filename)
	// 精确匹配
	if sensitiveFileNameSet[lower] {
		return true
	}
	// 前缀匹配 .env.* 变体
	if strings.HasPrefix(lower, ".env.") {
		return true
	}
	// 包含 credentials / secret / private_key 关键词
	if strings.Contains(lower, "credentials") ||
		strings.Contains(lower, "private_key") ||
		strings.HasSuffix(lower, ".pem") ||
		strings.HasSuffix(lower, ".key") {
		return true
	}
	return false
}

// ------------------------------------------------------------
// 内容脱敏
// ------------------------------------------------------------

// MaxRedactInputSize 限制脱敏输入长度，避免超长输入导致正则回溯
const MaxRedactInputSize = 10 * 1024 * 1024 // 10MB

// RedactSecrets 对文本内容进行密钥脱敏，返回脱敏后的内容与命中的密钥数量。
// 脱敏策略：
//   - 若 wholeFile 为 true，则整文件标记为 [REDACTED: 敏感文件]
//   - 否则逐条密钥模式匹配替换为占位符
//
// wholeFile=true 适用于 .env / 私钥等文件，避免逐行处理泄露结构信息。
func RedactSecrets(content string, wholeFile bool) (string, int) {
	if len(content) > MaxRedactInputSize {
		// 超长内容截断后再脱敏，防止正则 DoS
		content = content[:MaxRedactInputSize]
	}

	if wholeFile {
		return "[REDACTED: 敏感文件内容已隐藏，共 " + formatBytes(len(content)) + "]", 1
	}

	masked := content
	totalHits := 0
	for _, sp := range secretPatterns {
		// 限制单模式替换次数，防止异常输入导致过度回溯
		matches := sp.pattern.FindAllStringSubmatchIndex(masked, 64)
		if len(matches) == 0 {
			continue
		}
		// 从后往前替换，避免索引偏移
		mask := sp.mask
		for i := len(matches) - 1; i >= 0; i-- {
			m := matches[i]
			// 若 mask 含 ${1} 占位（保留前缀），用分组替换
			if strings.Contains(mask, "${1}") && len(m) >= 4 {
				replacement := strings.ReplaceAll(mask, "${1}", masked[m[2]:m[3]])
				masked = masked[:m[0]] + replacement + masked[m[1]:]
			} else {
				masked = masked[:m[0]] + mask + masked[m[1]:]
			}
			totalHits++
		}
	}
	return masked, totalHits
}

// formatBytes 友好格式化字节数
func formatBytes(n int) string {
	if n < 1024 {
		return formatInt(n) + " B"
	}
	if n < 1024*1024 {
		return formatInt(n/1024) + " KB"
	}
	return formatInt(n/(1024*1024)) + " MB"
}

// formatInt 避免引入 strconv 的额外依赖（本文件已聚焦）
func formatInt(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

// ------------------------------------------------------------
// 出站数据量监控
// ------------------------------------------------------------

// OutboundDataMonitor 追踪出站数据量，检测异常（如 Grok Build 的 27800× 泄露）。
// 设计：滑动窗口统计发送字节，当窗口内发送量超过阈值的倍数时触发告警回调。
type OutboundDataMonitor struct {
	// 窗口期内累计发送字节
	windowBytes int64
	// 窗口期起始时间
	windowStart int64 // unix nano
	// 窗口时长
	windowDuration time.Duration
	// 基准阈值（字节）：低于此值视为正常模型调用流量
	baselineBytes int64
	// 异常倍数阈值：发送量 / 基准 超过此值则告警
	anomalyMultiplier int64
	// 预计算的异常阈值（baselineBytes * anomalyMultiplier，溢出安全）
	anomalyThreshold int64
	// 当前窗口是否已触发告警（避免重复 goroutine 泄漏）
	anomalyFired bool
	// 告警回调
	onAnomaly func(stats OutboundStats)
	mu        sync.Mutex
}

// OutboundStats 出站数据统计快照
type OutboundStats struct {
	WindowBytes  int64
	Baseline     int64
	Ratio        float64
	WindowStart  time.Time
	WindowActive time.Duration
}

// NewOutboundDataMonitor 创建监控器。
//   - windowDuration：统计窗口（如 60s）
//   - baselineBytes：正常模型调用流量基准（如 1MB）
//   - anomalyMultiplier：异常倍数（如 100，即超过基准 100 倍告警）
//   - onAnomaly：告警回调（可为 nil）
func NewOutboundDataMonitor(windowDuration time.Duration, baselineBytes int64, anomalyMultiplier int64, onAnomaly func(OutboundStats)) *OutboundDataMonitor {
	if windowDuration <= 0 {
		windowDuration = 60 * time.Second
	}
	if baselineBytes <= 0 {
		baselineBytes = 1024 * 1024 // 1MB
	}
	if anomalyMultiplier <= 0 {
		anomalyMultiplier = 100
	}
	// 预计算异常阈值，防止 baselineBytes * anomalyMultiplier 整数溢出
	// （溢出可能产生负数或小正数，导致误报或漏报）
	threshold := baselineBytes * anomalyMultiplier
	if threshold/baselineBytes != anomalyMultiplier || threshold < 0 {
		threshold = 1<<63 - 1 // math.MaxInt64
	}
	return &OutboundDataMonitor{
		windowStart:       time.Now().UnixNano(),
		windowDuration:    windowDuration,
		baselineBytes:     baselineBytes,
		anomalyMultiplier: anomalyMultiplier,
		anomalyThreshold:  threshold,
		onAnomaly:         onAnomaly,
	}
}

// Record 记录一次出站发送的字节数，必要时触发告警。
func (m *OutboundDataMonitor) Record(bytes int) {
	if m == nil || bytes <= 0 {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now().UnixNano()
	// 窗口过期则重置
	if time.Duration(now-m.windowStart) >= m.windowDuration {
		m.windowBytes = 0
		m.windowStart = now
		m.anomalyFired = false
	}

	// 已持有互斥锁，直接累加（避免 atomic + mutex 混用反模式）
	m.windowBytes += int64(bytes)

	// 检测异常：每窗口仅触发一次告警，避免在高频记录下泄漏 goroutine
	if !m.anomalyFired && m.windowBytes > m.anomalyThreshold {
		m.anomalyFired = true
		if m.onAnomaly != nil {
			stats := OutboundStats{
				WindowBytes:  m.windowBytes,
				Baseline:     m.baselineBytes,
				Ratio:        float64(m.windowBytes) / float64(m.baselineBytes),
				WindowStart:  time.Unix(0, m.windowStart),
				WindowActive: time.Duration(now - m.windowStart),
			}
			// 异步触发回调，避免阻塞记录
			go m.onAnomaly(stats)
		}
	}
}

// Snapshot 返回当前窗口统计快照
func (m *OutboundDataMonitor) Snapshot() OutboundStats {
	if m == nil {
		return OutboundStats{}
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	now := time.Now().UnixNano()
	return OutboundStats{
		WindowBytes:  m.windowBytes,
		Baseline:     m.baselineBytes,
		Ratio:        float64(m.windowBytes) / float64(m.baselineBytes),
		WindowStart:  time.Unix(0, m.windowStart),
		WindowActive: time.Duration(now - m.windowStart),
	}
}
