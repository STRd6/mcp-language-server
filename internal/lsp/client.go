package lsp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/STRd6/mcp-language-server/internal/protocol"
)

type Client struct {
	Cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout *bufio.Reader
	stderr io.ReadCloser

	// Serializes writes to stdin. LSP framing is header + body in two
	// separate Write calls, and we have several concurrent writers:
	// Call/Notify from tool-handler goroutines, plus server-request
	// responses from handleMessages (since b. moved them into goroutines).
	// Without this mutex two writers can interleave and corrupt the frame.
	stdinMu sync.Mutex

	// Request ID counter
	nextID atomic.Int32

	// Response handlers
	handlers   map[string]chan *Message
	handlersMu sync.RWMutex

	// Server request handlers
	serverRequestHandlers map[string]ServerRequestHandler
	serverHandlersMu      sync.RWMutex

	// Notification handlers
	notificationHandlers map[string]NotificationHandler
	notificationMu       sync.RWMutex

	// Diagnostic cache
	diagnostics   map[protocol.DocumentUri][]protocol.Diagnostic
	diagnosticsMu sync.RWMutex

	// Per-URI waiters fired when textDocument/publishDiagnostics arrives.
	// Used by WaitForDiagnostics to replace the old 3s sleep.
	diagnosticWaiters   map[protocol.DocumentUri][]chan struct{}
	diagnosticWaitersMu sync.Mutex

	// Progress state: tokens that have fired WorkDoneProgressEnd. Per-token
	// channel-of-channels semantics handled by progressWaiters below.
	progressEnded   map[string]bool
	progressTitles  map[string]string
	progressWaiters []*progressWaiter
	progressMu      sync.Mutex

	// Files are currently opened by the LSP
	openFiles   map[string]*OpenFileInfo
	openFilesMu sync.RWMutex

	// Server capabilities from the initialize response. Set once by
	// InitializeLSPClient and read by tools that need to gate optional LSP
	// requests (e.g. textDocument/diagnostic pull mode) on what the server
	// actually advertised. Read-only after init, so no mutex.
	serverCapabilities *protocol.ServerCapabilities

	// Close is reachable from cleanup, signal handlers, and the idle
	// watchdog. Guarantee the teardown runs exactly once and return
	// the same result to every caller.
	closeOnce sync.Once
	closeErr  error
}

// progressWaiter holds a single WaitForProgress call's predicate and signal
// channel. The handler scans waiters on every $/progress end and closes the
// channel of any whose predicate matches.
type progressWaiter struct {
	match func(token, title string) bool
	done  chan struct{}
}

func NewClient(command string, args ...string) (*Client, error) {
	cmd := exec.Command(command, args...)
	// Copy env
	cmd.Env = os.Environ()

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stdin pipe: %w", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	client := &Client{
		Cmd:                   cmd,
		stdin:                 stdin,
		stdout:                bufio.NewReader(stdout),
		stderr:                stderr,
		handlers:              make(map[string]chan *Message),
		notificationHandlers:  make(map[string]NotificationHandler),
		serverRequestHandlers: make(map[string]ServerRequestHandler),
		diagnostics:           make(map[protocol.DocumentUri][]protocol.Diagnostic),
		diagnosticWaiters:     make(map[protocol.DocumentUri][]chan struct{}),
		progressEnded:         make(map[string]bool),
		progressTitles:        make(map[string]string),
		openFiles:             make(map[string]*OpenFileInfo),
	}

	// Start the LSP server process
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start LSP server: %w", err)
	}

	// Handle stderr in a separate goroutine with proper logging. Uses
	// bufio.Reader.ReadString rather than bufio.Scanner because Scanner's
	// 64KB MaxScanTokenSize fails silently on the first oversize line
	// (long stack trace, verbose telemetry dump, etc.) — after that the
	// goroutine exits, the stderr pipe stops draining, and ~64KB later
	// the LSP blocks on its next write to stderr. That deadlock has been
	// observed as "bridge + LSP both alive, idle, sleeping on epoll/futex"
	// after long-running sessions.
	go func() {
		reader := bufio.NewReader(stderr)
		for {
			line, err := reader.ReadString('\n')
			if line != "" {
				processLogger.Info("%s", strings.TrimRight(line, "\r\n"))
			}
			if err != nil {
				if err != io.EOF {
					lspLogger.Error("Error reading LSP server stderr: %v", err)
				}
				return
			}
		}
	}()

	// Start message handling loop
	go client.handleMessages()

	return client, nil
}

