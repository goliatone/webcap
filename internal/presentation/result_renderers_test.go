package presentation

import (
	"bytes"
	"strings"
	"testing"
	"time"

	pkgwebcap "github.com/goliatone/webcap"
	"github.com/goliatone/webcap/pkg/agents/skills"
)

func TestHumanDiffOutput(t *testing.T) {
	result := pkgwebcap.DiffResult{
		Mode:         pkgwebcap.DiffModeImage,
		OutputPath:   "diffs/home.png",
		MetadataPath: "diffs/home.png.json",
		Threshold:    0.1,
		Summary: pkgwebcap.DiffSummary{
			ChangedFiles:       1,
			TotalChangedPixels: 25,
		},
	}
	var buf bytes.Buffer
	if err := New(Options{Format: FormatHuman}).Present(&buf, result); err != nil {
		t.Fatalf("Present returned error: %v", err)
	}
	for _, expected := range []string{"Diff complete", "Status: changed", "Output: diffs/home.png", "Changed pixels: 25"} {
		if !strings.Contains(buf.String(), expected) {
			t.Fatalf("expected output to contain %q, got:\n%s", expected, buf.String())
		}
	}
}

func TestHumanSemanticOutput(t *testing.T) {
	result := pkgwebcap.SemanticDiffResult{
		Provider:     "openai",
		Model:        "gpt-test",
		Summary:      "Copy changed",
		Verdict:      pkgwebcap.SemanticDiffVerdictNeedsReview,
		Severity:     pkgwebcap.SemanticDiffSeverityMinor,
		MetadataPath: "semantic.json",
		Differences: []pkgwebcap.SemanticDifference{{
			Description: "Primary CTA text changed",
			Severity:    pkgwebcap.SemanticDiffSeverityMinor,
		}},
	}
	var buf bytes.Buffer
	if err := New(Options{Format: FormatHuman}).Present(&buf, result); err != nil {
		t.Fatalf("Present returned error: %v", err)
	}
	for _, expected := range []string{"Semantic diff complete", "Verdict: needs_review", "Provider: openai", "Summary: Copy changed", "Primary CTA text changed"} {
		if !strings.Contains(buf.String(), expected) {
			t.Fatalf("expected output to contain %q, got:\n%s", expected, buf.String())
		}
	}
}

func TestHumanWorkflowOutput(t *testing.T) {
	result := pkgwebcap.WorkflowCaptureResult{
		ScenarioID:   "checkout",
		ScenarioPath: "workflow.yaml",
		CurrentDir:   "current",
		CapturedAt:   time.Date(2026, 5, 11, 10, 0, 0, 0, time.UTC),
		Results: []pkgwebcap.WorkflowScreenCaptureResult{{
			ScreenID:   "home",
			Label:      "Home",
			OutputPath: "current/home.png",
		}},
	}
	var buf bytes.Buffer
	if err := New(Options{Format: FormatHuman}).Present(&buf, result); err != nil {
		t.Fatalf("Present returned error: %v", err)
	}
	for _, expected := range []string{"Workflow capture complete", "Scenario: checkout", "Captures: 1", "home Home -> current/home.png"} {
		if !strings.Contains(buf.String(), expected) {
			t.Fatalf("expected output to contain %q, got:\n%s", expected, buf.String())
		}
	}
}

func TestHumanWorkflowReportOutput(t *testing.T) {
	result := pkgwebcap.WorkflowReportResult{
		ScenarioID:   "checkout",
		ReportPath:   "reports/index.html",
		MetadataPath: "reports/report.json",
		DiffDir:      "diffs",
		Entries: []pkgwebcap.WorkflowReportEntry{
			{
				ScreenID: "home",
				Status:   pkgwebcap.WorkflowReviewStatus{Level: "warning", Label: "Capture Issues"},
			},
			{
				ScreenID: "checkout",
				Status:   pkgwebcap.WorkflowReviewStatus{Level: "info", Label: "Needs Review"},
			},
			{
				ScreenID: "settings",
				Status:   pkgwebcap.WorkflowReviewStatus{Level: "success", Label: "Ready"},
			},
		},
		Status: pkgwebcap.WorkflowReviewStatus{Level: "warning", Label: "Attention Required"},
	}
	var buf bytes.Buffer
	if err := New(Options{Format: FormatHuman}).Present(&buf, result); err != nil {
		t.Fatalf("Present returned error: %v", err)
	}
	for _, expected := range []string{"Workflow report complete", "Report: reports/index.html", "Status: Attention Required", "Needs attention: 2"} {
		if !strings.Contains(buf.String(), expected) {
			t.Fatalf("expected output to contain %q, got:\n%s", expected, buf.String())
		}
	}
}

func TestHumanWorkflowRunReportOutput(t *testing.T) {
	result := WorkflowRunReportResult{
		Capture: pkgwebcap.WorkflowCaptureResult{
			ScenarioID: "checkout",
			Results:    []pkgwebcap.WorkflowScreenCaptureResult{{ScreenID: "home"}},
		},
		Report: pkgwebcap.WorkflowReportResult{
			ReportPath:   "reports/index.html",
			MetadataPath: "reports/report.json",
			Status:       pkgwebcap.WorkflowReviewStatus{Level: "success", Label: "Ready"},
		},
	}
	var buf bytes.Buffer
	if err := New(Options{Format: FormatHuman}).Present(&buf, result); err != nil {
		t.Fatalf("Present returned error: %v", err)
	}
	for _, expected := range []string{"Workflow capture and report complete", "Captures: 1", "Report: reports/index.html", "Status: Ready"} {
		if !strings.Contains(buf.String(), expected) {
			t.Fatalf("expected output to contain %q, got:\n%s", expected, buf.String())
		}
	}
}

func TestHumanSkillInstallOutput(t *testing.T) {
	result := skills.InstallResult{
		Agent:        skills.AgentCodex,
		SkillName:    "webcap-agent",
		Destination:  "/tmp/webcap-agent",
		FilesWritten: 3,
	}
	var buf bytes.Buffer
	if err := New(Options{Format: FormatHuman}).Present(&buf, result); err != nil {
		t.Fatalf("Present returned error: %v", err)
	}
	for _, expected := range []string{"Skill install complete", "Agent: codex", "Destination: /tmp/webcap-agent", "Files written: 3"} {
		if !strings.Contains(buf.String(), expected) {
			t.Fatalf("expected output to contain %q, got:\n%s", expected, buf.String())
		}
	}
}
