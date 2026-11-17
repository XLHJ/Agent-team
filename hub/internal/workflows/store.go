package workflows

import (
	"database/sql"
	"time"

	"agent-control-plane/hub/internal/store"
)

// Store persists workflows over the shared SQLite handle.
type Store struct {
	db *store.DB
}

// NewStore builds the store and applies the schema.
func NewStore(db *store.DB) (*Store, error) {
	if _, err := db.Exec(schemaSQL); err != nil {
		return nil, err
	}
	return &Store{db: db}, nil
}

// SaveWorkflow upserts one workflow definition and replaces its stages.
func (s *Store) SaveWorkflow(w Workflow, stages []Stage) error {
	now := time.Now().UnixMilli()
	if _, err := s.db.Exec(`INSERT INTO workflows
 (workflow_id,name,description,status,created_at,updated_at)
 VALUES(?,?,?,?,?,?)
 ON CONFLICT(workflow_id) DO UPDATE SET
  name=excluded.name, description=excluded.description,
  status=excluded.status, updated_at=excluded.updated_at`,
		w.WorkflowID, w.Name, w.Description, w.Status, now, now); err != nil {
		return err
	}
	if _, err := s.db.Exec("DELETE FROM workflow_stages WHERE workflow_id=?",
		w.WorkflowID); err != nil {
		return err
	}
	for _, st := range stages {
		if _, err := s.db.Exec(`INSERT INTO workflow_stages
 (stage_id,workflow_id,order_index,agent_instance_id,agent_capability_tag,
  task_prompt,report_prompt,timeout_ms) VALUES(?,?,?,?,?,?,?,?)`,
			st.StageID, st.WorkflowID, st.OrderIndex, st.AgentInstance,
			st.CapabilityTag, st.TaskPrompt, st.ReportPrompt, st.TimeoutMS); err != nil {
			return err
		}
	}
	return nil
}

// ListWorkflows returns chains newest first.
func (s *Store) ListWorkflows() ([]Workflow, error) {
	rows, err := s.db.Query(`SELECT workflow_id,name,description,status,
 created_at,updated_at FROM workflows ORDER BY updated_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Workflow
	for rows.Next() {
		var w Workflow
		var c, u int64
		var desc sql.NullString
		if err := rows.Scan(&w.WorkflowID, &w.Name, &desc, &w.Status, &c, &u); err != nil {
			return nil, err
		}
		w.Description = desc.String
		w.CreatedAt = time.UnixMilli(c)
		w.UpdatedAt = time.UnixMilli(u)
		out = append(out, w)
	}
	return out, rows.Err()
}

// GetWorkflow loads one chain with its stages ordered.
func (s *Store) GetWorkflow(id string) (*Workflow, []Stage, error) {
	var w Workflow
	var c, u int64
	var desc sql.NullString
	err := s.db.QueryRow(`SELECT workflow_id,name,description,status,
 created_at,updated_at FROM workflows WHERE workflow_id=?`, id).
		Scan(&w.WorkflowID, &w.Name, &desc, &w.Status, &c, &u)
	if err != nil {
		return nil, nil, err
	}
	w.Description = desc.String
	w.CreatedAt = time.UnixMilli(c)
	w.UpdatedAt = time.UnixMilli(u)
	rows, err := s.db.Query(`SELECT stage_id,workflow_id,order_index,
 agent_instance_id,agent_capability_tag,task_prompt,report_prompt,timeout_ms
 FROM workflow_stages WHERE workflow_id=? ORDER BY order_index`, id)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	var stages []Stage
	for rows.Next() {
		var st Stage
		var inst, tag sql.NullString
		if err := rows.Scan(&st.StageID, &st.WorkflowID, &st.OrderIndex,
			&inst, &tag, &st.TaskPrompt, &st.ReportPrompt, &st.TimeoutMS); err != nil {
			return nil, nil, err
		}
		st.AgentInstance = inst.String
		st.CapabilityTag = tag.String
		stages = append(stages, st)
	}
	return &w, stages, rows.Err()
}

// DeleteWorkflow removes the definition and stages (runs stay for audit).
func (s *Store) DeleteWorkflow(id string) error {
	if _, err := s.db.Exec("DELETE FROM workflow_stages WHERE workflow_id=?", id); err != nil {
		return err
	}
	_, err := s.db.Exec("DELETE FROM workflows WHERE workflow_id=?", id)
	return err
}

// Audit writes one audit_log row (shared audit table with Phase 4).
func (s *Store) Audit(eventType, payload string) error {
	return s.db.Audit(eventType, payload)
}

// AllRunningRuns returns every run currently marked running.
func (s *Store) AllRunningRuns() ([]Run, error) {
	return s.RunningRuns()
}
