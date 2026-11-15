package main

import (
	"sort"

	"github.com/spf13/cobra"

	"github.com/openshift-storage-cert/storage-cert-harness/internal/catalog"
	"github.com/openshift-storage-cert/storage-cert-harness/internal/config"
	"github.com/openshift-storage-cert/storage-cert-harness/internal/core"
	"github.com/openshift-storage-cert/storage-cert-harness/internal/kb"
	"github.com/openshift-storage-cert/storage-cert-harness/internal/orchestrator"
)

func newPreflightCmd() *cobra.Command {
	var (
		catalogPath  string
		filter       config.Filter
		planPath     string
		backendsPath string
		backendName  string
	)
	var verbose bool
	cmd := &cobra.Command{
		Use:   "preflight",
		Short: "Run preflight checks for the tools selected by the catalog/filter",
		RunE: func(cmd *cobra.Command, _ []string) error {
			logger := newLoggerTo(verbose, cmd.OutOrStdout(), nil)
			cfg := config.Config{Filter: filter}
			resolved, err := configureExecution(cmd, &cfg, planPath, backendsPath, &backendName, false, logger)
			if err != nil {
				return err
			}
			cat, err := catalog.Load(catalogPath)
			if err != nil {
				return err
			}
			trs, err := orchestrator.ResolveExecutions(kb.FromCatalog(cat), cfg)
			if err != nil {
				return err
			}
			rc := &core.RunCtx{RunID: "preflight", Logger: logger, Backend: resolved}
			out := cmd.OutOrStdout()
			byTool := map[string][]core.TestRequirement{}
			for _, tr := range trs {
				key := tr.AutomationTool
				if tr.Variant != "" {
					key = core.ExecutionLabel(key+":"+tr.ID, tr.Variant)
				}
				byTool[key] = append(byTool[key], tr)
			}
			results := preflightOutput{out: out}
			tools := make([]string, 0, len(byTool))
			for tool := range byTool {
				tools = append(tools, tool)
			}
			sort.Strings(tools)
			for _, tool := range tools {
				ts := byTool[tool]
				if ts[0].AutomationTool == "" || ts[0].AutomationTool == core.CheckManual {
					for _, tr := range ts {
						results.add("SKIP", core.ExecutionLabel(tr.ID, tr.Variant), "no automation tool; manual check required")
					}
					continue
				}
				ti, ok := orchestrator.ResolveTool(ts[0])
				if !ok {
					results.add("FAIL", tool, "tool not registered")
					continue
				}
				if ti.Preflight == nil {
					results.add("SKIP", tool, "no preflight checks implemented")
					continue
				}
				findings, err := ti.Preflight.Check(cmd.Context(), rc, core.NewBag(), ts)
				for _, f := range findings {
					switch f.Level {
					case "info":
						results.add("PASS", tool, f.Message)
					case "warn":
						results.add("WARN", tool, f.Message)
					case "skip":
						results.add("SKIP", tool, f.Message)
					default:
						results.add("FAIL", tool, f.Message)
					}
				}
				if err != nil {
					results.add("FAIL", tool, err.Error())
				} else if len(findings) == 0 {
					results.add("SKIP", tool, "preflight returned no check results")
				}
			}
			if len(trs) == 0 {
				logger.Info("No test requirements selected.")
			}
			return results.finish()
		},
	}
	cmd.Flags().StringVar(&catalogPath, "catalog", "", "path to catalog.json (required)")
	cmd.Flags().StringVar(&planPath, "plan", "", "path to a test plan (YAML); provides selection/backend defaults")
	cmd.Flags().StringVar(&backendsPath, "backends", "", "path to a backends file (YAML)")
	cmd.Flags().StringVar(&backendName, "backend", "", "name of the active storage backend (overrides the plan)")
	cmd.Flags().StringSliceVar(&filter.Tools, "tool", nil, "restrict to these automation tools")
	cmd.Flags().StringSliceVar(&filter.IDs, "id", nil, "restrict to these TR ids")
	cmd.Flags().IntSliceVar(&filter.PartnerLevels, "partner-level", nil, "restrict to these partner levels (1|2|3)")
	cmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "debug logging")
	_ = cmd.MarkFlagRequired("catalog")
	return cmd
}
