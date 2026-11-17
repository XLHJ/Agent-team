// Package workflows implements the Phase-5 sequential collaboration chain:
// workflow/stage/run/run-stage persistence plus the sequential executor.
package workflows

import "time"

// Workflow status values.
const (
	WSDraft    = "draft"
	WSActive   = "active"
	WSArchived = "archived"
)

// Run status values.
const (
	RSCreated   = "created"
	RSRunning   = "running"
	RSCompleted = "completed"
	RSFailed    = "failed"
	RSCanceled  = "canceled"
)

// Stage status values.
const (
	SSPending    = "pending"
	SSDispatched = "dispatched"
	SSRunning    = "running"
	SSCompleted  = "completed"
	SSFailed     = "failed"
)

// Workflow is one chain definition.
type Workflow struct {
	WorkflowID  string    `json:"workflow_id"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	Status      string    `json:"status"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// Stage is one stage definition inside a workflow.
type Stage struct {
	StageID       string `json:"stage_id"`
	WorkflowID    string `json:"workflow_id"`
	OrderIndex    int    `json:"order_index"`
	AgentInstance string `json:"agent_instance_id,omitempty"`
	CapabilityTag string `json:"agent_capability_tag,omitempty"`
	TaskPrompt    string `json:"task_prompt"`
	ReportPrompt  string `json:"report_prompt"`
	TimeoutMS     int    `json:"timeout_ms,omitempty"`
}

// Run is one workflow execution instance.
type Run struct {
	RunID             string     `json:"run_id"`
	WorkflowID        string     `json:"workflow_id"`
	Status            string     `json:"status"`
	CurrentStageIndex int        `json:"current_stage_index"`
	StartedAt         time.Time  `json:"started_at"`
	FinishedAt        *time.Time `json:"finished_at,omitempty"`
	FailureReason     string     `json:"failure_reason,omitempty"`
}

// RunStage is one executed stage record of a run.
type RunStage struct {
	RunID      string     `json:"run_id"`
	StageID    string     `json:"stage_id"`
	TaskID     string     `json:"task_id,omitempty"`
	Status     string     `json:"status"`
	Report     string     `json:"report,omitempty"`
	RawOutput  string     `json:"raw_output,omitempty"`
	StartedAt  *time.Time `json:"started_at,omitempty"`
	FinishedAt *time.Time `json:"finished_at,omitempty"`
}

// Schema DDL for the four tables (doc section 3.2).
const schemaSQL = `
CREATE TABLE IF NOT EXISTS workflows (
  workflow_id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  description TEXT,
  status TEXT NOT NULL,
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL
);
CREATE TABLE IF NOT EXISTS workflow_stages (
  stage_id TEXT PRIMARY KEY,
  workflow_id TEXT NOT NULL,
  order_index INTEGER NOT NULL,
  agent_instance_id TEXT,
  agent_capability_tag TEXT,
  task_prompt TEXT NOT NULL,
  report_prompt TEXT NOT NULL,
  timeout_ms INTEGER
);
CREATE INDEX IF NOT EXISTS idx_stages_wf ON workflow_stages(workflow_id, order_index);
CREATE TABLE IF NOT EXISTS workflow_runs (
  run_id TEXT PRIMARY KEY,
  workflow_id TEXT NOT NULL,
  status TEXT NOT NULL,
  current_stage_index INTEGER NOT NULL,
  started_at INTEGER NOT NULL,
  finished_at INTEGER,
  failure_reason TEXT
);
CREATE INDEX IF NOT EXISTS idx_runs_wf ON workflow_runs(workflow_id, started_at DESC);
CREATE TABLE IF NOT EXISTS workflow_run_stages (
  run_id TEXT NOT NULL,
  stage_id TEXT NOT NULL,
  task_id TEXT,
  status TEXT NOT NULL,
  report TEXT,
  raw_output TEXT,
  started_at INTEGER,
  finished_at INTEGER,
  PRIMARY KEY (run_id, stage_id)
);
CREATE INDEX IF NOT EXISTS idx_runstages_task ON workflow_run_stages(task_id);
CREATE INDEX IF NOT EXISTS idx_runstages_run ON workflow_run_stages(run_id);
`

// HandoffFormat builds the strict input_context of doc chapter 4.1.
func HandoffFormat(prevReport, nextPrompt string) string {
	return "上一阶段进度报告：\n" + prevReport + "\n\n本阶段任务提示词：\n" + nextPrompt
}
