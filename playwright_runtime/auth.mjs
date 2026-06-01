export function playwrightCookies(cookies, targetURL) {
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
      const hostOnly = cookie?.host_only === true || cookie?.hostOnly === true || cookie?.HostOnly === true;
      if (domain && !hostOnly) {
        out.domain = domain;
      } else {
        out.url = cookieURLForTarget(targetURL, domain, out.path);
        delete out.path;
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
