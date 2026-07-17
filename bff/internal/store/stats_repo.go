package store

import (
	"database/sql"

	"github.com/dashboard/bff/internal/model"
)

// StatsRepo persists and queries job_stats_snapshot.
type StatsRepo struct {
	db *sql.DB
}

func NewStatsRepo(db *sql.DB) *StatsRepo { return &StatsRepo{db: db} }

func (r *StatsRepo) Insert(s model.StatsSnapshot) error {
	_, err := r.db.Exec(
		`INSERT INTO job_stats_snapshot (ts, active, queue, finish, failed, canceled, othercount)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		s.Ts, s.Active, s.Queue, s.Finish, s.Failed, s.Canceled, s.OtherCount,
	)
	return err
}

// Latest returns the most recent snapshot, or sql.ErrNoRows if none.
func (r *StatsRepo) Latest() (model.StatsSnapshot, error) {
	row := r.db.QueryRow(
		`SELECT ts, active, queue, finish, failed, canceled, othercount
		 FROM job_stats_snapshot ORDER BY ts DESC LIMIT 1`)
	var s model.StatsSnapshot
	err := row.Scan(&s.Ts, &s.Active, &s.Queue, &s.Finish, &s.Failed, &s.Canceled, &s.OtherCount)
	return s, err
}

// PreviousBefore returns the latest snapshot strictly older than the given ts.
func (r *StatsRepo) PreviousBefore(ts int64) (model.StatsSnapshot, error) {
	row := r.db.QueryRow(
		`SELECT ts, active, queue, finish, failed, canceled, othercount
		 FROM job_stats_snapshot WHERE ts < ? ORDER BY ts DESC LIMIT 1`, ts)
	var s model.StatsSnapshot
	err := row.Scan(&s.Ts, &s.Active, &s.Queue, &s.Finish, &s.Failed, &s.Canceled, &s.OtherCount)
	return s, err
}

// Range returns snapshots with ts in [from, to], ordered ascending.
func (r *StatsRepo) Range(from, to int64) ([]model.StatsSnapshot, error) {
	rows, err := r.db.Query(
		`SELECT ts, active, queue, finish, failed, canceled, othercount
		 FROM job_stats_snapshot WHERE ts >= ? AND ts <= ? ORDER BY ts ASC`,
		from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.StatsSnapshot
	for rows.Next() {
		var s model.StatsSnapshot
		if err := rows.Scan(&s.Ts, &s.Active, &s.Queue, &s.Finish, &s.Failed, &s.Canceled, &s.OtherCount); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// DeleteOlderThan removes snapshots older than the given ts.
func (r *StatsRepo) DeleteOlderThan(ts int64) (int64, error) {
	res, err := r.db.Exec(`DELETE FROM job_stats_snapshot WHERE ts < ?`, ts)
	if err != nil {
		return 0, err
	}
	n, _ := res.RowsAffected()
	return n, nil
}

// MetaGet / MetaSet for sample_meta key/value.
func (r *StatsRepo) MetaGet(key string) (string, error) {
	var v string
	err := r.db.QueryRow(`SELECT value FROM sample_meta WHERE key=?`, key).Scan(&v)
	return v, err
}

func (r *StatsRepo) MetaSet(key, value string) error {
	_, err := r.db.Exec(
		`INSERT INTO sample_meta(key, value) VALUES(?, ?)
		 ON CONFLICT(key) DO UPDATE SET value=excluded.value`, key, value)
	return err
}

// LatestSnapshotTS returns the latest snapshot ts, or 0 if none.
func (r *StatsRepo) LatestSnapshotTS() (int64, error) {
	var ts sql.NullInt64
	err := r.db.QueryRow(`SELECT MAX(ts) FROM job_stats_snapshot`).Scan(&ts)
	if err != nil {
		return 0, err
	}
	if !ts.Valid {
		return 0, nil
	}
	return ts.Int64, nil
}


