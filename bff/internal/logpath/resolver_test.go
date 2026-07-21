package logpath

import (
	"os"
	"path/filepath"
	"testing"
)

// setEnv 设置 NGP 环境变量并写一份最小 projects.conf。
// 返回清理函数。
func setEnv(t *testing.T, maindir string) func() {
	t.Helper()
	ngpDir := t.TempDir()
	confDir := filepath.Join(ngpDir, "configs", "ndp")
	if err := os.MkdirAll(confDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// 在 maindir 占位（path 字符串有效即可，不一定真实存在）
	conf := `<Projects><project><name>qqqq</name><dbname>ndp</dbname><maindir>` + maindir + `</maindir></project></Projects>`
	if err := os.WriteFile(filepath.Join(confDir, "projects.conf"), []byte(conf), 0o644); err != nil {
		t.Fatal(err)
	}
	old, had := os.LookupEnv("NGP")
	if err := os.Setenv("NGP", ngpDir); err != nil {
		t.Fatal(err)
	}
	return func() {
		if had {
			_ = os.Setenv("NGP", old)
		} else {
			_ = os.Unsetenv("NGP")
		}
	}
}

// TestLogDir_NoLine 验证 line 为空时目录为 {surveyDir}/list|LOG。
func TestLogDir_NoLine(t *testing.T) {
	maindir := t.TempDir()
	cleanup := setEnv(t, maindir)
	defer cleanup()

	r := New("NGP", "configs/ndp/projects.conf", nil)

	got, err := r.LogDir("qqqq", "survey1", "", "list")
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(maindir, "data", "qqqq", "survey1", "list")
	if got != want {
		t.Fatalf("LogDir list: got %q, want %q", got, want)
	}

	got, err = r.LogDir("qqqq", "survey1", "", "log")
	if err != nil {
		t.Fatal(err)
	}
	want = filepath.Join(maindir, "data", "qqqq", "survey1", "LOG")
	if got != want {
		t.Fatalf("LogDir log: got %q, want %q", got, want)
	}
}

// TestLogDir_WithLine 验证 line 非空时目录为 {surveyDir}/{line}/list|LOG。
func TestLogDir_WithLine(t *testing.T) {
	maindir := t.TempDir()
	cleanup := setEnv(t, maindir)
	defer cleanup()

	r := New("NGP", "configs/ndp/projects.conf", nil)

	got, err := r.LogDir("qqqq", "survey1", "lineA", "list")
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(maindir, "data", "qqqq", "survey1", "lineA", "list")
	if got != want {
		t.Fatalf("LogDir list with line: got %q, want %q", got, want)
	}

	got, err = r.LogDir("qqqq", "survey1", "lineA", "log")
	if err != nil {
		t.Fatal(err)
	}
	want = filepath.Join(maindir, "data", "qqqq", "survey1", "lineA", "LOG")
	if got != want {
		t.Fatalf("LogDir log with line: got %q, want %q", got, want)
	}
}

// TestLogDir_LineTraversalBlocked 验证 line 含 ".." 时被路径穿越检查拦截。
func TestLogDir_LineTraversalBlocked(t *testing.T) {
	maindir := t.TempDir()
	cleanup := setEnv(t, maindir)
	defer cleanup()

	r := New("NGP", "configs/ndp/projects.conf", nil)

	_, err := r.LogDir("qqqq", "survey1", "..", "list")
	if err == nil {
		t.Fatal("expected error for line=.. but got nil")
	}
	_, err = r.LogDir("qqqq", "survey1", "../../etc", "list")
	if err == nil {
		t.Fatal("expected error for line=../../etc but got nil")
	}
}
