# Changelog

All notable changes to this fork are documented here. Format loosely
follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/); this
project adheres to [SemVer](https://semver.org/spec/v2.0.0.html).

## [v0.3.0] – 2026-05-15

Focused on bridge↔LSP↔harness reliability: cold-start UX, long-running
session hangs, a cross-instance race, and orphaned-process cleanup.

### Fixed
- Bridge now exits on stdin EOF instead of waiting forever. Previously
  shell pipelines like `(echo init; echo list) | bridge | head -3` left
  the bridge alive after the upstream subshell exited, holding the
  parent shell's `eval` blocked on its child.
- `Client` now owns its file-watch handler; the package-level
  `fileWatchHandler` global raced whenever multiple Clients lived in
  the same process (every integration test). Caught by `go test -race`.
- `bufio.Scanner` on LSP stderr replaced with `bufio.Reader.ReadString`:
  Scanner's 64 KB `MaxScanTokenSize` silently killed the scanner goroutine
  on the first oversize line, after which stderr stopped draining and
  the LSP eventually blocked on its next write — manifesting as "bridge
  + LSP both alive, idle, sleeping on epoll/futex" after long sessions.
- Server-initiated request handlers now dispatch in goroutines; the old
  synchronous version blocked all further LSP traffic while a handler
  ran, and the response write going back into LSP stdin from inside the
  read loop risked a hard deadlock if LSP's stdin pipe filled.
- Added `Client.stdinMu` to serialize header+body writes — the LSP frame
  is two `Write` calls and previously had no concurrency control.
- Pull-mode `textDocument/diagnostic` requests are now capability-gated
  on `DiagnosticProvider`. Push-only LSPs (civet-lsp) used to receive a
  doomed request that rode their busy queue, blocking many seconds
  behind the work producing the `publishDiagnostics` we already had.
- `WaitForDiagnostics` push-mode timeout raised 3 s → 30 s and returns
  a bool so callers distinguish "server said zero diagnostics" from
  "server hasn't reported yet". The old 3 s was a magic number that
  only worked because pull-mode was accidentally extending the wait.

### Changed
- `WaitForServerReady` is now a no-op (was `time.Sleep(1 * time.Second)`
  marked TODO). Per LSP spec the server is ready after `initialize` +
  `initialized`; servers needing post-init project loading should
  signal via `$/progress`.
- Default LSP init is synchronous so every capability-gated tool
  appears in the first `tools/list` response. The async path remains
  available via `--lsp-init-async` for clients that honor
  `tools/list_changed`.
- `references` tool gates on `workspace/symbol` support in addition to
  `textDocument/references`, matching the implementation that calls
  `workspace/symbol` first.
- Diagnostics tool returns a clearer message when the wait times out
  without a publish: "the language server may still be performing its
  initial analysis. Try again in a few seconds."

### Added
- `HasPullDiagnosticsSupport` capability helper.
- `Client.ServerCapabilities()` accessor so tools can gate behavior on
  what the server advertised without threading capabilities through
  every call site.
- `Client.SetFileWatchHandler` (per-Client; replaces the racy global).
- `just coverage` recipe with `-coverpkg` covering integration tests
  and generated-file filtering on the summary.
- Unit tests for capability helpers, `DetectLanguageID`, `MessageID`,
  server-request handlers, and `Glob.String()` round-trips.
- Civet setup section in the README and documentation for
  `--lsp-init-async`.

### Performance
- Cold-call latency against the Civet repo: ~10.7 s → ~9.4 s, and the
  result is now correct by design rather than by accident.

## [v0.2.0] – 2026-05-13

Fork-establishing release.

### Added
- `--config` flag for per-LSP `initializationOptions` (JSON file keyed
  by LSP binary basename).
- `--idle-timeout` flag to shut down after a period of no MCP traffic.
- `--disable-tools` flag to suppress individual tools at startup.
- Civet-lsp added as a CI integration-test target alongside the
  existing Go / Python / Rust / TypeScript / clangd matrix; 5 separate
  jobs collapsed into one matrix.
- LSP `$/progress` tracking with `WaitForProgress` and
  `WaitForNextDiagnostics` helpers; integration test waits replaced
  with these instead of sleeps.
- Detection of `.hera` files as `hera` (for civet-lsp) and `.v` files
  as Coq.
- Tool annotations (title + ReadOnly/Destructive hints).
- Document symbols enriched, including C++ template prefixes.
- `justfile` `check` recipe fails on gopls diagnostics; `gofmt`,
  `staticcheck`, `errcheck`, `govulncheck` all wired up.

### Changed
- Module path renamed to `github.com/STRd6/mcp-language-server` (fork
  fork-of-the-fork).
- LSP handshake hardened: server-request handlers registered before
  `initialize`, single `initialized` notification, lazy LSP-specific
  init.
- Cleanup paths guarded with `sync.Once`.
- Watcher switched to LSP 3.17-compliant glob matcher (replacing the
  pre-existing string-prefix matcher), stopped pre-opening every
  matching file on `registerCapability`.
- File URIs built via `URIFromPath` instead of string concatenation.
- `clangd` integration tests fixed by rewriting `compile_commands.json`
  to avoid a dual-URI bug; rust struct-type hover snapshot refreshed.

### Fixed
- Latent panic in watcher relative-pattern matching.
- Python definition test flake; clangd hover and references tests
  re-enabled.
- CI regressions from upstream PR #82 / PR #127 adaptation.
