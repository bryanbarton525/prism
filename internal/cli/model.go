package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode"

	"github.com/bryanbarton525/prism/internal/config"
	"github.com/bryanbarton525/prism/internal/extensions"
	"github.com/bryanbarton525/prism/internal/llm"
	"github.com/bryanbarton525/prism/internal/llm/runtime"
	"github.com/spf13/cobra"
)

const modelReconnectNote = "Reconnect running Prism MCP servers to load saved changes. Host-only environment overrides may differ from this terminal; inspect the Prism MCP registration in Codex or Claude if settings still differ."

type modelReport struct {
	ConfigFile      string            `json:"config_file"`
	StateDir        string            `json:"state_dir"`
	Primary         runtime.Config    `json:"primary"`
	Fallback        *runtime.Config   `json:"fallback,omitempty"`
	Sources         map[string]string `json:"sources"`
	Saved           map[string]string `json:"saved"`
	APIKeyEnv       string            `json:"api_key_env,omitempty"`
	Warnings        []string          `json:"warnings"`
	Availability    string            `json:"availability"`
	AvailableModels []string          `json:"available_models,omitempty"`
	DryRun          bool              `json:"dry_run,omitempty"`
	SavedChange     bool              `json:"saved_change,omitempty"`
	Proposed        *runtime.Config   `json:"proposed,omitempty"`
	Note            string            `json:"note"`
}

func effectivePrimary() runtime.Config {
	primary := cfg.ModelRuntime.Primary
	if primary.Engine == "" {
		primary.Engine = runtime.EngineOllama
	}
	if primary.BaseURL == "" && primary.Engine == runtime.EngineOllama {
		primary.BaseURL = gf.ollamaHost
	}
	return primary
}

func agentEffectiveModel(declared string) (runtime.Config, string) {
	target := publicRuntime(effectivePrimary())
	if target.Model == "" {
		target.Model = declared
		return target, "agent specification"
	}
	source := cfg.RuntimeSources["model"]
	if source == "" || source == "default" {
		source = "shared runtime"
	}
	return target, source
}

func publicRuntime(target runtime.Config) runtime.Config {
	return llm.PublicConfig(target)
}

func modelSources() map[string]string {
	sources := map[string]string{}
	for key, value := range cfg.RuntimeSources {
		sources[key] = value
	}
	if cfg.ModelRuntime.Primary.BaseURL == "" && (cfg.ModelRuntime.Primary.Engine == "" || cfg.ModelRuntime.Primary.Engine == runtime.EngineOllama) {
		sources["base_url"] = cfg.OllamaHostSource
		if rootCmd.PersistentFlags().Changed("ollama-host") {
			sources["base_url"] = "flag: --ollama-host"
		}
	}
	return sources
}

func modelConfigPath() string {
	if cfg.ConfigFile != "" {
		return cfg.ConfigFile
	}
	return filepath.Join(gf.stateDir, "config.env")
}

func readModelReport() (modelReport, error) {
	saved, err := config.ReadEnv(modelConfigPath())
	if err != nil {
		return modelReport{}, err
	}
	report := modelReport{ConfigFile: modelConfigPath(), StateDir: gf.stateDir, Primary: publicRuntime(effectivePrimary()), Sources: modelSources(), Saved: map[string]string{}, APIKeyEnv: cfg.RuntimeKeyEnv, Warnings: []string{}, Availability: "not checked", Note: modelReconnectNote}
	for _, key := range []string{"engine", "base_url", "model", "api_key_env", "fallback_engine", "fallback_base_url", "fallback_model"} {
		if value, ok := saved["PRISM_MODEL_RUNTIME_"+strings.ToUpper(key)]; ok {
			if strings.HasSuffix(key, "base_url") {
				value = publicRuntime(runtime.Config{BaseURL: value}).BaseURL
			}
			report.Saved[key] = value
		}
	}
	if cfg.ModelRuntime.Fallback != nil {
		fallback := publicRuntime(*cfg.ModelRuntime.Fallback)
		report.Fallback = &fallback
	}
	values := map[string]string{"engine": string(report.Primary.Engine), "base_url": report.Primary.BaseURL, "model": report.Primary.Model, "api_key_env": report.APIKeyEnv}
	for _, key := range []string{"engine", "base_url", "model", "api_key_env"} {
		if value, ok := report.Saved[key]; ok && value != values[key] {
			report.Warnings = append(report.Warnings, fmt.Sprintf("Saved %s=%q is overridden by %s (effective %q). Update or unset that override to use the saved value.", key, value, report.Sources[key], values[key]))
		}
	}
	return report, nil
}

