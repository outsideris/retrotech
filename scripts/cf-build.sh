#!/usr/bin/env bash
#
# Cloudflare Pages build command wrapper.
# Runs the normal build, then notifies the Telegram webhook worker of the
# result (success/failure), and exits with the build's own status so Pages
# still marks the deployment correctly.
#
# Cloudflare Pages setup:
#   - Build command:          bash scripts/cf-build.sh
#   - Build output directory: dist
#   - Environment variable (encrypted): DEPLOY_WEBHOOK_URL
#       e.g. https://cf-webhook.outsideris.workers.dev/webhook/cloudflare?token=<SECRET_TOKEN>
#
# The token stays in the Pages env var (a secret), never in the repo.
set -uo pipefail

npm run build
code=$?

branch="${CF_PAGES_BRANCH:-unknown}"
commit="${CF_PAGES_COMMIT_SHA:-}"
short="${commit:0:7}"

if [ "$code" -eq 0 ]; then
  status="success"
  message="✅ RetroTech 배포 성공 (${branch} ${short})"
else
  status="failure"
  message="❌ RetroTech 배포 실패 (${branch} ${short}, build exit ${code})"
fi

if [ -n "${DEPLOY_WEBHOOK_URL:-}" ]; then
  # A webhook hiccup must not change the deploy result, so ignore curl errors.
  curl -fsS -X POST "$DEPLOY_WEBHOOK_URL" \
    -H 'content-type: application/json' \
    -d "{\"status\":\"${status}\",\"project\":\"retrotech\",\"environment\":\"${branch}\",\"message\":\"${message}\"}" \
    || echo "warn: deploy webhook POST failed (ignored)"
else
  echo "warn: DEPLOY_WEBHOOK_URL not set; skipping deploy notification"
fi

exit "$code"
