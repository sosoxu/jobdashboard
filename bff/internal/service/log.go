package service

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/dashboard/bff/internal/logpath"
)

// 默认分页参数与扫描上限
const (
	defaultPageSize = 200
	maxPageSize     = 2000
	// maxScanLines 限制单次扫描的最大行数，防止 GB 级文件 OOM。
	// 1 百万行通常已覆盖实际过滤需求；超过则置 truncated=true。
	maxScanLines = 1_000_000
	// pathCacheTTL 是日志文件路径解析结果（glob 找到的最新文件）的缓存有效期。
	// 在百万级文件目录下，频繁 glob 会带来可观开销；分页浏览期间使用缓存避免重复 glob。
	pathCacheTTL = 60 * time.Second
)

// LogService reads job log files from the filesystem with pagination support.
// 日志文件为原始文本内容，不做级别分类。list 文件支持段落快速定位。
//
// 文件定位规则（jobName 全局唯一）：
//   - 目录：{surveyDir}/list 或 {surveyDir}/LOG
//   - 文件名：{jobDesc}.{四位数编号}.{jobName}.list 或 .log
//   - 多个匹配时取 mtime 最新；mtime 相同时取编号最大者
//   - 使用 filepath.Glob 在内核侧做名字过滤，避免 ReadDir 全量遍历
type LogService struct {
	resolver *logpath.Resolver
	maxLines int
	cache    *logCache
	pathCache *pathCache
	maxScan  int
}

type logCache struct {
	mu    sync.RWMutex
	items map[string]logCacheEntry
}

type logCacheEntry struct {
	signature string
	lines     int
	sections  []LogSection // 仅 list 类型有值
}

// pathCache 缓存"日志目录 + jobName + ext"到"最新文件路径"的解析结果。
// 缓存项带 TTL（pathCacheTTL）；TTL 内复用结果，避免对百万级文件目录反复 glob。
// 注意：缓存命中后，调用方仍会通过文件 mtime/size 签名判断行数缓存是否有效，
// 一旦文件被替换或追加，签名变化会自然失效下层行数缓存。
type pathCache struct {
	mu    sync.RWMutex
	items map[string]pathCacheEntry
}

type pathCacheEntry struct {
	file      string
	checkedAt time.Time
}

func newPathCache() *pathCache {
	return &pathCache{items: make(map[string]pathCacheEntry)}
}

func (p *pathCache) get(key string) (string, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if e, ok := p.items[key]; ok && time.Since(e.checkedAt) < pathCacheTTL {
		return e.file, true
	}
	return "", false
}

func (p *pathCache) set(key, file string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.items[key] = pathCacheEntry{file: file, checkedAt: time.Now()}
}

func NewLogService(resolver *logpath.Resolver, maxLines int) *LogService {
	if maxLines <= 0 {
		maxLines = 5000
	}
	return &LogService{
		resolver:  resolver,
		maxLines:  maxLines,
		maxScan:   maxScanLines,
		cache:     &logCache{items: make(map[string]logCacheEntry)},
		pathCache: newPathCache(),
	}
}

// LogLine 是日志的原始文本行。
type LogLine struct {
	LineNo int    `json:"lineNo"` // 全文行号（从 1 开始，跨文件连续编号）
	Text   string `json:"text"`   // 原始行内容
}

// LogSection 是 list 文件中识别到的段落标题，用于快速跳转。
type LogSection struct {
	Name   string `json:"name"`
	LineNo int    `json:"lineNo"` // 段落起始行号（1-based）
}

// LogResult 是分页日志响应。
type LogResult struct {
	JobName   string       `json:"jobName"`
	Type      string       `json:"type"` // list | log
	Path      string       `json:"path"`
	Page      int          `json:"page"`
	PageSize  int          `json:"pageSize"`
	Total     int          `json:"total"`     // 总行数；-1 表示未知
	Lines     []LogLine    `json:"lines"`
	Truncated bool         `json:"truncated"` // 是否因超过 maxScanLines 而截断
	Files     []string     `json:"files"`
	Filtered  bool         `json:"filtered"`  // 是否启用了 keyword 过滤
	Cached    bool         `json:"cached"`    // Total/Sections 是否来自缓存
	Sections  []LogSection `json:"sections"`  // list 文件的段落列表；log 类型为空
}

