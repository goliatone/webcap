package presentation

import (
	"fmt"
	"io"

	pkgwebcap "github.com/goliatone/webcap"
)

func writeWorkflowCapture(w io.Writer, result pkgwebcap.WorkflowCaptureResult) error {
	if err := writeLines(w,
		"Workflow capture complete",
		fmt.Sprintf("Scenario: %s", result.ScenarioID),
		fmt.Sprintf("Scenario path: %s", result.ScenarioPath),
		fmt.Sprintf("Current dir: %s", result.CurrentDir),
		fmt.Sprintf("Captures: %d", len(result.Results)),
	); err != nil {
		return err
	}
	if len(result.Results) > 0 {
		if _, err := fmt.Fprintln(w, "Screens:"); err != nil {
			return err
		}
		for _, screen := range result.Results {
			if _, err := fmt.Fprintf(w, "  - %s %s -> %s", screen.ScreenID, screen.Label, screen.OutputPath); err != nil {
				return err
			}
			if metadataPath := firstNonEmpty(screen.MetadataPath, screen.Capture.MetadataPath); metadataPath != "" {
				if _, err := fmt.Fprintf(w, " metadata=%s", metadataPath); err != nil {
					return err
				}
			}
			if len(screen.Capture.Warnings) > 0 {
				if _, err := fmt.Fprintf(w, " (%d warnings)", len(screen.Capture.Warnings)); err != nil {
					return err
				}
			}
			if screen.Capture.Tiling != nil {
				if _, err := fmt.Fprintf(w, " tiled=%s tiles=%d/%d", screen.Capture.Tiling.Status, screen.Capture.Tiling.CompletedCount, screen.Capture.Tiling.TileCount); err != nil {
					return err
				}
			}
			if _, err := fmt.Fprintln(w); err != nil {
				return err
			}
		}
	}
	return nil
}

func writeWorkflowReport(w io.Writer, result pkgwebcap.WorkflowReportResult) error {
	if err := writeLines(w,
		"Workflow report complete",
		fmt.Sprintf("Scenario: %s", result.ScenarioID),
		fmt.Sprintf("Status: %s", firstNonEmpty(result.Status.Label, result.Status.Level)),
		fmt.Sprintf("Report: %s", result.ReportPath),
		fmt.Sprintf("Metadata: %s", result.MetadataPath),
		fmt.Sprintf("Diff dir: %s", result.DiffDir),
		fmt.Sprintf("Stories: %d", len(result.Stories)),
		fmt.Sprintf("Entries: %d", len(result.Entries)),
		fmt.Sprintf("Needs attention: %d", workflowAttentionCount(result.Entries)),
	); err != nil {
		return err
	}
	return nil
}

func writeWorkflowRunReport(w io.Writer, result WorkflowRunReportResult) error {
	if err := writeLines(w,
		"Workflow capture and report complete",
		fmt.Sprintf("Scenario: %s", result.Capture.ScenarioID),
		fmt.Sprintf("Captures: %d", len(result.Capture.Results)),
		fmt.Sprintf("Status: %s", firstNonEmpty(result.Report.Status.Label, result.Report.Status.Level)),
		fmt.Sprintf("Report: %s", result.Report.ReportPath),
		fmt.Sprintf("Metadata: %s", result.Report.MetadataPath),
		fmt.Sprintf("Needs attention: %d", workflowAttentionCount(result.Report.Entries)),
	); err != nil {
		return err
	}
	return nil
}

func workflowAttentionCount(entries []pkgwebcap.WorkflowReportEntry) int {
	count := 0
	for _, entry := range entries {
		switch entry.Status.Level {
		case "", "success", "pass", "passed", "ok":
			continue
		default:
			count++
		}
	}
	return count
}
