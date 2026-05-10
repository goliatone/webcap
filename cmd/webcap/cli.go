package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"strings"

	pkgwebcap "github.com/goliatone/webcap"
	"github.com/goliatone/webcap/pkg/agents/skills"
)

type cliInvocation struct {
	Command  string
	Shot     shotOptions
	Multi    multiOptions
	Diff     diffOptions
	Semantic semanticDiffOptions
	Workflow workflowOptions
	Report   reportOptions
	MCP      mcpOptions
	Skill    skillOptions
	Browser  browserOptions
	Provider semanticProviderOptions
}

type browserOptions struct {
	Engine               string
	BrowserPath          string
	Headless             bool
	PlaywrightBrowser    string
	PlaywrightNodeBinary string
	PlaywrightRuntimeDir string
	EngineSet            bool
	BrowserPathSet       bool
	HeadlessSet          bool
	PlaywrightBrowserSet bool
	NodeBinarySet        bool
	RuntimeDirSet        bool
}

type shotOptions struct {
	Request pkgwebcap.CaptureRequest
}

type multiOptions struct {
	ManifestPath string
	OutputDir    string
}

type diffOptions struct {
	Request pkgwebcap.DiffRequest
}

type semanticDiffOptions struct {
	Request  pkgwebcap.SemanticDiffRequest
	FocusCSV string
}

type semanticProviderOptions struct {
	OpenAIBaseURL      string
	AnthropicBaseURL   string
	CodexBin           string
	CodexProfile       string
	CodexOSS           bool
	CodexLocalProvider string
	CodexExtraArgs     []string
}

type workflowOptions struct {
	Action       string
	ScenarioPath string
	RunReport    bool
}

type reportOptions struct {
	Action       string
	ScenarioPath string
}

type mcpOptions struct {
	Action string
}

type skillOptions struct {
	Action string
	Agent  skills.Agent
	Force  bool
}

func parseCLI(args []string) (cliInvocation, error) {
	if len(args) == 0 {
		return cliInvocation{}, errors.New("expected subcommand: help, version, shot, multi, diff, semantic-diff, workflow, report, mcp, or skill")
	}

	switch strings.TrimSpace(args[0]) {
	case "help":
		return parseHelpCLI(args[1:])
	case "version":
		return parseVersionCLI(args[1:])
	case "shot":
		return parseShotCLI(args[1:])
	case "multi":
		return parseMultiCLI(args[1:])
	case "diff":
		return parseDiffCLI(args[1:])
	case "semantic-diff":
		return parseSemanticDiffCLI(args[1:])
	case "workflow":
		return parseWorkflowCLI(args[1:])
	case "report":
		return parseReportCLI(args[1:])
	case "mcp":
		return parseMCPCLI(args[1:])
	case "skill":
		return parseSkillCLI(args[1:])
	default:
		return cliInvocation{}, fmt.Errorf("unsupported subcommand %q", args[0])
	}
}

func parseHelpCLI(args []string) (cliInvocation, error) {
	if len(args) != 0 {
		return cliInvocation{}, errors.New("help does not accept positional arguments")
	}
	return cliInvocation{Command: "help"}, nil
}

func parseVersionCLI(args []string) (cliInvocation, error) {
	if len(args) != 0 {
		return cliInvocation{}, errors.New("version does not accept positional arguments")
	}
	return cliInvocation{Command: "version"}, nil
}

