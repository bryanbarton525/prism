package eval

import (
	"bytes"
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
)

// Export creates a local comparison dataset, preserving failed trials and raw
// request/response traces. No credentials are stored. An explicitly selected Laya origin is recorded.
func Export(r Report, dir string) error {
	if err := ValidateReport(r); err != nil {
		return err
	}
	if err := os.Mkdir(dir, 0700); err != nil {
		return err
	}
	files := map[string][]byte{}
	var trials bytes.Buffer
	enc := json.NewEncoder(&trials)
	for _, t := range r.Trials {
		if err := enc.Encode(t); err != nil {
			return err
		}
	}
	files["trials.jsonl"] = trials.Bytes()
	var metrics bytes.Buffer
	metricEncoder := json.NewEncoder(&metrics)
	for _, trial := range r.Trials {
		for _, metric := range trial.Grade.Metrics {
			if err := metricEncoder.Encode(map[string]any{"model": trial.Model, "case_id": trial.CaseID, "repetition": trial.Repetition, "metric": metric}); err != nil {
				return err
			}
		}
	}
	files["metrics.jsonl"] = metrics.Bytes()
	suite, err := json.MarshalIndent(r.Suite, "", "  ")
	if err != nil {
		return err
	}
	files["suite.json"] = suite
	var summary bytes.Buffer
	w := csv.NewWriter(&summary)
	_ = w.Write([]string{"model", "case", "trials", "passed", "pass_rate", "semantic_consistency"})
	for _, model := range r.Models {
		scores := Scores(r, model)
		for _, c := range r.Suite.Cases {
			s := scores[c.ID]
			_ = w.Write([]string{model, c.ID, strconv.Itoa(s.Trials), strconv.Itoa(s.Passed), fmt.Sprint(s.PassRate), fmt.Sprint(s.Consistency)})
		}
	}
	w.Flush()
	if err := w.Error(); err != nil {
		return err
	}
	files["summary.csv"] = summary.Bytes()
	metadata := r
	metadata.Trials = nil
	meta, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return err
	}
	files["metadata.json"] = meta
	files["README.md"] = []byte(fmt.Sprintf("# %s model comparison\n\nSynthetic MCP replay evaluation. Suite license: %s. Model output redistribution remains subject to each provider/model's terms.\n\nAll planned trials, including failures, are retained. Models receive identical fixed downstream MCP evidence; production Prism prompts and tool loops are used. Grader: %s. Semantic consistency measures agreement on declared JSON decision fields, not phrasing. Correctness is scored separately. No statistical significance is claimed by these small samples.\n\nReview traces for sensitive model-generated material before publishing. No external publication is performed by this command.\n", r.Suite.ID, r.Suite.License, r.Grader))
	checksums := map[string]string{}
	for name, data := range files {
		sum := sha256.Sum256(data)
		checksums[name] = hex.EncodeToString(sum[:])
		if err := os.WriteFile(filepath.Join(dir, name), data, 0600); err != nil {
			return err
		}
	}
	return WriteJSON(filepath.Join(dir, "manifest.json"), checksums)
}
