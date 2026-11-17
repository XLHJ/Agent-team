package workflows

import (
	"context"
	"log"
)

// Chain is the facade the panel and router talk to.
type Chain struct {
	Store  *Store
	Engine *Engine
}

// NewChain builds the facade over a store and an engine.
func NewChain(st *Store, eng *Engine, lg *log.Logger) *Chain {
	return &Chain{Store: st, Engine: eng}
}

// ListWorkflows returns chain definitions.
func (c *Chain) ListWorkflows() ([]Workflow, error) {
	return c.Store.ListWorkflows()
}

// GetWorkflow loads one chain with its stages.
func (c *Chain) GetWorkflow(id string) (*Workflow, []Stage, error) {
	return c.Store.GetWorkflow(id)
}

// SaveWorkflow stores a chain definition and its stages.
func (c *Chain) SaveWorkflow(w Workflow, stages []Stage) error {
	return c.Store.SaveWorkflow(w, stages)
}

// DeleteWorkflow removes a chain definition.
func (c *Chain) DeleteWorkflow(id string) error {
	return c.Store.DeleteWorkflow(id)
}

// ListRuns returns run history of one chain.
func (c *Chain) ListRuns(id string, limit int) ([]Run, error) {
	return c.Store.ListRuns(id, limit)
}

// GetRun loads one run with its stage records.
func (c *Chain) GetRun(id string) (*Run, []RunStage, error) {
	return c.Store.GetRun(id)
}

// StartRun creates a run and dispatches the first stage.
func (c *Chain) StartRun(ctx context.Context, workflowID, runID string) (*Run, error) {
	return c.Engine.Start(ctx, workflowID, runID, c.Engine.eligible, c.Engine.pick)
}

// CancelRun cancels a running chain.
func (c *Chain) CancelRun(runID string) error {
	return c.Engine.Cancel(runID)
}