func checkModel(ctx context.Context, primary runtime.Config) ([]string, error) {
	models, err := llm.ListModels(ctx, primary)
	if err != nil {
		return nil, err
	}
	if primary.Model != "" && !llm.ModelAvailable(primary, models) {
		return models, fmt.Errorf("model %q is not advertised by %s; available: %s; select an advertised ID or use --skip-check for an intentional unverified target", primary.Model, primary.BaseURL, strings.Join(models, ", "))
	}
	return models, nil
}

func outputModelReport(cmd *cobra.Command, report modelReport) error {
	if gf.jsonOut {
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		return enc.Encode(report)
	}
	out := cmd.OutOrStdout()
	if report.DryRun {
		fmt.Fprintln(out, "Preview only; no configuration saved.")
	}
	if report.SavedChange {
		fmt.Fprintln(out, "Saved shared offload model configuration.")
	}
	fmt.Fprintf(out, "Config: %s\nRuntime state: %s\n", report.ConfigFile, report.StateDir)
	for _, field := range []struct{ key, label, value string }{{"engine", "Engine", string(report.Primary.Engine)}, {"base_url", "Endpoint", report.Primary.BaseURL}, {"model", "Model", report.Primary.Model}} {
		value := field.value
		if value == "" {
			value = "not configured"
			if field.key == "model" {
				value = "agent-declared model (no shared override)"
			}
		}
		source := report.Sources[field.key]
		if source == "" {
			source = "default"
		}
		fmt.Fprintf(out, "%s: %s [%s]\n", field.label, value, source)
	}
	if report.APIKeyEnv != "" {
		fmt.Fprintf(out, "API key environment variable: %s (value hidden)\n", report.APIKeyEnv)
	}
	if report.Fallback != nil {
		fmt.Fprintf(out, "Fallback: %s %s model=%s\n", report.Fallback.Engine, report.Fallback.BaseURL, report.Fallback.Model)
	}
	if report.Proposed != nil {
		fmt.Fprintf(out, "Selected target: %s %s model=%s\n", report.Proposed.Engine, report.Proposed.BaseURL, report.Proposed.Model)
	}
	fmt.Fprintf(out, "Availability: %s\n", report.Availability)
	for _, warning := range report.Warnings {
		fmt.Fprintln(out, "Warning:", warning)
	}
	fmt.Fprintln(out, report.Note)
	return nil
}

func newModelCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "model", Short: "Select and inspect the shared offload model (Ollama, SGLang, vLLM)", Long: "Manage the shared offload model. For tool embedding models, use `prism models`."}
	var check bool
	status := &cobra.Command{Use: "status", Short: "Show effective and saved runtime settings and their sources", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		report, err := readModelReport()
		if err != nil {
			return err
		}
		if check {
			report.AvailableModels, err = checkModel(cmd.Context(), effectivePrimary())
			report.Availability = "advertised (inference not tested)"
			if err != nil {
				report.Availability = err.Error()
			} else if report.Primary.Model == "" {
				report.Availability = "server model list reachable; no shared model selected (per-agent models not checked)"
			}
		}
		if managedErr := addManagedModelWarnings(&report, effectivePrimary()); managedErr != nil {
			return managedErr
		}
		if outErr := outputModelReport(cmd, report); outErr != nil {
			return outErr
		}
		return err
	}}
	status.Flags().BoolVar(&check, "check", false, "Check server model availability without generating text")
	cmd.AddCommand(status, newModelSetCmd())
	return cmd
}

