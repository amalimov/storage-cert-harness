package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/openshift-storage-cert/storage-cert-harness/internal/catalog"
	"github.com/openshift-storage-cert/storage-cert-harness/internal/registry"
)

func newListCmd() *cobra.Command {
	var catalogPath string
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List registered tools and (optionally) catalog test requirements",
		RunE: func(cmd *cobra.Command, _ []string) error {
			out := cmd.OutOrStdout()
			_, _ = fmt.Fprintln(out, "Registered tools:")
			for _, name := range registry.Names() {
				ti, _ := registry.Get(name)
				_, _ = fmt.Fprintf(out, "  - %s (image: %s) provides %v\n", ti.Name, ti.Image, ti.Provides)
			}
			if catalogPath != "" {
				cat, err := catalog.Load(catalogPath)
				if err != nil {
					return err
				}
				_, _ = fmt.Fprintf(out, "\nCatalog test requirements (%d), schema %s, KB commit %s:\n",
					len(cat.TestRequirements), cat.SchemaVersion, cat.Provenance.KBGitCommit)
				for _, tr := range cat.TestRequirements {
					_, _ = fmt.Fprintf(out, "  - %s L%d [tool=%s] %q (sla=%d/%s, checks=%d/%s)\n",
						tr.ID, tr.PartnerLevel, tr.AutomationTool, tr.Title,
						tr.SLACount, tr.SLAStatus, tr.ChecksCount, tr.ChecksStatus)
				}
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&catalogPath, "catalog", "", "path to catalog.json (publishable)")
	return cmd
}
