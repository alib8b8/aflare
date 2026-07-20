package nodes

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/alib8b8/llm-box/internal/logger"
)

// 代码图谱节点的安全限制常量
const (
	codeGraphMaxPathLen    = 1024             // 输入路径最大长度
	codeGraphMaxDepth      = 5                // 目录遍历最大深度
	codeGraphMaxRegexMatch = 1000             // 每个正则单次匹配最大次数（防 ReDoS）
	codeGraphMaxNameLen    = 256              // 函数名/调用名最大长度
	codeGraphHardMaxFiles  = 10000            // 文件数硬上限
	codeGraphHardMaxSize   = 10 * 1024 * 1024 // 单文件大小硬上限 10MB
)

// 预编译正则表达式（包级变量），避免每次执行重新编译
var (
	// --- Go ---
	cgGoImportSingleRe    = regexp.MustCompile(`^\s*import\s+(?:[\w./]+\s+)?"([^"]+)"`)
	cgGoImportBlockLineRe = regexp.MustCompile(`^\s*(?:[\w./]+\s+)?"([^"]+)"`)
	cgGoFuncRe            = regexp.MustCompile(`^func\s+(?:\([^)]*\)\s+)?([A-Za-z_]\w*)\s*\(`)
	cgGoCallRe            = regexp.MustCompile(`([A-Za-z_][\w.]*)\s*\(`)

	// --- Python ---
	cgPyImportRe     = regexp.MustCompile(`^\s*import\s+([\w.]+)`)
	cgPyFromImportRe = regexp.MustCompile(`^\s*from\s+([\w.]+)\s+import\s`)
	cgPyFuncRe       = regexp.MustCompile(`^\s*def\s+([A-Za-z_]\w*)\s*\(`)
	cgPyClassRe      = regexp.MustCompile(`^\s*class\s+([A-Za-z_]\w*)`)
	cgPyCallRe       = regexp.MustCompile(`([A-Za-z_][\w.]*)\s*\(`)

	// --- JavaScript / TypeScript ---
	cgJsImportFromRe = regexp.MustCompile(`^\s*import\s+.*?\sfrom\s+['"]([^'"]+)['"]`)
	cgJsImportBareRe = regexp.MustCompile(`^\s*import\s+['"]([^'"]+)['"]`)
	cgJsFuncRe       = regexp.MustCompile(`^\s*function\s+([A-Za-z_$][\w$]*)\s*\(`)
	cgJsArrowFuncRe  = regexp.MustCompile(`^\s*(?:const|let|var)\s+([A-Za-z_$][\w$]*)\s*=\s*(?:async\s*)?\(?[^=]*=>`)
	cgJsCallRe       = regexp.MustCompile(`([A-Za-z_$][\w.]*)\s*\(`)
)

// cgControlKeywords 控制流关键字（提取调用时过滤，避免把 if/for 等误认为调用）
var cgControlKeywords = map[string]bool{
	"if": true, "for": true, "while": true, "switch": true, "select": true,
	"return": true, "def": true, "class": true, "function": true, "func": true,
	"import": true, "package": true, "type": true, "struct": true, "interface": true,
	"chan": true, "go": true, "defer": true, "range": true, "break": true,
	"continue": true, "case": true, "default": true, "else": true, "elif": true,
	"try": true, "catch": true, "finally": true, "throw": true, "do": true,
	"async": true, "await": true, "yield": true, "in": true, "of": true,
	"typeof": true, "instanceof": true, "void": true, "raise": true,
	"except": true, "with": true, "lambda": true, "pass": true, "extends": true,
	"var": true, "const": true, "let": true, "export": true, "namespace": true,
	"implements": true, "enum": true, "static": true, "abstract": true,
}

// cgSourceExts 支持的源码文件扩展名 → 语言映射
var cgSourceExts = map[string]string{
	".go":  "go",
	".py":  "python",
	".js":  "javascript",
	".jsx": "javascript",
	".ts":  "typescript",
	".tsx": "typescript",
}

