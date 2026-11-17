package panel

import (
	"context"
	"encoding/json"
	"time"

	"agent-control-plane/hub/internal/workflows"
)

func (h *Handler) workflowsList(params json.RawMessage) (any, *rpcError) {
	list, err := h.d.Wf.ListWorkflows()
	if err != nil {
		return nil, bizErr("DB_ERROR", err.Error())
	}
	return map[string]any{"workflows": list}, nil
}

func (h *Handler) workflowGet(params json.RawMessage) (any, *rpcError) {
	var p struct {
		WorkflowID string `json:"workflow_id"`
	}
	_ = json.Unmarshal(params, &p)
	wf, stages, err := h.d.Wf.GetWorkflow(p.WorkflowID)
	if err != nil {
		return nil, bizErr("WORKFLOW_NOT_FOUND", "no such workflow")
	}
	return map[string]any{"workflow": wf, "stages": stages}, nil
}

func (h *Handler) workflowSave(params json.RawMessage) (any, *rpcError) {
	var p struct {
		Workflow workflows.Workflow `json:"workflow"`
		Stages   []workflows.Stage  `json:"stages"`
	}
	_ = json.Unmarshal(params, &p)
	if p.Workflow.Name == "" {
		return nil, bizErr("BAD_WORKFLOW", "name required")
	}
	if len(p.Stages) == 0 {
		return nil, bizErr("BAD_WORKFLOW", "at least one stage required")
	}
	for i := range p.Stages {
		p.Stages[i].OrderIndex = i
		p.Stages[i].WorkflowID = p.Workflow.WorkflowID
		if p.Stages[i].StageID == "" {
			p.Stages[i].StageID = newID()
		}
		if p.Stages[i].TaskPrompt == "" {
			return nil, bizErr("BAD_STAGE", "task_prompt required")
		}
	}
	if err := h.d.Wf.SaveWorkflow(p.Workflow, p.Stages); err != nil {
		return nil, bizErr("DB_ERROR", err.Error())
	}
	return map[string]any{"workflow_id": p.Workflow.WorkflowID}, nil
}

func (h *Handler) workflowStart(ctx context.Context, params json.RawMessage) (any, *rpcError) {
	var p struct {
		WorkflowID string `json:"workflow_id"`
	}
	_ = json.Unmarshal(params, &p)
	runID := newID()
	if _, err := h.d.Wf.StartRun(ctx, p.WorkflowID, runID); err != nil {
		return nil, bizErr("START_FAILED", err.Error())
	}
	return map[string]any{"run_id": runID}, nil
}

func (h *Handler) workflowRuns(params json.RawMessage) (any, *rpcError) {
	var p struct {
		WorkflowID string `json:"workflow_id"`
		Limit      int    `json:"limit"`
	}
	_ = json.Unmarshal(params, &p)
	runs, err := h.d.Wf.ListRuns(p.WorkflowID, p.Limit)
	if err != nil {
		return nil, bizErr("DB_ERROR", err.Error())
	}
	return map[string]any{"runs": runs}, nil
}

func (h *Handler) workflowRunGet(params json.RawMessage) (any, *rpcError) {
	var p struct {
		RunID string `json:"run_id"`
	}
	_ = json.Unmarshal(params, &p)
	run, stages, err := h.d.Wf.GetRun(p.RunID)
	if err != nil {
		return nil, bizErr("RUN_NOT_FOUND", "no such run")
	}
	return map[string]any{"run": run, "stages": stages}, nil
}

func (h *Handler) workflowCancel(params json.RawMessage) (any, *rpcError) {
	var p struct {
		RunID string `json:"run_id"`
	}
	_ = json.Unmarshal(params, &p)
	if err := h.d.Wf.CancelRun(p.RunID); err != nil {
		return nil, bizErr("CANCEL_FAILED", err.Error())
	}
	return map[string]any{}, nil
}

func (h *Handler) workflowDelete(params json.RawMessage) (any, *rpcError) {
	var p struct {
		WorkflowID string `json:"workflow_id"`
	}
	_ = json.Unmarshal(params, &p)
	if err := h.d.Wf.DeleteWorkflow(p.WorkflowID); err != nil {
		return nil, bizErr("DELETE_FAILED", err.Error())
	}
	return map[string]any{}, nil
}

func (h *Handler) workflowExport(params json.RawMessage) (any, *rpcError) {
	var p struct {
		WorkflowID string `json:"workflow_id"`
	}
	_ = json.Unmarshal(params, &p)
	wf, stages, err := h.d.Wf.GetWorkflow(p.WorkflowID)
	if err != nil {
		return nil, bizErr("WORKFLOW_NOT_FOUND", "no such workflow")
	}
	file := workflows.ChainFile{
		Version:     workflows.ChainFileVersion,
		Name:        wf.Name,
		Description: wf.Description,
	}
	for _, st := range stages {
		file.Stages = append(file.Stages, workflows.ChainFileStage{
			AgentInstance: st.AgentInstance,
			CapabilityTag: st.CapabilityTag,
			TaskPrompt:    st.TaskPrompt,
			ReportPrompt:  st.ReportPrompt,
			TimeoutMS:     st.TimeoutMS,
		})
	}
	return map[string]any{"chain": file}, nil
}

func (h *Handler) workflowImport(params json.RawMessage) (any, *rpcError) {
	var p struct {
		Chain workflows.ChainFile `json:"chain"`
	}
	_ = json.Unmarshal(params, &p)
	if p.Chain.Version != workflows.ChainFileVersion {
		return nil, bizErr("BAD_CHAIN", "unsupported chain version")
	}
	if p.Chain.Name == "" {
		return nil, bizErr("BAD_CHAIN", "name required")
	}
	if len(p.Chain.Stages) == 0 {
		return nil, bizErr("BAD_CHAIN", "at least one stage required")
	}
	wf := workflows.Workflow{
		WorkflowID:  newID(),
		Name:        p.Chain.Name,
		Description: p.Chain.Description,
		Status:      workflows.WSDraft,
	}
	stages := make([]workflows.Stage, 0, len(p.Chain.Stages))
	for i, cs := range p.Chain.Stages {
		if cs.TaskPrompt == "" {
			return nil, bizErr("BAD_CHAIN", "stage task_prompt required")
		}
		stages = append(stages, workflows.Stage{
			StageID:       newID(),
			WorkflowID:    wf.WorkflowID,
			OrderIndex:    i,
			AgentInstance: cs.AgentInstance,
			CapabilityTag: cs.CapabilityTag,
			TaskPrompt:    cs.TaskPrompt,
			ReportPrompt:  cs.ReportPrompt,
			TimeoutMS:     cs.TimeoutMS,
		})
	}
	if err := h.d.Wf.SaveWorkflow(wf, stages); err != nil {
		return nil, bizErr("DB_ERROR", err.Error())
	}
	return map[string]any{"workflow_id": wf.WorkflowID}, nil
}

// newID returns a time-ordered unique id.
func newID() string {
	return "id-" + time.Now().UTC().Format("20060102T150405.000000000")
}
