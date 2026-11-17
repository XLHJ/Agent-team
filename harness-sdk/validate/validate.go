// Package validate loads the frozen protocol schemas once and validates params
// against them; both SDKs and the Hub share this single contract source.
package validate

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

// Validator validates method params.
type Validator struct {
	schemas map[string]*jsonschema.Schema
}

// Load compiles every <method>.schema.json under dir.
func Load(dir string) (*Validator, error) {
	files, err := filepath.Glob(filepath.Join(dir, "*.schema.json"))
	if err != nil {
		return nil, err
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("validate: no schemas in %s", dir)
	}
	v := &Validator{schemas: map[string]*jsonschema.Schema{}}
	for _, f := range files {
		raw, err := os.ReadFile(f)
		if err != nil {
			return nil, err
		}
		var doc any
		if err := json.Unmarshal(raw, &doc); err != nil {
			return nil, fmt.Errorf("%s: %w", f, err)
		}
		name := strings.TrimSuffix(filepath.Base(f), ".schema.json")
		c := jsonschema.NewCompiler()
		u := "mem://" + name
		if err := c.AddResource(u, doc); err != nil {
			return nil, fmt.Errorf("%s: %w", f, err)
		}
		sch, err := c.Compile(u)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", f, err)
		}
		v.schemas[name] = sch
	}
	return v, nil
}

// Validate checks one full JSON-RPC frame for method.
func (v *Validator) Validate(method string, frame json.RawMessage) error {
	sch, ok := v.schemas[method]
	if !ok {
		return errors.New("validate: unknown method " + method)
	}
	var doc any
	if err := json.Unmarshal(frame, &doc); err != nil {
		return err
	}
	return sch.Validate(doc)
}
