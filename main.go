package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/STRd6/mcp-language-server/internal/logging"
	"github.com/STRd6/mcp-language-server/internal/lsp"
	"github.com/STRd6/mcp-language-server/internal/protocol"
	"github.com/STRd6/mcp-language-server/internal/watcher"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// version is reported by --version and advertised as the MCP server
// version. Bump alongside the git tag.
const version = "v0.4.0"

// Create a logger for the core component
var coreLogger = logging.NewLogger(logging.Core)

type config struct {
	workspaceDir string
	lspCommand   string
	lspArgs      []string

	disabledToolStr string
	disabledTools   []string

	idleTimeout time.Duration

	configFile string
	lspConfig  map[string]any

	lspInitAsync bool
}

type mcpServer struct {
	config           config
	lspClient        *lsp.Client
	mcpServer        *server.MCPServer
	ctx              context.Context
	cancelFunc       context.CancelFunc
	workspaceWatcher *watcher.WorkspaceWatcher
	capabilities     *protocol.ServerCapabilities

	// lspReady is closed once initializeLSP returns (either success or
	// failure). Tool handlers gate on this so that, under --lsp-init-async,
	// ServeStdio can start answering MCP protocol traffic immediately
	// before slow LSPs (Kotlin/Gradle ~95s) finish their handshake. Under
	// the default sync path lspReady is closed before ServeStdio starts,
	// so waitForLSP returns immediately.
	lspReady   chan struct{}
	lspInitErr error
}

func printUsage() {
	out := flag.CommandLine.Output()
	_, _ = fmt.Fprintf(out, "Usage of %s:\n", os.Args[0])
	flag.PrintDefaults()
	_, _ = fmt.Fprint(out, `
Sample .mcp.json (drop into your project root, then start your MCP client):

{
  "mcpServers": {
    "language-server": {
      "command": "mcp-language-server",
      "args": [
        "--workspace",
        ".",
        "--lsp",
        "gopls"
      ]
    }
  }
}

Replace "gopls" with your language server (rust-analyzer, pyright-langserver,
typescript-language-server, clangd, civet-lsp, ...). Pass LSP-specific args
after a "--" entry in the args array. See the README for per-language setup.
`)
}

func parseConfig() (*config, error) {
	cfg := &config{}
	var showVersion bool
	flag.StringVar(&cfg.workspaceDir, "workspace", "", "Path to workspace directory")
	flag.StringVar(&cfg.lspCommand, "lsp", "", "LSP command to run (args should be passed after --)")
	flag.StringVar(&cfg.disabledToolStr, "disable-tools", "", "Comma-separated list of tools to disable")
	flag.DurationVar(&cfg.idleTimeout, "idle-timeout", 0, "Shut down after this duration of no MCP traffic (e.g. 10m); 0 disables")
	flag.StringVar(&cfg.configFile, "config", "", "Path to a JSON file whose keys are LSP binary names and values are passed as initializationOptions for that LSP (see README)")
	flag.BoolVar(&cfg.lspInitAsync, "lsp-init-async", false, "Initialize the LSP in a background goroutine so ServeStdio starts immediately. Capability-gated tools then register after the handshake via tools/list_changed (clients must honor it). Default: synchronous init, all tools available before ServeStdio.")
	flag.BoolVar(&showVersion, "version", false, "Print version and exit")
	flag.Usage = printUsage
	flag.Parse()

	if showVersion {
		_, _ = fmt.Fprintf(flag.CommandLine.Output(), "mcp-language-server %s\n", version)
		os.Exit(0)
	}

	// Get remaining args after -- as LSP arguments
	cfg.lspArgs = flag.Args()

	if cfg.disabledToolStr != "" {
		cfg.disabledTools = strings.Split(cfg.disabledToolStr, ",")
	}

	// Validate workspace directory
	if cfg.workspaceDir == "" {
		return nil, fmt.Errorf("workspace directory is required")
	}

	workspaceDir, err := filepath.Abs(cfg.workspaceDir)
	if err != nil {
		return nil, fmt.Errorf("failed to get absolute path for workspace: %v", err)
	}
	cfg.workspaceDir = workspaceDir

	if _, err := os.Stat(cfg.workspaceDir); os.IsNotExist(err) {
		return nil, fmt.Errorf("workspace directory does not exist: %s", cfg.workspaceDir)
	}

	// Validate LSP command
	if cfg.lspCommand == "" {
		return nil, fmt.Errorf("LSP command is required")
	}

	if _, err := exec.LookPath(cfg.lspCommand); err != nil {
		return nil, fmt.Errorf("LSP command not found: %s", cfg.lspCommand)
	}

	if cfg.configFile != "" {
		if err := parseConfigFile(cfg); err != nil {
			return nil, fmt.Errorf("failed to parse config file: %v", err)
		}
	}

	return cfg, nil
}

