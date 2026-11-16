package report

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"strings"
	"testing"

	"github.com/openshift-storage-cert/storage-cert-harness/internal/core"
)

func TestFlatJUnitVerdicts(t *testing.T) {
	r := core.Report{
		Verdicts: []core.Verdict{
			{TR: "EX-001", Variant: "small", Item: "latency", Scenario: "load & pressure", Outcome: core.OutcomePass},
			{TR: "EX-001", Item: "throughput", Outcome: core.OutcomeFail, Reason: "below gate"},
			{TR: "EX-003", Item: "check", Outcome: core.OutcomeError, Reason: "unavailable"},
			{TR: "EX-003", Item: "optional", Outcome: core.OutcomeSkip, Reason: "not measured"},
		},
		Measurements: []core.Measurement{{TR: "EX-001", Name: "information", Value: 3}},
		Attestations: []core.Attestation{{Claim: "informational", Value: "yes"}},
	}
	var buf bytes.Buffer
	if err := (JUnitExporter{FlatVerdicts: true}).Export(&buf, r); err != nil {
		t.Fatal(err)
	}
	var root juSuites
	if err := xml.Unmarshal(buf.Bytes(), &root); err != nil {
		t.Fatal(err)
	}
	if len(root.Suites) != 1 || root.Tests != 4 || root.Failures != 1 || root.Errors != 1 || root.Skipped != 1 {
		t.Fatalf("unexpected suite totals: %+v", root)
	}
	s := root.Suites[0]
	if s.Tests != 4 || s.Failures != 1 || s.Errors != 1 || s.Skipped != 1 || len(s.Cases) != 4 {
		t.Fatalf("unexpected verdict totals: %+v", s)
	}
	if s.Cases[0].Classname != "EX-001 [small]" || s.Cases[0].Name != "EX-001 [small] / latency [load & pressure]" || s.Cases[1].Name != "EX-001 / throughput" || s.Cases[1].Failure == nil || s.Cases[2].Error == nil || s.Cases[3].Skipped == nil {
		t.Fatalf("unexpected verdict cases: %+v", s.Cases)
	}
	if s.Cases[0].Failure != nil || s.Cases[0].Error != nil || s.Cases[0].Skipped != nil {
		t.Fatalf("passing testcase carries a non-pass outcome: %+v", s.Cases[0])
	}
	props := map[string]string{}
	for _, p := range root.Properties.Property {
		props[p.Name] = p.Value
	}
	if props["harness.checkup_junit_profile_version"] != "1" {
		t.Fatalf("missing checkup JUnit profile version: %+v", props)
	}
	caseProps := map[string]string{}
	for _, p := range s.Cases[0].Properties.Property {
		caseProps[p.Name] = p.Value
	}
	if caseProps["harness.tr_id"] != "EX-001" || caseProps["harness.item"] != "latency" || caseProps["harness.variant"] != "small" || caseProps["scenario"] != "load & pressure" {
		t.Fatalf("unexpected case identity: %+v", caseProps)
	}
}

func TestFlatJUnitRejectsInvalidVerdicts(t *testing.T) {
	for _, verdict := range []core.Verdict{
		{TR: "EX-001", Item: "latency", Outcome: "unknown"},
		{Item: "latency", Outcome: core.OutcomePass},
		{TR: "EX-001", Outcome: core.OutcomePass},
	} {
		var buf bytes.Buffer
		if err := (JUnitExporter{FlatVerdicts: true}).Export(&buf, core.Report{Verdicts: []core.Verdict{verdict}}); err == nil {
			t.Fatalf("expected invalid verdict to fail: %+v", verdict)
		}
		if buf.Len() != 0 {
			t.Fatalf("wrote a partial document for invalid verdict: %q", buf.String())
		}
	}
}

func TestFlatJUnitCarriesIncompleteStatus(t *testing.T) {
	var buf bytes.Buffer
	r := core.Report{Status: core.RunStatusRunning, Verdicts: []core.Verdict{{TR: "EX-001", Item: "latency", Outcome: core.OutcomePass}}}
	if err := (JUnitExporter{FlatVerdicts: true}).Export(&buf, r); err != nil {
		t.Fatal(err)
	}
	var root juSuites
	if err := xml.Unmarshal(buf.Bytes(), &root); err != nil {
		t.Fatal(err)
	}
	for _, p := range root.Properties.Property {
		if p.Name == "harness.status" && p.Value == "running" {
			return
		}
	}
	t.Fatalf("running checkpoint status missing: %+v", root.Properties)
}

func TestFlatJUnitAfterMerge(t *testing.T) {
	base := Build("base", "2.0", core.Provenance{}, []core.Verdict{
		{TR: "EX-001", Variant: "small", Scenario: "idle", Item: "latency", Outcome: core.OutcomeFail},
		{TR: "EX-001", Variant: "large", Scenario: "idle", Item: "latency", Outcome: core.OutcomePass},
	})
	next := Build("next", "2.0", core.Provenance{}, []core.Verdict{
		{TR: "EX-001", Variant: "small", Scenario: "idle", Item: "latency", Outcome: core.OutcomePass},
		{TR: "EX-001", Variant: "small", Scenario: "loaded", Item: "latency", Outcome: core.OutcomeFail},
		{TR: "EX-003", Item: "throughput", Outcome: core.OutcomePass},
	})
	merged, err := Merge(base, next)
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := (JUnitExporter{FlatVerdicts: true}).Export(&buf, merged); err != nil {
		t.Fatal(err)
	}
	var root juSuites
	if err := xml.Unmarshal(buf.Bytes(), &root); err != nil {
		t.Fatal(err)
	}
	if len(root.Suites) != 1 || root.Tests != 4 || root.Failures != 1 {
		t.Fatalf("unexpected merged counts: %+v", root)
	}
	got := map[string]bool{}
	for _, tc := range root.Suites[0].Cases {
		got[tc.Name] = tc.Failure != nil
	}
	for name, failed := range map[string]bool{
		"EX-001 [small] / latency [idle]":   false,
		"EX-001 [large] / latency [idle]":   false,
		"EX-001 [small] / latency [loaded]": true,
		"EX-003 / throughput":               false,
	} {
		if actual, ok := got[name]; !ok || actual != failed {
			t.Errorf("case %q: got %t, present %t, want failure %t", name, actual, ok, failed)
		}
	}
}

