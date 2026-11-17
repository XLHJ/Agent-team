// Package approvalsv2 is the Phase-4 approval engine: lifecycle state
// machine, timeout fail-closed, per-task exemptions, SQLite persistence.
package approvalsv2

import (
	"database/sql"
	"encoding/json"
	"log"
	"sync"
	"time"

	"agent-control-plane/hub/internal/store"
)

// Status names persisted in SQLite (doc section 2.1).
const (
	StPending  = "pending"
	StApproved = "resolved_approved"
	StDenied   = "resolved_denied"
	StModified = "resolved_modified"
	StTimeout  = "timeout_denied"
)

// Timeout policy (doc section 3.1): default 5min, clamp to [30s, 30min].
const (
	DefaultTimeout = 5 * time.Minute
	MinTimeout     = 30 * time.Second
	MaxTimeout     = 30 * time.Minute
)

// ExemptKey identifies one exemption: (task, instance, operation).
type ExemptKey struct {
	TaskID        string
	InstanceID    string
	OperationType string
}

// Engine runs the Phase-4 approval lifecycle.
type Engine struct {
	mu      sync.Mutex
	st      *store.DB
	exempts map[ExemptKey]bool
	timers  map[string]*time.Timer
	log     *log.Logger
	emu     sync.Mutex
	events  []Event
	subs    []func(Event)
	onAuto  func(*store.ApprovalRow)
}

// Event mirrors one lifecycle change; Phase-3 panel push subscribes here.
type Event struct {
	Type     string          `json:"event_type"`
	TS       time.Time       `json:"ts"`
	Approval json.RawMessage `json:"approval,omitempty"`
}

// New creates the engine.
func New(st *store.DB, lg *log.Logger) *Engine {
	return &Engine{
		st:      st,
		exempts: map[ExemptKey]bool{},
		timers:  map[string]*time.Timer{},
		log:     lg,
	}
}

// SetAutoResolve installs the callback routing timeout decisions to plugins.
func (e *Engine) SetAutoResolve(fn func(*store.ApprovalRow)) {
	e.onAuto = fn
}

// Subscribe registers a listener for lifecycle events.
func (e *Engine) Subscribe(fn func(Event)) {
	e.emu.Lock()
	e.subs = append(e.subs, fn)
	e.emu.Unlock()
}

func (e *Engine) emit(typ string, a *store.ApprovalRow) {
	raw, _ := json.Marshal(a)
	ev := Event{Type: typ, TS: time.Now(), Approval: raw}
	e.emu.Lock()
	subs := append([]func(Event){}, e.subs...)
	e.emu.Unlock()
	for _, fn := range subs {
		func() {
			defer func() { _ = recover() }()
			fn(ev)
		}()
	}
}

// audit writes one audit_log row keyed by approval id.
func (e *Engine) audit(typ, approvalID string) {
	payload, _ := json.Marshal(map[string]string{"approval_id": approvalID})
	_ = e.st.Audit(typ, string(payload))
}

// Request persists a new approval and arms its fail-closed timer (doc 2.2).
// applyExempt=true with a matching exemption and risk != high short-circuits
// to auto-approve with reason "exempted"; the row is still persisted so the
// resolved history shows it (doc 4.2).
func (e *Engine) Request(a store.ApprovalRow, applyExempt bool) (*store.ApprovalRow, bool, error) {
	if a.TimeoutMS <= 0 {
		a.TimeoutMS = DefaultTimeout.Milliseconds()
	} else if a.TimeoutMS < MinTimeout.Milliseconds() {
		e.log.Printf("warn: approval %s timeout_ms=%d below floor, clamped",
			a.ApprovalID, a.TimeoutMS)
		a.TimeoutMS = MinTimeout.Milliseconds()
	} else if a.TimeoutMS > MaxTimeout.Milliseconds() {
		e.log.Printf("warn: approval %s timeout_ms=%d above ceiling, clamped",
			a.ApprovalID, a.TimeoutMS)
		a.TimeoutMS = MaxTimeout.Milliseconds()
	}
	a.Status = StPending
	a.CreatedAt = time.Now()
	key := ExemptKey{a.TaskID, a.InstanceID, a.OperationType}
	e.mu.Lock()
	exempt := applyExempt && a.RiskLevel != "high" && e.exempts[key]
	e.mu.Unlock()
	if exempt {
		a.Status = StApproved
		a.Decision = sql.NullString{String: "approve", Valid: true}
		a.Reason = sql.NullString{String: "exempted", Valid: true}
		now := time.Now()
		a.ResolvedAt = &now
		if err := e.st.SaveApproval(&a); err != nil {
			return nil, false, err
		}
		e.audit("exemption.applied", a.ApprovalID)
		e.emit("approval.exempted", &a)
		return &a, true, nil
	}
	if err := e.st.SaveApproval(&a); err != nil {
		return nil, false, err
	}
	e.startTimer(a.ApprovalID, a.TimeoutMS)
	e.audit("approval.requested", a.ApprovalID)
	e.emit("approval.requested", &a)
	return &a, false, nil
}

// startTimer arms the timer fixed at enqueue time (doc 3.1: not extendable).
func (e *Engine) startTimer(id string, timeoutMS int64) {
	t := time.AfterFunc(time.Duration(timeoutMS)*time.Millisecond, func() {
		e.timeoutFire(id)
	})
	e.mu.Lock()
	e.timers[id] = t
	e.mu.Unlock()
}

