package logpath

import (
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"log/slog"
)

// Resolver resolves a project+survey to its log directory on the filesystem.
//
// Resolution rules (confirmed with stakeholder + docs/projects.conf):
//   - Read env $NGP (configurable env name), then read $NGP/configs/ndp/projects.conf.
//   - Parse the XML config: <Projects><project><name>..</name><dbname>..</dbname>
//     <maindir>..</maindir>...</project></Projects>. Find the project whose
//     <name> equals the requested project, read its <maindir>.
//   - Survey directory = {maindir}/data/{project}/{survey}.
//   - list log file = {surveyDir}/list ; LOG log dir = {surveyDir}/LOG.
type Resolver struct {
	envName     string
	projectsRel string
	logger      *slog.Logger
	cache       *cacheEntry
	mu          sync.Mutex
}

type cacheEntry struct {
	path    string
	mtime   time.Time
	maindir map[string]string // project name -> maindir
}

func New(envName, projectsRel string, logger *slog.Logger) *Resolver {
	if envName == "" {
		envName = "NGP"
	}
	if projectsRel == "" {
		projectsRel = "configs/ndp/projects.conf"
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &Resolver{envName: envName, projectsRel: projectsRel, logger: logger}
}

// Maindir returns the project main directory for the given project.
func (r *Resolver) Maindir(project string) (string, error) {
	if project == "" {
		return "", fmt.Errorf("project is empty")
	}
	m, err := r.load()
	if err != nil {
		return "", err
	}
	dir, ok := m[project]
	if !ok {
		return "", fmt.Errorf("project %q not found in projects.conf", project)
	}
	return dir, nil
}

// SurveyDir returns {maindir}/data/{project}/{survey}.
func (r *Resolver) SurveyDir(project, survey string) (string, error) {
	maindir, err := r.Maindir(project)
	if err != nil {
		return "", err
	}
	if survey == "" {
		return "", fmt.Errorf("survey is empty")
	}
	dir := filepath.Join(maindir, "data", project, survey)
	// Path-traversal safety: cleaned path must stay within maindir.
	cleanMain := filepath.Clean(maindir)
	cleanDir := filepath.Clean(dir)
	if !isWithin(cleanDir, cleanMain) {
		return "", fmt.Errorf("resolved path escapes project maindir: %s", cleanDir)
	}
	return cleanDir, nil
}

// LogPath returns the file path for a log type ("list" or "log").
// 保留兼容旧调用方：返回 {surveyDir}/list 或 {surveyDir}/LOG 路径（不区分文件/目录）。
func (r *Resolver) LogPath(project, survey, logType string) (string, error) {
	dir, err := r.SurveyDir(project, survey)
	if err != nil {
		return "", err
	}
	switch strings.ToLower(logType) {
	case "list":
		return filepath.Join(dir, "list"), nil
	case "log":
		return filepath.Join(dir, "LOG"), nil
	default:
		return "", fmt.Errorf("unknown log type %q (want list|log)", logType)
	}
}

// LogDir 返回指定日志类型对应的"目录"路径。
// 新方案下，list 与 LOG 均为子目录，目录中存放形如
// `{jobDesc}.{四位数编号}.{jobName}.list|.log` 的多个日志文件。
//   - line 为空：list → {surveyDir}/list，log → {surveyDir}/LOG
//   - line 非空：list → {surveyDir}/{line}/list，log → {surveyDir}/{line}/LOG
//     （作业含测线数据时，日志落到测线子目录下）
// line 必须是单段目录名（不允许包含路径分隔符或为 "." / ".."），
// 从源头避免路径穿越。
func (r *Resolver) LogDir(project, survey, line, logType string) (string, error) {
	dir, err := r.SurveyDir(project, survey)
	if err != nil {
		return "", err
	}
	// line 非空时插入测线段；line 必须是单段目录名，禁止穿越。
	if line != "" {
		if !isSafeSingleSegment(line) {
			return "", fmt.Errorf("invalid line %q: must be a single path segment", line)
		}
		dir = filepath.Join(dir, line)
	}
	switch strings.ToLower(logType) {
	case "list":
		return filepath.Join(dir, "list"), nil
	case "log":
		return filepath.Join(dir, "LOG"), nil
	default:
		return "", fmt.Errorf("unknown log type %q (want list|log)", logType)
	}
}

// isSafeSingleSegment 判断 s 是否为一个安全的单段目录名：
// 不为空、不为 "." / ".."、不含路径分隔符（/ 或 \）。
func isSafeSingleSegment(s string) bool {
	if s == "" || s == "." || s == ".." {
		return false
	}
	if strings.ContainsAny(s, `/\`) {
		return false
	}
	// 清洗后若发生变化（例如含 .. 拼接），也视为不安全
	if filepath.Clean(s) != s {
		return false
	}
	return true
}

func (r *Resolver) load() (map[string]string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	ngp := os.Getenv(r.envName)
	if ngp == "" {
		return nil, fmt.Errorf("env %s not set", r.envName)
	}
	confPath := filepath.Join(ngp, r.projectsRel)
	info, err := os.Stat(confPath)
	if err != nil {
		return nil, fmt.Errorf("stat projects.conf %s: %w", confPath, err)
	}
	if r.cache != nil && r.cache.path == confPath && r.cache.mtime.Equal(info.ModTime()) {
		return r.cache.maindir, nil
	}
	data, err := os.ReadFile(confPath)
	if err != nil {
		return nil, fmt.Errorf("read projects.conf: %w", err)
	}
	m, err := parseProjectsConf(data)
	if err != nil {
		return nil, fmt.Errorf("parse projects.conf: %w", err)
	}
	r.cache = &cacheEntry{path: confPath, mtime: info.ModTime(), maindir: m}
	r.logger.Info("projects.conf loaded", "projects", len(m), "path", confPath)
	return m, nil
}

// isWithin reports whether target is within root (root prefix, after cleaning).
func isWithin(target, root string) bool {
	if root == "" {
		return false
	}
	rel, err := filepath.Rel(root, target)
	if err != nil {
		return false
	}
	rel = filepath.ToSlash(rel)
	if rel == ".." || strings.HasPrefix(rel, "../") {
		return false
	}
	return true
}

// --- strict XML schema for projects.conf ---

type projectsConf struct {
	XMLName  xml.Name       `xml:"Projects"`
	Projects []projectEntry `xml:"project"`
}

type projectEntry struct {
	Name    string `xml:"name"`
	DBName  string `xml:"dbname"`
	Maindir string `xml:"maindir"`
}

// parseProjectsConf parses the projects.conf XML into a project->maindir map.
func parseProjectsConf(data []byte) (map[string]string, error) {
	var conf projectsConf
	if err := xml.Unmarshal(data, &conf); err != nil {
		return nil, err
	}
	out := make(map[string]string, len(conf.Projects))
	for _, p := range conf.Projects {
		if p.Name == "" || p.Maindir == "" {
			continue
		}
		if _, exists := out[p.Name]; !exists {
			out[p.Name] = p.Maindir
		}
	}
	return out, nil
}
