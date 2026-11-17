package store

import "time"

// ListApprovals filters rows; empty filter = match all; newest first.
func (d *DB) ListApprovals(status, instanceID, taskID string, limit int) ([]*ApprovalRow, error) {
	if limit <= 0 {
		limit = 200
	}
	q := `SELECT approval_id,task_id,instance_id,
 operation_type,risk_level,context_summary,timeout_ms,mode,status,
 decision,modified_params,reason,params_preview,created_at,resolved_at
 FROM approvals WHERE 1=1`
	var args []any
	if status != "" {
		q += " AND status=?"
		args = append(args, status)
	}
	if instanceID != "" {
		q += " AND instance_id=?"
		args = append(args, instanceID)
	}
	if taskID != "" {
		q += " AND task_id=?"
		args = append(args, taskID)
	}
	q += " ORDER BY created_at DESC, approval_id LIMIT ?"
	args = append(args, limit)
	rows, err := d.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*ApprovalRow
	for rows.Next() {
		r := &ApprovalRow{}
		var c int64
		var rs *int64
		if err := rows.Scan(&r.ApprovalID, &r.TaskID, &r.InstanceID,
			&r.OperationType, &r.RiskLevel, &r.ContextSummary,
			&r.TimeoutMS, &r.Mode, &r.Status,
			&r.Decision, &r.ModifiedParams, &r.Reason, &r.ParamsPreview,
			&c, &rs); err != nil {
			return nil, err
		}
		r.CreatedAt = time.UnixMilli(c)
		if rs != nil {
			t := time.UnixMilli(*rs)
			r.ResolvedAt = &t
		}
		out = append(out, r)
	}
	return out, rows.Err()
}
