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

# Measure test coverage of production packages
coverage:
  #!/usr/bin/env bash
  # -coverpkg points at internal/cmd so integration tests, which live in a
  # separate package tree, still count toward production-code numbers.
  # Generated files (detected via "DO NOT EDIT" / "Generated code" markers)
  # are stripped from the profile so the headline reflects testable code.
  # Summary is printed even if some packages fail (e.g. missing LSP).
  set -uo pipefail
  go test -coverpkg=./internal/...,./cmd/... -coverprofile=cover.out ./...
  rc=$?
  echo
  if [ -s cover.out ]; then
    gen=$(grep -rlE "^// Code generated|^// Generated code|DO NOT EDIT" \
      --include='*.go' internal cmd 2>/dev/null | paste -sd'|' -)
    if [ -n "$gen" ]; then
      grep -vE "($gen):" cover.out > cover.out.tmp && mv cover.out.tmp cover.out
      echo "Excluded generated files: $(echo "$gen" | tr '|' ' ')"
      echo
    fi
    go tool cover -func=cover.out | tail -1
    echo
    echo "Per-function breakdown: go tool cover -func=cover.out"
    echo "HTML report:            go tool cover -html=cover.out -o cover.html"
  fi
  exit $rc