func (c *Client) RegisterNotificationHandler(method string, handler NotificationHandler) {
	c.notificationMu.Lock()
	defer c.notificationMu.Unlock()
	c.notificationHandlers[method] = handler
}

func (c *Client) RegisterServerRequestHandler(method string, handler ServerRequestHandler) {
	c.serverHandlersMu.Lock()
	defer c.serverHandlersMu.Unlock()
	c.serverRequestHandlers[method] = handler
}

// defaultInitOptions are the InitializationOptions used when the caller
// doesn't supply a per-LSP override. Mostly gopls-specific keys; other
// LSPs ignore unknown fields per the spec.
var defaultInitOptions = map[string]any{
	"codelenses": map[string]bool{
		"generate":           true,
		"regenerate_cgo":     true,
		"test":               true,
		"tidy":               true,
		"upgrade_dependency": true,
		"vendor":             true,
		"vulncheck":          false,
	},
	"semanticTokens": true,
}

func (c *Client) InitializeLSPClient(ctx context.Context, workspaceDir string, customInitOptions map[string]any) (*protocol.InitializeResult, error) {
	initOptions := defaultInitOptions
	if len(customInitOptions) > 0 {
		initOptions = customInitOptions
	}

	initParams := &protocol.InitializeParams{
		WorkspaceFoldersInitializeParams: protocol.WorkspaceFoldersInitializeParams{
			WorkspaceFolders: []protocol.WorkspaceFolder{
				{
					URI:  protocol.URI(protocol.URIFromPath(workspaceDir)),
					Name: workspaceDir,
				},
			},
		},

		XInitializeParams: protocol.XInitializeParams{
			ProcessID: int32(os.Getpid()),
			ClientInfo: &protocol.ClientInfo{
				Name:    "mcp-language-server",
				Version: "0.1.0",
			},
			RootPath: workspaceDir,
			RootURI:  protocol.URIFromPath(workspaceDir),
			Capabilities: protocol.ClientCapabilities{
				Workspace: protocol.WorkspaceClientCapabilities{
					Configuration: true,
					DidChangeConfiguration: protocol.DidChangeConfigurationClientCapabilities{
						DynamicRegistration: true,
					},
					DidChangeWatchedFiles: protocol.DidChangeWatchedFilesClientCapabilities{
						DynamicRegistration:    true,
						RelativePatternSupport: true,
					},
				},
				TextDocument: protocol.TextDocumentClientCapabilities{
					Synchronization: &protocol.TextDocumentSyncClientCapabilities{
						DynamicRegistration: true,
						DidSave:             true,
					},
					Completion: protocol.CompletionClientCapabilities{
						CompletionItem: protocol.ClientCompletionItemOptions{},
					},
					CodeLens: &protocol.CodeLensClientCapabilities{
						DynamicRegistration: true,
					},
					DocumentSymbol: protocol.DocumentSymbolClientCapabilities{
						HierarchicalDocumentSymbolSupport: true,
					},
					CodeAction: protocol.CodeActionClientCapabilities{
						CodeActionLiteralSupport: protocol.ClientCodeActionLiteralOptions{
							CodeActionKind: protocol.ClientCodeActionKindOptions{
								ValueSet: []protocol.CodeActionKind{},
							},
						},
					},
					PublishDiagnostics: protocol.PublishDiagnosticsClientCapabilities{
						VersionSupport: true,
					},
					SemanticTokens: protocol.SemanticTokensClientCapabilities{
						Requests: protocol.ClientSemanticTokensRequestOptions{
							Range: &protocol.Or_ClientSemanticTokensRequestOptions_range{},
							Full:  &protocol.Or_ClientSemanticTokensRequestOptions_full{},
						},
						// LSP servers only emit tokens whose type/modifier the
						// client advertises. We list every standard type and
						// modifier from the spec so the server's full legend is
						// preserved end-to-end — required for semantic_tokens
						// tool to dump anything useful.
						TokenTypes: []string{
							"namespace", "type", "class", "enum", "interface", "struct",
							"typeParameter", "parameter", "variable", "property", "enumMember",
							"event", "function", "method", "macro", "keyword", "modifier",
							"comment", "string", "number", "regexp", "operator", "decorator",
							"label",
						},
						TokenModifiers: []string{
							"declaration", "definition", "readonly", "static", "deprecated",
							"abstract", "async", "modification", "documentation", "defaultLibrary",
						},
						Formats: []protocol.TokenFormat{protocol.Relative},
					},
				},
				// WorkDoneProgress enables server-initiated $/progress
				// notifications — used by WaitForProgress to skip
				// indexing/setup sleeps.
				Window: protocol.WindowClientCapabilities{
					WorkDoneProgress: true,
				},
			},
			InitializationOptions: initOptions,
		},
	}

	// Register handlers BEFORE sending initialize so any server-initiated
	// requests (e.g. workspace/configuration) that arrive during or
	// immediately after the handshake reach a registered handler instead
	// of getting a "method not found" reply that puts strict servers
	// (Kotlin LSP, async-lsp-based servers) into a broken state.
	c.RegisterServerRequestHandler("workspace/applyEdit", HandleApplyEdit)
	c.RegisterServerRequestHandler("workspace/configuration", HandleWorkspaceConfiguration)
	c.RegisterServerRequestHandler("client/registerCapability", HandleRegisterCapability)
	c.RegisterServerRequestHandler("window/workDoneProgress/create", HandleWorkDoneProgressCreate)
	c.RegisterNotificationHandler("window/showMessage", HandleServerMessage)
	c.RegisterNotificationHandler("textDocument/publishDiagnostics",
		func(params json.RawMessage) { HandleDiagnostics(c, params) })
	c.RegisterNotificationHandler("$/progress",
		func(params json.RawMessage) { c.handleProgress(params) })

	var result protocol.InitializeResult
	if err := c.Call(ctx, "initialize", initParams, &result); err != nil {
		return nil, fmt.Errorf("initialize failed: %w", err)
	}
	c.serverCapabilities = &result.Capabilities

	// Single initialized notification (no duplicate Notify call).
	if err := c.Initialized(ctx, protocol.InitializedParams{}); err != nil {
		return nil, fmt.Errorf("initialization failed: %w", err)
	}

	// LSP sepecific Initialization
	path := strings.ToLower(c.Cmd.Path)
	switch {
	case strings.Contains(path, "typescript-language-server"):
		err := initializeTypescriptLanguageServer(ctx, c, workspaceDir)
		if err != nil {
			return nil, err
		}
	}

	return &result, nil
}

