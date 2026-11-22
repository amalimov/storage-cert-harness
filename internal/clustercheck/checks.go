package clustercheck

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/openshift-storage-cert/storage-cert-harness/internal/core"
)

// CheckCRD returns a Finding indicating whether the specified CRD exists on the
// cluster. Use with RunCtx.CheckOnce() to cache results across tools.
func CheckCRD(ctx context.Context, cli, crdName string) core.Finding {
	if cli == "" {
		return core.Finding{
			Level:   "skip",
			Message: fmt.Sprintf("CRD %s: neither kubectl nor oc found on PATH", crdName),
		}
	}
	out, err := exec.CommandContext(ctx, cli, "get", "crd", crdName, "-o", "name").CombinedOutput() // #nosec G204 -- CLI is selected by cluster detection and args are structured.
	if err != nil {
		return core.Finding{
			Level:   "error",
			Message: fmt.Sprintf("CRD %s not found: %v: %s", crdName, err, strings.TrimSpace(string(out))),
		}
	}
	return core.Finding{
		Level:   "info",
		Message: fmt.Sprintf("CRD %s present", crdName),
	}
}

// CheckBinary returns a Finding indicating whether the specified binary is
// available on PATH. Use with RunCtx.CheckOnce() to cache results across tools.
func CheckBinary(binaryName string) core.Finding {
	_, err := exec.LookPath(binaryName)
	if err != nil {
		return core.Finding{
			Level:   "error",
			Message: fmt.Sprintf("host execution needs %q on PATH: %v", binaryName, err),
		}
	}
	return core.Finding{
		Level:   "info",
		Message: fmt.Sprintf("host binary found: %s", binaryName),
	}
}

// CheckStorageClass returns a Finding indicating whether the specified
// StorageClass exists on the cluster. Use with RunCtx.CheckOnce() to cache
// results across tools.
func CheckStorageClass(ctx context.Context, cli, scName string) core.Finding {
	if scName == "" {
		return core.Finding{
			Level:   "skip",
			Message: "storage class existence: no storage_class configured",
		}
	}
	if cli == "" {
		return core.Finding{
			Level:   "skip",
			Message: fmt.Sprintf("storage class %q existence: neither kubectl nor oc found on PATH", scName),
		}
	}
	out, err := exec.CommandContext(ctx, cli, "get", "storageclass", scName, "-o", "name").CombinedOutput() // #nosec G204 -- CLI is selected by cluster detection and args are structured.
	if err != nil {
		return core.Finding{
			Level:   "error",
			Message: fmt.Sprintf("storage class %q lookup failed: %v: %s", scName, err, strings.TrimSpace(string(out))),
		}
	}
	return core.Finding{
		Level:   "info",
		Message: fmt.Sprintf("storage class %q present", scName),
	}
}

// CheckVolumePopulator returns a Finding indicating whether the cluster has
// registered a populator for the requested CDI source kind. Use with
// RunCtx.CheckOnce() to cache results across tools.
func CheckVolumePopulator(ctx context.Context, cli, group, kind string) core.Finding {
	if cli == "" {
		return core.Finding{
			Level:   "skip",
			Message: fmt.Sprintf("VolumePopulator %s/%s: neither kubectl nor oc found on PATH", group, kind),
		}
	}
	hasPopulator, err := HasVolumePopulator(ctx, cli, group, kind)
	if err != nil {
		return core.Finding{
			Level:   "error",
			Message: fmt.Sprintf("checking VolumePopulator %s/%s: %v", group, kind, err),
		}
	}
	if hasPopulator {
		return core.Finding{
			Level:   "info",
			Message: fmt.Sprintf("VolumePopulator %s/%s registered", group, kind),
		}
	}
	return core.Finding{
		Level:   "info",
		Message: fmt.Sprintf("VolumePopulator %s/%s not registered", group, kind),
	}
}
