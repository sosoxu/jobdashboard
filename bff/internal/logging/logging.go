package logging

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/dashboard/bff/internal/config"
)

// Setup 根据配置初始化全局 slog logger。
//   - cfg.Log.File 为空：只输出到 stdout（兼容旧行为）。
//   - cfg.Log.File 非空：同时输出到 stdout 和文件；文件超过 MaxSizeMB 时滚动切分，
//     旧文件重命名为 <file>.<timestamp>.log，超过 MaxBackups 数量则删除最旧。
//
// 返回的 cleanup 函数用于关闭文件句柄，应在进程退出时调用。
func Setup(cfg *config.LogCfg) (cleanup func(), err error) {
	level := parseLevel(cfg.Level)

	if cfg.File == "" {
		// 兼容旧行为：只输出到 stdout
		logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: level}))
		slog.SetDefault(logger)
		return func() {}, nil
	}

	// 创建日志目录
	if dir := filepath.Dir(cfg.File); dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("create log dir %s: %w", dir, err)
		}
	}

	rotator, err := newRotator(cfg.File, cfg.MaxSizeMB, cfg.MaxBackups, cfg.MaxAgeDays)
	if err != nil {
		return nil, err
	}

	// 同时输出到 stdout 和文件
	multi := io.MultiWriter(os.Stdout, rotator)
	logger := slog.New(slog.NewTextHandler(multi, &slog.HandlerOptions{Level: level}))
	slog.SetDefault(logger)

	slog.Info("logger initialized",
		"file", cfg.File,
		"level", cfg.Level,
		"maxSizeMB", cfg.MaxSizeMB,
		"maxBackups", cfg.MaxBackups,
		"maxAgeDays", cfg.MaxAgeDays)

	return func() {
		_ = rotator.Close()
	}, nil
}

func parseLevel(s string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// rotator 是一个支持按大小滚动的 io.WriteCloser。
// 每次写入时检查当前文件大小，超过 maxSize 时：
//   - 关闭当前文件
//   - 重命名为 <file>.<时间戳>.log（时间戳精确到秒，避免覆盖）
//   - 按 MaxBackups 限制删除多余的旧文件
//   - 打开新文件继续写入
//
// 注意：rotator 内部使用互斥锁保护并发写入。
type rotator struct {
	mu         sync.Mutex
	filePath   string
	fp         *os.File
	currentSize int64
	maxSize    int64
	maxBackups int
	maxAgeDays int
}

func newRotator(filePath string, maxSizeMB, maxBackups, maxAgeDays int) (*rotator, error) {
	if maxSizeMB <= 0 {
		maxSizeMB = 100
	}
	if maxBackups < 0 {
		maxBackups = 7
	}
	if maxAgeDays < 0 {
		maxAgeDays = 30
	}
	r := &rotator{
		filePath:   filePath,
		maxSize:    int64(maxSizeMB) * 1024 * 1024,
		maxBackups: maxBackups,
		maxAgeDays: maxAgeDays,
	}
	if err := r.openFile(); err != nil {
		return nil, err
	}
	return r, nil
}

func (r *rotator) openFile() error {
	fp, err := os.OpenFile(r.filePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return fmt.Errorf("open log file %s: %w", r.filePath, err)
	}
	info, err := fp.Stat()
	if err != nil {
		fp.Close()
		return fmt.Errorf("stat log file %s: %w", r.filePath, err)
	}
	r.fp = fp
	r.currentSize = info.Size()
	return nil
}

func (r *rotator) Write(p []byte) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// 检查是否需要滚动（写入前判断，避免写入超限）
	if r.currentSize+int64(len(p)) > r.maxSize {
		if err := r.rotate(); err != nil {
			// 滚动失败时仍尝试写入原文件，避免日志丢失
			slog.Error("log rotate failed", "err", err)
		}
	}

	n, err := r.fp.Write(p)
	r.currentSize += int64(n)
	return n, err
}

func (r *rotator) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.fp != nil {
		return r.fp.Close()
	}
	return nil
}

func (r *rotator) rotate() error {
	// 关闭当前文件
	if r.fp != nil {
		_ = r.fp.Close()
		r.fp = nil
	}

	// 重命名当前文件为带时间戳的备份
	ts := time.Now().Format("20060102-150405")
	backupPath := fmt.Sprintf("%s.%s.log", r.filePath, ts)
	if err := os.Rename(r.filePath, backupPath); err != nil {
		// 重命名失败可能是因为文件已被其他进程移动；尝试继续打开新文件
		slog.Error("rename log file failed", "from", r.filePath, "to", backupPath, "err", err)
	}

	// 清理超量的旧备份文件
	r.pruneBackups()

	// 清理过期的旧备份文件
	if r.maxAgeDays > 0 {
		r.pruneByAge()
	}

	// 打开新文件
	return r.openFile()
}

// pruneBackups 按 MaxBackups 数量限制删除多余的旧备份文件。
// 备份文件命名格式：<filePath>.<timestamp>.log，按文件名（含时间戳）降序排序后保留最新的 N 个。
func (r *rotator) pruneBackups() {
	if r.maxBackups <= 0 {
		return
	}
	dir := filepath.Dir(r.filePath)
	base := filepath.Base(r.filePath)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	var backups []string
	prefix := base + "."
	suffix := ".log"
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if strings.HasPrefix(name, prefix) && strings.HasSuffix(name, suffix) {
			backups = append(backups, filepath.Join(dir, name))
		}
	}
	// 降序排序（最新的在前）
	sort.Sort(sort.Reverse(sort.StringSlice(backups)))
	// 删除超量的旧文件
	for i := r.maxBackups; i < len(backups); i++ {
		_ = os.Remove(backups[i])
	}
}

// pruneByAge 删除超过 MaxAgeDays 天的旧备份文件。
func (r *rotator) pruneByAge() {
	dir := filepath.Dir(r.filePath)
	base := filepath.Base(r.filePath)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	prefix := base + "."
	suffix := ".log"
	cutoff := time.Now().AddDate(0, 0, -r.maxAgeDays)
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasPrefix(name, prefix) || !strings.HasSuffix(name, suffix) {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		if info.ModTime().Before(cutoff) {
			_ = os.Remove(filepath.Join(dir, name))
		}
	}
}
