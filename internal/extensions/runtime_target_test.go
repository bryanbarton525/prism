package extensions

import "testing"

func TestRuntimeTargetValidate(t *testing.T) {
	if err := (RuntimeTarget{}).Validate(); err == nil {
		t.Fatal("expected validation error")
	}
	if err := (RuntimeTarget{Engine: "ollama", BaseURL: "http://localhost:11434", Model: "llama"}).Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestRuntimeTargetDrifted(t *testing.T) {
	a := RuntimeTarget{Engine: "ollama", BaseURL: "http://localhost:11434", Model: "llama3"}
	b := RuntimeTarget{Engine: "ollama", BaseURL: "http://localhost:11434", Model: "llama3"}
	if a.Drifted(b) {
		t.Fatal("expected no drift")
	}
	b.Model = "llama3.1"
	if !a.Drifted(b) {
		t.Fatal("expected drift")
	}
}
