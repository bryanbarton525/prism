package app

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/bryanbarton525/prism/internal/agent"
	"github.com/bryanbarton525/prism/internal/plugins"
	"github.com/bryanbarton525/prism/internal/result"
)

type runtimeEvidence struct {
	promptBlock string
	artifacts   []result.Artifact
}

func collectRuntimeEvidence(ctx context.Context, registry *plugins.Registry, spec *agent.Spec, task string) runtimeEvidence {
	var out runtimeEvidence
	for _, tool := range spec.Tools {
		plugin, ok := registry.Get(tool)
		if !ok {
			msg := fmt.Sprintf("runtime tool %q is declared by the agent but is not implemented by Prism", tool)
			out.artifacts = append(out.artifacts, result.Artifact{
				Type:    "runtime_tool_status",
				Label:   "runtime-tool:" + tool,
				Content: msg,
			})
			out.promptBlock += "\n\n# Runtime Tool Evidence: " + tool + "\n\n" + msg
			continue
		}

		toolName, ok := defaultPluginTool(plugin)
		if !ok {
			msg := fmt.Sprintf("runtime plugin %q has no callable tools", plugin.Name())
			out.artifacts = append(out.artifacts, result.Artifact{
				Type:    "runtime_tool_status",
				Label:   "runtime-plugin:" + plugin.Name(),
				Content: msg,
			})
			out.promptBlock += "\n\n# Runtime Plugin Evidence: " + plugin.Name() + "\n\n" + msg
			continue
		}
		call := plugins.ToolCall{
			Tool: toolName,
			Args: kubernetesArgs(task),
		}
		toolResult, err := plugin.Call(ctx, call)
		if err != nil {
			toolResult = plugins.ToolResult{
				Label:   "runtime-plugin:" + plugin.Name(),
				Content: "[error] " + err.Error(),
			}
		}
		artifact := result.Artifact{
			Type:    "command_output",
			Label:   toolResult.Label,
			Content: strings.TrimSpace(toolResult.Content),
		}
		out.artifacts = append(out.artifacts, artifact)
		if artifact.Content != "" {
			out.promptBlock += "\n\n# Runtime Plugin Evidence: " + plugin.Name() + "\n\n```text\n" +
				artifact.Content + "\n```"
		}
	}
	return out
}

func defaultPluginTool(plugin plugins.Plugin) (string, bool) {
	for _, tool := range plugin.Tools() {
		if tool.ReadOnly {
			return tool.Name, true
		}
	}
	tools := plugin.Tools()
	if len(tools) == 0 {
		return "", false
	}
	return tools[0].Name, true
}

func kubernetesArgs(task string) map[string]string {
	args := map[string]string{}
	ns := extractKubernetesNamespace(task)
	if ns != "" {
		args["namespace"] = ns
	}
	if deploy := extractKubernetesDeployment(task); deploy != "" {
		args["deployment"] = deploy
	}
	if pod := extractKubernetesPod(task); pod != "" {
		args["pod"] = pod
	}
	return args
}

var (
	namespacePatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?i)\bnamespace\s*[:=]\s*([a-z0-9]([-a-z0-9.]*[a-z0-9])?)`),
		regexp.MustCompile(`(?i)\bnamespace\s+([a-z0-9]([-a-z0-9.]*[a-z0-9])?)`),
		regexp.MustCompile(`\s-n\s+([a-z0-9]([-a-z0-9.]*[a-z0-9])?)`),
	}
	deploymentPatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?i)\bdeployment\s*[:=]\s*([a-z0-9]([-a-z0-9.]*[a-z0-9])?)`),
		regexp.MustCompile(`(?i)\bdeploy(?:ment)?/([a-z0-9]([-a-z0-9.]*[a-z0-9])?)`),
	}
	podPatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?i)\bpod\s*[:=]\s*([a-z0-9]([-a-z0-9.]*[a-z0-9])?)`),
		regexp.MustCompile(`(?i)\bpod/([a-z0-9]([-a-z0-9.]*[a-z0-9])?)`),
	}
)

func extractKubernetesNamespace(task string) string {
	return firstRegexCapture(task, namespacePatterns)
}

func extractKubernetesDeployment(task string) string {
	return firstRegexCapture(task, deploymentPatterns)
}

func extractKubernetesPod(task string) string {
	return firstRegexCapture(task, podPatterns)
}

func firstRegexCapture(s string, patterns []*regexp.Regexp) string {
	for _, re := range patterns {
		if m := re.FindStringSubmatch(s); len(m) > 1 {
			return m[1]
		}
	}
	return ""
}
