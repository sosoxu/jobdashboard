package store

import (
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

// Open opens (and migrates) the SQLite database at path.
func Open(path string) (*sql.DB, error) {
	if dir := filepath.Dir(path); dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("mkdir %s: %w", dir, err)
		}
	}
	dsn := "file:" + path + "?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}

	// 显式设置所有 pragma，不依赖 DSN（modernc.org/sqlite 的 DSN pragma 解析
	// 在某些版本可能不生效）。
	pragmas := []string{
		"PRAGMA journal_mode=WAL;",      // WAL 模式：读写不互斥
		"PRAGMA busy_timeout=5000;",     // 锁等待最多 5s
		"PRAGMA synchronous=NORMAL;",    // WAL 下安全且更快
		"PRAGMA cache_size=-8000;",      // 8MB 页缓存
		"PRAGMA foreign_keys=ON;",
	}
	for _, p := range pragmas {
		if _, err := db.Exec(p); err != nil {
			db.Close()
			return nil, fmt.Errorf("pragma %q: %w", p, err)
		}
	}

	// 连接池配置：允许并发读，限制最大连接数避免 SQLite "database is locked"。
	// modernc.org/sqlite 在 WAL 模式下支持多连接并发读。
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)

	// 启动时验证 pragma 实际生效情况，输出到日志。
	verifyPragmas(db)

	if err := migrate(db); err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}

// verifyPragmas 读取并打印 SQLite 关键 pragma 的实际值，便于生产诊断。
func verifyPragmas(db *sql.DB) {
	checks := []string{"journal_mode", "busy_timeout", "synchronous", "cache_size"}
	for _, name := range checks {
		var val string
		if err := db.QueryRow("PRAGMA " + name).Scan(&val); err != nil {
			slog.Warn("sqlite pragma verify failed", "pragma", name, "err", err)
			continue
		}
		slog.Info("sqlite pragma", "name", name, "value", val)
	}
}

func migrate(db *sql.DB) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS job_stats_snapshot (
			id          INTEGER PRIMARY KEY AUTOINCREMENT,
			ts          INTEGER NOT NULL,
			active      INTEGER NOT NULL DEFAULT 0,
			queue       INTEGER NOT NULL DEFAULT 0,
			finish      INTEGER NOT NULL DEFAULT 0,
			failed      INTEGER NOT NULL DEFAULT 0,
			canceled    INTEGER NOT NULL DEFAULT 0,
			othercount  INTEGER NOT NULL DEFAULT 0
		);`,
		`CREATE INDEX IF NOT EXISTS idx_stats_ts ON job_stats_snapshot(ts);`,

		`CREATE TABLE IF NOT EXISTS user_job_stat (
			id          INTEGER PRIMARY KEY AUTOINCREMENT,
			ts          INTEGER NOT NULL,
			user_name   TEXT NOT NULL,
			job_count   INTEGER NOT NULL DEFAULT 0
		);`,
		`CREATE INDEX IF NOT EXISTS idx_user_ts ON user_job_stat(ts);`,
		`CREATE INDEX IF NOT EXISTS idx_user_name_ts ON user_job_stat(user_name, ts);`,

		`CREATE TABLE IF NOT EXISTS sample_meta (
			key   TEXT PRIMARY KEY,
			value TEXT NOT NULL
		);`,

		`CREATE TABLE IF NOT EXISTS users (
			id            INTEGER PRIMARY KEY AUTOINCREMENT,
			username      TEXT NOT NULL UNIQUE,
			password_hash TEXT NOT NULL,
			created_at    INTEGER NOT NULL
		);`,
	}
	for _, s := range stmts {
		if _, err := db.Exec(s); err != nil {
			return fmt.Errorf("migrate: %w\nstmt: %s", err, s)
		}
	}
	return nil
}
