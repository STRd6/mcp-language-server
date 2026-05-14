// Package internal contains shared helpers for Civet tests
package internal

import (
	"path/filepath"
	"testing"

	"github.com/isaacphi/mcp-language-server/integrationtests/tests/common"
)

// GetTestSuite returns a test suite for civet-lsp.
func GetTestSuite(t *testing.T) *common.TestSuite {
	repoRoot, err := filepath.Abs("../../../..")
	if err != nil {
		t.Fatalf("Failed to get repo root: %v", err)
	}

	config := common.LSPTestConfig{
		Name:             "civet",
		Command:          "civet-lsp",
		Args:             []string{"--stdio"},
		WorkspaceDir:     filepath.Join(repoRoot, "integrationtests/workspaces/civet"),
		InitializeTimeMs: 3000,
	}

	suite := common.NewTestSuite(t, config)
	if err := suite.Setup(); err != nil {
		t.Fatalf("Failed to set up test suite: %v", err)
	}
	t.Cleanup(func() {
		suite.Cleanup()
	})
	return suite
}
