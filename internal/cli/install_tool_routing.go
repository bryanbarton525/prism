package cli

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/bryanbarton525/prism/internal/fileatomic"
	"github.com/bryanbarton525/prism/internal/filelock"
	"github.com/bryanbarton525/prism/internal/llm"
	"github.com/bryanbarton525/prism/internal/llm/runtime"
	"github.com/bryanbarton525/prism/internal/toolmodel"
)

var installEnvName = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

func promptGuidedToolRouting(input *guidedInput, flags *installFlags) error {
	fmt.Fprintln(input.cmd.OutOrStdout(), "Optional local tool recommendations can rank MCP tools for selected agents.")
	model, err := promptChoice(input, "Tool model [none/potion/onnx] (none): ", "none", []string{"none", "potion", "onnx"})
	if err != nil {
		return err
	}
	if model != "none" {
		flags.toolModel = model
		if len(flags.specialists) > 0 {
			fmt.Fprintf(input.cmd.OutOrStdout(), "Selected specialist IDs: %s\n", strings.Join(flags.specialists, ", "))
		}
		if flags.runtimeAgentAs != "" {
			fmt.Fprintf(input.cmd.OutOrStdout(), "Selected managed runtime agent ID: %s\n", flags.runtimeAgentAs)
		}
		answer, err := input.answer("Opt-in agent identities (comma-separated; blank leaves disabled): ")
		if err != nil {
			return err
		}
		for _, item := range strings.Split(answer, ",") {
			if name := strings.TrimSpace(item); name != "" {
				flags.toolRecommendAgents = append(flags.toolRecommendAgents, name)
			}
		}
	}
	engine, err := promptChoice(input, "Primary LLM endpoint [none/ollama/sglang/vllm] (none): ", "none", []string{"none", "ollama", "sglang", "vllm"})
	if err != nil {
		return err
	}
	if engine != "none" {
		flags.primaryEngine = engine
		if flags.primaryURL, err = input.answer("Primary endpoint URL: "); err != nil {
			return err
		}
		if flags.primaryModel, err = input.answer("Primary model name: "); err != nil {
			return err
		}
		if flags.primaryAPIKeyEnv, err = input.answer("API key environment variable (blank if none): "); err != nil {
			return err
		}
	}
	service, err := promptChoice(input, "Decision service [none/local/jev/install] (none): ", "none", []string{"none", "local", "jev", "install"})
	if err != nil {
		return err
	}
	if service != "none" {
		flags.decisionService = service
		if service == "install" {
			flags.kevDevice, err = promptChoice(input, "Kev device [cpu/auto] (cpu): ", "cpu", []string{"cpu", "auto"})
			if err != nil {
				return err
			}
			port, answerErr := input.answer("Kev loopback port (8009): ")
			if answerErr != nil {
				return answerErr
			}
			if port != "" {
				flags.kevPort, err = strconv.Atoi(port)
				if err != nil {
					return fmt.Errorf("invalid Kev port %q", port)
				}
			} else {
				flags.kevPort = 8009
			}
			flags.decisionURL = fmt.Sprintf("http://127.0.0.1:%d", flags.kevPort)
			flags.decisionModel = "kev-latest"
			return nil
		}
		if flags.decisionURL, err = input.answer("Decision service URL: "); err != nil {
			return err
		}
		if flags.decisionKeyEnv, err = input.answer("API key environment variable (blank if none): "); err != nil {
			return err
		}
		if service == "jev" {
			flags.decisionModel = "jev-latest"
		} else {
			flags.decisionModel = "kev-latest"
		}
	}
	return nil
}

