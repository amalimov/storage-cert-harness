package kubeburnerocp

import (
	"context"
	"fmt"

	"github.com/openshift-storage-cert/storage-cert-harness/internal/clustercheck"
	"github.com/openshift-storage-cert/storage-cert-harness/internal/core"
)

// commonChecks performs tool-level preflight checks that all kubeburnerocp TRs need.
func commonChecks(ctx context.Context, rc *core.RunCtx, cli string) []core.Finding {
	var findings []core.Finding

	// kube-burner-ocp binary
	findings = append(findings, rc.CheckOnce("binary:kube-burner-ocp",
		func(ctx context.Context) core.Finding {
			return clustercheck.CheckBinary("kube-burner-ocp")
		}))

	// Storage class (if configured)
	if rc.Backend != nil && rc.Backend.StorageClass != "" {
		findings = append(findings, core.Finding{
			Level:   "info",
			Message: "storage_class=" + rc.Backend.StorageClass,
		})
		findings = append(findings, rc.CheckOnce("storageclass:"+rc.Backend.StorageClass,
			func(ctx context.Context) core.Finding {
				return clustercheck.CheckStorageClass(ctx, cli, rc.Backend.StorageClass)
			}))
	}

	return findings
}

// checkWorkloadPrereqs checks workload-specific binary and cluster prerequisites.
// Returns findings with BlocksTRs set if the workload cannot run.
func checkWorkloadPrereqs(ctx context.Context, tr core.TestRequirement, rc *core.RunCtx, cli string, workload string) []core.Finding {
	var findings []core.Finding

	// Workload-specific host binaries
	spec, ok := workloadSpecs[workload]
	if !ok {
		findings = append(findings, core.Finding{
			Level:     "error",
			Message:   fmt.Sprintf("%s: unknown workload %q", tr.ID, workload),
			BlocksTRs: []string{tr.ID},
		})
		return findings
	}

	for _, bin := range spec.hostBins {
		finding := rc.CheckOnce("binary:"+bin,
			func(ctx context.Context) core.Finding {
				return clustercheck.CheckBinary(bin)
			})
		if finding.Level == "error" {
			finding.BlocksTRs = []string{tr.ID}
			finding.Message = fmt.Sprintf("workload %s needs %s", workload, finding.Message)
		}
		findings = append(findings, finding)
	}

	// Workload-specific cluster prerequisites
	for _, prereq := range spec.prereqs {
		var finding core.Finding
		if cli == "" {
			finding = core.Finding{
				Level:   "skip",
				Message: prereq.Name + ": neither kubectl nor oc found on PATH",
			}
		} else {
			finding = prereq.Check(ctx, cli)
			if finding.Level == "error" {
				finding.BlocksTRs = []string{tr.ID}
			}
		}
		findings = append(findings, finding)
	}

	return findings
}
