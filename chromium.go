package webcap

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/chromedp/cdproto/browser"
	"github.com/chromedp/cdproto/emulation"
	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"
)

const (
	disableAnimationsScript = `(function () {
  const id = "__webcap_disable_animations__";
  const attach = () => {
    if (document.getElementById(id)) return;
    if (!document.documentElement) return;
    const style = document.createElement("style");
    style.id = id;
    style.textContent = "*, *::before, *::after { animation: none !important; transition: none !important; caret-color: transparent !important; } html { scroll-behavior: auto !important; }";
    document.documentElement.appendChild(style);
  };
  if (!document.documentElement) {
    document.addEventListener("DOMContentLoaded", attach, { once: true });
    return;
  }
  attach();
})();`
)

type ChromiumOptions struct {
	BrowserPath string
	Headless    bool
	Now         func() time.Time
}

type ChromiumEngine struct {
	opts ChromiumOptions
}

func NewChromiumEngine(opts ChromiumOptions) *ChromiumEngine {
	return &ChromiumEngine{opts: opts}
}

func (e *ChromiumEngine) Name() string {
	return "chromium"
}

func (e *ChromiumEngine) now() time.Time {
	if e != nil && e.opts.Now != nil {
		return e.opts.Now().UTC()
	}
	return time.Now().UTC()
}

func (e *ChromiumEngine) Capture(ctx context.Context, req CaptureRequest) (EngineResult, error) {
	normalized, err := NormalizeCaptureRequest(req)
	if err != nil {
		return EngineResult{}, err
	}

	captureStartedAt := e.now()
	allocCtx, cancel := chromedp.NewExecAllocator(ctx, e.allocatorOptions(normalized)...)
	defer cancel()

	browserCtx, cancel := chromedp.NewContext(allocCtx)
	defer cancel()

	timeoutCtx, cancel := context.WithTimeout(browserCtx, normalized.TimeoutDuration())
	defer cancel()

	engineResult := e.newEngineResult(normalized)
	if err := chromedp.Run(timeoutCtx, preNavigationActions(normalized)...); err != nil {
		return EngineResult{}, wrapCaptureError("browser_setup", err)
	}
	if err := runChromiumScript(timeoutCtx, "before_navigate", normalized.BeforeNavigateJS); err != nil {
		return EngineResult{}, err
	}

	if warning := e.populateBrowserVersion(timeoutCtx, &engineResult); warning != nil {
		engineResult.Warnings = append(engineResult.Warnings, *warning)
	}

	engineResult.Timing.NavigationStartedAt = e.now()
	if err := chromedp.Run(timeoutCtx, chromedp.Navigate(normalized.URL)); err != nil {
		return EngineResult{}, wrapCaptureError("navigate", err)
	}
	engineResult.Timing.NavigationCompletedAt = e.now()

	if err := runChromiumScript(timeoutCtx, "after_navigate", normalized.AfterNavigateJS); err != nil {
		return EngineResult{}, err
	}
	if err := e.waitForReadiness(timeoutCtx, normalized); err != nil {
		return EngineResult{}, err
	}
	if err := runChromiumScript(timeoutCtx, "before_capture", normalized.BeforeCaptureJS); err != nil {
		return EngineResult{}, err
	}
	engineResult.Timing.ReadyAt = e.now()

	payload, bounds, matchCount, tiling, err := captureChromiumScreenshot(timeoutCtx, normalized)
	if err != nil {
		if tiling != nil {
			engineResult.Tiling = tiling
			if bounds != nil {
				engineResult.Artifact.Bounds = bounds
				engineResult.Artifact.MatchCount = matchCount
			}
			engineResult.Timing.CapturedAt = e.now()
			engineResult.Timing.TotalDuration = engineResult.Timing.CapturedAt.Sub(captureStartedAt).String()
			return engineResult, err
		}
		return EngineResult{}, err
	}
	if bounds != nil {
		engineResult.Artifact.Bounds = bounds
		engineResult.Artifact.MatchCount = matchCount
	}

	engineResult.Artifact.Bytes = payload
	engineResult.Tiling = tiling
	engineResult.Timing.CapturedAt = e.now()
	if engineResult.Timing.ReadyAt.IsZero() {
		engineResult.Timing.ReadyAt = engineResult.Timing.CapturedAt
	}
	engineResult.Timing.TotalDuration = engineResult.Timing.CapturedAt.Sub(captureStartedAt).String()
	return engineResult, nil
}

