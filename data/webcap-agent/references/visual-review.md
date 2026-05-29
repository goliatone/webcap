# Visual review guidance

## MCP-first usage

When `webcap` MCP tools are available, call them directly for capture, diff, semantic diff, and workflow operations. Use CLI examples only as a fallback or when recording reproducible commands in a report.

## Deterministic captures

- Use fixed viewports or named presets.
- Disable CSS animations and transitions.
- Emulate reduced motion when animation is not under review.
- Wait for fonts before capture.
- Prefer selector waits or `wait_for_function` predicate waits over arbitrary sleep.
- Use `network_idle` only for pages that stop background polling.

Use `wait_for_function` for app-rendered state such as hydrated dashboards, Storybook iframes, and route-level data loading. MCP `capture_page` and `capture_section` accept `wait_for_function`; the CLI flag is `--wait-for-function`.

## Auth-guarded captures

For protected routes, use auth handoff plus guards instead of fixed waits. Provide `cookie_jar`, `storage_state`, explicit `cookies`, or safe local-only `headers` before navigation, then configure `expect_url`, `fail_on_url`, and a login/unauthorized `fail_on_selector` when available.

If a capture shows a login page, treat it as an auth or guard failure. Do not accept the artifact as a valid visual baseline.

## Selector strategy

Use `--selector` for a single target, `--selectors` for a union of specific regions, `--selector-all` for repeated elements, and padding when neighboring visual context matters.

Avoid cropping so tightly that the user cannot understand the surrounding state.

## Diff strategy

Run pixel diff before semantic diff. Pixel diff answers what changed; semantic diff answers whether the change matters.

Use semantic modes intentionally:

- `focused` for known areas such as CTA, navigation, or form state.
- `copy` for text changes.
- `layout` for spacing, alignment, and responsive regressions.
- `accessibility` for visible contrast, focus, and state concerns.
- `general` when triaging broad visual changes.

## Reporting

Report the artifact paths, verdict, severity, and specific visible issues. Separate confirmed regressions from expected changes and from observations that need product or design judgment.
