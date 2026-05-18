package definition_test

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/STRd6/mcp-language-server/integrationtests/tests/common"
	"github.com/STRd6/mcp-language-server/integrationtests/tests/go/internal"
	"github.com/STRd6/mcp-language-server/internal/tools"
)

// TestReadDefinitionAtPosition exercises the positional definition tool —
// instead of resolving by name via workspace/symbol, it issues
// textDocument/definition at a precise (line, column) and returns the
// resolved symbol's source.
func TestReadDefinitionAtPosition(t *testing.T) {
	suite := internal.GetTestSuite(t)

	ctx, cancel := context.WithTimeout(suite.Context, 10*time.Second)
	defer cancel()

	tests := []struct {
		name         string
		file         string
		line         int
		column       int
		expectedText string
		snapshotName string
	}{
		{
			// main.go:13 → `fmt.Println(FooBar())` — column points at `FooBar`.
			name:         "FunctionCall",
			file:         "main.go",
			line:         13,
			column:       14,
			expectedText: "func FooBar()",
			snapshotName: "foobar-call",
		},
		{
			// consumer.go:7 → `message := HelperFunction()` — column on `HelperFunction`.
			name:         "HelperFunctionCall",
			file:         "consumer.go",
			line:         7,
			column:       14,
			expectedText: "func HelperFunction()",
			snapshotName: "helper-call",
		},
		{
			// consumer.go:11 → `s := &SharedStruct{` — column on `SharedStruct`.
			name:         "StructLiteral",
			file:         "consumer.go",
			line:         11,
			column:       9,
			expectedText: "type SharedStruct struct",
			snapshotName: "struct-literal",
		},
		{
			// consumer.go:19 → `fmt.Println(s.Method())` — column on `Method`.
			name:         "MethodCall",
			file:         "consumer.go",
			line:         19,
			column:       17,
			expectedText: "func (s *SharedStruct) Method()",
			snapshotName: "method-call",
		},
		{
			// consumer.go:1 is `package main` — no symbol at column 1.
			name:         "NoSymbolAtPosition",
			file:         "consumer.go",
			line:         1,
			column:       1,
			expectedText: "No definition found",
			snapshotName: "no-symbol",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			filePath := filepath.Join(suite.WorkspaceDir, tc.file)

			result, err := tools.ReadDefinitionAtPosition(ctx, suite.Client, filePath, tc.line, tc.column)
			if err != nil {
				t.Fatalf("Failed to read definition at position: %v", err)
			}

			if !strings.Contains(result, tc.expectedText) {
				t.Errorf("Definition does not contain expected text %q\nGot:\n%s", tc.expectedText, result)
			}

			common.SnapshotTest(t, "go", "definition_at_position", tc.snapshotName, result)
		})
	}
}