func parseShotCLI(args []string) (cliInvocation, error) {
	var (
		stderr       bytes.Buffer
		outputPath   string
		metadataPath string
		selectorsCSV string
		selectorsAll string
		viewport     string
	)

	invocation := cliInvocation{Command: "shot"}
	fs := flag.NewFlagSet("webcap shot", flag.ContinueOnError)
	fs.SetOutput(&stderr)
	registerBrowserFlags(fs, &invocation.Browser)
	fs.StringVar(&outputPath, "output", "", "Output image path.")
	fs.StringVar(&metadataPath, "metadata", "", "Optional metadata sidecar path.")
	fs.BoolVar(&invocation.Shot.Request.FullPage, "full-page", false, "Capture the full page.")
	fs.StringVar(&invocation.Shot.Request.Selector, "selector", "", "Capture the first match for a CSS selector.")
	fs.StringVar(&selectorsCSV, "selectors", "", "Capture the union of the first match for each comma-separated CSS selector.")
	fs.StringVar(&invocation.Shot.Request.SelectorAll, "selector-all", "", "Capture the union of all matches for a CSS selector.")
	fs.StringVar(&selectorsAll, "selectors-all", "", "Capture the union of all matches for each comma-separated CSS selector.")
	fs.IntVar(&invocation.Shot.Request.Padding, "padding", 0, "Add padding around selector captures.")
	fs.StringVar(&invocation.Shot.Request.Wait, "wait", "", "Extra wait duration such as 250ms or 2s.")
	fs.StringVar(&invocation.Shot.Request.WaitFor, "wait-for", "", "Wait for a selector to become visible before capture.")
	fs.StringVar(&invocation.Shot.Request.JavaScript, "javascript", "", "Run JavaScript before capture.")
	fs.StringVar(&invocation.Shot.Request.Timeout, "timeout", "", "Overall capture timeout such as 30s.")
	fs.StringVar(&viewport, "viewport", "", "Viewport in WIDTHxHEIGHT form, for example 1440x1200.")
	fs.StringVar(&invocation.Shot.Request.ViewportPreset, "viewport-preset", "", "Use a named viewport preset such as desktop-xl or mobile.")
	fs.StringVar(&invocation.Shot.Request.DevicePreset, "device-preset", "", "Use a named device preset such as iphone-15.")
	fs.StringVar((*string)(&invocation.Shot.Request.Readiness), "readiness", string(pkgwebcap.ReadinessComplete), "Readiness mode: none, interactive, complete, or network_idle.")
	fs.StringVar(&invocation.Shot.Request.ReadinessIdle, "readiness-idle", "", "Idle duration used by network_idle readiness, such as 500ms.")
	fs.BoolVar(&invocation.Shot.Request.DisableAnimations, "disable-animations", false, "Disable CSS animations and transitions before capture.")
	fs.BoolVar(&invocation.Shot.Request.ReducedMotion, "reduced-motion", false, "Emulate prefers-reduced-motion: reduce.")
	fs.BoolVar(&invocation.Shot.Request.WaitForFonts, "wait-for-fonts", false, "Wait for document fonts to finish loading before capture.")

	if err := fs.Parse(args); err != nil {
		message := strings.TrimSpace(stderr.String())
		if message == "" {
			message = err.Error()
		}
		return cliInvocation{}, errors.New(message)
	}
	recordVisitedBrowserFlags(fs, &invocation.Browser)
	if len(fs.Args()) != 1 {
		return cliInvocation{}, errors.New("shot requires exactly one positional url argument")
	}

	invocation.Shot.Request.URL = strings.TrimSpace(fs.Args()[0])
	invocation.Shot.Request.OutputPath = strings.TrimSpace(outputPath)
	invocation.Shot.Request.MetadataPath = strings.TrimSpace(metadataPath)
	invocation.Shot.Request.Selectors = splitCSV(selectorsCSV)
	invocation.Shot.Request.SelectorsAll = splitCSV(selectorsAll)

	parsedViewport, err := parseViewport(viewport)
	if err != nil {
		return cliInvocation{}, err
	}
	invocation.Shot.Request.Viewport = parsedViewport

	return invocation, nil
}

func parseMultiCLI(args []string) (cliInvocation, error) {
	var stderr bytes.Buffer

	invocation := cliInvocation{Command: "multi"}
	fs := flag.NewFlagSet("webcap multi", flag.ContinueOnError)
	fs.SetOutput(&stderr)
	registerBrowserFlags(fs, &invocation.Browser)
	fs.StringVar(&invocation.Multi.OutputDir, "output-dir", "", "Override manifest output_dir.")
	if err := fs.Parse(args); err != nil {
		message := strings.TrimSpace(stderr.String())
		if message == "" {
			message = err.Error()
		}
		return cliInvocation{}, errors.New(message)
	}
	recordVisitedBrowserFlags(fs, &invocation.Browser)
	if len(fs.Args()) != 1 {
		return cliInvocation{}, errors.New("multi requires exactly one manifest path")
	}
	invocation.Multi.ManifestPath = strings.TrimSpace(fs.Args()[0])
	return invocation, nil
}

