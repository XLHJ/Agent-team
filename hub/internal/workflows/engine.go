package workflows

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"
)

// Dispatcher sends one task to an agent instance.
type Dispatcher func(ctx context.Context, instanceID, taskID, prompt, inputContext, reportPrompt string, timeoutMS int) error

// Engine drives sequential workflow runs.
type Engine struct {
	store    *Store
	disp     Dispatcher
	log      *log.Logger
	mu       sync.Mutex
	eligible Eligible
	pick     Selector
}

// NewEngine builds the executor.
func NewEngine(st *Store, disp Dispatcher, lg *log.Logger) *Engine {
	return &Engine{store: st, disp: disp, log: lg}
}

// Eligible reports whether an instance may serve a stage (report.generate).
type Eligible func(instanceID string) bool

// Selector picks an instance for a stage (doc 3.3 capability matching).
type Selector func(tag string) (string, bool)

// Start creates a run and dispatches stage 0 (doc 5.2).
func (e *Engine) Start(ctx context.Context, workflowID, runID string,
	eligible Eligible, pick Selector) (*Run, error) {
	_, stages, err := e.store.GetWorkflow(workflowID)
	if err != nil {
		return nil, err
	}
	if len(stages) == 0 {
		return nil, ErrNoStages
	}
	run := Run{
		RunID: runID, WorkflowID: workflowID, Status: RSRunning,
		CurrentStageIndex: 0, StartedAt: time.Now(),
	}
	if err := e.store.CreateRun(run, stages); err != nil {
		return nil, err
	}
	e.audit("workflow.started", workflowID, runID)
	if err := e.dispatchStage(ctx, &run, stages, 0, "", eligible, pick); err != nil {
		e.failRun(&run, stages[0].StageID, err.Error())
		return &run, err
	}
	return &run, nil
}

// dispatchStage binds an agent and dispatches one stage task.
func (e *Engine) dispatchStage(ctx context.Context, run *Run, stages []Stage,
	idx int, prevReport string, eligible Eligible, pick Selector) error {
	st := stages[idx]
	instance := st.AgentInstance
	if instance == "" {
		if pick == nil {
			return ErrNoEligibleAgent
		}
		picked, ok := pick(st.CapabilityTag)
		if !ok {
			return ErrNoEligibleAgent
		}
		instance = picked
	}
	if eligible != nil && !eligible(instance) {
		return ErrNoEligibleAgent
	}
	type Selector func(tag string) (string, bool)

	taskID := fmt.Sprintf("wf-%s-%d", run.RunID, idx)
	input := ""
	if idx > 0 {
		input = HandoffFormat(prevReport, st.TaskPrompt)
	}
	if err := e.disp(ctx, instance, taskID, st.TaskPrompt, input, st.ReportPrompt, st.TimeoutMS); err != nil {
		return err
	}
	if err := e.store.SetRunStageTask(run.RunID, st.StageID, taskID); err != nil {
		return err
	}
	if err := e.store.UpdateRunStatus(run.RunID, RSRunning, idx, ""); err != nil {
		return err
	}
	return nil
}

func (e *Engine) failRun(run *Run, stageID, reason string) {
	run.Status = RSFailed
	run.FailureReason = reason
	_ = e.store.UpdateRunStage(run.RunID, stageID, SSFailed, "", "", "")
	_ = e.store.UpdateRunStatus(run.RunID, RSFailed, run.CurrentStageIndex, reason)
	e.audit("workflow.failed", run.WorkflowID, run.RunID)
}

