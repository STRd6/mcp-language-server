# Changelog

All notable changes to this fork are documented here. Format loosely
follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/); this
project adheres to [SemVer](https://semver.org/spec/v2.0.0.html).

## [v0.4.5] – 2026-06-02

### Fixed
- Stale diagnostics/edits for already-open documents. The MCP never sent
  `textDocument/didChange` for files it touched, so once a doc was opened
  the server kept analyzing its original `didOpen` content and ignored
  on-disk edits (`didChangeWatchedFiles` does not update open docs). Both
  `get_diagnostics` and `edit_file` now sync the server's overlay:
  `GetDiagnosticsForFile` pushes the current disk content before querying
  (via the new `Client.NotifyChangeIfChanged`, which sends a `didChange`
  only when the content actually differs from what the server was last
  told — a no-op resend can make some servers drop cross-file
  diagnostics), and `ApplyTextEdits` notifies the server after writing.
  For push-only servers a real sync now waits for the next publish rather
  than reading the stale cache.

## [v0.4.4] – 2026-05-30

### Fixed
- `get_diagnostics`: for pull-capable servers, `GetDiagnosticsForFile`
  issued the `textDocument/diagnostic` request but discarded the
  response and read the push cache, which can lag the edit that
  triggered the call. The pull report's items (computed at request
  time, so authoritative and race-free) are now stored via the new
  `Client.SetFileDiagnostics` and preferred over the cache. No-op for
  push-only servers.

## [v0.4.3] – 2026-05-18

