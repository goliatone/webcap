package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"

	pkgwebcap "github.com/goliatone/webcap"
	commandwebcap "github.com/goliatone/webcap/commands/webcap"
	webcapmcp "github.com/goliatone/webcap/mcp"
)

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
		"shot":     runShot,
		"multi":    runMulti,
		"diff":     runDiff,
		"mcp":      runMCP,
		"workflow": runWorkflow,
		"report":   runReport,
	}
	handler, ok := handlers[invocation.Command]
	if !ok {
		return fmt.Errorf("unsupported command %q", invocation.Command)
	}
	return handler(ctx, invocation)
}

func runShot(ctx context.Context, invocation cliInvocation) error {
	service := newCaptureService(invocation.Browser)
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
	service := newCaptureService(invocation.Browser)
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
	service := newCaptureService(invocation.Browser)
	server, err := webcapmcp.NewServer(webcapmcp.Config{
		Name:         "webcap",
		Version:      "0.1.0",
		Capture:      service,
		Diff:         service,
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
	service := newScenarioCaptureService(invocation.Browser, scenario)
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
	service := pkgwebcap.NewService(nil)
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

func newCaptureService(browser browserOptions) *pkgwebcap.Service {
	engine, err := pkgwebcap.NewEngine(browserEngineConfig(browser))
	if err != nil {
		log.Fatal(err)
	}
	return pkgwebcap.NewService(engine)
}

func newScenarioCaptureService(browser browserOptions, scenario pkgwebcap.WorkflowScenario) *pkgwebcap.Service {
	engine, err := pkgwebcap.NewEngine(mergeScenarioEngineConfig(browser, scenario))
	if err != nil {
		log.Fatal(err)
	}
	return pkgwebcap.NewService(engine)
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
