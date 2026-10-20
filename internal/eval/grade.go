package eval

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/bryanbarton525/prism/internal/result"
)

type MetricResult struct {
	Name      string  `json:"name"`
	Score     float64 `json:"score"`
	Threshold float64 `json:"threshold"`
	Passed    bool    `json:"passed"`
	Reason    string  `json:"reason"`
}

type Grade struct {
	Metrics           []MetricResult  `json:"metrics"`
	Passed            bool            `json:"passed"`
	Checks            map[string]bool `json:"checks"`
	SemanticSignature string          `json:"semantic_signature,omitempty"`
}

func pointer(root any, path string) (any, bool) {
	current := root
	for _, part := range strings.Split(strings.TrimPrefix(path, "/"), "/") {
		part = strings.ReplaceAll(strings.ReplaceAll(part, "~1", "/"), "~0", "~")
		switch value := current.(type) {
		case map[string]any:
			var ok bool
			current, ok = value[part]
			if !ok {
				return nil, false
			}
		case []any:
			index, err := strconv.Atoi(part)
			if err != nil || index < 0 || index >= len(value) {
				return nil, false
			}
			current = value[index]
		default:
			return nil, false
		}
	}
	return current, true
}

func outputObject(raw string) (map[string]any, bool) {
	raw = strings.TrimSpace(raw)
	if strings.HasPrefix(raw, "```json") && strings.HasSuffix(raw, "```") {
		raw = strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(raw, "```json"), "```"))
	} else if strings.HasPrefix(raw, "```") && strings.HasSuffix(raw, "```") {
		raw = strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(raw, "```"), "```"))
	}
	var object map[string]any
	if err := json.Unmarshal([]byte(raw), &object); err != nil || object == nil {
		return nil, false
	}
	return object, true
}