// cgLanguageWhitelist 语言参数白名单
var cgLanguageWhitelist = map[string]bool{
	"go": true, "python": true, "javascript": true, "typescript": true, "auto": true,
}

// cgOutputFormatWhitelist 输出格式白名单
var cgOutputFormatWhitelist = map[string]bool{
	"json": true, "mermaid": true,
}

// --- 数据结构 ---

// codeGraphFunction 函数定义（名称 + 起始行号）
type codeGraphFunction struct {
	Name string `json:"name"`
	Line int    `json:"line"`
}

// codeGraphCall 函数调用（名称 + 行号，用于构建 mermaid 边）
type codeGraphCall struct {
	Name string
	Line int
}

// codeGraphFile 单个文件的图谱信息
type codeGraphFile struct {
	Path      string              `json:"path"`
	Language  string              `json:"language"`
	Imports   []string            `json:"imports"`
	Functions []codeGraphFunction `json:"functions"`
	Calls     []string            `json:"calls"`
	ModTime   int64               `json:"mod_time"`
	Size      int64               `json:"size"`
	// callEdges 用于构建 mermaid 调用边，不输出到 JSON
	callEdges []codeGraphCall `json:"-"`
}

// codeGraphStats 统计信息
type codeGraphStats struct {
	FilesAnalyzed  int `json:"files_analyzed"`
	TotalFunctions int `json:"total_functions"`
	TotalImports   int `json:"total_imports"`
	TotalCalls     int `json:"total_calls"`
	CachedFiles    int `json:"cached_files"`
	UpdatedFiles   int `json:"updated_files"`
}

// codeGraphResult 图谱总结果
type codeGraphResult struct {
	Files []codeGraphFile `json:"files"`
	Stats codeGraphStats  `json:"stats"`
}

// codeGraphCache 持久化缓存
type codeGraphCache struct {
	mu       sync.RWMutex
	rootPath string
	cacheDir string
	data     map[string]codeGraphFile
	loaded   bool
}

var (
	graphCache     *codeGraphCache
	graphCacheOnce sync.Once
)

func getGraphCache() *codeGraphCache {
	graphCacheOnce.Do(func() {
		graphCache = &codeGraphCache{
			data:   make(map[string]codeGraphFile),
			loaded: false,
		}
	})
	return graphCache
}

func (c *codeGraphCache) cachePath(root string) string {
	return filepath.Join(root, ".llm-box", "codegraph-cache.json")
}

func (c *codeGraphCache) Load(root string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	cachePath := c.cachePath(root)
	data, err := os.ReadFile(cachePath)
	if err != nil {
		if os.IsNotExist(err) {
			c.rootPath = root
			c.loaded = true
			return nil
		}
		return err
	}

	var loaded struct {
		RootPath string                   `json:"root_path"`
		Files    map[string]codeGraphFile `json:"files"`
	}
	if err := json.Unmarshal(data, &loaded); err != nil {
		return err
	}

	c.rootPath = root
	c.data = loaded.Files
	if c.data == nil {
		c.data = make(map[string]codeGraphFile)
	}
	c.loaded = true
	return nil
}

func (c *codeGraphCache) Save(root string) error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	cacheDir := filepath.Join(root, ".llm-box")
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return err
	}

	cachePath := c.cachePath(root)
	toSave := struct {
		RootPath string                   `json:"root_path"`
		Version  string                   `json:"version"`
		Files    map[string]codeGraphFile `json:"files"`
		SavedAt  string                   `json:"saved_at"`
	}{
		RootPath: root,
		Version:  "1.0",
		Files:    c.data,
		SavedAt:  time.Now().UTC().Format(time.RFC3339),
	}

	data, err := json.MarshalIndent(toSave, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(cachePath, data, 0644)
}

func (c *codeGraphCache) Get(path string) (codeGraphFile, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	f, ok := c.data[path]
	return f, ok
}

func (c *codeGraphCache) Set(path string, file codeGraphFile) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.data[path] = file
}

