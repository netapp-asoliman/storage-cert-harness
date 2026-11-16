package main

import (
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/openshift-storage-cert/storage-cert-harness/internal/core"
)

func TestWriteReportFilesIncludesFlatJUnit(t *testing.T) {
	dir := t.TempDir()
	rep := core.Report{Verdicts: []core.Verdict{{TR: "EX-001", Item: "latency", Outcome: core.OutcomeFail}}}
	if err := writeReportFiles(dir, rep, nil); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"report.junit.xml", "report.junit.flat.xml"} {
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		var document struct {
			XMLName xml.Name `xml:"testsuites"`
			Suites  []struct {
				Cases []struct {
					Classname string `xml:"classname,attr"`
				} `xml:"testcase"`
			} `xml:"testsuite"`
		}
		if err := xml.Unmarshal(data, &document); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if name == "report.junit.flat.xml" && (len(document.Suites) != 1 || len(document.Suites[0].Cases) != 1 || document.Suites[0].Cases[0].Classname != "EX-001") {
			t.Fatalf("unexpected flat JUnit: %s", data)
		}
	}
}

func TestFlatJUnitContinuedRun(t *testing.T) {
	dir := t.TempDir()
	catalog := filepath.Join(dir, "catalog.json")
	thresholds := filepath.Join(dir, "thresholds.json")
	const catalogJSON = `{"schema_version":"2.0","provenance":{"kb_git_commit":"fixture"},"test_requirements":[{"id":"EX-001","automation_tool":"example","sla_status":"defined","sla_count":1,"sla":[{"metric":"latency_ms","type":"latency","unit":"ms","percentile":"p99","measured_by":["example"]}]},{"id":"EX-003","automation_tool":"example","sla_status":"defined","sla_count":1,"sla":[{"metric":"read_throughput","type":"throughput","unit":"MiB/s","measured_by":["example"]}]}]}`
	if err := os.WriteFile(catalog, []byte(catalogJSON), 0o600); err != nil {
		t.Fatal(err)
	}
	writeThresholds := func(gate float64) {
		t.Helper()
		data := fmt.Sprintf(`{"schema_version":"2.0","provenance":{"kb_git_commit":"fixture"},"test_requirements":[{"id":"EX-001","sla":[{"metric":"latency_ms","type":"latency","operator":"<=","value":%g,"unit":"ms","percentile":"p99"}]},{"id":"EX-003","sla":[{"metric":"read_throughput","type":"throughput","operator":">=","value":1000,"unit":"MiB/s"}]}]}`, gate)
		if err := os.WriteFile(thresholds, []byte(data), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	run := func(args ...string) error {
		t.Helper()
		cmd := newRunCmd()
		cmd.SetArgs(append([]string{"--catalog", catalog, "--thresholds", thresholds, "--skip-env", "--no-attestations"}, args...))
		return cmd.Execute()
	}
	first := filepath.Join(dir, "first")
	writeThresholds(0.5)
	if err := run("--id", "EX-001", "--output", first); err == nil {
		t.Fatal("expected first run to fail its SLA")
	}
	writeThresholds(1)
	continued := filepath.Join(dir, "continued")
	if err := run("--id", "EX-001,EX-003", "--retry-failed", "--continue-from", filepath.Join(first, "report.json"), "--output", continued); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(continued, "report.junit.flat.xml"))
	if err != nil {
		t.Fatal(err)
	}
	var root struct {
		Suites []struct {
			Tests    int `xml:"tests,attr"`
			Failures int `xml:"failures,attr"`
			Cases    []struct {
				Name string `xml:"name,attr"`
			} `xml:"testcase"`
		} `xml:"testsuite"`
	}
	if err := xml.Unmarshal(data, &root); err != nil {
		t.Fatal(err)
	}
	if len(root.Suites) != 1 || root.Suites[0].Tests != 2 || root.Suites[0].Failures != 0 {
		t.Fatalf("unexpected continued report: %s", data)
	}
	names := map[string]bool{}
	for _, tc := range root.Suites[0].Cases {
		names[tc.Name] = true
	}
	if !names["EX-001 / sla:latency_ms@p99"] || !names["EX-003 / sla:read_throughput"] {
		t.Fatalf("missing continued verdicts: %s", data)
	}
}
