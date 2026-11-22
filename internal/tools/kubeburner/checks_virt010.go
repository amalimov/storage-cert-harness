package kubeburner

import (
	"context"

	"github.com/openshift-storage-cert/storage-cert-harness/internal/clustercheck"
	"github.com/openshift-storage-cert/storage-cert-harness/internal/core"
)

// checksForVirt010 performs all preflight checks specific to TR-VIRT-010.
// Returns findings with BlocksTRs set to prevent this TR from running if checks fail.
func checksForVirt010(ctx context.Context, tr core.TestRequirement, rc *core.RunCtx, cli string) []core.Finding {
	var findings []core.Finding
	params := resolveParams([]core.TestRequirement{tr})

	// Parameter validation
	if err := params.validateFor(tr.ID); err != nil {
		findings = append(findings, core.Finding{
			Level:     "error",
			Message:   tr.ID + " parameters: " + err.Error(),
			BlocksTRs: []string{tr.ID},
		})
	} else {
		findings = append(findings, core.Finding{
			Level:   "info",
			Message: tr.ID + " parameters valid",
		})
	}

	// Snapshot CRD if snapshots are needed
	if params.SnapshotCount > 0 {
		finding := rc.CheckOnce("crd:virtualmachinesnapshots.snapshot.kubevirt.io",
			func(ctx context.Context) core.Finding {
				return clustercheck.CheckCRD(ctx, cli, "virtualmachinesnapshots.snapshot.kubevirt.io")
			})
		if finding.Level == "error" {
			finding.BlocksTRs = []string{tr.ID}
		}
		findings = append(findings, finding)

		// Wait-for-create support check
		finding = clustercheck.WaitForCreateSupported(ctx, "")
		if finding.Level == "error" {
			finding.BlocksTRs = []string{tr.ID}
		}
		findings = append(findings, finding)
	}

	// TR-specific storage check
	if rc.Backend != nil && rc.Backend.StorageClass != "" {
		storageFindings := workloadStoragePreflight(ctx, tr, rc.Backend.StorageClass)
		for i := range storageFindings {
			if storageFindings[i].Level == "error" {
				storageFindings[i].BlocksTRs = []string{tr.ID}
			}
		}
		findings = append(findings, storageFindings...)
	}

	return findings
}
