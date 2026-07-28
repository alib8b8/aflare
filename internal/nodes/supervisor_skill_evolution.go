// Copyright (c) 2026 llm-box Contributors
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

package nodes

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

// ============================================================
// Skill 自演进机制（借鉴 jiuwenswarm 的 Skill 自演进）
// Agent 技能越用越强：自动识别异常、优化技能、积累经验
// ============================================================

// SkillRecord 记录一个技能的使用情况和效果
type SkillRecord struct {
	Name            string   `json:"name"`
	Description     string   `json:"description"`
	UseCount        int      `json:"use_count"`
	SuccessCount    int      `json:"success_count"`
	FailCount       int      `json:"fail_count"`
	SuccessRate     float64  `json:"success_rate"`
	AvgLatencyMs    int64    `json:"avg_latency_ms"`
	BestPractices   []string `json:"best_practices,omitempty"`
	KnownPitfalls   []string `json:"known_pitfalls,omitempty"`
	OptimizedPrompt string   `json:"optimized_prompt,omitempty"`
	CreatedAt       string   `json:"created_at"`
	UpdatedAt       string   `json:"updated_at"`
}

// SkillEvolution Skill 自演进引擎
type SkillEvolution struct {
	skills    map[string]*SkillRecord
	maxSkills int
	mu        sync.RWMutex
}

const (
	defaultMaxSkills      = 100
	maxBestPractices      = 20
	maxKnownPitfalls      = 20
	maxOptimizedPromptLen = 4096
)

// NewSkillEvolution 创建技能自演进引擎
func NewSkillEvolution() *SkillEvolution {
	return &SkillEvolution{
		skills:    make(map[string]*SkillRecord),
		maxSkills: defaultMaxSkills,
	}
}

// RecordExecution 记录一次技能执行结果，自动更新成功率
func (se *SkillEvolution) RecordExecution(skillName string, success bool, latencyMs int64) {
	if skillName == "" || len(skillName) > 100 {
		return
	}
	// SE-3: latencyMs 边界校验，防止异常值污染统计数据（最大 1 小时）
	if latencyMs < 0 || latencyMs > 3600000 {
		return
	}

	se.mu.Lock()
	defer se.mu.Unlock()

	skill, exists := se.skills[skillName]
	if !exists {
		if len(se.skills) >= se.maxSkills {
			// SE-4: 达到上限时输出日志，避免静默数据丢失
			fmt.Printf("[SkillEvolution] maxSkills limit reached (%d), skipping new skill: %s\n", se.maxSkills, skillName)
			return // 达到上限，不再添加新技能
		}
		skill = &SkillRecord{
			Name:      skillName,
			CreatedAt: time.Now().Format(time.RFC3339),
		}
		se.skills[skillName] = skill
	}

	skill.UseCount++
	if success {
		skill.SuccessCount++
	} else {
		skill.FailCount++
	}

	// 更新成功率
	if skill.UseCount > 0 {
		skill.SuccessRate = float64(skill.SuccessCount) / float64(skill.UseCount)
	}

	// 更新平均延迟（滑动平均）
	if skill.AvgLatencyMs == 0 {
		skill.AvgLatencyMs = latencyMs
	} else {
		skill.AvgLatencyMs = (skill.AvgLatencyMs*7 + latencyMs*3) / 10
	}

	skill.UpdatedAt = time.Now().Format(time.RFC3339)
}

// AddBestPractice 添加最佳实践
func (se *SkillEvolution) AddBestPractice(skillName, practice string) {
	// SE-5: skillName 长度校验
	if skillName == "" || len(skillName) > 100 {
		return
	}
	if practice == "" || len(practice) > 500 {
		return
	}

	se.mu.Lock()
	defer se.mu.Unlock()

	skill, exists := se.skills[skillName]
	if !exists {
		return
	}
	// 去重
	for _, bp := range skill.BestPractices {
		if bp == practice {
			return
		}
	}
	if len(skill.BestPractices) >= maxBestPractices {
		// SE-6: 显式复制，避免底层 array 内存滞留
		skill.BestPractices = append([]string(nil), skill.BestPractices[1:]...) // 移除最旧的
	}
	skill.BestPractices = append(skill.BestPractices, practice)
}

// AddKnownPitfall 添加已知陷阱
func (se *SkillEvolution) AddKnownPitfall(skillName, pitfall string) {
	// SE-5: skillName 长度校验
	if skillName == "" || len(skillName) > 100 {
		return
	}
	if pitfall == "" || len(pitfall) > 500 {
		return
	}

	se.mu.Lock()
	defer se.mu.Unlock()

	skill, exists := se.skills[skillName]
	if !exists {
		return
	}
	for _, kp := range skill.KnownPitfalls {
		if kp == pitfall {
			return
		}
	}
	if len(skill.KnownPitfalls) >= maxKnownPitfalls {
		// SE-6: 显式复制，避免底层 array 内存滞留
		skill.KnownPitfalls = append([]string(nil), skill.KnownPitfalls[1:]...)
	}
	skill.KnownPitfalls = append(skill.KnownPitfalls, pitfall)
}