// Read 读取作业日志（分页）。
//   - 通过 jobName 在 {surveyDir}/list 或 LOG 目录下 glob 定位最新日志文件
//   - 无 keyword：扫描到 offset+pageSize 行即停，Total/Sections 优先取缓存
//   - 有 keyword：扫描整个文件收集匹配行（限 maxScanLines 行），分页返回
//   - logType=list 时，附带返回段落列表（缓存命中或扫描全文后填充）
func (s *LogService) Read(ctx context.Context, jobName, project, survey, logType, keyword string, page, pageSize int) (*LogResult, error) {
	logDir, err := s.resolver.LogDir(project, survey, logType)
	if err != nil {
		return nil, err
	}
	keyword = strings.TrimSpace(keyword)
	if page < 1 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = defaultPageSize
	}
	if pageSize > maxPageSize {
		pageSize = maxPageSize
	}

	ext := logExt(logType)
	files, err := s.locateLogFiles(logDir, jobName, ext)
	if err != nil {
		return nil, err
	}

	isList := strings.EqualFold(logType, "list")
	path := ""
	if len(files) > 0 {
		path = files[0]
	}
	out := &LogResult{
		JobName:  jobName,
		Type:     logType,
		Path:     path,
		Page:     page,
		PageSize: pageSize,
		Lines:    []LogLine{},
		Files:    files,
		Filtered: keyword != "",
		Total:    -1,
		Sections: nil,
	}

	if len(files) == 0 {
		// 没有匹配的日志文件：返回空结果（total=0），避免 500 错误。
		out.Total = 0
		return out, nil
	}

	if keyword != "" {
		return s.readFiltered(ctx, out, files, isList, keyword, page, pageSize)
	}
	return s.readRaw(ctx, out, files, isList, page, pageSize)
}

// locateLogFiles 通过 jobName 在 logDir 下 glob 定位日志文件，取最新一个返回。
// 路径解析结果带 TTL 缓存，避免分页浏览期间反复 glob。
func (s *LogService) locateLogFiles(logDir, jobName, ext string) ([]string, error) {
	cacheKey := logDir + "|" + jobName + "|" + ext
	if file, ok := s.pathCache.get(cacheKey); ok && file != "" {
		// 缓存命中：复用上次的解析结果。若文件已被删除则 fallthrough 重新 glob。
		if _, err := os.Stat(file); err == nil {
			return []string{file}, nil
		}
	}

	file, err := findLatestLog(logDir, jobName, ext)
	if err != nil {
		return nil, err
	}
	if file == "" {
		// 没有匹配文件：不写缓存，让下次请求重新尝试。
		return nil, nil
	}
	s.pathCache.set(cacheKey, file)
	return []string{file}, nil
}

// logExt 返回日志文件后缀（不含点）。
func logExt(logType string) string {
	if strings.EqualFold(logType, "log") {
		return "log"
	}
	return "list"
}

// findLatestLog 在 logDir 中查找形如 *.{jobName}.{ext} 的文件，返回最新一个。
// 排序规则：mtime 降序（主），编号降序（次）。
// 返回空字符串表示无匹配。
func findLatestLog(logDir, jobName, ext string) (string, error) {
	pattern := filepath.Join(logDir, "*."+jobName+"."+ext)
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return "", fmt.Errorf("glob 日志文件失败 %s: %w", pattern, err)
	}
	if len(matches) == 0 {
		return "", nil
	}
	type candidate struct {
		path  string
		mtime time.Time
		seq   int // 四位数编号；解析失败按 -1 处理（落到最后）
	}
	cands := make([]candidate, 0, len(matches))
	for _, m := range matches {
		info, err := os.Stat(m)
		if err != nil || info.IsDir() {
			continue
		}
		cands = append(cands, candidate{
			path:  m,
			mtime: info.ModTime(),
			seq:   parseSeqNumber(filepath.Base(m), jobName, ext),
		})
	}
	if len(cands) == 0 {
		return "", nil
	}
	sort.Slice(cands, func(i, j int) bool {
		// mtime 降序
		if !cands[i].mtime.Equal(cands[j].mtime) {
			return cands[i].mtime.After(cands[j].mtime)
		}
		// 编号降序
		return cands[i].seq > cands[j].seq
	})
	return cands[0].path, nil
}

// parseSeqNumber 从文件名 `{jobDesc}.{编号}.{jobName}.{ext}` 中提取四位数编号。
// 解析失败返回 -1（参与排序时落在最后）。
func parseSeqNumber(base, jobName, ext string) int {
	// 去掉后缀 .{ext}
	suffix := "." + jobName + "." + ext
	if !strings.HasSuffix(base, suffix) {
		return -1
	}
	core := strings.TrimSuffix(base, suffix)
	// 取最后一个 "." 后的段，应为四位数编号
	idx := strings.LastIndexByte(core, '.')
	if idx < 0 {
		return -1
	}
	seqStr := core[idx+1:]
	n, err := strconv.Atoi(seqStr)
	if err != nil || n < 0 {
		return -1
	}
	return n
}