func (e *ChromiumEngine) allocatorOptions(req CaptureRequest) []chromedp.ExecAllocatorOption {
	options := append([]chromedp.ExecAllocatorOption{}, chromedp.DefaultExecAllocatorOptions[:]...)
	if strings.TrimSpace(e.opts.BrowserPath) != "" {
		options = append(options, chromedp.ExecPath(strings.TrimSpace(e.opts.BrowserPath)))
	}
	return append(
		options,
		chromedp.Flag("headless", e.opts.Headless),
		chromedp.Flag("hide-scrollbars", true),
		chromedp.Flag("mute-audio", true),
		chromedp.WindowSize(req.Viewport.Width, req.Viewport.Height),
	)
}

func (e *ChromiumEngine) newEngineResult(req CaptureRequest) EngineResult {
	result := EngineResult{
		Artifact: CaptureArtifact{
			ImageFormat: defaultImageFormat,
			Mode:        req.Mode(),
			URL:         req.URL,
			Viewport:    req.Viewport,
		},
		Browser: BrowserInfo{
			Engine:      e.Name(),
			BrowserPath: strings.TrimSpace(e.opts.BrowserPath),
			Headless:    e.opts.Headless,
			UserAgent:   req.UserAgent,
		},
	}
	if selectors, _ := req.TargetSelectors(); len(selectors) > 0 {
		result.Artifact.Selectors = append([]string(nil), selectors...)
	}
	return result
}

func preNavigationActions(req CaptureRequest) []chromedp.Action {
	actions := []chromedp.Action{
		emulation.SetDeviceMetricsOverride(
			int64(req.Viewport.Width),
			int64(req.Viewport.Height),
			req.Viewport.ScaleFactor,
			req.Viewport.Mobile,
		),
	}
	if req.UserAgent != "" {
		actions = append(actions, emulation.SetUserAgentOverride(req.UserAgent))
	}
	if req.ReducedMotion {
		actions = append(actions, chromedpEmulateReducedMotion())
	}
	if req.DisableAnimations {
		actions = append(actions, addDisableAnimationsScript())
	}
	return actions
}

func chromedpEmulateReducedMotion() chromedp.Action {
	return emulation.SetEmulatedMedia().WithFeatures([]*emulation.MediaFeature{
		{Name: "prefers-reduced-motion", Value: "reduce"},
	})
}

func addDisableAnimationsScript() chromedp.Action {
	return chromedp.ActionFunc(func(ctx context.Context) error {
		_, err := page.AddScriptToEvaluateOnNewDocument(disableAnimationsScript).Do(ctx)
		return err
	})
}

func runChromiumScript(ctx context.Context, operation, script string) error {
	if script == "" {
		return nil
	}
	var ignored any
	if err := chromedp.Run(ctx, chromedp.Evaluate(script, &ignored)); err != nil {
		return wrapCaptureError(operation, err)
	}
	return nil
}

func (e *ChromiumEngine) populateBrowserVersion(ctx context.Context, result *EngineResult) *CaptureWarning {
	err := chromedp.Run(ctx,
		chromedp.ActionFunc(func(ctx context.Context) error {
			protocolVersion, product, revision, userAgent, jsVersion, err := browser.GetVersion().Do(ctx)
			if err != nil {
				return err
			}
			result.Browser.ProtocolVersion = protocolVersion
			result.Browser.Product = product
			result.Browser.Revision = revision
			result.Browser.JSVersion = jsVersion
			if strings.TrimSpace(result.Browser.UserAgent) == "" {
				result.Browser.UserAgent = userAgent
			}
			return nil
		}),
	)
	if err == nil {
		return nil
	}
	warning := errorWarning(wrapCaptureError("browser_version", err))
	return &warning
}