func parseDiffCLI(args []string) (cliInvocation, error) {
	var (
		stderr       bytes.Buffer
		outputPath   string
		metadataPath string
	)

	invocation := cliInvocation{Command: "diff"}
	fs := flag.NewFlagSet("webcap diff", flag.ContinueOnError)
	fs.SetOutput(&stderr)
	fs.StringVar(&outputPath, "output", "", "Output diff image path or diff directory.")
	fs.StringVar(&metadataPath, "metadata", "", "Optional metadata sidecar path.")
	fs.Float64Var(&invocation.Diff.Request.Threshold, "threshold", 0, "Per-channel normalized threshold from 0 to 1.")
	if err := fs.Parse(args); err != nil {
		message := strings.TrimSpace(stderr.String())
		if message == "" {
			message = err.Error()
		}
		return cliInvocation{}, errors.New(message)
	}
	if len(fs.Args()) != 2 {
		return cliInvocation{}, errors.New("diff requires base and compare paths")
	}
	invocation.Diff.Request.BasePath = strings.TrimSpace(fs.Args()[0])
	invocation.Diff.Request.ComparePath = strings.TrimSpace(fs.Args()[1])
	invocation.Diff.Request.OutputPath = strings.TrimSpace(outputPath)
	invocation.Diff.Request.MetadataPath = strings.TrimSpace(metadataPath)
	return invocation, nil
}

func parseSemanticDiffCLI(args []string) (cliInvocation, error) {
	var (
		stderr        bytes.Buffer
		mode          string
		pixelDiffPath string
		threshold     float64
	)

	invocation := cliInvocation{Command: "semantic-diff"}
	fs := flag.NewFlagSet("webcap semantic-diff", flag.ContinueOnError)
	fs.SetOutput(&stderr)
	registerSemanticProviderFlags(fs, &invocation.Provider)
	fs.StringVar(&invocation.Semantic.Request.Provider, "provider", "", "Semantic diff provider: openai, anthropic, or codex-cli.")
	fs.StringVar(&invocation.Semantic.Request.Model, "model", "", "Provider model name.")
	fs.StringVar(&mode, "mode", string(pkgwebcap.SemanticDiffModeGeneral), "Semantic diff mode: general, focused, copy, layout, accessibility, or custom.")
	fs.StringVar(&invocation.Semantic.Request.Prompt, "prompt", "", "Additional prompt instructions.")
	fs.StringVar(&invocation.Semantic.Request.PromptPath, "prompt-file", "", "Path to a prompt instruction file.")
	fs.StringVar(&invocation.Semantic.FocusCSV, "focus", "", "Comma-separated focus areas.")
	fs.StringVar(&invocation.Semantic.Request.MetadataPath, "metadata", "", "Semantic diff metadata JSON path.")
	fs.StringVar(&invocation.Semantic.Request.RawResponsePath, "raw-response", "", "Optional raw provider response path.")
	fs.BoolVar(&invocation.Semantic.Request.PersistRawResponse, "persist-raw-response", false, "Persist the raw provider response for debugging.")
	fs.StringVar(&invocation.Semantic.Request.Timeout, "timeout", "", "Provider request timeout such as 60s.")
	fs.IntVar(&invocation.Semantic.Request.MaxOutputTokens, "max-output-tokens", 0, "Provider max output token limit.")
	fs.StringVar(&pixelDiffPath, "pixel-diff-image", "", "Existing pixel diff image to include as optional context.")
	fs.IntVar(&invocation.Semantic.Request.PixelContext.ChangedPixels, "changed-pixels", 0, "Existing pixel diff changed pixel count.")
	fs.IntVar(&invocation.Semantic.Request.PixelContext.TotalPixels, "total-pixels", 0, "Existing pixel diff total pixel count.")
	fs.Float64Var(&invocation.Semantic.Request.PixelContext.ChangedPercent, "changed-percent", 0, "Existing pixel diff changed percent.")
	fs.Float64Var(&threshold, "threshold", 0, "Existing pixel diff threshold.")
	if err := fs.Parse(args); err != nil {
		message := strings.TrimSpace(stderr.String())
		if message == "" {
			message = err.Error()
		}
		return cliInvocation{}, errors.New(message)
	}
	if len(fs.Args()) != 2 {
		return cliInvocation{}, errors.New("semantic-diff requires current and reference image paths")
	}
	invocation.Semantic.Request.CurrentPath = strings.TrimSpace(fs.Args()[0])
	invocation.Semantic.Request.ReferencePath = strings.TrimSpace(fs.Args()[1])
	invocation.Semantic.Request.Mode = pkgwebcap.SemanticDiffMode(mode)
	invocation.Semantic.Request.Focus = splitCSV(invocation.Semantic.FocusCSV)
	invocation.Semantic.Request.PixelContext.PixelDiffImagePath = strings.TrimSpace(pixelDiffPath)
	invocation.Semantic.Request.PixelContext.Threshold = threshold
	return invocation, nil
}

