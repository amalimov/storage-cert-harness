package kubeburnerocp

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
	"TR-STOR-006": checksForStor006,
	"TR-VIRT-004": checksForVirt004,
	"TR-VIRT-019": checksForVirt019,
}

type preflight struct{}

func (preflight) Check(ctx context.Context, rc *core.RunCtx, bag *core.Bag, trs []core.TestRequirement) ([]core.Finding, error) {
	cli := clustercheck.KubeCLI()
	var findings []core.Finding

	// Tool-level checks
	findings = append(findings, commonChecks(ctx, rc, cli)...)

	// Per-TR specific checks
	for _, tr := range trs {
		if checkFunc, exists := trCheckFuncs[tr.ID]; exists {
			findings = append(findings, checkFunc(ctx, tr, rc, cli)...)
		} else {
			// Fallback for TRs without specific check functions - still validate params and workload prereqs
			params := resolveParams([]core.TestRequirement{tr})
			if err := params.validate(); err != nil {
				findings = append(findings, core.Finding{
					Level:     "error",
					Message:   tr.ID + " parameters: " + err.Error(),
					BlocksTRs: []string{tr.ID},
				})
			} else {
				findings = append(findings, core.Finding{
					Level:   "info",
					Message: tr.ID + " workload=" + params.Workload,
				})
				// Check workload prerequisites even for TRs without specific check functions
				findings = append(findings, checkWorkloadPrereqs(ctx, tr, rc, cli, params.Workload)...)
			}
		}
	}

	// Store resolved params in Bag
	p := resolveParams(trs)
	bag.Set("params", p)

	return findings, nil
}

type provisioner struct{}

func (provisioner) Provision(_ context.Context, rc *core.RunCtx, bag *core.Bag, _ []core.TestRequirement) error {
	dir := rc.WorkDir
	if dir == "" {
		dir = os.TempDir()
	}
	resultsDir := strings.TrimRight(dir, "/") + "/" + resultsSubdir
	if err := safefs.MkdirAll(resultsDir, 0o755); err != nil {
		return fmt.Errorf("kube-burner-ocp: results dir: %w", err)
	}
	bag.Set("results_dir", resultsDir)
	rc.Logger.Info("kube-burner-ocp: provisioned results dir", "dir", resultsDir)
	return nil
}