func (c *Client) Close() error {
	c.closeOnce.Do(func() {
		// Try to close all open files first
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		// Attempt to close files but continue shutdown regardless
		c.CloseAllFiles(ctx)

		// Force kill the LSP process if it doesn't exit within timeout
		forcedKill := make(chan struct{})
		go func() {
			select {
			case <-time.After(2 * time.Second):
				lspLogger.Warn("LSP process did not exit within timeout, forcing kill")
				if c.Cmd.Process != nil {
					if err := c.Cmd.Process.Kill(); err != nil {
						lspLogger.Error("Failed to kill process: %v", err)
					} else {
						lspLogger.Info("Process killed successfully")
					}
				}
				close(forcedKill)
			case <-forcedKill:
				// Channel closed from completion path
				return
			}
		}()

		// Close stdin to signal the server
		if err := c.stdin.Close(); err != nil {
			lspLogger.Error("Failed to close stdin: %v", err)
		}

		// Wait for process to exit
		c.closeErr = c.Cmd.Wait()
		close(forcedKill) // Stop the force kill goroutine
	})

	return c.closeErr
}

type ServerState int

const (
	StateStarting ServerState = iota
	StateReady
	StateError
)

// ServerCapabilities returns the capabilities the server advertised in its
// initialize response, or nil if InitializeLSPClient hasn't completed.
func (c *Client) ServerCapabilities() *protocol.ServerCapabilities {
	return c.serverCapabilities
}

