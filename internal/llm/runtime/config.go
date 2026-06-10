package runtime

import "time"

type Engine string

const (
	EngineSGLang Engine = "sglang"
	EngineVLLM   Engine = "vllm"
)

type Config struct {
	Engine    Engine            `json:"engine" yaml:"engine"`
	BaseURL   string            `json:"base_url" yaml:"base_url"`
	APIKey    string            `json:"api_key,omitempty" yaml:"api_key,omitempty"`
	Model     string            `json:"model,omitempty" yaml:"model,omitempty"`
	Headers   map[string]string `json:"headers,omitempty" yaml:"headers,omitempty"`
	TimeoutMS int               `json:"timeout_ms,omitempty" yaml:"timeout_ms,omitempty"`
}

type RuntimeConfig struct {
	Primary  Config  `json:"primary" yaml:"primary"`
	Fallback *Config `json:"fallback,omitempty" yaml:"fallback,omitempty"`
}

func (c Config) Timeout() time.Duration {
	if c.TimeoutMS <= 0 {
		return 0
	}
	return time.Duration(c.TimeoutMS) * time.Millisecond
}
