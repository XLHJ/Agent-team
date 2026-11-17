// Package lifecycle owns the plugin connection state machine shared by the
// CLI harness and both plugin forms. Phase-2 decision: no automatic reconnect —
// a drop surfaces to the user, who retries once per click.
package lifecycle

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"sync"
	"time"

	"agent-control-plane/harness-sdk/rpc"
	"agent-control-plane/harness-sdk/wire"
)

// State values (master plan 4.2 state machine).
type State string

const (
	Disconnected State = "DISCONNECTED"
	Connecting   State = "CONNECTING"
	Registering  State = "REGISTERING"
	Ready        State = "READY"
	Draining     State = "DRAINING"
)

// Config describes the instance to register.
type Config struct {
	HubURL       string // ws://127.0.0.1:<port>/ws
	HTTPBase     string // http://127.0.0.1:<port>
	Token        string
	InstanceID   string
	AgentType    string
	AgentVersion string
	Capabilities []string
	Metadata     map[string]any
	Heartbeat    time.Duration // default 30s
}

// Hooks receive state and recovery callbacks.
type Hooks struct {
	OnState      func(State)
	OnDisconnect func(reason string)
	// OnTaskLost fires for tasks still dispatched/running when a reconnect
	// succeeds; the caller reports task.failed TASK_LOST_ON_RECONNECT or
	// resumes reporting, per Phase-2 7.1.
	OnTaskLost func(taskID string)
}

// Manager is one connection lifecycle.
type Manager struct {
	cfg   Config
	hooks Hooks
	val   rpc.Validator

	mu    sync.Mutex
	state State
	cl    *rpc.Client
	since time.Time
	stop  chan struct{}
}

// New builds a Manager in DISCONNECTED.
func New(cfg Config, hooks Hooks, val rpc.Validator) *Manager {
	if cfg.Heartbeat == 0 {
		cfg.Heartbeat = 30 * time.Second
	}
	return &Manager{cfg: cfg, hooks: hooks, val: val, state: Disconnected, stop: make(chan struct{})}
}

// State returns the current state.
func (m *Manager) State() State {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.state
}

func (m *Manager) set(s State) {
	m.mu.Lock()
	m.state = s
	m.mu.Unlock()
	if m.hooks.OnState != nil {
		m.hooks.OnState(s)
	}
}

// Client returns the live client or nil.
func (m *Manager) Client() *rpc.Client {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.cl
}

// Connect performs one connect + register attempt.
func (m *Manager) Connect(ctx context.Context) error {
	if m.State() != Disconnected {
		return fmt.Errorf("lifecycle: connect from state %s", m.State())
	}
	return m.connect(ctx)
}

// Retry is the manual retry entry point: exactly one attempt.
func (m *Manager) Retry(ctx context.Context) error {
	if s := m.State(); s != Disconnected {
		return fmt.Errorf("lifecycle: retry from state %s", s)
	}
	return m.connect(ctx)
}

func (m *Manager) connect(ctx context.Context) error {
	m.set(Connecting)
	cl, err := rpc.Dial(ctx, m.cfg.HubURL, m.cfg.Token, m.val)
	if err != nil {
		m.set(Disconnected)
		return err
	}
	m.set(Registering)
	params := map[string]any{
		"agent_type":       m.cfg.AgentType,
		"instance_id":      m.cfg.InstanceID,
		"protocol_version": wire.ProtocolVersion,
		"capabilities":     m.cfg.Capabilities,
	}
	if m.cfg.AgentVersion != "" {
		params["agent_version"] = m.cfg.AgentVersion
	}
	if len(m.cfg.Metadata) > 0 {
		params["metadata"] = m.cfg.Metadata
	}
	if _, rerr, err := cl.Call(ctx, wire.MRegister, params); err != nil {
		cl.Close()
		m.set(Disconnected)
		return err
	} else if rerr != nil {
		cl.Close()
		m.set(Disconnected)
		return fmt.Errorf("register rejected: %s", rerr.BusinessCode())
	}

	// Bind the client before recovery so OnTaskLost hooks can report over it.
	m.mu.Lock()
	m.cl = cl
	m.mu.Unlock()

	// Recover tasks the Hub still holds for this instance. A restarted process
	// carries no prior timestamp, so every non-terminal task of the instance
	// counts as lost from the previous life.
	m.recover(ctx)
	m.since = time.Now()
	m.set(Ready)
	go m.heartbeat(ctx, cl)
	go m.watch(cl)
	return nil
}

func (m *Manager) watch(cl *rpc.Client) {
	select {
	case <-cl.Done():
	case <-m.stop:
		return
	}
	m.mu.Lock()
	if m.cl == cl {
		m.cl = nil
	}
	m.mu.Unlock()
	m.set(Disconnected)
	if m.hooks.OnDisconnect != nil {
		m.hooks.OnDisconnect("connection lost")
	}
}

func (m *Manager) heartbeat(ctx context.Context, cl *rpc.Client) {
	t := time.NewTicker(m.cfg.Heartbeat)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-cl.Done():
			return
		case <-m.stop:
			return
		case <-t.C:
			_ = cl.Notify(ctx, wire.MHeartbeat, map[string]any{
				"instance_id": m.cfg.InstanceID,
				"timestamp":   time.Now().UTC().Format(time.RFC3339),
			})
		}
	}
}

type taskRow struct {
	TaskID string `json:"task_id"`
	State  string `json:"state"`
}

func (m *Manager) recover(ctx context.Context) {
	q := url.Values{}
	q.Set("instance_id", m.cfg.InstanceID)
	if !m.since.IsZero() {
		q.Set("since", m.since.UTC().Format(time.RFC3339Nano))
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, m.cfg.HTTPBase+"/tasks?"+q.Encode(), nil)
	if err != nil {
		return
	}
	if m.cfg.Token != "" {
		req.Header.Set("X-Hub-Token", m.cfg.Token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return
	}
	var rows []taskRow
	if err := json.NewDecoder(resp.Body).Decode(&rows); err != nil {
		return
	}
	for _, r := range rows {
		if r.State == "dispatched" || r.State == "running" || r.State == "waiting_approval" {
			if m.hooks.OnTaskLost != nil {
				m.hooks.OnTaskLost(r.TaskID)
			}
		}
	}
}

// Drain stops taking tasks (Phase-2 4.2 DRAINING).
func (m *Manager) Drain() { m.set(Draining) }

// Close tears the connection down.
func (m *Manager) Close() {
	select {
	case <-m.stop:
	default:
		close(m.stop)
	}
	if cl := m.Client(); cl != nil {
		cl.Close()
	}
	m.set(Disconnected)
}

// ErrNotReady guards callers that need READY.
var ErrNotReady = errors.New("lifecycle: not ready")

// Ready reports whether the manager may send events.
func (m *Manager) Ready() bool { return m.State() == Ready }
