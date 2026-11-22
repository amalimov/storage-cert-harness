package kubeburnerocp

import (
	"context"

	"github.com/openshift-storage-cert/storage-cert-harness/internal/core"
)

// checksForStor006 performs all preflight checks specific to TR-STOR-006.
func checksForStor006(ctx context.Context, tr core.TestRequirement, rc *core.RunCtx, cli string) []core.Finding {
	var findings []core.Finding
	params := resolveParams([]core.TestRequirement{tr})

	// Parameter validation
	if err := params.validate(); err != nil {
		findings = append(findings, core.Finding{
			Level:     "error",
			Message:   tr.ID + " parameters: " + err.Error(),
			BlocksTRs: []string{tr.ID},
		})
		findings = append(findings, core.Finding{
			Level:   "skip",
			Message: tr.ID + " workload prerequisites: cannot select checks without a valid workload",
		})
		return findings
	}

	findings = append(findings, core.Finding{
		Level:   "info",
		Message: tr.ID + " workload=" + params.Workload,
	})

	// Workload-specific prerequisites
	findings = append(findings, checkWorkloadPrereqs(ctx, tr, rc, cli, params.Workload)...)

	return findings
}
