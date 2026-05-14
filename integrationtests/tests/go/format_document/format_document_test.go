package format_document_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/STRd6/mcp-language-server/integrationtests/tests/go/internal"
	"github.com/STRd6/mcp-language-server/internal/lsp"
	"github.com/STRd6/mcp-language-server/internal/tools"
)

// misformatted Go source — extra spaces, wrong indentation, missing newline.
// gofmt should canonicalise to single-tab indents, single spaces, trailing newline.
const misformatted = `package main

import   "fmt"

func   Hello(  name   string  )    string   {
	return  fmt.Sprintf( "hello, %s", name )
}
`

const wellFormatted = `package main

import "fmt"

func Hello(name string) string {
	return fmt.Sprintf("hello, %s", name)
}
`

// TestFormatDocument_Full verifies the format_document tool runs gofmt via gopls
// and writes the corrected output back to disk.
func TestFormatDocument_Full(t *testing.T) {
	suite := internal.GetTestSuite(t)

	if !lsp.HasFormattingSupport(suite.Capabilities) {
		t.Fatal("gopls did not advertise formatting capability — environment issue")
	}

	// Write a fresh misformatted file into the per-suite workspace copy. This
	// avoids polluting the upstream fixture and means the test is hermetic.
	target := filepath.Join(suite.WorkspaceDir, "needs_format.go")
	if err := os.WriteFile(target, []byte(misformatted), 0644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	t.Cleanup(func() { _ = os.Remove(target) })

	ctx, cancel := context.WithTimeout(suite.Context, 5*time.Second)
	defer cancel()

	result, err := tools.FormatDocument(ctx, suite.Client, target, "full", 0, 0, 0, 0, "")
	if err != nil {
		t.Fatalf("FormatDocument failed: %v", err)
	}
	if !strings.Contains(result, "Successfully formatted") {
		t.Errorf("expected success message in result; got:\n%s", result)
	}

	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("read formatted file: %v", err)
	}
	if string(got) != wellFormatted {
		t.Errorf("file not formatted as expected.\n--- want ---\n%s\n--- got ---\n%s", wellFormatted, got)
	}
}

// TestFormatDocument_NoChanges verifies the no-op case: formatting an already
// well-formatted file should report no changes and leave the file untouched.
func TestFormatDocument_NoChanges(t *testing.T) {
	suite := internal.GetTestSuite(t)

	if !lsp.HasFormattingSupport(suite.Capabilities) {
		t.Fatal("gopls did not advertise formatting capability — environment issue")
	}

	target := filepath.Join(suite.WorkspaceDir, "already_clean.go")
	if err := os.WriteFile(target, []byte(wellFormatted), 0644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	t.Cleanup(func() { _ = os.Remove(target) })

	ctx, cancel := context.WithTimeout(suite.Context, 5*time.Second)
	defer cancel()

	result, err := tools.FormatDocument(ctx, suite.Client, target, "full", 0, 0, 0, 0, "")
	if err != nil {
		t.Fatalf("FormatDocument failed: %v", err)
	}
	if !strings.Contains(result, "No formatting changes") {
		t.Errorf("expected no-change message; got:\n%s", result)
	}

	got, _ := os.ReadFile(target)
	if string(got) != wellFormatted {
		t.Errorf("file should be unchanged.\n--- want ---\n%s\n--- got ---\n%s", wellFormatted, got)
	}
}