func validateInstallToolRouting(flags installFlags) error {
	if flags.toolModel != "" && flags.toolModel != "potion" && flags.toolModel != "onnx" {
		return fmt.Errorf("--tool-model must be potion or onnx")
	}
	if len(flags.toolRecommendAgents) > 0 && flags.toolModel == "" {
		return fmt.Errorf("--tool-recommend-agent requires --tool-model")
	}
	if flags.primaryEngine == "" {
		if flags.primaryURL != "" || flags.primaryModel != "" || flags.primaryAPIKeyEnv != "" {
			return fmt.Errorf("primary runtime settings require --primary-engine")
		}
	} else {
		if flags.primaryEngine != "ollama" && flags.primaryEngine != "sglang" && flags.primaryEngine != "vllm" {
			return fmt.Errorf("unsupported primary engine %q", flags.primaryEngine)
		}
		if flags.primaryURL == "" || flags.primaryModel == "" {
			return fmt.Errorf("--primary-engine requires --primary-url and --primary-model")
		}
		parsed, err := url.Parse(flags.primaryURL)
		if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Hostname() == "" || parsed.User != nil || parsed.Fragment != "" || parsed.RawQuery != "" {
			return fmt.Errorf("--primary-url must be an HTTP(S) endpoint without credentials or a fragment")
		}
	}
	for _, name := range []string{flags.primaryAPIKeyEnv, flags.decisionKeyEnv} {
		if name != "" && !installEnvName.MatchString(name) {
			return fmt.Errorf("invalid API key environment variable name %q", name)
		}
	}
	if flags.decisionService == "" {
		if flags.decisionURL != "" || flags.decisionKeyEnv != "" || flags.decisionModel != "" {
			return fmt.Errorf("decision settings require --decision-service")
		}
		return nil
	}
	if flags.decisionService == "install" {
		if flags.kevDevice != "" && flags.kevDevice != "cpu" && flags.kevDevice != "auto" {
			return fmt.Errorf("--kev-device must be cpu or auto")
		}
		if flags.kevPort < 1024 || flags.kevPort > 65535 {
			return fmt.Errorf("--kev-port must be between 1024 and 65535")
		}
		if flags.decisionModel != "" && flags.decisionModel != "kev-latest" {
			return fmt.Errorf("managed Kev requires --decision-model kev-latest")
		}
		if flags.decisionKeyEnv != "" {
			return fmt.Errorf("managed Kev does not support --decision-key-env")
		}
		if flags.decisionURL != "" && flags.decisionURL != fmt.Sprintf("http://127.0.0.1:%d", flags.kevPort) {
			return fmt.Errorf("managed Kev URL must match loopback --kev-port")
		}
		return nil
	}
	if flags.decisionService != "local" && flags.decisionService != "jev" {
		return fmt.Errorf("--decision-service must be local, jev, or install")
	}
	if flags.decisionURL == "" {
		return fmt.Errorf("--decision-service requires --decision-url")
	}
	model := decisionModel(flags)
	if _, err := toolmodel.NewDecisionClient(flags.decisionURL, flags.decisionKeyEnv, model); err != nil {
		return err
	}
	parsed, _ := url.Parse(flags.decisionURL)
	if flags.decisionService == "jev" && parsed.Scheme != "https" {
		return fmt.Errorf("remote Jev requires HTTPS")
	}
	if flags.decisionService == "local" && parsed.Hostname() != "localhost" && !net.ParseIP(parsed.Hostname()).IsLoopback() {
		return fmt.Errorf("local Kev requires a loopback URL")
	}
	return nil
}

func decisionModel(flags installFlags) string {
	if flags.decisionModel != "" {
		return flags.decisionModel
	}
	if flags.decisionService == "jev" {
		return "jev-latest"
	}
	return "kev-latest"
}

func firstNonEmptyInstall(value, fallback string) string {
	if value != "" {
		return value
	}
	return fallback
}

