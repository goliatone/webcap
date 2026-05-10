package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"os"

	pkgwebcap "github.com/goliatone/webcap"
	commandwebcap "github.com/goliatone/webcap/commands/webcap"
	webcapmcp "github.com/goliatone/webcap/mcp"
	"github.com/goliatone/webcap/pkg/agents/skills"
	"github.com/goliatone/webcap/pkg/llms"
	"github.com/goliatone/webcap/pkg/version"
)

const webcapAgentSkillName = "webcap-agent"

var skillSourceFS func() fs.FS = webcapAgentSkillSource

func main() {
	invocation, err := parseCLI(os.Args[1:])
	if err != nil {
		log.Fatal(err)
	}

	if err := run(context.Background(), invocation); err != nil {
		log.Fatal(err)
	}
}

func run(ctx context.Context, invocation cliInvocation) error {
	handlers := map[string]func(context.Context, cliInvocation) error{
		"help":          runHelp,
		"version":       runVersion,
		"shot":          runShot,
		"multi":         runMulti,
		"diff":          runDiff,
		"semantic-diff": runSemanticDiff,
		"mcp":           runMCP,
		"workflow":      runWorkflow,
		"report":        runReport,
		"skill":         runSkill,
	}
	handler, ok := handlers[invocation.Command]
	if !ok {
		return fmt.Errorf("unsupported command %q", invocation.Command)
	}
	return handler(ctx, invocation)
}

func runSkill(ctx context.Context, invocation cliInvocation) error {
	if invocation.Skill.Action != "install" {
		return fmt.Errorf("unsupported skill action %q", invocation.Skill.Action)
	}
	handler := commandwebcap.NewSkillInstallHandler()
	result, err := handler.Handle(ctx, commandwebcap.SkillInstallMessage{
		Request: skills.InstallRequest{
			Agent:     invocation.Skill.Agent,
			SkillName: webcapAgentSkillName,
			Source:    skillSourceFS(),
			SourceDir: webcapAgentSkillName,
		},
	})
	if err != nil {
		return err
	}
	printJSON(result)
	return nil
}

func runSemanticDiff(ctx context.Context, invocation cliInvocation) error {
	service := pkgwebcap.NewServiceWithOptions(nil, semanticServiceOptions(invocation.Provider))
	handler := commandwebcap.NewSemanticDiffHandler(service)
	result, err := handler.Handle(ctx, commandwebcap.SemanticDiffMessage{
		Request: invocation.Semantic.Request,
	})
	if err != nil {
		return err
	}
	printJSON(result)
	return nil
}

func runHelp(_ context.Context, _ cliInvocation) error {
	_, err := fmt.Fprint(os.Stdout, helpText)
	return err
}

func runVersion(_ context.Context, _ cliInvocation) error {
	return version.Print(os.Stdout)
}

func runShot(ctx context.Context, invocation cliInvocation) error {
	service := newCaptureService(invocation.Browser, invocation.Provider)
	handler := commandwebcap.NewCaptureShotHandler(service)
	result, err := handler.Handle(ctx, commandwebcap.CaptureShotMessage{
		Request: invocation.Shot.Request,
	})
	if err != nil {
		return err
	}
	printJSON(result)
	return nil
}

func runMulti(ctx context.Context, invocation cliInvocation) error {
	manifest, err := pkgwebcap.LoadManifest(invocation.Multi.ManifestPath)
	if err != nil {
		return err
	}
	service := newCaptureService(invocation.Browser, invocation.Provider)
	handler := commandwebcap.NewCaptureBatchHandler(service)
	result, err := handler.Handle(ctx, commandwebcap.CaptureBatchMessage{
		Manifest:  manifest,
		OutputDir: invocation.Multi.OutputDir,
	})
	if err != nil {
		return err
	}
	printJSON(result)
	return nil
}

func runDiff(ctx context.Context, invocation cliInvocation) error {
	service := pkgwebcap.NewService(nil)
	handler := commandwebcap.NewDiffHandler(service)
	result, err := handler.Handle(ctx, commandwebcap.DiffMessage{
		Request: invocation.Diff.Request,
	})
	if err != nil {
		return err
	}
	printJSON(result)
	return nil
}

