package takeover

import (
	"bufio"
	"io"
	"sync"
	"time"

	"github.com/aymanbagabas/go-pty"

	"agent-control-plane/harness-sdk/detect"
)

// ptyChan runs the CLI inside a PTY (ConPTY on Windows). Observe-only quality:
// screen text is forwarded as log lines, no structured parsing.
type ptyChan struct {
	path   string
	args   []string
	tr     Translator
	p      pty.Pty
	cmd    *pty.Cmd
	events chan Event
	done   chan struct{}
	mu     sync.Mutex
	err    error
}

func newPTY(path string, args []string, tr Translator) *ptyChan {
	return &ptyChan{path: path, args: args, tr: tr, events: make(chan Event, 64), done: make(chan struct{})}
}

func (c *ptyChan) Start() error {
	p, err := pty.New()
	if err != nil {
		return err
	}
	c.p = p
	resolved := detect.CommandSource(c.path, c.args...)
	cmd := p.Command(resolved.Path, resolved.Args[1:]...)
	c.cmd = cmd
	if err := cmd.Start(); err != nil {
		_ = p.Close()
		return err
	}
	_ = p.Resize(200, 50)
	go func() {
		sc := bufio.NewScanner(p)
		sc.Buffer(make([]byte, 64*1024), 1024*1024)
		for sc.Scan() {
			select {
			case c.events <- Event{Kind: EvLog, Step: sc.Text()}:
			case <-c.done:
				return
			}
		}
	}()
	go func() {
		err := cmd.Wait()
		c.mu.Lock()
		c.err = err
		c.mu.Unlock()
		close(c.done)
		close(c.events)
	}()
	return nil
}

func (c *ptyChan) SendTask(prompt string) error {
	_, err := io.WriteString(c.p, prompt+"\r\n")
	return err
}

func (c *ptyChan) AnswerApproval(string, string, []byte) error { return nil }

func (c *ptyChan) Cancel() error {
	_, err := c.p.Write([]byte{0x03})
	return err
}

func (c *ptyChan) Interrupt() error {
	_ = c.Cancel()
	select {
	case <-c.done:
		return nil
	case <-time.After(3 * time.Second):
	}
	if c.cmd != nil && c.cmd.Process != nil {
		_ = c.cmd.Process.Kill()
	}
	return nil
}

func (c *ptyChan) Events() <-chan Event { return c.events }

func (c *ptyChan) ExitErr() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.err
}

func (c *ptyChan) PartialText() string { return "" }

func (c *ptyChan) Close() error {
	if c.p != nil {
		return c.p.Close()
	}
	return nil
}
