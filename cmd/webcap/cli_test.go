package main

import (
	"errors"
	"testing"

	pkgwebcap "github.com/goliatone/webcap"
	"github.com/goliatone/webcap/pkg/agents/skills"
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
	if invocation.Output.Format != "human" {
		t.Fatalf("unexpected default output format: %s", invocation.Output.Format)
	}
}

func TestParseOutputOptions(t *testing.T) {
	tests := []struct {
		name   string
		args   []string
		format string
	}{
		{name: "json shorthand", args: []string{"shot", "--json", "http://localhost:3000"}, format: "json"},
		{name: "format json", args: []string{"shot", "--format", "json", "http://localhost:3000"}, format: "json"},
		{name: "format human", args: []string{"shot", "--format", "human", "http://localhost:3000"}, format: "human"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			invocation, err := parseCLI(tt.args)
			if err != nil {
				t.Fatalf("parseCLI returned error: %v", err)
			}
			if invocation.Output.Format != tt.format {
				t.Fatalf("unexpected format: got %s want %s", invocation.Output.Format, tt.format)
			}
		})
	}
}

func TestParseOutputOptionsNoColor(t *testing.T) {
	invocation, err := parseCLI([]string{"shot", "--no-color", "http://localhost:3000"})
	if err != nil {
		t.Fatalf("parseCLI returned error: %v", err)
	}
	if !invocation.Output.NoColor {
		t.Fatal("expected no-color output option")
	}
}

func TestParseOutputOptionsRejectsUnsupportedFormat(t *testing.T) {
	_, err := parseCLI([]string{"shot", "--format", "xml", "http://localhost:3000"})
	if err == nil {
		t.Fatal("expected unsupported format error")
	}
	var parseErr cliParseError
	if !errors.As(err, &parseErr) {
		t.Fatalf("expected cliParseError, got %T", err)
	}
	if parseErr.Output.Format != "xml" {
		t.Fatalf("expected parse error to preserve output context, got %#v", parseErr.Output)
	}
}

