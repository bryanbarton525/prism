package prompt

import (
	"strings"
)

type SectionKind string

const (
	SectionStableSystem      SectionKind = "stable_system"
	SectionStableRole        SectionKind = "stable_role"
	SectionStableSchema      SectionKind = "stable_schema"
	SectionStableProjectRule SectionKind = "stable_project_rule"
	SectionVolatileTask      SectionKind = "volatile_task"
	SectionVolatileEvidence  SectionKind = "volatile_evidence"
	SectionVolatileTool      SectionKind = "volatile_tool"
)

type Section struct {
	Kind    SectionKind
	Title   string
	Content string
}

type Builder struct {
	sections []Section
}

func (b *Builder) Add(kind SectionKind, title string, content string) *Builder {
	if strings.TrimSpace(content) == "" {
		return b
	}
	b.sections = append(b.sections, Section{Kind: kind, Title: title, Content: content})
	return b
}

func (b *Builder) Build() string {
	var out []string
	for _, stable := range []bool{true, false} {
		for _, section := range b.sections {
			if isStable(section.Kind) != stable {
				continue
			}
			out = append(out, render(section))
		}
	}
	return strings.TrimSpace(strings.Join(out, "\n\n---\n\n"))
}

func isStable(kind SectionKind) bool {
	switch kind {
	case SectionStableSystem, SectionStableRole, SectionStableSchema, SectionStableProjectRule:
		return true
	default:
		return false
	}
}

func render(section Section) string {
	title := strings.TrimSpace(section.Title)
	content := strings.TrimSpace(section.Content)
	if title == "" {
		title = string(section.Kind)
	}
	return "## " + title + "\n\n" + content
}
