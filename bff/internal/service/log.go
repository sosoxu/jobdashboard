package service

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/dashboard/bff/internal/logpath"
)

// LogService reads job log files from the filesystem.
type LogService struct {
	resolver   *logpath.Resolver
	maxLines   int
}

func NewLogService(resolver *logpath.Resolver, maxLines int) *LogService {
	if maxLines <= 0 {
		maxLines = 5000
	}
	return &LogService{resolver: resolver, maxLines: maxLines}
}

// LogLevel constants.
const (
	LevelAll   = "all"
	LevelInfo  = "info"
	LevelWarn  = "warn"
	LevelError = "error"
)

// LogLine is one classified log line.
type LogLine struct {
	Level string `json:"level"`
	Ts    string `json:"ts"`
	Msg   string `json:"msg"`
}

// LogResult is the response of the log read endpoint.
type LogResult struct {
	JobName   string    `json:"jobName"`
	Type      string    `json:"type"`
	Path      string    `json:"path"`
	Lines     []LogLine `json:"lines"`
	Truncated bool      `json:"truncated"`
}

// Read reads a job's log file (or directory) for the given type.
func (s *LogService) Read(ctx context.Context, jobName, project, survey, logType, level, keyword string) (*LogResult, error) {
	path, err := s.resolver.LogPath(project, survey, logType)
	if err != nil {
		return nil, err
	}
	level = strings.ToLower(level)
	if level == "" {
		level = LevelAll
	}

	files, err := logFiles(path)
	if err != nil {
		return nil, err
	}

	out := &LogResult{JobName: jobName, Type: logType, Path: path, Lines: []LogLine{}}
	count := 0
	for _, f := range files {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		fh, err := os.Open(f)
		if err != nil {
			continue
		}
		scanner := bufio.NewScanner(fh)
		// Allow long lines.
		buf := make([]byte, 0, 1024*1024)
		scanner.Buffer(buf, 1024*1024)
		for scanner.Scan() {
			line := scanner.Text()
			lvl, ts, msg := classify(line)
			if !matchLevel(lvl, level) {
				continue
			}
			if keyword != "" && !strings.Contains(strings.ToLower(msg), strings.ToLower(keyword)) {
				continue
			}
			out.Lines = append(out.Lines, LogLine{Level: lvl, Ts: ts, Msg: msg})
			count++
			if count >= s.maxLines {
				out.Truncated = true
				fh.Close()
				return out, nil
			}
		}
		fh.Close()
	}
	return out, nil
}

// logFiles returns the list of files to read for a path. If path is a file,
// returns [path]. If path is a directory, returns its regular files sorted.
func logFiles(path string) ([]string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("日志路径不可访问: %s (%w)", path, err)
	}
	if !info.IsDir() {
		return []string{path}, nil
	}
	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, fmt.Errorf("读取日志目录失败: %w", err)
	}
	var files []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		files = append(files, filepath.Join(path, e.Name()))
	}
	sort.Strings(files)
	return files, nil
}

// classify detects level, timestamp and message from a raw log line.
var (
	tsRe   = regexp.MustCompile(`\b(\d{4}[-/]\d{2}[-/]\d{2}[ T]\d{2}:\d{2}:\d{2}([.,]\d+)?)`)
	lvlRe  = regexp.MustCompile(`(?i)\b(INFO|WARN(?:ING)?|ERROR(?:S)?|DEBUG|FATAL)\b`)
)

func classify(line string) (level, ts, msg string) {
	msg = line
	if m := tsRe.FindStringSubmatch(line); len(m) > 1 {
		ts = m[1]
	}
	if m := lvlRe.FindStringSubmatch(line); len(m) > 1 {
		level = normalizeLevel(m[1])
	}
	if level == "" {
		// Fallback: keyword sniff.
		low := strings.ToLower(line)
		switch {
		case strings.Contains(low, "error") || strings.Contains(low, "fail"):
			level = LevelError
		case strings.Contains(low, "warn"):
			level = LevelWarn
		default:
			level = LevelInfo
		}
	}
	return level, ts, msg
}

func normalizeLevel(s string) string {
	switch strings.ToUpper(s) {
	case "INFO":
		return LevelInfo
	case "WARN", "WARNING":
		return LevelWarn
	case "ERROR", "ERRORS":
		return LevelError
	case "FATAL":
		return LevelError
	case "DEBUG":
		return LevelInfo
	default:
		return LevelInfo
	}
}

func matchLevel(lineLevel, filter string) bool {
	if filter == LevelAll || filter == "" {
		return true
	}
	return lineLevel == filter
}
