package graphify

import (
	"encoding/json"
	"fmt"
	"math"
	"reflect"
	"strings"
)

const (
	PinnedUpstreamVersion = "v0.9.61"
	PinnedUpstreamCommit  = "fe66389083369c3159aa391117185c8f58b4d07c"
	PinnedContractID      = "prism-graphify-mcp-v0.9.61"

	MaxGraphResponseBytes = 8 << 10
	MaxTraversalDepth     = 3
	MaxPathHops           = 6
	MaxToolTokenBudget    = 2048
	MaxQuestionBytes      = 2048
	MaxLabelBytes         = 512
)

type ToolContract struct {
	Name        string         `json:"name"`
	InputSchema map[string]any `json:"input_schema"`
}

var pinnedToolContracts = []ToolContract{
	{
		Name: "query_graph",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"question":       map[string]any{"type": "string", "description": "Natural language question or keyword search"},
				"mode":           map[string]any{"type": "string", "enum": []any{"bfs", "dfs"}, "default": "bfs", "description": "bfs=broad context, dfs=trace a specific path"},
				"depth":          map[string]any{"type": "integer", "default": 3, "description": "Traversal depth (1-6)"},
				"token_budget":   map[string]any{"type": "integer", "default": 2000, "description": "Max output tokens"},
				"context_filter": map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "Optional explicit edge-context filter, e.g. ['call', 'field']"},
				"project_path":   map[string]any{"type": "string", "description": "Absolute path to a project directory containing graphify-out/graph.json. Optional — defaults to the graph this server was started with."},
			},
			"required": []any{"question"},
		},
	},
	{
		Name: "get_node",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"label":        map[string]any{"type": "string", "description": "Node label or ID to look up"},
				"project_path": map[string]any{"type": "string", "description": "Absolute path to a project directory containing graphify-out/graph.json. Optional — defaults to the graph this server was started with."},
			},
			"required": []any{"label"},
		},
	},
	{
		Name: "get_neighbors",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"label":           map[string]any{"type": "string"},
				"relation_filter": map[string]any{"type": "string", "description": "Optional: filter by relation type"},
				"token_budget":    map[string]any{"type": "integer", "default": 2000, "description": "Max output tokens"},
				"project_path":    map[string]any{"type": "string", "description": "Absolute path to a project directory containing graphify-out/graph.json. Optional — defaults to the graph this server was started with."},
			},
			"required": []any{"label"},
		},
	},
	{
		Name: "shortest_path",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"source":       map[string]any{"type": "string", "description": "Source concept label or keyword"},
				"target":       map[string]any{"type": "string", "description": "Target concept label or keyword"},
				"max_hops":     map[string]any{"type": "integer", "default": 8, "description": "Maximum hops to consider"},
				"undirected":   map[string]any{"type": "boolean", "default": false, "description": "Ignore stored edge direction when searching"},
				"project_path": map[string]any{"type": "string", "description": "Absolute path to a project directory containing graphify-out/graph.json. Optional — defaults to the graph this server was started with."},
			},
			"required": []any{"source", "target"},
		},
	},
}

func ApprovedTools() []string {
	names := make([]string, 0, len(pinnedToolContracts))
	for _, contract := range pinnedToolContracts {
		names = append(names, contract.Name)
	}
	return names
}

func PinnedToolContracts() []ToolContract {
	out := make([]ToolContract, 0, len(pinnedToolContracts))
	for _, contract := range pinnedToolContracts {
		out = append(out, ToolContract{Name: contract.Name, InputSchema: cloneSchema(contract.InputSchema)})
	}
	return out
}

func ValidatePinnedToolInventory(tools []ToolContract) error {
	byName := make(map[string]map[string]any, len(tools))
	for _, tool := range tools {
		if tool.Name == "" {
			return fmt.Errorf("Graphify MCP published a tool with no name")
		}
		if _, duplicate := byName[tool.Name]; duplicate {
			return fmt.Errorf("Graphify MCP published duplicate tool %q", tool.Name)
		}
		byName[tool.Name] = tool.InputSchema
	}
	for _, expected := range pinnedToolContracts {
		actual, ok := byName[expected.Name]
		if !ok {
			return fmt.Errorf("Graphify MCP contract drift: required tool %q is missing", expected.Name)
		}
		if !reflect.DeepEqual(canonicalSchema(actual), canonicalSchema(expected.InputSchema)) {
			return fmt.Errorf("Graphify MCP contract drift: input schema for %q does not match %s", expected.Name, PinnedContractID)
		}
	}
	return nil
}

