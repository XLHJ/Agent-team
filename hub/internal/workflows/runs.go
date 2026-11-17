package workflows

import (
	"database/sql"
	"time"
)

// CreateRun inserts a run row and its pending stage rows.
func (s *Store) CreateRun(r Run, stages []Stage) error {
	if _, err := s.db.Exec(`INSERT INTO workflow_runs
 (run_id,workflow_id,status,current_stage_index,started_at)
 VALUES(?,?,?,?,?)`,
		r.RunID, r.WorkflowID, r.Status, r.CurrentStageIndex,
		r.StartedAt.UnixMilli()); err != nil {
		return err
	}
	for _, st := range stages {
		if _, err := s.db.Exec(`INSERT INTO workflow_run_stages
 (run_id,stage_id,status) VALUES(?,?,?)`,
			r.RunID, st.StageID, SSPending); err != nil {
			return err
		}
	}
	return nil
}

// UpdateRunStatus writes run status, stage pointer and failure reason.
func (s *Store) UpdateRunStatus(runID, status string, stageIdx int, reason string) error {
	var finished any
	if status != RSRunning && status != RSCreated {
		finished = time.Now().UnixMilli()
	}
	_, err := s.db.Exec(`UPDATE workflow_runs SET
 status=?, current_stage_index=?, failure_reason=?, finished_at=?
 WHERE run_id=?`,
		status, stageIdx, reason, finished, runID)
	return err
}

// UpdateRunStage writes one executed stage record.
func (s *Store) UpdateRunStage(runID, stageID, status, taskID, report, raw string) error {
	var started, finished any
	if status != SSPending {
		started = time.Now().UnixMilli()
	}
	if status == SSCompleted || status == SSFailed {
		finished = time.Now().UnixMilli()
	}
	_, err := s.db.Exec(`UPDATE workflow_run_stages SET
 status=?, task_id=?, report=?, raw_output=?, started_at=?, finished_at=?
 WHERE run_id=? AND stage_id=?`,
		status, taskID, report, raw, started, finished, runID, stageID)
	return err
}

// GetRun loads one run with its stage records.
func (s *Store) GetRun(runID string) (*Run, []RunStage, error) {
	var r Run
	var started int64
	var finished sql.NullInt64
	var reason sql.NullString
	err := s.db.QueryRow(`SELECT run_id,workflow_id,status,current_stage_index,
 started_at,finished_at,failure_reason FROM workflow_runs WHERE run_id=?`, runID).
		Scan(&r.RunID, &r.WorkflowID, &r.Status, &r.CurrentStageIndex,
			&started, &finished, &reason)
	if err != nil {
		return nil, nil, err
	}
	r.StartedAt = time.UnixMilli(started)
	if finished.Valid {
		t := time.UnixMilli(finished.Int64)
		r.FinishedAt = &t
	}
	r.FailureReason = reason.String
	stages, err := s.listRunStages(runID)
	return &r, stages, err
}

// ListRuns returns runs of one workflow, newest first.
func (s *Store) ListRuns(workflowID string, limit int) ([]Run, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := s.db.Query(`SELECT run_id,workflow_id,status,current_stage_index,
 started_at,finished_at,failure_reason FROM workflow_runs
 WHERE workflow_id=? ORDER BY started_at DESC LIMIT ?`, workflowID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Run
	for rows.Next() {
		var r Run
		var started int64
		var finished sql.NullInt64
		var reason sql.NullString
		if err := rows.Scan(&r.RunID, &r.WorkflowID, &r.Status,
			&r.CurrentStageIndex, &started, &finished, &reason); err != nil {
			return nil, err
		}
		r.StartedAt = time.UnixMilli(started)
		if finished.Valid {
			t := time.UnixMilli(finished.Int64)
			r.FinishedAt = &t
		}
		r.FailureReason = reason.String
		out = append(out, r)
	}
	return out, rows.Err()
}

// RunningRuns returns runs still marked running (boot recovery, doc 7.2).
func (s *Store) RunningRuns() ([]Run, error) {
	rows, err := s.db.Query(`SELECT run_id,workflow_id,status,current_stage_index,
 started_at,finished_at,failure_reason FROM workflow_runs WHERE status=?`,
		RSRunning)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Run
	for rows.Next() {
		var r Run
		var started int64
		var finished sql.NullInt64
		var reason sql.NullString
		if err := rows.Scan(&r.RunID, &r.WorkflowID, &r.Status,
			&r.CurrentStageIndex, &started, &finished, &reason); err != nil {
			return nil, err
		}
		r.StartedAt = time.UnixMilli(started)
		r.FailureReason = reason.String
		out = append(out, r)
	}
	return out, rows.Err()
}

// CleanupRuns deletes runs older than the 90 day window (doc 7.1).
func (s *Store) CleanupRuns(retention time.Duration) (int64, error) {
	cutoff := time.Now().Add(-retention).UnixMilli()
	res, err := s.db.Exec("DELETE FROM workflow_runs WHERE started_at < ?", cutoff)
	if err != nil {
		return 0, err
	}
	n, _ := res.RowsAffected()
	return n, nil
}
