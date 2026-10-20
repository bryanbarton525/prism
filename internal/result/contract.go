package result

import (
	"encoding/json"
	"fmt"
	"github.com/google/jsonschema-go/jsonschema"
)

// ResolveDecisionSchema accepts a bounded self-contained schema. Remote refs
// are rejected: request validation must not fetch arbitrary external resources.
func ResolveDecisionSchema(schema map[string]any) (*jsonschema.Resolved, error) {
	if len(schema) == 0 {
		return nil, nil
	}
	data, err := json.Marshal(schema)
	if err != nil || len(data) > 16384 {
		return nil, fmt.Errorf("decision schema must be valid JSON at most 16 KiB")
	}
	var check func(any) error
	check = func(v any) error {
		switch x := v.(type) {
		case map[string]any:
			for k, value := range x {
				if k == "$ref" || k == "$dynamicRef" || k == "$id" {
					return fmt.Errorf("decision schema references are not supported")
				}
				if err := check(value); err != nil {
					return err
				}
			}
		case []any:
			for _, value := range x {
				if err := check(value); err != nil {
					return err
				}
			}
		}
		return nil
	}
	if err := check(schema); err != nil {
		return nil, err
	}
	if schema["type"] != "object" {
		return nil, fmt.Errorf("decision schema must have type object")
	}
	var compiled jsonschema.Schema
	if err := json.Unmarshal(data, &compiled); err != nil {
		return nil, err
	}
	return compiled.Resolve(nil)
}
