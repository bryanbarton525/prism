package eval

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"sort"
)

type Score struct {
	Trials      int     `json:"trials"`
	Passed      int     `json:"passed"`
	PassRate    float64 `json:"pass_rate"`
	Consistency float64 `json:"consistency"`
}
type Baseline struct {
	ControlMode   string           `json:"control_mode,omitempty"`
	LayaMode      string           `json:"laya_mode,omitempty"`
	LayaEnabled   bool             `json:"laya_enabled,omitempty"`
	Version       int              `json:"version"`
	Grader        string           `json:"grader"`
	SuiteHash     string           `json:"suite_hash"`
	SourceRunHash string           `json:"source_run_hash"`
	Model         string           `json:"model"`
	Generation    Generation       `json:"generation"`
	Cases         map[string]Score `json:"cases"`
}

func Scores(r Report, model string) map[string]Score {
	out := map[string]Score{}
	signatures := map[string]map[string]int{}
	for _, t := range r.Trials {
		if t.Model != model {
			continue
		}
		s := out[t.CaseID]
		s.Trials++
		if t.Grade.Passed {
			s.Passed++
		}
		if signatures[t.CaseID] == nil {
			signatures[t.CaseID] = map[string]int{}
		}
		if t.Grade.SemanticSignature != "" {
			signatures[t.CaseID][t.Grade.SemanticSignature]++
		}
		out[t.CaseID] = s
	}
	for id, s := range out {
		largest := 0
		for _, count := range signatures[id] {
			if count > largest {
				largest = count
			}
		}
		s.PassRate = float64(s.Passed) / float64(s.Trials)
		s.Consistency = float64(largest) / float64(s.Trials)
		out[id] = s
	}
	return out
}

// ValidateReport recomputes grades instead of trusting editable saved scores.
func ValidateReport(r Report) error {
	if (r.ControlMode != "" && r.ControlMode != "lexical") || (r.ControlMode != "" && r.LayaURL != "") {
		return fmt.Errorf("invalid evaluation control mode")
	}
	if r.Version != Version || r.Grader != GraderVersion || !r.Completed || r.Repeat < 1 || len(r.Models) == 0 || r.Generation.MaxTokens < 1 || r.Repeat > 100 || math.IsNaN(r.Generation.Temperature) || math.IsInf(r.Generation.Temperature, 0) || r.Generation.Temperature < 0 || r.Generation.Temperature > 2 {
		return fmt.Errorf("incomplete or incompatible evaluation report")
	}
	if (r.LayaURL == "" && r.LayaMode != "") || (r.LayaURL != "" && r.LayaMode != "first" && r.LayaMode != "advisory") {
		return fmt.Errorf("invalid Laya report mode")
	}
	if err := r.Suite.Validate(); err != nil {
		return err
	}
	if Hash(r.Suite) != r.SuiteHash {
		return fmt.Errorf("suite hash mismatch")
	}
	cases := map[string]Case{}
	for _, c := range r.Suite.Cases {
		cases[c.ID] = c
	}
	models := map[string]bool{}
	for _, m := range r.Models {
		if m == "" || models[m] {
			return fmt.Errorf("invalid model matrix")
		}
		models[m] = true
	}
	seen := map[string]bool{}
	for _, t := range r.Trials {
		c, ok := cases[t.CaseID]
		key := fmt.Sprintf("%s\x00%s\x00%d", t.Model, t.CaseID, t.Repetition)
		if !ok || !models[t.Model] || t.Repetition < 1 || t.Repetition > r.Repeat || seen[key] {
			return fmt.Errorf("unexpected or duplicate trial")
		}
		seen[key] = true
		if t.ControlMode != r.ControlMode || (!t.Routing && len(t.Routes) > 0) || ((r.LayaURL != "" && r.LayaMode == "advisory") || r.ControlMode == "lexical") != (t.Recommendation != nil) || (r.LayaURL != "" && r.LayaMode == "first") != t.Routing {
			return fmt.Errorf("recommendation trace does not match report mode")
		}
		if Hash(GradeTrial(c, t)) != Hash(t.Grade) {
			return fmt.Errorf("grade mismatch for %s/%s/%d", t.Model, t.CaseID, t.Repetition)
		}
	}
	if len(seen) != len(cases)*len(models)*r.Repeat {
		return fmt.Errorf("missing trials")
	}
	return nil
}

