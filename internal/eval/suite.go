// Package eval runs versioned MCP workflows against model runtimes and checks
// their functional outcomes against explicitly approved regression baselines.
package eval

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"strings"

	prismbundle "github.com/bryanbarton525/prism"
	"gopkg.in/yaml.v3"
)

const Version = 1
const GraderVersion = "mcp-outcomes-v1"

type Suite struct {
	Version     int    `json:"version" yaml:"version"`
	ID          string `json:"id" yaml:"id"`
	License     string `json:"license" yaml:"license"`
	Description string `json:"description" yaml:"description"`
	Cases       []Case `json:"cases" yaml:"cases"`
}

type ToolRef struct {
	Server string `json:"server" yaml:"server"`
	Tool   string `json:"tool" yaml:"tool"`
}

type Case struct {
	ExpectedToolSequence []ToolRef     `json:"expected_tool_sequence,omitempty" yaml:"expected_tool_sequence,omitempty"`
	ID                   string        `json:"id" yaml:"id"`
	AgentID              string        `json:"agent_id" yaml:"agent_id"`
	Skills               []string      `json:"skills" yaml:"skills"`
	Task                 string        `json:"task" yaml:"task"`
	Tools                []FixtureTool `json:"tools" yaml:"tools"`
	Checks               []Check       `json:"checks" yaml:"checks"`
	ConsistencyPaths     []string      `json:"consistency_paths" yaml:"consistency_paths"`
	RequiredBridgeCalls  []string      `json:"required_bridge_calls,omitempty" yaml:"required_bridge_calls,omitempty"`
	ForbiddenTools       []string      `json:"forbidden_tools,omitempty" yaml:"forbidden_tools,omitempty"`
	MaxBytes             int           `json:"max_bytes,omitempty" yaml:"max_bytes,omitempty"`
}

type FixtureTool struct {
	Server      string         `json:"server" yaml:"server"`
	Name        string         `json:"name" yaml:"name"`
	Description string         `json:"description" yaml:"description"`
	InputSchema map[string]any `json:"input_schema" yaml:"input_schema"`
	Arguments   map[string]any `json:"arguments" yaml:"arguments"`
	Response    map[string]any `json:"response" yaml:"response"`
	IsError     bool           `json:"is_error,omitempty" yaml:"is_error,omitempty"`
	Required    bool           `json:"required" yaml:"required"`
}

type Check struct {
	Path   string `json:"path" yaml:"path"`
	Equals any    `json:"equals" yaml:"equals"`
}

func Hash(value any) string {
	data, _ := json.Marshal(value)
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func LoadSuite(path string) (Suite, error) {
	var data []byte
	var err error
	if path == "" {
		data, err = fs.ReadFile(prismbundle.EvaluationFS(), "evals/mcp-response-v1/suite.yaml")
	} else {
		data, err = os.ReadFile(path)
	}
	if err != nil {
		return Suite{}, err
	}
	if len(data) > 5<<20 {
		return Suite{}, fmt.Errorf("suite exceeds 5 MiB")
	}
	var suite Suite
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(&suite); err != nil {
		return suite, err
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		return suite, fmt.Errorf("suite must contain one YAML document")
	}
	return suite, suite.Validate()
}

func (s Suite) Validate() error {
	if _, err := json.Marshal(s); err != nil {
		return fmt.Errorf("suite must contain JSON-compatible values: %w", err)
	}
	if s.Version != Version || strings.TrimSpace(s.ID) == "" || s.License == "" || len(s.Cases) == 0 || len(s.Cases) > 100 {
		return fmt.Errorf("suite requires version %d, id, license, and 1–100 cases", Version)
	}
	ids := map[string]bool{}
	for _, c := range s.Cases {
		if c.ID == "" || ids[c.ID] || c.AgentID == "" || c.Task == "" || len(c.Checks) == 0 || len(c.ConsistencyPaths) == 0 {
			return fmt.Errorf("case %q requires unique id, agent_id, task, checks, and consistency_paths", c.ID)
		}
		ids[c.ID] = true
		if c.MaxBytes < 0 || c.MaxBytes > 1<<20 {
			return fmt.Errorf("case %s: max_bytes must be 0–1048576", c.ID)
		}
		paths := append([]string{}, c.ConsistencyPaths...)
		for _, check := range c.Checks {
			paths = append(paths, check.Path)
		}
		for _, path := range paths {
			if !strings.HasPrefix(path, "/") {
				return fmt.Errorf("case %s: check paths must be JSON pointers", c.ID)
			}
		}
		tools := map[string]bool{}
		for _, tool := range c.Tools {
			key := tool.Server + "/" + tool.Name
			if tool.Server == "" || tool.Name == "" || tools[key] || tool.InputSchema["type"] != "object" || tool.Response == nil {
				return fmt.Errorf("case %s: invalid or duplicate fixture tool %s", c.ID, key)
			}
			tools[key] = true
		}
		for _, ref := range c.ExpectedToolSequence {
			if !tools[ref.Server+"/"+ref.Tool] {
				return fmt.Errorf("case %s: sequence refers to unknown fixture tool", c.ID)
			}
		}
	}
	return nil
}
