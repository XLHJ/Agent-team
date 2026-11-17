package server

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/url"
	"time"

	"agent-control-plane/hub/internal/cdp"
)

// Kitesurf bridge (https://github.com/cloudflare/kitesurf) - Cloudflare's
// Workers-native browser engine speaking CDP. The user runs kitesurf
// themselves (wrangler dev or deployed to Workers) and configures its browser
// WebSocket endpoint from the panel; the hub stores it in app_settings and
// every agent registered in the panel can then render pages through
//
//	GET  /kitesurf/v1/status                configured? kitesurf healthy?
//	POST /kitesurf/v1/render {url, screenshot}
//
// Kitesurf is Apache-2.0 (Cloudflare, Inc.); we neither vendor nor modify its
// source, we only speak its documented CDP/HTTP interface.

const kitesurfEndpointKey = "kitesurf.ws_endpoint"

func (s *Server) kitesurfEndpoint() string {
	v, _ := s.Store.SettingGet(kitesurfEndpointKey)
	return v
}

func (s *Server) kitesurfStatus(w http.ResponseWriter, _ *http.Request) {
	ep := s.kitesurfEndpoint()
	out := map[string]any{"configured": ep != "", "endpoint": ep}
	if ep != "" {
		client := &http.Client{Timeout: 5 * time.Second}
		// health probe uses scheme://host only
		u, err := url.Parse(trimWSToHTTP(ep))
		var healthURL string
		if err == nil && u.Host != "" {
			u2 := *u
			u2.Path = ""
			u2.RawQuery = ""
			healthURL = u2.String()
		}
		resp, herr := client.Get(healthURL + "/health")
		if herr != nil {
			out["reachable"] = false
			out["error"] = herr.Error()
		} else {
			defer resp.Body.Close()
			out["reachable"] = resp.StatusCode == http.StatusOK
		}
	}
	writeJSON(w, 200, out)
}

// trimWSToHTTP converts ws(s)://host/... to http(s)://host for /health probes.
func trimWSToHTTP(ep string) string {
	for _, pair := range [][2]string{{"ws://", "http://"}, {"wss://", "https://"}} {
		if len(ep) >= len(pair[0]) && ep[:len(pair[0])] == pair[0] {
			return pair[1] + ep[len(pair[0]):]
		}
	}
	return ep
}

type kitesurfRenderReq struct {
	URL        string `json:"url"`
	Screenshot bool   `json:"screenshot"`
	TimeoutSec int    `json:"timeout_sec"`
}

func (s *Server) kitesurfRender(w http.ResponseWriter, r *http.Request) {
	ep := s.kitesurfEndpoint()
	if ep == "" {
		writeJSON(w, 409, map[string]string{"error": "KITESURF_NOT_CONFIGURED"})
		return
	}
	var q kitesurfRenderReq
	if err := json.NewDecoder(r.Body).Decode(&q); err != nil || q.URL == "" {
		writeJSON(w, 400, map[string]string{"error": "url is required"})
		return
	}
	limit := q.TimeoutSec
	if limit <= 0 {
		limit = 30
	}
	ctx, cancel := contextWithTimeout(r.Context(), time.Duration(limit)*time.Second)
	defer cancel()
	snap, err := cdp.Render(ctx, ep, q.URL, q.Screenshot)
	if err != nil {
		writeJSON(w, 502, map[string]string{"error": "RENDER_FAILED", "detail": err.Error()})
		return
	}
	out := map[string]any{
		"url": snap.URL, "title": snap.Title, "text": snap.Text, "ok": true,
	}
	if snap.ScreenshotPNG != nil {
		out["screenshot_base64"] = base64.StdEncoding.EncodeToString(snap.ScreenshotPNG)
	}
	writeJSON(w, 200, out)
}

func contextWithTimeout(parent context.Context, d time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(parent, d)
}
