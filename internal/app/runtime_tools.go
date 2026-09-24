package app

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"regexp"
	"strings"

	"github.com/bryanbarton525/prism/internal/agent"
	"github.com/bryanbarton525/prism/internal/plugins"
	"github.com/bryanbarton525/prism/internal/result"
	"github.com/bryanbarton525/prism/internal/skill"
	"github.com/bryanbarton525/prism/internal/textutil"
)

type runtimeEvidence struct {
	promptBlock string
	artifacts   []result.Artifact
	byteSize    int
}

func collectRuntimeEvidence(ctx context.Context, registry *plugins.Registry, spec *agent.Spec, task string) runtimeEvidence {
	var out runtimeEvidence
	for _, tool := range spec.Tools {
		// Graphify is an interactive, dedicated downstream-MCP capability. It
		// is handled by the tool loop rather than eagerly as a runtime plugin.
		if tool == "graphify" {
			continue
		}
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

		toolName, toolSpec, ok := defaultPluginTool(plugin)
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
			Args: runtimeToolArgs(task),
		}
		toolResult, err := plugin.Call(ctx, call)
		if err != nil {
			toolResult = plugins.ToolResult{
				Label:   "runtime-plugin:" + plugin.Name(),
				Content: "[error] " + err.Error(),
			}
		}
		// The ToolSpec MaxBytes contract is enforced here, not just trusted:
		// a plugin that overruns its own declared bound must not blow the
		// evidence budget.
		content := strings.TrimSpace(toolResult.Content)
		if toolSpec.MaxBytes > 0 {
			content = textutil.TruncateWithin(content, toolSpec.MaxBytes, "\n[truncated to plugin max_bytes]")
		}
		artifact := result.Artifact{
			Type:    "runtime_evidence",
			Label:   toolResult.Label,
			Content: content,
		}
		out.artifacts = append(out.artifacts, artifact)
		out.byteSize += len(artifact.Content)
		if toolResult.EvidencePack != nil {
			data, err := json.MarshalIndent(toolResult.EvidencePack, "", "  ")
			if err == nil {
				content := string(data)
				out.artifacts = append(out.artifacts, result.Artifact{
					Type:    "evidence_pack",
					Label:   "evidence-pack:" + toolResult.EvidencePack.Kind,
					Content: content,
				})
				out.byteSize += len(content)
			}
		}
		if artifact.Content != "" {
			out.promptBlock += "\n\n# Runtime Plugin Evidence: " + plugin.Name() + "\n\n```text\n" +
				artifact.Content + "\n```"
		}
	}
	return out
}

func collectSkillResourceAttachments(fsys fs.FS, attachedSkills, requested []string) (runtimeEvidence, error) {
	var out runtimeEvidence
	for _, value := range requested {
		skillName, resourcePath, ok := strings.Cut(value, ":")
		if !ok || strings.TrimSpace(skillName) == "" || strings.TrimSpace(resourcePath) == "" {
			return runtimeEvidence{}, fmt.Errorf("skill resource %q must use <skill>:<path>", value)
		}
		attached := false
		for _, name := range attachedSkills {
			if strings.EqualFold(name, skillName) {
				attached, skillName = true, name
				break
			}
		}
		if !attached {
			return runtimeEvidence{}, fmt.Errorf("skill %q is not attached to this run", skillName)
		}
		remaining := 128*1024 - out.byteSize
		if remaining <= 0 {
			return runtimeEvidence{}, fmt.Errorf("per-run skill resource budget of 131072 bytes is exhausted")
		}
		maxRead := int64(32 * 1024)
		if int64(remaining) < maxRead {
			maxRead = int64(remaining)
		}
		resource, err := skill.ReadResource(fsys, skillName, resourcePath, skill.ReadResourceOptions{MaxReadBytes: maxRead})
		if err != nil {
			return runtimeEvidence{}, err
		}
		artifact := result.Artifact{Type: "skill_resource", Label: "skill-resource:" + skillName + "/" + resource.Path, Content: resource.Content}
		out.artifacts = append(out.artifacts, artifact)
		out.byteSize += len(resource.Content)
		out.promptBlock += "\n\n# Explicit Skill Resource: " + skillName + "/" + resource.Path + "\n\n```text\n" + resource.Content + "\n```"
	}
	return out, nil
}

func runtimeToolArgs(task string) map[string]string {
	args := kubernetesArgs(task)
	args["task"] = task
	args["query"] = extractSearchQuery(task)
	return args
}

func defaultPluginTool(plugin plugins.Plugin) (string, plugins.ToolSpec, bool) {
	for _, tool := range plugin.Tools() {
		if tool.ReadOnly {
			return tool.Name, tool, true
		}
	}
	return "", plugins.ToolSpec{}, false
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
		regexp.MustCompile(`(?:^|\s)-n\s+([a-z0-9]([-a-z0-9.]*[a-z0-9])?)`),
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

func extractSearchQuery(task string) string {
	task = strings.TrimSpace(task)
	if len(task) <= 80 {
		return task
	}
	fields := strings.Fields(task)
	if len(fields) > 12 {
		fields = fields[:12]
	}
	return strings.Join(fields, " ")
}
