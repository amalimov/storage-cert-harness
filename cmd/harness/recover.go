package main

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/openshift-storage-cert/storage-cert-harness/internal/core"
	"github.com/openshift-storage-cert/storage-cert-harness/internal/report"
	"github.com/spf13/cobra"
)

func newRecoverCmd() *cobra.Command {
	var reportPath, outputDir, reason string
	cmd := &cobra.Command{
		Use:   "recover",
		Short: "Finalize a stale running report after an interrupted process",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if reportPath == "" {
				if outputDir == "" {
					return fmt.Errorf("provide --report or --output")
				}
				reportPath = filepath.Join(outputDir, "report.json")
			}
			rep, err := report.ReadJSON(reportPath)
			if err != nil {
				return err
			}
			if rep.Status != core.RunStatusRunning {
				return fmt.Errorf("report status is %q; only running reports can be recovered", rep.Status)
			}
			if reason == "" {
				reason = "process ended without finalizing the run"
			}
			rep.Incidents = append(rep.Incidents, core.Incident{
				ID:                  "run-abandoned",
				Type:                "abandoned",
				Severity:            "error",
				Message:             reason,
				BlocksCertification: true,
				Impact:              "the report was recovered from a stale running checkpoint",
			})
			report.Finalize(&rep, core.RunStatusAbandoned, reason, time.Now().UTC().Format(time.RFC3339))
			if outputDir == "" {
				outputDir = filepath.Dir(reportPath)
			}
			if err := writeReportFiles(outputDir, rep, nil); err != nil {
				return err
			}
			return report.WriteMarkdown(cmd.OutOrStdout(), rep)
		},
	}
	cmd.Flags().StringVar(&reportPath, "report", "", "path to a stale report.json")
	cmd.Flags().StringVar(&outputDir, "output", "", "directory containing report.json and receiving recovered reports")
	cmd.Flags().StringVar(&reason, "reason", "", "reason to record for the abandoned run")
	return cmd
}
