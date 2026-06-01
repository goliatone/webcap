import { captureError } from "./wait_for_function.mjs";

export function verifyURLGuards(guards, finalURL) {
  const url = String(finalURL || "");
  const outcomes = [];
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
  return outcomes;
}

export function guardOutcome(kind, value, finalURL, matched, passed) {
  return {
    kind,
    value,
    final_url: finalURL,
    matched,
    status: passed ? "passed" : "failed",
  };
}