func (c *codeGraphCache) Delete(path string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.data, path)
}

func (c *codeGraphCache) Keys() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	keys := make([]string, 0, len(c.data))
	for k := range c.data {
		keys = append(keys, k)
	}
	return keys
}

// --- 节点实现 ---

// CodeGraphNode 代码图谱节点 — 解析源代码，提取函数定义、调用关系、导入依赖
type CodeGraphNode struct{}

func init() {
	Register(&CodeGraphNode{})
}

func (n *CodeGraphNode) Name() string {
	return "code_graph"
}

func (n *CodeGraphNode) Description() string {
	return "Build a code graph from source files: extract functions, calls, and imports"
}

func (n *CodeGraphNode) Schema() NodeSchema {
	return NodeSchema{
		Name:        "code_graph",
		Description: "Parse source code files to extract function definitions, call relationships, and import dependencies, then output a code graph (JSON or Mermaid). Supports Go, Python, JavaScript, TypeScript. Features: persistent cache, incremental updates.",
		Input:       "string - not used",
		Output:      "string - code graph in JSON or Mermaid format",
		Params: []ParamSchema{
			{Name: "path", Type: "string", Description: "File or directory path to analyze (relative to working directory)", Required: true},
			{Name: "language", Type: "string", Description: "Code language: go/python/javascript/typescript/auto (default: auto)", Required: false, Default: "auto"},
			{Name: "output_format", Type: "string", Description: "Output format: json or mermaid (default: json)", Required: false, Default: "json"},
			{Name: "max_files", Type: "string", Description: "Max number of files to analyze (default: 100)", Required: false, Default: "100"},
			{Name: "max_file_size", Type: "string", Description: "Max single file size in bytes (default: 1048576 = 1MB)", Required: false, Default: "1048576"},
			{Name: "use_cache", Type: "string", Description: "Use persistent cache for incremental updates (default: true)", Required: false, Default: "true"},
			{Name: "refresh", Type: "string", Description: "Force refresh cache, ignore existing cache (default: false)", Required: false, Default: "false"},
			{Name: "save_cache", Type: "string", Description: "Save results to persistent cache (default: true)", Required: false, Default: "true"},
		},
	}
}

