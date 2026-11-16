package core

import "testing"

func TestOutcomeValid(t *testing.T) {
	for _, tc := range []struct {
		outcome Outcome
		valid   bool
	}{
		{OutcomePass, true},
		{OutcomeFail, true},
		{OutcomeError, true},
		{OutcomeSkip, true},
		{"", false},
		{"PASS", false},
		{"unknown", false},
	} {
		if got := tc.outcome.Valid(); got != tc.valid {
			t.Errorf("Outcome(%q).Valid() = %t, want %t", tc.outcome, got, tc.valid)
		}
	}
}