// StageCompleted advances the chain when a stage task completes (doc 4.2).
// A missing report fails the whole run with REPORT_MISSING.
func (e *Engine) StageCompleted(ctx context.Context, runID string, stages []Stage,
	report, raw string, eligible Eligible, pick Selector) error {
	run, runStages, err := e.store.GetRun(runID)
	if err != nil {
		return err
	}
	if run.Status != RSRunning {
		return nil
	}
	idx := run.CurrentStageIndex
	if idx < 0 || idx >= len(stages) {
		return nil
	}
	stageID := stages[idx].StageID
	if report == "" {
		_ = e.store.UpdateRunStage(runID, stageID, SSFailed, "", "", raw)
		e.failRun(run, stageID, "REPORT_MISSING")
		return nil
	}
	_ = runStages
	_ = e.store.UpdateRunStage(runID, stageID, SSCompleted, "", report, raw)
	if idx == len(stages)-1 {
		run.Status = RSCompleted
		_ = e.store.UpdateRunStatus(runID, RSCompleted, idx, "")
		e.audit("workflow.completed", run.WorkflowID, runID)
		return nil
	}
	run.CurrentStageIndex = idx + 1
	if err := e.dispatchStage(ctx, run, stages, idx+1, report, eligible, pick); err != nil {
		e.failRun(run, stages[idx+1].StageID, err.Error())
		return err
	}
	return nil
}

// StageFailed terminates the chain on a stage task failure (doc 5.3).
func (e *Engine) StageFailed(runID, code, msg string) {
	run, _, err := e.store.GetRun(runID)
	if err != nil || run.Status != RSRunning {
		return
	}
	e.failRun(run, "", "STAGE_FAILED: "+code+" "+msg)
}

// Cancel marks a run canceled (doc 5.2 step 7).
func (e *Engine) Cancel(runID string) error {
	run, _, err := e.store.GetRun(runID)
	if err != nil {
		return err
	}
	if run.Status != RSRunning {
		return ErrRunNotRunning
	}
	if err := e.store.UpdateRunStatus(runID, RSCanceled, run.CurrentStageIndex, ""); err != nil {
		return err
	}
	e.audit("workflow.canceled", run.WorkflowID, runID)
	return nil
}

// Recover marks every still-running run failed on boot (doc 7.2).
func (e *Engine) Recover() {
	runs, err := e.store.RunningRuns()
	if err != nil {
		return
	}
	for _, r := range runs {
		_ = e.store.UpdateRunStatus(r.RunID, RSFailed, r.CurrentStageIndex, "HUB_RESTART_INTERRUPTED")
		e.audit("workflow.failed", r.WorkflowID, r.RunID)
	}
	if len(runs) > 0 {
		e.log.Printf("workflow recovery: %d interrupted run(s) marked failed", len(runs))
	}
}

func (e *Engine) audit(typ, workflowID, runID string) {
	payload, _ := json.Marshal(map[string]string{
		"workflow_id": workflowID, "run_id": runID,
	})
	_ = e.store.Audit(typ, string(payload))
}

// OnTaskCompleted advances the chain that owns taskID (doc 5.2 step 5).
func (e *Engine) OnTaskCompleted(ctx context.Context, taskID, report, raw string) {
	run, stages, err := e.findByTask(taskID)
	if err != nil {
		return
	}
	_ = e.StageCompleted(ctx, run.RunID, stages, report, raw, e.eligible, e.pick)
}

// OnTaskFailed terminates the chain that owns taskID (doc 5.2 step 6).
func (e *Engine) OnTaskFailed(taskID, code, msg string) {
	run, _, err := e.findByTask(taskID)
	if err != nil {
		return
	}
	e.StageFailed(run.RunID, code, msg)
}

// findByTask maps a dispatched task id back to its run and stage set.
func (e *Engine) findByTask(taskID string) (*Run, []Stage, error) {
	runs, err := e.store.AllRunningRuns()
	if err != nil {
		return nil, nil, err
	}
	for _, r := range runs {
		rs, err := e.store.RunStages(r.RunID)
		if err != nil {
			continue
		}
		for _, s := range rs {
			if s.TaskID == taskID {
				_, stages, err := e.store.GetWorkflow(r.WorkflowID)
				if err != nil {
					return nil, nil, err
				}
				return &r, stages, nil
			}
		}
	}
	return nil, nil, ErrRunNotFound
}
