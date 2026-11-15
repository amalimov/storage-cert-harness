package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/openshift-storage-cert/storage-cert-harness/internal/attestation"
	"github.com/openshift-storage-cert/storage-cert-harness/internal/catalog"
	"github.com/openshift-storage-cert/storage-cert-harness/internal/clustercheck"
	"github.com/openshift-storage-cert/storage-cert-harness/internal/config"
	"github.com/openshift-storage-cert/storage-cert-harness/internal/core"
	"github.com/openshift-storage-cert/storage-cert-harness/internal/kb"
	"github.com/openshift-storage-cert/storage-cert-harness/internal/orchestrator"
	"github.com/openshift-storage-cert/storage-cert-harness/internal/report"
	"github.com/openshift-storage-cert/storage-cert-harness/internal/safefs"
	"github.com/openshift-storage-cert/storage-cert-harness/internal/thresholds"
)

func newRunCmd() *cobra.Command {
	var (
		cfg              config.Config
		thresholdsPath   string
		outputDir        string
		verbose          bool
		planPath         string
		backendsPath     string
		backendName      string
		attestationsPath string
		attnQuestions    string
		noAttestations   bool
		scenario         string
		continueFrom     string
		retryFailed      bool
		skipEnv          bool
		runTimeout       time.Duration
	)
	cmd := &cobra.Command{
		Use:   "run",
		Short: "Run selected certification tests and emit a report",
		RunE: func(cmd *cobra.Command, _ []string) (runErr error) {
			var logFile io.WriteCloser
			var closeLog func()
			if outputDir != "" {
				root, err := safefs.OpenRootDir(outputDir, 0o755)
				if err != nil {
					return err
				}
				f, err := root.OpenFile("run.log", os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
				if err != nil {
					_ = root.Close()
					return err
				}
				logFile = f
				closeLog = func() {
					_ = f.Close()
					_ = root.Close()
				}
				defer func() {
					if runErr != nil {
						_, _ = fmt.Fprintf(logFile, "error: %v\n", runErr)
					}
					closeLog()
				}()
			}
			logger := newLoggerWithOutput(verbose, logFile)
			resolved, err := configureExecution(cmd, &cfg, planPath, backendsPath, &backendName, true, logger)
			if err != nil {
				return err
			}
			if cmd.Flags().Changed("scenario") {
				cfg.Scenario = scenario
			}

			cat, err := catalog.Load(cfg.CatalogPath)
			if err != nil {
				return err
			}

			// Resolve thresholds: --thresholds flag, then HARNESS_THRESHOLDS.
			path := thresholdsPath
			if path == "" {
				path = os.Getenv("HARNESS_THRESHOLDS")
			}

			// Build the execution TRs: join if a thresholds bundle is supplied
			// (verifies provenance/commit match + id parity), else catalog-only.
			var trs []core.TestRequirement
			if path != "" {
				th, err := thresholds.Load(path)
				if err != nil {
					return err
				}
				trs, err = kb.Join(cat, th)
				if err != nil {
					return err
				}
			} else {
				trs = kb.FromCatalog(cat)
			}

			selected := orchestrator.SelectTRs(trs, cfg.Filter)
			if len(selected) == 0 {
				return fmt.Errorf("no test requirements selected")
			}
			// Refuse a run that needs SLA/check numbers without a bundle (5273).
			if path == "" {
				for _, tr := range selected {
					if tr.SLACount > 0 || tr.ChecksCount > 0 {
						return fmt.Errorf("TR %q has scorable criteria (%d sla, %d checks); supply --thresholds or HARNESS_THRESHOLDS",
							tr.ID, tr.SLACount, tr.ChecksCount)
					}
				}
			}

			ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
			defer stop()

			runID := time.Now().UTC().Format("20060102-150405")
			workdir := cfg.WorkDir
			if workdir == "" {
				workdir = filepath.Join(os.TempDir(), "harness-"+runID)
			}
			// Absolute: tools use it as both cmd.Dir and a results path (must agree).
			if abs, err := filepath.Abs(workdir); err == nil {
				workdir = abs
			}
			rc := &core.RunCtx{RunID: runID, WorkDir: workdir, Logger: logger, LogOutput: logFile, Backend: resolved}

			// Resolve the prior report and self attestations before running cluster
			// tests, so the human gate is the first run phase.
			var prior *core.Report
			if continueFrom != "" {
				p, err := report.ReadJSON(continueFrom)
				if err != nil {
					return err
				}
				if resolved != nil && p.Environment.Platform.Backend != "" && p.Environment.Platform.Backend != resolved.Name {
					return fmt.Errorf("--continue-from: prior report backend %q != current backend %q; refusing to merge mismatched runs",
						p.Environment.Platform.Backend, resolved.Name)
				}
				prior = &p
			}
			if prior != nil {
				before := len(selected)
				selected = orchestrator.ResumeExecutionsWithOptions(selected, *prior, cfg.Scenario, retryFailed)
				logger.Info("continue-from: selecting incomplete executions", "previously_selected", before, "resuming", len(selected), "skipped_completed", before-len(selected), "scenario", cfg.Scenario)
			}
			var attestations []core.Attestation
			switch {
			case prior != nil && len(prior.Attestations) > 0:
				logger.Info("continue-from: inheriting attestations from prior report", "count", len(prior.Attestations))
			case attestationsPath != "":
				atts, err := loadAttestations(attestationsPath)
				if err != nil {
					return err
				}
				attestations = atts
			case noAttestations:
			case isInteractive(os.Stdin):
				qs, err := attestation.LoadQuestions(attnQuestions)
				if err != nil {
					return err
				}
				atts, err := attestation.Prompt(ctx, qs, os.Stdin, cmd.ErrOrStderr())
				if err != nil {
					return err
				}
				attestations = atts
			default:
				logger.Warn("no attestations provided and not interactive; skipping (use --attestations or --no-attestations to silence)")
			}

			partKind := "initial"
			if prior != nil {
				partKind = "resume"
			}
			startedAt := time.Now().UTC().Format(time.RFC3339)
			initial := report.NewRunning(runID, cat.SchemaVersion, cat.Provenance, selected, partKind, startedAt)
			for i := range initial.Executions {
				initial.Executions[i].Scenario = cfg.Scenario
			}
			if prior != nil {
				initial.Attestations = prior.Attestations
				merged, mergeErr := report.Merge(*prior, initial)
				if mergeErr != nil {
					return mergeErr
				}
				initial = merged
			} else {
				initial.Attestations = attestations
			}
			checkpoint := newReportCheckpoint(outputDir, initial)
			if outputDir != "" {
				checkpoint.Save(initial)
				if checkpointErr := checkpoint.Err(); checkpointErr != nil {
					return fmt.Errorf("write initial report checkpoint: %w", checkpointErr)
				}
			}
			finalOutputWritten := false
			defer func() {
				if !finalOutputWritten && runErr != nil {
					checkpoint.Finalize(core.RunStatusPartial, runErr.Error())
				}
			}()

			rc.PartKind = partKind
			rc.Checkpoint = checkpoint.Save
			rc.RecordIncident = checkpoint.RecordIncident
			runCtx := ctx
			var cancelRun context.CancelFunc
			if runTimeout > 0 {
				runCtx, cancelRun = context.WithTimeout(ctx, runTimeout)
				defer cancelRun()
			}
			// selected, not trs: --continue-from narrows this list to the
			// executions still outstanding, and handing the orchestrator the
			// full catalog instead would re-derive the selection and re-run
			// everything the prior report had already completed.
			rep, err := orchestrator.Run(runCtx, cfg, selected, cat.Provenance, cat.SchemaVersion, rc)
			if err != nil {
				return err
			}
			if checkpointErr := checkpoint.Err(); checkpointErr != nil {
				return fmt.Errorf("write report checkpoint: %w", checkpointErr)
			}

			// Optionally continue from a prior report: reuse its environment (no
			// second collection) and merge this run's verdicts into it. See ADR-0014.
			// Enrich the report with the run environment (auto-collected from the
			// cluster). Best-effort; see ADR-0014. The CSI driver is resolved from the
			// StorageClass under test (the active backend), not the cluster default.
			// Skipped when continuing from a prior report (its environment is reused)
			// or when --skip-env is set.
			switch {
			case prior != nil:
				logger.Info("continue-from: reusing environment from prior report", "path", continueFrom)
			case skipEnv:
				logger.Info("skipping environment collection (--skip-env)")
			default:
				scUnderTest := ""
				if resolved != nil {
					scUnderTest = resolved.StorageClass
				}
				if cli := clustercheck.KubeCLI(); cli != "" {
					rep.Environment = clustercheck.CollectEnvironment(ctx, cli, scUnderTest)
				}
				if resolved != nil {
					rep.Environment.Platform.Backend = resolved.Name
				}
			}

			rep.Attestations = attestations

			if prior != nil {
				merged, err := report.Merge(*prior, rep)
				if err != nil {
					return err
				}
				rep = merged
			}

			if outputDir != "" {
				var savedAttestations []core.Attestation
				if len(attestations) > 0 && attestationsPath == "" {
					savedAttestations = attestations
				}
				if err := writeReportFiles(outputDir, rep, savedAttestations); err != nil {
					return err
				}
				finalOutputWritten = true
			}
			if err := report.WriteMarkdown(cmd.OutOrStdout(), rep); err != nil {
				return err
			}

			if rep.Status != core.RunStatusComplete {
				return fmt.Errorf("run did not complete: status=%s reason=%s", rep.Status, rep.CompletionReason)
			}
			if rep.Summary.Fail > 0 || rep.Summary.Error > 0 {
				return fmt.Errorf("certification failed: %d fail, %d error", rep.Summary.Fail, rep.Summary.Error)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&cfg.CatalogPath, "catalog", "", "path to catalog.json (required)")
	cmd.Flags().StringVar(&thresholdsPath, "thresholds", "", "path to real thresholds.json (or set HARNESS_THRESHOLDS)")
	cmd.Flags().StringVar(&cfg.WorkDir, "workdir", "", "working directory for run artifacts")
	cmd.Flags().IntVar(&cfg.Concurrency, "concurrency", 1, "max tools run in parallel (respects per-tool parallel_safe/exclusivity_groups)")
	cmd.Flags().StringSliceVar(&cfg.ExtraMetrics, "report-metric", nil, "add report metrics for each selected TR (name or name@percentile; repeat or comma-separate)")
	cmd.Flags().StringVar(&outputDir, "output", "", "directory to write reports and run.log")
	cmd.Flags().StringVar(&planPath, "plan", "", "path to a test plan (YAML); provides selection/concurrency/backend defaults")
	cmd.Flags().StringVar(&backendsPath, "backends", "", "path to a backends file (YAML)")
	cmd.Flags().StringVar(&backendName, "backend", "", "name of the active storage backend (overrides the plan)")
	cmd.Flags().StringVar(&attestationsPath, "attestations", "", "path to attestation JSON (self-reported, unobservable claims)")
	cmd.Flags().StringVar(&attnQuestions, "attestation-questions", "", "path to a custom attestation question set (YAML); defaults to the built-in set")
	cmd.Flags().BoolVar(&noAttestations, "no-attestations", false, "do not prompt for manual attestations")
	cmd.Flags().StringVar(&scenario, "scenario", "", "run label stamped onto every verdict (e.g. under-pressure); overrides the plan")
	cmd.Flags().StringVar(&continueFrom, "continue-from", "", "path to a prior report.json to reuse its environment and merge this run into")
	cmd.Flags().BoolVar(&retryFailed, "retry-failed", false, "with --continue-from, rerun previously completed fail/error executions")
	cmd.Flags().BoolVar(&skipEnv, "skip-env", false, "skip cluster environment collection")
	cmd.Flags().DurationVar(&runTimeout, "timeout", 0, "cancel the run after this duration (0 means no harness timeout)")
	cmd.Flags().StringSliceVar(&cfg.Filter.Tools, "tool", nil, "restrict to these automation tools")
	cmd.Flags().StringSliceVar(&cfg.Filter.IDs, "id", nil, "restrict to these TR ids")
	cmd.Flags().IntSliceVar(&cfg.Filter.PartnerLevels, "partner-level", nil, "restrict to these partner levels (1|2|3)")
	cmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "debug logging")
	_ = cmd.MarkFlagRequired("catalog")
	return cmd
}
