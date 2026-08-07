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

package nodes

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// ========== 持久化索引 ==========

// ckgIndex 持久化索引结构
type ckgIndex struct {
	Path         string            `json:"path"`
	Entities     []ckgEntity       `json:"entities"`
	Relations    []ckgRelation     `json:"relations"`
	Concepts     []ckgConcept      `json:"concepts"`
	FileHashes   map[string]string `json:"file_hashes"` // 文件路径 -> SHA256哈希
	CreatedAt    time.Time         `json:"created_at"`
	UpdatedAt    time.Time         `json:"updated_at"`
	TotalFiles   int               `json:"total_files"`   // 索引文件总数
	TotalLines   int               `json:"total_lines"`   // 代码总行数
	TotalTokens  int               `json:"total_tokens"`  // 预估 Token 数（用于全量审查）
	TokensSaved  int               `json:"tokens_saved"`  // 通过增量更新节省的 Token
	SavingsRatio float64           `json:"savings_ratio"` // Token 节省比例（0-1）
	mu           sync.RWMutex      `json:"-"`
}

var (
	ckgIndexCache = make(map[string]*ckgIndex)
	ckgIndexMu    sync.RWMutex
	ckgIndexDir   = ".aflare-cache"
)

// computeFileHash 计算文件 SHA256 哈希
func computeFileHash(path string) (string, error) {
	data, err := os.ReadFile(path) // #nosec G304 -- internally generated index path
	if err != nil {
		return "", err
	}
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:]), nil
}

// loadIndex 从磁盘加载索引
func (n *CodeKnowledgeGraphNode) loadIndex(indexPath string) (*ckgIndex, error) {
	ckgIndexMu.RLock()
	if cached, ok := ckgIndexCache[indexPath]; ok {
		ckgIndexMu.RUnlock()
		return cached, nil
	}
	ckgIndexMu.RUnlock()

	data, err := os.ReadFile(indexPath) // #nosec G304 -- internally generated index path
	if err != nil {
		return nil, err
	}

	var idx ckgIndex
	if err := json.Unmarshal(data, &idx); err != nil {
		return nil, err
	}

	ckgIndexMu.Lock()
	ckgIndexCache[indexPath] = &idx
	ckgIndexMu.Unlock()

	return &idx, nil
}

// saveIndex 保存索引到磁盘
func (n *CodeKnowledgeGraphNode) saveIndex(idx *ckgIndex, indexPath string) error {
	idx.mu.Lock()
	idx.UpdatedAt = time.Now()
	data, err := json.MarshalIndent(idx, "", "  ")
	idx.mu.Unlock()
	if err != nil {
		return err
	}

	// 确保目录存在
	dir := filepath.Dir(indexPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	// 写入临时文件，然后原子重命名
	tmpPath := indexPath + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		return err
	}

	return os.Rename(tmpPath, indexPath)
}

// detectChangedFiles 检测变更的文件（新增、修改）
func (n *CodeKnowledgeGraphNode) detectChangedFiles(files []string, idx *ckgIndex) (added, modified []string) {
	if idx == nil {
		return files, nil
	}

	idx.mu.RLock()
	defer idx.mu.RUnlock()

	currentHashes := make(map[string]string)
	for _, f := range files {
		hash, err := computeFileHash(f)
		if err != nil {
			continue
		}
		currentHashes[f] = hash
	}

	for _, f := range files {
		currentHash, ok := currentHashes[f]
		if !ok {
			continue
		}

		oldHash, exists := idx.FileHashes[f]
		if !exists {
			added = append(added, f)
		} else if oldHash != currentHash {
			modified = append(modified, f)
		}
	}

	return added, modified
}

