# Help
help:
  just -l

# Build
build:
  go build -o mcp-language-server

# Install locally
install:
  go install

# Format code
fmt:
  gofmt -w .

# Generate LSP types and methods
generate:
  go run ./cmd/generate

# Run code audit checks
check:
  #!/usr/bin/env bash
  # gopls check exits 0 even when it reports diagnostics; fail the recipe
  # ourselves below if any output is produced. 2>&1 covers gopls versions
  # that write findings to stderr.
  set -euo pipefail
  gofmt -l .
  test -z "$(gofmt -l .)"
  go tool staticcheck ./...
  go tool errcheck ./...
  out="$(find . -path './integrationtests/workspaces' -prune -o \
    -path './integrationtests/test-output' -prune -o \
    -name '*.go' -print | xargs gopls check 2>&1)"
  if [ -n "$out" ]; then
    printf '%s\n' "$out"
    exit 1
  fi
  go tool govulncheck ./...

# Run tests
test:
  go test ./...

# Update snapshot tests
snapshot:
  UPDATE_SNAPSHOTS=true go test ./integrationtests/...
