# webcap CLI reference

Use the CLI as a fallback when MCP tools are not available or when a reproducible command is useful.

## Single screenshot

```bash
webcap shot http://localhost:3000 \
  --viewport-preset desktop-xl \
  --readiness complete \
  --disable-animations \
  --reduced-motion \
  --wait-for-fonts \
  --output artifacts/current/home.png
```

For a component or region:

```bash
webcap shot http://localhost:3000 \
  --selector ".checkout-panel" \
  --padding 16 \
  --output artifacts/current/checkout-panel.png
```

## Manifest capture

```bash
webcap multi ./webcap.yaml --output-dir artifacts/current
```

Use manifests for repeatable multi-screen capture. Keep output paths stable so reports and diffs can be compared between runs.

## Pixel diff

```bash
webcap diff artifacts/reference/home.png artifacts/current/home.png \
  --threshold 0.05 \
  --output artifacts/diff/home.png \
  --metadata artifacts/diff/home.json
```

## Semantic diff

```bash
OPENAI_API_KEY=... webcap semantic-diff artifacts/current/home.png artifacts/reference/home.png \
  --provider openai \
  --model gpt-5.1 \
  --mode focused \
  --focus "primary CTA,nav labels,form state" \
  --pixel-diff-image artifacts/diff/home.png \
  --metadata artifacts/semantic/home.json
```

## Workflow report

```bash
webcap workflow capture-scenario ./workflow.yaml --run-report
webcap report scenario ./workflow.yaml
```

Use workflow reports for multi-step UI reviews where each screen has stable identifiers, reference images, and review policy.
