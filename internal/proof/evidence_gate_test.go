package proof

import "testing"

func TestNoChecksCannotProveSuccess(t *testing.T) {
	got := Classify(ProcessResult{ExitCode: 0}, nil)
	if got != "UNVERIFIED_NO_CHECKS" {
		t.Fatalf("no-check execution reported %s", got)
	}
}

func TestFalseSuccessWithCriteria(t *testing.T) {
	checks := []CheckResult{{Kind: "file_exists", Target: "missing", Passed: false}}
	got := Classify(ProcessResult{ExitCode: 0}, checks)
	if got != "FALSE_SUCCESS_OR_PARTIAL_EXECUTION" {
		t.Fatalf("failed acceptance criteria reported %s", got)
	}
}
