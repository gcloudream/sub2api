#!/usr/bin/env bash
# Run on a build workstation, never on the small production server.
set -euo pipefail
root=$(cd "$(dirname "$0")/../.." && pwd)
version=${1:?Usage: build-release.sh VERSION OUTPUT_DIRECTORY}
output=${2:?Provide an output directory outside the source checkout}
mkdir -p "$output"
output=$(cd "$output" && pwd)
case "$output/" in "$root/"*) echo 'Output must be outside the checkout' >&2; exit 1;; esac
cd "$root"
test -z "$(git status --porcelain)" || { echo 'Commit source changes before building' >&2; exit 1; }
git rev-parse "v${version}^{commit}" >/dev/null
git diff --exit-code "v${version}" HEAD -- backend/
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
    src/stores/__tests__/auth.spec.ts --maxWorkers=2 --minWorkers=1
  pnpm run build
) 2>&1 | tee "$output/frontend.log"
(
  cd backend
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
  printf 'version=%s\ncommit=%s\nbuild_date=%s\n' "$version" "$commit" "$build_date"
  node --version
  pnpm --version
  (cd backend && go version)
} > "$output/manifest.txt"
(cd "$output" && shasum -a 256 sub2api source.tar.gz Dockerfile docker-entrypoint.sh > SHA256SUMS)
echo "Release artifacts: $output"