// WaitForServerReady is a no-op kept for API stability. Per the LSP spec the
// server is ready to receive requests as soon as the initialize handshake +
// initialized notification have completed, both of which InitializeLSPClient
// does synchronously. Servers that need post-init project loading must
// signal it via $/progress; callers that need that should use the progress
// helpers (e.g. WaitForProgress) instead of a blanket sleep.
func (c *Client) WaitForServerReady(ctx context.Context) error {
	return nil
}

// WaitForDiagnostics blocks until the LSP publishes diagnostics for uri or
// timeout expires, whichever comes first. After the first publish, sleeps
// settle to coalesce follow-up updates (e.g. project-wide rescans). The
// caller reads from the diagnostic cache after this returns. Returns true if
// a publish was observed (or the cache already held a prior publish), false
// if we timed out without ever seeing one — callers use the bool to
// distinguish "server said zero diagnostics" from "server hasn't reported yet".
func (c *Client) WaitForDiagnostics(ctx context.Context, uri protocol.DocumentUri, timeout, settle time.Duration) bool {
	ch := make(chan struct{}, 1)

	c.diagnosticWaitersMu.Lock()
	c.diagnosticWaiters[uri] = append(c.diagnosticWaiters[uri], ch)
	c.diagnosticWaitersMu.Unlock()

	defer func() {
		c.diagnosticWaitersMu.Lock()
		waiters := c.diagnosticWaiters[uri]
		for i, w := range waiters {
			if w == ch {
				c.diagnosticWaiters[uri] = append(waiters[:i], waiters[i+1:]...)
				break
			}
		}
		if len(c.diagnosticWaiters[uri]) == 0 {
			delete(c.diagnosticWaiters, uri)
		}
		c.diagnosticWaitersMu.Unlock()
	}()

	// If diagnostics already cached (file was opened earlier), fire immediately.
	c.diagnosticsMu.RLock()
	_, hasExisting := c.diagnostics[uri]
	c.diagnosticsMu.RUnlock()
	if hasExisting {
		return true
	}

	select {
	case <-ch:
		// First publish arrived. Sleep settle to absorb redundant follow-ups
		// (Civet republishes after a ~100ms project-wide propagation pass).
		select {
		case <-time.After(settle):
		case <-ctx.Done():
		}
		return true
	case <-time.After(timeout):
		lspLogger.Debug("WaitForDiagnostics timed out for %s after %s", uri, timeout)
		return false
	case <-ctx.Done():
		return false
	}
}

// WaitForNextDiagnostics waits for the NEXT textDocument/publishDiagnostics
// for uri (ignoring any cached prior publish), then settles. Use after edits
// that should provoke a fresh server-side re-analysis — e.g. notifying the
// LSP that a dependency file changed.
func (c *Client) WaitForNextDiagnostics(ctx context.Context, uri protocol.DocumentUri, timeout, settle time.Duration) {
	ch := make(chan struct{}, 1)

	c.diagnosticWaitersMu.Lock()
	c.diagnosticWaiters[uri] = append(c.diagnosticWaiters[uri], ch)
	c.diagnosticWaitersMu.Unlock()

	defer func() {
		c.diagnosticWaitersMu.Lock()
		waiters := c.diagnosticWaiters[uri]
		for i, w := range waiters {
			if w == ch {
				c.diagnosticWaiters[uri] = append(waiters[:i], waiters[i+1:]...)
				break
			}
		}
		if len(c.diagnosticWaiters[uri]) == 0 {
			delete(c.diagnosticWaiters, uri)
		}
		c.diagnosticWaitersMu.Unlock()
	}()

	select {
	case <-ch:
		select {
		case <-time.After(settle):
		case <-ctx.Done():
		}
	case <-time.After(timeout):
		lspLogger.Debug("WaitForNextDiagnostics timed out for %s after %s", uri, timeout)
	case <-ctx.Done():
	}
}

