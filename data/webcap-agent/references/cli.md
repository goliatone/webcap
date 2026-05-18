# webcap CLI reference

Use the CLI as a fallback when MCP tools are not available or when a reproducible command is useful.

Result commands print human summaries by default. Add `--json` or `--format json` when an agent or script needs structured output. Add `--no-color` for deterministic redirected output.
`webcap shot <url>` captures the full page by default. Add `--visible` only when the current viewport is the intended artifact.

## Single screenshot

```bash
webcap shot \
  --json \
  --viewport-preset desktop-xl \
  --readiness complete \
  --disable-animations \
  --reduced-motion \
  --wait-for-fonts \
  --output artifacts/current/home.png \
  http://localhost:3000
```

For a visible viewport only:

```bash
webcap shot \
  --visible \
  --output artifacts/current/home-visible.png \
  http://localhost:3000
```

For a component or region:

```bash
webcap shot \
  --selector ".checkout-panel" \
  --padding 16 \
  --output artifacts/current/checkout-panel.png \
  http://localhost:3000
```

For app-owned readiness:

```bash
webcap shot \
  --wait-for-function 'window.__webcapReady === true' \
  --output artifacts/current/home.png \
  http://localhost:3000
```

For a Storybook iframe:

```bash
webcap shot \
  --wait-for-function 'document.body.classList.contains("sb-show-main") && !document.querySelector(".sb-preparing,.sb-errordisplay") && document.querySelector("#storybook-root")?.children.length > 0' \
  --output artifacts/current/button-primary.png \
  'http://localhost:6006/iframe.html?id=components-button--primary'
```

For an oversized full-page or selector target, opt into tile artifacts:

```bash
webcap shot \
  --selector ".long-table" \
  --oversize tile \
  --tile-max-height 4096 \
  --output artifacts/current/long-table.png \
  http://localhost:3000
```

Unstitched tiled captures write deterministic tile files and metadata. Add `--tile-stitch` only when a single image is needed for pixel or semantic comparison.

## Manifest capture

```bash
webcap multi --output-dir artifacts/current ./webcap.yaml
```

Use manifests for repeatable multi-screen capture. Keep output paths stable so reports and diffs can be compared between runs.

## Pixel diff

```bash
webcap diff \
  --threshold 0.05 \
  --output artifacts/diff/home.png \
  --metadata artifacts/diff/home.json \
  artifacts/reference/home.png artifacts/current/home.png
```

## Semantic diff

```bash
OPENAI_API_KEY=... webcap semantic-diff \
  --json \
  --provider openai \
  --model gpt-5.1 \
  --mode focused \
  --focus "primary CTA,nav labels,form state" \
  --pixel-diff-image artifacts/diff/home.png \
  --metadata artifacts/semantic/home.json \
  artifacts/current/home.png artifacts/reference/home.png
```

## Workflow report

```bash
webcap workflow capture-scenario --run-report ./workflow.yaml
webcap report scenario --json ./workflow.yaml
```

Use workflow reports for multi-step UI reviews where each screen has stable identifiers, reference images, and review policy.
