package main

import (
	"github.com/spf13/cobra"

	// Tool integrations — add a line here to compile in a new tool.
	_ "github.com/openshift-storage-cert/storage-cert-harness/internal/tools/example"
	_ "github.com/openshift-storage-cert/storage-cert-harness/internal/tools/kubeburner"
	_ "github.com/openshift-storage-cert/storage-cert-harness/internal/tools/kubeburnerocp"
	_ "github.com/openshift-storage-cert/storage-cert-harness/internal/tools/virtbench"
)

// version is overridable at build time: -ldflags "-X main.version=..."
var version = "0.0.0-dev"

func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:           "harness",
		Short:         "Storage certification test harness",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.AddCommand(
		newVersionCmd(),
		newListCmd(),
		newValidateCmd(),
		newPreflightCmd(),
		newRunCmd(),
		newAttestCmd(),
		newRecoverCmd(),
		newSimulateReportCmd(),
	)
	return root
}
