package service

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestParseSeqNumber 验证从 `{jobDesc}.{编号}.{jobName}.{ext}` 中提取四位数编号。
func TestParseSeqNumber(t *testing.T) {
	cases := []struct {
		name    string
		base    string
		jobName string
		ext     string
		want    int
	}{
		{"normal", "zlm-test-web1.job.0001.J2026071516507498.list", "J2026071516507498", "list", 1},
		{"four-digit", "zlm-test-web1.job.5567.J2026071516507498.log", "J2026071516507498", "log", 5567},
		{"desc-with-dots", "a.b.c.0421.J1.list", "J1", "list", 421},
		{"suffix-mismatch", "zlm-test-web1.job.0001.OTHER.list", "J2026071516507498", "list", -1},
		{"non-numeric-seq", "zlm-test-web1.job.ABC.J1.list", "J1", "list", -1},
		{"no-dot", "ABC0001J1list", "J1", "list", -1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := parseSeqNumber(tc.base, tc.jobName, tc.ext)
			if got != tc.want {
				t.Fatalf("parseSeqNumber(%q,%q,%q) = %d, want %d", tc.base, tc.jobName, tc.ext, got, tc.want)
			}
		})
	}
}

// TestFindLatestLog_ByMtime 验证：多个匹配文件中按 mtime 最新优先返回。
func TestFindLatestLog_ByMtime(t *testing.T) {
	dir := t.TempDir()
	jobName := "J2026071516507498"
	ext := "list"

	// 旧文件（编号大，但 mtime 老）→ 不应被选中
	oldPath := filepath.Join(dir, "zlm-test-web1.job.9999."+jobName+"."+ext)
	if err := os.WriteFile(oldPath, []byte("old\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	past := time.Now().Add(-2 * time.Hour)
	if err := os.Chtimes(oldPath, past, past); err != nil {
		t.Fatal(err)
	}

	// 新文件（编号小，但 mtime 新）→ 应被选中
	newPath := filepath.Join(dir, "zlm-test-web1.job.0001."+jobName+"."+ext)
	if err := os.WriteFile(newPath, []byte("new\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	if err := os.Chtimes(newPath, now, now); err != nil {
		t.Fatal(err)
	}

	got, err := findLatestLog(dir, jobName, ext)
	if err != nil {
		t.Fatal(err)
	}
	if got != newPath {
		t.Fatalf("findLatestLog returned %q, want %q", got, newPath)
	}
}

// TestFindLatestLog_BySeqTiebreaker 验证：mtime 相同时按编号降序选择。
func TestFindLatestLog_BySeqTiebreaker(t *testing.T) {
	dir := t.TempDir()
	jobName := "J1"
	ext := "log"

	// 三个文件设置相同 mtime
	ts := time.Now()
	files := []string{
		filepath.Join(dir, "job.0001."+jobName+"."+ext),
		filepath.Join(dir, "job.0042."+jobName+"."+ext),
		filepath.Join(dir, "job.0007."+jobName+"."+ext),
	}
	for _, f := range files {
		if err := os.WriteFile(f, []byte("x\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(f, ts, ts); err != nil {
			t.Fatal(err)
		}
	}

	got, err := findLatestLog(dir, jobName, ext)
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(dir, "job.0042."+jobName+"."+ext)
	if got != want {
		t.Fatalf("findLatestLog returned %q, want %q (max seq)", got, want)
	}
}

// TestFindLatestLog_NoMatch 验证：无匹配文件返回空字符串。
func TestFindLatestLog_NoMatch(t *testing.T) {
	dir := t.TempDir()
	// 写一个不匹配的文件
	if err := os.WriteFile(filepath.Join(dir, "other.J1.list"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := findLatestLog(dir, "J1", "log")
	if err != nil {
		t.Fatal(err)
	}
	if got != "" {
		t.Fatalf("findLatestLog returned %q, want empty", got)
	}
}

// TestFindLatestLog_SkipsDirectories 验证：匹配的目录条目被跳过。
func TestFindLatestLog_SkipsDirectories(t *testing.T) {
	dir := t.TempDir()
	jobName := "J1"
	ext := "list"

	// 创建一个名字匹配的目录
	if err := os.Mkdir(filepath.Join(dir, "weird.0001."+jobName+"."+ext), 0o755); err != nil {
		t.Fatal(err)
	}
	// 创建一个真正匹配的文件
	goodPath := filepath.Join(dir, "real.0002."+jobName+"."+ext)
	if err := os.WriteFile(goodPath, []byte("ok"), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := findLatestLog(dir, jobName, ext)
	if err != nil {
		t.Fatal(err)
	}
	if got != goodPath {
		t.Fatalf("findLatestLog returned %q, want %q (should skip directories)", got, goodPath)
	}
}

// TestLocateLogFiles_PathCacheTTL 验证：路径缓存命中后不再 glob。
func TestLocateLogFiles_PathCacheTTL(t *testing.T) {
	dir := t.TempDir()
	jobName := "J1"
	ext := "list"

	svc := NewLogService(nil, 100, nil)

	// 第一次调用：无文件 → 空结果
	files, err := svc.locateLogFiles(dir, jobName, ext)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 0 {
		t.Fatalf("expected no files initially, got %v", files)
	}

	// 在目录中创建匹配文件
	path := filepath.Join(dir, "job.0001."+jobName+"."+ext)
	if err := os.WriteFile(path, []byte("data"), 0o644); err != nil {
		t.Fatal(err)
	}

	// 第二次调用：仍应重新 glob（前一次空结果不写缓存）
	files, err = svc.locateLogFiles(dir, jobName, ext)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 || files[0] != path {
		t.Fatalf("expected [%s], got %v", path, files)
	}

	// 第三次调用：应命中缓存
	files2, err := svc.locateLogFiles(dir, jobName, ext)
	if err != nil {
		t.Fatal(err)
	}
	if len(files2) != 1 || files2[0] != path {
		t.Fatalf("cache hit expected [%s], got %v", path, files2)
	}

	// 删除文件：缓存应失效（stat 失败 → 重新 glob）
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	files3, err := svc.locateLogFiles(dir, jobName, ext)
	if err != nil {
		t.Fatal(err)
	}
	if len(files3) != 0 {
		t.Fatalf("expected empty after delete, got %v", files3)
	}
}

// TestLogExt 验证 logType 到后缀的映射。
func TestLogExt(t *testing.T) {
	if got := logExt("list"); got != "list" {
		t.Errorf("logExt(list) = %q, want list", got)
	}
	if got := logExt("LIST"); got != "list" {
		t.Errorf("logExt(LIST) = %q, want list", got)
	}
	if got := logExt("log"); got != "log" {
		t.Errorf("logExt(log) = %q, want log", got)
	}
	if got := logExt("LOG"); got != "log" {
		t.Errorf("logExt(LOG) = %q, want log", got)
	}
	// 未知类型默认为 list
	if got := logExt("unknown"); got != "list" {
		t.Errorf("logExt(unknown) = %q, want list (default)", got)
	}
}
