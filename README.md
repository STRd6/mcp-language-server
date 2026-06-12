# MCP Language Server

[![CI](https://github.com/STRd6/mcp-language-server/actions/workflows/go.yml/badge.svg?branch=main)](https://github.com/STRd6/mcp-language-server/actions/workflows/go.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/STRd6/mcp-language-server)](https://goreportcard.com/report/github.com/STRd6/mcp-language-server)
[![Go Version](https://img.shields.io/github/go-mod/go-version/STRd6/mcp-language-server)](https://github.com/STRd6/mcp-language-server/blob/main/go.mod)
[![GoDoc](https://pkg.go.dev/badge/github.com/STRd6/mcp-language-server)](https://pkg.go.dev/github.com/STRd6/mcp-language-server)

This is an [MCP](https://modelcontextprotocol.io/introduction) server that runs and exposes a [language server](https://microsoft.github.io/language-server-protocol/) to LLMs. Not a language server for MCP, whatever that would be.

> This is a downstream fork of [isaacphi/mcp-language-server](https://github.com/isaacphi/mcp-language-server) with a few extra tools, flags, and stability fixes. See [Changes from upstream](#changes-from-upstream) below.

## Demo

`mcp-language-server` helps MCP enabled clients navigate codebases more easily by giving them access semantic tools like get definition, references, rename, and diagnostics.

![Demo](demo.gif)

## Setup

1. **Install Go**: Follow instructions at <https://golang.org/doc/install>
2. **Install this server**: `go install github.com/STRd6/mcp-language-server@latest`

   Or, to use upstream's last release (no longer actively maintained): `go install github.com/isaacphi/mcp-language-server@latest`.
3. **Install a language server**: _follow one of the guides below_
4. **Configure your MCP client**: _follow one of the guides below_

<details>
  <summary>Go (gopls)</summary>
  <div>
    <p><strong>Install gopls</strong>: <code>go install golang.org/x/tools/gopls@latest</code></p>
    <p><strong>Configure your MCP client</strong>: This will be different but similar for each client. For Claude Desktop, add the following to <code>~/Library/Application\ Support/Claude/claude_desktop_config.json</code></p>

<pre>
{
  "mcpServers": {
    "language-server": {
      "command": "mcp-language-server",
      "args": ["--workspace", "/Users/you/dev/yourproject/", "--lsp", "gopls"],
      "env": {
        "PATH": "/opt/homebrew/bin:/Users/you/go/bin",
        "GOPATH": "/users/you/go",
        "GOCACHE": "/users/you/Library/Caches/go-build",
        "GOMODCACHE": "/Users/you/go/pkg/mod"
      }
    }
  }
}
</pre>

<p><strong>Note</strong>: Not all clients will need these environment variables. For Claude Desktop you will need to update the environment variables above based on your machine and username:</p>
<ul>
  <li><code>PATH</code> needs to contain the path to <code>go</code> and to <code>gopls</code>. Get this with <code>echo $(which go):$(which gopls)</code></li>
  <li><code>GOPATH</code>, <code>GOCACHE</code>, and <code>GOMODCACHE</code> may be different on your machine. These are the defaults.</li>
</ul>

  </div>
</details>
<details>
  <summary>Rust (rust-analyzer)</summary>
  <div>
    <p><strong>Install rust-analyzer</strong>: <code>rustup component add rust-analyzer</code></p>
    <p><strong>Configure your MCP client</strong>: This will be different but similar for each client. For Claude Desktop, add the following to <code>~/Library/Application\ Support/Claude/claude_desktop_config.json</code></p>

<pre>
{
  "mcpServers": {
    "language-server": {
      "command": "mcp-language-server",
      "args": [
        "--workspace",
        "/Users/you/dev/yourproject/",
        "--lsp",
        "rust-analyzer"
      ]
    }
  }
}
</pre>
  </div>
</details>
<details>
  <summary>Python (pyright)</summary>
  <div>
    <p><strong>Install pyright</strong>: <code>npm install -g pyright</code></p>
    <p><strong>Configure your MCP client</strong>: This will be different but similar for each client. For Claude Desktop, add the following to <code>~/Library/Application\ Support/Claude/claude_desktop_config.json</code></p>

<pre>
{
  "mcpServers": {
    "language-server": {
      "command": "mcp-language-server",
      "args": [
        "--workspace",
        "/Users/you/dev/yourproject/",
        "--lsp",
        "pyright-langserver",
        "--",
        "--stdio"
      ]
    }
  }
}
</pre>
  </div>
</details>
<details>
  <summary>Typescript (typescript-language-server)</summary>
  <div>
    <p><strong>Install typescript-language-server</strong>: <code>npm install -g typescript typescript-language-server</code></p>
    <p><strong>Configure your MCP client</strong>: This will be different but similar for each client. For Claude Desktop, add the following to <code>~/Library/Application\ Support/Claude/claude_desktop_config.json</code></p>

<pre>
{
  "mcpServers": {
    "language-server": {
      "command": "mcp-language-server",
      "args": [
        "--workspace",
        "/Users/you/dev/yourproject/",
        "--lsp",
        "typescript-language-server",
        "--",
        "--stdio"
      ]
    }
  }
}
</pre>
  </div>
</details>
<details>
  <summary>C/C++ (clangd)</summary>
  <div>
    <p><strong>Install clangd</strong>: Download prebuilt binaries from the <a href="https://github.com/clangd/clangd/releases">official LLVM releases page</a> or install via your system's package manager (e.g., <code>apt install clangd</code>, <code>brew install clangd</code>).</p>
    <p><strong>Configure your MCP client</strong>: This will be different but similar for each client. For Claude Desktop, add the following to <code>~/Library/Application\\ Support/Claude/claude_desktop_config.json</code></p>

<pre>
{
  "mcpServers": {
    "language-server": {
      "command": "mcp-language-server",
      "args": [
        "--workspace",
        "/Users/you/dev/yourproject/",
        "--lsp",
        "/path/to/your/clangd_binary",
        "--",
        "--compile-commands-dir=/path/to/yourproject/build_or_compile_commands_dir"
      ]
    }
  }
}
</pre>
    <p><strong>Note</strong>:</p>
    <ul>
      <li>Replace <code>/path/to/your/clangd_binary</code> with the actual path to your clangd executable.</li>
      <li><code>--compile-commands-dir</code> should point to the directory containing your <code>compile_commands.json</code> file (e.g., <code>./build</code>, <code>./cmake-build-debug</code>).</li>
      <li>Ensure <code>compile_commands.json</code> is generated for your project for clangd to work effectively.</li>
    </ul>
  </div>
</details>
<details>
  <summary>Civet (civet-lsp)</summary>
  <div>
    <p><strong>Install civet-lsp</strong>: <code>npm install -g @danielx/civet-language-server</code> (provides the <code>civet-lsp</code> binary).</p>
    <p><strong>Configure your MCP client</strong>: This will be different but similar for each client. For Claude Desktop, add the following to <code>~/Library/Application\ Support/Claude/claude_desktop_config.json</code></p>

<pre>
{
  "mcpServers": {
    "language-server": {
      "command": "mcp-language-server",
      "args": [
        "--workspace",
        "/Users/you/dev/yourproject/",
        "--lsp",
        "civet-lsp",
        "--",
        "--stdio"
      ]
    }
  }
}
</pre>
  </div>
</details>
<details>
  <summary>Other</summary>
  <div>
    <p>I have only tested this repo with the servers above but it should be compatible with many more. Note:</p>
    <ul>
      <li>The language server must communicate over stdio.</li>
      <li>Any aruments after <code>--</code> are sent as arguments to the language server.</li>
      <li>Any env variables are passed on to the language server.</li>
    </ul>
  </div>
</details>

## Flags

In addition to `--workspace` and `--lsp`:

- `--disable-tools=tool1,tool2,...` — suppress specific tools after registration. Useful when pairing with another tool (e.g. disabling `edit_file` when running alongside `aider`).
- `--idle-timeout=10m` — suspend the LSP subprocess and release all file watches after this duration with no MCP traffic; the next tool call restarts the LSP transparently. Default `0` (disabled). Useful when the parent editor keeps idle sessions (and their MCP children) alive indefinitely — each idle session otherwise pins an LSP process plus its inotify watches.
- `--lsp-init-async` — initialize the LSP in a background goroutine so `ServeStdio` starts immediately. Capability-gated tools then register after the handshake via `tools/list_changed`, so the client must honor that notification (Claude Desktop / Cursor do; some MCP clients only read `tools/list` once at startup). Default is synchronous init so all tools appear in the first `tools/list` response.
- `--config=/path/to/init.json` — pass per-LSP `initializationOptions`. The file is a JSON object keyed by LSP binary name; the matching entry becomes the LSP's `initializationOptions`. Solves cases like `rust-analyzer` needing `linkedProjects` or `gopls` needing specific build flags.

  ```jsonc
  {
    "rust-analyzer": { "linkedProjects": ["./Cargo.toml"] },
    "gopls":          { "buildFlags": ["-tags=integration"] }
  }
  ```

  When `--config` is absent, falls back to the previous hardcoded gopls-friendly defaults (`codelenses`, `semanticTokens`). Other LSPs ignore unknown init options per the LSP spec.

## Tools

Each tool is annotated with title + `ReadOnlyHint` / `DestructiveHint` so MCP clients can gate auto-approval and surface clearer labels. Tools that depend on a specific LSP capability are only registered when the connected LSP advertises support, so the tools/list response reflects what the language server can actually do.

Always-on tools (every LSP):

- `edit_file` (destructive): apply multiple text edits to a file based on line numbers. More reliable and token-efficient than search-and-replace edit tools.
- `diagnostics` (read-only): diagnostic information for a specific file. Waits for `textDocument/publishDiagnostics` rather than sleeping; works with push-only LSPs that reject `textDocument/diagnostic`.

Capability-gated tools (registered only if the LSP advertises the capability):

- `definition` (read-only): complete source-code definition of a symbol.
- `references` (read-only): all usages and references of a symbol throughout the codebase.
- `hover` (read-only): documentation, type hints, or other hover information at a position.
- `rename_symbol` (destructive): rename a symbol across the project.
- `document_symbols` (read-only): hierarchical symbol outline of a file (classes, functions, methods, etc.).
- `code_actions` (read-only): available code actions (quick fixes, refactorings, source actions) for a range.
- `format_document` (destructive): format a document (or a range within it) and apply the resulting edits to disk.
- `semantic_tokens` (read-only): full semantic-tokens response, decoded with the server's legend. Intended for debugging LSP semantic-token providers.

## Changes from upstream

This fork tracks `isaacphi/main` and folds in fixes from contributor forks. Notable additions:

- **LSP init scheduling** — synchronous by default so every capability-gated tool is registered before the first `tools/list` response (works with clients that don't honor `tools/list_changed`). Opt into the original lazy behavior with `--lsp-init-async`: `ServeStdio` starts immediately and tool handlers gate per-call on `waitForLSP` so a slow LSP (Kotlin/Gradle, large rust-analyzer indexes) doesn't stall the MCP handshake. Adapted from upstream PR #127.
- **Handshake hardening** — server-request handlers registered before `initialize` so `workspace/configuration` etc. don't see "method not found"; duplicate `initialized` notification removed; cleanup paths guarded with `sync.Once`.
- **Preopen removal** — the old "open every matching file on `client/registerCapability`" goroutine is gone. None of gopls / typescript-language-server / clangd / civet-lsp / rust-analyzer actually needed it, and it was the source of issue #83 ("too many open files").
- **Faster diagnostics** — wait for `textDocument/publishDiagnostics` instead of a fixed 3 s sleep; soft-fail on `-32601` from push-only LSPs.
- **LSP-3.17 glob matching** — bespoke pattern matcher replaced with the gopls implementation (cached, supports `**`, `{}`, `[...]`, `[!...]`).
- **File URIs** — built via `protocol.URIFromPath` everywhere; Windows paths no longer produce invalid `file://C:\...` URIs.
- **Richer document_symbols** — advertise `HierarchicalDocumentSymbolSupport=true` so LSPs return nested children (struct fields, interface methods).
- **C++ definition lookback** — capture `template<...>` and `[[attribute]]` lines preceding the LSP-reported symbol range.
- **Tool annotations** — every tool tagged with title + ReadOnly / Destructive hints.
- **New always-on tools** — `document_symbols`, `code_actions`, `format_document`, `semantic_tokens` (each registered only if the LSP advertises the capability).
- **New flags** — `--disable-tools`, `--idle-timeout`, `--config` (see [Flags](#flags)).
- **Extra language detection** — `.civet → civet`, `.hera → hera`, `.v → coq`.
- **Integration-test matrix** — clangd and civet-lsp added to CI alongside go / rust / python / typescript; each language runs as a separate CI job.

## About

This codebase makes use of edited code from [gopls](https://go.googlesource.com/tools/+/refs/heads/master/gopls/internal/protocol) to handle LSP communication. See ATTRIBUTION for details. Everything here is covered by a permissive BSD style license.

[mcp-go](https://github.com/mark3labs/mcp-go) is used for MCP communication. Thank you for your service.

This is beta software. Please let me know by creating an issue if you run into any problems or have suggestions of any kind.

## Contributing

Please keep PRs small and open Issues first for anything substantial. AI slop O.K. as long as it is tested, passes checks, and doesn't smell too bad.

### Setup

Clone the repo:

```bash
git clone https://github.com/STRd6/mcp-language-server.git
cd mcp-language-server
```

A [justfile](https://just.systems/man/en/) is included for convenience:

```bash
just -l
Available recipes:
    build    # Build
    check    # Run code audit checks
    fmt      # Format code
    generate # Generate LSP types and methods
    help     # Help
    install  # Install locally
    snapshot # Update snapshot tests
    test     # Run tests
```

Configure your Claude Desktop (or similar) to use the local binary:

```json
{
  "mcpServers": {
    "language-server": {
      "command": "/full/path/to/your/clone/mcp-language-server/mcp-language-server",
      "args": [
        "--workspace",
        "/path/to/workspace",
        "--lsp",
        "language-server-executable"
      ],
      "env": {
        "LOG_LEVEL": "DEBUG"
      }
    }
  }
}
```

Rebuild after making changes.

### Logging

Setting the `LOG_LEVEL` environment variable to DEBUG enables verbose logging to stderr for all components including messages to and from the language server and the language server's logs.

### LSP interaction

- `internal/lsp/methods.go` contains generated code to make calls to the connected language server.
- `internal/protocol/tsprotocol.go` contains generated code for LSP types. I borrowed this from `gopls`'s source code. Thank you for your service.
- LSP allows language servers to return different types for the same methods. Go doesn't like this so there are some ugly workarounds in `internal/protocol/interfaces.go`.

### Local Development and Snapshot Tests

There is a snapshot test suite that makes it a lot easier to try out changes to tools. These run actual language servers on mock workspaces and capture output and logs.

You will need the language servers installed locally to run them. The matrix covers `gopls`, `rust-analyzer`, `basedpyright-langserver`, `typescript-language-server`, `clangd`, and `civet-lsp`. Each runs as its own CI job (see `.github/workflows/go.yml`).

```
integrationtests/
├── tests/        # Tests are in this folder
├── snapshots/    # Snapshots of tool outputs
├── test-output/  # Gitignored folder showing the final state of each workspace and logs after each test run
└── workspaces/   # Mock workspaces that the tools run on
```

To update snapshots, run `UPDATE_SNAPSHOTS=true go test ./integrationtests/...`
