// Copyright (c) 2026 aflare Contributors
//
// aflare‍​‌​​​​​‌​‌​​​‌‌​​‌​​‌‌​​​‌​‌​​‌​​​​​​​‌​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​​‌‌​‌​‌​‌​​​‌​‌‌‌‌‌‌‌​​​​​​​‌‌​‌‌‌‌‌​​​​​‌‌​​​​‌‌‌‌​‌​‌‌‌​‌‌‌‌‌​‌‌‌‌​​‌​​‌​​‌‌​​​​​​​​​​​​​​​​​​‌‌‌‌​​​‌‌‌​‌‌‌​‌⁠
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published
// by the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

package nodes

import (
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/alib8b8/aflare/internal/logger"
	"github.com/alib8b8/aflare/internal/nodes/core"
)

// secretdetect.go — 密钥脱敏与出站数据量监控
//
// 借鉴 Grok Build 隐私丑闻的教训：默认上传整个仓库且 .env 不脱敏。
// 本模块实现"隐私优先"原则：
//   1. 文件读取时自动识别并脱敏常见密钥格式（API Key / Token / 密码）
//   2. .env / .env.* / 含密钥特征的文件按整文件脱敏
//   3. 出站数据量监控：追踪累计发送字节，异常倍数告警
//
// 密钥模式识别与 RedactSecrets 的实现已下沉到 internal/nodes/core
// （core/security.go），以便 LLM 出口路径（package core 与 providers）
// 在不引入循环依赖的前提下复用同一份脱敏逻辑；本文件的 RedactSecrets
// 委托到 core.RedactSecrets，保持原有公开 API 不变。

// ------------------------------------------------------------
// 敏感文件名识别
// ------------------------------------------------------------

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
//
// 实现已下沉到 core.RedactSecrets（见 core/security.go），此处委托以保持
// 原有公开 API（file_read 等调用方）不变，并避免与 core 重复维护密钥模式。
func RedactSecrets(content string, wholeFile bool) (string, int) {
	return core.RedactSecrets(content, wholeFile)
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
			go func() {
				defer func() { _ = recover() }()
				m.onAnomaly(stats)
			}()
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

// ------------------------------------------------------------
// 全局出站数据监控器（singleton）
// ------------------------------------------------------------
//
// OutboundDataMonitor 之前没有任何生产代码调用——README 宣称的"出站数据量
// 异常监控"形同虚设。这里提供一个进程级 singleton，供 LLM 出口与 HTTP 出口
// 节点统一上报出站字节数；当窗口内发送量超过基准的指定倍数时打 Warn 日志。
//
// core 与 providers 无法直接 import nodes（会循环依赖），因此 nodes 在 init
// 时把本 singleton 通过 core.SetGlobalOutboundRecorder 注入到 core，core 与
// providers 改用 core.RecordOutbound 上报。

const (
	envOutboundMonitorDisable       = "AFLARE_OUTBOUND_MONITOR_DISABLE"
	envOutboundMonitorBaselineBytes = "AFLARE_OUTBOUND_MONITOR_BASELINE_BYTES"
	envOutboundMonitorMultiplier    = "AFLARE_OUTBOUND_MONITOR_MULTIPLIER"
	envOutboundMonitorWindow        = "AFLARE_OUTBOUND_MONITOR_WINDOW"

	defaultOutboundWindow        = 60 * time.Second
	defaultOutboundBaselineBytes = int64(1024 * 1024) // 1MB
	defaultOutboundMultiplier    = int64(100)
)

var (
	globalOutboundMonitorOnce sync.Once
	globalOutboundMonitor     *OutboundDataMonitor
)

// newGlobalOutboundMonitorFromEnv 根据环境变量构造一个监控器；当
// AFLARE_OUTBOUND_MONITOR_DISABLE=1 时返回 nil。每次调用都重新读取环境变量，
// 便于单元测试；进程级 singleton（GetGlobalOutboundMonitor）通过 sync.Once
// 缓存首次结果。
func newGlobalOutboundMonitorFromEnv() *OutboundDataMonitor {
	if v := strings.ToLower(strings.TrimSpace(os.Getenv(envOutboundMonitorDisable))); v == "1" || v == "true" {
		return nil
	}

	window := defaultOutboundWindow
	if raw := os.Getenv(envOutboundMonitorWindow); raw != "" {
		if d, err := time.ParseDuration(raw); err == nil && d > 0 {
			window = d
		}
	}

	baseline := defaultOutboundBaselineBytes
	if raw := os.Getenv(envOutboundMonitorBaselineBytes); raw != "" {
		if n, err := strconv.ParseInt(raw, 10, 64); err == nil && n > 0 {
			baseline = n
		}
	}

	multiplier := defaultOutboundMultiplier
	if raw := os.Getenv(envOutboundMonitorMultiplier); raw != "" {
		if n, err := strconv.ParseInt(raw, 10, 64); err == nil && n > 0 {
			multiplier = n
		}
	}

	return NewOutboundDataMonitor(window, baseline, multiplier, func(s OutboundStats) {
		// 用 internal/logger 打 Warn（不用标准 log）。OutboundDataMonitor.Record
		// 已在异步 goroutine 中 recover，回调 panic 不会影响主流程。
		logger.Warn("[security] outbound data anomaly detected",
			"window_bytes", s.WindowBytes,
			"baseline", s.Baseline,
			"ratio", s.Ratio,
		)
	})
}

// GetGlobalOutboundMonitor 返回进程级出站数据监控器 singleton，首次调用时
// 根据 AFLARE_OUTBOUND_MONITOR_* 环境变量初始化。当
// AFLARE_OUTBOUND_MONITOR_DISABLE=1 时返回 nil（监控关闭）。结果通过
// sync.Once 缓存整个进程生命周期，请在进程启动前设置环境变量。
func GetGlobalOutboundMonitor() *OutboundDataMonitor {
	globalOutboundMonitorOnce.Do(func() {
		globalOutboundMonitor = newGlobalOutboundMonitorFromEnv()
	})
	return globalOutboundMonitor
}

func init() {
	// 把 nodes 拥有的 monitor 注入 core，使 core 与 providers 能通过
	// core.RecordOutbound 上报出站字节，避免 core 反向 import nodes。
	core.SetGlobalOutboundRecorder(GetGlobalOutboundMonitor())
}
