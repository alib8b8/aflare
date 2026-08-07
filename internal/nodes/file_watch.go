// Copyright (c) 2026 aflare Contributors
//
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

// file_watch.go 实现 file_watch 节点，监控文件系统变化（创建/修改/删除）。
// 借鉴 Grok Build 的自研文件监控思路，采用轮询对比而非 fsnotify，
// 以避免引入第三方依赖。适用于 log-monitor、file-organizer 等工作流模板。
package nodes

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// FileWatchNode 监控文件系统变化的节点
type FileWatchNode struct{}

func init() {
	Register(&FileWatchNode{})
}

// file_watch 相关的安全限制常量
const (
	fwMaxDepth         = 5     // 最大递归深度（子目录层数）
	fwMaxFiles         = 10000 // 单次快照最大文件数（防 DoS）
	fwMaxPathLen       = 4096  // 输出路径最大长度
	fwMinDuration      = 1 * time.Second
	fwMaxDuration      = 600 * time.Second // 10 分钟
	fwMinInterval      = 100 * time.Millisecond
	fwMaxInterval      = 60 * time.Second
	fwDefaultMaxEvents = 1000
	fwHardMaxEvents    = 100000 // 最大事件数硬上限（防 DoS）
)

// fileMeta 保存文件元数据用于快照对比
type fileMeta struct {
	ModTime time.Time
	Size    int64
}

// watchEvent 表示一次文件变化事件
type watchEvent struct {
	Type      string `json:"type"`
	Path      string `json:"path"`
	Timestamp string `json:"timestamp"`
	Size      int64  `json:"size,omitempty"`
}

// watchOutput 是节点返回的 JSON 结构
type watchOutput struct {
	WatchedPath     string       `json:"watched_path"`
	Duration        string       `json:"duration"`
	EventsCollected int          `json:"events_collected"`
	Events          []watchEvent `json:"events"`
}

// Name 返回节点名
func (n *FileWatchNode) Name() string {
	return "file_watch"
}

// Description 返回节点描述
func (n *FileWatchNode) Description() string {
	return "Watch a file or directory for changes (create/modify/delete events)"
}

// Schema 返回节点 schema
func (n *FileWatchNode) Schema() NodeSchema {
	return NodeSchema{
		Name:        "file_watch",
		Description: "Polls a file or directory for create/modify/delete events and returns them as JSON. Suitable for log-monitor and file-organizer workflows.",
		Input:       "string - not used",
		Output:      "string - JSON with watched_path, duration, events_collected, and events array",
		Params: []ParamSchema{
			{Name: "path", Type: "string", Description: "要监控的文件或目录路径", Required: true},
			{Name: "duration", Type: "string", Description: "监控持续时间（如 30s, 5m）", Required: false, Default: "30s"},
			{Name: "interval", Type: "string", Description: "轮询间隔（如 1s, 500ms）", Required: false, Default: "1s"},
			{Name: "events", Type: "string", Description: "关注的事件类型，逗号分隔：create,modify,delete", Required: false, Default: "create,modify,delete"},
			{Name: "pattern", Type: "string", Description: "文件名 glob 匹配模式", Required: false, Default: "*"},
			{Name: "max_events", Type: "string", Description: "最大收集事件数（防 DoS）", Required: false, Default: "1000"},
		},
	}
}