// parseConfigFile reads cfg.configFile and pulls out the entry whose key
// matches the basename of cfg.lspCommand. Its value becomes the
// initializationOptions sent to the LSP. Other keys are ignored so the
// same file can configure several LSPs.
func parseConfigFile(cfg *config) error {
	data, err := os.ReadFile(cfg.configFile)
	if err != nil {
		return fmt.Errorf("failed to read config file: %v", err)
	}

	var all map[string]any
	if err := json.Unmarshal(data, &all); err != nil {
		return fmt.Errorf("failed to parse JSON config: %v", err)
	}

	name := strings.TrimSuffix(filepath.Base(cfg.lspCommand), filepath.Ext(cfg.lspCommand))
	if entry, ok := all[name]; ok {
		m, ok := entry.(map[string]any)
		if !ok {
			return fmt.Errorf("config for %q must be a JSON object", name)
		}
		cfg.lspConfig = m
	}
	return nil
}

func newServer(config *config) (*mcpServer, error) {
	ctx, cancel := context.WithCancel(context.Background())
	return &mcpServer{
		config:     *config,
		ctx:        ctx,
		cancelFunc: cancel,
		lspReady:   make(chan struct{}),
	}, nil
}

// waitForLSP blocks until the LSP handshake has completed or ctx is done.
// Returns the LSP initialization error (if any) so handlers can fail fast.
func (s *mcpServer) waitForLSP(ctx context.Context) error {
	select {
	case <-s.lspReady:
		return s.lspInitErr
	case <-ctx.Done():
		return fmt.Errorf("context cancelled while waiting for LSP: %w", ctx.Err())
	}
}

func (s *mcpServer) initializeLSP() error {
	if err := os.Chdir(s.config.workspaceDir); err != nil {
		return fmt.Errorf("failed to change to workspace directory: %v", err)
	}

	client, err := lsp.NewClient(s.config.lspCommand, s.config.lspArgs...)
	if err != nil {
		return fmt.Errorf("failed to create LSP client: %v", err)
	}
	s.lspClient = client
	s.workspaceWatcher = watcher.NewWorkspaceWatcher(client)

	initResult, err := client.InitializeLSPClient(s.ctx, s.config.workspaceDir, s.config.lspConfig)
	if err != nil {
		return fmt.Errorf("initialize failed: %v", err)
	}

	s.capabilities = &initResult.Capabilities
	coreLogger.Debug("Server capabilities: %+v", initResult.Capabilities)

	go s.workspaceWatcher.WatchWorkspace(s.ctx, s.config.workspaceDir)
	return client.WaitForServerReady(s.ctx)
}

func (s *mcpServer) start(onIdle func()) error {
	opts := []server.ServerOption{
		server.WithLogging(),
		server.WithRecovery(),
	}
	if s.config.idleTimeout > 0 && onIdle != nil {
		timer := time.AfterFunc(s.config.idleTimeout, func() {
			coreLogger.Info("Idle timeout (%s) reached, initiating shutdown", s.config.idleTimeout)
			onIdle()
		})
		hooks := &server.Hooks{}
		hooks.AddBeforeAny(func(ctx context.Context, id any, method mcp.MCPMethod, message any) {
			timer.Reset(s.config.idleTimeout)
		})
		opts = append(opts, server.WithHooks(hooks))
		coreLogger.Info("Idle timeout armed: %s", s.config.idleTimeout)
	}

	s.mcpServer = server.NewMCPServer(
		"MCP Language Server",
		version,
		opts...,
	)

	s.registerAlwaysOnTools()

	if s.config.lspInitAsync {
		// Async path: ServeStdio starts immediately, capability-gated tools
		// register from the background goroutine once the LSP handshake
		// completes. mcp-go emits tools/list_changed so clients that honor
		// it pick the new tools up live. Clients that don't (e.g. clients
		// that cache tools/list and never refresh) will only ever see the
		// always-on tools — use the default sync path for those.
		go func() {
			err := s.initializeLSP()
			s.lspInitErr = err
			if err != nil {
				coreLogger.Error("LSP initialization failed: %v", err)
			} else {
				coreLogger.Info("LSP initialized successfully")
				s.registerCapabilityTools(s.capabilities)
			}
			close(s.lspReady)
		}()
		return server.ServeStdio(s.mcpServer)
	}

	// Sync path (default): finish the LSP handshake and register every
	// capability-gated tool before ServeStdio so the first tools/list
	// response is the complete set. Fail fast if the LSP can't init,
	// since we have no useful work to do without it.
	if err := s.initializeLSP(); err != nil {
		s.lspInitErr = err
		close(s.lspReady)
		return fmt.Errorf("LSP initialization failed: %v", err)
	}
	coreLogger.Info("LSP initialized successfully")
	s.registerCapabilityTools(s.capabilities)
	close(s.lspReady)

	return server.ServeStdio(s.mcpServer)
}

