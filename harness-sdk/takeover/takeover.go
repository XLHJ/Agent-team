// Package takeover opens the control channel to a CLI agent using the highest
// available method from the signature table: native JSON-RPC, structured
// stdio, or PTY (ConPTY on Windows) as the last resort.
package takeover

import (
	"fmt"

	"agent-control-plane/harness-sdk/detect"
)

// Event kinds emitted by a channel.
const (
	EvProgress  = "progress"
	EvPartial   = "partial"
	EvCompleted = "completed"
	EvFailed    = "failed"
	EvLog       = "log"
	EvApproval  = "approval"
)

// Event is one translated agent-side event.
type Event struct {
	Kind           string
	Step           string
	RawOutput      string
	ErrorCode      string
	ErrorMessage   string
	ApprovalID     string
	OperationType  string
	RiskLevel      string
	ContextSummary string
}

// Translator encodes prompts/approval answers and decodes agent lines.
type Translator interface {
	EncodePrompt(prompt string) ([]byte, error)
	DecodeLine(line []byte) (Event, bool)
	EncodeApprovalResponse(approvalID, decision string, modified []byte) ([]byte, error)
}

// Channel is an open takeover.
type Channel interface {
	Start() error
	SendTask(prompt string) error
	AnswerApproval(approvalID, decision string, modified []byte) error
	Cancel() error
	Interrupt() error
	Events() <-chan Event
	// ExitErr returns the child's exit error after Events is closed.
	ExitErr() error
	PartialText() string
	Close() error
}

// Open builds the channel for method.
func Open(sig detect.Signature, path, method string, tr Translator) (Channel, error) {
	switch method {
	case detect.MethodNative, detect.MethodStdio:
		return newProc(path, sig.Args, tr, method == detect.MethodNative), nil
	case detect.MethodPTY:
		return newPTY(path, sig.Args, tr), nil
	}
	return nil, fmt.Errorf("takeover: unknown method %q", method)
}

// Choose returns the highest-priority method the signature declares.
func Choose(sig detect.Signature) (string, bool) {
	if len(sig.Methods) == 0 {
		return "", false
	}
	return sig.Methods[0], true
}
