package proof

import (
	"context"
	"testing"
)

func TestAbsentChangedFileFails(t *testing.T) {
	spec := RunSpec{Workdir: t.TempDir(), Checks: CheckSpec{FilesChanged: []string{"missing.txt"}}}
	got := Verify(context.Background(), spec, Snapshot{"missing.txt": "ABSENT"})
	if len(got) != 1 || got[0].Passed {
		t.Fatalf("missing file accepted as changed: %+v", got)
	}
}