// Execute 执行代码图谱分析
func (n *CodeGraphNode) Execute(ctx context.Context, input string, params map[string]string) (string, error) {
	// 1. 校验 path 参数
	rawPath := params["path"]
	if rawPath == "" {
		return "", fmt.Errorf("path parameter is required")
	}
	if len(rawPath) > codeGraphMaxPathLen {
		return "", fmt.Errorf("path too long (max %d characters)", codeGraphMaxPathLen)
	}

	// 2. 校验 language 参数（白名单）
	language := getParam(params, "language", "auto")
	if !cgLanguageWhitelist[language] {
		return "", fmt.Errorf("invalid language: %s (supported: go, python, javascript, typescript, auto)", language)
	}

	// 3. 校验 output_format 参数（白名单）
	outputFormat := getParam(params, "output_format", "json")
	if !cgOutputFormatWhitelist[outputFormat] {
		return "", fmt.Errorf("invalid output_format: %s (supported: json, mermaid)", outputFormat)
	}

	// 4. 解析 max_files 参数
	maxFiles, err := strconv.Atoi(getParam(params, "max_files", "100"))
	if err != nil || maxFiles < 1 {
		maxFiles = 100
	}
	if maxFiles > codeGraphHardMaxFiles {
		maxFiles = codeGraphHardMaxFiles
	}

	// 5. 解析 max_file_size 参数
	maxFileSize, err := strconv.ParseInt(getParam(params, "max_file_size", "1048576"), 10, 64)
	if err != nil || maxFileSize < 1 {
		maxFileSize = 1048576
	}
	if maxFileSize > codeGraphHardMaxSize {
		maxFileSize = codeGraphHardMaxSize
	}

	// 6. 缓存相关参数
	useCache := strings.ToLower(getParam(params, "use_cache", "true")) == "true"
	refresh := strings.ToLower(getParam(params, "refresh", "false")) == "true"
	saveCache := strings.ToLower(getParam(params, "save_cache", "true")) == "true"

	// 7. 校验路径（防路径遍历，返回绝对路径）
	safePath, err := validateReadPath(rawPath)
	if err != nil {
		return "", fmt.Errorf("path validation failed: %w", err)
	}

	// 8. 判断是文件还是目录
	info, err := os.Stat(safePath)
	if err != nil {
		return "", fmt.Errorf("failed to stat path: %w", err)
	}

	var filePaths []string
	isDir := info.IsDir()
	if isDir {
		filePaths, err = n.collectSourceFiles(ctx, safePath, language, maxFiles)
		if err != nil {
			return "", fmt.Errorf("failed to collect source files: %w", err)
		}
	} else {
		filePaths = []string{safePath}
	}

	if len(filePaths) == 0 {
		return "", fmt.Errorf("no source files found at path: %s", rawPath)
	}

	// 9. 加载缓存（目录模式）
	cache := getGraphCache()
	cacheLoaded := false
	var cachedCount, updatedCount int

	if useCache && isDir && !refresh {
		if loadErr := cache.Load(safePath); loadErr == nil {
			cacheLoaded = true
		}
	}

	// 10. 逐个解析文件（支持增量更新）
	result := codeGraphResult{
		Files: make([]codeGraphFile, 0, len(filePaths)),
	}

	for _, fp := range filePaths {
		if err := ctx.Err(); err != nil {
			return "", fmt.Errorf("context cancelled: %w", err)
		}

		fi, statErr := os.Stat(fp)
		if statErr != nil {
			continue
		}

		// 尝试从缓存获取
		if useCache && !refresh && cacheLoaded {
			if cached, ok := cache.Get(fp); ok {
				if cached.ModTime == fi.ModTime().Unix() && cached.Size == fi.Size() {
					result.Files = append(result.Files, cached)
					cachedCount++
					continue
				}
			}
		}

		// 需要重新解析
		cf, perr := n.parseFile(fp, language, maxFileSize)
		if perr != nil {
			if !isDir {
				return "", fmt.Errorf("failed to parse file %s: %w", rawPath, perr)
			}
			logger.Warn("code_graph: failed to parse file, skipping", "path", fp, "error", perr)
			continue
		}

		cf.ModTime = fi.ModTime().Unix()
		cf.Size = fi.Size()
		result.Files = append(result.Files, cf)
		updatedCount++

		// 更新缓存
		if useCache && saveCache && isDir {
			cache.Set(fp, cf)
		}
	}

	if len(result.Files) == 0 {
		return "", fmt.Errorf("no files could be parsed at path: %s", rawPath)
	}

	// 11. 保存缓存
	if useCache && saveCache && isDir && updatedCount > 0 {
		if saveErr := cache.Save(safePath); saveErr != nil {
			logger.Warn("code_graph: failed to save cache", "error", saveErr)
		}
	}

	// 12. 计算统计信息
	result.Stats = n.computeStats(result.Files)
	result.Stats.CachedFiles = cachedCount
	result.Stats.UpdatedFiles = updatedCount

	// 13. 按 output_format 输出
	switch outputFormat {
	case "json":
		return n.outputJSON(result)
	case "mermaid":
		return n.outputMermaid(result), nil
	default:
		return "", fmt.Errorf("unsupported output format: %s", outputFormat)
	}
}

// --- 目录遍历 ---

// collectSourceFiles 遍历目录收集源码文件（限制深度、跳过符号链接、按扩展名/语言过滤）
func (n *CodeGraphNode) collectSourceFiles(ctx context.Context, root, language string, maxFiles int) ([]string, error) {
	var files []string
	if err := n.walkDir(ctx, root, 0, language, maxFiles, &files); err != nil {
		return nil, err
	}
	return files, nil
}