func runMCP(ctx context.Context, invocation cliInvocation) error {
	if invocation.MCP.Action != "serve" {
		return fmt.Errorf("unsupported mcp action %q", invocation.MCP.Action)
	}
	service := newCaptureService(invocation.Browser, invocation.Provider)
	server, err := webcapmcp.NewServer(webcapmcp.Config{
		Name:         "webcap",
		Version:      version.Tag,
		Capture:      service,
		Diff:         service,
		SemanticDiff: service,
		LoadManifest: pkgwebcap.LoadManifest,
	})
	if err != nil {
		return err
	}
	return server.Serve(ctx, os.Stdin, os.Stdout)
}

func runWorkflow(ctx context.Context, invocation cliInvocation) error {
	scenario, err := pkgwebcap.LoadWorkflowScenario(invocation.Workflow.ScenarioPath)
	if err != nil {
		return err
	}
	service := newScenarioCaptureService(invocation.Browser, invocation.Provider, scenario)
	handler := commandwebcap.NewCaptureScenarioHandler(service)
	captureResult, err := handler.Handle(ctx, commandwebcap.CaptureScenarioMessage{
		Scenario: scenario,
	})
	if err != nil {
		return err
	}
	if !invocation.Workflow.RunReport {
		printJSON(captureResult)
		return nil
	}
	return runWorkflowReport(ctx, service, scenario, captureResult)
}

func runWorkflowReport(ctx context.Context, service *pkgwebcap.Service, scenario pkgwebcap.WorkflowScenario, captureResult pkgwebcap.WorkflowCaptureResult) error {
	reportHandler := commandwebcap.NewWorkflowReportHandler(service)
	reportResult, err := reportHandler.Handle(ctx, commandwebcap.WorkflowReportMessage{
		Request: pkgwebcap.WorkflowReportRequest{Scenario: scenario},
	})
	if err != nil {
		return err
	}
	printJSON(struct {
		Capture pkgwebcap.WorkflowCaptureResult `json:"capture"`
		Report  pkgwebcap.WorkflowReportResult  `json:"report"`
	}{
		Capture: captureResult,
		Report:  reportResult,
	})
	return nil
}

func runReport(ctx context.Context, invocation cliInvocation) error {
	scenario, err := pkgwebcap.LoadWorkflowScenario(invocation.Report.ScenarioPath)
	if err != nil {
		return err
	}
	service := pkgwebcap.NewServiceWithOptions(nil, semanticServiceOptions(invocation.Provider))
	handler := commandwebcap.NewWorkflowReportHandler(service)
	result, err := handler.Handle(ctx, commandwebcap.WorkflowReportMessage{
		Request: pkgwebcap.WorkflowReportRequest{Scenario: scenario},
	})
	if err != nil {
		return err
	}
	printJSON(result)
	return nil
}

func newCaptureService(browser browserOptions, provider semanticProviderOptions) *pkgwebcap.Service {
	engine, err := pkgwebcap.NewEngine(browserEngineConfig(browser))
	if err != nil {
		log.Fatal(err)
	}
	return pkgwebcap.NewServiceWithOptions(engine, semanticServiceOptions(provider))
}

func newScenarioCaptureService(browser browserOptions, provider semanticProviderOptions, scenario pkgwebcap.WorkflowScenario) *pkgwebcap.Service {
	engine, err := pkgwebcap.NewEngine(mergeScenarioEngineConfig(browser, scenario))
	if err != nil {
		log.Fatal(err)
	}
	return pkgwebcap.NewServiceWithOptions(engine, semanticServiceOptions(provider))
}

func semanticServiceOptions(provider semanticProviderOptions) pkgwebcap.Options {
	return pkgwebcap.Options{SemanticDiff: pkgwebcap.SemanticDiffOptions{
		OpenAIBaseURL:    provider.OpenAIBaseURL,
		AnthropicBaseURL: provider.AnthropicBaseURL,
		LLMs: llms.Options{
			CodexCLI: llms.CodexCLIOptions{
				CommandPath:   provider.CodexBin,
				Profile:       provider.CodexProfile,
				UseOSS:        provider.CodexOSS,
				LocalProvider: provider.CodexLocalProvider,
				ExtraArgs:     append([]string(nil), provider.CodexExtraArgs...),
			},
		},
	}}
}

