import assert from "node:assert/strict";
import test from "node:test";

import { playwrightCookies } from "./auth.mjs";

test("playwrightCookies translates host-only cookies to URL cookies", () => {
  const cookies = playwrightCookies(
    [
      {
        name: "session",
        value: "cookie-secret",
        domain: "example.test",
        path: "/admin",
        host_only: true,
      },
    ],
    "https://example.test/admin/dashboard"
  );

  assert.equal(cookies.length, 1);
  assert.equal(cookies[0].domain, undefined);
  assert.equal(cookies[0].path, undefined);
  assert.equal(cookies[0].url, "https://example.test/admin");
});

test("playwrightCookies preserves domain cookies as domain cookies", () => {
  const cookies = playwrightCookies(
    [
      {
        name: "session",
        value: "cookie-secret",
        domain: ".example.test",
        path: "/admin",
      },
    ],
    "https://example.test/admin/dashboard"
  );

  assert.equal(cookies.length, 1);
  assert.equal(cookies[0].domain, ".example.test");
  assert.equal(cookies[0].url, undefined);
});
