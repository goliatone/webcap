const waitMetadata = { wait: "wait_for_function" };
const timeoutMessage = "wait_for_function did not become truthy before timeout";
const failureMessage = "wait_for_function predicate failed";

export function captureError(code, operation, message, metadata = undefined) {
  const err = new Error(message);
  err.webcap = {
    code,
    operation,
    message,
    metadata,
  };
  return err;
}

export function errorEnvelopeFrom(err) {
  if (err?.webcap) {
    return err.webcap;
  }
  const message = String(err?.message || err || "playwright capture failed").trim() || "playwright capture failed";
  return {
    code: "capture_error",
    operation: "playwright_capture",
    message,
  };
}

export async function evaluateWaitForFunctionPredicate({ predicateSource, evalTimeoutMs }) {
  try {
    let value = (0, eval)(`(${String(predicateSource)})`);
    if (typeof value === "function") {
      value = value();
    }
    if (value && typeof value.then === "function") {
      const settled = await Promise.race([
        Promise.resolve(value).then((resolved) => ({ resolved })),
        new Promise((resolve) => setTimeout(() => resolve({ timedOut: true }), evalTimeoutMs)),
      ]);
      if (settled.timedOut) {
        return false;
      }
      value = settled.resolved;
    }
    return !!value;
  } catch {
    throw new Error("wait_for_function predicate failed");
  }
}

export async function waitForUserFunction(page, source, timeoutMs, opts = {}) {
  const now = opts.now || (() => Date.now());
  const pollIntervalMs = Math.max(1, opts.pollIntervalMs || 100);
  const waitForTimeout =
    opts.waitForTimeout ||
    ((ms) => {
      if (page && typeof page.waitForTimeout === "function") {
        return page.waitForTimeout(ms);
      }
      return new Promise((resolve) => setTimeout(resolve, ms));
    });
  const deadline = now() + timeoutMs;

  for (;;) {
    const remaining = deadline - now();
    if (remaining <= 0) {
      throw waitForFunctionTimeoutError();
    }

    try {
      const evaluatePromise = page.evaluate(evaluateWaitForFunctionPredicate, {
        predicateSource: String(source),
        evalTimeoutMs: Math.max(1, remaining),
      });
      evaluatePromise.catch(() => {});

      const ready = await withTimeout(evaluatePromise, remaining, waitForFunctionTimeoutError());
      if (ready) {
        return;
      }
    } catch (err) {
      if (err?.webcap) {
        throw err;
      }
      const message = String(err?.message || "");
      if (message.includes(failureMessage)) {
        throw waitForFunctionPredicateError();
      }
      throw err;
    }

    await waitForTimeout(Math.min(pollIntervalMs, Math.max(1, deadline - now())));
  }
}

export function waitForFunctionTimeoutError() {
  return captureError("timeout_error", "wait_ready", timeoutMessage, waitMetadata);
}

export function waitForFunctionPredicateError() {
  return captureError("capture_error", "wait_ready", failureMessage, waitMetadata);
}

async function withTimeout(promise, timeoutMs, timeoutErr) {
  let timer;
  const timeout = new Promise((_, reject) => {
    timer = setTimeout(() => reject(timeoutErr), Math.max(1, timeoutMs));
  });
  try {
    return await Promise.race([promise, timeout]);
  } finally {
    clearTimeout(timer);
  }
}
