package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"

	pkgwebcap "github.com/goliatone/webcap"
	commandwebcap "github.com/goliatone/webcap/commands/webcap"
	"github.com/goliatone/webcap/internal/presentation"
	webcapmcp "github.com/goliatone/webcap/mcp"
	"github.com/goliatone/webcap/pkg/agents/skills"
	"github.com/goliatone/webcap/pkg/llms"
	"github.com/goliatone/webcap/pkg/version"
)

const webcapAgentSkillName = "webcap-agent"

var skillSourceFS func() fs.FS = webcapAgentSkillSource

func main() {
	os.Exit(runCLI(context.Background(), os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

func runCLI(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	invocation, err := parseCLI(args)
	if err != nil {
		opts := defaultOutputOptions()
		var parseErr cliParseError
		if errors.As(err, &parseErr) {
			opts = parseErr.Output
			_ = normalizeOutputOptions(&opts)
		}
		_ = presentError(stderr, opts, err)
		return 1
	}
	if err := newApp(stdin, stdout, stderr).run(ctx, invocation); err != nil {
		_ = presentError(stderr, invocation.Output, err)
		return 1
	}
	return 0
}

type app struct {
	stdin              io.Reader
	stdout             io.Writer
	stderr             io.Writer
	newCaptureService  func(browserOptions, semanticProviderOptions) (cliService, error)
	newScenarioService func(browserOptions, semanticProviderOptions, pkgwebcap.WorkflowScenario) (cliService, error)
	newService         func(semanticProviderOptions) cliService
	loadManifest       func(string) (pkgwebcap.Manifest, error)
	loadScenario       func(string) (pkgwebcap.WorkflowScenario, error)
}

type cliService interface {
	pkgwebcap.CaptureService
	pkgwebcap.DiffService
	pkgwebcap.SemanticDiffService
	pkgwebcap.WorkflowService
}

func newApp(stdin io.Reader, stdout, stderr io.Writer) app {
	return app{
		stdin:  stdin,
		stdout: stdout,
		stderr: stderr,
		newCaptureService: func(browser browserOptions, provider semanticProviderOptions) (cliService, error) {
			return newCaptureService(browser, provider)
		},
		newScenarioService: func(browser browserOptions, provider semanticProviderOptions, scenario pkgwebcap.WorkflowScenario) (cliService, error) {
			return newScenarioCaptureService(browser, provider, scenario)
		},
		newService: func(provider semanticProviderOptions) cliService {
			return pkgwebcap.NewServiceWithOptions(nil, semanticServiceOptions(provider))
		},
		loadManifest: pkgwebcap.LoadManifest,
		loadScenario: pkgwebcap.LoadWorkflowScenario,
	}
}

func run(ctx context.Context, invocation cliInvocation) error {
	return newApp(os.Stdin, os.Stdout, os.Stderr).run(ctx, invocation)
}

func (a app) run(ctx context.Context, invocation cliInvocation) error {
	switch invocation.Command {
	case "help":
		return runHelp(ctx, invocation, a.stdout)
	case "version":
		return runVersion(ctx, invocation, a.stdout)
	case "mcp":
		return a.runMCP(ctx, invocation, a.stdin, a.stdout)
	}
	handlers := map[string]func(context.Context, cliInvocation) (any, error){
		"shot":          a.runShot,
		"multi":         a.runMulti,
		"diff":          a.runDiff,
		"semantic-diff": a.runSemanticDiff,
		"workflow":      a.runWorkflow,
		"report":        a.runReport,
		"skill":         runSkill,
	}
	handler, ok := handlers[invocation.Command]
	if !ok {
		return fmt.Errorf("unsupported command %q", invocation.Command)
	}
	result, err := handler(ctx, invocation)
	if err != nil {
		return err
	}
	return presenter(invocation.Output).Present(a.stdout, result)
}

func runSkill(ctx context.Context, invocation cliInvocation) (any, error) {
	if invocation.Skill.Action != "install" {
		return nil, fmt.Errorf("unsupported skill action %q", invocation.Skill.Action)
	}
	handler := commandwebcap.NewSkillInstallHandler()
	result, err := handler.Handle(ctx, commandwebcap.SkillInstallMessage{
		Request: skills.InstallRequest{
			Agent:     invocation.Skill.Agent,
			SkillName: webcapAgentSkillName,
			Source:    skillSourceFS(),
			SourceDir: webcapAgentSkillName,
			Force:     invocation.Skill.Force,
		},
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (a app) runSemanticDiff(ctx context.Context, invocation cliInvocation) (any, error) {
	service := a.newService(invocation.Provider)
	handler := commandwebcap.NewSemanticDiffHandler(service)
	result, err := handler.Handle(ctx, commandwebcap.SemanticDiffMessage{
		Request: invocation.Semantic.Request,
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func runHelp(_ context.Context, _ cliInvocation, stdout io.Writer) error {
	_, err := fmt.Fprint(stdout, helpText)
	return err
}

func runVersion(_ context.Context, _ cliInvocation, stdout io.Writer) error {
	return version.Print(stdout)
}

func (a app) runShot(ctx context.Context, invocation cliInvocation) (any, error) {
	service, err := a.newCaptureService(invocation.Browser, invocation.Provider)
	if err != nil {
		return nil, err
	}
	handler := commandwebcap.NewCaptureShotHandler(service)
	result, err := handler.Handle(ctx, commandwebcap.CaptureShotMessage{
		Request: invocation.Shot.Request,
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (a app) runMulti(ctx context.Context, invocation cliInvocation) (any, error) {
	manifest, err := a.loadManifest(invocation.Multi.ManifestPath)
	if err != nil {
		return nil, err
	}
	service, err := a.newCaptureService(invocation.Browser, invocation.Provider)
	if err != nil {
		return nil, err
	}
	handler := commandwebcap.NewCaptureBatchHandler(service)
	result, err := handler.Handle(ctx, commandwebcap.CaptureBatchMessage{
		Manifest:  manifest,
		OutputDir: invocation.Multi.OutputDir,
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (a app) runDiff(ctx context.Context, invocation cliInvocation) (any, error) {
	service := a.newService(invocation.Provider)
	handler := commandwebcap.NewDiffHandler(service)
	result, err := handler.Handle(ctx, commandwebcap.DiffMessage{
		Request: invocation.Diff.Request,
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (a app) runMCP(ctx context.Context, invocation cliInvocation, stdin io.Reader, stdout io.Writer) error {
	if invocation.MCP.Action != "serve" {
		return fmt.Errorf("unsupported mcp action %q", invocation.MCP.Action)
	}
	service, err := a.newCaptureService(invocation.Browser, invocation.Provider)
	if err != nil {
		return err
	}
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
	return server.Serve(ctx, stdin, stdout)
}

func (a app) runWorkflow(ctx context.Context, invocation cliInvocation) (any, error) {
	scenario, err := a.loadScenario(invocation.Workflow.ScenarioPath)
	if err != nil {
		return nil, err
	}
	service, err := a.newScenarioService(invocation.Browser, invocation.Provider, scenario)
	if err != nil {
		return nil, err
	}
	handler := commandwebcap.NewCaptureScenarioHandler(service)
	captureResult, err := handler.Handle(ctx, commandwebcap.CaptureScenarioMessage{
		Scenario: scenario,
	})
	if err != nil {
		return nil, err
	}
	if !invocation.Workflow.RunReport {
		return captureResult, nil
	}
	return runWorkflowReport(ctx, service, scenario, captureResult)
}

func runWorkflowReport(ctx context.Context, service pkgwebcap.WorkflowService, scenario pkgwebcap.WorkflowScenario, captureResult pkgwebcap.WorkflowCaptureResult) (any, error) {
	reportHandler := commandwebcap.NewWorkflowReportHandler(service)
	reportResult, err := reportHandler.Handle(ctx, commandwebcap.WorkflowReportMessage{
		Request: pkgwebcap.WorkflowReportRequest{Scenario: scenario},
	})
	if err != nil {
		return nil, err
	}
	return presentation.WorkflowRunReportResult{
		Capture: captureResult,
		Report:  reportResult,
	}, nil
}

func (a app) runReport(ctx context.Context, invocation cliInvocation) (any, error) {
	scenario, err := a.loadScenario(invocation.Report.ScenarioPath)
	if err != nil {
		return nil, err
	}
	service := a.newService(invocation.Provider)
	handler := commandwebcap.NewWorkflowReportHandler(service)
	result, err := handler.Handle(ctx, commandwebcap.WorkflowReportMessage{
		Request: pkgwebcap.WorkflowReportRequest{Scenario: scenario},
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func newCaptureService(browser browserOptions, provider semanticProviderOptions) (*pkgwebcap.Service, error) {
	engine, err := pkgwebcap.NewEngine(browserEngineConfig(browser))
	if err != nil {
		return nil, err
	}
	return pkgwebcap.NewServiceWithOptions(engine, semanticServiceOptions(provider)), nil
}

func newScenarioCaptureService(browser browserOptions, provider semanticProviderOptions, scenario pkgwebcap.WorkflowScenario) (*pkgwebcap.Service, error) {
	engine, err := pkgwebcap.NewEngine(mergeScenarioEngineConfig(browser, scenario))
	if err != nil {
		return nil, err
	}
	return pkgwebcap.NewServiceWithOptions(engine, semanticServiceOptions(provider)), nil
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

func presenter(opts outputOptions) presentation.Presenter {
	format := presentation.Format(opts.Format)
	if format == "" {
		format = presentation.FormatHuman
	}
	return presentation.New(presentation.Options{
		Format: format,
		Color:  !opts.NoColor,
	})
}

func presentError(stderr io.Writer, opts outputOptions, err error) error {
	return presenter(opts).PresentError(stderr, err)
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
  webcap report scenario [flags] <scenario-path>
  webcap mcp serve [flags]
  webcap skill install [flags] --agent <codex|claude>

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

Output flags for result commands:
  --json                       Render machine-readable JSON output.
  --format                     Output format: human or json.
  --no-color                   Disable terminal color styling.

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
