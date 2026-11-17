package store

import (
	"database/sql"
	"encoding/json"
	"errors"
	"time"
)

// ApprovalRow mirrors the approvals table (doc section 6.1).
type ApprovalRow struct {
	ApprovalID     string         `json:"approval_id"`
	TaskID         string         `json:"task_id"`
	InstanceID     string         `json:"instance_id"`
	OperationType  string         `json:"operation_type"`
	RiskLevel      string         `json:"risk_level"`
	ContextSummary string         `json:"context_summary"`
	TimeoutMS      int64          `json:"timeout_ms"`
	Mode           string         `json:"mode"`
	Status         string         `json:"status"`
	Decision       sql.NullString `json:"decision"`
	ModifiedParams sql.NullString `json:"modified_params"`
	Reason         sql.NullString `json:"reason"`
	ParamsPreview  sql.NullString `json:"params_preview"`
	CreatedAt      time.Time      `json:"created_at"`
	ResolvedAt     *time.Time     `json:"resolved_at"`
}

// ErrDupApproval is returned when approval_id already exists.
var ErrDupApproval = errors.New("approval_id already exists")

// ErrAlreadyDone maps to APPROVAL_ALREADY_RESOLVED.
var ErrAlreadyDone = errors.New("approval already resolved")

// ErrNotFound maps to APPROVAL_NOT_FOUND.
var ErrNotFound = errors.New("approval not found")

// SaveApproval inserts one pending row. DB-first commit point.
func (d *DB) SaveApproval(a *ApprovalRow) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	var dup int
	if err := d.db.QueryRow(
		"SELECT COUNT(*) FROM approvals WHERE approval_id=?",
		a.ApprovalID).Scan(&dup); err != nil {
		return err
	}
	if dup > 0 {
		return ErrDupApproval
	}
	_, err := d.db.Exec(`INSERT INTO approvals(
 approval_id,task_id,instance_id,operation_type,risk_level,
 context_summary,timeout_ms,mode,status,created_at)
VALUES(?,?,?,?,?,?,?,?,?,?)`,
		a.ApprovalID, a.TaskID, a.InstanceID, a.OperationType,
		a.RiskLevel, a.ContextSummary, a.TimeoutMS, a.Mode,
		"pending", a.CreatedAt.UnixMilli())
	return err
}

// ResolveApproval applies a decision. Returns ErrAlreadyDone when the row is
// past PENDING (doc 2.1: terminal states are irreversible).
func (d *DB) ResolveApproval(id, status, decision, modified, reason string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	res, err := d.db.Exec(`UPDATE approvals SET
 status=?, decision=?, modified_params=?, reason=?, resolved_at=?
 WHERE approval_id=? AND status='pending'`,
		status, decision, nullIfEmpty(modified),
		nullIfEmpty(reason), time.Now().UnixMilli(), id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		if d.approvalExists(id) {
			return ErrAlreadyDone
		}
		return ErrNotFound
	}
	return nil
}

func (d *DB) approvalExists(id string) bool {
	var n int
	_ = d.db.QueryRow(
		"SELECT COUNT(*) FROM approvals WHERE approval_id=?",
		id).Scan(&n)
	return n > 0
}

func nullIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// GetApproval returns one row by id.
func (d *DB) GetApproval(id string) (*ApprovalRow, error) {
	r := &ApprovalRow{}
	var c int64
	var rs *int64
	row := d.db.QueryRow(`SELECT approval_id,task_id,instance_id,
 operation_type,risk_level,context_summary,timeout_ms,mode,status,
 decision,modified_params,reason,params_preview,created_at,resolved_at
 FROM approvals WHERE approval_id=?`, id)

	if err := row.Scan(&r.ApprovalID, &r.TaskID, &r.InstanceID,
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
	return r, nil
}

// MarshalJSON flattens sql.NullString fields to bare values and omits them
// when invalid, so HTTP snapshots match the Phase-1/2 JSON shape.
func (r ApprovalRow) MarshalJSON() ([]byte, error) {
	type alias struct {
		ApprovalID     string     `json:"approval_id"`
		TaskID         string     `json:"task_id"`
		InstanceID     string     `json:"instance_id"`
		OperationType  string     `json:"operation_type"`
		RiskLevel      string     `json:"risk_level"`
		ContextSummary string     `json:"context_summary"`
		TimeoutMS      int64      `json:"timeout_ms"`
		Mode           string     `json:"mode"`
		Status         string     `json:"status"`
		Decision       *string    `json:"decision,omitempty"`
		ModifiedParams *string    `json:"modified_params,omitempty"`
		Reason         *string    `json:"reason,omitempty"`
		ParamsPreview  *string    `json:"params_preview,omitempty"`
		CreatedAt      time.Time  `json:"created_at"`
		ResolvedAt     *time.Time `json:"resolved_at,omitempty"`
	}
	str := func(n sql.NullString) *string {
		if !n.Valid {
			return nil
		}
		v := n.String
		return &v
	}
	return json.Marshal(alias{
		ApprovalID:     r.ApprovalID,
		TaskID:         r.TaskID,
		InstanceID:     r.InstanceID,
		OperationType:  r.OperationType,
		RiskLevel:      r.RiskLevel,
		ContextSummary: r.ContextSummary,
		TimeoutMS:      r.TimeoutMS,
		Mode:           r.Mode,
		Status:         r.Status,
		Decision:       str(r.Decision),
		ModifiedParams: str(r.ModifiedParams),
		Reason:         str(r.Reason),
		ParamsPreview:  str(r.ParamsPreview),
		CreatedAt:      r.CreatedAt,
		ResolvedAt:     r.ResolvedAt,
	})
}