func TestJUnitExport(t *testing.T) {
	r := Build("run", "2.0", core.Provenance{}, []core.Verdict{
		{TR: "EX-001", Level: 3, Item: "sla:lat", Outcome: core.OutcomePass, DurationS: 12.5, Expected: "<= 2 ms", Actual: "0.8 ms"},
		{TR: "EX-002", Level: 3, Item: "check:x", Outcome: core.OutcomeError, Reason: "boom"},
		{TR: "EX-003", Level: 2, Item: "sla:iops", Outcome: core.OutcomeSkip, Reason: "tbv"},
	})
	r.Environment.Hardware.Nodes = 6
	r.Environment.Platform.OCPVersion = "4.22.11"
	r.Environment.Platform.CSIDriver.Name = "csi.example.com"
	r.Attestations = []core.Attestation{{Claim: "ra_url", Value: "https://x", SignedBy: "human:me"}}
	// A measured value with no SLA must still surface (ADR-0014).
	r.Measurements = []core.Measurement{{TR: "EX-001", Level: 3, Name: "podReady", Value: 4.2, Unit: "ms", Percentile: "p99"}}

	var buf bytes.Buffer
	if err := (JUnitExporter{}).Export(&buf, r); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, want := range []string{
		`name="environment-hardware"`,
		`name="environment-platform"`,
		`value="4.22.11"`,
		`name="level-2"`,
		`name="level-3"`,
		`classname="EX-001"`,
		`time="12.500"`,
		`<error message="boom">`,
		`<skipped message="tbv">`,
		`name="measurements"`,
		`name="podReady@p99"`,
		`value="4.2"`,
		`name="attestations"`,
		`value="https://x"`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("junit output missing %q\n---\n%s", want, out)
		}
	}
	var markdown bytes.Buffer
	if err := WriteMarkdown(&markdown, r); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"## Attestations", "ra url", "https://x", "Signed by: human:me"} {
		if !strings.Contains(markdown.String(), want) {
			t.Errorf("markdown output missing %q\n---\n%s", want, markdown.String())
		}
	}
}

// A verdict should be traceable to the files it was graded from in JUnit too,
// not only in the JSON report: the catalog accepts either format. Artifacts are
// recorded per execution, so every item of that execution carries them.
func TestJUnitCarriesExecutionArtifacts(t *testing.T) {
	r := core.Report{
		SchemaVersion: "2.0",
		Executions: []core.RunExecution{{
			TR: "TR-VIRT-018", State: core.ExecutionCompleted, Outcome: core.OutcomeFail,
			Artifacts: []string{
				"virtbench/TR-VIRT-018/20261018-180304/2-disk/run/summary_vm_creation_results.json",
				"virtbench/TR-VIRT-018/20261018-180304/2-disk/run/vm_creation_results.json",
			},
		}},
		Verdicts: []core.Verdict{
			{TR: "TR-VIRT-018", Item: "sla:clone_duration@p99", Outcome: core.OutcomeFail, Level: 3},
		},
	}
	var buf bytes.Buffer
	if err := (JUnitExporter{}).Export(&buf, r); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, want := range []string{
		`name="artifacts"`,
		"virtbench/TR-VIRT-018/20261018-180304/2-disk/run/vm_creation_results.json",
		`name="harness.junit_profile_version" value="1"`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("junit output missing %q\n---\n%s", want, out)
		}
	}
}

// An execution with no recorded artifacts must not emit an empty property.
func TestJUnitOmitsArtifactsWhenNoneRecorded(t *testing.T) {
	r := core.Report{
		SchemaVersion: "2.0",
		Executions:    []core.RunExecution{{TR: "TR-VIRT-018", State: core.ExecutionCompleted}},
		Verdicts:      []core.Verdict{{TR: "TR-VIRT-018", Item: "sla:clone_duration@p99", Outcome: core.OutcomePass, Level: 3}},
	}
	var buf bytes.Buffer
	if err := (JUnitExporter{}).Export(&buf, r); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(buf.String(), `name="artifacts"`) {
		t.Errorf("emitted an empty artifacts property\n---\n%s", buf.String())
	}
}

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
	for _, flat := range []bool{false, true} {
		t.Run(fmt.Sprintf("flat=%t", flat), func(t *testing.T) {
			var buf bytes.Buffer
			if err := (JUnitExporter{FlatVerdicts: flat}).Export(&buf, r); err != nil {
				t.Fatal(err)
			}
			var root juSuites
			if err := xml.Unmarshal(buf.Bytes(), &root); err != nil {
				t.Fatal(err)
			}
			got := map[string]string{}
			for _, suite := range root.Suites {
				for _, tc := range suite.Cases {
					var scenario, artifacts string
					for _, p := range tc.Properties.Property {
						switch p.Name {
						case "scenario":
							scenario = p.Value
						case "artifacts":
							artifacts = p.Value
						}
					}
					got[scenario] = artifacts
				}
			}
			if len(got) != 3 || got["idle"] != "idle/result.json" || got["load"] != "load/result.json" || got["empty"] != "" {
				t.Fatalf("unexpected artifact attribution: %+v", got)
			}
		})
	}
}
