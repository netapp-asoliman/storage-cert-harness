package report

import (
	"testing"

	"github.com/openshift-storage-cert/storage-cert-harness/internal/core"
)

func v(level int, tr string, o core.Outcome) core.Verdict {
	return core.Verdict{TR: tr, Level: level, Item: "item", Outcome: o}
}

func TestBuildCertifiedLevel(t *testing.T) {
	cases := []struct {
		name     string
		verdicts []core.Verdict
		want     int
	}{
		// Today's reality: every TR is Level 2. An all-L2 pass must read as level 2,
		// not 0 (L1 is a separate external suite, not tested here).
		{"l2 only pass", []core.Verdict{v(2, "A", core.OutcomePass)}, 2},
		{"l2 fail", []core.Verdict{v(2, "A", core.OutcomeFail)}, 0},
		{"l2 pass, l3 fail", []core.Verdict{
			v(2, "B", core.OutcomePass), v(3, "C", core.OutcomeFail),
		}, 2},
		{"l2 error blocks", []core.Verdict{v(2, "A", core.OutcomeError)}, 0},
		{"l2 all skip is not earned", []core.Verdict{v(2, "A", core.OutcomeSkip)}, 0},
		{"untested lower level does not block", []core.Verdict{
			v(2, "A", core.OutcomeSkip), v(3, "B", core.OutcomePass),
		}, 3},
		{"l2 pass despite a skip sibling", []core.Verdict{
			v(2, "A", core.OutcomePass), v(2, "A", core.OutcomeSkip),
		}, 2},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := Build("run", "2.0", core.Provenance{}, c.verdicts)
			if r.Summary.CertifiedLevel != c.want {
				t.Fatalf("certified level = %d, want %d (levels=%+v)", r.Summary.CertifiedLevel, c.want, r.Levels)
			}
		})
	}
}
