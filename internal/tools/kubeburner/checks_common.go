package kubeburner

import (
	"context"
	"fmt"

	"github.com/openshift-storage-cert/storage-cert-harness/internal/clustercheck"
	"github.com/openshift-storage-cert/storage-cert-harness/internal/core"
)

// commonChecks performs tool-level preflight checks that all kubeburner TRs need.
// These checks are not TR-specific.
func commonChecks(ctx context.Context, rc *core.RunCtx, cli string, trs []core.TestRequirement) []core.Finding {
	var findings []core.Finding

	// Binary availability
	findings = append(findings, rc.CheckOnce("binary:kube-burner",
		func(ctx context.Context) core.Finding {
			return clustercheck.CheckBinary("kube-burner")
		}))

	// Storage class existence
	sc := ""
	if rc.Backend != nil {
		sc = rc.Backend.StorageClass
	}
	if sc == "" {
		findings = append(findings, core.Finding{
			Level:   "error",
			Message: "kube-burner: storage_class required (pass --backends/--backend)",
		})
	} else {
		findings = append(findings, rc.CheckOnce("storageclass:"+sc,
			func(ctx context.Context) core.Finding {
				return clustercheck.CheckStorageClass(ctx, cli, sc)
			}))
		findings = append(findings, core.Finding{Level: "info", Message: "storage_class=" + sc})
	}

	// Cluster capabilities needed by all kubeburner TRs
	findings = append(findings, rc.CheckOnce("crd:kubevirts.kubevirt.io",
		func(ctx context.Context) core.Finding {
			return clustercheck.CheckCRD(ctx, cli, "kubevirts.kubevirt.io")
		}))

	findings = append(findings, rc.CheckOnce("crd:datavolumes.cdi.kubevirt.io",
		func(ctx context.Context) core.Finding {
			return clustercheck.CheckCRD(ctx, cli, "datavolumes.cdi.kubevirt.io")
		}))

	// CDI VolumeImportSource populator check
	if cli != "" {
		usePopulator, err := clustercheck.HasVolumePopulator(ctx, cli, "cdi.kubevirt.io", "VolumeImportSource")
		if err != nil {
			findings = append(findings, core.Finding{
				Level:   "error",
				Message: fmt.Sprintf("kube-burner: checking CDI VolumeImportSource populator: %v", err),
			})
		} else if usePopulator {
			findings = append(findings, core.Finding{
				Level:   "info",
				Message: "CDI VolumeImportSource populator registered; using populator imports",
			})
		} else {
			findings = append(findings, core.Finding{
				Level:   "info",
				Message: "CDI VolumeImportSource populator not registered; using legacy CDI imports",
			})
		}
	} else {
		findings = append(findings, core.Finding{
			Level:   "skip",
			Message: "CDI VolumeImportSource populator: neither kubectl nor oc found on PATH",
		})
	}

	return findings
}
