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
go run ./cmd/webcap shot --output shots/home.png http://localhost:3000
```

## CLI

```bash
webcap help
webcap shot --output shots/home.png http://localhost:3000
webcap shot --visible --output shots/home-visible.png http://localhost:3000
webcap shot --json --output shots/home.png http://localhost:3000
webcap multi --output-dir ./shots ./shots.yaml
webcap diff --output ./diff.png ./baseline.png ./current.png
OPENAI_API_KEY=... webcap semantic-diff --provider openai --model gpt-5.1 --focus "primary CTA,nav labels" --metadata ./semantic.json ./current.png ./baseline.png
webcap semantic-diff --provider codex-cli --model gpt-5.1 --codex-profile work --metadata ./semantic.json ./current.png ./baseline.png
webcap workflow capture-scenario --run-report ./workflow.yaml
webcap report scenario ./workflow.yaml
webcap mcp serve
webcap skill install --agent codex
webcap skill install --agent claude
webcap skill install --agent codex --force
```

Result commands render concise human summaries by default. Use `--json` or `--format json` for the machine-readable result structs used by automation and future API responses. Use `--format human` to force human output and `--no-color` for deterministic output in CI, redirected output, and tests.
`webcap shot <url>` captures the full page by default. Add `--visible` when you only want the current viewport.
Oversized full-page or selector captures fail early by default with structured limit metadata. Use `--oversize tile` to opt into deterministic tile artifacts, for example `--tile-max-height 4096`; unstitched tiled captures write `<output>.tile-0000.png` style files plus the metadata sidecar. Add `--tile-stitch` when the capture must produce a single image for pixel or semantic comparison.

Browser readiness modes such as `complete` and `network_idle` do not always mean the app UI is ready. Use selector waits for visible DOM targets, or `--wait-for-function` when readiness is best expressed as JavaScript:

```bash
webcap shot \
  --wait-for-function 'window.__webcapReady === true' \
  --output shots/home.png \
  http://localhost:3000
```

For important visual review flows, prefer stable app-owned markers such as `data-webcap-ready="true"`. For Storybook iframe captures, wait for the preview to show the story and for the root to contain rendered content:

```bash
webcap shot \
  --wait-for-function 'document.body.classList.contains("sb-show-main") && !document.querySelector(".sb-preparing,.sb-errordisplay") && document.querySelector("#storybook-root")?.children.length > 0' \
  --output shots/story.png \
  http://localhost:6006/iframe.html?id=components-button--primary
```

## Authenticated captures

`webcap shot` does not script login forms. Use `webcap auth login` for local form-login helpers, or acquire auth state with app tooling, `curl`, or Playwright, then hand that state to the capture and add guards so redirects to login fail clearly.

Cookie-authenticated local admin flow:

```bash
BASE_URL=http://localhost:9090
TARGET_URL="$BASE_URL/admin/translations/queue"
COOKIE_JAR=/tmp/admin-cookies.txt
export ADMIN_PASSWORD='<local-dev-password>'

webcap auth login \
  --base-url "$BASE_URL" \
  --target-url "$TARGET_URL" \
  --cookie-jar "$COOKIE_JAR" \
  --identifier admin@example.test \
  --password-env ADMIN_PASSWORD \
  --expect-cookie admin_session

webcap auth inspect \
  --cookie-jar "$COOKIE_JAR" \
  --url "$TARGET_URL" \
  --expect-cookie admin_session

webcap shot \
  --cookie-jar "$COOKIE_JAR" \
  --expect-url /admin/translations/queue \
  --fail-on-url /admin/login \
  --fail-on-selector 'form[action*="/login"]' \
  --output shots/translations-queue.png \
  "$TARGET_URL"
```

The embedded login helper fetches `/admin/login`, extracts CSRF from `X-CSRF-Token` or a hidden `_token` input, posts `identifier` and `password`, and validates the expected cookie. Use the app-specific cookie name: Garchen-style local admin apps commonly use `admin_session`; other go-admin examples may use `admin_user`. `admin_debug_session` is only a debug cookie and is not treated as auth.

Custom login scripts receive the same environment contract and should write the cookie jar:

```bash
webcap auth login \
  --script ./custom-login.sh \
  --base-url "$BASE_URL" \
  --target-url "$TARGET_URL" \
  --cookie-jar "$COOKIE_JAR" \
  --identifier admin@example.test \
  --password-env ADMIN_PASSWORD \
  --expect-cookie admin_session
```

Custom scripts run under `bash` with `WEBCAP_BASE_URL`, `WEBCAP_LOGIN_PATH`, `WEBCAP_COOKIE_JAR`, `WEBCAP_IDENTIFIER`, `WEBCAP_PASSWORD`, `WEBCAP_EXPECT_COOKIE`, and optional `WEBCAP_TARGET_URL`. Script stdout and stderr are diagnostic output and are redacted before presentation.

For local-only development auth headers:

```bash
webcap shot \
  --header "Authorization: Bearer $WEB_AUTH_TOKEN" \
  --expect-url /admin \
  --fail-on-url /login \
  http://localhost:9090/admin
```

For Playwright teams, pass a storage-state file:

```bash
webcap shot \
  --engine playwright \
  --storage-state .auth/admin-state.json \
  --expect-url /admin \
  http://localhost:9090/admin
```

Cookie jars and explicit cookies work in both engines. Playwright storage state is full-fidelity in the Playwright engine. Chromium imports cookies from Playwright storage-state files only when `origins` is empty; origin localStorage/sessionStorage currently fails with `unsupported_error` instead of being silently ignored. URL guards use substring matching, not regex or glob syntax.

Auth values are redacted from `resolved_config`, metadata sidecars, JSON errors, MCP responses, warnings, and human output. Shell history is outside `webcap`'s control, so prefer files or environment expansion for secrets.

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
    URL:             "http://localhost:3000",
    FullPage:        true,
    WaitForFunction: `document.querySelector("[data-webcap-ready=true]") !== null`,
    OutputPath:      "shots/home.png",
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
Hosts can also configure semantic payload budgets such as `MaxImageBytes`, `MaxProviderImageBytes`, `MaxImageLongEdge`, `MaxImagePixels`, `MaxEncodedImageBytes`, `MaxCombinedEncodedImageBytes`, and `MaxRequestBodyBytes`. `MaxImageBytes` is a hard source-file guard before reads; set `ResizeImages: true` to shrink temporary provider-safe image copies for provider, encoded, and final request budgets while preserving the original artifacts.
Provider failures use stable error codes such as `provider_rate_limited`, `provider_auth`, `provider_quota`, `provider_invalid_request`, `provider_payload_too_large`, `provider_unavailable`, `provider_timeout`, and `provider_execution_failed`; JSON output and workflow warnings include only safe metadata like status, retry-after, request IDs, provider error codes, and budget values.

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
webcap shot --engine playwright --playwright-browser chromium http://localhost:3000
```