func parseMCPCLI(args []string) (cliInvocation, error) {
	if len(args) == 0 {
		return cliInvocation{}, errors.New("mcp requires a nested subcommand such as serve")
	}
	switch strings.TrimSpace(args[0]) {
	case "serve":
		return parseMCPServeCLI(args[1:])
	default:
		return cliInvocation{}, fmt.Errorf("unsupported mcp subcommand %q", args[0])
	}
}

func parseMCPServeCLI(args []string) (cliInvocation, error) {
	var stderr bytes.Buffer

	invocation := cliInvocation{
		Command: "mcp",
		MCP: mcpOptions{
			Action: "serve",
		},
	}
	fs := flag.NewFlagSet("webcap mcp serve", flag.ContinueOnError)
	fs.SetOutput(&stderr)
	registerBrowserFlags(fs, &invocation.Browser)
	registerSemanticProviderFlags(fs, &invocation.Provider)
	if err := fs.Parse(args); err != nil {
		message := strings.TrimSpace(stderr.String())
		if message == "" {
			message = err.Error()
		}
		return cliInvocation{}, errors.New(message)
	}
	recordVisitedBrowserFlags(fs, &invocation.Browser)
	if len(fs.Args()) != 0 {
		return cliInvocation{}, errors.New("mcp serve does not accept positional arguments")
	}
	return invocation, nil
}

func parseSkillCLI(args []string) (cliInvocation, error) {
	if len(args) == 0 {
		return cliInvocation{}, errors.New("skill requires a nested subcommand such as install")
	}
	switch strings.TrimSpace(args[0]) {
	case "install":
		return parseSkillInstallCLI(args[1:])
	default:
		return cliInvocation{}, fmt.Errorf("unsupported skill subcommand %q", args[0])
	}
}

func parseSkillInstallCLI(args []string) (cliInvocation, error) {
	var (
		stderr bytes.Buffer
		agent  string
	)

	invocation := cliInvocation{
		Command: "skill",
		Skill: skillOptions{
			Action: "install",
		},
	}
	fs := flag.NewFlagSet("webcap skill install", flag.ContinueOnError)
	fs.SetOutput(&stderr)
	fs.StringVar(&agent, "agent", "", "Agent to install for: codex or claude.")
	fs.BoolVar(&invocation.Skill.Force, "force", false, "Replace conflicting installed skill files.")
	if err := fs.Parse(args); err != nil {
		message := strings.TrimSpace(stderr.String())
		if message == "" {
			message = err.Error()
		}
		return cliInvocation{}, errors.New(message)
	}
	if len(fs.Args()) != 0 {
		return cliInvocation{}, errors.New("skill install does not accept positional arguments")
	}
	if strings.TrimSpace(agent) == "" {
		return cliInvocation{}, errors.New("skill install requires --agent codex or --agent claude")
	}
	parsedAgent, err := skills.ParseAgent(agent)
	if err != nil {
		return cliInvocation{}, err
	}
	invocation.Skill.Agent = parsedAgent
	return invocation, nil
}

func parseWorkflowCLI(args []string) (cliInvocation, error) {
	if len(args) == 0 {
		return cliInvocation{}, errors.New("workflow requires a nested subcommand such as capture-scenario or capture-mvp")
	}
	switch strings.TrimSpace(args[0]) {
	case "capture-scenario":
		return parseWorkflowCaptureCLI(args[1:])
	default:
		return cliInvocation{}, fmt.Errorf("unsupported workflow subcommand %q", args[0])
	}
}

func parseWorkflowCaptureCLI(args []string) (cliInvocation, error) {
	var stderr bytes.Buffer

	invocation := cliInvocation{
		Command: "workflow",
		Workflow: workflowOptions{
			Action: "capture",
		},
	}
	fs := flag.NewFlagSet("webcap workflow capture", flag.ContinueOnError)
	fs.SetOutput(&stderr)
	registerBrowserFlags(fs, &invocation.Browser)
	registerSemanticProviderFlags(fs, &invocation.Provider)
	fs.BoolVar(&invocation.Workflow.RunReport, "run-report", false, "Generate the workflow report after captures finish.")
	if err := fs.Parse(args); err != nil {
		message := strings.TrimSpace(stderr.String())
		if message == "" {
			message = err.Error()
		}
		return cliInvocation{}, errors.New(message)
	}
	recordVisitedBrowserFlags(fs, &invocation.Browser)
	if len(fs.Args()) != 1 {
		return cliInvocation{}, errors.New("workflow capture-scenario requires exactly one scenario path")
	}
	invocation.Workflow.ScenarioPath = strings.TrimSpace(fs.Args()[0])
	return invocation, nil
}

