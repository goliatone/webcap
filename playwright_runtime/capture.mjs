import { chromium, firefox, webkit } from "playwright";
import { captureError, errorEnvelopeFrom, waitForUserFunction } from "./wait_for_function.mjs";

const browserMap = { chromium, firefox, webkit };
const warnings = [];

const chunks = [];
for await (const chunk of process.stdin) chunks.push(chunk);

const payload = JSON.parse(Buffer.concat(chunks).toString("utf8"));
const request = payload.request ?? {};
const options = payload.options ?? {};
const browserName = String(options.browser_name || "chromium").trim().toLowerCase();
const browserType = browserMap[browserName];

if (!browserType) {
  throw new Error(`unsupported playwright browser "${browserName}"`);
}

const timings = {};
const startedAt = new Date();
const viewport = request.viewport ?? {};
const auth = request.auth ?? {};
const guards = request.guards ?? {};
const guardOutcomes = [];
const timeoutMs = parseDuration(request.timeout, 30000);
const waitMs = parseDuration(request.wait, 0);
const idleMs = parseDuration(request.readiness_idle, 500);

const browser = await browserType.launch({
  headless: options.headless !== false,
  executablePath: options.browser_path || undefined,
});

let context;
try {
  const contextOptions = {
    viewport: {
      width: viewport.width || 1440,
      height: viewport.height || 1200,
    },
    deviceScaleFactor: viewport.scale_factor || 1,
    userAgent: request.user_agent || undefined,
    reducedMotion: request.reduced_motion ? "reduce" : "no-preference",
  };
  if (auth.storage_state) {
    contextOptions.storageState = String(auth.storage_state);
  }
  const extraHTTPHeaders = authHeaders(auth.headers);
  if (Object.keys(extraHTTPHeaders).length) {
    contextOptions.extraHTTPHeaders = extraHTTPHeaders;
  }
  if (viewport.mobile && browserName !== "firefox") {
    contextOptions.isMobile = true;
  } else if (viewport.mobile && browserName === "firefox") {
    warnings.push({
      code: "playwright_firefox_mobile_unsupported",
      message: "Firefox does not support Playwright isMobile emulation; using viewport and user agent only.",
    });
  }
  context = await browser.newContext(contextOptions);
  const authCookies = playwrightCookies(auth.cookies, request.url);
  if (authCookies.length) {
    await context.addCookies(authCookies);
  }

  if (request.disable_animations) {
    await context.addInitScript(() => {
      const attach = () => {
        const id = "__webcap_disable_animations__";
        if (document.getElementById(id) || !document.documentElement) return;
        const style = document.createElement("style");
        style.id = id;
        style.textContent =
          "*, *::before, *::after { animation: none !important; transition: none !important; caret-color: transparent !important; } html { scroll-behavior: auto !important; }";
        document.documentElement.appendChild(style);
      };
      if (!document.documentElement) {
        document.addEventListener("DOMContentLoaded", attach, { once: true });
        return;
      }
      attach();
    });
  }

  const page = await context.newPage();

  if (request.before_navigate_js) {
    await page.evaluate(String(request.before_navigate_js));
  }

  timings.navigation_started_at = new Date().toISOString();
  await page.goto(String(request.url), {
    waitUntil: mapWaitUntil(request.readiness),
    timeout: timeoutMs,
  });
  timings.navigation_completed_at = new Date().toISOString();
  verifyURLGuards(guards, page.url());

  if (request.after_navigate_js) {
    await page.evaluate(String(request.after_navigate_js));
  }

  if (request.disable_animations) {
    await page.evaluate(() => {
      const id = "__webcap_disable_animations__";
      if (document.getElementById(id) || !document.documentElement) return;
      const style = document.createElement("style");
      style.id = id;
      style.textContent =
        "*, *::before, *::after { animation: none !important; transition: none !important; caret-color: transparent !important; } html { scroll-behavior: auto !important; }";
      document.documentElement.appendChild(style);
    });
  }

  await applyReadiness(page, request.readiness, timeoutMs, idleMs);

  if (request.wait_for_fonts) {
    await page.evaluate(async () => {
      if (!document.fonts) return;
      await document.fonts.ready;
    });
  }

  if (request.wait_for) {
    await page.locator(String(request.wait_for)).first().waitFor({
      state: "visible",
      timeout: timeoutMs,
    });
  }

  if (request.wait_for_function) {
    await waitForUserFunction(page, request.wait_for_function, timeoutMs);
  }

  if (waitMs > 0) {
    await page.waitForTimeout(waitMs);
  }

  if (request.javascript) {
    await page.evaluate(String(request.javascript));
  }

  if (request.before_capture_js) {
    await page.evaluate(String(request.before_capture_js));
  }
  guardOutcomes.push(...verifyURLGuards(guards, page.url()));
  guardOutcomes.push(...(await verifySelectorGuards(page, guards, page.url())));

  timings.ready_at = new Date().toISOString();

  const selectors = targetSelectors(request);
  let artifact = {
    image_format: "png",
    mode: captureMode(request),
    url: page.url(),
    viewport: viewport,
    selectors,
  };
  let bytes;

  if (request.full_page) {
    bytes = await page.screenshot({ fullPage: true, type: "png" });
  } else if (!selectors.length) {
    bytes = await page.screenshot({ type: "png" });
  } else {
    const clip = await page.evaluate(
      ({ selectors, useAll, padding }) => {
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
            throw new Error(`selector not found: ${selector}`);
          }
          for (const node of nodes) {
            const r = node.getBoundingClientRect();
            if (r.width <= 0 || r.height <= 0) continue;
            rects.push({
              x: r.left + window.scrollX,
              y: r.top + window.scrollY,
              width: r.width,
              height: r.height,
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
          matchCount: rects.length,
        };
      },
      { selectors, useAll: useAllSelectors(request), padding: request.padding || 0 }
    );

    bytes = await page.screenshot({
      type: "png",
      clip: {
        x: clip.x,
        y: clip.y,
        width: clip.width,
        height: clip.height,
      },
    });
    artifact = {
      ...artifact,
      bounds: {
        x: clip.x,
        y: clip.y,
        width: clip.width,
        height: clip.height,
      },
      match_count: clip.matchCount,
    };
  }

  const capturedAt = new Date();
  const browserInfo = {
    engine: "playwright",
    browser_path: options.browser_path || "",
    product: `${browserName}/${browser.version()}`,
    user_agent: await page.evaluate(() => navigator.userAgent),
    headless: options.headless !== false,
  };
  const totalDurationMs = capturedAt.getTime() - startedAt.getTime();

  process.stdout.write(
    JSON.stringify({
      artifact,
      browser: browserInfo,
      timing: {
        navigation_started_at: timings.navigation_started_at,
        navigation_completed_at: timings.navigation_completed_at,
        ready_at: timings.ready_at,
        captured_at: capturedAt.toISOString(),
        total_duration: formatDuration(totalDurationMs),
      },
	      warnings,
	      guards: guardOutcomes,
	      bytes_base64: Buffer.from(bytes).toString("base64"),
	    })
	  );
} catch (err) {
  process.stderr.write(JSON.stringify(errorEnvelopeFrom(err)));
  process.exitCode = 1;
} finally {
  if (context) await context.close();
  await browser.close();
}

function mapWaitUntil(readiness) {
  switch (String(readiness || "complete")) {
    case "interactive":
      return "domcontentloaded";
    case "network_idle":
      return "load";
    case "none":
      return "commit";
    case "complete":
    default:
      return "load";
  }
}

async function applyReadiness(page, readiness, timeoutMs, idleMs) {
  switch (String(readiness || "complete")) {
    case "network_idle":
      await page.waitForLoadState("networkidle", { timeout: timeoutMs }).catch(async () => {
        await page.waitForFunction(
          (idleTarget) => {
            if (document.readyState !== "complete") return false;
            const entries = performance.getEntriesByType("resource");
            const lastEnd = entries.length
              ? Math.max(...entries.map((entry) => entry.responseEnd || entry.duration || 0))
              : performance.now();
            return performance.now() - lastEnd >= idleTarget;
          },
          idleMs,
          { timeout: timeoutMs }
        );
      });
      return;
    case "interactive":
      await page.waitForLoadState("domcontentloaded", { timeout: timeoutMs });
      return;
    case "none":
      return;
    case "complete":
    default:
      await page.waitForLoadState("load", { timeout: timeoutMs });
  }
}

function targetSelectors(request) {
  if (request.selector) return [String(request.selector)];
  if (Array.isArray(request.selectors) && request.selectors.length) return request.selectors.map(String);
  if (request.selector_all) return [String(request.selector_all)];
  if (Array.isArray(request.selectors_all) && request.selectors_all.length) return request.selectors_all.map(String);
  return [];
}

function useAllSelectors(request) {
  return !!request.selector_all || (Array.isArray(request.selectors_all) && request.selectors_all.length > 0);
}

function captureMode(request) {
  if (request.full_page) return "full_page";
  if (request.selector) return "selector";
  if (Array.isArray(request.selectors) && request.selectors.length) return "selectors";
  if (request.selector_all) return "selector_all";
  if (Array.isArray(request.selectors_all) && request.selectors_all.length) return "selectors_all";
  return "viewport";
}

function parseDuration(raw, fallback) {
  if (!raw) return fallback;
  const value = String(raw).trim();
  if (!value) return fallback;
  const match = value.match(/^(\d+(?:\.\d+)?)(ms|s|m|h)$/);
  if (!match) return fallback;
  const amount = Number(match[1]);
  const unit = match[2];
  switch (unit) {
    case "ms":
      return amount;
    case "s":
      return amount * 1000;
    case "m":
      return amount * 60_000;
    case "h":
      return amount * 3_600_000;
    default:
      return fallback;
  }
}

function formatDuration(ms) {
  if (ms < 1000) return `${ms}ms`;
  const seconds = (ms / 1000).toFixed(3).replace(/0+$/, "").replace(/\.$/, "");
  return `${seconds}s`;
}

function authHeaders(headers) {
  const out = {};
  if (!Array.isArray(headers)) return out;
  for (const header of headers) {
    const name = String(header?.name || "").trim();
    if (!name) continue;
    out[name] = String(header?.value ?? "");
  }
  return out;
}

function playwrightCookies(cookies, targetURL) {
  if (!Array.isArray(cookies)) return [];
  return cookies
    .map((cookie) => {
      const out = {
        name: String(cookie?.name || "").trim(),
        value: String(cookie?.value ?? ""),
        path: String(cookie?.path || "/"),
        secure: !!cookie?.secure,
        httpOnly: !!cookie?.httpOnly,
      };
      if (!out.name) return null;
      const domain = String(cookie?.domain || "").trim();
      if (domain && !cookie?.HostOnly) {
        out.domain = domain;
      } else {
        out.url = cookieURLForTarget(targetURL, domain, out.path);
      }
      const sameSite = normalizeSameSite(cookie?.sameSite);
      if (sameSite) out.sameSite = sameSite;
      const expires = Number(cookie?.expires || 0);
      if (expires > 0) out.expires = expires;
      return out;
    })
    .filter(Boolean);
}

function cookieURLForTarget(targetURL, domain, path) {
  try {
    const parsed = new URL(String(targetURL));
    if (domain && domain !== "localhost") parsed.hostname = domain.replace(/^\./, "");
    parsed.pathname = path || "/";
    parsed.search = "";
    parsed.hash = "";
    return parsed.toString();
  } catch {
    return String(targetURL);
  }
}

function normalizeSameSite(value) {
  switch (String(value || "").trim().toLowerCase()) {
    case "strict":
      return "Strict";
    case "lax":
      return "Lax";
    case "none":
      return "None";
    default:
      return undefined;
  }
}

function verifyURLGuards(guards, finalURL) {
  const url = String(finalURL || "");
  const outcomes = [];
  const expectURL = String(guards?.expect_url || "").trim();
  if (expectURL) {
    const matched = url.includes(expectURL);
    outcomes.push(guardOutcome("expect_url", expectURL, url, matched, matched));
    if (!matched) {
      throw captureError("capture_error", "verify_url_guard", "final URL did not contain expected substring", {
        guard: "expect_url",
        expect_url: expectURL,
        final_url: url,
      });
    }
  }
  for (const forbidden of Array.isArray(guards?.fail_on_url) ? guards.fail_on_url : []) {
    const value = String(forbidden || "").trim();
    if (!value) continue;
    const matched = url.includes(value);
    outcomes.push(guardOutcome("fail_on_url", value, url, matched, !matched));
    if (matched) {
      throw captureError("capture_error", "verify_url_guard", "final URL matched forbidden substring", {
        guard: "fail_on_url",
        fail_on_url: value,
        final_url: url,
      });
    }
  }
  return outcomes;
}

async function verifySelectorGuards(page, guards, finalURL) {
  const selectors = Array.isArray(guards?.fail_on_selector)
    ? guards.fail_on_selector.map((value) => String(value || "").trim()).filter(Boolean)
    : [];
  if (!selectors.length) return [];
  const matched = await page.evaluate((items) => {
    for (const selector of items) {
      const nodes = Array.from(document.querySelectorAll(selector));
      for (const node of nodes) {
        const style = window.getComputedStyle(node);
        const rect = node.getBoundingClientRect();
        if (style && style.visibility !== "hidden" && style.display !== "none" && rect.width > 0 && rect.height > 0) {
          return selector;
        }
      }
    }
    return "";
  }, selectors);
  if (matched) {
    throw captureError("capture_error", "verify_selector_guard", "page matched forbidden selector", {
      guard: "fail_on_selector",
      selector: matched,
      final_url: String(finalURL || ""),
    });
  }
  return selectors.map((selector) =>
    guardOutcome("fail_on_selector", selector, String(finalURL || ""), selector === matched, selector !== matched)
  );
}

function guardOutcome(kind, value, finalURL, matched, passed) {
  return {
    kind,
    value,
    final_url: finalURL,
    matched,
    status: passed ? "passed" : "failed",
  };
}
