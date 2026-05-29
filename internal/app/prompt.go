package app

import (
	"fmt"
	"strings"

	"github.com/bryanbarton525/prism/internal/skill"
)

// assemblePrompt builds the system and user messages following the progressive
// disclosure order described in the implementation plan:
//
//  1. Constitution — the agent's identity, mission, and operating boundaries.
//  2. Skill index  — a compact metadata table listing each attached skill with
//     its name and description (helps the model understand scope without
//     loading the full body yet).
//  3. Skill bodies — the full SKILL.md content for each attached skill, in the
//     order specified by skillNames.
//
// The task supplied by the orchestrator becomes the user message so that
// model conversation history handling works correctly with Ollama.
//
// skillNames is used to control iteration order so the prompt is deterministic
// (maps in Go have non-deterministic iteration order).
func assemblePrompt(constitution string, skills map[string]*skill.Skill, skillNames []string, task string) (systemMsg, userMsg string) {
	var sb strings.Builder

	// ── Phase 1: Constitution ─────────────────────────────────────────────
	if constitution != "" {
		sb.WriteString(constitution)
		sb.WriteString("\n\n")
	}

	// ── Phase 2: Skill index (progressive disclosure — metadata only) ─────
	if len(skills) > 0 {
		sb.WriteString("# Attached Agent Skills\n\n")
		sb.WriteString("The following skills are attached to this invocation. ")
		sb.WriteString("Use only the procedures and tool references described within them.\n\n")
		for _, name := range skillNames {
			if sk, ok := skills[name]; ok {
				sb.WriteString(sk.MetadataSummary())
				sb.WriteString("\n")
			}
		}
		sb.WriteString("\n")

		// ── Phase 3: Full skill bodies ─────────────────────────────────────
		for _, name := range skillNames {
			if sk, ok := skills[name]; ok {
				sb.WriteString(sk.FullText())
				sb.WriteString("\n\n")
			}
		}
	}

	// ── Scope reminder ────────────────────────────────────────────────────
	if len(skills) > 0 {
		sb.WriteString(fmt.Sprintf(
			"You must only perform work covered by the %d attached skill(s) listed above. "+
				"Refuse any request that falls outside these skills or your constitution.\n\n",
			len(skills),
		))
	}

	systemMsg = strings.TrimSpace(sb.String())
	userMsg = strings.TrimSpace(task)
	return systemMsg, userMsg
}

// truncateToTokenBudget trims text to fit within an approximate token budget
// using the heuristic of 4 characters per token. It appends a notice so the
// model knows the context was reduced.
func truncateToTokenBudget(text string, tokenBudget int) string {
	if tokenBudget <= 0 {
		return text
	}
	charBudget := tokenBudget * 4
	if len(text) <= charBudget {
		return text
	}
	const notice = "\n\n[System: prompt truncated to fit context budget]"
	if charBudget <= len(notice) {
		return notice
	}
	return text[:charBudget-len(notice)] + notice
}

// AssemblePromptForTest exposes assemblePrompt for golden tests in internal/benchmark.
func AssemblePromptForTest(constitution string, skills map[string]*skill.Skill, skillNames []string, task string) (string, string) {
	return assemblePrompt(constitution, skills, skillNames, task)
}
