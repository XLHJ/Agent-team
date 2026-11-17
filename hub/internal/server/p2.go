package server

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"agent-control-plane/harness-sdk/detect"
)

// guard enforces the bootstrap token on mutating and query endpoints.
func (s *Server) guard(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.Token != "" {
			tok := r.Header.Get("X-Hub-Token")
			if tok == "" {
				tok = r.URL.Query().Get("token")
			}
			if tok != s.Token {
				writeJSON(w, 401, map[string]string{"error": "UNAUTHORIZED"})
				return
			}
		}
		next(w, r)
	}
}

func (s *Server) candidates(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, 200, s.Intake.List())
}

func (s *Server) rescan(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, 200, s.Intake.Scan(s.Sigs))
}

type sigReq struct {
	Signature string `json:"signature"`
}

func (s *Server) confirm(w http.ResponseWriter, r *http.Request) {
	var q sigReq
	if err := json.NewDecoder(r.Body).Decode(&q); err != nil {
		writeJSON(w, 400, map[string]string{"error": "bad json"})
		return
	}
	c, ok := s.Intake.Get(q.Signature)
	if !ok {
		writeJSON(w, 404, map[string]string{"error": "CANDIDATE_NOT_FOUND"})
		return
	}
	if len(c.Methods) == 0 {
		_ = s.Intake.SetState(q.Signature, "failed", "no takeover method")
		writeJSON(w, 422, map[string]string{"error": "NO_METHOD"})
		return
	}
	if _, err := s.Take.Spawn(c.Signature, c.Path, c.Methods); err != nil {
		_ = s.Intake.SetState(q.Signature, "failed", err.Error())
		writeJSON(w, 502, map[string]string{"error": "SPAWN_FAILED", "detail": err.Error()})
		return
	}
	_ = s.Intake.SetState(q.Signature, "confirmed", "")
	writeJSON(w, 200, map[string]string{"signature": q.Signature, "state": "confirmed"})
}

func (s *Server) ignore(w http.ResponseWriter, r *http.Request) {
	var q sigReq
	if err := json.NewDecoder(r.Body).Decode(&q); err != nil {
		writeJSON(w, 400, map[string]string{"error": "bad json"})
		return
	}
	if err := s.Intake.SetState(q.Signature, "ignored", ""); err != nil {
		writeJSON(w, 404, map[string]string{"error": "CANDIDATE_NOT_FOUND"})
		return
	}
	writeJSON(w, 200, map[string]string{"signature": q.Signature, "state": "ignored"})
}

func (s *Server) disconnect(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/agents/")
	id = strings.TrimSuffix(id, "/disconnect")
	if s.Take.Stop(id) != nil {
		writeJSON(w, 404, map[string]string{"error": "PROCESS_NOT_TRACKED"})
		return
	}
	s.Reg.SetConnStatus(id, "UNREACHABLE")
	if c, ok := s.Pool.Get(id); ok {
		c.Close(1000, "disconnected by user")
	}
	writeJSON(w, 200, map[string]string{"instance_id": id, "state": "disconnected"})
}

// tasksList serves the reconnect recovery query:
// GET /tasks?instance_id=...&since=<RFC3339Nano>.
func (s *Server) tasksList(w http.ResponseWriter, r *http.Request) {
	var since time.Time
	if v := r.URL.Query().Get("since"); v != "" {
		if t, err := time.Parse(time.RFC3339Nano, v); err == nil {
			since = t
		}
	}
	writeJSON(w, 200, s.Tasks.List(r.URL.Query().Get("instance_id"), since))
}

// runners lists harness processes tracked by the takeover manager.
func (s *Server) runners(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, 200, s.Take.Snapshot())
}

var _ = detect.MethodPTY

func (s *Server) retry(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/agents/")
	id = strings.TrimSuffix(id, "/retry")
	if err := s.Take.Ctrl(id, "retry"); err != nil {
		writeJSON(w, 404, map[string]string{"error": "PROCESS_NOT_TRACKED"})
		return
	}
	writeJSON(w, 200, map[string]string{"instance_id": id, "action": "retry"})
}
