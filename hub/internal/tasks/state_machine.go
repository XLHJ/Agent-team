package tasks

import (
	"errors"
	"sort"
	"sync"
	"time"
)

// State values.
type State string

const (
	Created         State = "created"
	Dispatched      State = "dispatched"
	Running         State = "running"
	WaitingApproval State = "waiting_approval"
	Completed       State = "completed"
	Failed          State = "failed"
	Canceled        State = "canceled"
)

// Task is a task record.
type Task struct {
	TaskID       string    `json:"task_id"`
	InstanceID   string    `json:"instance_id"`
	State        State     `json:"state"`
	Prompt       string    `json:"prompt"`
	TimeoutMS    int       `json:"timeout_ms"`
	RawOutput    string    `json:"raw_output,omitempty"`
	Report       string    `json:"report,omitempty"`
	Artifacts    []string  `json:"artifacts,omitempty"`
	ErrorCode    string    `json:"error_code,omitempty"`
	ErrorMessage string    `json:"error_message,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// Errors.
var (
	ErrNotFound = errors.New("task not found")
	ErrIllegal  = errors.New("illegal transition")
)

// transitions is the legal transition table.
var transitions = map[State][]State{
	Dispatched:      {Running, WaitingApproval, Completed, Failed, Canceled},
	Running:         {WaitingApproval, Completed, Failed, Canceled},
	WaitingApproval: {Running, Failed, Canceled},
}

// Store is the in-memory task store.
type Store struct {
	mu         sync.RWMutex
	m          map[string]*Task
	onTerminal func(string)
}

// New creates a Store.
func New() *Store {
	return &Store{m: map[string]*Task{}}
}

func (s *Store) move(id string, to State) (*Task, error) {
	t, ok := s.m[id]
	if !ok {
		return nil, ErrNotFound
	}
	for _, legal := range transitions[t.State] {
		if legal == to {
			t.State = to
			t.UpdatedAt = time.Now()
			return t, nil
		}
	}
	return nil, ErrIllegal
}

// Dispatch creates a task in dispatched state.
func (s *Store) Dispatch(id, instanceID, prompt string, timeoutMS int) *Task {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	t := &Task{TaskID: id, InstanceID: instanceID, State: Dispatched, Prompt: prompt, TimeoutMS: timeoutMS, CreatedAt: now, UpdatedAt: now}
	s.m[id] = t
	return t
}

// Progress moves dispatched/waiting_approval to running.
func (s *Store) Progress(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	t, ok := s.m[id]
	if !ok {
		return ErrNotFound
	}
	if t.State == Dispatched || t.State == WaitingApproval {
		t.State = Running
		t.UpdatedAt = time.Now()
		return nil
	}
	if t.State == Running {
		t.UpdatedAt = time.Now()
		return nil
	}
	return ErrNotFound
}

// WaitingApproval moves to waiting_approval.
func (s *Store) WaitingApproval(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.move(id, WaitingApproval)
	return err
}

// Complete stores terminal success data.
func (s *Store) Complete(id, raw, report string, artifacts []string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	t, err := s.move(id, Completed)
	if err != nil {
		return err
	}
	t.RawOutput, t.Report, t.Artifacts = raw, report, artifacts
	return nil
}

// Fail stores terminal failure data.
func (s *Store) Fail(id, code, msg, partial string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	t, err := s.move(id, Failed)
	if err != nil {
		return err
	}
	t.ErrorCode, t.ErrorMessage, t.RawOutput = code, msg, partial
	return nil
}

// Cancel moves to canceled.
func (s *Store) Cancel(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.move(id, Canceled)
	return err
}

// Get returns a copy of a task.
func (s *Store) Get(id string) (Task, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	t, ok := s.m[id]
	if !ok {
		return Task{}, false
	}
	return *t, true
}

// Snapshot returns all tasks sorted by creation time.
func (s *Store) Snapshot() []Task {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Task, 0, len(s.m))
	for _, t := range s.m {
		out = append(out, *t)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.Before(out[j].CreatedAt) })
	return out
}

// List returns tasks for an instance created at/after since (zero = all).
func (s *Store) List(instanceID string, since time.Time) []Task {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Task, 0, len(s.m))
	for _, t := range s.m {
		if instanceID != "" && t.InstanceID != instanceID {
			continue
		}
		if !since.IsZero() && t.CreatedAt.Before(since) {
			continue
		}
		out = append(out, *t)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.Before(out[j].CreatedAt) })
	return out
}

// OnTerminal installs the terminal-state hook used to clear exemptions.
func (s *Store) OnTerminal(fn func(taskID string)) {
	s.mu.Lock()
	s.onTerminal = fn
	s.mu.Unlock()
}

func (s *Store) terminal(id string) {
	if s.onTerminal != nil {
		s.onTerminal(id)
	}
}