// readRaw 无过滤分页：
//   - 缓存命中（基于文件 mtime/size 签名）：只扫描到 offset+pageSize 即停，性能好。
//   - 缓存未命中：扫描整个文件，统计 total、检测段落（list），并填充缓存。
func (s *LogService) readRaw(ctx context.Context, out *LogResult, files []string, isList bool, page, pageSize int) (*LogResult, error) {
	if total, sections, hit := s.getCache(files); hit {
		out.Total = total
		out.Cached = true
		if isList {
			out.Sections = sections
		}
	}

	offset := (page - 1) * pageSize
	lineNo := 0
	collected := 0
	scannedToEnd := true
	var detectedSections []LogSection

	for _, f := range files {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		fh, err := os.Open(f)
		if err != nil {
			continue
		}
		scanner := newBigScanner(fh)
		// 用于识别 "----/ModuleName/----" 三行模块段落
		var prevPrevText, prevText string
		var prevPrevNo, prevNo int
		for scanner.Scan() {
			lineNo++
			if lineNo > s.maxScan {
				out.Truncated = true
				scannedToEnd = false
				break
			}
			line := scanner.Text()
			// list 类型：检测段落标题（仅缓存未命中时需要）
			if isList && !out.Cached {
				if name, ok := detectSection(line); ok {
					detectedSections = append(detectedSections, LogSection{Name: name, LineNo: lineNo})
				} else if name, hitNo, ok := detectModuleSection(prevPrevText, prevText, line, prevPrevNo, prevNo); ok {
					detectedSections = append(detectedSections, LogSection{Name: "Module: " + name, LineNo: hitNo})
				}
			}
			prevPrevText = prevText
			prevPrevNo = prevNo
			prevText = line
			prevNo = lineNo
			// 已收集够本页且总行数已知 → 停止扫描
			if collected >= pageSize && out.Total >= 0 {
				scannedToEnd = false
				break
			}
			// 当前页范围内 → 收集
			if lineNo > offset && collected < pageSize {
				out.Lines = append(out.Lines, LogLine{
					LineNo: lineNo,
					Text:   line,
				})
				collected++
			}
		}
		fh.Close()
		if out.Truncated || !scannedToEnd {
			break
		}
	}

	// 扫描到末尾 → 得到准确 total/sections，写入缓存
	if scannedToEnd {
		out.Total = lineNo
		if isList && !out.Cached {
			out.Sections = detectedSections
			s.setCache(files, lineNo, detectedSections)
		} else if !out.Cached {
			s.setCache(files, lineNo, nil)
		}
		out.Cached = false
	}
	return out, nil
}

// readFiltered 有过滤分页：扫描整个文件收集匹配行，分页返回。
// 注意：过滤模式下段落信息仍会返回（来自缓存或扫描期间检测），便于用户跳转。
func (s *LogService) readFiltered(ctx context.Context, out *LogResult, files []string, isList bool, keyword string, page, pageSize int) (*LogResult, error) {
	// 先取段落缓存（若有）
	if isList {
		if _, sections, hit := s.getCache(files); hit {
			out.Sections = sections
			out.Cached = true
		}
	}

	kwLow := strings.ToLower(keyword)
	matched := make([]LogLine, 0, page*pageSize)
	lineNo := 0
	var detectedSections []LogSection

	for _, f := range files {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		fh, err := os.Open(f)
		if err != nil {
			continue
		}
		scanner := newBigScanner(fh)
		for scanner.Scan() {
			lineNo++
			if lineNo > s.maxScan {
				out.Truncated = true
				fh.Close()
				goto done
			}
			line := scanner.Text()
			if isList && (out.Sections == nil) {
				if name, ok := detectSection(line); ok {
					detectedSections = append(detectedSections, LogSection{Name: name, LineNo: lineNo})
				}
			}
			if !strings.Contains(strings.ToLower(line), kwLow) {
				continue
			}
			matched = append(matched, LogLine{
				LineNo: lineNo,
				Text:   line,
			})
		}
		fh.Close()
	}
done:
	out.Total = len(matched)
	if isList && out.Sections == nil {
		out.Sections = detectedSections
	}
	offset := (page - 1) * pageSize
	if offset >= len(matched) {
		out.Lines = []LogLine{}
		return out, nil
	}
	end := offset + pageSize
	if end > len(matched) {
		end = len(matched)
	}
	out.Lines = matched[offset:end]
	return out, nil
}

// getCache 读取缓存的总行数与段落列表。hit=true 表示缓存命中。
func (s *LogService) getCache(files []string) (int, []LogSection, bool) {
	sig := filesSignature(files)
	key := filesKey(files)
	s.cache.mu.RLock()
	defer s.cache.mu.RUnlock()
	if e, ok := s.cache.items[key]; ok && e.signature == sig {
		return e.lines, e.sections, true
	}
	return -1, nil, false
}

