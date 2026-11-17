package store

import (
	"database/sql"
	"strconv"
	"time"
)

// UsageCall is one model invocation detail inside a usage.report.
type UsageCall struct {
	Model        string
	Source       string
	InputTokens  int64
	OutputTokens int64
	TTFTMs       int64
	TokensPerS   float64
	DurationMs   int64
}

// UsageReportIn is the parsed usage.report payload.
type UsageReportIn struct {
	TaskID     string
	InstanceID string
	Calls      []UsageCall
}

// UsageRec is one aggregated (model, source) row of the usage dashboard.
type UsageRec struct {
	Model        string  `json:"model"`
	Source       string  `json:"source"`
	Calls        int64   `json:"calls"`
	InputTokens  int64   `json:"input_tokens"`
	OutputTokens int64   `json:"output_tokens"`
	AvgTTFTMs    float64 `json:"avg_ttft_ms"`
	AvgSpeedTPS  float64 `json:"avg_speed_tokens_per_s"`
}

// UsageLogRec is one raw usage_logs row (history view).
type UsageLogRec struct {
	ID           int64   `json:"id"`
	TaskID       string  `json:"task_id"`
	InstanceID   string  `json:"instance_id"`
	Model        string  `json:"model"`
	Source       string  `json:"source"`
	InputTokens  int64   `json:"input_tokens"`
	OutputTokens int64   `json:"output_tokens"`
	TTFTMs       int64   `json:"ttft_ms"`
	TokensPerS   float64 `json:"tokens_per_s"`
	DurationMs   int64   `json:"duration_ms"`
	CreatedAt    int64   `json:"created_at"`
}

const msHour = int64(3600 * 1000)
const msDay = int64(24 * msHour)

// UsageInsert persists one usage.report as raw detail rows.
func (d *DB) UsageInsert(rep UsageReportIn) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	now := time.Now().UnixMilli()
	for _, c := range rep.Calls {
		if _, err := d.db.Exec(
			`INSERT INTO usage_logs
 (task_id,instance_id,model,source,input_tokens,output_tokens,ttft_ms,tokens_per_s,duration_ms,created_at)
 VALUES(?,?,?,?,?,?,?,?,?,?)`,
			rep.TaskID, rep.InstanceID, c.Model, c.Source,
			c.InputTokens, c.OutputTokens, c.TTFTMs, c.TokensPerS, c.DurationMs, now,
		); err != nil {
			return err
		}
	}
	return nil
}

// UsageAggregate folds usage_logs rows past the watermark into the hourly and
// daily aggregate tables (incremental; safe to run repeatedly). Returns the
// number of detail rows folded in.
func (d *DB) UsageAggregate() (int64, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	var wm int64
	if err := d.db.QueryRow(
		`SELECT value FROM usage_agg_state WHERE key='usage_logs_agg_id'`).Scan(&wm); err != nil {
		wm = 0
	}
	rows, err := d.db.Query(`
SELECT (created_at/`+itoa(msHour)+`)*`+itoa(msHour)+`,
       (created_at/`+itoa(msDay)+`)*`+itoa(msDay)+`,
       model, source, COUNT(*), SUM(input_tokens), SUM(output_tokens),
       SUM(ttft_ms), SUM(duration_ms)
FROM usage_logs WHERE id > ? GROUP BY 1, 2, 3, 4`, wm)
	if err != nil {
		return 0, err
	}
	defer rows.Close()
	var folded int64
	type aggRow struct {
		hb, db                    int64
		model, source             string
		calls, in, out, ttft, dur int64
	}
	var rowsData []aggRow
	for rows.Next() {
		var r aggRow
		if err := rows.Scan(&r.hb, &r.db, &r.model, &r.source,
			&r.calls, &r.in, &r.out, &r.ttft, &r.dur); err != nil {
			return 0, err
		}
		folded += r.calls
		rowsData = append(rowsData, r)
	}
	if err := rows.Err(); err != nil {
		return 0, err
	}
	upsert := func(table string, bucket int64, r aggRow) error {
		_, err := d.db.Exec(
			`INSERT INTO `+table+`
 (bucket,model,source,calls,input_tokens,output_tokens,ttft_sum,duration_sum)
 VALUES(?,?,?,?,?,?,?,?)
 ON CONFLICT(bucket,model,source) DO UPDATE SET
 calls=calls+excluded.calls,
 input_tokens=input_tokens+excluded.input_tokens,
 output_tokens=output_tokens+excluded.output_tokens,
 ttft_sum=ttft_sum+excluded.ttft_sum,
 duration_sum=duration_sum+excluded.duration_sum`,
			bucket, r.model, r.source, r.calls, r.in, r.out, r.ttft, r.dur)
		return err
	}
	for _, r := range rowsData {
		if err := upsert("usage_stats_hourly", r.hb, r); err != nil {
			return 0, err
		}
		if err := upsert("usage_stats_daily", r.db, r); err != nil {
			return 0, err
		}
	}
	if folded == 0 {
		return 0, nil
	}
	var maxID sql.NullInt64
	if err := d.db.QueryRow(`SELECT MAX(id) FROM usage_logs`).Scan(&maxID); err != nil {
		return 0, err
	}
	if maxID.Valid {
		if _, err := d.db.Exec(
			`INSERT INTO usage_agg_state(key,value) VALUES('usage_logs_agg_id',?)
 ON CONFLICT(key) DO UPDATE SET value=excluded.value`, maxID.Int64); err != nil {
			return 0, err
		}
	}
	return folded, nil
}

