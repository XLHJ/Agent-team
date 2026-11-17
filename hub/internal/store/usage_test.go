package store

import (
	"path/filepath"
	"testing"
	"time"
)

func openTestDB(t *testing.T) *DB {
	t.Helper()
	db, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := db.Schema(); err != nil {
		t.Fatalf("schema: %v", err)
	}
	return db
}

func TestUsageInsertAggregateSummary(t *testing.T) {
	db := openTestDB(t)
	err := db.UsageInsert(UsageReportIn{
		TaskID: "t-1", InstanceID: "inst-a",
		Calls: []UsageCall{
			{Model: "dsv4.1", Source: "official", InputTokens: 100, OutputTokens: 200,
				TTFTMs: 300, TokensPerS: 40, DurationMs: 5000},
			{Model: "dsv4.1", Source: "official", InputTokens: 50, OutputTokens: 100,
				TTFTMs: 500, TokensPerS: 20, DurationMs: 5000},
			{Model: "gpt-6-sol", Source: "relay-station-A", InputTokens: 10, OutputTokens: 30,
				TTFTMs: 100, TokensPerS: 60, DurationMs: 500},
		},
	})
	if err != nil {
		t.Fatalf("insert: %v", err)
	}
	logs, err := db.UsageLogsList(10)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(logs) != 3 || logs[0].TaskID != "t-1" {
		t.Fatalf("unexpected logs: %+v", logs)
	}
	// First aggregate pass.
	folded, err := db.UsageAggregate()
	if err != nil {
		t.Fatalf("aggregate: %v", err)
	}
	if folded != 3 {
		t.Fatalf("folded = %d, want 3", folded)
	}
	sums, err := db.UsageSummary("usage_stats_hourly", 0)
	if err != nil {
		t.Fatalf("summary: %v", err)
	}
	if len(sums) != 2 {
		t.Fatalf("want 2 groups, got %d: %+v", len(sums), sums)
	}
	// Ordered by output tokens DESC: dsv4.1 (300 out) first.
	first := sums[0]
	if first.Model != "dsv4.1" || first.Source != "official" ||
		first.Calls != 2 || first.InputTokens != 150 || first.OutputTokens != 300 {
		t.Fatalf("bad first row: %+v", first)
	}
	if first.AvgTTFTMs != 400 { // (300+500)/2
		t.Fatalf("avg ttft = %v, want 400", first.AvgTTFTMs)
	}
	// speed = 300 output tokens over 10s total duration = 30 tps.
	if first.AvgSpeedTPS != 30 {
		t.Fatalf("avg speed = %v, want 30", first.AvgSpeedTPS)
	}
	// Second aggregate pass must be a no-op (watermark).
	folded2, err := db.UsageAggregate()
	if err != nil {
		t.Fatalf("aggregate2: %v", err)
	}
	if folded2 != 0 {
		t.Fatalf("re-aggregation double counted: %d", folded2)
	}
	// Daily table mirrors the same numbers.
	daily, err := db.UsageSummary("usage_stats_daily", 0)
	if err != nil {
		t.Fatalf("daily summary: %v", err)
	}
	if len(daily) != 2 || daily[0].OutputTokens != 300 {
		t.Fatalf("bad daily rows: %+v", daily)
	}
	// Cleanup keeps fresh rows.
	n, err := db.UsageCleanup(31 * 24 * time.Hour)
	if err != nil || n != 0 {
		t.Fatalf("cleanup fresh = %d, %v", n, err)
	}
}

func TestUsageCleanupRetention(t *testing.T) {
	db := openTestDB(t)
	if err := db.UsageInsert(UsageReportIn{TaskID: "t-old",
		Calls: []UsageCall{{Model: "m", Source: "s", TTFTMs: 1, DurationMs: 1}}}); err != nil {
		t.Fatalf("insert: %v", err)
	}
	// Backdate the row manually.
	if _, err := db.Exec(
		`UPDATE usage_logs SET created_at = ?`,
		time.Now().Add(-32*24*time.Hour).UnixMilli()); err != nil {
		t.Fatalf("backdate: %v", err)
	}
	n, err := db.UsageCleanup(31 * 24 * time.Hour)
	if err != nil || n != 1 {
		t.Fatalf("cleanup = %d, %v", n, err)
	}
	logs, _ := db.UsageLogsList(10)
	if len(logs) != 0 {
		t.Fatalf("logs not deleted: %+v", logs)
	}
}