// walkDir 递归遍历目录，收集源码文件路径
func (n *CodeGraphNode) walkDir(ctx context.Context, dir string, depth int, language string, maxFiles int, files *[]string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if depth > codeGraphMaxDepth {
		return nil
	}
	if len(*files) >= maxFiles {
		return nil
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("failed to read directory %s: %w", dir, err)
	}
	for _, entry := range entries {
		if len(*files) >= maxFiles {
			return nil
		}
		full := filepath.Join(dir, entry.Name())
		// 用 Lstat 检查，不读取符号链接
		fi, err := os.Lstat(full)
		if err != nil {
			continue
		}
		// 跳过符号链接
		if fi.Mode()&os.ModeSymlink != 0 {
			continue
		}
		if entry.IsDir() {
			// 跳过隐藏目录和常见依赖/生成目录
			name := entry.Name()
			if strings.HasPrefix(name, ".") || name == "vendor" || name == "node_modules" || name == "testdata" || name == "dist" || name == "build" {
				continue
			}
			if err := n.walkDir(ctx, full, depth+1, language, maxFiles, files); err != nil {
				return err
			}
		} else if fi.Mode().IsRegular() {
			if n.shouldIncludeFile(entry.Name(), language) {
				*files = append(*files, full)
			}
		}
	}
	return nil
}

// shouldIncludeFile 根据扩展名和语言过滤判断是否应包含该文件
func (n *CodeGraphNode) shouldIncludeFile(filename, language string) bool {
	ext := strings.ToLower(filepath.Ext(filename))
	lang, ok := cgSourceExts[ext]
	if !ok {
		return false
	}
	if language == "auto" {
		return true
	}
	return lang == language
}

// --- 文件解析 ---

// parseFile 解析单个源码文件，返回文件图谱信息
func (n *CodeGraphNode) parseFile(absPath string, language string, maxFileSize int64) (codeGraphFile, error) {
	// 检查文件大小
	fi, err := os.Stat(absPath)
	if err != nil {
		return codeGraphFile{}, fmt.Errorf("failed to stat file: %w", err)
	}
	if fi.Size() > maxFileSize {
		return codeGraphFile{}, fmt.Errorf("file too large (size: %d, max: %d)", fi.Size(), maxFileSize)
	}

	data, err := os.ReadFile(absPath)
	if err != nil {
		return codeGraphFile{}, fmt.Errorf("failed to read file: %w", err)
	}

	// 检测或使用指定语言
	detectedLang := language
	if language == "auto" {
		detectedLang = n.detectLanguage(absPath)
		if detectedLang == "" {
			return codeGraphFile{}, fmt.Errorf("could not detect language for: %s", absPath)
		}
	}

	// 计算相对工作目录的路径用于输出展示
	relPath := absPath
	if workDir != "" {
		if rel, err := filepath.Rel(workDir, absPath); err == nil {
			relPath = rel
		}
	}

	cf := codeGraphFile{
		Path:      cgSanitizeString(relPath),
		Language:  detectedLang,
		Imports:   []string{},
		Functions: []codeGraphFunction{},
		Calls:     []string{},
	}

	content := string(data)
	switch detectedLang {
	case "go":
		n.parseGo(content, &cf)
	case "python":
		n.parsePython(content, &cf)
	case "javascript", "typescript":
		n.parseJavaScript(content, &cf)
	}

	return cf, nil
}

// detectLanguage 根据文件扩展名检测语言
func (n *CodeGraphNode) detectLanguage(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	return cgSourceExts[ext]
}

