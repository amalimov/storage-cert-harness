package kubeburner

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/openshift-storage-cert/storage-cert-harness/internal/clustercheck"
	"github.com/openshift-storage-cert/storage-cert-harness/internal/core"
	"github.com/openshift-storage-cert/storage-cert-harness/internal/safefs"
)

// trCheckFuncs maps TR IDs to their specific check functions.
var trCheckFuncs = map[string]func(context.Context, core.TestRequirement, *core.RunCtx, string) []core.Finding{
	"TR-VIRT-010": checksForVirt010,
	"TR-VIRT-027": checksForVirt027,
}

type preflight struct{}

func (preflight) Check(ctx context.Context, rc *core.RunCtx, bag *core.Bag, trs []core.TestRequirement) ([]core.Finding, error) {
	cli := clustercheck.KubeCLI()
	var findings []core.Finding

	// Tool-level checks (all TRs need these)
	findings = append(findings, commonChecks(ctx, rc, cli, trs)...)

	// Determine if any TR needs snapshots (for snapshot resolution later)
	needsSnapshot := false
	for _, tr := range trs {
		params := resolveParams([]core.TestRequirement{tr})
		needsSnapshot = needsSnapshot || params.SnapshotCount > 0
	}

	// Per-TR specific checks
	for _, tr := range trs {
		if checkFunc, exists := trCheckFuncs[tr.ID]; exists {
			findings = append(findings, checkFunc(ctx, tr, rc, cli)...)
		} else {
			// Fallback for TRs without specific check functions - just validate params
			params := resolveParams([]core.TestRequirement{tr})
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
		}
	}

	// Resolve parameters and store in Bag for later stages
	p := resolveParams(trs)

	// Check for CDI VolumeImportSource populator and update params
	if cli != "" {
		usePopulator, err := clustercheck.HasVolumePopulator(ctx, cli, "cdi.kubevirt.io", "VolumeImportSource")
		if err == nil {
			p.UsePopulator = usePopulator
		}
	}

	bag.Set("params", p)

	// Snapshot class resolution if any TR needs snapshots
	sc := ""
	if rc.Backend != nil {
		sc = rc.Backend.StorageClass
	}
	if sc != "" && needsSnapshot {
		selection, err := clustercheck.ResolveSnapshot(ctx, sc, rc.Backend.SnapshotClass, "", true)
		if err != nil {
			findings = append(findings, core.Finding{Level: "error", Message: err.Error()})
		} else {
			bag.Set("snapshot_selection", selection)
			findings = append(findings, selection.Finding())
		}
	}

	return findings, nil
}

type provisioner struct{}

func (provisioner) Provision(ctx context.Context, rc *core.RunCtx, bag *core.Bag, _ []core.TestRequirement) error {
	if selection, ok := core.GetAs[clustercheck.SnapshotSelection](bag, "snapshot_selection"); ok && rc.Backend != nil && rc.Backend.SnapshotClass != "" {
		// Pin the resolved class for the whole run through a private profile.
		created, err := prepareVMStorage(ctx, selection)
		bag.Set("snapshot_storage_class", created)
		if err != nil {
			if rc.Logger != nil {
				rc.Logger.Error("kube-burner: storage setup failed", "storage_class", selection.StorageClass, "storage_profile", created, "err", err)
			}
			return fmt.Errorf("prepare snapshot storage: %w", err)
		}
	}
	dir := rc.WorkDir
	if dir == "" {
		dir = os.TempDir()
	}
	resultsDir := strings.TrimRight(dir, "/") + "/" + resultsSubdir
	if err := safefs.MkdirAll(resultsDir, 0o755); err != nil {
		return fmt.Errorf("kube-burner: results dir: %w", err)
	}
	bag.Set("results_dir", resultsDir)
	rc.Logger.Info("kube-burner: provisioned results dir", "dir", resultsDir)
	return nil
}
