// Package intake holds the post-scan candidate list and its confirm/ignore
// states. Scanning happens once at startup and on explicit rescan (Phase-2
// 4.5: no background filesystem watching).
package intake

import (
	"errors"
	"sort"
	"sync"

	"agent-control-plane/harness-sdk/detect"
)

// Candidate states.
const (
	Pending   = "pending"
	Confirmed = "confirmed"
	Ignored   = "ignored"
	Failed    = "failed"
)

// Candidate is a detected CLI with its intake state.
type Candidate struct {
	detect.Candidate
	State string `json:"state"`
	Error string `json:"error,omitempty"`
}

// Manager stores candidates by signature.
type Manager struct {
	mu    sync.Mutex
	m     map[string]*Candidate
	order []string
}

// New creates an empty Manager.
func New() *Manager { return &Manager{m: map[string]*Candidate{}} }

// Scan rebuilds the list; ignored and confirmed entries keep their state.
func (m *Manager) Scan(table *detect.Table) []Candidate {
	found := detect.Scan(table)
	m.mu.Lock()
	defer m.mu.Unlock()
	next := map[string]*Candidate{}
	for _, c := range found {
		rec := &Candidate{Candidate: c, State: Pending}
		if old, ok := m.m[c.Signature]; ok && (old.State == Ignored || old.State == Confirmed) {
			rec.State = old.State
		}
		next[c.Signature] = rec
	}
	m.m = next
	m.order = m.order[:0]
	for sig := range next {
		m.order = append(m.order, sig)
	}
	sort.Strings(m.order)
	out := make([]Candidate, 0, len(m.order))
	for _, sig := range m.order {
		if c, ok := next[sig]; ok {
			out = append(out, *c)
		}
	}
	return out
}

// List returns candidates ordered by signature.
func (m *Manager) List() []Candidate {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]Candidate, 0, len(m.order))
	for _, sig := range m.order {
		if c, ok := m.m[sig]; ok {
			out = append(out, *c)
		}
	}
	return out
}

// Get returns one candidate.
func (m *Manager) Get(signature string) (Candidate, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	c, ok := m.m[signature]
	if !ok {
		return Candidate{}, false
	}
	return *c, true
}

// SetState updates a candidate's state (and error text).
func (m *Manager) SetState(signature, state, errText string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	c, ok := m.m[signature]
	if !ok {
		return errors.New("intake: unknown candidate " + signature)
	}
	c.State = state
	c.Error = errText
	return nil
}
