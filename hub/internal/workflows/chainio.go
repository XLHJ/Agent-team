package workflows

// ChainFileVersion is the format tag of the JSON interchange file.
const ChainFileVersion = "chain.v1"

// ChainFile is the portable JSON representation of one chain definition
// (name, description and stages only; IDs, status, timestamps and run
// history are intentionally excluded).
type ChainFile struct {
	Version     string           `json:"version"`
	Name        string           `json:"name"`
	Description string           `json:"description,omitempty"`
	Stages      []ChainFileStage `json:"stages"`
}

// ChainFileStage is one stage inside a ChainFile.
type ChainFileStage struct {
	AgentInstance string `json:"agent_instance_id,omitempty"`
	CapabilityTag string `json:"agent_capability_tag,omitempty"`
	TaskPrompt    string `json:"task_prompt"`
	ReportPrompt  string `json:"report_prompt"`
	TimeoutMS     int    `json:"timeout_ms,omitempty"`
}
