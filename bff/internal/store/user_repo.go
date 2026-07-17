package store

import (
	"database/sql"

	"github.com/dashboard/bff/internal/model"
)

// UserRepo persists and queries user_job_stat.
type UserRepo struct {
	db *sql.DB
}

func NewUserRepo(db *sql.DB) *UserRepo { return &UserRepo{db: db} }

// ReplaceAllForTS deletes existing rows for the given ts and inserts the batch.
// Each sampling tick writes one full snapshot of per-user counts.
func (r *UserRepo) ReplaceAllForTS(ts int64, stats []model.UserJobStat) error {
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM user_job_stat WHERE ts = ?`, ts); err != nil {
		_ = tx.Rollback()
		return err
	}
	stmt, err := tx.Prepare(`INSERT INTO user_job_stat (ts, user_name, job_count) VALUES (?, ?, ?)`)
	if err != nil {
		_ = tx.Rollback()
		return err
	}
	defer stmt.Close()
	for _, s := range stats {
		if _, err := stmt.Exec(ts, s.UserName, s.JobCount); err != nil {
			_ = tx.Rollback()
			return err
		}
	}
	return tx.Commit()
}

// TopAtLatest returns the per-user counts at the latest sampling ts,
// ordered by job_count descending, limited to `limit`. Also returns the ts.
func (r *UserRepo) TopAtLatest(limit int) (int64, []model.UserJobStat, error) {
	var ts int64
	var tsNull sql.NullInt64
	err := r.db.QueryRow(`SELECT MAX(ts) FROM user_job_stat`).Scan(&tsNull)
	if err != nil {
		return 0, nil, err
	}
	if !tsNull.Valid {
		return 0, nil, nil
	}
	ts = tsNull.Int64

	rows, err := r.db.Query(
		`SELECT user_name, job_count FROM user_job_stat WHERE ts = ?
		 ORDER BY job_count DESC LIMIT ?`, ts, limit)
	if err != nil {
		return 0, nil, err
	}
	defer rows.Close()
	var out []model.UserJobStat
	for rows.Next() {
		var s model.UserJobStat
		if err := rows.Scan(&s.UserName, &s.JobCount); err != nil {
			return 0, nil, err
		}
		out = append(out, s)
	}
	return ts, out, rows.Err()
}

// TotalAtLatest returns the total job count summed across users at the latest ts.
func (r *UserRepo) TotalAtLatest() (int, error) {
	var n sql.NullInt64
	err := r.db.QueryRow(
		`SELECT SUM(job_count) FROM user_job_stat WHERE ts = (SELECT MAX(ts) FROM user_job_stat)`).Scan(&n)
	if err != nil {
		return 0, err
	}
	if !n.Valid {
		return 0, nil
	}
	return int(n.Int64), nil
}

// DeleteOlderThan removes user stats older than the given ts.
func (r *UserRepo) DeleteOlderThan(ts int64) (int64, error) {
	res, err := r.db.Exec(`DELETE FROM user_job_stat WHERE ts < ?`, ts)
	if err != nil {
		return 0, err
	}
	n, _ := res.RowsAffected()
	return n, nil
}