func printToolRoutingPlan(cmd *cobra.Command, flags installFlags, stateDir string) {
	if flags.toolModel == "" && flags.primaryEngine == "" && flags.decisionService == "" {
		return
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Tool routing configuration: %s/config.env\n", stateDir)
	if flags.toolModel != "" {
		fmt.Fprintf(cmd.OutOrStdout(), "  model: %s (download/verify if absent); opt-in agents: %s\n", flags.toolModel, strings.Join(flags.toolRecommendAgents, ", "))
	}
	if flags.primaryEngine != "" {
		fmt.Fprintf(cmd.OutOrStdout(), "  primary: %s %s model=%s (health probe); key env=%s\n", flags.primaryEngine, flags.primaryURL, flags.primaryModel, flags.primaryAPIKeyEnv)
	}
	if flags.decisionService != "" {
		if flags.decisionService == "install" {
			fmt.Fprintf(cmd.OutOrStdout(), "  decision: install pinned Kev source, Python dependencies, 0.8B model, and persistent systemd user service at http://127.0.0.1:%d (device=%s; downloads and one inference probe)\n", flags.kevPort, firstNonEmptyInstall(flags.kevDevice, "cpu"))
		} else {
			fmt.Fprintf(cmd.OutOrStdout(), "  decision: %s %s model=%s (one inference probe); key env=%s\n", flags.decisionService, flags.decisionURL, decisionModel(flags), flags.decisionKeyEnv)
		}
	}
}

func prepareInstallToolRouting(cmd *cobra.Command, flags installFlags, stateDir string) error {
	if flags.toolModel != "" || flags.primaryEngine != "" || flags.decisionService != "" {
		if override := os.Getenv("PRISM_CONFIG_FILE"); override != "" {
			selected, err := filepath.Abs(filepath.Join(stateDir, "config.env"))
			if err != nil {
				return err
			}
			configured, err := filepath.Abs(override)
			if err != nil {
				return err
			}
			if configured != selected {
				return fmt.Errorf("PRISM_CONFIG_FILE points to %s, but installer selected %s; align them before setup", configured, selected)
			}
		}
	}
	baseContext := cmd.Context()
	if baseContext == nil {
		baseContext = context.Background()
	}
	for _, name := range []string{flags.primaryAPIKeyEnv, flags.decisionKeyEnv} {
		if name != "" && os.Getenv(name) == "" {
			return fmt.Errorf("API key environment variable %s is unset", name)
		}
	}
	if flags.primaryEngine != "" {
		cfg := runtime.Config{Engine: runtime.Engine(flags.primaryEngine), BaseURL: flags.primaryURL, Model: flags.primaryModel, APIKey: os.Getenv(flags.primaryAPIKeyEnv)}
		client, err := llm.NewRuntime(cfg)
		if err != nil {
			return fmt.Errorf("primary runtime: %w", err)
		}
		ctx, cancel := context.WithTimeout(baseContext, 8*time.Second)
		defer cancel()
		status, err := client.Health(ctx)
		if err != nil {
			return fmt.Errorf("primary runtime health: %w", err)
		}
		if status == nil || !status.Healthy {
			return fmt.Errorf("primary runtime is not healthy")
		}
	}
	if flags.toolModel == "potion" {
		if err := toolmodel.SetupPotion(baseContext, stateDir); err != nil {
			return err
		}
	}
	if flags.toolModel == "onnx" {
		if err := toolmodel.SetupMiniLM(baseContext, stateDir); err != nil {
			return err
		}
	}
	if flags.decisionService == "install" {
		if err := installManagedKev(baseContext, stateDir, flags.kevPort, flags.kevUV, firstNonEmptyInstall(flags.kevDevice, "cpu")); err != nil {
			return err
		}
	} else if flags.decisionService != "" {
		client, _ := toolmodel.NewDecisionClient(flags.decisionURL, flags.decisionKeyEnv, decisionModel(flags))
		ctx, cancel := context.WithTimeout(baseContext, 8*time.Second)
		defer cancel()
		if _, err := client.Score(ctx, "Choose a tool for checking service health", []toolmodel.KevTool{{Name: "health", Description: "Check the health of a service"}}); err != nil {
			return fmt.Errorf("decision service probe: %w", err)
		}
	}
	return nil
}

func saveInstallToolRouting(flags installFlags, stateDir string) error {
	settings := map[string]string{}
	if flags.toolModel != "" {
		settings["PRISM_TOOL_RECOMMEND_MODEL"] = flags.toolModel
		settings["PRISM_TOOL_RECOMMEND_AGENTS"] = strings.Join(flags.toolRecommendAgents, ",")
	}
	if flags.primaryEngine != "" {
		settings["PRISM_MODEL_RUNTIME_ENGINE"] = flags.primaryEngine
		settings["PRISM_MODEL_RUNTIME_BASE_URL"] = flags.primaryURL
		settings["PRISM_MODEL_RUNTIME_MODEL"] = flags.primaryModel
		settings["PRISM_MODEL_RUNTIME_API_KEY_ENV"] = flags.primaryAPIKeyEnv
	}
	if flags.decisionService != "" {
		decisionURL := flags.decisionURL
		if flags.decisionService == "install" {
			decisionURL = fmt.Sprintf("http://127.0.0.1:%d", flags.kevPort)
		}
		settings["PRISM_KEV_URL"] = decisionURL
		settings["PRISM_KEV_API_KEY_ENV"] = flags.decisionKeyEnv
		settings["PRISM_KEV_MODEL"] = decisionModel(flags)
	}
	if len(settings) == 0 {
		return nil
	}
	if err := os.MkdirAll(stateDir, 0o700); err != nil {
		return err
	}
	unlock, err := filelock.Acquire(context.Background(), filepath.Join(stateDir, "runtime-config.lock"), 5*time.Second)
	if err != nil {
		return err
	}
	defer unlock()
	path := filepath.Join(stateDir, "config.env")
	data, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	lines := strings.Split(strings.TrimSuffix(string(data), "\n"), "\n")
	if len(data) == 0 {
		lines = nil
	}
	seen := map[string]bool{}
	for i, line := range lines {
		key, _, ok := strings.Cut(strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "export ")), "=")
		key = strings.TrimSpace(key)
		if !ok || strings.HasPrefix(strings.TrimSpace(line), "#") {
			continue
		}
		if value, found := settings[key]; found {
			lines[i] = key + "=" + strconv.Quote(value)
			seen[key] = true
		}
	}
	for _, key := range []string{"PRISM_TOOL_RECOMMEND_MODEL", "PRISM_TOOL_RECOMMEND_AGENTS", "PRISM_MODEL_RUNTIME_ENGINE", "PRISM_MODEL_RUNTIME_BASE_URL", "PRISM_MODEL_RUNTIME_MODEL", "PRISM_MODEL_RUNTIME_API_KEY_ENV", "PRISM_KEV_URL", "PRISM_KEV_API_KEY_ENV", "PRISM_KEV_MODEL"} {
		if value, ok := settings[key]; ok && !seen[key] {
			lines = append(lines, key+"="+strconv.Quote(value))
		}
	}
	tmp, err := os.CreateTemp(stateDir, ".config-env-")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return err
	}
	if _, err := tmp.WriteString(strings.Join(lines, "\n") + "\n"); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return fileatomic.Replace(tmp.Name(), path)
}