func browserEngineConfig(browser browserOptions) pkgwebcap.EngineConfig {
	return pkgwebcap.EngineConfig{
		EngineName:           pkgwebcap.NormalizeEngineName(browser.Engine),
		BrowserPath:          browser.BrowserPath,
		Headless:             browser.Headless,
		PlaywrightBrowser:    browser.PlaywrightBrowser,
		PlaywrightNodeBinary: browser.PlaywrightNodeBinary,
		PlaywrightRuntimeDir: browser.PlaywrightRuntimeDir,
	}
}

func mergeScenarioEngineConfig(browser browserOptions, scenario pkgwebcap.WorkflowScenario) pkgwebcap.EngineConfig {
	config := pkgwebcap.EngineConfig{
		EngineName: pkgwebcap.NormalizeEngineName(firstNonEmptyValue(
			func() string {
				if browser.EngineSet {
					return browser.Engine
				}
				return ""
			}(),
			scenario.Environment.Engine,
			string(pkgwebcap.EngineChromium),
		)),
		BrowserPath: firstNonEmptyValue(
			func() string {
				if browser.BrowserPathSet {
					return browser.BrowserPath
				}
				return ""
			}(),
			scenario.Environment.BrowserPath,
		),
		Headless:             true,
		PlaywrightBrowser:    firstNonEmptyValue(scenario.Environment.PlaywrightBrowser, browser.PlaywrightBrowser),
		PlaywrightNodeBinary: firstNonEmptyValue(scenario.Environment.PlaywrightNodeBinary, browser.PlaywrightNodeBinary),
		PlaywrightRuntimeDir: firstNonEmptyValue(scenario.Environment.PlaywrightRuntimeDir, browser.PlaywrightRuntimeDir),
	}
	if scenario.Environment.Headless != nil {
		config.Headless = *scenario.Environment.Headless
	}
	if browser.HeadlessSet {
		config.Headless = browser.Headless
	}
	if browser.PlaywrightBrowserSet {
		config.PlaywrightBrowser = browser.PlaywrightBrowser
	}
	if browser.NodeBinarySet {
		config.PlaywrightNodeBinary = browser.PlaywrightNodeBinary
	}
	if browser.RuntimeDirSet {
		config.PlaywrightRuntimeDir = browser.PlaywrightRuntimeDir
	}
	return config
}

func firstNonEmptyValue(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func printJSON(value any) {
	encoded, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		log.Fatal(err)
	}
	if _, err := os.Stdout.Write(append(encoded, '\n')); err != nil {
		log.Fatal(err)
	}
}

const helpText = `webcap captures browser screenshots, runs capture manifests, compares images, and serves MCP tools.

Usage:
  webcap help
  webcap version
  webcap shot [flags] <url>
  webcap multi [flags] <manifest-path>
  webcap diff [flags] <base-path> <compare-path>
  webcap semantic-diff [flags] <current-image> <reference-image>
  webcap workflow capture-scenario [flags] <scenario-path>
  webcap report scenario <scenario-path>
  webcap mcp serve [flags]
  webcap skill install --agent <codex|claude>

Commands:
  help                         Show this help message.
  version                      Show version and build information.
  shot                         Capture a single URL.
  multi                        Run a YAML or JSON capture manifest.
  diff                         Compare two image artifacts.
  semantic-diff                Compare two image artifacts with a vision LLM provider.
  workflow capture-scenario    Capture every screen in a workflow scenario.
  report scenario              Generate a workflow HTML review report.
  mcp serve                    Start the stdio MCP server.
  skill install                Install the bundled webcap-agent skill.

Common browser flags:
  --engine                     Capture engine: chromium or playwright.
  --browser-binary             Optional browser executable path.
  --headless                   Run Chromium in headless mode.
  --playwright-browser         Playwright browser: chromium, firefox, or webkit.
  --node-binary                Node.js binary used by the Playwright engine.
  --playwright-runtime-dir     Optional override for the Playwright runtime directory.

Semantic provider flags:
  --openai-base-url            Override the OpenAI semantic provider endpoint.
  --anthropic-base-url         Override the Anthropic semantic provider endpoint.
  --codex-bin                  Codex CLI binary path.
  --codex-profile              Codex CLI profile name.
  --codex-oss                  Run Codex CLI with OSS mode.
  --codex-local-provider       Codex CLI local provider name.
  --codex-extra-arg            Additional argument passed to codex exec; repeat for multiple values.
`
