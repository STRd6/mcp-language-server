// Package internal contains shared helpers for Clangd tests
package internal

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/isaacphi/mcp-language-server/integrationtests/tests/common"
)

// GetTestSuite returns a test suite for Clangd language server tests
func GetTestSuite(t *testing.T) *common.TestSuite {
	repoRoot, err := filepath.Abs("../../../..")
	if err != nil {
		t.Fatalf("Failed to get repo root: %v", err)
	}

	sourceWorkspace := filepath.Join(repoRoot, "integrationtests/workspaces/clangd")
	config := common.LSPTestConfig{
		Name:             "clangd",
		Command:          "clangd",
		WorkspaceDir:     sourceWorkspace,
		InitializeTimeMs: 2000,
	}

	suite := common.NewTestSuite(t, config)
	if err := suite.Setup(); err != nil {
		t.Fatalf("Failed to set up test suite: %v", err)
	}

	// Rewrite the copied compile_commands.json so its absolute paths
	// point at the test-output workspace clangd will operate in. Without
	// this, clangd indexes the source workspace paths from compile_commands
	// while tests open files at the test-output paths — references/
	// definitions then return *both* URIs, doubling output and breaking
	// snapshots.
	rewriteCompileCommands(t, sourceWorkspace, suite.WorkspaceDir)

	t.Cleanup(func() {
		suite.Cleanup()
	})

	return suite
}

func rewriteCompileCommands(t *testing.T, fromDir, toDir string) {
	path := filepath.Join(toDir, "compile_commands.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Logf("compile_commands.json not present at %s: %v", path, err)
		return
	}
	rewritten := strings.ReplaceAll(string(data), fromDir, toDir)
	if err := os.WriteFile(path, []byte(rewritten), 0644); err != nil {
		t.Fatalf("rewrite compile_commands.json: %v", err)
	}
}
