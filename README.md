# webcap

`webcap` is a Go package and CLI for browser screenshots, manifest-driven capture batches, image diffs, workflow captures, HTML review reports, and a stdio MCP server.

## Install

Install the released binary with Homebrew:

```bash
brew tap goliatone/homebrew-tap
brew install webcap
```

Or install from source with Go:

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
OPENAI_API_KEY=... webcap semantic-diff ./current.png ./baseline.png --provider openai --model gpt-5.1 --focus "primary CTA,nav labels" --metadata ./semantic.json
webcap semantic-diff ./current.png ./baseline.png --provider codex-cli --model gpt-5.1 --codex-profile work --metadata ./semantic.json
webcap workflow capture-scenario ./workflow.yaml --run-report
webcap report scenario ./workflow.yaml
webcap mcp serve
webcap skill install --agent codex
webcap skill install --agent claude
webcap skill install --agent codex --force
```

The standalone CLI does not include application-specific scenario aliases or paths. Provide scenario files explicitly.
Use `--openai-base-url` or `--anthropic-base-url` with `semantic-diff`, `workflow capture-scenario`, `report scenario`, or `mcp serve` when routing built-in providers through a local fake server, proxy, or compatible gateway.
Use `--codex-bin`, `--codex-profile`, `--codex-oss`, `--codex-local-provider`, and repeated `--codex-extra-arg` flags to configure the local Codex CLI process. CLI providers reuse local tool configuration and may still use subscriptions, API accounts, or local model resources.

`webcap skill install` installs the bundled `webcap-agent` guidance for AI coding agents. The skill tells agents to prefer webcap MCP tools when available, use the CLI as a fallback or reproducible command surface, and keep capture, pixel diff, semantic diff, and workflow report artifacts organized. Existing modified skill files are not overwritten unless `--force` is provided.

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

Semantic diff is opt-in and uses provider API keys from process configuration, not workflow YAML:

```go
service := webcap.NewServiceWithOptions(nil, webcap.Options{
    SemanticDiff: webcap.SemanticDiffOptions{
        DefaultProvider: "openai",
        DefaultModels: map[string]string{
            "openai": "gpt-5.1",
        },
    },
})

result, err := service.SemanticDiff(ctx, webcap.SemanticDiffRequest{
    CurrentPath:   "shots/current.png",
    ReferencePath: "shots/baseline.png",
    Mode:          webcap.SemanticDiffModeFocused,
    Focus:         []string{"primary CTA", "navigation labels"},
    MetadataPath:  "shots/current.semantic.json",
})
```

`NewService` and `NewServiceWithOptions` include the shipped `openai`, `anthropic`, and `codex-cli` providers by default. Register standalone LLM providers through `pkg/llms.Options.Providers`; `SemanticDiffOptions.Providers` remains available for direct `webcap.SemanticDiffProvider` implementations and overrides any adapted LLM provider with the same normalized name:

```go
service := webcap.NewServiceWithOptions(nil, webcap.Options{
    SemanticDiff: webcap.SemanticDiffOptions{
        LLMs: llms.Options{
            Providers: map[string]llms.Provider{
                "local-agent": myLLMProvider,
            },
        },
        DefaultModels: map[string]string{
            "local-agent": "gpt-5.1",
        },
    },
})
```

Set `OPENAI_API_KEY` or `ANTHROPIC_API_KEY` for the built-in providers. Raw provider responses are not written into metadata; use `PersistRawResponse` or `--persist-raw-response` only for local debugging because screenshots and model output can contain sensitive UI content.
Hosts can set `Options.SemanticDiff.RedactImage` to replace or reject screenshot paths before provider payload encoding.

Configure Codex CLI at process level, not in workflow YAML:

```go
service := webcap.NewServiceWithOptions(nil, webcap.Options{
    SemanticDiff: webcap.SemanticDiffOptions{
        LLMs: llms.Options{
            CodexCLI: llms.CodexCLIOptions{
                CommandPath: "/usr/local/bin/codex",
                Profile:     "work",
            },
        },
    },
})
```

Workflow reports can run semantic diff after the normal pixel diff and reuse the existing diff image as context:

```yaml
defaults:
  semantic_diff:
    enabled: true
    provider: openai
    model: gpt-5.1
    mode: general
    run: changed_only
    focus:
      - primary CTA visibility
      - navigation labels
```

Use `provider: codex-cli` in workflow YAML when the process already configured the Codex binary/profile flags or package options. Claude CLI support is deferred until local image input behavior is proven.

Per-screen overrides can change prompt mode or focus:

```yaml
screens:
  - id: checkout
    route: /checkout
    reference_image: refs/checkout.png
    semantic_diff:
      mode: focused
      prompt: Compare only the checkout form and payment button state.
```

Semantic findings are advisory by default. To make them affect workflow report status, configure an explicit policy such as `advisory_policy: enforce` with `failure_severity: major` or `failure_verdicts: [regression]`.

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