// WaitForProgress blocks until a $/progress notification with kind="end"
// arrives for a token (or its associated Begin title) matching the predicate,
// or until ctx is cancelled or timeout elapses. If the matching token has
// already ended, returns immediately. Returns nil on success, ctx.Err() on
// cancel, or an error on timeout.
//
// Use to replace ad-hoc sleeps that wait for "LSP has finished indexing":
// each server uses a distinguishing token or title for its workspace setup.
func (c *Client) WaitForProgress(ctx context.Context, match func(token, title string) bool, timeout time.Duration) error {
	c.progressMu.Lock()
	for token, ended := range c.progressEnded {
		if ended && match(token, c.progressTitles[token]) {
			c.progressMu.Unlock()
			return nil
		}
	}
	w := &progressWaiter{match: match, done: make(chan struct{})}
	c.progressWaiters = append(c.progressWaiters, w)
	c.progressMu.Unlock()

	defer func() {
		c.progressMu.Lock()
		for i, x := range c.progressWaiters {
			if x == w {
				c.progressWaiters = append(c.progressWaiters[:i], c.progressWaiters[i+1:]...)
				break
			}
		}
		c.progressMu.Unlock()
	}()

	select {
	case <-w.done:
		return nil
	case <-time.After(timeout):
		return fmt.Errorf("progress wait timed out after %s", timeout)
	case <-ctx.Done():
		return ctx.Err()
	}
}

// handleProgress is the $/progress notification handler. It tracks
// Begin/End frames per token and signals any matching WaitForProgress
// callers when an End frame arrives.
func (c *Client) handleProgress(params json.RawMessage) {
	var p protocol.ProgressParams
	if err := json.Unmarshal(params, &p); err != nil {
		lspLogger.Debug("progress: unmarshal params failed: %v", err)
		return
	}
	tokenStr := progressTokenString(p.Token)
	// Value is the payload, with discriminator field "kind".
	raw, _ := json.Marshal(p.Value)
	var kind struct {
		Kind  string `json:"kind"`
		Title string `json:"title"`
	}
	_ = json.Unmarshal(raw, &kind)

	c.progressMu.Lock()
	switch kind.Kind {
	case "begin":
		c.progressTitles[tokenStr] = kind.Title
		lspLogger.Debug("progress begin: token=%q title=%q", tokenStr, kind.Title)
	case "end":
		c.progressEnded[tokenStr] = true
		title := c.progressTitles[tokenStr]
		lspLogger.Debug("progress end: token=%q title=%q", tokenStr, title)
		for _, w := range c.progressWaiters {
			if w.match(tokenStr, title) {
				select {
				case <-w.done:
				default:
					close(w.done)
				}
			}
		}
	}
	c.progressMu.Unlock()
}

// progressTokenString reduces an Or_ProgressToken to its string form.
// JSON unmarshal produces either string or float64 (per the Or [int32 string]
// type); both render the same to callers matching tokens.
func progressTokenString(t protocol.ProgressToken) string {
	switch v := t.Value.(type) {
	case string:
		return v
	case float64:
		return fmt.Sprintf("%g", v)
	default:
		return fmt.Sprintf("%v", v)
	}
}

type OpenFileInfo struct {
	Version int32
	URI     protocol.DocumentUri
}

