// Package shared is the common plugin body: registration, heartbeat, push
// dispatch and approval plumbing. Form packages add identity and adapters.
package shared

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"agent-control-plane/harness-sdk/lifecycle"
	"agent-control-plane/harness-sdk/validate"
	"agent-control-plane/harness-sdk/wire"
)

// Config describes the plugin instance.
type Config struct {
	HubURL       string
	HTTPBase     string
	Token        string
	InstanceID   string
	AgentType    string
	Version      string
	Form         string // embedded | sidecar
	Capabilities []string
	ApprovalMode string
	Reason       string // FormChoiceReason, written to metadata
}

// Task is one dispatch.
type Task struct {
	TaskID       string `json:"task_id"`
	Prompt       string `json:"prompt"`
	InputContext string `json:"input_context"`
	ReportPrompt string `json:"report_prompt"`
}

// Approval is one inbound decision.
type Approval struct {
	ApprovalID string          `json:"approval_id"`
	Decision   string          `json:"decision"`
	Modified   json.RawMessage `json:"modified_params"`
}

// Hooks are the adapter callbacks.
type Hooks struct {
	OnTask      func(Task)
	OnCancel    func(taskID string)
	OnInterrupt func()
	OnApproval  func(Approval)
	OnState     func(lifecycle.State)
}

// Plugin is a connected plugin instance.
type Plugin struct {
	cfg Config
	h   Hooks
	lc  *lifecycle.Manager
	mu  sync.Mutex
}

// Start registers and serves until ctx ends.
func Start(ctx context.Context, cfg Config, h Hooks, val *validate.Validator) (*Plugin, error) {
	p := &Plugin{cfg: cfg, h: h}
	p.lc = lifecycle.New(lifecycle.Config{
		HubURL: cfg.HubURL, HTTPBase: cfg.HTTPBase, Token: cfg.Token,
		InstanceID: cfg.InstanceID, AgentType: cfg.AgentType, AgentVersion: cfg.Version,
		Capabilities: cfg.Capabilities,
		Metadata:     map[string]any{"form": cfg.Form, "approval_mode": cfg.ApprovalMode, "reason": cfg.Reason},
	}, lifecycle.Hooks{OnState: h.OnState, OnDisconnect: func(string) {}}, val)
	if err := p.lc.Connect(ctx); err != nil {
		return p, err
	}
	go p.loop(ctx)
	return p, nil
}

func (p *Plugin) loop(ctx context.Context) {
	for {
		cl := p.lc.Client()
		if cl == nil {
			select {
			case <-ctx.Done():
				return
			case <-time.After(100 * time.Millisecond):
			}
			continue
		}
		select {
		case <-ctx.Done():
			return
		case <-cl.Done():
		case f := <-cl.Push():
			p.onPush(f)
		}
	}
}

func (p *Plugin) onPush(f wire.Frame) {
	switch f.Method {
	case wire.MTaskDispatch:
		var t Task
		if json.Unmarshal(f.Params, &t) == nil && p.h.OnTask != nil {
			p.h.OnTask(t)
		}
	case wire.MTaskCancel:
		var t struct {
			TaskID string `json:"task_id"`
		}
		if json.Unmarshal(f.Params, &t) == nil && p.h.OnCancel != nil {
			p.h.OnCancel(t.TaskID)
		}
	case wire.MSessionInterrupt:
		if p.h.OnInterrupt != nil {
			p.h.OnInterrupt()
		}
	case wire.MApprovalRespond:
		var a Approval
		if json.Unmarshal(f.Params, &a) == nil && p.h.OnApproval != nil {
			p.h.OnApproval(a)
		}
	}
}

func (p *Plugin) notify(method string, params any) error {
	cl := p.lc.Client()
	if cl == nil {
		return fmt.Errorf("plugin: offline")
	}
	return cl.Notify(context.Background(), method, params)
}

// Progress reports progress 0-100.
func (p *Plugin) Progress(taskID string, pct int, step string) error {
	return p.notify(wire.MTaskProgress, map[string]any{"task_id": taskID, "progress": pct, "step": step})
}

// Completed reports success with output and optional report.
func (p *Plugin) Completed(taskID, raw, report string) error {
	return p.notify(wire.MTaskCompleted, map[string]any{"task_id": taskID, "raw_output": raw, "report": report})
}

// Failed reports a failure.
func (p *Plugin) Failed(taskID, code, msg string) error {
	return p.notify(wire.MTaskFailed, map[string]any{"task_id": taskID, "error_code": code, "error_message": msg})
}

// UsageCall is one model invocation detail reported at task end.
type UsageCall struct {
	Model        string  `json:"model"`
	Source       string  `json:"source"`
	InputTokens  int64   `json:"input_tokens"`
	OutputTokens int64   `json:"output_tokens"`
	TTFTMs       int64   `json:"ttft_ms"`
	TokensPerS   float64 `json:"tokens_per_s"`
	DurationMs   int64   `json:"duration_ms"`
}

// UsageReport reports the task-level usage summary (call it once when the
// task finishes, aggregating every model invocation made during the task).
func (p *Plugin) UsageReport(taskID string, calls []UsageCall) error {
	return p.notify(wire.MUsageReport, map[string]any{
		"task_id":     taskID,
		"instance_id": p.cfg.InstanceID,
		"calls":       calls,
	})
}

// RequestApproval raises an approval.
func (p *Plugin) RequestApproval(taskID, approvalID, opType, risk, summary string) error {
	return p.notify(wire.MApprovalRequested, map[string]any{
		"approval_id": approvalID, "task_id": taskID, "instance_id": p.cfg.InstanceID,
		"operation_type": opType, "risk_level": risk, "context_summary": summary,
	})
}

// Close disconnects.
func (p *Plugin) Close() { p.lc.Close() }

// Wait blocks until SIGINT/SIGTERM.
func Wait() {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
	<-ch
}
