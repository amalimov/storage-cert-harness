package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/openshift-storage-cert/storage-cert-harness/internal/attestation"
	"github.com/openshift-storage-cert/storage-cert-harness/internal/core"
	"github.com/openshift-storage-cert/storage-cert-harness/internal/report"
)

func newAttestCmd() *cobra.Command {
	var (
		reportPath       string
		attestationsPath string
		attnQuestions    string
		outputDir        string
	)
	cmd := &cobra.Command{
		Use:   "attest",
		Short: "Add attestations to an existing report without rerunning tests",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			rep, err := report.ReadJSON(reportPath)
			if err != nil {
				return err
			}

			var attestations []core.Attestation
			switch {
			case attestationsPath != "":
				attestations, err = loadAttestations(attestationsPath)
				if err != nil {
					return err
				}
			case isInteractive(os.Stdin):
				qs, loadErr := attestation.LoadQuestions(attnQuestions)
				if loadErr != nil {
					return loadErr
				}
				attestations, err = attestation.Prompt(cmd.Context(), qs, os.Stdin, cmd.ErrOrStderr())
				if err != nil {
					return err
				}
			default:
				return fmt.Errorf("provide --attestations or run interactively to add attestations")
			}
			if len(attestations) == 0 {
				return fmt.Errorf("no attestations provided")
			}

			rep.Attestations = attestations
			if outputDir == "" {
				outputDir = filepath.Dir(reportPath)
			}
			if err := writeReportFiles(outputDir, rep, attestations); err != nil {
				return err
			}
			return report.WriteMarkdown(cmd.OutOrStdout(), rep)
		},
	}
	cmd.Flags().StringVar(&reportPath, "report", "", "path to an existing report.json")
	cmd.Flags().StringVar(&attestationsPath, "attestations", "", "path to attestation JSON (self-reported, unobservable claims)")
	cmd.Flags().StringVar(&attnQuestions, "attestation-questions", "", "path to a custom attestation question set (YAML); defaults to the built-in set")
	cmd.Flags().StringVar(&outputDir, "output", "", "directory to write report.json / report.md / report.junit.xml (defaults to the input report directory)")
	_ = cmd.MarkFlagRequired("report")
	return cmd
}