// UsageSummary returns (model, source)-grouped aggregates for the window
// [sinceMs, now] of the given stats table ("usage_stats_hourly" or
// "usage_stats_daily"). Average speed is derived as output tokens / duration,
// the most robust aggregation of per-call speed.
func (d *DB) UsageSummary(table string, sinceMs int64) ([]UsageRec, error) {
	if table != "usage_stats_hourly" && table != "usage_stats_daily" {
		table = "usage_stats_hourly"
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	rows, err := d.db.Query(
		`SELECT model, source, SUM(calls), SUM(input_tokens), SUM(output_tokens),
 SUM(ttft_sum), SUM(duration_sum)
 FROM `+table+` WHERE bucket >= ? GROUP BY model, source ORDER BY SUM(output_tokens) DESC`,
		sinceMs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []UsageRec
	for rows.Next() {
		var r UsageRec
		var ttftSum, durSum int64
		if err := rows.Scan(&r.Model, &r.Source, &r.Calls, &r.InputTokens,
			&r.OutputTokens, &ttftSum, &durSum); err != nil {
			return nil, err
		}
		if r.Calls > 0 {
			r.AvgTTFTMs = float64(ttftSum) / float64(r.Calls)
		}
		if durSum > 0 {
			r.AvgSpeedTPS = float64(r.OutputTokens) / (float64(durSum) / 1000.0)
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// UsageLogsList returns recent raw detail rows, newest first.
func (d *DB) UsageLogsList(limit int) ([]UsageLogRec, error) {
	if limit <= 0 {
		limit = 200
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	rows, err := d.db.Query(
		`SELECT id,task_id,instance_id,model,source,input_tokens,output_tokens,
 ttft_ms,tokens_per_s,duration_ms,created_at
 FROM usage_logs ORDER BY id DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []UsageLogRec
	for rows.Next() {
		var r UsageLogRec
		if err := rows.Scan(&r.ID, &r.TaskID, &r.InstanceID, &r.Model, &r.Source,
			&r.InputTokens, &r.OutputTokens, &r.TTFTMs, &r.TokensPerS,
			&r.DurationMs, &r.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// UsageCleanup deletes raw usage_logs older than the retention window
// (default 31 days). Aggregated tables are kept indefinitely. Returns the
// number of deleted rows.
func (d *DB) UsageCleanup(retention time.Duration) (int64, error) {
	cutoff := time.Now().Add(-retention).UnixMilli()
	res, err := d.db.Exec(`DELETE FROM usage_logs WHERE created_at < ?`, cutoff)
	if err != nil {
		return 0, err
	}
	n, _ := res.RowsAffected()
	return n, nil
}

// itoa renders an integer constant for embedding in SQL literals.
func itoa(v int64) string {
	return strconv.FormatInt(v, 10)
}
