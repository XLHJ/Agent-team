// Package cdp implements a minimal Chrome DevTools Protocol client, just
// enough to drive kitesurf (https://github.com/cloudflare/kitesurf), the
// Cloudflare Workers browser engine, over its browser WebSocket endpoint
// (ws://host:port/devtools/browser): open a target, navigate, pull page text
// and take a screenshot. No puppeteer required.
package cdp

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"sync"

	"nhooyr.io/websocket"
)

// Snapshot is the result of rendering one URL.
type Snapshot struct {
	URL           string `json:"url"`
	Title         string `json:"title"`
	Text          string `json:"text"`
	ScreenshotPNG []byte `json:"-"` // raw PNG bytes when requested
}

type conn struct {
	c       *websocket.Conn
	wmu     sync.Mutex
	mu      sync.Mutex
	nextID  int
	pending map[int64]chan map[string]any
	events  chan map[string]any
}

// Dial connects to a browser-level CDP WebSocket endpoint.
func Dial(ctx context.Context, wsURL string) (*conn, error) {
	c, _, err := websocket.Dial(ctx, wsURL, nil)
	if err != nil {
		return nil, fmt.Errorf("cdp dial %s: %w", wsURL, err)
	}
	w := &conn{c: c, pending: map[int64]chan map[string]any{}, events: make(chan map[string]any, 32)}
	go w.readLoop()
	return w, nil
}

func (w *conn) readLoop() {
	ctx := context.Background()
	for {
		_, raw, err := w.c.Read(ctx)
		if err != nil {
			w.mu.Lock()
			for _, ch := range w.pending {
				close(ch)
			}
			w.mu.Unlock()
			close(w.events)
			return
		}
		var m map[string]any
		if json.Unmarshal(raw, &m) != nil {
			continue
		}
		if id, ok := num(m["id"]); ok {
			w.mu.Lock()
			ch, alive := w.pending[int64(id)]
			delete(w.pending, int64(id))
			w.mu.Unlock()
			if alive {
				ch <- m
				close(ch)
			}
			continue
		}
		select {
		case w.events <- m:
		default:
		}
	}
}

func num(v any) (int64, bool) {
	switch t := v.(type) {
	case float64:
		return int64(t), true
	case int64:
		return t, true
	}
	return 0, false
}

// Call sends one CDP command and waits for its response. When sessionID is
// non-empty it is attached (flatten mode).
func (w *conn) Call(ctx context.Context, sessionID, method string, params map[string]any) (map[string]any, error) {
	w.mu.Lock()
	w.nextID++
	id := int64(w.nextID)
	ch := make(chan map[string]any, 1)
	w.pending[id] = ch
	w.mu.Unlock()
	msg := map[string]any{"id": id, "method": method}
	if params != nil {
		msg["params"] = params
	}
	if sessionID != "" {
		msg["sessionId"] = sessionID
	}
	b, err := json.Marshal(msg)
	if err != nil {
		return nil, err
	}
	w.wmu.Lock()
	err = w.c.Write(ctx, websocket.MessageText, b)
	w.wmu.Unlock()
	if err != nil {
		return nil, err
	}
	select {
	case resp, ok := <-ch:
		if !ok {
			return nil, fmt.Errorf("cdp connection closed during %s", method)
		}
		if e, has := resp["error"]; has {
			return nil, fmt.Errorf("cdp %s: %v", method, e)
		}
		return resp, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// waitEvent blocks until the given CDP event arrives or ctx expires.
func (w *conn) waitEvent(ctx context.Context, method string) bool {
	for {
		select {
		case ev, ok := <-w.events:
			if !ok {
				return false
			}
			if m, _ := ev["method"].(string); m == method {
				return true
			}
		case <-ctx.Done():
			return false
		}
	}
}

func (w *conn) Close() error { return w.c.Close(websocket.StatusNormalClosure, "") }

// Render navigates to url on a fresh target and returns text + optional
// screenshot. Total runtime is bounded by ctx.
func Render(ctx context.Context, wsURL, url string, wantScreenshot bool) (*Snapshot, error) {
	w, err := Dial(ctx, wsURL)
	if err != nil {
		return nil, err
	}
	defer w.Close()
	r, err := w.Call(ctx, "", "Target.createTarget", map[string]any{"url": "about:blank"})
	if err != nil {
		return nil, err
	}
	targetID, _ := r["result"].(map[string]any)["targetId"].(string)
	r, err = w.Call(ctx, "", "Target.attachToTarget",
		map[string]any{"targetId": targetID, "flatten": true})
	if err != nil {
		return nil, err
	}
	sessionID, _ := r["result"].(map[string]any)["sessionId"].(string)
	if _, err = w.Call(ctx, sessionID, "Page.enable", nil); err != nil {
		return nil, err
	}
	if _, err = w.Call(ctx, sessionID, "Page.navigate", map[string]any{"url": url}); err != nil {
		return nil, err
	}
	// kitesurf renders without Chrome; loadEventFired signals page ready.
	if !w.waitEvent(ctx, "Page.loadEventFired") {
		return nil, fmt.Errorf("timeout waiting for %s to load", url)
	}
	snap := &Snapshot{URL: url}
	if rr, err := w.Call(ctx, sessionID, "Runtime.evaluate", map[string]any{
		"expression":    "JSON.stringify({title: document.title, text: document.body.innerText})",
		"returnByValue": true,
	}); err == nil {
		res, _ := rr["result"].(map[string]any)["result"].(map[string]any)
		if v, _ := res["value"].(string); v != "" {
			var out struct {
				Title string `json:"title"`
				Text  string `json:"text"`
			}
			if json.Unmarshal([]byte(v), &out) == nil {
				snap.Title = out.Title
				snap.Text = out.Text
			}
		}
	}
	if wantScreenshot {
		if rr, err := w.Call(ctx, sessionID, "Page.captureScreenshot", map[string]any{"format": "png"}); err == nil {
			if b64, _ := rr["result"].(map[string]any)["data"].(string); b64 != "" {
				if raw, err := base64.StdEncoding.DecodeString(b64); err == nil {
					snap.ScreenshotPNG = raw
				}
			}
		}
	}
	_, _ = w.Call(ctx, "", "Target.closeTarget", map[string]any{"targetId": targetID})
	return snap, nil
}
