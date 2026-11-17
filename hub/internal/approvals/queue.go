package approvals

import (
	"errors"
	"sort"
	"sync"
	"time"
)

// Status values.
type Status string

const (
	Pending  Status = "pending"
	Resolved Status = "resolved"
)

// Approval is an approval object.
type Approval struct {
	ApprovalID     string         `json:"approval_id"`
	TaskID         string         `json:"task_id"`
	InstanceID     string         `json:"instance_id"`
	OperationType  string         `json:"operation_type"`
	RiskLevel      string         `json:"risk_level"`
	ContextSummary string         `json:"context_summary"`
	TimeoutMS      int            `json:"timeout_ms"`
	Mode           string         `json:"mode"`
	Status         Status         `json:"status"`
	Decision       string         `json:"decision,omitempty"`
	ModifiedParams map[string]any `json:"modified_params,omitempty"`
	Reason         string         `json:"reason,omitempty"`
	DeliveryFailed bool           `json:"delivery_failed,omitempty"`
	CreatedAt      time.Time      `json:"created_at"`
	ResolvedAt     *time.Time     `json:"resolved_at,omitempty"`
}

// Errors.
var (
	ErrExists      = errors.New("approval_id exists")
	ErrNotFound    = errors.New("approval not found")
	ErrAlreadyDone = errors.New("approval already resolved")
)

// Queue is the in-memory approval queue.
type Queue struct {
	mu sync.Mutex
	m  map[string]*Approval
}

// New creates a Queue.
func New() *Queue {
	return &Queue{m: map[string]*Approval{}}
}

// Enqueue inserts a pending approval; ErrExists on duplicate id.
func (q *Queue) Enqueue(a Approval) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	if _, ok := q.m[a.ApprovalID]; ok {
		return ErrExists
	}
	a.Status = Pending
	a.CreatedAt = time.Now()
	q.m[a.ApprovalID] = &a
	return nil
}

// Resolve atomically decides a pending approval.
func (q *Queue) Resolve(id, decision string, modified map[string]any, reason string) (*Approval, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	a, ok := q.m[id]
	if !ok {
		return nil, ErrNotFound
	}
	if a.Status == Resolved {
		return nil, ErrAlreadyDone
	}
	now := time.Now()
	a.Status = Resolved
	a.Decision = decision
	a.ModifiedParams = modified
	a.Reason = reason
	a.ResolvedAt = &now
	return a, nil
}

// MarkDeliveryFailed flags an undeliverable decision.
func (q *Queue) MarkDeliveryFailed(id string) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if a, ok := q.m[id]; ok {
		a.DeliveryFailed = true
	}
}

// Get returns a copy.
func (q *Queue) Get(id string) (Approval, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()
	a, ok := q.m[id]
	if !ok {
		return Approval{}, false
	}
	return *a, true
}

// Snapshot returns all approvals sorted by created_at.
func (q *Queue) Snapshot() []Approval {
	q.mu.Lock()
	defer q.mu.Unlock()
	out := make([]Approval, 0, len(q.m))
	for _, a := range q.m {
		out = append(out, *a)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.Before(out[j].CreatedAt) })
	return out
}
