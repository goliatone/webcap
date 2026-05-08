package main

import (
	"testing"

	pkgwebcap "github.com/goliatone/webcap"
)

func TestParseShotCLI(t *testing.T) {
	invocation, err := parseCLI([]string{
		"shot",
		"--selectors", ".hero,.cta",
		"--output", "shots/home.png",
		"--viewport", "1600x900",
		"http://localhost:3000",
	})
	if err != nil {
		t.Fatalf("parseCLI returned error: %v", err)
	}
	if invocation.Command != "shot" {
		t.Fatalf("unexpected command: %s", invocation.Command)
	}
	if invocation.Shot.Request.URL != "http://localhost:3000" {
		t.Fatalf("unexpected url: %s", invocation.Shot.Request.URL)
	}
	if len(invocation.Shot.Request.Selectors) != 2 {
		t.Fatalf("unexpected selectors: %#v", invocation.Shot.Request.Selectors)
	}
	if invocation.Shot.Request.Viewport.Width != 1600 || invocation.Shot.Request.Viewport.Height != 900 {
		t.Fatalf("unexpected viewport: %+v", invocation.Shot.Request.Viewport)
	}
	if invocation.Shot.Request.Readiness != pkgwebcap.ReadinessComplete {
		t.Fatalf("unexpected readiness: %s", invocation.Shot.Request.Readiness)
	}
}

func TestParseMultiCLI(t *testing.T) {
	invocation, err := parseCLI([]string{"multi", "--output-dir", "shots", "webcap.yaml"})
	if err != nil {
		t.Fatalf("parseCLI returned error: %v", err)
	}
	if invocation.Multi.ManifestPath != "webcap.yaml" {
		t.Fatalf("unexpected manifest path: %s", invocation.Multi.ManifestPath)
	}
	if invocation.Multi.OutputDir != "shots" {
		t.Fatalf("unexpected output dir: %s", invocation.Multi.OutputDir)
	}
}

func TestParseDiffCLI(t *testing.T) {
	invocation, err := parseCLI([]string{"diff", "--threshold", "0.2", "--output", "diffs", "base", "compare"})
	if err != nil {
		t.Fatalf("parseCLI returned error: %v", err)
	}
	if invocation.Command != "diff" {
		t.Fatalf("unexpected command: %s", invocation.Command)
	}
	if invocation.Diff.Request.BasePath != "base" || invocation.Diff.Request.ComparePath != "compare" {
		t.Fatalf("unexpected diff paths: %#v", invocation.Diff.Request)
	}
	if invocation.Diff.Request.Threshold != 0.2 {
		t.Fatalf("unexpected threshold: %v", invocation.Diff.Request.Threshold)
	}
	if invocation.Diff.Request.OutputPath != "diffs" {
		t.Fatalf("unexpected output path: %s", invocation.Diff.Request.OutputPath)
	}
}

func TestParseShotCLIWithDeterministicFlags(t *testing.T) {
	invocation, err := parseCLI([]string{
		"shot",
		"--viewport-preset", "desktop-xl",
		"--readiness", "network_idle",
		"--readiness-idle", "900ms",
		"--disable-animations",
		"--reduced-motion",
		"--wait-for-fonts",
		"http://localhost:3000",
	})
	if err != nil {
		t.Fatalf("parseCLI returned error: %v", err)
	}
	if invocation.Shot.Request.ViewportPreset != "desktop-xl" {
		t.Fatalf("unexpected viewport preset: %s", invocation.Shot.Request.ViewportPreset)
	}
	if invocation.Shot.Request.Readiness != pkgwebcap.ReadinessNetworkIdle {
		t.Fatalf("unexpected readiness: %s", invocation.Shot.Request.Readiness)
	}
	if invocation.Shot.Request.ReadinessIdle != "900ms" {
		t.Fatalf("unexpected readiness idle: %s", invocation.Shot.Request.ReadinessIdle)
	}
	if !invocation.Shot.Request.DisableAnimations || !invocation.Shot.Request.ReducedMotion || !invocation.Shot.Request.WaitForFonts {
		t.Fatal("expected deterministic flags to be enabled")
	}
}