func (e *ChromiumEngine) waitForReadiness(ctx context.Context, req CaptureRequest) error {
	var ignored any
	actions := make([]chromedp.Action, 0, 6)
	if req.DisableAnimations {
		actions = append(actions, chromedp.Evaluate(disableAnimationsScript, &ignored))
	}
	if readinessAction := e.readinessAction(req); readinessAction != nil {
		actions = append(actions, readinessAction)
	}
	if req.WaitForFonts {
		actions = append(actions, waitForFontsAction(req, &ignored))
	}
	if req.WaitFor != "" {
		actions = append(actions, chromedp.WaitVisible(req.WaitFor, chromedp.ByQuery))
	}
	if req.WaitDuration() > 0 {
		actions = append(actions, chromedp.Sleep(req.WaitDuration()))
	}
	if req.JavaScript != "" {
		actions = append(actions, chromedp.Evaluate(req.JavaScript, &ignored))
	}
	if len(actions) == 0 {
		return nil
	}
	if err := chromedp.Run(ctx, actions...); err != nil {
		return wrapCaptureError("wait_ready", err)
	}
	return nil
}

func waitForFontsAction(req CaptureRequest, ignored *any) chromedp.Action {
	return chromedp.Poll(`document.fonts ? document.fonts.status === "loaded" : true`, ignored,
		chromedp.WithPollingInterval(100*time.Millisecond),
		chromedp.WithPollingTimeout(req.TimeoutDuration()),
	)
}

func captureChromiumScreenshot(ctx context.Context, req CaptureRequest) ([]byte, *Bounds, int, *CaptureTiling, error) {
	switch req.Mode() {
	case CaptureModeFullPage:
		return captureChromiumFullPage(ctx, req)
	case CaptureModeViewport:
		payload, bounds, matchCount, err := captureChromiumViewport(ctx)
		return payload, bounds, matchCount, nil, err
	default:
		return captureChromiumSelector(ctx, req)
	}
}

func captureChromiumFullPage(ctx context.Context, req CaptureRequest) ([]byte, *Bounds, int, *CaptureTiling, error) {
	target, err := measureChromiumFullPageBounds(ctx)
	if err != nil {
		return nil, nil, 0, nil, err
	}
	limits := tileLimits(req)
	if targetExceedsLimits(target, limits) {
		if effectiveOversizePolicy(req) == OversizePolicyFail {
			return nil, nil, 0, nil, newOversizeError("capture_full_page", req.Mode(), target, limits, effectiveOversizePolicy(req))
		}
		tiling, err := captureChromiumTiles(ctx, req, target)
		if err != nil {
			return nil, &target, 0, tiling, err
		}
		tiling.Warnings = append(tiling.Warnings, CaptureWarning{
			Code:    "full_page_tiling",
			Message: "fixed or sticky elements may repeat across full-page tiles",
		})
		return nil, &target, 0, tiling, nil
	}
	var payload []byte
	if err := chromedp.Run(ctx, chromedp.FullScreenshot(&payload, 100)); err != nil {
		return nil, nil, 0, nil, wrapCaptureError("capture_full_page", err)
	}
	return payload, nil, 0, nil, nil
}

func captureChromiumViewport(ctx context.Context) ([]byte, *Bounds, int, error) {
	var payload []byte
	if err := chromedp.Run(ctx, chromedp.CaptureScreenshot(&payload)); err != nil {
		return nil, nil, 0, wrapCaptureError("capture_viewport", err)
	}
	return payload, nil, 0, nil
}

func captureChromiumSelector(ctx context.Context, req CaptureRequest) ([]byte, *Bounds, int, *CaptureTiling, error) {
	script, err := buildSelectorClipScript(req)
	if err != nil {
		return nil, nil, 0, nil, err
	}
	var clip selectorClip
	var payload []byte
	if err := chromedp.Run(ctx, chromedp.Evaluate(script, &clip)); err != nil {
		return nil, nil, 0, nil, wrapCaptureError("capture_selector", err)
	}
	target := Bounds{X: clip.X, Y: clip.Y, Width: clip.Width, Height: clip.Height}
	limits := tileLimits(req)
	if targetExceedsLimits(normalizeTileTarget(target), limits) {
		if effectiveOversizePolicy(req) == OversizePolicyFail {
			return nil, nil, 0, nil, newOversizeError("capture_selector", req.Mode(), normalizeTileTarget(target), limits, effectiveOversizePolicy(req))
		}
		tiling, err := captureChromiumTiles(ctx, req, target)
		if err != nil {
			return nil, &target, clip.MatchCount, tiling, err
		}
		return nil, &target, clip.MatchCount, tiling, nil
	}
	if err := chromedp.Run(ctx, captureSelectorClip(req, &clip, &payload)); err != nil {
		return nil, nil, 0, nil, wrapCaptureError("capture_selector", err)
	}
	return payload, &target, clip.MatchCount, nil, nil
}