func GradeTrial(c Case, t Trial) Grade {
	checks := map[string]bool{"mcp_response": !t.MCPError && t.Error == "" && t.Result.Status == result.StatusOK, "model_identity": len(t.Calls) > 0}
	for _, call := range t.Calls {
		if call.Response == nil || call.Response.Model != t.Model {
			checks["model_identity"] = false
		}
	}
	if r := t.Recommendation; r != nil {
		checks["laya_applied"] = r.Injected && r.Error == "" && r.Result.ScoreKind == "laya_choice" && r.Result.ModelIdentity == "laya-english" && !r.Result.Incomplete && len(r.Result.Warnings) == 0 && len(r.Result.Tools) > 0
	}
	object, valid := outputObject(t.Result.RawOutput)
	summary, summaryOK := object["summary"].(string)
	findings, findingsOK := object["findings"].([]any)
	confidence, _ := object["confidence"].(string)
	checks["response_schema"] = valid && summaryOK && strings.TrimSpace(summary) != "" && findingsOK && len(findings) <= 5 && (confidence == "low" || confidence == "medium" || confidence == "high")
	for _, finding := range findings {
		if _, ok := finding.(string); !ok {
			checks["response_schema"] = false
		}
	}
	for i, check := range c.Checks {
		value, found := pointer(object, check.Path)
		checks[fmt.Sprintf("outcome:%d:%s", i, check.Path)] = found && Hash(value) == Hash(check.Equals)
	}
	checks["tool_arguments"] = true
	for _, call := range t.ToolCalls {
		if !call.Matched {
			checks["tool_arguments"] = false
		}
	}
	for _, tool := range c.Tools {
		if !tool.Required {
			continue
		}
		found := false
		for _, call := range t.ToolCalls {
			if call.Server == tool.Server && call.Tool == tool.Name && call.Matched {
				found = true
			}
		}
		checks["required_tool:"+tool.Server+"/"+tool.Name] = found
	}
	for _, name := range c.RequiredBridgeCalls {
		checks["required_bridge:"+name] = bridgeSucceeded(t, name)
	}
	if len(c.ExpectedToolSequence) > 0 {
		var sequence []ToolRef
		for _, call := range t.ToolCalls {
			sequence = append(sequence, ToolRef{Server: call.Server, Tool: call.Tool})
		}
		checks["tool_sequence"] = Hash(sequence) == Hash(c.ExpectedToolSequence)
	}
	for _, name := range c.ForbiddenTools {
		allowed := true
		for _, call := range t.Calls {
			messages := messagesForCalls(call)
			for _, message := range messages {
				for _, tool := range message {
					if tool == name {
						allowed = false
					}
				}
			}
		}
		checks["forbidden_tool:"+name] = allowed
	}
	grade := Grade{Passed: true, Checks: checks}
	for _, pass := range checks {
		if !pass {
			grade.Passed = false
		}
	}
	var names []string
	for name := range checks {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		pass := checks[name]
		score := 0.0
		reason := "Requirement was not met; inspect the recorded model and MCP calls."
		if pass {
			score = 1
			reason = "Requirement satisfied."
		}
		if !pass {
			switch {
			case strings.HasPrefix(name, "outcome:"):
				for i, check := range c.Checks {
					if name == fmt.Sprintf("outcome:%d:%s", i, check.Path) {
						actual, found := pointer(object, check.Path)
						expectedJSON, _ := json.Marshal(check.Equals)
						actualJSON, _ := json.Marshal(actual)
						reason = fmt.Sprintf("Expected %s at %s; observed %s (present=%t).", expectedJSON, check.Path, actualJSON, found)
					}
				}
			case strings.HasPrefix(name, "required_tool:"):
				reason = "Required MCP tool was not called with the expected arguments."
			case name == "response_schema":
				reason = "Expected JSON summary, string findings array (at most five), and low/medium/high confidence."
			case name == "model_identity":
				reason = "A model call failed or the served model ID did not match the requested model."
			case name == "mcp_response":
				reason = fmt.Sprintf("MCP/agent execution failed: status=%s, error=%s, summary=%s", t.Result.Status, t.Error, t.Result.Summary)
			case name == "tool_sequence":
				reason = "Observed MCP call order or count differs from the exact expected sequence."
			case strings.HasPrefix(name, "forbidden_tool:"):
				reason = "Model attempted a forbidden downstream tool."
			}
		}
		grade.Metrics = append(grade.Metrics, MetricResult{Name: name, Score: score, Threshold: 1, Passed: pass, Reason: reason})
	}
	if valid {
		semantic := map[string]any{}
		complete := true
		for _, path := range c.ConsistencyPaths {
			value, found := pointer(object, path)
			if !found {
				complete = false
			}
			semantic[path] = value
		}
		if complete {
			grade.SemanticSignature = Hash(semantic)
		}
	}
	return grade
}

type runtimeMessage []string

func messagesForCalls(call ModelCall) []runtimeMessage {
	var out []runtimeMessage
	for _, message := range call.Request.Messages {
		for _, tool := range message.ToolCalls {
			if tool.Function.Name == "call_mcp_tool" {
				name, _ := tool.Function.Arguments["tool"].(string)
				out = append(out, runtimeMessage{name})
			}
		}
	}
	if call.Response != nil {
		for _, tool := range call.Response.Message.ToolCalls {
			if tool.Function.Name == "call_mcp_tool" {
				name, _ := tool.Function.Arguments["tool"].(string)
				out = append(out, runtimeMessage{name})
			}
		}
	}
	return out
}

// Only correlated tool replies count as bridge execution evidence. Model-written
// artifacts cannot satisfy this check.
func bridgeSucceeded(t Trial, name string) bool {
	for _, call := range t.Calls {
		names := map[string]string{}
		for _, message := range call.Request.Messages {
			for _, tool := range message.ToolCalls {
				names[tool.ID] = tool.Function.Name
			}
		}
		for _, message := range call.Request.Messages {
			if message.Role != "tool" || names[message.ToolCallID] != name {
				continue
			}
			var body map[string]any
			if json.Unmarshal([]byte(message.Content), &body) == nil && body != nil {
				if _, hasError := body["error"]; !hasError {
					return true
				}
			}
		}
	}
	return false
}
