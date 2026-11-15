package main

import (
	"sync"
	"time"

	"github.com/openshift-storage-cert/storage-cert-harness/internal/core"
	"github.com/openshift-storage-cert/storage-cert-harness/internal/report"
)

// reportCheckpoint serializes concurrent orchestrator snapshots and keeps the
// latest report available for recovery if the process exits unexpectedly.
type reportCheckpoint struct {
	mu        sync.Mutex
	outputDir string
	latest    core.Report
	lastErr   error
}

func newReportCheckpoint(outputDir string, initial core.Report) *reportCheckpoint {
	return &reportCheckpoint{outputDir: outputDir, latest: initial}
}

func (c *reportCheckpoint) Save(snapshot core.Report) {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()

	merged := snapshot
	if c.latest.RunID != "" {
		if candidate, err := report.Merge(c.latest, snapshot); err == nil {
			merged = candidate
		} else {
			c.lastErr = err
		}
	}
	c.latest = merged
	if c.outputDir == "" {
		return
	}
	if err := writeReportFiles(c.outputDir, merged, nil); err != nil {
		c.lastErr = err
	}
}

func (c *reportCheckpoint) RecordIncident(incident core.Incident) {
	if c == nil {
		return
	}
	c.mu.Lock()
	latest := c.latest
	for _, existing := range latest.Incidents {
		if existing.ID != "" && existing.ID == incident.ID {
			c.mu.Unlock()
			return
		}
	}
	latest.Incidents = append(latest.Incidents, incident)
	report.RefreshLifecycle(&latest)
	c.mu.Unlock()
	c.Save(latest)
}

func (c *reportCheckpoint) Finalize(status core.RunStatus, reason string) {
	if c == nil {
		return
	}
	c.mu.Lock()
	latest := c.latest
	report.Finalize(&latest, status, reason, time.Now().UTC().Format(time.RFC3339))
	c.latest = latest
	if c.outputDir != "" {
		if err := writeReportFiles(c.outputDir, latest, nil); err != nil {
			c.lastErr = err
		}
	}
	c.mu.Unlock()
}

func (c *reportCheckpoint) Err() error {
	if c == nil {
		return nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.lastErr
}

func (c *reportCheckpoint) Latest() core.Report {
	if c == nil {
		return core.Report{}
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.latest
}
