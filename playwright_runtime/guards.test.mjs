import assert from "node:assert/strict";
import test from "node:test";

import { verifyURLGuards } from "./guards.mjs";

test("verifyURLGuards records passing forbidden and expected outcomes", () => {
  const outcomes = verifyURLGuards(
    { expect_url: "/admin", fail_on_url: ["/login"] },
    "http://localhost:3000/admin"
  );

  assert.deepEqual(outcomes, [
    {
      kind: "fail_on_url",
      value: "/login",
      final_url: "http://localhost:3000/admin",
      matched: false,
      status: "passed",
    },
    {
      kind: "expect_url",
      value: "/admin",
      final_url: "http://localhost:3000/admin",
      matched: true,
      status: "passed",
    },
  ]);
});

test("verifyURLGuards prioritizes forbidden URL over expected URL miss", () => {
  assert.throws(
    () =>
      verifyURLGuards(
        { expect_url: "/admin/translations/queue", fail_on_url: ["/admin/login"] },
        "http://localhost:3000/admin/login"
      ),
    (err) => {
      assert.equal(err.webcap?.operation, "verify_url_guard");
      assert.equal(err.webcap?.metadata?.guard, "fail_on_url");
      assert.equal(err.webcap?.metadata?.fail_on_url, "/admin/login");
      assert.equal(err.webcap?.metadata?.expect_url, undefined);
      return true;
    }
  );
});

test("verifyURLGuards reports expected URL failures after forbidden checks pass", () => {
  assert.throws(
    () => verifyURLGuards({ expect_url: "/admin", fail_on_url: ["/login"] }, "http://localhost:3000/settings"),
    (err) => {
      assert.equal(err.webcap?.operation, "verify_url_guard");
      assert.equal(err.webcap?.metadata?.guard, "expect_url");
      assert.equal(err.webcap?.metadata?.expect_url, "/admin");
      return true;
    }
  );
});