func measureChromiumFullPageBounds(ctx context.Context) (Bounds, error) {
	var bounds Bounds
	script := `(() => {
  const doc = document.documentElement;
  const body = document.body;
  const width = Math.max(doc ? doc.scrollWidth : 0, body ? body.scrollWidth : 0, window.innerWidth);
  const height = Math.max(doc ? doc.scrollHeight : 0, body ? body.scrollHeight : 0, window.innerHeight);
  return { x: 0, y: 0, width, height };
})()`
	if err := chromedp.Run(ctx, chromedp.Evaluate(script, &bounds)); err != nil {
		return Bounds{}, wrapCaptureError("measure_full_page", err)
	}
	return normalizeTileTarget(bounds), nil
}

func captureChromiumTiles(ctx context.Context, req CaptureRequest, target Bounds) (*CaptureTiling, error) {
	tiling, err := planTiles(target, req.Tile, req.Viewport.ScaleFactor)
	if err != nil {
		return nil, err
	}
	for idx := range tiling.Tiles {
		payload, captureErr := captureChromiumTile(ctx, req, tiling.Tiles[idx].SourceBounds)
		if captureErr != nil {
			tiling.Tiles[idx].Status = CaptureTileFailed
			tiling.Tiles[idx].Error = captureErr.Error()
			tiling.Status = CaptureTilingPartial
			tiling.FailedCount = 1
			tiling.TileCount = len(tiling.Tiles)
			for _, tile := range tiling.Tiles {
				if tile.Status == CaptureTileCompleted {
					tiling.CompletedCount++
				}
			}
			return tiling, &PartialCaptureError{
				Operation:       "capture_tiles",
				FailedTileIndex: tiling.Tiles[idx].Index,
				CompletedCount:  tiling.CompletedCount,
				TotalCount:      len(tiling.Tiles),
				Err:             captureErr,
			}
		}
		tiling.Tiles[idx].Bytes = payload
		tiling.Tiles[idx].ByteSize = len(payload)
		tiling.Tiles[idx].Status = CaptureTileCompleted
	}
	tiling.CompletedCount = len(tiling.Tiles)
	tiling.TileCount = len(tiling.Tiles)
	tiling.Status = CaptureTilingComplete
	return tiling, nil
}

func captureChromiumTile(ctx context.Context, req CaptureRequest, bounds Bounds) ([]byte, error) {
	var payload []byte
	err := chromedp.Run(ctx, chromedp.ActionFunc(func(ctx context.Context) error {
		screenshot, err := page.CaptureScreenshot().
			WithFormat(page.CaptureScreenshotFormatPng).
			WithCaptureBeyondViewport(true).
			WithClip(&page.Viewport{
				X:      bounds.X,
				Y:      bounds.Y,
				Width:  bounds.Width,
				Height: bounds.Height,
				Scale:  req.Viewport.ScaleFactor,
			}).
			Do(ctx)
		if err != nil {
			return err
		}
		payload = screenshot
		return nil
	}))
	if err != nil {
		var captureErr *Error
		if errors.As(err, &captureErr) {
			return nil, captureErr
		}
		return nil, wrapCaptureError("capture_tile", err)
	}
	return payload, nil
}

