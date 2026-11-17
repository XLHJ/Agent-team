package cdp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"nhooyr.io/websocket"
)

// mockCDP speaks just enough CDP for Render(): createTarget, attachToTarget,
// Page.enable/navigate/screenshot, Runtime.evaluate, closeTarget, plus one
// Page.loadEventFired event after navigate.
func mockCDP(t *testing.T) *httptest.Server {
	t.Helper()
	var mu sync.Mutex
	var nextID int
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer c.Close(websocket.StatusNormalClosure, "")
		send := func(v map[string]any) {
			b, _ := json.Marshal(v)
			_ = c.Write(r.Context(), websocket.MessageText, b)
		}
		for {
			_, raw, err := c.Read(r.Context())
			if err != nil {
				return
			}
			var m map[string]any
			if json.Unmarshal(raw, &m) != nil {
				continue
			}
			id, _ := m["id"].(float64)
			method, _ := m["method"].(string)
			mu.Lock()
			nextID++
			evID := nextID
			mu.Unlock()
			switch method {
			case "Target.createTarget":
				send(map[string]any{"id": id, "result": map[string]any{"targetId": "t-1"}})
			case "Target.attachToTarget":
				send(map[string]any{"id": id, "result": map[string]any{"sessionId": "s-1"}})
			case "Page.navigate":
				send(map[string]any{"id": id, "result": map[string]any{}})
				go func() {
					time.Sleep(30 * time.Millisecond)
					send(map[string]any{"method": "Page.loadEventFired", "params": map[string]any{}})
				}()
				_ = evID
			case "Runtime.evaluate":
				send(map[string]any{"id": id, "result": map[string]any{"result": map[string]any{
					"value": `{"title":"示例页面","text":"你好 kitesurf"}`,
				}}})
			case "Page.captureScreenshot":
				send(map[string]any{"id": id, "result": map[string]any{
					"data": "aGVsbG8tcG5n", // "hello-png"
				}})
			case "Page.enable", "Target.closeTarget":
				send(map[string]any{"id": id, "result": map[string]any{}})
			default:
				send(map[string]any{"id": id, "error": map[string]any{"message": "unknown " + method}})
			}
		}
	}))
}

func TestRender(t *testing.T) {
	srv := mockCDP(t)
	defer srv.Close()
	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	snap, err := Render(ctx, wsURL, "https://example.com/page", true)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if snap.Title != "示例页面" || snap.Text != "你好 kitesurf" {
		t.Fatalf("bad snapshot: %+v", snap)
	}
	if string(snap.ScreenshotPNG) != "hello-png" {
		t.Fatalf("bad screenshot: %q", snap.ScreenshotPNG)
	}
}

func TestRenderTimeout(t *testing.T) {
	// A server that never sends loadEventFired: Render must fail with the
	// context deadline rather than hang.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer c.Close(websocket.StatusNormalClosure, "")
		for {
			if _, _, err := c.Read(r.Context()); err != nil {
				return
			}
		}
	}))
	defer srv.Close()
	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http")
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	if _, err := Render(ctx, wsURL, "https://example.com", false); err == nil {
		t.Fatal("want timeout error, got nil")
	}
}
