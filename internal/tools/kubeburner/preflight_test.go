package kubeburner

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/openshift-storage-cert/storage-cert-harness/internal/core"
)

func TestPreflightContinuesAfterInvalidParameters(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	got, err := (preflight{}).Check(context.Background(), &core.RunCtx{}, core.NewBag(), []core.TestRequirement{
		{ID: "TR-VIRT-010"},
		{ID: "TR-VIRT-027", Params: map[string]any{"replicas": 1, "snapshot_count": 1}},
	})
	if err != nil {
		t.Fatal(err)
	}
	// New structure: common checks (binary, storage, CRDs, populator) + per-TR checks
	// Expected: error (binary), error (storage_class), skip (kubevirt CRD), skip (CDI CRD),
	//           skip (populator), error (TR-VIRT-010 params), info (TR-VIRT-027 params)
	if len(got) < 7 {
		t.Fatalf("findings = %+v, want at least 7 checks (common + per-TR)", got)
	}

	// Check that both TRs got their parameter checks
	var foundVirt010Param, foundVirt027Param bool
	for _, f := range got {
		if strings.Contains(f.Message, "TR-VIRT-010") && strings.Contains(f.Message, "parameters") {
			foundVirt010Param = true
			if f.Level != "error" {
				t.Errorf("TR-VIRT-010 param check = %+v, want error", f)
			}
		}
		if strings.Contains(f.Message, "TR-VIRT-027") && strings.Contains(f.Message, "parameters") {
			foundVirt027Param = true
			if f.Level != "info" {
				t.Errorf("TR-VIRT-027 param check = %+v, want info", f)
			}
		}
	}
	if !foundVirt010Param {
		t.Fatal("missing TR-VIRT-010 parameter check")
	}
	if !foundVirt027Param {
		t.Fatal("missing TR-VIRT-027 parameter check")
	}
}

func TestPreflightSuccessfulSnapshot(t *testing.T) {
	dir := t.TempDir()
	script := `#!/bin/sh
case "$2" in
  volumepopulator) echo '{"items":[]}' ;;
  storageclass) echo '{"provisioner":"test-driver","items":[{"metadata":{"name":"test-sc"},"provisioner":"test-driver"}]}' ;;
  storageprofile) echo '{"status":{"snapshotClass":"test-snapshot","claimPropertySets":[{"volumeMode":"Block","accessModes":["ReadWriteMany"]}]}}' ;;
  cdiconfig) echo '{"status":{}}' ;;
  volumesnapshotclass) echo '{"driver":"test-driver"}' ;;
  crd)
    case "$3" in
      kubevirts.kubevirt.io|datavolumes.cdi.kubevirt.io|virtualmachinesnapshots.snapshot.kubevirt.io)
        echo "crd.apiextensions.k8s.io/$3" ;;
    esac ;;
esac
exit 0
`
	for _, binary := range []string{"kube-burner", "kubectl"} {
		if err := os.WriteFile(filepath.Join(dir, binary), []byte(script), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", dir)
	bag := core.NewBag()
	got, err := (preflight{}).Check(context.Background(), &core.RunCtx{Backend: &core.ResolvedBackend{StorageClass: "test-sc"}}, bag,
		[]core.TestRequirement{{ID: "TR-VIRT-010", Params: map[string]any{"replicas": 1, "snapshot_count": 1}}})
	if err != nil {
		t.Fatal(err)
	}
	// New structure has more findings due to explicit ordering:
	// Common: binary, storage_class (info + check), kubevirt CRD, CDI CRD, populator
	// Per-TR: params, snapshot CRD, wait-for-create, storage preflight
	// Plus snapshot resolution
	if len(got) < 10 {
		t.Fatalf("findings = %+v, want at least 10 checks", got)
	}
	for _, f := range got {
		if f.Level == "error" {
			t.Errorf("finding = %+v, want all checks to pass", f)
		}
	}
	p, ok := core.GetAs[Params](bag, "params")
	if !ok || p.UsePopulator {
		t.Fatalf("params = %+v, want legacy CDI import fallback", p)
	}
}