func captureSelectorClip(req CaptureRequest, clip *selectorClip, payload *[]byte) chromedp.Action {
	return chromedp.ActionFunc(func(ctx context.Context) error {
		if clip.Width <= 0 || clip.Height <= 0 {
			return newCaptureError(CodeCapture, "capture_selector", "capture clip must have positive width and height", nil)
		}
		screenshot, err := page.CaptureScreenshot().
			WithFormat(page.CaptureScreenshotFormatPng).
			WithCaptureBeyondViewport(true).
			WithClip(&page.Viewport{
				X:      clip.X,
				Y:      clip.Y,
				Width:  clip.Width,
				Height: clip.Height,
				Scale:  req.Viewport.ScaleFactor,
			}).
			Do(ctx)
		if err != nil {
			return err
		}
		*payload = screenshot
		return nil
	})
}

func (e *ChromiumEngine) readinessAction(req CaptureRequest) chromedp.Action {
	switch req.Readiness {
	case "", ReadinessComplete:
		return chromedp.Poll(`document.readyState === "complete"`, new(bool),
			chromedp.WithPollingInterval(100*time.Millisecond),
			chromedp.WithPollingTimeout(req.TimeoutDuration()),
		)
	case ReadinessInteractive:
		return chromedp.Poll(`document.readyState === "interactive" || document.readyState === "complete"`, new(bool),
			chromedp.WithPollingInterval(100*time.Millisecond),
			chromedp.WithPollingTimeout(req.TimeoutDuration()),
		)
	case ReadinessNetworkIdle:
		idleMillis := req.ReadinessIdleDuration().Milliseconds()
		expression := fmt.Sprintf(`(() => {
  if (document.readyState !== "complete") return false;
  const entries = performance.getEntriesByType("resource");
  const lastEnd = entries.length ? Math.max(...entries.map(entry => entry.responseEnd || entry.duration || 0)) : performance.now();
  return performance.now() - lastEnd >= %d;
})()`, idleMillis)
		return chromedp.Poll(expression, new(bool),
			chromedp.WithPollingInterval(100*time.Millisecond),
			chromedp.WithPollingTimeout(req.TimeoutDuration()),
		)
	case ReadinessNone:
		return nil
	default:
		return nil
	}
}

type selectorClip struct {
	X          float64 `json:"x"`
	Y          float64 `json:"y"`
	Width      float64 `json:"width"`
	Height     float64 `json:"height"`
	MatchCount int     `json:"matchCount"`
}

func buildSelectorClipScript(req CaptureRequest) (string, error) {
	selectors, useAll := req.TargetSelectors()
	encoded, err := json.Marshal(selectors)
	if err != nil {
		return "", err
	}
	padding := req.Padding

	return fmt.Sprintf(`(() => {
  const selectors = %s;
  const useAll = %t;
  const padding = %d;
  const rects = [];
  const pageWidth = Math.max(
    document.documentElement.scrollWidth,
    document.body ? document.body.scrollWidth : 0,
    window.innerWidth
  );
  const pageHeight = Math.max(
    document.documentElement.scrollHeight,
    document.body ? document.body.scrollHeight : 0,
    window.innerHeight
  );

  for (const selector of selectors) {
    const nodes = useAll
      ? Array.from(document.querySelectorAll(selector))
      : [document.querySelector(selector)].filter(Boolean);

    if (!nodes.length) {
      throw new Error("selector not found: " + selector);
    }

    for (const node of nodes) {
      const r = node.getBoundingClientRect();
      if (r.width <= 0 || r.height <= 0) {
        continue;
      }
      rects.push({
        x: r.left + window.scrollX,
        y: r.top + window.scrollY,
        width: r.width,
        height: r.height
      });
    }
  }

  if (!rects.length) {
    throw new Error("no visible matches found");
  }

  let left = Infinity;
  let top = Infinity;
  let right = -Infinity;
  let bottom = -Infinity;
  for (const rect of rects) {
    left = Math.min(left, rect.x);
    top = Math.min(top, rect.y);
    right = Math.max(right, rect.x + rect.width);
    bottom = Math.max(bottom, rect.y + rect.height);
  }

  left = Math.max(0, left - padding);
  top = Math.max(0, top - padding);
  right = Math.min(pageWidth, right + padding);
  bottom = Math.min(pageHeight, bottom + padding);

  return {
    x: left,
    y: top,
    width: Math.max(1, right - left),
    height: Math.max(1, bottom - top),
    matchCount: rects.length
  };
})()`, string(encoded), useAll, padding), nil
}
