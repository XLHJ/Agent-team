package panel

import (
	"encoding/json"
)

func (h *Handler) filesList(params json.RawMessage) (any, *rpcError) {
	if !h.d.Files.Enabled() {
		return nil, bizErr("FILES_DISABLED", "no workspace configured")
	}
	var p struct {
		Path string `json:"path"`
	}
	_ = json.Unmarshal(params, &p)
	entries, err := h.d.Files.List(p.Path)
	if err != nil {
		return nil, bizErr("FILE_ACCESS_DENIED", err.Error())
	}
	return map[string]any{"entries": entries}, nil
}

func (h *Handler) filesRead(params json.RawMessage) (any, *rpcError) {
	if !h.d.Files.Enabled() {
		return nil, bizErr("FILES_DISABLED", "no workspace configured")
	}
	var p struct {
		Path string `json:"path"`
	}
	_ = json.Unmarshal(params, &p)
	res, err := h.d.Files.Read(p.Path)
	if err != nil {
		if err.Error() == "binary file" {
			return nil, bizErr("BINARY_FILE", "no text preview")
		}
		return nil, bizErr("FILE_ACCESS_DENIED", err.Error())
	}
	return map[string]any{"file": res}, nil
}
