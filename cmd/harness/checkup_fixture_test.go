package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/openshift-storage-cert/storage-cert-harness/internal/core"
)

func TestCheckupSmokeFixture(t *testing.T) {
	dir := filepath.Join("..", "..", "examples", "checkup-smoke")
	thresholds, err := os.ReadFile(filepath.Join(dir, "thresholds.json"))
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(thresholds, &doc); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name  string
		value int
		gate  float64
		want  core.Outcome
	}{
		{"defaults", 4, 10, core.OutcomePass},
		{"sla only", 4, 3, core.OutcomeFail},
		{"value only", 12, 10, core.OutcomeFail},
		{"value and sla", 12, 15, core.OutcomePass},
	} {
		t.Run(tc.name, func(t *testing.T) {
			trs := doc["test_requirements"].([]any)
			trs[0].(map[string]any)["sla"].([]any)[0].(map[string]any)["value"] = tc.gate
			data, err := json.Marshal(doc)
			if err != nil {
				t.Fatal(err)
			}
			tmp := t.TempDir()
			input := filepath.Join(tmp, "thresholds.json")
			if err := os.WriteFile(input, data, 0o600); err != nil {
				t.Fatal(err)
			}
			plan := filepath.Join(dir, "checkup-csi-smoke.yaml")
			if tc.value != 4 {
				contents, err := os.ReadFile(plan)
				if err != nil {
					t.Fatal(err)
				}
				plan = filepath.Join(tmp, "plan.yaml")
				contents = append(contents, []byte(fmt.Sprintf("\noverrides:\n  EX-006:\n    sample_value: %d\n", tc.value))...)
				if err := os.WriteFile(plan, contents, 0o600); err != nil {
					t.Fatal(err)
				}
			}
			cmd := newRunCmd()
			cmd.SetArgs([]string{"--catalog", filepath.Join(dir, "catalog.json"), "--thresholds", input, "--plan", plan, "--skip-env", "--no-attestations", "--output", tmp})
			err = cmd.Execute()
			if (err != nil) != (tc.want == core.OutcomeFail) {
				t.Fatalf("run error = %v, expected outcome %s", err, tc.want)
			}
			data, err = os.ReadFile(filepath.Join(tmp, "report.json"))
			if err != nil {
				t.Fatal(err)
			}
			var rep core.Report
			if err := json.Unmarshal(data, &rep); err != nil {
				t.Fatal(err)
			}
			got := map[string]core.Outcome{}
			for _, v := range rep.Verdicts {
				got[v.TR] = v.Outcome
			}
			if len(rep.Verdicts) != 2 || got["EX-006"] != tc.want || got["EX-003"] != core.OutcomePass {
				t.Fatalf("unexpected verdicts: %+v", rep.Verdicts)
			}
			for _, v := range rep.Verdicts {
				if v.TR == "EX-006" && (v.Actual != fmt.Sprintf("%d count", tc.value) || v.Expected != fmt.Sprintf("< %g count", tc.gate)) {
					t.Fatalf("wrong value or threshold in verdict: %+v", v)
				}
			}
		})
	}
}

func TestCheckupSmokeVariantsUseTheirOwnValues(t *testing.T) {
	dir := filepath.Join("..", "..", "examples", "checkup-smoke")
	tmp := t.TempDir()
	plan := filepath.Join(tmp, "plan.yaml")
	const contents = `schema_version: "1"
name: example-variants
select:
  ids: [EX-006]
variants:
  EX-006:
    - name: small
      params:
        sample_value: 4
    - name: large
      params:
        sample_value: 12
`
	if err := os.WriteFile(plan, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
	cmd := newRunCmd()
	cmd.SetArgs([]string{"--catalog", filepath.Join(dir, "catalog.json"), "--thresholds", filepath.Join(dir, "thresholds.json"), "--plan", plan, "--skip-env", "--no-attestations", "--output", tmp})
	if err := cmd.Execute(); err == nil {
		t.Fatal("expected the large variant to fail its SLA")
	}
	data, err := os.ReadFile(filepath.Join(tmp, "report.json"))
	if err != nil {
		t.Fatal(err)
	}
	var rep core.Report
	if err := json.Unmarshal(data, &rep); err != nil {
		t.Fatal(err)
	}
	got := map[string]core.Verdict{}
	for _, v := range rep.Verdicts {
		got[v.Variant] = v
	}
	if len(rep.Verdicts) != 2 || got["small"].Actual != "4 count" || got["small"].Outcome != core.OutcomePass || got["large"].Actual != "12 count" || got["large"].Outcome != core.OutcomeFail {
		t.Fatalf("unexpected variant verdicts: %+v", rep.Verdicts)
	}
}