// timeoutFire runs the TIMEOUT_DENIED path (doc 2.4): fail closed, route
// deny + reason "timeout" to the plugin via the onAuto callback.
func (e *Engine) timeoutFire(id string) {
	e.mu.Lock()
	delete(e.timers, id)
	e.mu.Unlock()
	row, err := e.st.GetApproval(id)
	if err != nil || row.Status != StPending {
		return
	}
	if err := e.st.ResolveApproval(id, StTimeout, "deny", "", "timeout"); err != nil {
		e.log.Printf("timeout resolve %s: %v", id, err)
		return
	}
	e.audit("approval.timeout", id)
	updated, _ := e.st.GetApproval(id)
	e.emit("approval.resolved", updated)
	if e.onAuto != nil {
		e.onAuto(updated)
	}
}

func (e *Engine) stopTimer(id string) {
	e.mu.Lock()
	if t, ok := e.timers[id]; ok {
		t.Stop()
		delete(e.timers, id)
	}
	e.mu.Unlock()
}

// Resolve applies a human decision (doc 2.3). DB-first commit: on store
// error nothing changes in memory and the caller surfaces an error.
func (e *Engine) Resolve(id, decision, modifiedJSON, reason string, exempt bool) (*store.ApprovalRow, error) {
	var modified string
	if decision == "modify" {
		if err := validateJSONObject(modifiedJSON, 64*1024); err != nil {
			return nil, err
		}
		modified = modifiedJSON
	}
	row, err := e.st.GetApproval(id)
	if err != nil {
		return nil, ErrNotFound
	}
	if row.Status != StPending {
		return nil, ErrAlreadyDone
	}
	switch decision {
	case "approve":
		if exempt && row.RiskLevel != "high" {
			key := ExemptKey{row.TaskID, row.InstanceID, row.OperationType}
			e.mu.Lock()
			e.exempts[key] = true
			e.mu.Unlock()
			e.audit("exemption.granted", id)
		}
		err = e.st.ResolveApproval(id, StApproved, "approve", "", reason)
	case "deny":
		err = e.st.ResolveApproval(id, StDenied, "deny", "", reason)
	case "modify":
		err = e.st.ResolveApproval(id, StModified, "modify", modified, reason)
	default:
		return nil, ErrBadDecision
	}
	if err != nil {
		if err == store.ErrNotFound {
			return nil, ErrNotFound
		}
		return nil, ErrAlreadyDone
	}
	e.stopTimer(id)
	out, _ := e.st.GetApproval(id)
	e.audit("approval.resolved", id)
	e.emit("approval.resolved", out)
	return out, nil
}

// Recover reloads PENDING rows at boot and re-arms timers with the remaining
// window measured from created_at (doc 6.2); expired rows resolve instantly.
func (e *Engine) Recover() error {
	rows, err := e.st.ListApprovals(StPending, "", "", 0)
	if err != nil {
		return err
	}
	now := time.Now()
	for _, r := range rows {
		remain := time.Duration(r.TimeoutMS)*time.Millisecond - now.Sub(r.CreatedAt)
		if remain <= 0 {
			e.timeoutFire(r.ApprovalID)
			continue
		}
		e.startTimer(r.ApprovalID, remain.Milliseconds())
	}
	e.log.Printf("approval recovery: %d pending row(s) re-armed", len(rows))
	return nil
}

// ClearExempts drops all exemptions for one finished task (doc 4.3).
func (e *Engine) ClearExempts(taskID string) int {
	e.mu.Lock()
	n := 0
	for k := range e.exempts {
		if k.TaskID == taskID {
			delete(e.exempts, k)
			n++
		}
	}
	e.mu.Unlock()
	if n > 0 {
		e.audit("exemption.cleared", taskID)
	}
	return n
}

// Cleanup runs the retention sweep (doc 6.3) and audits the deleted count.
func (e *Engine) Cleanup(retention time.Duration) {
	n, err := e.st.CleanupOld(retention)
	if err != nil {
		e.log.Printf("cleanup: %v", err)
		return
	}
	if n > 0 {
		payload, _ := json.Marshal(map[string]int64{"deleted": n})
		_ = e.st.Audit("approval.cleanup", string(payload))
	}
}

// Close stops all timers (graceful shutdown).
func (e *Engine) Close() {
	e.mu.Lock()
	for id, t := range e.timers {
		t.Stop()
		delete(e.timers, id)
	}
	e.mu.Unlock()
}

// List returns approvals; status "resolved" maps to every terminal state.
func (e *Engine) List(status, instanceID, taskID string, limit int) ([]*store.ApprovalRow, error) {
	q := status
	switch status {
	case "resolved":
		rows, err := e.st.ListApprovals("", instanceID, taskID, limit)
		if err != nil {
			return nil, err
		}
		out := rows[:0]
		for _, r := range rows {
			if r.Status != StPending {
				out = append(out, r)
			}
		}
		return out, nil
	case "pending", "":
		if status == "pending" {
			q = StPending
		}
		return e.st.ListApprovals(q, instanceID, taskID, limit)
	}
	return e.st.ListApprovals(q, instanceID, taskID, limit)
}
