#!/usr/bin/env bash
# Idempotent Cloud Agent setup for double-entry-generator.
# Prepares the Go CLI toolchain (build, test, lint) and the MkDocs docs site.
set -euo pipefail

# Resolve repository root regardless of where the script is invoked from.
REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$REPO_ROOT"

echo "==> Downloading Go modules"
go mod download

echo "==> Building the double-entry-generator CLI"
make build

echo "==> Installing Go dev tools (ginkgo, goimports, golangci-lint)"
# Install into ~/.local/bin, which is on the default login-shell PATH, so the
# tools are reachable as bare commands (GOPATH/bin is not always on PATH).
export GOBIN="$HOME/.local/bin"
mkdir -p "$GOBIN"
# ginkgo is resolved from go.mod so it matches the pinned test framework version.
go install github.com/onsi/ginkgo/v2/ginkgo
go install golang.org/x/tools/cmd/goimports@latest
go install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.64.8

echo "==> Ensuring uv is available for the docs site"
if ! command -v uv >/dev/null 2>&1; then
  curl -LsSf https://astral.sh/uv/install.sh | sh
fi
# uv installs into ~/.local/bin, which is already on PATH for interactive shells.
export PATH="$HOME/.local/bin:$PATH"

echo "==> Syncing MkDocs documentation dependencies"
uv sync --project docs

echo "==> Setup complete"
