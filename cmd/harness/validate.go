package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/openshift-storage-cert/storage-cert-harness/internal/catalog"
	"github.com/openshift-storage-cert/storage-cert-harness/internal/kb"
	"github.com/openshift-storage-cert/storage-cert-harness/internal/thresholds"
)

func newValidateCmd() *cobra.Command {
	var catalogPath, thresholdsPath string
	cmd := &cobra.Command{
		Use:   "validate",
		Short: "Validate a catalog and (optionally) a thresholds bundle, verifying they join",
		RunE: func(cmd *cobra.Command, _ []string) error {
			out := cmd.OutOrStdout()
			cat, err := catalog.Load(catalogPath)
			if err != nil {
				return err
			}
			_, _ = fmt.Fprintf(out, "catalog OK: %d test requirements, schema %s, KB commit %s\n",
				len(cat.TestRequirements), cat.SchemaVersion, cat.Provenance.KBGitCommit)
			if thresholdsPath != "" {
				th, err := thresholds.Load(thresholdsPath)
				if err != nil {
					return err
				}
				_, _ = fmt.Fprintf(out, "thresholds OK: %d test requirements, schema %s, KB commit %s\n",
					len(th.TestRequirements), th.SchemaVersion, th.Provenance.KBGitCommit)
				trs, err := kb.Join(cat, th)
				if err != nil {
					return err
				}
				_, _ = fmt.Fprintf(out, "join OK: %d joined test requirements (catalog + thresholds agree)\n", len(trs))
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&catalogPath, "catalog", "", "path to catalog.json (required)")
	cmd.Flags().StringVar(&thresholdsPath, "thresholds", "", "path to thresholds.json (optional)")
	_ = cmd.MarkFlagRequired("catalog")
	return cmd
}
