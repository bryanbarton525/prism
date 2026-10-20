package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/bryanbarton525/prism/internal/llm/runtime"
)

// ValidateModelTarget validates operator configuration without contacting it.
func ValidateModelTarget(cfg runtime.Config) error {
	switch cfg.Engine {
	case runtime.EngineOllama, runtime.EngineSGLang, runtime.EngineVLLM:
	default:
		return fmt.Errorf("unsupported engine %q; choose ollama, sglang, or vllm", cfg.Engine)
	}
	u, err := url.Parse(cfg.BaseURL)
	if err != nil || u.Hostname() == "" || (u.Scheme != "http" && u.Scheme != "https") || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return fmt.Errorf("endpoint must be an HTTP(S) URL without credentials, query, or fragment")
	}
	return nil
}

// PublicConfig is safe for diagnostic output, including legacy endpoint values.
func PublicConfig(cfg runtime.Config) runtime.Config {
	cfg.APIKey, cfg.Headers = "", nil
	u, err := url.Parse(cfg.BaseURL)
	if err != nil {
		cfg.BaseURL = "(invalid endpoint)"
		return cfg
	}
	u.User, u.RawQuery, u.Fragment = nil, "", ""
	cfg.BaseURL = u.String()
	return cfg
}

// ListModels checks advertised availability, not inference readiness. It never
// generates text, pulls weights, or follows redirects to another endpoint.
func ListModels(ctx context.Context, cfg runtime.Config) ([]string, error) {
	if err := ValidateModelTarget(cfg); err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	base := strings.TrimRight(cfg.BaseURL, "/")
	endpoint := base + "/api/tags"
	if cfg.Engine != runtime.EngineOllama {
		if !strings.HasSuffix(base, "/v1") {
			base += "/v1"
		}
		endpoint = base + "/models"
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("building model-list request")
	}
	for key, value := range cfg.Headers {
		req.Header.Set(key, value)
	}
	if cfg.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+cfg.APIKey)
	}
	client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("model list unavailable: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("model list returned HTTP %d (check endpoint and authentication)", resp.StatusCode)
	}
	const maxBody = 4 << 20
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBody+1))
	if err != nil || len(body) > maxBody {
		return nil, fmt.Errorf("cannot read model list or response exceeds 4 MiB")
	}
	var parsed struct {
		Models *[]struct {
			Name string `json:"name"`
		} `json:"models"`
		Data *[]struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("invalid model-list response")
	}
	models := []string{}
	if cfg.Engine == runtime.EngineOllama {
		if parsed.Models == nil {
			return nil, fmt.Errorf("invalid Ollama model list: missing models array")
		}
		for _, item := range *parsed.Models {
			if item.Name != "" {
				models = append(models, item.Name)
			}
		}
	} else {
		if parsed.Data == nil {
			return nil, fmt.Errorf("invalid model list: missing data array")
		}
		for _, item := range *parsed.Data {
			if item.ID != "" {
				models = append(models, item.ID)
			}
		}
	}
	sort.Strings(models)
	return models, nil
}

func ModelAvailable(cfg runtime.Config, models []string) bool {
	for _, model := range models {
		name := cfg.Model[strings.LastIndex(cfg.Model, "/")+1:]
		if model == cfg.Model || (cfg.Engine == runtime.EngineOllama && !strings.Contains(name, ":") && model == cfg.Model+":latest") {
			return true
		}
	}
	return false
}
