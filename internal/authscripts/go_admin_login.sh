#!/usr/bin/env bash
set -euo pipefail

if ! command -v curl >/dev/null 2>&1; then
  echo "missing required command: curl" >&2
  exit 127
fi

: "${WEBCAP_BASE_URL:?WEBCAP_BASE_URL is required}"
: "${WEBCAP_LOGIN_PATH:?WEBCAP_LOGIN_PATH is required}"
: "${WEBCAP_COOKIE_JAR:?WEBCAP_COOKIE_JAR is required}"
: "${WEBCAP_IDENTIFIER:?WEBCAP_IDENTIFIER is required}"
: "${WEBCAP_PASSWORD:?WEBCAP_PASSWORD is required}"

login_url="${WEBCAP_BASE_URL%/}${WEBCAP_LOGIN_PATH}"
tmp_dir="${TMPDIR:-/tmp}/webcap-auth-$$"
mkdir -p "$tmp_dir"
trap 'rm -rf "$tmp_dir"' EXIT

headers="$tmp_dir/headers.txt"
body="$tmp_dir/body.html"
post_headers="$tmp_dir/post-headers.txt"
post_body="$tmp_dir/post-body.html"

curl -fsS -D "$headers" -o "$body" -b "$WEBCAP_COOKIE_JAR" -c "$WEBCAP_COOKIE_JAR" "$login_url"

csrf="$(
  awk 'tolower($0) ~ /^x-csrf-token:/ {sub(/^[^:]+:[[:space:]]*/, ""); gsub(/\r$/, ""); print; exit}' "$headers"
)"
if [ -z "$csrf" ]; then
  csrf="$(
    {
      sed -n "s/.*name=['\"]_token['\"][^>]*value=['\"]\\([^'\"]*\\)['\"].*/\\1/p" "$body"
      sed -n "s/.*value=['\"]\\([^'\"]*\\)['\"][^>]*name=['\"]_token['\"].*/\\1/p" "$body"
    } | head -n 1
  )"
fi
if [ -z "$csrf" ]; then
  echo "missing CSRF token on login page" >&2
  exit 2
fi

final_url="$(
  curl -fsS -L -D "$post_headers" -o "$post_body" -w '%{url_effective}' \
    -b "$WEBCAP_COOKIE_JAR" -c "$WEBCAP_COOKIE_JAR" \
    --data-urlencode "identifier=$WEBCAP_IDENTIFIER" \
    --data-urlencode "password=$WEBCAP_PASSWORD" \
    --data-urlencode "_token=$csrf" \
    "$login_url"
)"

case "$final_url" in
  *"$WEBCAP_LOGIN_PATH"*)
    echo "login ended on login route; credentials or CSRF token may be invalid" >&2
    exit 3
    ;;
esac

echo "login request completed"
