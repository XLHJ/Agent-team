package store

import "time"

// CleanupOld deletes resolved approvals older than the retention window.
// PENDING rows are never deleted (doc section 6.3). Returns deleted count.
func (d *DB) CleanupOld(retention time.Duration) (int64, error) {
	cutoff := time.Now().Add(-retention).UnixMilli()
	res, err := d.db.Exec(`DELETE FROM approvals
 WHERE status!='pending' AND resolved_at IS NOT NULL
 AND resolved_at < ?`, cutoff)
	if err != nil {
		return 0, err
	}
	n, _ := res.RowsAffected()
	return n, nil
}
