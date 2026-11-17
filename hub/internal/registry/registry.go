package registry

import (
	"errors"
	"sort"
	"strings"
	"sync"
	"time"
)

// Status values surfaced to snapshots.
const (
	StatusReady       = "READY"
	StatusUnreachable = "UNREACHABLE"
)

// Instance is a registered agent record.
type Instance struct {
	AgentType       string    `json:"agent_type"`
	InstanceID      string    `json:"instance_id"`
	ProtocolVersion string    `json:"protocol_version"`
	Capabilities    []string  `json:"capabilities"`
	ApprovalMode    string    `json:"approval_mode"` // native_forward / tool_hook
	Status          string    `json:"status"`        // agent status: idle/busy/waiting_approval/error
	ConnStatus      string    `json:"conn_status"`   // READY/UNREACHABLE
	CurrentTaskID   string    `json:"current_task_id,omitempty"`
	LastHeartbeatAt time.Time `json:"last_heartbeat_at"`
	// Source is the intake path prefix (cli:<name> / plugin:<name>).
	Source string `json:"source"`
}

// SourceOf derives the display source from an instance id prefix.
func SourceOf(id string) string {
	parts := strings.SplitN(id, ":", 3)
	if len(parts) >= 2 {
		return parts[0] + ":" + parts[1]
	}
	return parts[0]
}

// ErrConflict is returned on duplicate instance_id.
var ErrConflict = errors.New("instance_id conflict")

// Registry is the in-memory instance registry.
type Registry struct {
	mu sync.RWMutex
	m  map[string]*Instance
}

// New creates a Registry.
func New() *Registry {
	return &Registry{m: map[string]*Instance{}}
}

// Register inserts an instance; ErrConflict when the id exists.
func (r *Registry) Register(in Instance) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.m[in.InstanceID]; ok {
		return ErrConflict
	}
	in.ConnStatus = StatusReady
	in.LastHeartbeatAt = time.Now()
	in.Source = SourceOf(in.InstanceID)
	r.m[in.InstanceID] = &in
	return nil
}

// Replace rebinds an existing (disconnected) record to a new connection.
func (r *Registry) Replace(in Instance) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.m[in.InstanceID]; !ok {
		return false
	}
	in.ConnStatus = StatusReady
	in.LastHeartbeatAt = time.Now()
	in.Source = SourceOf(in.InstanceID)
	r.m[in.InstanceID] = &in
	return true
}

// Heartbeat refreshes last-heartbeat for a known instance.
func (r *Registry) Heartbeat(id string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	in, ok := r.m[id]
	if !ok {
		return false
	}
	in.LastHeartbeatAt = time.Now()
	in.ConnStatus = StatusReady
	return true
}

// SetStatus updates the agent status (and optional task id).
func (r *Registry) SetStatus(id, status, taskID string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	in, ok := r.m[id]
	if !ok {
		return false
	}
	in.Status = status
	in.CurrentTaskID = taskID
	return true
}

// SetConnStatus sets READY/UNREACHABLE.
func (r *Registry) SetConnStatus(id, cs string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	in, ok := r.m[id]
	if !ok {
		return false
	}
	in.ConnStatus = cs
	return true
}

// HasCapability reports whether id declared cap.
func (r *Registry) HasCapability(id, cap string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	in, ok := r.m[id]
	if !ok {
		return false
	}
	for _, c := range in.Capabilities {
		if c == cap {
			return true
		}
	}
	return false
}

// Has reports whether id is registered.
func (r *Registry) Has(id string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, ok := r.m[id]
	return ok
}

// Get returns a copy of the instance.
func (r *Registry) Get(id string) (Instance, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	in, ok := r.m[id]
	if !ok {
		return Instance{}, false
	}
	return *in, true
}

// Remove deletes an instance.
func (r *Registry) Remove(id string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.m, id)
}

// Snapshot returns all instances sorted by instance_id.
func (r *Registry) Snapshot() []Instance {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]Instance, 0, len(r.m))
	for _, in := range r.m {
		out = append(out, *in)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].InstanceID < out[j].InstanceID })
	return out
}
