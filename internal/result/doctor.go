package result

// DoctorCheck holds the result of a single connectivity or config check.
type DoctorCheck struct {
	Name    string `json:"name"`
	Status  string `json:"status"` // ok | warn | fail
	Message string `json:"message,omitempty"`
}

// DoctorResult is the structured output of prism config doctor and the MCP doctor tool.
type DoctorResult struct {
	OllamaHost string        `json:"ollama_host"`
	AgentDir   string        `json:"agent_dir"`
	SkillsDir  string        `json:"skills_dir"`
	AgentCount int           `json:"agent_count"`
	SkillCount int           `json:"skill_count"`
	Checks     []DoctorCheck `json:"checks"`
	Status     string        `json:"status"` // ok | degraded
}
