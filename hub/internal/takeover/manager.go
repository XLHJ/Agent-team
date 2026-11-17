// Package takeover spawns and supervises harness-runner processes. The runner
// registers itself with instance_id cli:<signature>:<pid>; the Hub only owns
// the process tree (kill-on-close Job Object on Windows).
package takeover

import (
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// Proc is one spawned runner.
type Proc struct {
	PID       int       `json:"pid"`
	Signature string    `json:"signature"`
	Path      string    `json:"path"`
	Started   time.Time `json:"started_at"`
	cmd       *exec.Cmd
	job       jobHandle
}

// Manager supervises runner processes.
type Manager struct {
	mu    sync.Mutex
	m     map[int]*Proc
	bySig map[string]int
	bin   string
	url   string
	tok   string
	lg    *log.Logger
	stop  bool
}

// New builds a Manager. bin may be empty until configured.
func New(bin, hubURL, token string, lg *log.Logger) *Manager {
	return &Manager{m: map[int]*Proc{}, bySig: map[string]int{}, bin: bin, url: hubURL, tok: token, lg: lg}
}

// SetBinary updates the runner binary path (flag may arrive later).
func (m *Manager) SetBinary(bin string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.bin = bin
}

// Spawn starts a runner for a detected CLI.
func (m *Manager) Spawn(signature, path string, methods []string) (*Proc, error) {
	m.mu.Lock()
	bin := m.bin
	m.mu.Unlock()
	if bin == "" {
		return nil, fmt.Errorf("takeover: harness binary not configured")
	}
	cmd := exec.Command(bin,
		"--signature", signature,
		"--path", path,
		"--methods", strings.Join(methods, ","),
	)
	cmd.Env = append(os.Environ(),
		"HUB_WS_URL="+m.url,
		"BOOTSTRAP_TOKEN="+m.tok,
		"ACP_AGENT_NAME="+signature,
	)
	cmd.Stdin = os.Stdin
	stdin, _ := cmd.StdinPipe()
	_ = stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	prepare(cmd)
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	p := &Proc{PID: cmd.Process.Pid, Signature: signature, Path: path, Started: time.Now(), cmd: cmd}
	assign(cmd, &p.job)
	m.mu.Lock()
	m.m[p.PID] = p
	m.bySig[signature] = p.PID
	m.mu.Unlock()
	go m.reap(p)
	m.lg.Printf("takeover: spawned %s pid=%d path=%s", signature, p.PID, path)
	return p, nil
}

func (m *Manager) reap(p *Proc) {
	_ = p.cmd.Wait()
	closeJob(&p.job)
	m.mu.Lock()
	delete(m.m, p.PID)
	delete(m.bySig, p.Signature)
	m.mu.Unlock()
	m.lg.Printf("takeover: pid=%d exited", p.PID)
}

func (m *Manager) procFor(instanceID string) (*Proc, error) {
	parts := strings.SplitN(instanceID, ":", 3)
	if len(parts) < 2 {
		return nil, fmt.Errorf("takeover: bad instance id %q", instanceID)
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	pid, ok := m.bySig[parts[1]]
	if !ok {
		return nil, fmt.Errorf("takeover: %s not tracked", instanceID)
	}
	p, ok := m.m[pid]
	if !ok {
		return nil, fmt.Errorf("takeover: pid %d not tracked", pid)
	}
	return p, nil
}

func (m *Manager) Stop(instanceID string) error {
	p, err := m.procFor(instanceID)
	if err != nil {
		return err
	}
	return terminate(p)
}

func (m *Manager) Connected(instanceID string) bool {
	_, err := m.procFor(instanceID)
	return err == nil
}

func (m *Manager) Snapshot() []Proc {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]Proc, 0, len(m.m))
	for _, p := range m.m {
		out = append(out, *p)
	}
	return out
}

func (m *Manager) CloseAll() {
	m.mu.Lock()
	all := make([]*Proc, 0, len(m.m))
	for _, p := range m.m {
		all = append(all, p)
	}
	m.mu.Unlock()
	for _, p := range all {
		_ = terminate(p)
	}
}

func (m *Manager) Ctrl(instanceID, line string) error {
	p, err := m.procFor(instanceID)
	if err != nil {
		return err
	}
	w, wok := any(p.cmd.Stdin).(io.WriteCloser)
	if !wok || w == nil {
		return fmt.Errorf("takeover: %s not controllable", instanceID)
	}
	_, err = w.Write([]byte(line + "\n"))
	return err
}
