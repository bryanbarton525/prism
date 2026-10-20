package prism

import (
	"embed"
	"io/fs"
)

//go:embed evals/mcp-response-v1/suite.yaml evals/mcp-response-v2/suite.yaml
var evaluationBundle embed.FS

// EvaluationFS contains versioned model evaluation suites shipped with the binary.
func EvaluationFS() fs.FS { return evaluationBundle }