// parseGo 解析 Go 源码：提取 import、func 定义、函数调用
func (n *CodeGraphNode) parseGo(content string, cf *codeGraphFile) {
	lines := strings.Split(content, "\n")
	inImportBlock := false
	callSeen := make(map[string]bool)
	var callOrder []string

	for i, line := range lines {
		lineNum := i + 1
		trimmed := strings.TrimSpace(line)

		// 跳过注释行
		if cgIsCommentLine(trimmed, false) {
			continue
		}

		// --- imports ---
		if inImportBlock {
			if trimmed == ")" {
				inImportBlock = false
				continue
			}
			if m := cgGoImportBlockLineRe.FindStringSubmatch(line); len(m) > 1 {
				cf.Imports = append(cf.Imports, cgTruncateName(m[1]))
			}
			continue
		}
		if strings.HasPrefix(trimmed, "import") {
			if m := cgGoImportSingleRe.FindStringSubmatch(line); len(m) > 1 {
				cf.Imports = append(cf.Imports, cgTruncateName(m[1]))
			} else if strings.Contains(trimmed, "(") {
				inImportBlock = true
			}
			continue
		}

		// --- functions ---
		if m := cgGoFuncRe.FindStringSubmatch(line); len(m) > 1 {
			cf.Functions = append(cf.Functions, codeGraphFunction{
				Name: cgTruncateName(cgSanitizeName(m[1])),
				Line: lineNum,
			})
			// func 定义行不再提取调用
			continue
		}

		// --- calls ---
		n.extractCalls(line, cgGoCallRe, callSeen, &callOrder, lineNum, cf)
	}

	cf.Calls = callOrder
}

// parsePython 解析 Python 源码：提取 import、def/class 定义、函数调用
func (n *CodeGraphNode) parsePython(content string, cf *codeGraphFile) {
	lines := strings.Split(content, "\n")
	callSeen := make(map[string]bool)
	var callOrder []string

	for i, line := range lines {
		lineNum := i + 1
		trimmed := strings.TrimSpace(line)

		// 跳过注释行
		if cgIsCommentLine(trimmed, true) {
			continue
		}

		// --- imports ---
		if m := cgPyImportRe.FindStringSubmatch(line); len(m) > 1 {
			cf.Imports = append(cf.Imports, cgTruncateName(m[1]))
			continue
		}
		if m := cgPyFromImportRe.FindStringSubmatch(line); len(m) > 1 {
			cf.Imports = append(cf.Imports, cgTruncateName(m[1]))
			continue
		}

		// --- functions ---
		if m := cgPyFuncRe.FindStringSubmatch(line); len(m) > 1 {
			cf.Functions = append(cf.Functions, codeGraphFunction{
				Name: cgTruncateName(cgSanitizeName(m[1])),
				Line: lineNum,
			})
			continue
		}

		// --- classes (作为函数节点) ---
		if m := cgPyClassRe.FindStringSubmatch(line); len(m) > 1 {
			cf.Functions = append(cf.Functions, codeGraphFunction{
				Name: cgTruncateName(cgSanitizeName(m[1])),
				Line: lineNum,
			})
			continue
		}

		// --- calls ---
		n.extractCalls(line, cgPyCallRe, callSeen, &callOrder, lineNum, cf)
	}

	cf.Calls = callOrder
}

// parseJavaScript 解析 JavaScript/TypeScript 源码：提取 import、function/arrow 定义、函数调用
func (n *CodeGraphNode) parseJavaScript(content string, cf *codeGraphFile) {
	lines := strings.Split(content, "\n")
	callSeen := make(map[string]bool)
	var callOrder []string

	for i, line := range lines {
		lineNum := i + 1
		trimmed := strings.TrimSpace(line)

		// 跳过注释行
		if cgIsCommentLine(trimmed, false) {
			continue
		}

		// --- imports ---
		if m := cgJsImportFromRe.FindStringSubmatch(line); len(m) > 1 {
			cf.Imports = append(cf.Imports, cgTruncateName(m[1]))
			continue
		}
		if m := cgJsImportBareRe.FindStringSubmatch(line); len(m) > 1 {
			cf.Imports = append(cf.Imports, cgTruncateName(m[1]))
			continue
		}

		// --- functions (function 声明) ---
		if m := cgJsFuncRe.FindStringSubmatch(line); len(m) > 1 {
			cf.Functions = append(cf.Functions, codeGraphFunction{
				Name: cgTruncateName(cgSanitizeName(m[1])),
				Line: lineNum,
			})
			continue
		}
		// --- functions (箭头函数 const/let/var = ... =>) ---
		if m := cgJsArrowFuncRe.FindStringSubmatch(line); len(m) > 1 {
			cf.Functions = append(cf.Functions, codeGraphFunction{
				Name: cgTruncateName(cgSanitizeName(m[1])),
				Line: lineNum,
			})
			continue
		}

		// --- calls ---
		n.extractCalls(line, cgJsCallRe, callSeen, &callOrder, lineNum, cf)
	}

	cf.Calls = callOrder
}

