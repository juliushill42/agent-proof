package proof

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestFalseSuccessDetected(t *testing.T) {
	root := t.TempDir()
	p, _, err := ChaosScenario("fake-success", root)
	if err != nil {
		t.Fatal(err)
	}
	if p.Result != "FAIL" || p.Classification != "FALSE_SUCCESS_OR_PARTIAL_EXECUTION" {
		t.Fatalf("unexpected result: %+v", p)
	}
}

func TestHungAgentDetected(t *testing.T) {
	root := t.TempDir()
	p, _, err := ChaosScenario("hung-agent", root)
	if err != nil {
		t.Fatal(err)
	}
	if p.Classification != "HUNG_OR_TIMEOUT" {
		t.Fatalf("unexpected classification: %s", p.Classification)
	}
}

func TestRecoveryVerified(t *testing.T) {
	root := t.TempDir()
	p, _, err := ChaosScenario("recovery", root)
	if err != nil {
		t.Fatal(err)
	}
	if p.Result != "PASS" || p.Classification != "RECOVERED_AND_VERIFIED" {
		t.Fatalf("unexpected result: %+v", p)
	}
}

func TestChangedFile(t *testing.T) {
	root := t.TempDir()
	p, _, err := ChaosScenario("changed-file", root)
	if err != nil {
		t.Fatal(err)
	}
	if p.Result != "PASS" {
		t.Fatalf("unexpected result: %+v", p)
	}
}

func TestEvidenceHashStable(t *testing.T) {
	p := Proof{SchemaVersion: "1", Watermark: Watermark, RunID: "x", Result: "PASS"}
	p.EvidenceHash = EvidenceHash(p)
	if p.EvidenceHash != EvidenceHash(p) {
		t.Fatal("hash unstable")
	}
}

func TestFileExistsCheck(t *testing.T) {
	wd := t.TempDir()
	if err := os.WriteFile(filepath.Join(wd, "x"), []byte("ok"), 0o644); err != nil {
		t.Fatal(err)
	}
	s := RunSpec{Workdir: wd, Checks: CheckSpec{FilesExist: []string{"x"}}}
	rs := Verify(context.Background(), s, Snapshot{})
	if len(rs) != 1 || !rs[0].Passed {
		t.Fatalf("unexpected checks: %+v", rs)
	}
}
