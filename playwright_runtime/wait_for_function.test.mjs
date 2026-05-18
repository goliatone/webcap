import assert from "node:assert/strict";
import test from "node:test";

import {
  errorEnvelopeFrom,
  evaluateWaitForFunctionPredicate,
  waitForUserFunction,
} from "./wait_for_function.mjs";

test("evaluateWaitForFunctionPredicate supports expression and callable forms", async () => {
  globalThis.__webcapReady = true;

  assert.equal(
    await evaluateWaitForFunctionPredicate({
      predicateSource: "globalThis.__webcapReady === true",
      evalTimeoutMs: 20,
    }),
    true
  );
  assert.equal(
    await evaluateWaitForFunctionPredicate({
      predicateSource: "() => globalThis.__webcapReady === true",
      evalTimeoutMs: 20,
    }),
    true
  );
  assert.equal(
    await evaluateWaitForFunctionPredicate({
      predicateSource: "async () => globalThis.__webcapReady === true",
      evalTimeoutMs: 20,
    }),
    true
  );
});

test("evaluateWaitForFunctionPredicate treats pending promises as not ready", async () => {
  assert.equal(
    await evaluateWaitForFunctionPredicate({
      predicateSource: "() => new Promise(() => {})",
      evalTimeoutMs: 5,
    }),
    false
  );
});

test("evaluateWaitForFunctionPredicate redacts thrown predicate details", async () => {
  await assert.rejects(
    evaluateWaitForFunctionPredicate({
      predicateSource: '() => { throw new Error("distinctive predicate source") }',
      evalTimeoutMs: 20,
    }),
    (err) => {
      assert.equal(err.message, "wait_for_function predicate failed");
      assert.equal(err.message.includes("distinctive predicate source"), false);
      return true;
    }
  );
});

test("waitForUserFunction polls until predicate becomes truthy", async () => {
  globalThis.__webcapReady = false;
  let waits = 0;
  const page = {
    evaluate: (fn, args) => fn(args),
    waitForTimeout: async () => {
      waits += 1;
      globalThis.__webcapReady = true;
    },
  };

  await waitForUserFunction(page, "globalThis.__webcapReady === true", 100, { pollIntervalMs: 1 });
  assert.equal(waits, 1);
});

test("waitForUserFunction times out when page evaluation never settles", async () => {
  const page = {
    evaluate: () => new Promise(() => {}),
    waitForTimeout: async () => {},
  };

  await assert.rejects(waitForUserFunction(page, "() => true", 5, { pollIntervalMs: 1 }), (err) => {
    assert.equal(err.webcap?.code, "timeout_error");
    assert.equal(err.webcap?.operation, "wait_ready");
    assert.deepEqual(err.webcap?.metadata, { wait: "wait_for_function" });
    return true;
  });
});

test("errorEnvelopeFrom preserves structured wait_for_function errors", () => {
  const err = new Error("wait_for_function predicate failed");
  err.webcap = {
    code: "capture_error",
    operation: "wait_ready",
    message: "wait_for_function predicate failed",
    metadata: { wait: "wait_for_function" },
  };

  assert.deepEqual(errorEnvelopeFrom(err), err.webcap);
});

test("errorEnvelopeFrom keeps unrelated runtime errors generic", () => {
  assert.deepEqual(errorEnvelopeFrom(new Error("wait_for_function predicate failed from hook")), {
    code: "capture_error",
    operation: "playwright_capture",
    message: "wait_for_function predicate failed from hook",
  });
});
