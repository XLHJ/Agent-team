// Package rpc is the JSON-RPC 2.0 client shared by the CLI harness and every
// plugin form. It owns framing, request/response correlation and the inbound
// push channel; lifecycle/reconnect lives one layer up.
package rpc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"nhooyr.io/websocket"

	"agent-control-plane/harness-sdk/wire"
)

// Validator checks a params object against a method schema (optional).
type Validator interface {
	Validate(method string, params json.RawMessage) error
}

// ErrClosed is returned when the connection is gone.
var ErrClosed = errors.New("rpc: connection closed")

// Client is a connected JSON-RPC peer.
type Client struct {
	c       *websocket.Conn
	val     Validator
	push    chan wire.Frame
	done    chan struct{}
	once    sync.Once
	seq     atomic.Uint64
	mu      sync.Mutex
	pending map[string]chan wire.Frame
}

// Dial opens a connection. token is appended as ?token= when non-empty.
func Dial(ctx context.Context, url, token string, val Validator) (*Client, error) {
	if token != "" {
		url = url + "?token=" + token
	}
	c, _, err := websocket.Dial(ctx, url, &websocket.DialOptions{})
	if err != nil {
		return nil, fmt.Errorf("rpc: dial %s: %w", url, err)
	}
	cl := &Client{c: c, val: val, push: make(chan wire.Frame, 64), done: make(chan struct{}), pending: map[string]chan wire.Frame{}}
	go cl.readLoop()
	return cl, nil
}

// Push returns the inbound channel (downstream methods).
func (c *Client) Push() <-chan wire.Frame { return c.push }

// Done is closed when the connection dies.
func (c *Client) Done() <-chan struct{} { return c.done }

func (c *Client) readLoop() {
	defer c.shutdown()
	ctx := context.Background()
	for {
		_, raw, err := c.c.Read(ctx)
		if err != nil {
			return
		}
		var f wire.Frame
		if err := json.Unmarshal(raw, &f); err != nil {
			continue
		}
		if len(f.ID) > 0 && f.Method == "" {
			c.mu.Lock()
			ch, ok := c.pending[string(f.ID)]
			delete(c.pending, string(f.ID))
			c.mu.Unlock()
			if ok {
				select {
				case ch <- f:
				default:
				}
			}
			continue
		}
		if c.val != nil && f.Method != "" && len(f.Params) > 0 {
			if err := c.val.Validate(f.Method, raw); err != nil {
				continue
			}
		}
		select {
		case c.push <- f:
		case <-c.done:
			return
		}
	}
}

func (c *Client) shutdown() {
	c.once.Do(func() {
		close(c.done)
		_ = c.c.Close(websocket.StatusNormalClosure, "")
		c.mu.Lock()
		for k, ch := range c.pending {
			close(ch)
			delete(c.pending, k)
		}
		c.mu.Unlock()
	})
}

// Close closes the connection.
func (c *Client) Close() { c.shutdown() }

// Call sends a request and waits for its response.
func (c *Client) Call(ctx context.Context, method string, params any) (json.RawMessage, *wire.ErrorObject, error) {
	id := c.seq.Add(1)
	raw, err := wire.Request(id, method, params)
	if err != nil {
		return nil, nil, err
	}
	if c.val != nil {
		if err := c.val.Validate(method, raw); err != nil {
			return nil, nil, fmt.Errorf("rpc: outbound %s invalid: %w", method, err)
		}
	}
	ch := make(chan wire.Frame, 1)
	key := wireID(id)
	c.mu.Lock()
	c.pending[key] = ch
	c.mu.Unlock()

	wctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	if err := c.c.Write(wctx, websocket.MessageText, raw); err != nil {
		c.mu.Lock()
		delete(c.pending, key)
		c.mu.Unlock()
		return nil, nil, err
	}
	select {
	case f, ok := <-ch:
		if !ok {
			return nil, nil, ErrClosed
		}
		if f.Error != nil {
			return nil, f.Error, nil
		}
		return f.Result, nil, nil
	case <-c.done:
		return nil, nil, ErrClosed
	case <-ctx.Done():
		return nil, nil, ctx.Err()
	}
}

// Notify sends an upstream event. Protocol v0.1 schemas require every frame
// to carry an id, so events go out as requests whose reply is discarded.
func (c *Client) Notify(ctx context.Context, method string, params any) error {
	id := c.seq.Add(1)
	raw, err := wire.Request(id, method, params)
	if err != nil {
		return err
	}
	if c.val != nil {
		if err := c.val.Validate(method, raw); err != nil {
			return fmt.Errorf("rpc: outbound %s invalid: %w", method, err)
		}
	}
	ch := make(chan wire.Frame, 1)
	key := wireID(id)
	c.mu.Lock()
	c.pending[key] = ch
	c.mu.Unlock()
	time.AfterFunc(30*time.Second, func() {
		c.mu.Lock()
		if cur, ok := c.pending[key]; ok && cur == ch {
			delete(c.pending, key)
		}
		c.mu.Unlock()
	})
	wctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	return c.c.Write(wctx, websocket.MessageText, raw)
}

// Reply answers a request pushed by the Hub.
func (c *Client) Reply(ctx context.Context, id json.RawMessage, result any) error {
	raw, err := wire.Response(id, result)
	if err != nil {
		return err
	}
	return c.c.Write(ctx, websocket.MessageText, raw)
}

func wireID(v uint64) string {
	b, _ := json.Marshal(v)
	return string(b)
}