func TestParseJSONModePositionalErrorKeepsOutputContext(t *testing.T) {
	_, err := parseCLI([]string{"shot", "--json"})
	if err == nil {
		t.Fatal("expected missing positional argument")
	}
	var parseErr cliParseError
	if !errors.As(err, &parseErr) {
		t.Fatalf("expected cliParseError, got %T", err)
	}
	if parseErr.Output.Format != "json" {
		t.Fatalf("expected json output context, got %#v", parseErr.Output)
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

func TestParseSemanticDiffCLI(t *testing.T) {
	invocation, err := parseCLI([]string{
		"semantic-diff",
		"--provider", "openai",
		"--model", "gpt-test",
		"--mode", "focused",
		"--prompt", "Check checkout CTA.",
		"--focus", "checkout button,navigation labels",
		"--metadata", "semantic.json",
		"--timeout", "45s",
		"--max-output-tokens", "500",
		"--openai-base-url", "https://openai.test/v1/responses",
		"--anthropic-base-url", "https://anthropic.test/v1/messages",
		"--codex-bin", "/usr/local/bin/codex",
		"--codex-profile", "work",
		"--codex-oss",
		"--codex-local-provider", "ollama",
		"--codex-extra-arg", "--reasoning-effort",
		"--codex-extra-arg", "low",
		"--pixel-diff-image", "diff.png",
		"--changed-pixels", "10",
		"--total-pixels", "100",
		"--changed-percent", "10",
		"current.png",
		"reference.png",
	})
	if err != nil {
		t.Fatalf("parseCLI returned error: %v", err)
	}
	if invocation.Command != "semantic-diff" {
		t.Fatalf("unexpected command: %s", invocation.Command)
	}
	req := invocation.Semantic.Request
	if req.CurrentPath != "current.png" || req.ReferencePath != "reference.png" || req.Provider != "openai" || req.Model != "gpt-test" {
		t.Fatalf("unexpected semantic request: %#v", req)
	}
	if req.Mode != pkgwebcap.SemanticDiffModeFocused || len(req.Focus) != 2 || req.PixelContext.PixelDiffImagePath != "diff.png" {
		t.Fatalf("unexpected semantic options: %#v", req)
	}
	if req.MaxOutputTokens != 500 || req.Timeout != "45s" || req.MetadataPath != "semantic.json" {
		t.Fatalf("unexpected semantic limits/metadata: %#v", req)
	}
	if invocation.Provider.OpenAIBaseURL != "https://openai.test/v1/responses" || invocation.Provider.AnthropicBaseURL != "https://anthropic.test/v1/messages" {
		t.Fatalf("unexpected semantic provider options: %#v", invocation.Provider)
	}
	if invocation.Provider.CodexBin != "/usr/local/bin/codex" || invocation.Provider.CodexProfile != "work" || !invocation.Provider.CodexOSS || invocation.Provider.CodexLocalProvider != "ollama" {
		t.Fatalf("unexpected codex provider options: %#v", invocation.Provider)
	}
	if len(invocation.Provider.CodexExtraArgs) != 2 || invocation.Provider.CodexExtraArgs[0] != "--reasoning-effort" || invocation.Provider.CodexExtraArgs[1] != "low" {
		t.Fatalf("unexpected codex extra args: %#v", invocation.Provider.CodexExtraArgs)
	}
}

func TestParseHelpCLI(t *testing.T) {
	invocation, err := parseCLI([]string{"help"})
	if err != nil {
		t.Fatalf("parseCLI returned error: %v", err)
	}
	if invocation.Command != "help" {
		t.Fatalf("unexpected command: %s", invocation.Command)
	}
}

func TestParseVersionCLI(t *testing.T) {
	invocation, err := parseCLI([]string{"version"})
	if err != nil {
		t.Fatalf("parseCLI returned error: %v", err)
	}
	if invocation.Command != "version" {
		t.Fatalf("unexpected command: %s", invocation.Command)
	}
}

func TestParseVersionCLIRejectsArguments(t *testing.T) {
	if _, err := parseCLI([]string{"version", "extra"}); err == nil {
		t.Fatal("expected version arguments to be rejected")
	}
}

func TestParseHelpCLIRejectsArguments(t *testing.T) {
	if _, err := parseCLI([]string{"help", "shot"}); err == nil {
		t.Fatal("expected help arguments to be rejected")
	}
}

func TestParseHelpAndVersionRemainHumanOnly(t *testing.T) {
	if _, err := parseCLI([]string{"help", "--json"}); err == nil {
		t.Fatal("expected help --json to be rejected")
	}
	if _, err := parseCLI([]string{"version", "--format", "json"}); err == nil {
		t.Fatal("expected version --format json to be rejected")
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
		"--openai-base-url", "https://openai.test/v1/responses",
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
	if invocation.Provider.OpenAIBaseURL != "https://openai.test/v1/responses" {
		t.Fatalf("unexpected provider options: %#v", invocation.Provider)
	}
}

func TestParseMCPServeCLIRejectsJSONFlag(t *testing.T) {
	if _, err := parseCLI([]string{"mcp", "serve", "--json"}); err == nil {
		t.Fatal("expected mcp serve --json to be rejected")
	}
}

func TestParseWorkflowCaptureScenarioCLI(t *testing.T) {
	invocation, err := parseCLI([]string{
		"workflow",
		"capture-scenario",
		"--engine", "playwright",
		"--playwright-browser", "firefox",
		"--run-report",
		"--anthropic-base-url", "https://anthropic.test/v1/messages",
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
	if invocation.Provider.AnthropicBaseURL != "https://anthropic.test/v1/messages" {
		t.Fatalf("unexpected provider options: %#v", invocation.Provider)
	}
}

func TestParseWorkflowCaptureMVPCLI(t *testing.T) {
	if _, err := parseCLI([]string{"workflow", "capture-mvp"}); err == nil {
		t.Fatal("expected capture-mvp to be unsupported in standalone CLI")
	}
}

func TestParseReportScenarioCLI(t *testing.T) {
	invocation, err := parseCLI([]string{"report", "scenario", "--openai-base-url", "https://openai.test/v1/responses", "workflow.yaml"})
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
	if invocation.Provider.OpenAIBaseURL != "https://openai.test/v1/responses" {
		t.Fatalf("unexpected provider options: %#v", invocation.Provider)
	}
}

func TestParseReportMVPCLI(t *testing.T) {
	if _, err := parseCLI([]string{"report", "mvp"}); err == nil {
		t.Fatal("expected report mvp to be unsupported in standalone CLI")
	}
}

func TestParseSkillInstallCLI(t *testing.T) {
	tests := []struct {
		name  string
		agent string
		want  skills.Agent
	}{
		{name: "codex", agent: "codex", want: skills.AgentCodex},
		{name: "claude", agent: "claude", want: skills.AgentClaude},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			invocation, err := parseCLI([]string{"skill", "install", "--agent", tt.agent})
			if err != nil {
				t.Fatalf("parseCLI returned error: %v", err)
			}
			if invocation.Command != "skill" || invocation.Skill.Action != "install" {
				t.Fatalf("unexpected skill invocation: %#v", invocation)
			}
			if invocation.Skill.Agent != tt.want {
				t.Fatalf("unexpected agent: got %q want %q", invocation.Skill.Agent, tt.want)
			}
		})
	}
}

func TestParseSkillInstallCLIWithForce(t *testing.T) {
	invocation, err := parseCLI([]string{"skill", "install", "--agent", "codex", "--force"})
	if err != nil {
		t.Fatalf("parseCLI returned error: %v", err)
	}
	if !invocation.Skill.Force {
		t.Fatal("expected force option to be enabled")
	}
}

func TestParseSkillInstallCLIRejectsMissingAgent(t *testing.T) {
	if _, err := parseCLI([]string{"skill", "install"}); err == nil {
		t.Fatal("expected missing agent to be rejected")
	}
}

func TestParseSkillInstallCLIRejectsUnsupportedAgent(t *testing.T) {
	if _, err := parseCLI([]string{"skill", "install", "--agent", "cursor"}); err == nil {
		t.Fatal("expected unsupported agent to be rejected")
	}
}

func TestParseSkillInstallCLIRejectsPositionalArguments(t *testing.T) {
	if _, err := parseCLI([]string{"skill", "install", "--agent", "codex", "extra"}); err == nil {
		t.Fatal("expected skill install positional arguments to be rejected")
	}
}