func main() {
	// parseConfig is called before any logging so that --version (and
	// flag-parse errors) print cleanly without a leading "starting" line.
	config, err := parseConfig()
	if err != nil {
		coreLogger.Fatal("%v", err)
	}

	coreLogger.Info("MCP Language Server %s starting", version)

	done := make(chan struct{})
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	server, err := newServer(config)
	if err != nil {
		coreLogger.Fatal("%v", err)
	}

	// Parent process monitoring channel
	parentDeath := make(chan struct{})

	// Monitor parent process termination
	// Claude desktop does not properly kill child processes for MCP servers
	go func() {
		ppid := os.Getppid()
		coreLogger.Debug("Monitoring parent process: %d", ppid)

		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				currentPpid := os.Getppid()
				if currentPpid != ppid && (currentPpid == 1 || ppid == 1) {
					coreLogger.Info("Parent process %d terminated (current ppid: %d), initiating shutdown", ppid, currentPpid)
					close(parentDeath)
					return
				}
			case <-done:
				return
			}
		}
	}()

	// Handle shutdown triggers
	go func() {
		select {
		case sig := <-sigChan:
			coreLogger.Info("Received signal %v in PID: %d", sig, os.Getpid())
			cleanup(server, done)
		case <-parentDeath:
			coreLogger.Info("Parent death detected, initiating shutdown")
			cleanup(server, done)
		}
	}()

	if err := server.start(func() { cleanup(server, done) }); err != nil {
		coreLogger.Error("Server error: %v", err)
		cleanup(server, done)
		os.Exit(1)
	}

	// ServeStdio returned nil — stdin reached EOF. The harness closed our
	// input (process exiting, or user closed the connection), or we're at
	// the receiving end of a shell pipeline whose left side has exited.
	// Without this cleanup the bridge would wait on <-done forever, since
	// done is only closed by the signal handler or the PPID-becomes-1
	// detector — neither of which fires on a plain stdin close while the
	// parent process stays alive. Reported as "mcp-language-server doesn't
	// exit on stdin EOF, holds shell pipelines hostage."
	coreLogger.Info("Stdin closed, initiating shutdown")
	cleanup(server, done)
	<-done
	coreLogger.Info("Server shutdown complete for PID: %d", os.Getpid())
	os.Exit(0)
}

var cleanupOnce sync.Once

func cleanup(s *mcpServer, done chan struct{}) {
	cleanupOnce.Do(func() { runCleanup(s, done) })
}

func runCleanup(s *mcpServer, done chan struct{}) {
	coreLogger.Info("Cleanup initiated for PID: %d", os.Getpid())

	// Create a context with timeout for shutdown operations
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if s.lspClient != nil {
		coreLogger.Info("Closing open files")
		s.lspClient.CloseAllFiles(ctx)

		// Create a shorter timeout context for the shutdown request
		shutdownCtx, shutdownCancel := context.WithTimeout(ctx, 500*time.Millisecond)
		defer shutdownCancel()

		// Run shutdown in a goroutine with timeout to avoid blocking if LSP doesn't respond
		shutdownDone := make(chan struct{})
		go func() {
			coreLogger.Info("Sending shutdown request")
			if err := s.lspClient.Shutdown(shutdownCtx); err != nil {
				coreLogger.Error("Shutdown request failed: %v", err)
			}
			close(shutdownDone)
		}()

		// Wait for shutdown with timeout
		select {
		case <-shutdownDone:
			coreLogger.Info("Shutdown request completed")
		case <-time.After(1 * time.Second):
			coreLogger.Warn("Shutdown request timed out, proceeding with exit")
		}

		coreLogger.Info("Sending exit notification")
		if err := s.lspClient.Exit(ctx); err != nil {
			coreLogger.Error("Exit notification failed: %v", err)
		}

		coreLogger.Info("Closing LSP client")
		if err := s.lspClient.Close(); err != nil {
			coreLogger.Error("Failed to close LSP client: %v", err)
		}
	}

	// Send signal to the done channel
	select {
	case <-done: // Channel already closed
	default:
		close(done)
	}

	coreLogger.Info("Cleanup completed for PID: %d", os.Getpid())
}