func TestParseShotCLIWithPlaywrightEngineFlags(t *testing.T) {
	invocation, err := parseCLI([]string{
		"shot",
		"--engine", "playwright",
		"--playwright-browser", "firefox",
		"--node-binary", "node",
		"--playwright-runtime-dir", "/tmp/runtime",
		"http://localhost:3000",
	})
	if err != nil {
		t.Fatalf("parseCLI returned error: %v", err)
	}
	if invocation.Browser.Engine != "playwright" {
		t.Fatalf("unexpected engine: %s", invocation.Browser.Engine)
	}
	if invocation.Browser.PlaywrightBrowser != "firefox" {
		t.Fatalf("unexpected playwright browser: %s", invocation.Browser.PlaywrightBrowser)
	}
	if invocation.Browser.PlaywrightRuntimeDir != "/tmp/runtime" {
		t.Fatalf("unexpected runtime dir: %s", invocation.Browser.PlaywrightRuntimeDir)
	}
}

func TestParseMCPServeCLI(t *testing.T) {
	invocation, err := parseCLI([]string{
		"mcp",
		"serve",
		"--engine", "playwright",
		"--playwright-browser", "webkit",
	})
	if err != nil {
		t.Fatalf("parseCLI returned error: %v", err)
	}
	if invocation.Command != "mcp" {
		t.Fatalf("unexpected command: %s", invocation.Command)
	}
	if invocation.MCP.Action != "serve" {
		t.Fatalf("unexpected mcp action: %s", invocation.MCP.Action)
	}
	if invocation.Browser.Engine != "playwright" {
		t.Fatalf("unexpected engine: %s", invocation.Browser.Engine)
	}
	if invocation.Browser.PlaywrightBrowser != "webkit" {
		t.Fatalf("unexpected browser: %s", invocation.Browser.PlaywrightBrowser)
	}
}

func TestParseWorkflowCaptureScenarioCLI(t *testing.T) {
	invocation, err := parseCLI([]string{
		"workflow",
		"capture-scenario",
		"--engine", "playwright",
		"--playwright-browser", "firefox",
		"--run-report",
		"workflow.yaml",
	})
	if err != nil {
		t.Fatalf("parseCLI returned error: %v", err)
	}
	if invocation.Command != "workflow" {
		t.Fatalf("unexpected command: %s", invocation.Command)
	}
	if invocation.Workflow.Action != "capture" {
		t.Fatalf("unexpected workflow action: %s", invocation.Workflow.Action)
	}
	if invocation.Workflow.ScenarioPath != "workflow.yaml" {
		t.Fatalf("unexpected scenario path: %s", invocation.Workflow.ScenarioPath)
	}
	if !invocation.Workflow.RunReport {
		t.Fatal("expected run-report=true")
	}
	if invocation.Browser.Engine != "playwright" || invocation.Browser.PlaywrightBrowser != "firefox" {
		t.Fatalf("unexpected browser config: %+v", invocation.Browser)
	}
	if !invocation.Browser.EngineSet || !invocation.Browser.PlaywrightBrowserSet {
		t.Fatal("expected explicit workflow browser flags to be tracked")
	}
}

func TestParseWorkflowCaptureMVPCLI(t *testing.T) {
	if _, err := parseCLI([]string{"workflow", "capture-mvp"}); err == nil {
		t.Fatal("expected capture-mvp to be unsupported in standalone CLI")
	}
}

func TestParseReportScenarioCLI(t *testing.T) {
	invocation, err := parseCLI([]string{"report", "scenario", "workflow.yaml"})
	if err != nil {
		t.Fatalf("parseCLI returned error: %v", err)
	}
	if invocation.Command != "report" {
		t.Fatalf("unexpected command: %s", invocation.Command)
	}
	if invocation.Report.Action != "generate" {
		t.Fatalf("unexpected report action: %s", invocation.Report.Action)
	}
	if invocation.Report.ScenarioPath != "workflow.yaml" {
		t.Fatalf("unexpected scenario path: %s", invocation.Report.ScenarioPath)
	}
}

func TestParseReportMVPCLI(t *testing.T) {
	if _, err := parseCLI([]string{"report", "mvp"}); err == nil {
		t.Fatal("expected report mvp to be unsupported in standalone CLI")
	}
}