func ValidateToolArguments(name string, args map[string]any) error {
	if !IsApprovedTool(name) {
		return fmt.Errorf("Graphify tool %q is not approved", name)
	}
	if args == nil {
		args = map[string]any{}
	}
	for key := range args {
		if key == "project_path" {
			return fmt.Errorf("Graphify tool project_path is not permitted; Prism uses the recorded workspace/index binding")
		}
		if !toolAcceptsArgument(name, key) {
			return fmt.Errorf("Graphify tool %q does not accept argument %q", name, key)
		}
	}
	switch name {
	case "query_graph":
		if err := requiredString(args, "question", MaxQuestionBytes); err != nil {
			return err
		}
		if _, present := args["mode"]; present {
			mode, ok := optionalString(args, "mode", 8)
			if !ok || (mode != "bfs" && mode != "dfs") {
				return fmt.Errorf("Graphify query_graph mode must be bfs or dfs")
			}
		}
		if err := optionalIntRange(args, "depth", 1, MaxTraversalDepth); err != nil {
			return err
		}
		if err := optionalIntRange(args, "token_budget", 1, MaxToolTokenBudget); err != nil {
			return err
		}
		if values, ok := args["context_filter"]; ok {
			list, ok := values.([]any)
			if !ok {
				if strings, stringsOK := values.([]string); stringsOK {
					list = make([]any, len(strings))
					for i := range strings {
						list[i] = strings[i]
					}
				} else {
					return fmt.Errorf("Graphify query_graph context_filter must be an array of strings")
				}
			}
			if len(list) > 8 {
				return fmt.Errorf("Graphify query_graph context_filter may contain at most 8 entries")
			}
			for _, item := range list {
				text, ok := item.(string)
				if !ok || len(text) > MaxLabelBytes {
					return fmt.Errorf("Graphify query_graph context_filter must be an array of strings")
				}
			}
		}
	case "get_node":
		return requiredString(args, "label", MaxLabelBytes)
	case "get_neighbors":
		if err := requiredString(args, "label", MaxLabelBytes); err != nil {
			return err
		}
		if _, present := args["relation_filter"]; present {
			if _, ok := optionalString(args, "relation_filter", MaxLabelBytes); !ok {
				return fmt.Errorf("Graphify get_neighbors relation_filter must be a string")
			}
		}
		return optionalIntRange(args, "token_budget", 1, MaxToolTokenBudget)
	case "shortest_path":
		if err := requiredString(args, "source", MaxLabelBytes); err != nil {
			return err
		}
		if err := requiredString(args, "target", MaxLabelBytes); err != nil {
			return err
		}
		if err := optionalIntRange(args, "max_hops", 1, MaxPathHops); err != nil {
			return err
		}
		if value, ok := args["undirected"]; ok {
			if _, ok := value.(bool); !ok {
				return fmt.Errorf("Graphify shortest_path undirected must be a boolean")
			}
		}
	}
	return nil
}

func toolAcceptsArgument(name, key string) bool {
	for _, contract := range pinnedToolContracts {
		if contract.Name != name {
			continue
		}
		properties, _ := contract.InputSchema["properties"].(map[string]any)
		_, ok := properties[key]
		return ok
	}
	return false
}

func requiredString(args map[string]any, key string, maxBytes int) error {
	value, ok := optionalString(args, key, maxBytes)
	if !ok || value == "" {
		return fmt.Errorf("Graphify tool argument %q is required and must be a non-empty string", key)
	}
	return nil
}

func optionalString(args map[string]any, key string, maxBytes int) (string, bool) {
	value, present := args[key]
	if !present {
		return "", false
	}
	text, ok := value.(string)
	if !ok {
		return "", false
	}
	text = strings.TrimSpace(text)
	if len(text) > maxBytes {
		return "", false
	}
	return text, true
}

func optionalIntRange(args map[string]any, key string, min, max int) error {
	value, present := args[key]
	if !present {
		return nil
	}
	var integer int64
	switch value := value.(type) {
	case int:
		integer = int64(value)
	case int64:
		integer = value
	case float64:
		if math.Trunc(value) != value {
			return fmt.Errorf("Graphify tool argument %q must be an integer", key)
		}
		if value < float64(min) || value > float64(max) {
			return fmt.Errorf("Graphify tool argument %q must be between %d and %d", key, min, max)
		}
		return nil
	case json.Number:
		parsed, err := value.Int64()
		if err != nil {
			return fmt.Errorf("Graphify tool argument %q must be an integer", key)
		}
		integer = parsed
	default:
		return fmt.Errorf("Graphify tool argument %q must be an integer", key)
	}
	if integer < int64(min) || integer > int64(max) {
		return fmt.Errorf("Graphify tool argument %q must be between %d and %d", key, min, max)
	}
	return nil
}

func canonicalSchema(schema map[string]any) map[string]any {
	if schema == nil {
		return nil
	}
	data, err := json.Marshal(schema)
	if err != nil {
		return nil
	}
	var normalized map[string]any
	if err := json.Unmarshal(data, &normalized); err != nil {
		return nil
	}
	return normalized
}

func cloneSchema(schema map[string]any) map[string]any {
	return canonicalSchema(schema)
}