func (c *Client) OpenFile(ctx context.Context, filepath string) error {
	docURI := protocol.URIFromPath(filepath)

	c.openFilesMu.Lock()
	if _, exists := c.openFiles[string(docURI)]; exists {
		c.openFilesMu.Unlock()
		return nil // Already open
	}
	c.openFilesMu.Unlock()

	// Skip files that do not exist or cannot be read
	content, err := os.ReadFile(filepath)
	if err != nil {
		return fmt.Errorf("error reading file: %w", err)
	}

	params := protocol.DidOpenTextDocumentParams{
		TextDocument: protocol.TextDocumentItem{
			URI:        docURI,
			LanguageID: DetectLanguageID(filepath),
			Version:    1,
			Text:       string(content),
		},
	}

	if err := c.Notify(ctx, "textDocument/didOpen", params); err != nil {
		return err
	}

	c.openFilesMu.Lock()
	c.openFiles[string(docURI)] = &OpenFileInfo{
		Version: 1,
		URI:     docURI,
	}
	c.openFilesMu.Unlock()

	lspLogger.Debug("Opened file: %s", filepath)

	return nil
}

func (c *Client) NotifyChange(ctx context.Context, filepath string) error {
	docURI := protocol.URIFromPath(filepath)

	content, err := os.ReadFile(filepath)
	if err != nil {
		return fmt.Errorf("error reading file: %w", err)
	}

	c.openFilesMu.Lock()
	fileInfo, isOpen := c.openFiles[string(docURI)]
	if !isOpen {
		c.openFilesMu.Unlock()
		return fmt.Errorf("cannot notify change for unopened file: %s", filepath)
	}

	// Increment version
	fileInfo.Version++
	version := fileInfo.Version
	c.openFilesMu.Unlock()

	params := protocol.DidChangeTextDocumentParams{
		TextDocument: protocol.VersionedTextDocumentIdentifier{
			TextDocumentIdentifier: protocol.TextDocumentIdentifier{
				URI: docURI,
			},
			Version: version,
		},
		ContentChanges: []protocol.TextDocumentContentChangeEvent{
			{
				Value: protocol.TextDocumentContentChangeWholeDocument{
					Text: string(content),
				},
			},
		},
	}

	return c.Notify(ctx, "textDocument/didChange", params)
}

func (c *Client) CloseFile(ctx context.Context, filepath string) error {
	docURI := protocol.URIFromPath(filepath)

	c.openFilesMu.Lock()
	if _, exists := c.openFiles[string(docURI)]; !exists {
		c.openFilesMu.Unlock()
		return nil // Already closed
	}
	c.openFilesMu.Unlock()

	params := protocol.DidCloseTextDocumentParams{
		TextDocument: protocol.TextDocumentIdentifier{
			URI: docURI,
		},
	}
	lspLogger.Debug("Closing file: %s", params.TextDocument.URI.Dir())
	if err := c.Notify(ctx, "textDocument/didClose", params); err != nil {
		return err
	}

	c.openFilesMu.Lock()
	delete(c.openFiles, string(docURI))
	c.openFilesMu.Unlock()

	return nil
}

func (c *Client) IsFileOpen(filepath string) bool {
	uri := string(protocol.URIFromPath(filepath))
	c.openFilesMu.RLock()
	defer c.openFilesMu.RUnlock()
	_, exists := c.openFiles[uri]
	return exists
}

// CloseAllFiles closes all currently open files
func (c *Client) CloseAllFiles(ctx context.Context) {
	c.openFilesMu.Lock()
	filesToClose := make([]string, 0, len(c.openFiles))

	// First collect all URIs that need to be closed
	for uri := range c.openFiles {
		filePath := protocol.DocumentUri(uri).Path()
		filesToClose = append(filesToClose, filePath)
	}
	c.openFilesMu.Unlock()

	// Then close them all
	for _, filePath := range filesToClose {
		err := c.CloseFile(ctx, filePath)
		if err != nil {
			lspLogger.Error("Error closing file %s: %v", filePath, err)
		}
	}

	lspLogger.Debug("Closed %d files", len(filesToClose))
}

func (c *Client) GetFileDiagnostics(uri protocol.DocumentUri) []protocol.Diagnostic {
	c.diagnosticsMu.RLock()
	defer c.diagnosticsMu.RUnlock()

	return c.diagnostics[uri]
}
