# Visual review guidance

## MCP-first usage

When `webcap` MCP tools are available, call them directly for capture, diff, semantic diff, and workflow operations. Use CLI examples only as a fallback or when recording reproducible commands in a report.

## Deterministic captures

- Use fixed viewports or named presets.
- Disable CSS animations and transitions.
- Emulate reduced motion when animation is not under review.
- Wait for fonts before capture.
- Prefer selector waits over arbitrary sleep.
- Use `network_idle` only for pages that stop background polling.

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