// extractCalls 从一行中提取函数调用（限制匹配次数，过滤控制流关键字，去重保留顺序）
func (n *CodeGraphNode) extractCalls(line string, re *regexp.Regexp, seen map[string]bool, order *[]string, lineNum int, cf *codeGraphFile) {
	matches := re.FindAllStringSubmatch(line, codeGraphMaxRegexMatch)
	for _, m := range matches {
		if len(m) < 2 {
			continue
		}
		name := m[1]
		// 过滤控制流关键字
		if cgControlKeywords[name] {
			continue
		}
		name = cgTruncateName(cgSanitizeName(name))
		if name == "" {
			continue
		}
		// 去重（保留首次出现顺序），用于 JSON 输出
		if !seen[name] {
			seen[name] = true
			*order = append(*order, name)
		}
		// 记录每次调用（含行号），用于 mermaid 边构建
		cf.callEdges = append(cf.callEdges, codeGraphCall{Name: name, Line: lineNum})
	}
}

// --- 统计与输出 ---

// computeStats 计算统计信息
func (n *CodeGraphNode) computeStats(files []codeGraphFile) codeGraphStats {
	stats := codeGraphStats{FilesAnalyzed: len(files)}
	for _, f := range files {
		stats.TotalFunctions += len(f.Functions)
		stats.TotalImports += len(f.Imports)
		stats.TotalCalls += len(f.Calls)
	}
	return stats
}

// outputJSON 输出 JSON 格式
func (n *CodeGraphNode) outputJSON(result codeGraphResult) (string, error) {
	// 确保空数组序列化为 [] 而非 null
	for i := range result.Files {
		if result.Files[i].Imports == nil {
			result.Files[i].Imports = []string{}
		}
		if result.Files[i].Functions == nil {
			result.Files[i].Functions = []codeGraphFunction{}
		}
		if result.Files[i].Calls == nil {
			result.Files[i].Calls = []string{}
		}
	}
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal JSON: %w", err)
	}
	return string(data), nil
}