type modelSetFlags struct {
	engine, url, keyEnv string
	dryRun, skipCheck   bool
}

func newModelSetCmd() *cobra.Command {
	var flags modelSetFlags
	cmd := &cobra.Command{Use: "set [model]", Short: "Save a shared model; omit the model to open the keyboard picker", Args: cobra.MaximumNArgs(1), Example: "  prism model set\n  prism model set qwen3.6:27b --engine ollama\n  prism model set my-model --engine sglang --url http://server:30000/v1", RunE: func(cmd *cobra.Command, args []string) error { return runModelSet(cmd, args, flags) }}
	cmd.Flags().StringVar(&flags.engine, "engine", "", "Serving engine: ollama, sglang, vllm (default: current engine)")
	cmd.Flags().StringVar(&flags.url, "url", "", "Serving endpoint (default: current endpoint; required when switching to SGLang/vLLM)")
	cmd.Flags().StringVar(&flags.keyEnv, "api-key-env", "", "Environment variable containing the API key; use an empty value to clear")
	cmd.Flags().BoolVar(&flags.dryRun, "dry-run", false, "Preview configuration without writing; explicit model does not contact the server")
	cmd.Flags().BoolVar(&flags.skipCheck, "skip-check", false, "Save an unverified model without contacting the server")
	return cmd
}

func selectModelTarget(cmd *cobra.Command, model string, flags modelSetFlags) (runtime.Config, string, error) {
	current := effectivePrimary()
	target, keyEnv := current, cfg.RuntimeKeyEnv
	if flags.engine != "" {
		target.Engine = runtime.Engine(flags.engine)
	}
	if flags.url != "" {
		target.BaseURL = strings.TrimRight(strings.TrimSpace(flags.url), "/")
	} else if target.Engine != current.Engine {
		if target.Engine != runtime.EngineOllama {
			return target, "", fmt.Errorf("switching to %s requires --url (example: prism model set %s --engine %s --url http://server:30000/v1)", target.Engine, model, target.Engine)
		}
		target.BaseURL = gf.ollamaHost
	}
	if target.Engine != current.Engine || target.BaseURL != current.BaseURL {
		target.APIKey, target.Headers, keyEnv = "", nil, ""
	}
	if cmd.Flags().Changed("api-key-env") {
		keyEnv = flags.keyEnv
		if keyEnv != "" && !installEnvName.MatchString(keyEnv) {
			return target, "", fmt.Errorf("invalid API key environment variable name %q", keyEnv)
		}
		target.APIKey = os.Getenv(keyEnv)
	}
	target.Model = strings.TrimSpace(model)
	if target.Model == "" || strings.IndexFunc(target.Model, unicode.IsControl) >= 0 {
		return target, "", fmt.Errorf("model ID must be nonempty and contain no control characters")
	}
	return target, keyEnv, llm.ValidateModelTarget(target)
}

