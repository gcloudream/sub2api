#!/usr/bin/env bash
# Run on a build workstation, never on the small production server.
set -euo pipefail
root=$(cd "$(dirname "$0")/../.." && pwd)
version=${1:?Usage: build-release.sh VERSION OUTPUT_DIRECTORY}
output=${2:?Provide an output directory outside the source checkout}
mkdir -p "$output"
output=$(cd "$output" && pwd)
test -z "$(ls -A "$output")" || { echo "Use a new, empty output directory" >&2; exit 1; }
case "$output/" in "$root/"*) echo 'Output must be outside the checkout' >&2; exit 1;; esac
cd "$root"
test -z "$(git status --porcelain)" || { echo 'Commit source changes before building' >&2; exit 1; }
git rev-parse "v${version}^{commit}" >/dev/null
git merge-base --is-ancestor "v${version}" HEAD || {
  echo 'Merge the requested official release into the Fantasy branch before building' >&2
  exit 1
}
upstream_commit=$(git rev-parse "v${version}^{commit}")
commit=$(git rev-parse HEAD)
build_date=$(date -u +%Y-%m-%dT%H:%M:%SZ)
export NODE_OPTIONS=${NODE_OPTIONS:---max-old-space-size=4096}
export GOMAXPROCS=${GOMAXPROCS:-2}
(
  cd frontend
  pnpm install --frozen-lockfile
  pnpm exec vitest run src/views/__tests__/HomeView.compact.spec.ts \
    src/views/auth/__tests__/LoginView.spec.ts src/views/auth/__tests__/RegisterView.spec.ts \
    src/components/charts/__tests__ src/stores/__tests__/app.spec.ts \
    src/stores/__tests__/auth.spec.ts src/components/payment/__tests__ \
    src/views/user/__tests__/PaymentView.spec.ts src/views/user/__tests__/PaymentResultView.spec.ts \
    src/views/__tests__/UsageDocsView.spec.ts src/utils/__tests__/usageDocs.spec.ts \
    src/utils/__tests__/rechargeBonus.spec.ts src/utils/__tests__/decimalAmount.spec.ts \
    src/components/layout/__tests__/AppSidebar.spec.ts src/views/user/__tests__/GroupAccountsView.spec.ts \
    --maxWorkers=2 --minWorkers=1
  pnpm run build
) 2>&1 | tee "$output/frontend.log"
(
  cd backend
  go test -p 2 -tags=unit ./internal/service ./internal/handler ./internal/handler/admin \
    ./internal/payment/... -run 'Payment|Refund|Recharge|BlindBox|EasyPay|Alipay|UserGroupAccount'
  CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -p 2 -tags embed \
    -ldflags="-s -w -X main.Version=$version -X main.Commit=$commit -X main.Date=$build_date -X main.BuildType=release" \
    -trimpath -o "$output/sub2api" ./cmd/server
) 2>&1 | tee "$output/backend.log"
mkdir -p "$output/resources"
cp -R backend/resources/. "$output/resources/"
cp deploy/docker-entrypoint.sh "$output/docker-entrypoint.sh"
cp tools/fantasy/Dockerfile.artifact "$output/Dockerfile"
git archive --format=tar.gz -o "$output/source.tar.gz" HEAD
{
  printf 'version=%s\nupstream_commit=%s\ncommit=%s\nbuild_date=%s\n' "$version" "$upstream_commit" "$commit" "$build_date"
  node --version
  pnpm --version
  (cd backend && go version)
} > "$output/manifest.txt"
(cd "$output" && shasum -a 256 sub2api source.tar.gz Dockerfile docker-entrypoint.sh > SHA256SUMS)
echo "Release artifacts: $output"
