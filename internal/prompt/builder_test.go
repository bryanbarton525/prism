package prompt

import (
	"strings"
	"testing"
)

func TestBuilderRendersStableBeforeVolatile(t *testing.T) {
	var b Builder
	got := b.Add(SectionVolatileTask, "Task", "do work").
		Add(SectionStableSystem, "System", "rules").
		Build()
	if strings.Index(got, "System") > strings.Index(got, "Task") {
		t.Fatalf("stable section rendered after volatile:\n%s", got)
	}
}

func TestBuilderSkipsEmptySectionsAndRendersTitles(t *testing.T) {
	var b Builder
	got := b.Add(SectionStableRole, "Role", "  ").
		Add(SectionStableSchema, "Schema", "{}").
		Build()
	if strings.Contains(got, "Role") {
		t.Fatalf("empty section rendered:\n%s", got)
	}
	if !strings.Contains(got, "## Schema\n\n{}") {
		t.Fatalf("title not rendered consistently:\n%s", got)
	}
}

func TestBuilderOutputIsDeterministic(t *testing.T) {
	var a Builder
	var b Builder
	one := a.Add(SectionVolatileEvidence, "Evidence", "e").
		Add(SectionStableSystem, "System", "s").
		Build()
	two := b.Add(SectionVolatileEvidence, "Evidence", "e").
		Add(SectionStableSystem, "System", "s").
		Build()
	if one != two {
		t.Fatalf("outputs differ:\n%s\n---\n%s", one, two)
	}
}
