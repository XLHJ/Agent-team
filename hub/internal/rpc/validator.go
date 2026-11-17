package rpc

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

// Validator validates full frames against per-method schemas.
type Validator struct {
	schemas map[string]*jsonschema.Schema
}

// NewValidator compiles every <method>.schema.json in dir.
func NewValidator(dir string) (*Validator, error) {
	v := &Validator{schemas: map[string]*jsonschema.Schema{}}
	files, err := filepath.Glob(filepath.Join(dir, "*.schema.json"))
	if err != nil {
		return nil, err
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("no schemas in %s", dir)
	}
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

// Validate checks frame against method schema; returns JSON path of failure.
func (v *Validator) Validate(method string, frame any) (field string, err error) {
	sch, ok := v.schemas[method]
	if !ok {
		return "", errors.New("method not found")
	}
	if err := sch.Validate(frame); err != nil {
		var ve *jsonschema.ValidationError
		if errors.As(err, &ve) {
			return strings.Join(ve.InstanceLocation, "/"), err
		}
		return "", err
	}
	return "", nil
}
