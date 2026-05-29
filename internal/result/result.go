// Package result defines the normalized response schema shared by the CLI and MCP adapters.
package result

// Usage holds token and timing estimates returned by the Ollama runtime.
type Usage struct {
	PromptTokensEstimate     int `json:"prompt_tokens_estimate"`
	CompletionTokensEstimate int `json:"completion_tokens_estimate"`
	DurationMs               int `json:"duration_ms"`
}

// RunResult is the normalized envelope returned for every agent invocation.
type RunResult struct {
	AgentID    string         `json:"agent_id"`
	Model      string         `json:"model"`
	Status     string         `json:"status"`
	Summary    string         `json:"summary"`
	Findings   []string       `json:"findings"`
	Artifacts  []string       `json:"artifacts"`
	Confidence string         `json:"confidence"`
	RawOutput  string         `json:"raw_output,omitempty"`
	Error      string         `json:"error,omitempty"`
	Usage      Usage          `json:"usage"`
	SkillsUsed []string       `json:"skills_used,omitempty"`
}

// AgentSummary is a lightweight description of an agent used in listing output.
type AgentSummary struct {
	ID              string   `json:"id"`
	Name            string   `json:"name"`
	Description     string   `json:"description"`
	Model           string   `json:"model"`
	AllowedSkills   []string `json:"allowed_skills"`
	LatencyBudgetMs int      `json:"latency_budget_ms"`
}

// Constitution is the full textual contract for an agent.
type Constitution struct {
	AgentID string `json:"agent_id"`
	Text    string `json:"text"`
}

// DoctorCheck holds the result of a single connectivity or config check.
type DoctorCheck struct {
	Name    string `json:"name"`
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
}

// DoctorResult is the structured output of the doctor tool.
type DoctorResult struct {
	OllamaHost  string        `json:"ollama_host"`
	AgentDir    string        `json:"agent_dir"`
	AgentCount  int           `json:"agent_count"`
	SkillCount  int           `json:"skill_count"`
	Checks      []DoctorCheck `json:"checks"`
	Status      string        `json:"status"`
}