// Execute 实现 Node 接口
func (n *FileWatchNode) Execute(ctx context.Context, input string, params map[string]string) (string, error) {
	// 1. 校验 path（使用统一的 validateReadPath 安全校验）
	userPath := params["path"]
	if userPath == "" {
		return "", fmt.Errorf("path parameter is required")
	}
	safePath, err := validateReadPath(userPath)
	if err != nil {
		return "", fmt.Errorf("path validation failed: %w", err)
	}

	// 2. 解析 duration，范围 1s ~ 600s
	durationStr := getParam(params, "duration", "30s")
	duration, err := time.ParseDuration(durationStr)
	if err != nil {
		return "", fmt.Errorf("invalid duration %q: %w", durationStr, err)
	}
	if duration < fwMinDuration || duration > fwMaxDuration {
		return "", fmt.Errorf("duration must be between %s and %s, got %s", fwMinDuration, fwMaxDuration, duration)
	}

	// 3. 解析 interval，范围 100ms ~ 60s，且必须 < duration
	intervalStr := getParam(params, "interval", "1s")
	interval, err := time.ParseDuration(intervalStr)
	if err != nil {
		return "", fmt.Errorf("invalid interval %q: %w", intervalStr, err)
	}
	if interval < fwMinInterval || interval > fwMaxInterval {
		return "", fmt.Errorf("interval must be between %s and %s, got %s", fwMinInterval, fwMaxInterval, interval)
	}
	if interval >= duration {
		return "", fmt.Errorf("interval (%s) must be less than duration (%s)", interval, duration)
	}

	// 4. 解析 events 白名单：只允许 create/modify/delete
	eventsStr := getParam(params, "events", "create,modify,delete")
	wantedEvents, err := parseWatchEvents(eventsStr)
	if err != nil {
		return "", err
	}

	// 5. 校验 pattern：拒绝含 .. 的模式，并用 filepath.Match 校验语法
	pattern := getParam(params, "pattern", "*")
	if strings.Contains(pattern, "..") {
		return "", fmt.Errorf("pattern must not contain '..': %q", pattern)
	}
	if _, err := filepath.Match(pattern, ""); err != nil {
		return "", fmt.Errorf("invalid pattern %q: %w", pattern, err)
	}

	// 6. 解析 max_events
	maxEventsStr := getParam(params, "max_events", "1000")
	maxEvents, err := parseIntWithDefault(maxEventsStr, fwDefaultMaxEvents)
	if err != nil {
		return "", fmt.Errorf("invalid max_events %q: %w", maxEventsStr, err)
	}
	if maxEvents <= 0 {
		return "", fmt.Errorf("max_events must be positive, got %d", maxEvents)
	}
	if maxEvents > fwHardMaxEvents {
		maxEvents = fwHardMaxEvents
	}

	// 7. 启动时快照目标路径
	current, err := snapshotPath(safePath, userPath, pattern)
	if err != nil {
		return "", fmt.Errorf("initial snapshot failed: %w", err)
	}

	// 8. 子 context：duration 到期后自动取消（同时响应父 ctx 取消）
	watchCtx, cancel := context.WithTimeout(ctx, duration)
	defer cancel()

	// 9. 轮询对比，收集事件
	events := make([]watchEvent, 0, 16)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-watchCtx.Done():
			// context 取消（duration 到期或父 ctx 取消）：立即返回已收集事件
			return buildWatchOutput(userPath, duration, events)
		case <-ticker.C:
			next, err := snapshotPath(safePath, userPath, pattern)
			if err != nil {
				// 单次快照失败不致命，跳过本轮继续监控
				continue
			}
			diff := diffSnapshots(current, next, wantedEvents)
			if len(diff) > 0 {
				events = append(events, diff...)
			}
			current = next
			// 达到 max_events 上限：截断后立即停止
			if len(events) >= maxEvents {
				if len(events) > maxEvents {
					events = events[:maxEvents]
				}
				return buildWatchOutput(userPath, duration, events)
			}
		}
	}
}

// parseWatchEvents 解析 events 参数并校验白名单（只允许 create/modify/delete）
func parseWatchEvents(s string) (map[string]bool, error) {
	valid := map[string]bool{"create": true, "modify": true, "delete": true}
	result := make(map[string]bool)
	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if !valid[part] {
			return nil, fmt.Errorf("invalid event type %q (allowed: create, modify, delete)", part)
		}
		result[part] = true
	}
	if len(result) == 0 {
		return nil, fmt.Errorf("events parameter cannot be empty")
	}
	return result, nil
}

// parseIntWithDefault 将字符串解析为整数，空字符串返回默认值
func parseIntWithDefault(s string, defaultVal int) (int, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return defaultVal, nil
	}
	var n int
	if _, err := fmt.Sscanf(s, "%d", &n); err != nil {
		return 0, fmt.Errorf("not a valid integer: %w", err)
	}
	return n, nil
}

