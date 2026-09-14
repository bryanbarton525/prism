package graphify

import (
	"encoding/json"
	"fmt"
)

const (
	PinnedPackageRequirement = "graphifyy[mcp]==0.9.61"
	PinnedPythonRequirement  = ">=3.10"
	PinnedMCPEntypoint       = "graphify-mcp"
)

type ReleaseMetadata struct {
	MetadataVersion int `json:"metadata_version"`
	Upstream        struct {
		Repository           string   `json:"repository"`
		Tag                  string   `json:"tag"`
		Commit               string   `json:"commit"`
		PythonPackage        string   `json:"python_package"`
		PythonRequires       string   `json:"python_requires"`
		MCPExtraRequirements []string `json:"mcp_extra_requirements"`
		MCPEntrypoint        string   `json:"mcp_entrypoint"`
		MCPLaunch            struct {
			Arguments []string `json:"arguments"`
		} `json:"mcp_launch"`
		CodeIndexing struct {
			Command []string `json:"command"`
			Output  string   `json:"output"`
		} `json:"code_indexing"`
		ReleaseAsset struct {
			Name   string `json:"name"`
			SHA256 string `json:"sha256"`
		} `json:"release_asset"`
		License string `json:"license"`
	} `json:"upstream"`
	Contract struct {
		ID string `json:"id"`
	} `json:"contract"`
	DeliberatelyUnsupported []string `json:"deliberately_unsupported"`
}

func ValidateReleaseMetadata(data []byte) error {
	var metadata ReleaseMetadata
	if err := json.Unmarshal(data, &metadata); err != nil {
		return fmt.Errorf("decode Graphify release metadata: %w", err)
	}
	if metadata.MetadataVersion != 1 {
		return fmt.Errorf("Graphify release metadata version must be 1")
	}
	if metadata.Upstream.Repository != "https://github.com/Graphify-Labs/graphify" {
		return fmt.Errorf("Graphify release metadata has an unexpected upstream repository")
	}
	if metadata.Upstream.Tag != PinnedUpstreamVersion || metadata.Upstream.Commit != PinnedUpstreamCommit {
		return fmt.Errorf("Graphify release metadata must pin %s (%s)", PinnedUpstreamVersion, PinnedUpstreamCommit)
	}
	if metadata.Upstream.PythonPackage != PinnedPackageRequirement || metadata.Upstream.PythonRequires != PinnedPythonRequirement {
		return fmt.Errorf("Graphify release metadata has an unexpected Python dependency pin")
	}
	if metadata.Upstream.MCPEntrypoint != PinnedMCPEntypoint {
		return fmt.Errorf("Graphify release metadata must identify %q as the MCP entry point", PinnedMCPEntypoint)
	}
	if !equalStrings(metadata.Upstream.MCPLaunch.Arguments, []string{"--graph", "<absolute-workspace>/graphify-out/graph.json"}) {
		return fmt.Errorf("Graphify release metadata has an unexpected MCP launch contract")
	}
	if !equalStrings(metadata.Upstream.CodeIndexing.Command, []string{"graphify", "extract", "<absolute-workspace>", "--code-only", "--no-viz"}) ||
		metadata.Upstream.CodeIndexing.Output != "<absolute-workspace>/graphify-out/graph.json" {
		return fmt.Errorf("Graphify release metadata has an unexpected deterministic code-index contract")
	}
	if len(metadata.Upstream.MCPExtraRequirements) != 2 ||
		metadata.Upstream.MCPExtraRequirements[0] != "mcp>=1,<3" ||
		metadata.Upstream.MCPExtraRequirements[1] != "starlette>=1.3.1,<2" {
		return fmt.Errorf("Graphify release metadata has an unexpected MCP extra contract")
	}
	if metadata.Upstream.ReleaseAsset.Name != "graphify-self-graph.tar.gz" ||
		metadata.Upstream.ReleaseAsset.SHA256 != "1e48264a79b8ab7e7a5e99d779fbf6878d5f3afba9a83fb50c68b341120e74a6" {
		return fmt.Errorf("Graphify release metadata has an unexpected release asset digest")
	}
	if metadata.Upstream.License != "Apache-2.0" {
		return fmt.Errorf("Graphify release metadata must retain the Apache-2.0 notice")
	}
	if metadata.Contract.ID != PinnedContractID {
		return fmt.Errorf("Graphify release metadata must identify contract %q", PinnedContractID)
	}
	if len(metadata.DeliberatelyUnsupported) == 0 {
		return fmt.Errorf("Graphify release metadata must disclose unsupported dependency operations")
	}
	return nil
}

func equalStrings(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range want {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}