// buildIndexIncremental 增量构建索引
func (n *CodeKnowledgeGraphNode) buildIndexIncremental(root string, idx *ckgIndex, forceRebuild bool) (*ckgIndex, error) {
	files, err := n.collectFiles(root)
	if err != nil {
		return nil, err
	}

	// 计算预估 Token 数（用于显示节省比例）
	estimateTokensForFiles := func(fileList []string) int {
		totalTokens := 0
		for _, f := range fileList {
			data, err := os.ReadFile(f) // #nosec G304 -- internally generated index path
			if err != nil {
				continue
			}
			// 粗略估算：1 token ≈ 4 字符（英文）或 1.5 字符（中文/代码）
			tokens := len(data) / 3
			totalTokens += tokens
		}
		return totalTokens
	}

	if idx == nil || forceRebuild {
		// 全量构建
		idx = &ckgIndex{
			Path:       root,
			Entities:   []ckgEntity{},
			Relations:  []ckgRelation{},
			Concepts:   []ckgConcept{},
			FileHashes: make(map[string]string),
			CreatedAt:  time.Now(),
			UpdatedAt:  time.Now(),
			TotalFiles: len(files),
		}

		for _, f := range files {
			entities, relations := n.extractFromFile(f)
			idx.Entities = append(idx.Entities, entities...)
			idx.Relations = append(idx.Relations, relations...)

			if hash, err := computeFileHash(f); err == nil {
				idx.FileHashes[f] = hash
			}
		}

		idx.Concepts = n.extractConcepts(idx.Entities)
		idx.TotalTokens = estimateTokensForFiles(files)
		idx.TokensSaved = 0
		idx.SavingsRatio = 0
		return idx, nil
	}

	// 增量更新
	added, modified := n.detectChangedFiles(files, idx)

	if len(added) == 0 && len(modified) == 0 {
		// 无变更，直接返回
		return idx, nil
	}

	// 计算 Token 节省
	changedFiles := append(added, modified...)
	tokensForChanged := estimateTokensForFiles(changedFiles)
	tokensForAll := estimateTokensForFiles(files)

	idx.mu.Lock()
	defer idx.mu.Unlock()

	// 记录节省的 Token（全量审查需要的 - 实际增量审查的）
	previousTokensSaved := idx.TokensSaved
	idx.TokensSaved = previousTokensSaved + (tokensForAll - tokensForChanged)
	if tokensForAll > 0 {
		idx.SavingsRatio = float64(idx.TokensSaved) / float64(tokensForAll+idx.TokensSaved)
	}
	idx.TotalTokens = tokensForAll
	idx.TotalFiles = len(files)

	// 移除已修改文件的旧实体
	for _, f := range changedFiles {
		oldHash := idx.FileHashes[f]
		if oldHash != "" {
			// 移除该文件相关的实体和关系
			newEntities := make([]ckgEntity, 0)
			for _, e := range idx.Entities {
				if e.Location != f {
					newEntities = append(newEntities, e)
				}
			}
			idx.Entities = newEntities

			newRelations := make([]ckgRelation, 0)
			for _, r := range idx.Relations {
				// 简单起见，保留所有关系（实际应更精确）
				newRelations = append(newRelations, r)
			}
			idx.Relations = newRelations
		}
	}

	// 提取新实体
	for _, f := range changedFiles {
		entities, relations := n.extractFromFile(f)
		idx.Entities = append(idx.Entities, entities...)
		idx.Relations = append(idx.Relations, relations...)

		if hash, err := computeFileHash(f); err == nil {
			idx.FileHashes[f] = hash
		}
	}

	// 清理不存在的文件
	deletedFiles := make([]string, 0)
	for f := range idx.FileHashes {
		exists := false
		for _, currentFile := range files {
			if currentFile == f {
				exists = true
				break
			}
		}
		if !exists {
			deletedFiles = append(deletedFiles, f)
		}
	}

	for _, f := range deletedFiles {
		delete(idx.FileHashes, f)
		// 移除该文件的实体
		newEntities := make([]ckgEntity, 0)
		for _, e := range idx.Entities {
			if e.Location != f {
				newEntities = append(newEntities, e)
			}
		}
		idx.Entities = newEntities
	}

	idx.Concepts = n.extractConcepts(idx.Entities)
	idx.UpdatedAt = time.Now()

	return idx, nil
}

// GetTokenSavingsReport 生成 Token 节省统计报告
func (n *CodeKnowledgeGraphNode) GetTokenSavingsReport(indexPath string) string {
	idx, err := n.loadIndex(indexPath)
	if err != nil {
		return "❌ 无法加载索引"
	}

	idx.mu.RLock()
	defer idx.mu.RUnlock()

	var report string
	report += "📊 Token 节省统计\n"
	report += "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n\n"

	report += fmt.Sprintf("📁 索引路径: %s\n", idx.Path)
	report += fmt.Sprintf("📄 文件总数: %d\n", idx.TotalFiles)
	report += fmt.Sprintf("🔢 实体数量: %d\n", len(idx.Entities))
	report += fmt.Sprintf("🔗 关系数量: %d\n", len(idx.Relations))
	report += fmt.Sprintf("💡 概念数量: %d\n\n", len(idx.Concepts))

	if idx.TotalTokens > 0 {
		report += "📈 Token 分析\n"
		report += "─────────────────────────────────────────\n"
		report += fmt.Sprintf("  全量审查预估 Token: %d\n", idx.TotalTokens)

		if idx.TokensSaved > 0 {
			report += fmt.Sprintf("  累计节省 Token: %d\n", idx.TokensSaved)
			savingsPercent := idx.SavingsRatio * 100
			report += fmt.Sprintf("  节省比例: %.1f%%\n", savingsPercent)

			// 计算节省的成本（假设 GPT-4o-mini 定价）
			costPer1K := 0.00015 // USD
			savedCost := float64(idx.TokensSaved) / 1000 * costPer1K
			report += fmt.Sprintf("  节省成本: $%.4f\n", savedCost)
		} else {
			report += "  （首次索引，无节省数据）\n"
		}
	}

	report += fmt.Sprintf("\n🕐 创建时间: %s\n", idx.CreatedAt.Format("2006-01-02 15:04:05"))
	report += fmt.Sprintf("🕐 更新时间: %s\n", idx.UpdatedAt.Format("2006-01-02 15:04:05"))

	return report
}

// GetCompactSavingsReport 生成简洁的 Token 节省报告
func (n *CodeKnowledgeGraphNode) GetCompactSavingsReport(indexPath string) string {
	idx, err := n.loadIndex(indexPath)
	if err != nil {
		return ""
	}

	idx.mu.RLock()
	defer idx.mu.RUnlock()

	if idx.TokensSaved > 0 {
		savingsPercent := idx.SavingsRatio * 100
		return fmt.Sprintf("📊 Token 节省: %d (%.1f%%) | 文件: %d | 实体: %d",
			idx.TokensSaved, savingsPercent, idx.TotalFiles, len(idx.Entities))
	}
	return fmt.Sprintf("📊 文件: %d | 实体: %d | 首次索引", idx.TotalFiles, len(idx.Entities))
}

// getIndexFilePath 获取索引文件路径
func (n *CodeKnowledgeGraphNode) getIndexFilePath(root string) string {
	absPath, err := filepath.Abs(root)
	if err != nil {
		absPath = root // fallback to raw path
	}
	hash := sha256.Sum256([]byte(absPath))
	hashStr := hex.EncodeToString(hash[:16]) // Use 16 bytes for lower collision risk
	return filepath.Join(os.TempDir(), ckgIndexDir, fmt.Sprintf("ckg-index-%s.json", hashStr))
}
