package takeover

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"sync"
	"time"

	"agent-control-plane/harness-sdk/detect"
)

// proc is a structured-stdio channel (NDJSON in/out).
type proc struct {
	path    string
	args    []string
	tr      Translator
	native  bool
	cmd     *exec.Cmd
	stdin   io.WriteCloser
	events  chan Event
	mu      sync.Mutex
	exitErr error
	kill    func()
	done    chan struct{}
	partial []string
}

func newProc(path string, args []string, tr Translator, native bool) *proc {
	return &proc{path: path, args: args, tr: tr, native: native, events: make(chan Event, 64), done: make(chan struct{})}
}

func (p *proc) Start() error {
	cmd := detect.CommandSource(p.path, p.args...)
	configureGroup(cmd) // process group now, job object after start
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	p.cmd = cmd
	p.stdin = stdin
	p.kill = func() { killTree(cmd) }
	assignToJob(cmd)
	go p.readLines(stdout)
	go p.readLog(stderr)
	go func() {
		err := cmd.Wait()
		p.mu.Lock()
		p.exitErr = err
		p.mu.Unlock()
		close(p.done)
		close(p.events)
	}()
	return nil
}

func (p *proc) readLog(r io.Reader) {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 64*1024), 1024*1024)
	for sc.Scan() {
		if line := sc.Text(); line != "" {
			select {
			case p.events <- Event{Kind: EvLog, Step: line}:
			case <-p.done:
				return
			}
		}
	}
}

func (p *proc) readLines(r io.Reader) {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 64*1024), 8*1024*1024)
	for sc.Scan() {
		ev, ok := p.tr.DecodeLine(sc.Bytes())
		if !ok {
			continue
		}
		if ev.Kind == EvPartial {
			p.partial = append(p.partial, ev.RawOutput)
			continue
		}
		select {
		case p.events <- ev:
		case <-p.done:
			return
		}
	}
}

func (p *proc) SendTask(prompt string) error {
	var payload []byte
	var err error
	if p.native {
		req := map[string]any{
			"jsonrpc": "2.0", "id": 1, "method": "task",
			"params": map[string]any{"prompt": prompt},
		}
		payload, err = json.Marshal(req)
		payload = append(payload, '\n')
	} else {
		payload, err = p.tr.EncodePrompt(prompt)
	}
	if err != nil {
		return err
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.stdin == nil {
		return errors.New("takeover: stdin closed")
	}
	_, err = p.stdin.Write(payload)
	return err
}

func (p *proc) AnswerApproval(id, decision string, modified []byte) error {
	b, err := p.tr.EncodeApprovalResponse(id, decision, modified)
	if err != nil || len(b) == 0 {
		return err
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	_, err = p.stdin.Write(b)
	return err
}

// Cancel asks politely: closing stdin ends most CLI one-shot sessions.
func (p *proc) Cancel() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.stdin != nil {
		return p.stdin.Close()
	}
	return nil
}

// Interrupt waits a short grace period, then kills the process tree.
func (p *proc) Interrupt() error {
	_ = p.Cancel()
	select {
	case <-p.done:
		return nil
	case <-time.After(3 * time.Second):
	}
	if p.kill != nil {
		p.kill()
	}
	return nil
}

func (p *proc) Events() <-chan Event { return p.events }

func (p *proc) ExitErr() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.exitErr
}

func (p *proc) Close() error {
	if p.kill != nil {
		p.kill()
	}
	return nil
}

// Partial joins accumulated partial outputs (codex agent messages).
func (p *proc) Partial() []string { return p.partial }

var _ = fmt.Sprintf

// PartialText joins accumulated partial outputs (codex agent messages).
func (p *proc) PartialText() string {
	p.mu.Lock()
	defer p.mu.Unlock()
	var out string
	for _, s := range p.partial {
		out += s
	}
	return out
}
