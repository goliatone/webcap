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

## Auth-guarded capture

Use `webcap auth login` for local form-login helpers, or acquire auth state outside `webcap`, then pass it into the capture and add guards for wrong-page detection.

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
  --output artifacts/current/translations-queue.png \
  "$TARGET_URL"
```

The embedded helper supports go-admin style CSRF login by reading `X-CSRF-Token` or a hidden `_token` input. Use the app-specific expected cookie: Garchen-style local admin apps commonly use `admin_session`; other go-admin examples may use `admin_user`. `admin_debug_session` is only a debug cookie.

Custom scripts use the same contract and write `WEBCAP_COOKIE_JAR`:

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

Custom scripts receive `WEBCAP_BASE_URL`, `WEBCAP_LOGIN_PATH`, `WEBCAP_COOKIE_JAR`, `WEBCAP_IDENTIFIER`, `WEBCAP_PASSWORD`, `WEBCAP_EXPECT_COOKIE`, and optional `WEBCAP_TARGET_URL`. Script stdout and stderr are diagnostic output and are redacted before presentation.

For local-only header auth:

```bash
webcap shot \
  --header "Authorization: Bearer $WEB_AUTH_TOKEN" \
  --expect-url /admin \
  --fail-on-url /login \
  --output artifacts/current/admin.png \
  http://localhost:9090/admin
```

For Playwright storage state:

```bash
webcap shot \
  --engine playwright \
  --storage-state .auth/admin-state.json \
  --expect-url /admin \
  --output artifacts/current/admin.png \
  http://localhost:9090/admin
```

Cookie jars and explicit cookies work in Chromium and Playwright. Playwright storage state is full-fidelity only with the Playwright engine; Chromium imports cookies only and rejects origin storage. URL guards are substring matches.

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
