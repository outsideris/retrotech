#!/usr/bin/env bash
#
# Cloudflare Pages build command wrapper.
# Runs the test suite as a gate, then the build, then notifies Chime (the
# notification service that fans out to Telegram and Buzz) of the result. A
# failing test OR build exits non-zero, so Cloudflare marks the deployment
# failed and the bad version is never published (the previous deploy stays
# live).
#
# Cloudflare Pages setup:
#   - Build command:          bash scripts/cf-build.sh
#   - Build output directory: dist
#   - Go is auto-detected from go.mod (no GO_VERSION needed; set it only if a
#     build fails on the Go version).
#   - Environment variables (encrypted), both required for the notification:
#       DEPLOY_WEBHOOK_URL    terrarium's public inbox address for Chime:
#                             https://terrarium-inbox.outsideris.workers.dev/in/chime
#       DEPLOY_WEBHOOK_TOKEN  that inbox's secret (`terrarium inbox url chime`),
#                             sent as `Authorization: Bearer`. It never goes in
#                             the URL.
#   - Set ANALYTICS_ID (GA4 id) for production builds to ship analytics.
#
# The token stays in the Pages env var (a secret), never in the repo.
set -uo pipefail

# Gate: tests must pass before building/deploying. `phase` lets the
# notification say which step failed.
phase="test"
go test ./...
code=$?

if [ "$code" -eq 0 ]; then
  phase="build"
  go run ./cmd/build
  code=$?
fi

# Escapes a value for a JSON string. Branch names and commit subjects may
# carry quotes and backslashes.
json_escape() {
  printf '%s' "$1" | tr -d '\001-\037' | sed -e 's/\\/\\\\/g' -e 's/"/\\"/g'
}

# Cloudflare Pages build-time variables (empty locally). Chime ignores empty
# fields, so they can always be included.
branch="${CF_PAGES_BRANCH:-}"
commit="${CF_PAGES_COMMIT_SHA:-}"
subject="$(git log -1 --pretty=%s 2>/dev/null || true)"

case "$branch" in
  "") environment="" ;;
  main) environment="production" ;;
  *) environment="preview" ;;
esac

if [ "$code" -eq 0 ]; then
  status="success"
  level="info"
  message=""
  url="${CF_PAGES_URL:-}"
else
  # The previous deploy stays live, so a failed deploy is a warning, not an
  # outage. The failed deployment's URL serves nothing, so it is left out.
  status="failed"
  level="warn"
  message="${phase} failed (exit ${code})"
  url=""
fi

if [ -n "${DEPLOY_WEBHOOK_URL:-}" ] && [ -n "${DEPLOY_WEBHOOK_TOKEN:-}" ]; then
  # Chime's notification fields (top-level keys); it adds the status emoji and
  # the "Deploy succeeded/failed" title itself. The inbox answers once the
  # event is queued, and delivery to the channels happens later.
  payload=$(printf '{"project":"retrotech","status":"%s","level":"%s","environment":"%s","branch":"%s","commitSha":"%s","commitMessage":"%s","url":"%s","message":"%s"}' \
    "$status" "$level" "$environment" "$(json_escape "$branch")" "$(json_escape "$commit")" \
    "$(json_escape "$subject")" "$(json_escape "$url")" "$message")
  # A webhook hiccup must not change the deploy result, so curl errors are
  # ignored.
  curl -fsS --max-time 15 -X POST "$DEPLOY_WEBHOOK_URL" \
    -H "Authorization: Bearer ${DEPLOY_WEBHOOK_TOKEN}" \
    -H 'content-type: application/json' \
    -d "$payload" \
    || echo "warn: deploy webhook POST failed (ignored)"
else
  echo "warn: DEPLOY_WEBHOOK_URL or DEPLOY_WEBHOOK_TOKEN not set; skipping deploy notification"
fi

exit "$code"
