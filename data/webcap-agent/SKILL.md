---
name: webcap-agent
description: Use webcap MCP tools or CLI commands to capture browser screenshots, compare visual artifacts, and produce review reports.
---

# webcap agent skill

Use this skill when you need browser screenshots, manifest captures, pixel diffs, semantic visual diffs, or workflow review reports for a local app or site.

## Default approach

Prefer the `webcap` MCP tools when they are available in the current agent runtime. They provide structured inputs and outputs, avoid brittle shell parsing, and keep capture and diff artifacts predictable.

Use the `webcap` CLI when MCP tools are not available, when you need a reproducible command for a report, or when the user explicitly asks for terminal commands.

CLI result commands print human summaries by default. Use `--json` or `--format json` for machine-readable output, and use `--no-color` when capturing output for tests, scripts, or logs.

## Capture defaults

For deterministic captures, set a fixed viewport or viewport preset, disable animations, use reduced motion, and wait for fonts. Use `network_idle` readiness only when the page has a stable network boundary; otherwise use selector waits or `wait_for_function` predicate waits for the UI state that matters.

Use `wait_for_function` through MCP `capture_page` and `capture_section` arguments, or `--wait-for-function` in the CLI, when app readiness depends on client state that is not captured by navigation readiness. Prefer stable app-owned markers such as `data-webcap-ready` over fixed sleeps.

Plain `webcap shot <url>` captures the full page by default. Add `--visible` when the current viewport is the intended artifact. Use selector capture when the review target is a component or region.

For very tall full-page or selector targets, keep the default fail-fast behavior unless tile artifacts are acceptable. Use `--oversize tile` with explicit tile limits, and add `--tile-stitch` only when downstream comparison needs a single image.

For auth-guarded pages, use `webcap auth login` and `webcap auth inspect` for local go-admin style form login when possible, then pass the cookie jar into `webcap shot`. Otherwise acquire auth outside `webcap` and pass it into the capture. Prefer `cookie_jar`, Playwright `storage_state`, explicit `cookies`, or safe local-only `headers` plus `expect_url` and `fail_on_url`/`fail_on_selector` guards. Do not add fixed waits to hide login redirects; guards should fail the capture when the browser lands on an auth fallback.

When using `webcap auth login`, pass passwords through `--password-env`, set the app-specific `--expect-cookie` (`admin_session` for Garchen-style local admin, `admin_user` for some go-admin examples), and treat `admin_debug_session` as debug-only, not authenticated state. Custom scripts receive `WEBCAP_*` variables and write `WEBCAP_COOKIE_JAR`; their stdout/stderr are diagnostics and are redacted before output.

Treat cookie values, authorization headers, and storage-state contents as secrets. `webcap` redacts them from metadata and structured output, but command lines can still be visible in shell history; prefer files or environment expansion for sensitive values.

## Artifact conventions

Keep artifacts grouped by scenario or task:

- current captures: `artifacts/current/`
- reference captures: `artifacts/reference/`
- pixel diffs: `artifacts/diff/`
- semantic diff metadata: `artifacts/semantic/`
- workflow reports: `artifacts/report/`

Name files after the route, screen, component, or workflow step being reviewed. Prefer stable names over timestamps when the artifact is meant to be compared across runs.

## Review workflow

1. Capture the current UI with MCP tools or the CLI.
2. Capture or locate the reference UI.
3. Run a pixel diff for measurable image changes.
4. Run semantic diff when visual meaning, copy, layout intent, accessibility, or interaction state needs judgment.
5. Generate a workflow report when reviewing multiple screens or steps.
6. Summarize findings with artifact paths, changed areas, severity, and any fixes made.

See `references/cli.md` for CLI command examples and `references/visual-review.md` for review guidance.
