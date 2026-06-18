#!/usr/bin/env bash
#
# Cloudflare Pages build command wrapper.
# Runs the test suite as a gate, then the build, then notifies the Telegram
# webhook worker of the result. A failing test OR build exits non-zero, so
# Cloudflare marks the deployment failed and the bad version is never
# published (the previous deploy stays live).
#
# Cloudflare Pages setup:
#   - Build command:          bash scripts/cf-build.sh
#   - Build output directory: dist
#   - Go is auto-detected from go.mod (no GO_VERSION needed; set it only if a
#     build fails on the Go version).
#   - Environment variable (encrypted): DEPLOY_WEBHOOK_URL
#       Use the worker's GENERIC endpoint (it reads top-level status/project/...):
#       e.g. https://cf-webhook.outsideris.workers.dev/webhook/generic?token=<SECRET_TOKEN>
#       (the /webhook/cloudflare endpoint is for Cloudflare's NATIVE notification
#        payload, not this custom one.)
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

# Cloudflare Pages build-time variables (empty locally). The worker ignores
# empty fields, so they can always be included.
branch="${CF_PAGES_BRANCH:-}"
commit="${CF_PAGES_COMMIT_SHA:-}"
url="${CF_PAGES_URL:-}"

if [ "$code" -eq 0 ]; then
  status="success"
  message=""
else
  status="failure"
  message="${phase} failed (exit ${code})"
fi

if [ -n "${DEPLOY_WEBHOOK_URL:-}" ]; then
  # Payload matches the worker's /webhook/generic adapter (top-level keys); it
  # adds the status emoji/label itself. A webhook hiccup must not change the
  # deploy result, so curl errors are ignored.
  curl -fsS -X POST "$DEPLOY_WEBHOOK_URL" \
    -H 'content-type: application/json' \
    -d "{\"status\":\"${status}\",\"project\":\"retrotech\",\"branch\":\"${branch}\",\"commitSha\":\"${commit}\",\"url\":\"${url}\",\"message\":\"${message}\"}" \
    || echo "warn: deploy webhook POST failed (ignored)"
else
  echo "warn: DEPLOY_WEBHOOK_URL not set; skipping deploy notification"
fi

exit "$code"
