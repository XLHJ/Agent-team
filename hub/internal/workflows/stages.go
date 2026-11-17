package workflows

import (
	"database/sql"
	"time"
)

// listRunStages returns the stage records of one run ordered by stage order.
func (s *Store) listRunStages(runID string) ([]RunStage, error) {
	rows, err := s.db.Query(`SELECT rs.run_id,rs.stage_id,rs.task_id,rs.status,
 rs.report,rs.raw_output,rs.started_at,rs.finished_at
 FROM workflow_run_stages rs
 LEFT JOIN workflow_stages st ON st.stage_id=rs.stage_id
 WHERE rs.run_id=? ORDER BY st.order_index`, runID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []RunStage
	for rows.Next() {
		var r RunStage
		var taskID, report, raw sql.NullString
		var started, finished sql.NullInt64
		if err := rows.Scan(&r.RunID, &r.StageID, &taskID, &r.Status,
			&report, &raw, &started, &finished); err != nil {
			return nil, err
		}
		r.TaskID = taskID.String
		r.Report = report.String
		r.RawOutput = raw.String
		if started.Valid {
			t := time.UnixMilli(started.Int64)
			r.StartedAt = &t
		}
		if finished.Valid {
			t := time.UnixMilli(finished.Int64)
			r.FinishedAt = &t
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// RunStages exposes the stage records of a run.
func (s *Store) RunStages(runID string) ([]RunStage, error) {
	return s.listRunStages(runID)
}

// SetRunStageTask records the dispatched task id of one stage.
func (s *Store) SetRunStageTask(runID, stageID, taskID string) error {
	_, err := s.db.Exec(`UPDATE workflow_run_stages
 SET status=?, task_id=? WHERE run_id=? AND stage_id=?`,
		SSDispatched, taskID, runID, stageID)
	return err
}
