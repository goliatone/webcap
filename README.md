# webcap

`webcap` is a Go package and CLI for browser screenshots, manifest-driven capture batches, image diffs, workflow captures, HTML review reports, and a stdio MCP server.

## Install

```bash
go install github.com/goliatone/webcap/cmd/webcap@latest
```

For local development from this checkout:

```bash
go run ./cmd/webcap shot http://localhost:3000 --full-page --output shots/home.png
```

## CLI

```bash
webcap help
webcap shot http://localhost:3000 --full-page --output shots/home.png
webcap multi ./shots.yaml --output-dir ./shots
webcap diff ./baseline.png ./current.png --output ./diff.png
webcap workflow capture-scenario ./workflow.yaml --run-report
webcap report scenario ./workflow.yaml
webcap mcp serve
```

The standalone CLI does not include application-specific scenario aliases or paths. Provide scenario files explicitly.

## Go Package

```go
engine, err := webcap.NewEngine(webcap.EngineConfig{
    EngineName: webcap.EngineChromium,
    Headless:   true,
})
if err != nil {
    return err
}

service := webcap.NewService(engine)
result, err := service.Capture(ctx, webcap.CaptureRequest{
    URL:        "http://localhost:3000",
    FullPage:   true,
    OutputPath: "shots/home.png",
})
```

Workflow defaults are caller-owned:

```go
opts := webcap.Options{
    Workflow: webcap.WorkflowOptions{
        DefaultSelectedScenario: "launch-success",
        DefaultPresentationMode: "review",
        HandoffQueryParam:       "simulator_handoff",
        BuildHandoff:           webcap.DefaultWorkflowHandoff,
    },
}

scenario, err := webcap.LoadWorkflowScenarioWithOptions("./workflow.yaml", opts.Workflow)
service := webcap.NewServiceWithOptions(engine, opts)
```

## Playwright Runtime

The Playwright engine uses the Node bridge in `playwright_runtime`.

```bash
cd playwright_runtime
npm install
npx playwright install chromium firefox webkit
```

Then run with:

```bash
webcap shot http://localhost:3000 --engine playwright --playwright-browser chromium
```