### Added
- Eight new capability-gated MCP tools wrapping LSP handlers that the
  Civet LSP gained in [Civet PR #2069](https://github.com/DanielXMoore/Civet/pull/2069),
  so an agent driving the MCP can verify those handlers end-to-end
  without a real editor:
  - `signature_help` — parameter tooltips at a call site. Accepts
    optional `triggerCharacter` / `triggerKind` / `isRetrigger` inputs
    so the SignatureHelpContext branches are reachable.
  - `type_definition` — `textDocument/typeDefinition` for the symbol
    at a position; renders the same way as `definition_at_position`.
  - `implementation` — `textDocument/implementation` for the symbol
    at a position.
  - `document_highlight` — in-file highlight ranges grouped by kind
    (Write for declarations, Read for accesses, Text for plain refs).
  - `prepare_rename` — probes whether a rename is allowed at a
    position and reports the range/placeholder. Standalone for LSP
    verification — `rename_symbol` still calls `textDocument/rename`
    directly without chaining. Gated on the `prepareProvider`
    sub-capability inside `RenameOptions`.
  - `folding_range` — foldable ranges across a file (relevant for
    Civet's whitespace-significant blocks).
  - `selection_range` — smart-expand chain at a position, flattened
    outermost-first.
  - `linked_editing_range` — paired edit ranges (e.g. JSX open/close
    tag matching); surfaces `null` explicitly rather than as an error.
- Capability helpers: `HasSignatureHelpSupport`,
  `HasTypeDefinitionSupport`, `HasImplementationSupport`,
  `HasDocumentHighlightSupport`, `HasFoldingRangeSupport`,
  `HasSelectionRangeSupport`, `HasLinkedEditingRangeSupport`, and
  `HasPrepareRenameSupport` (introspects `RenameOptions.PrepareProvider`).

### Fixed
- `Tuple_ParameterInformation_label_Item1` now decodes from the LSP
  wire `[start, end]` JSON array shape. The generator emitted a struct
  with `Fld0`/`Fld1` JSON keys, so signature help responses that used
  the offset form of `ParameterInformation.label` failed with
  `unmarshal failed to match one of [Tuple_ParameterInformation_label_Item1 string]`.

## [v0.4.2] – 2026-05-17

### Added
- `definition_at_position` and `references_at_position` — positional
  variants of the existing name-based tools, mirroring `hover`'s shape
  (`filePath`, `line`, `column`). They call `textDocument/definition`
  and `textDocument/references` directly instead of going through
  `workspace/symbol`. Fixes three problems with the name-based path:
  no call-site disambiguation when two unrelated symbols share a name,
  build-output (e.g. `dist/`) showing up as extra results because the
  LSP indexes it, and `references` duplicating its output once per
  `workspace/symbol` hit when one logical symbol has multiple matches.
  The original name-based `definition` / `references` tools are kept
  for the case where the caller only has a name.

## [v0.4.1] – 2026-05-17

### Performance
- `diagnostics` and `references` now cache `textDocument/documentSymbol`
  per URI inside `GetLineRangesToDisplay`. Previously the loop called
  `GetFullDefinition` once per location, and every call did its own
  `documentSymbol` RPC — so N identical round-trips serialized through
  the LSP's request queue. With civet-lsp on Civet's `parser.hera`
  emitting 1.6k–17k cascading TS diagnostics, the tool call hung past
  95 s; the fix brings it down to ~16 s end-to-end (≈1 LSP RPC instead
  of 17k). Container-search semantics preserved (outer container wins,
  matching the original `searchSymbols` behavior).

## [v0.4.0] – 2026-05-15

mcp-go bumped 10 versions past dependabot's target, picking up native
`logging/setLevel` handling along the way. Closes upstream issue #79.

### Added
- `logging/setLevel` is now implemented. The server already advertised
  the logging capability via `server.WithLogging()`, but mcp-go pre-v0.39
  didn't have a handler for the method, so clients saw method-not-found.
  v0.54.0 ships `handleSetLevel` plus `SessionWithLogging`; we get it
  for free. Verified end-to-end: valid levels return `{}`, invalid
  levels return `-32602 invalid params`.
- `--version` CLI flag (and `-version`). Replaces the stale hardcoded
  `"v0.0.2"` previously baked into the MCP server registration with a
  single `version` constant used in both places.

### Changed
- `edit_file` reports insert-as-replace as `Inserted N lines` instead of
  the literally-true-but-misleading `N lines removed, M lines added`.
  Detection: every edit in the batch must preserve the original lines
  verbatim as a prefix (insert-after) or suffix (insert-before) of
  `newText`. Mixed batches keep the existing message.
- `parseConfig` is called before the "starting" log line so `--version`
  output isn't polluted by it.

### Dependencies
- `github.com/mark3labs/mcp-go` v0.28.0 → v0.54.0. Breaking change in
  v0.39: `CallToolParams.Arguments` is now `any` (was `map[string]any`)
  with typed accessors (`RequireString`/`RequireInt`/`GetBool`/etc.).
  Every tool handler in `tools.go` migrated; -120 lines of manual
  type-switching gone. The `contextLines` bool back-compat (from
  v0.3.1) is preserved via a raw type check, since `GetInt` doesn't
  accept bool.

## [v0.3.1] – 2026-05-15

Bug-fix patch on top of v0.3.0.

### Fixed
- `GetFullDefinition` no longer panics with "index out of range" when an
  LSP returns a symbol position past EOF. The template/attribute
  scan-back loop now bounds-checks against the file's line count.
  Triggered in the wild by civet-lsp on `.hera` files when source-map
  remap escaped the file — one bad position took down the whole
  diagnostics response. Matching guards added to
  `GetLineRangesToDisplay` so the contract is obvious at the call site.
- `diagnostics` tool's `contextLines` parameter is now functional. The
  schema declared it as a boolean but the handler read it as int, so it
  was effectively inert and only `LSP_CONTEXT_LINES` could change the
  count. Switched the schema to a number with default 5; bool values
  are still accepted for back-compat (`true` → 5, `false` → 0).
- `code_actions` tool no longer wraps LSP errors in an extra
  `failed to get code actions:` prefix; the underlying message comes
  through unmodified.

### Added
- `--help` now prints a sample `.mcp.json` snippet so first-time users
  can drop a working config into their project without leaving the CLI.

### Internal
- `printUsage` writes are no longer flagged by errcheck.

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