// snapshotPath 递归遍历目标路径并生成 (路径 -> fileMeta) 映射。
// userPath 用于在事件输出中构造用户可读的路径（保留用户输入的相对路径前缀）。
// 安全要点：用 Lstat 不跟踪符号链接；限制深度和文件数。
func snapshotPath(rootPath, userPath, pattern string) (map[string]fileMeta, error) {
	result := make(map[string]fileMeta)

	info, err := os.Lstat(rootPath)
	if err != nil {
		if os.IsNotExist(err) {
			// 路径不存在：返回空快照，后续轮询中可能出现 create 事件
			return result, nil
		}
		return nil, err
	}

	// 不跟踪符号链接（根路径本身是 symlink 时返回空）
	if info.Mode()&os.ModeSymlink != 0 {
		return result, nil
	}

	// 单个文件：键为 userPath
	if !info.IsDir() {
		matched, _ := filepath.Match(pattern, filepath.Base(rootPath))
		if matched {
			result[sanitizePath(userPath)] = fileMeta{
				ModTime: info.ModTime(),
				Size:    info.Size(),
			}
		}
		return result, nil
	}

	// 目录递归（WalkDir 内部使用 Lstat，不跟踪符号链接）
	err = filepath.WalkDir(rootPath, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil //nolint:nilerr // WalkDir callback: skip access error, continue traversal
		}
		// 不跟踪符号链接
		if d.Type()&os.ModeSymlink != 0 {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		// 跳过根目录本身
		if p == rootPath {
			return nil
		}
		// 计算相对路径与深度
		rel, err := filepath.Rel(rootPath, p)
		if err != nil {
			return nil //nolint:nilerr // WalkDir callback: skip on error, continue traversal
		}
		relSlash := filepath.ToSlash(rel)
		depthCount := strings.Count(relSlash, "/")
		if depthCount > fwMaxDepth {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		// 只关注文件，跳过目录
		if d.IsDir() {
			return nil
		}
		// 单次快照文件数限制（防 DoS）
		if len(result) >= fwMaxFiles {
			return filepath.SkipDir
		}
		// 模式匹配
		matched, _ := filepath.Match(pattern, d.Name())
		if !matched {
			return nil
		}
		// 获取详细的 modtime/size
		fi, err := d.Info()
		if err != nil {
			return nil //nolint:nilerr // WalkDir callback: skip on error, continue traversal
		}
		// 键 = userPath + "/" + rel（用户可读的相对路径）
		key := sanitizePath(filepath.ToSlash(filepath.Join(userPath, rel)))
		result[key] = fileMeta{
			ModTime: fi.ModTime(),
			Size:    fi.Size(),
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

// diffSnapshots 对比两个快照，返回事件列表。
// 新增文件 → create；modtime/size 变化 → modify；文件消失 → delete。
func diffSnapshots(old, cur map[string]fileMeta, wanted map[string]bool) []watchEvent {
	// 同一轮 diff 共享同一个时间戳，保证事件时间一致性
	now := time.Now().UTC().Format(time.RFC3339)
	var events []watchEvent

	// 新增和修改
	for path, curMeta := range cur {
		oldMeta, existed := old[path]
		if !existed {
			if wanted["create"] {
				events = append(events, watchEvent{
					Type:      "create",
					Path:      path,
					Timestamp: now,
					Size:      curMeta.Size,
				})
			}
		} else if oldMeta.ModTime != curMeta.ModTime || oldMeta.Size != curMeta.Size {
			if wanted["modify"] {
				events = append(events, watchEvent{
					Type:      "modify",
					Path:      path,
					Timestamp: now,
					Size:      curMeta.Size,
				})
			}
		}
	}
	// 删除
	for path := range old {
		if _, existed := cur[path]; !existed {
			if wanted["delete"] {
				events = append(events, watchEvent{
					Type:      "delete",
					Path:      path,
					Timestamp: now,
				})
			}
		}
	}

	// 按类型和路径排序，保证输出确定性
	sort.Slice(events, func(i, j int) bool {
		if events[i].Type != events[j].Type {
			return events[i].Type < events[j].Type
		}
		return events[i].Path < events[j].Path
	})
	return events
}

// sanitizePath 清洗路径输出：移除控制字符，限制长度。
// 防止文件名中嵌入的控制字符破坏 JSON 输出或被用于终端转义攻击。
func sanitizePath(p string) string {
	if p == "" {
		return ""
	}
	var b strings.Builder
	b.Grow(len(p))
	for _, r := range p {
		// 保留可见 ASCII 字符（>= 0x20）和制表符，移除其他控制字符
		if r >= 0x20 || r == '\t' {
			b.WriteRune(r)
		}
	}
	s := b.String()
	if len(s) > fwMaxPathLen {
		s = s[:fwMaxPathLen]
	}
	return s
}

// buildWatchOutput 构造最终 JSON 输出
func buildWatchOutput(watchedPath string, duration time.Duration, events []watchEvent) (string, error) {
	if events == nil {
		events = []watchEvent{}
	}
	out := watchOutput{
		WatchedPath:     watchedPath,
		Duration:        duration.String(),
		EventsCollected: len(events),
		Events:          events,
	}
	data, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal watch output: %w", err)
	}
	return string(data), nil
}