func parseReportCLI(args []string) (cliInvocation, error) {
	if len(args) == 0 {
		return cliInvocation{}, errors.New("report requires a nested subcommand such as scenario or mvp")
	}
	switch strings.TrimSpace(args[0]) {
	case "scenario":
		return parseReportScenarioCLI(args[1:])
	default:
		return cliInvocation{}, fmt.Errorf("unsupported report subcommand %q", args[0])
	}
}

func parseReportScenarioCLI(args []string) (cliInvocation, error) {
	var stderr bytes.Buffer

	invocation := cliInvocation{
		Command: "report",
		Report: reportOptions{
			Action: "generate",
		},
	}
	fs := flag.NewFlagSet("webcap report", flag.ContinueOnError)
	fs.SetOutput(&stderr)
	registerSemanticProviderFlags(fs, &invocation.Provider)
	if err := fs.Parse(args); err != nil {
		message := strings.TrimSpace(stderr.String())
		if message == "" {
			message = err.Error()
		}
		return cliInvocation{}, errors.New(message)
	}
	if len(fs.Args()) != 1 {
		return cliInvocation{}, errors.New("report scenario requires exactly one scenario path")
	}
	invocation.Report.ScenarioPath = strings.TrimSpace(fs.Args()[0])
	return invocation, nil
}

func registerBrowserFlags(fs *flag.FlagSet, browser *browserOptions) {
	fs.StringVar(&browser.Engine, "engine", string(pkgwebcap.EngineChromium), "Capture engine: chromium or playwright.")
	fs.BoolVar(&browser.Headless, "headless", true, "Run Chromium in headless mode.")
	fs.StringVar(&browser.BrowserPath, "browser-binary", "", "Optional browser executable path.")
	fs.StringVar(&browser.PlaywrightBrowser, "playwright-browser", "chromium", "Playwright browser: chromium, firefox, or webkit.")
	fs.StringVar(&browser.PlaywrightNodeBinary, "node-binary", "node", "Node.js binary used by the Playwright engine.")
	fs.StringVar(&browser.PlaywrightRuntimeDir, "playwright-runtime-dir", "", "Optional override for the Playwright runtime directory.")
}

func registerSemanticProviderFlags(fs *flag.FlagSet, provider *semanticProviderOptions) {
	fs.StringVar(&provider.OpenAIBaseURL, "openai-base-url", "", "Override the OpenAI semantic provider endpoint.")
	fs.StringVar(&provider.AnthropicBaseURL, "anthropic-base-url", "", "Override the Anthropic semantic provider endpoint.")
	fs.StringVar(&provider.CodexBin, "codex-bin", "", "Codex CLI binary path.")
	fs.StringVar(&provider.CodexProfile, "codex-profile", "", "Codex CLI profile name.")
	fs.BoolVar(&provider.CodexOSS, "codex-oss", false, "Run Codex CLI with OSS mode.")
	fs.StringVar(&provider.CodexLocalProvider, "codex-local-provider", "", "Codex CLI local provider name.")
	fs.Var((*stringSliceFlag)(&provider.CodexExtraArgs), "codex-extra-arg", "Additional argument passed to codex exec; repeat for multiple values.")
}

type stringSliceFlag []string

func (f *stringSliceFlag) String() string {
	if f == nil {
		return ""
	}
	return strings.Join(*f, ",")
}

func (f *stringSliceFlag) Set(value string) error {
	value = strings.TrimSpace(value)
	if value != "" {
		*f = append(*f, value)
	}
	return nil
}

func recordVisitedBrowserFlags(fs *flag.FlagSet, browser *browserOptions) {
	fs.Visit(func(f *flag.Flag) {
		switch f.Name {
		case "engine":
			browser.EngineSet = true
		case "browser-binary":
			browser.BrowserPathSet = true
		case "headless":
			browser.HeadlessSet = true
		case "playwright-browser":
			browser.PlaywrightBrowserSet = true
		case "node-binary":
			browser.NodeBinarySet = true
		case "playwright-runtime-dir":
			browser.RuntimeDirSet = true
		}
	})
}

func parseViewport(raw string) (pkgwebcap.Viewport, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return pkgwebcap.Viewport{}, nil
	}
	var width, height int
	if _, err := fmt.Sscanf(raw, "%dx%d", &width, &height); err != nil {
		return pkgwebcap.Viewport{}, fmt.Errorf("invalid viewport %q, expected WIDTHxHEIGHT", raw)
	}
	return pkgwebcap.Viewport{Width: width, Height: height}, nil
}

func splitCSV(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		out = append(out, part)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