func runModelSet(cmd *cobra.Command, args []string, flags modelSetFlags) error {
	path, err := filepath.Abs(filepath.Join(gf.stateDir, "config.env"))
	if err != nil {
		return err
	}
	configured, err := filepath.Abs(modelConfigPath())
	if err != nil {
		return err
	}
	if configured != path {
		return fmt.Errorf("configuration points to %s, but selected runtime uses %s; align PRISM_CONFIG_FILE and --state-dir before saving", configured, path)
	}
	var target runtime.Config
	var keyEnv string
	if len(args) == 0 {
		if gf.jsonOut {
			return fmt.Errorf("--json cannot open a picker; use prism model set <model> --engine ollama")
		}
		target, keyEnv, err = pickModelTarget(cmd, flags)
	} else {
		target, keyEnv, err = selectModelTarget(cmd, args[0], flags)
	}
	if err != nil {
		return err
	}
	if keyEnv != "" && os.Getenv(keyEnv) == "" && !flags.skipCheck && !flags.dryRun {
		return fmt.Errorf("API key environment variable %s is unset; set it or use --skip-check for offline setup", keyEnv)
	}
	report, err := readModelReport()
	if err != nil {
		return err
	}
	public := publicRuntime(target)
	report.Proposed = &public
	report.DryRun = flags.dryRun
	report.Availability = "unverified (check skipped)"
	if !flags.skipCheck && !flags.dryRun {
		report.AvailableModels, err = checkModel(cmd.Context(), target)
		if err != nil {
			return fmt.Errorf("configuration not saved: %w", err)
		}
		report.Availability = "advertised (inference not tested)"
	}
	settings := map[string]string{"PRISM_MODEL_RUNTIME_ENGINE": string(target.Engine), "PRISM_MODEL_RUNTIME_BASE_URL": target.BaseURL, "PRISM_MODEL_RUNTIME_MODEL": target.Model, "PRISM_MODEL_RUNTIME_API_KEY_ENV": keyEnv}
	// Never carry a credential from a different saved or effective endpoint.
	savedEngineChanged := report.Saved["engine"] != "" && report.Saved["engine"] != string(target.Engine)
	savedURLChanged := report.Saved["base_url"] != "" && report.Saved["base_url"] != target.BaseURL
	if savedEngineChanged || savedURLChanged || effectivePrimary().BaseURL != target.BaseURL || effectivePrimary().Engine != target.Engine || cmd.Flags().Changed("api-key-env") {
		settings["PRISM_MODEL_RUNTIME_API_KEY"] = ""
	}
	if err := addManagedModelWarnings(&report, target); err != nil {
		return err
	}
	if !flags.dryRun {
		if err := config.WriteEnv(cmd.Context(), gf.stateDir, settings); err != nil {
			return err
		}
		loaded, err := config.LoadForStateDir(gf.stateDir)
		if err != nil {
			return err
		}
		cfg = loaded
		reloaded, err := readModelReport()
		if err != nil {
			return err
		}
		reloaded.SavedChange, reloaded.Proposed, reloaded.Availability, reloaded.AvailableModels = true, &public, report.Availability, report.AvailableModels
		report = reloaded
	}
	if report.Primary.Engine != target.Engine || report.Primary.BaseURL != target.BaseURL || report.Primary.Model != target.Model {
		report.Warnings = append(report.Warnings, "Selected target differs from the effective settings shown above. Environment/.env overrides may remain active; saving does not override them.")
	}
	if target.APIKey != effectivePrimary().APIKey {
		report.Warnings = append(report.Warnings, "Effective authentication differs from the selected target's validation credentials; set --api-key-env explicitly and inspect PRISM_MODEL_RUNTIME_API_KEY and PRISM_MODEL_RUNTIME_API_KEY_ENV overrides.")
	}
	if !flags.dryRun {
		if err := addManagedModelWarnings(&report, target); err != nil {
			report.Warnings = append(report.Warnings, "Configuration saved, but managed-agent diagnostics could not be refreshed: "+err.Error())
		}
	}
	return outputModelReport(cmd, report)
}

func runtimeTargetConflicts(recorded *extensions.RuntimeTarget, target runtime.Config) bool {
	return recorded != nil && (!strings.EqualFold(strings.TrimSpace(recorded.Engine), string(target.Engine)) || strings.TrimSpace(recorded.BaseURL) != target.BaseURL || (target.Model != "" && strings.TrimSpace(recorded.Model) != target.Model))
}

func addManagedModelWarnings(report *modelReport, target runtime.Config) error {
	manifest, err := extensions.NewStore(gf.stateDir).LoadManifest()
	if err != nil {
		return err
	}
	for _, entry := range manifest.Entries {
		if entry.Kind == "agent" && runtimeTargetConflicts(entry.Runtime, target) {
			report.Warnings = append(report.Warnings, fmt.Sprintf("Managed agent %q records a different runtime target; reconcile before execution with: prism --state-dir %q agent model set %s --use-configured-model", entry.Identity, gf.stateDir, entry.Identity))
		}
	}
	return nil
}