func (s *LogService) setCache(files []string, lines int, sections []LogSection) {
	sig := filesSignature(files)
	key := filesKey(files)
	s.cache.mu.Lock()
	s.cache.items[key] = logCacheEntry{signature: sig, lines: lines, sections: sections}
	s.cache.mu.Unlock()
}

func filesKey(files []string) string {
	return strings.Join(files, "|")
}

func filesSignature(files []string) string {
	var b strings.Builder
	for _, f := range files {
		info, err := os.Stat(f)
		if err != nil {
			continue
		}
		fmt.Fprintf(&b, "%s|%d|%d|", f, info.ModTime().UnixNano(), info.Size())
	}
	return b.String()
}

func newBigScanner(fh *os.File) *bufio.Scanner {
	scanner := bufio.NewScanner(fh)
	// Allow long lines (up to 1 MiB).
	buf := make([]byte, 0, 1024*1024)
	scanner.Buffer(buf, 1024*1024)
	return scanner
}

// --- list 文件段落检测 ---
//
// 段落标题识别规则（基于示例 zlm-test-web1.job.5567.*.list）：
//  1. 形如 "===  Start of Job Code  ===" 的等号包裹行
//  2. 形如 "***** Enter Module  AM *****" 的星号包裹行
//  3. 形如 "-------------- Job Structure --------------" 的减号包裹行
//  4. 形如 "==== Job resource report ====" 的混合等号行
//  5. 形如 "**********  List Information for All Threads  **********" 的混合星号行
//  6. 关键词行：Module Run Time Information / Job Information Table /
//     Job Start Date / Job End Date / Job Done Successful / NGP V
//  7. 模块段落：连续三行 - "------" / "ModuleName" / "------"
var (
	reEquals = regexp.MustCompile(`^={2,}\s*(.+?)\s*={2,}$`)
	reStars  = regexp.MustCompile(`^\*{2,}\s*(.+?)\s*\*{2,}$`)
	reDashes = regexp.MustCompile(`^-{2,}\s*(.+?)\s*-{2,}$`)
	reDashOnly = regexp.MustCompile(`^-{2,}$`)
)

var sectionKeywords = []string{
	"Module Run Time Information",
	"Job Information Table",
	"Job Start Date",
	"Job End Date",
	"Job Done Successful",
	"Job Done",
	"NGP V",
}

// detectSection 判断一行是否为段落标题，返回标题名与是否命中。
// 排除纯装饰行（如 "------...------" 全是同一字符），只保留真正含内容的标题。
func detectSection(line string) (string, bool) {
	s := strings.TrimSpace(line)
	if s == "" {
		return "", false
	}
	if m := reEquals.FindStringSubmatch(s); len(m) > 1 {
		name := strings.TrimSpace(m[1])
		if isMeaningfulName(name, "=") {
			return name, true
		}
	}
	if m := reStars.FindStringSubmatch(s); len(m) > 1 {
		name := strings.TrimSpace(m[1])
		if isMeaningfulName(name, "*") {
			return name, true
		}
	}
	if m := reDashes.FindStringSubmatch(s); len(m) > 1 {
		name := strings.TrimSpace(m[1])
		if isMeaningfulName(name, "-") {
			return name, true
		}
	}
	for _, k := range sectionKeywords {
		if strings.Contains(s, k) {
			return s, true
		}
	}
	return "", false
}

// isMeaningfulName 判断捕获到的标题是否包含至少一个非装饰字符。
// 避免把 "-------------------------------------------" 这种纯装饰行误识别为段落。
func isMeaningfulName(name, decoChar string) bool {
	if name == "" {
		return false
	}
	// 去掉所有装饰字符后还有内容才算
	trimmed := strings.Trim(name, decoChar)
	trimmed = strings.TrimSpace(trimmed)
	return trimmed != ""
}

// detectModuleSection 用于扫描时跟踪上下文，识别 "----/ModuleName/----" 三行段落。
// 调用方维护最近两行（prevPrev, prev），传入当前行 cur：
//   - 若 prevPrev 与 cur 都是纯减号行，且 prev 是单个非空单词 → 命中模块段落
//   - 返回 name, hitLineNo（命中行号 = prev 行号）
// 注：此函数为有状态扫描的辅助函数，本实现未启用；保留供未来增强使用。
func detectModuleSection(prevPrev, prev, cur string, prevPrevNo, prevNo int) (string, int, bool) {
	if prevPrev == "" || prev == "" || cur == "" {
		return "", 0, false
	}
	if reDashOnly.MatchString(strings.TrimSpace(prevPrev)) &&
		reDashOnly.MatchString(strings.TrimSpace(cur)) {
		name := strings.TrimSpace(prev)
		// 模块名通常为单词（不含空格、冒号、等号）
		if name != "" && !strings.ContainsAny(name, " :=*-/") {
			return name, prevNo, true
		}
	}
	return "", 0, false
}
