package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sort"

	"github.com/spf13/cobra"

	"github.com/openshift-storage-cert/storage-cert-harness/internal/attestation"
	"github.com/openshift-storage-cert/storage-cert-harness/internal/backend"
	"github.com/openshift-storage-cert/storage-cert-harness/internal/config"
	"github.com/openshift-storage-cert/storage-cert-harness/internal/core"
	"github.com/openshift-storage-cert/storage-cert-harness/internal/plan"
	"github.com/openshift-storage-cert/storage-cert-harness/internal/report"
	"github.com/openshift-storage-cert/storage-cert-harness/internal/safefs"
)

func newLoggerWithOutput(verbose bool, logOutput io.Writer) *slog.Logger {
	return newLoggerTo(verbose, os.Stderr, logOutput)
}

func newLoggerTo(verbose bool, terminal io.Writer, logOutput io.Writer) *slog.Logger {
	lvl := slog.LevelInfo
	if verbose {
		lvl = slog.LevelDebug
	}
	output := terminal
	if logOutput != nil {
		output = io.MultiWriter(terminal, logOutput)
	}
	return slog.New(slog.NewTextHandler(output, &slog.HandlerOptions{Level: lvl}))
}

// isInteractive reports whether f is a terminal (character device), so the run
// only prompts for attestations when a human can answer — CI and replay skip it.
func isInteractive(f *os.File) bool {
	fi, err := f.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}

func writeReportFiles(outputDir string, rep core.Report, attestations []core.Attestation) error {
	report.RefreshLifecycle(&rep)
	if len(attestations) > 0 {
		if err := attestation.WriteFile(filepath.Join(outputDir, "attestations.json"), attestations); err != nil {
			return err
		}
	}
	if err := safefs.WriteWriterAtomic(filepath.Join(outputDir, "report.json"), 0o600, func(w io.Writer) error {
		return report.WriteJSON(w, rep)
	}); err != nil {
		return err
	}
	if err := safefs.WriteWriterAtomic(filepath.Join(outputDir, "report.md"), 0o600, func(w io.Writer) error {
		return report.WriteMarkdown(w, rep)
	}); err != nil {
		return err
	}
	return safefs.WriteWriterAtomic(filepath.Join(outputDir, "report.junit.xml"), 0o600, func(w io.Writer) error {
		return (report.JUnitExporter{}).Export(w, rep)
	})
}

// loadAttestations reads a partner attestation file of the form
// {"signed_by": "...", "claims": {"<claim>": "<value>", ...}} into sorted
// core.Attestation entries. These are self-reported, unobservable claims only
// (ADR-0014).
func loadAttestations(path string) ([]core.Attestation, error) {
	b, err := safefs.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("attestations: %w", err)
	}
	var doc struct {
		SignedBy string            `json:"signed_by"`
		Claims   map[string]string `json:"claims"`
	}
	if err := json.Unmarshal(b, &doc); err != nil {
		return nil, fmt.Errorf("attestations: %w", err)
	}
	keys := make([]string, 0, len(doc.Claims))
	for k := range doc.Claims {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := make([]core.Attestation, 0, len(keys))
	for _, k := range keys {
		out = append(out, core.Attestation{Claim: k, Value: doc.Claims[k], SignedBy: doc.SignedBy})
	}
	return out, nil
}

func configureExecution(cmd *cobra.Command, cfg *config.Config, planPath, backendsPath string, backendName *string, applyConcurrency bool, logger *slog.Logger) (*core.ResolvedBackend, error) {
	if planPath != "" {
		p, err := plan.Load(planPath)
		if err != nil {
			return nil, err
		}
		if !cmd.Flags().Changed("tool") && !cmd.Flags().Changed("id") && !cmd.Flags().Changed("partner-level") {
			cfg.Filter = p.Filter()
		}
		if applyConcurrency && !cmd.Flags().Changed("concurrency") && p.Concurrency > 0 {
			cfg.Concurrency = p.Concurrency
		}
		if !cmd.Flags().Changed("backend") && p.Backend != "" {
			*backendName = p.Backend
		}
		cfg.Scenario = p.Scenario
		cfg.Overrides = p.Overrides
		cfg.Variants = p.Variants
		cfg.Scheduling = p.SchedulingOverrides()
		cfg.ReportMetrics = p.ReportMetrics
		logger.Info("loaded plan", "name", p.Name, "path", planPath)
	}
	if *backendName == "" {
		return nil, nil
	}
	if backendsPath == "" {
		return nil, fmt.Errorf("backend %q selected but no --backends file given", *backendName)
	}
	set, err := backend.Load(backendsPath)
	if err != nil {
		return nil, err
	}
	b, ok := set.Get(*backendName)
	if !ok {
		return nil, fmt.Errorf("backend %q not found in %s", *backendName, backendsPath)
	}
	resolved, err := backend.Resolve(b, nil, logger)
	if err != nil {
		return nil, err
	}
	logger.Info("using backend", "name", resolved.Name, "vendor", resolved.Vendor, "storage_class", resolved.StorageClass)
	return resolved, nil
}