// outputMermaid 输出 Mermaid 流程图格式
func (n *CodeGraphNode) outputMermaid(result codeGraphResult) string {
	var b strings.Builder
	b.WriteString("```mermaid\ngraph TD\n")

	// 节点 ID 去重映射
	usedIDs := make(map[string]bool)
	makeID := func(prefix, name string) string {
		base := cgMermaidID(prefix, name)
		id := base
		counter := 1
		for usedIDs[id] {
			id = fmt.Sprintf("%s_%d", base, counter)
			counter++
		}
		usedIDs[id] = true
		return id
	}

	// 每个文件的节点信息：fileID 和 函数名->mermaid节点ID 映射
	type fileNodeInfo struct {
		fileID  string
		funcIDs map[string]string
	}
	fileNodes := make([]fileNodeInfo, len(result.Files))

	// 输出节点定义：文件 + 函数
	for i, f := range result.Files {
		fileID := makeID("file", f.Path)
		fileNodes[i].fileID = fileID
		fileNodes[i].funcIDs = make(map[string]string)
		b.WriteString(fmt.Sprintf("    %s[\"%s\"]\n", fileID, cgMermaidLabel(f.Path)))
		for _, fn := range f.Functions {
			funcID := makeID("func", fn.Name)
			fileNodes[i].funcIDs[fn.Name] = funcID
			b.WriteString(fmt.Sprintf("    %s[\"%s\"]\n", funcID, cgMermaidLabel(fn.Name)))
		}
	}

	// 输出边：定义关系（文件 → 函数）
	for i, f := range result.Files {
		for _, fn := range f.Functions {
			funcID := fileNodes[i].funcIDs[fn.Name]
			b.WriteString(fmt.Sprintf("    %s --> %s\n", fileNodes[i].fileID, funcID))
		}
	}

	// 输出边：调用关系（函数 → 函数，仅同文件内定义的函数）
	for i, f := range result.Files {
		// 构建该文件中函数名集合
		funcNames := make(map[string]bool)
		for _, fn := range f.Functions {
			funcNames[fn.Name] = true
		}
		// 构建函数行号区间，用于判定调用归属
		type funcRange struct {
			name       string
			start, end int
		}
		ranges := make([]funcRange, len(f.Functions))
		for j, fn := range f.Functions {
			end := -1 // -1 表示文件末尾
			if j+1 < len(f.Functions) {
				end = f.Functions[j+1].Line
			}
			ranges[j] = funcRange{name: fn.Name, start: fn.Line, end: end}
		}
		// 为每个调用找到所属函数并建边（去重）
		seenEdges := make(map[string]bool)
		for _, call := range f.callEdges {
			// 只连接同文件中定义的函数
			if !funcNames[call.Name] {
				continue
			}
			// 找到包含该调用行号的函数区间
			caller := ""
			for _, r := range ranges {
				if call.Line >= r.start && (r.end == -1 || call.Line < r.end) {
					caller = r.name
					break
				}
			}
			// 无归属或自调用，跳过
			if caller == "" || caller == call.Name {
				continue
			}
			edgeKey := caller + "|" + call.Name
			if seenEdges[edgeKey] {
				continue
			}
			seenEdges[edgeKey] = true
			fromID := fileNodes[i].funcIDs[caller]
			toID := fileNodes[i].funcIDs[call.Name]
			if fromID != "" && toID != "" {
				b.WriteString(fmt.Sprintf("    %s --> %s\n", fromID, toID))
			}
		}
	}

	b.WriteString("```\n")
	return b.String()
}

// --- 辅助函数 ---

// cgIsCommentLine 判断是否为注释行
// isPython: Python 用 # 注释；Go/JS/TS 用 //、/*、*（块注释续行）
func cgIsCommentLine(trimmed string, isPython bool) bool {
	if trimmed == "" {
		return true
	}
	if isPython {
		return strings.HasPrefix(trimmed, "#")
	}
	return strings.HasPrefix(trimmed, "//") ||
		strings.HasPrefix(trimmed, "/*") ||
		strings.HasPrefix(trimmed, "*")
}

// cgSanitizeString 清洗字符串中的控制字符（用于路径输出）
func cgSanitizeString(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r < 0x20 || r == 0x7f {
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

// cgSanitizeName 清洗标识符（仅保留字母数字下划线点美元符号）
func cgSanitizeName(s string) string {
	var b strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') ||
			(r >= '0' && r <= '9') || r == '_' || r == '.' || r == '$' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// cgTruncateName 截断过长的名称（超 256 字符截断）
func cgTruncateName(name string) string {
	if len(name) > codeGraphMaxNameLen {
		return name[:codeGraphMaxNameLen]
	}
	return name
}

// cgMermaidID 生成 Mermaid 节点 ID（仅字母数字下划线，其余替换为下划线）
func cgMermaidID(prefix, name string) string {
	var b strings.Builder
	b.WriteString(prefix)
	b.WriteByte('_')
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') ||
			(r >= '0' && r <= '9') || r == '_' {
			b.WriteRune(r)
		} else {
			b.WriteByte('_')
		}
	}
	return b.String()
}

// cgMermaidLabel 生成 Mermaid 节点标签（转义特殊字符）
func cgMermaidLabel(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "\"", "\\\"")
	return s
}
