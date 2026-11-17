package approvalsv2

import (
	"bytes"
	"encoding/json"
	"fmt"
)

// validateJSONObject enforces the Modify contract (doc section 5.1): a JSON
// object, at most maxBytes when serialized.
func validateJSONObject(s string, maxBytes int) error {
	dec := json.NewDecoder(bytes.NewReader([]byte(s)))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return fmt.Errorf("%w: not valid JSON", ErrBadModify)
	}
	if dec.More() {
		return fmt.Errorf("%w: trailing data after JSON value", ErrBadModify)
	}
	m, ok := v.(map[string]any)
	if !ok {
		return fmt.Errorf("%w: must be a JSON object", ErrBadModify)
	}
	_ = m
	if len(s) > maxBytes {
		return fmt.Errorf("%w: exceeds %d bytes", ErrBadModify, maxBytes)
	}
	return nil
}