func CreateBaseline(r Report, model string) (Baseline, error) {
	b := Baseline{ControlMode: r.ControlMode, LayaMode: r.LayaMode, LayaEnabled: r.LayaURL != "", Version: Version, Grader: GraderVersion, SuiteHash: r.SuiteHash, SourceRunHash: Hash(r), Model: model, Generation: r.Generation, Cases: Scores(r, model)}
	if err := ValidateReport(r); err != nil {
		return b, err
	}
	if r.Repeat < 3 {
		return b, fmt.Errorf("baseline requires at least three repetitions per case")
	}
	if len(b.Cases) != len(r.Suite.Cases) {
		return b, fmt.Errorf("model %q missing from report", model)
	}
	for id, score := range b.Cases {
		if score.PassRate != 1 || score.Consistency != 1 {
			return b, fmt.Errorf("case %s is not baseline eligible: all trials must pass and agree semantically", id)
		}
	}
	return b, nil
}

func Compare(b Baseline, r Report, model string) ([]string, error) {
	if err := ValidateReport(r); err != nil {
		return nil, err
	}
	if err := CheckBaseline(b, r.Suite, r.Generation, r.Repeat); err != nil {
		return nil, err
	}
	if b.ControlMode != r.ControlMode {
		return nil, fmt.Errorf("baseline and candidate lexical control modes differ")
	}
	if b.LayaEnabled != (r.LayaURL != "") || b.LayaMode != r.LayaMode {
		return nil, fmt.Errorf("baseline and candidate Laya modes differ")
	}
	scores := Scores(r, model)
	var regressions []string
	for _, c := range r.Suite.Cases {
		floor, ok := b.Cases[c.ID]
		if !ok || floor.Trials < 3 || floor.Passed != floor.Trials || floor.PassRate != 1 || floor.Consistency != 1 {
			return nil, fmt.Errorf("invalid baseline for case %s", c.ID)
		}
		score := scores[c.ID]
		if score.Trials < floor.Trials {
			return nil, fmt.Errorf("case %s needs at least %d candidate trials", c.ID, floor.Trials)
		}
		if score.PassRate < floor.PassRate || score.Consistency < floor.Consistency {
			regressions = append(regressions, fmt.Sprintf("%s: pass %.1f%% (baseline %.1f%%), consistency %.1f%% (baseline %.1f%%)", c.ID, 100*score.PassRate, 100*floor.PassRate, 100*score.Consistency, 100*floor.Consistency))
		}
	}
	sort.Strings(regressions)
	return regressions, nil
}

func ReadJSON(path string, value any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, value)
}

// WriteJSON refuses to replace an existing run or approved baseline.
func WriteJSON(path string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	_, err = file.Write(append(data, '\n'))
	closeErr := file.Close()
	if err != nil {
		return err
	}
	return closeErr
}

// CheckBaseline validates compatibility before spending model calls on a test.
func CheckBaseline(b Baseline, suite Suite, g Generation, repeat int) error {
	if (b.ControlMode != "" && b.ControlMode != "lexical") || (b.ControlMode != "" && (b.LayaEnabled || b.LayaMode != "")) {
		return fmt.Errorf("invalid baseline control mode")
	}
	if b.Version != Version || b.Grader != GraderVersion || b.SuiteHash != Hash(suite) || b.Generation != g || b.SourceRunHash == "" || b.Model == "" || len(b.Cases) != len(suite.Cases) {
		return fmt.Errorf("baseline and candidate suite, grader or generation settings differ")
	}
	for _, c := range suite.Cases {
		floor, ok := b.Cases[c.ID]
		if !ok || floor.Trials < 3 || floor.Trials > 100 || floor.Passed != floor.Trials || floor.PassRate != 1 || floor.Consistency != 1 {
			return fmt.Errorf("invalid baseline for case %s", c.ID)
		}
		if repeat < floor.Trials {
			return fmt.Errorf("case %s needs at least %d candidate trials", c.ID, floor.Trials)
		}
	}
	return nil
}
