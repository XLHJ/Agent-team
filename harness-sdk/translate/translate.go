// Package translate maps a CLI's native stream to control-plane events.
// Phase 2 ships the per-signature skeletons; Phase-3 adapters refine them.
package translate

import (
	"encoding/json"
	"fmt"
	"strings"

	"agent-control-plane/harness-sdk/takeover"
)

// For returns the translator for a signature.
func For(signature string) takeover.Translator {
	switch signature {
	case "claude-code":
		return claude{}
	case "codex":
		return codex{}
	case "hermes":
		return passthrough{}
	default:
		return passthrough{}
	}
}

// claude maps Claude Code stream-json (SDK mode).
type claude struct{}

func (claude) EncodePrompt(prompt string) ([]byte, error) {
	msg := map[string]any{
		"type": "user",
		"message": map[string]any{
			"role":    "user",
			"content": []map[string]string{{"type": "text", "text": prompt}},
		},
	}
	b, err := json.Marshal(msg)
	return append(b, '\n'), err
}

func (claude) DecodeLine(line []byte) (takeover.Event, bool) {
	var f struct {
		Type    string `json:"type"`
		Subtype string `json:"subtype"`
		Message struct {
			Content []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content"`
		} `json:"message"`
		Result    string `json:"result"`
		IsError   bool   `json:"is_error"`
		RequestID string `json:"request_id"`
		Request   struct {
			Subtype    string         `json:"subtype"`
			ToolName   string         `json:"tool_name"`
			Input      map[string]any `json:"input"`
			Permission string         `json:"permission_suggestions"`
		} `json:"request"`
	}
	if err := json.Unmarshal(line, &f); err != nil {
		return takeover.Event{}, false
	}
	switch f.Type {
	case "assistant":
		var text strings.Builder
		for _, c := range f.Message.Content {
			if c.Type == "text" && c.Text != "" {
				text.WriteString(c.Text)
			}
		}
		if text.Len() > 0 {
			return takeover.Event{Kind: takeover.EvProgress, Step: snippet(text.String())}, true
		}
	case "result":
		if f.IsError {
			return takeover.Event{Kind: takeover.EvFailed, ErrorCode: "AGENT_ERROR", ErrorMessage: snippet(f.Result)}, true
		}
		return takeover.Event{Kind: takeover.EvCompleted, RawOutput: f.Result}, true
	case "system":
		if f.Subtype == "init" {
			return takeover.Event{Kind: takeover.EvLog, Step: "session initialized"}, true
		}
	case "control_request":
		if f.Request.Subtype == "can_use_tool" {
			params, _ := json.Marshal(f.Request.Input)
			return takeover.Event{
				Kind:           takeover.EvApproval,
				ApprovalID:     f.RequestID,
				OperationType:  f.Request.ToolName,
				RiskLevel:      "medium",
				ContextSummary: snippet(string(params)),
			}, true
		}
	}
	return takeover.Event{}, false
}

func (claude) EncodeApprovalResponse(approvalID, decision string, modified []byte) ([]byte, error) {
	behavior := "deny"
	if decision == "approve" || decision == "modify" {
		behavior = "allow"
	}
	inner := map[string]any{"behavior": behavior}
	if len(modified) > 0 {
		var in map[string]any
		if json.Unmarshal(modified, &in) == nil {
			inner["updatedInput"] = in
		}
	}
	msg := map[string]any{
		"type": "control_response",
		"response": map[string]any{
			"subtype":    "success",
			"request_id": approvalID,
			"response":   inner,
		},
	}
	b, err := json.Marshal(msg)
	return append(b, '\n'), err
}

// codex maps `codex exec --json` JSONL.
type codex struct{}

func (codex) EncodePrompt(prompt string) ([]byte, error) {
	return []byte(prompt + "\n"), nil
}

func (codex) DecodeLine(line []byte) (takeover.Event, bool) {
	var f struct {
		Type string `json:"type"`
		Item struct {
			Type    string `json:"type"`
			Text    string `json:"text"`
			Command string `json:"command"`
		} `json:"item"`
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(line, &f); err != nil {
		return takeover.Event{}, false
	}
	switch f.Type {
	case "item.started":
		if f.Item.Command != "" {
			return takeover.Event{Kind: takeover.EvProgress, Step: "exec: " + snippet(f.Item.Command)}, true
		}
	case "item.completed":
		if f.Item.Type == "agent_message" && f.Item.Text != "" {
			return takeover.Event{Kind: takeover.EvPartial, RawOutput: f.Item.Text}, true
		}
		if f.Item.Type == "thinking" && f.Item.Text != "" {
			return takeover.Event{Kind: takeover.EvLog, Step: snippet(f.Item.Text)}, true
		}
	case "turn.completed":
		return takeover.Event{Kind: takeover.EvCompleted}, true
	case "turn.failed":
		return takeover.Event{Kind: takeover.EvFailed, ErrorCode: "AGENT_ERROR", ErrorMessage: snippet(f.Error.Message)}, true
	case "error":
		return takeover.Event{Kind: takeover.EvFailed, ErrorCode: "AGENT_ERROR", ErrorMessage: snippet(f.Error.Message)}, true
	}
	return takeover.Event{}, false
}

func (codex) EncodeApprovalResponse(string, string, []byte) ([]byte, error) { return nil, nil }

// passthrough consumes {"kind":"progress|completed|failed",...} NDJSON and is
// also used for Hermes until Phase 3 supplies the real native mapping.
type passthrough struct{}

func (passthrough) EncodePrompt(prompt string) ([]byte, error) {
	b, err := json.Marshal(map[string]any{"type": "user", "prompt": prompt})
	return append(b, '\n'), err
}

func (passthrough) DecodeLine(line []byte) (takeover.Event, bool) {
	var f struct {
		Kind      string `json:"kind"`
		Step      string `json:"step"`
		RawOutput string `json:"raw_output"`
		ErrorCode string `json:"error_code"`
		ErrorMsg  string `json:"error_message"`
	}
	if err := json.Unmarshal(line, &f); err != nil {
		return takeover.Event{}, false
	}
	switch f.Kind {
	case "progress":
		return takeover.Event{Kind: takeover.EvProgress, Step: f.Step}, true
	case "completed":
		return takeover.Event{Kind: takeover.EvCompleted, RawOutput: f.RawOutput}, true
	case "failed":
		return takeover.Event{Kind: takeover.EvFailed, ErrorCode: f.ErrorCode, ErrorMessage: f.ErrorMsg}, true
	}
	return takeover.Event{}, false
}

func (passthrough) EncodeApprovalResponse(string, string, []byte) ([]byte, error) { return nil, nil }

func snippet(s string) string {
	s = strings.TrimSpace(strings.ReplaceAll(s, "\n", " "))
	if len(s) > 120 {
		return s[:120] + "…"
	}
	return s
}

var _ = fmt.Sprintf
