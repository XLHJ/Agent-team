package ws

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"nhooyr.io/websocket"
)

// Conn wraps a websocket connection with guarded writes.
type Conn struct {
	C          *websocket.Conn
	mu         sync.Mutex
	InstanceID string
	LastSeen   time.Time
}

// Send writes one JSON text frame.
func (c *Conn) Send(ctx context.Context, v any) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.C.Write(ctx, websocket.MessageText, b)
}

// Close closes the underlying connection.
func (c *Conn) Close(code websocket.StatusCode, reason string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	_ = c.C.Close(code, reason)
}

// Pool maps instance_id to connection.
type Pool struct {
	mu sync.RWMutex
	m  map[string]*Conn
}

// NewPool creates a Pool.
func NewPool() *Pool {
	return &Pool{m: map[string]*Conn{}}
}

// Add inserts a connection for id (overwrites).
func (p *Pool) Add(id string, c *Conn) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.m[id] = c
}

// Get returns the connection for id.
func (p *Pool) Get(id string) (*Conn, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	c, ok := p.m[id]
	return c, ok
}

// RemoveIf removes id when it still maps to c.
func (p *Pool) RemoveIf(id string, c *Conn) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if cur, ok := p.m[id]; ok && cur == c {
		delete(p.m, id)
	}
}

// SendTo sends v to id; false when no live connection.
func (p *Pool) SendTo(ctx context.Context, id string, v any) bool {
	c, ok := p.Get(id)
	if !ok {
		return false
	}
	return c.Send(ctx, v) == nil
}

// WriteRaw writes one raw text frame.
func (c *Conn) WriteRaw(ctx context.Context, b []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.C.Write(ctx, websocket.MessageText, b)
}
