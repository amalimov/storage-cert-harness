package kubeburnerocp

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/openshift-storage-cert/storage-cert-harness/internal/core"
)

func TestPreflightContinuesAfterInvalidWorkload(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	trs := []core.TestRequirement{
		{ID: "INVALID", Params: map[string]any{"workload": "unknown"}},
		{ID: "MIGRATION", Params: map[string]any{"workload": WorkloadVirtMigration, "iterations": 1, "iteration_vms": 2}},
		{ID: "PARALLEL", Params: map[string]any{"workload": WorkloadVirtParallel}},
	}
	got, err := (preflight{}).Check(context.Background(), &core.RunCtx{}, core.NewBag(), trs)
	if err != nil {
		t.Fatal(err)
	}
	// New structure: common checks first (binary, storage), then per-TR checks
	// Count check categories rather than exact order
	var invalidParam, migrationWorkload, parallelWorkload, binaryError, prereqSkip int
	for _, f := range got {
		if strings.Contains(f.Message, "INVALID") && strings.Contains(f.Message, "parameters") {
			invalidParam++
		}
		if strings.Contains(f.Message, "MIGRATION") && strings.Contains(f.Message, "workload") {
			migrationWorkload++
		}
		if strings.Contains(f.Message, "PARALLEL") && strings.Contains(f.Message, "workload") {
			parallelWorkload++
		}
		if f.Level == "error" && strings.Contains(f.Message, "PATH") {
			binaryError++
		}
		if f.Level == "skip" && strings.Contains(f.Message, "neither kubectl nor oc") {
			prereqSkip++
		}
	}

	if invalidParam == 0 {
		t.Error("missing INVALID parameter error")
	}
	if migrationWorkload == 0 {
		t.Error("missing MIGRATION workload check")
	}
	if parallelWorkload == 0 {
		t.Error("missing PARALLEL workload check")
	}
	if binaryError == 0 {
		t.Error("missing binary availability errors")
	}
}

func TestPreflightSuccessfulMigration(t *testing.T) {
	dir := t.TempDir()
	script := `#!/bin/sh
case "$2" in
  crd)
    case "$3" in
      kubevirts.kubevirt.io|datavolumes.cdi.kubevirt.io)
        echo "crd.apiextensions.k8s.io/$3" ;;
    esac ;;
  storageclass)
    echo '{"items":[{"metadata":{"name":"test-sc"}}]}' ;;
esac
exit 0
`
	for _, binary := range []string{"kube-burner-ocp", "virtctl", "kubectl"} {
		content := "#!/bin/sh\nexit 0\n"
		if binary == "kubectl" {
			content = script
		}
		if err := os.WriteFile(filepath.Join(dir, binary), []byte(content), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", dir)
	got, err := (preflight{}).Check(context.Background(), &core.RunCtx{Backend: &core.ResolvedBackend{StorageClass: "test-sc"}}, core.NewBag(),
		[]core.TestRequirement{{ID: "MIGRATION", Params: map[string]any{
			"workload": WorkloadVirtMigration, "iterations": 1, "iteration_vms": 2,
		}}})
	if err != nil {
		t.Fatal(err)
	}
	// New structure has: common binary, storage class (info + check), per-TR workload,
	// workload binaries, workload CRD prereqs
	if len(got) < 6 {
		t.Fatalf("findings = %+v, want at least workload, storage class, 2 binaries, and 2 CRDs", got)
	}
	for _, f := range got {
		if f.Level == "error" {
			t.Errorf("finding = %+v, want all checks to pass", f)
		}
	}
}

// A plan that names a valid workload but omits a flag that workload requires
// used to clear preflight and then fail seconds into the run, after the tool
// had started creating cluster objects. The catalog's declared params are not
// the ones every workload consumes — virt-migration is declared with
// "concurrency" but driven by "iterations" and "iteration_vms" — so only the
// workload spec can catch this.
func TestPreflightRejectsMissingWorkloadParam(t *testing.T) {
	dir := t.TempDir()
	for _, binary := range []string{"kube-burner-ocp", "virtctl", "kubectl"} {
		if err := os.WriteFile(filepath.Join(dir, binary), []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", dir)
	got, err := (preflight{}).Check(context.Background(), &core.RunCtx{Backend: &core.ResolvedBackend{StorageClass: "test-sc"}}, core.NewBag(),
		[]core.TestRequirement{{ID: "MIGRATION", Params: map[string]any{
			"workload": WorkloadVirtMigration, "concurrency": 2, // what the catalog declares, not what the tool needs
		}}})
	if err != nil {
		t.Fatal(err)
	}
	var failed bool
	for _, f := range got {
		if f.Level == "error" && strings.Contains(f.Message, `missing required param "iterations"`) {
			failed = true
		}
	}
	if !failed {
		t.Fatalf("preflight passed a plan the runner would reject: %+v", got)
	}
}