// OptimizePrompt 根据历史经验优化技能的 prompt
func (se *SkillEvolution) OptimizePrompt(skillName, basePrompt string) string {
	se.mu.RLock()
	defer se.mu.RUnlock()

	skill, exists := se.skills[skillName]
	if !exists || skill.UseCount < 3 {
		return basePrompt // 数据不足，不优化
	}

	// 如果成功率低于 60%，添加已知陷阱提示
	if skill.SuccessRate < 0.6 && len(skill.KnownPitfalls) > 0 {
		basePrompt += "\n\nKnown pitfalls to avoid:\n"
		for i, pitfall := range skill.KnownPitfalls {
			if i >= 5 {
				break
			}
			// SE-2: 对 pitfall 文本进行转义，防止间接 prompt 注入
			basePrompt += fmt.Sprintf("- %s\n", sanitizeForPrompt(pitfall))
		}
	}

	// 如果有最佳实践，添加到 prompt
	if len(skill.BestPractices) > 0 {
		basePrompt += "\n\nBest practices:\n"
		for i, bp := range skill.BestPractices {
			if i >= 5 {
				break
			}
			// SE-2: 对 bp 文本进行转义，防止间接 prompt 注入
			basePrompt += fmt.Sprintf("- %s\n", sanitizeForPrompt(bp))
		}
	}

	// SE-7: 按 rune 截断，避免破坏 UTF-8 字符
	if len(basePrompt) > maxOptimizedPromptLen {
		runes := []rune(basePrompt)
		if len(runes) > maxOptimizedPromptLen {
			basePrompt = string(runes[:maxOptimizedPromptLen])
		}
	}

	return basePrompt
}

// sanitizeForPrompt 对用户提供的文本进行转义，去除换行符、制表符，
// 只保留可打印字符，防止间接 prompt 注入。
func sanitizeForPrompt(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		// 去除换行符、制表符及其他控制字符
		if r == '\n' || r == '\r' || r == '\t' {
			continue
		}
		// 只允许可打印字符（控制字符与 DEL 移除）
		if r < 0x20 || r == 0x7f {
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

// GetSkill 获取技能记录
func (se *SkillEvolution) GetSkill(skillName string) (*SkillRecord, bool) {
	se.mu.RLock()
	defer se.mu.RUnlock()
	skill, ok := se.skills[skillName]
	if !ok {
		return nil, false
	}
	// SE-1: 返回深拷贝，避免锁外数据竞争
	return cloneSkillRecord(skill), true
}

// ListSkills 列出所有技能
func (se *SkillEvolution) ListSkills() []*SkillRecord {
	se.mu.RLock()
	defer se.mu.RUnlock()
	result := make([]*SkillRecord, 0, len(se.skills))
	for _, skill := range se.skills {
		// SE-1: 返回每个元素的深拷贝
		result = append(result, cloneSkillRecord(skill))
	}
	return result
}

// GetLowPerformingSkills 返回成功率低于阈值的技能（需要改进）
func (se *SkillEvolution) GetLowPerformingSkills(threshold float64) []*SkillRecord {
	if threshold < 0 || threshold > 1 {
		threshold = 0.6
	}
	se.mu.RLock()
	defer se.mu.RUnlock()
	var result []*SkillRecord
	for _, skill := range se.skills {
		if skill.UseCount >= 3 && skill.SuccessRate < threshold {
			// SE-1: 返回深拷贝
			result = append(result, cloneSkillRecord(skill))
		}
	}
	return result
}

// cloneSkillRecord 深拷贝 SkillRecord，包括其切片字段，
// 确保调用方在锁外访问的副本与内部 map 中的记录互不影响。
func cloneSkillRecord(s *SkillRecord) *SkillRecord {
	if s == nil {
		return nil
	}
	cp := *s
	if s.BestPractices != nil {
		cp.BestPractices = append([]string(nil), s.BestPractices...)
	}
	if s.KnownPitfalls != nil {
		cp.KnownPitfalls = append([]string(nil), s.KnownPitfalls...)
	}
	return &cp
}

// GetSkillStats 返回技能统计概览
func (se *SkillEvolution) GetSkillStats() map[string]interface{} {
	se.mu.RLock()
	defer se.mu.RUnlock()

	totalUses := 0
	totalSuccess := 0
	for _, skill := range se.skills {
		totalUses += skill.UseCount
		totalSuccess += skill.SuccessCount
	}

	avgSuccessRate := 0.0
	if totalUses > 0 {
		avgSuccessRate = float64(totalSuccess) / float64(totalUses)
	}

	return map[string]interface{}{
		"total_skills":     len(se.skills),
		"total_executions": totalUses,
		"total_success":    totalSuccess,
		"avg_success_rate": avgSuccessRate,
	}
}
