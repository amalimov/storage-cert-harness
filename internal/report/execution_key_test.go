package report

import (
	"bytes"
	"encoding/xml"
	"testing"

	"github.com/openshift-storage-cert/storage-cert-harness/internal/core"
)

func TestJUnitArtifactsMatchExecutionScenario(t *testing.T) {
	r := core.Report{
		Executions: []core.RunExecution{
			{TR: "TR-A", Variant: "small", Scenario: "idle", Artifacts: []string{"idle/result.json"}},
			{TR: "TR-A", Variant: "small", Scenario: "load", Artifacts: []string{"load/result.json"}},
			{TR: "TR-A", Variant: "small", Scenario: "empty"},
		},
		Verdicts: []core.Verdict{
			{TR: "TR-A", Variant: "small", Scenario: "idle", Item: "check", Outcome: core.OutcomePass},
			{TR: "TR-A", Variant: "small", Scenario: "load", Item: "check", Outcome: core.OutcomePass},
			{TR: "TR-A", Variant: "small", Scenario: "empty", Item: "check", Outcome: core.OutcomePass},
		},
	}
	var buf bytes.Buffer
	if err := (JUnitExporter{}).Export(&buf, r); err != nil {
		t.Fatal(err)
	}
	var root juSuites
	if err := xml.Unmarshal(buf.Bytes(), &root); err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	for _, suite := range root.Suites {
		for _, tc := range suite.Cases {
			for _, p := range tc.Properties.Property {
				if p.Name == "artifacts" {
					got[tc.Name] = p.Value
				}
			}
		}
	}
	if got["check [idle]"] != "idle/result.json" || got["check [load]"] != "load/result.json" || got["check [empty]"] != "" {
		t.Fatalf("unexpected artifact attribution: %+v", got)
	}
}
