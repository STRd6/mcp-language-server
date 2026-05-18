package references_test

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

// TestFindReferencesAtPosition exercises the positional references tool —
// instead of fanning workspace/symbol matches into per-symbol references
// requests, it issues textDocument/references once at a precise (line,
// column) and groups the resulting locations by file.
func TestFindReferencesAtPosition(t *testing.T) {
	suite := internal.GetTestSuite(t)

	ctx, cancel := context.WithTimeout(suite.Context, 10*time.Second)
	defer cancel()

	tests := []struct {
		name          string
		file          string
		line          int
		column        int
		expectedText  string
		expectedFiles int
		snapshotName  string
	}{
		{
			// helper.go:4 → declaration of HelperFunction; references live in
			// consumer.go and another_consumer.go.
			name:          "FunctionDeclaration",
			file:          "helper.go",
			line:          4,
			column:        6,
			expectedText:  "ConsumerFunction",
			expectedFiles: 2,
			snapshotName:  "helper-function",
		},
		{
			// types.go:6 → declaration of SharedStruct; used in both consumers.
			name:          "StructDeclaration",
			file:          "types.go",
			line:          6,
			column:        6,
			expectedText:  "SharedStruct",
			expectedFiles: 2,
			snapshotName:  "shared-struct",
		},
		{
			// types.go:14 → SharedStruct.Method() declaration; `Method` starts
			// at col 24. Used in consumer.go:19.
			name:          "MethodDeclaration",
			file:          "types.go",
			line:          14,
			column:        24,
			expectedText:  "s.Method()",
			expectedFiles: 1,
			snapshotName:  "struct-method",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			filePath := filepath.Join(suite.WorkspaceDir, tc.file)

			result, err := tools.FindReferencesAtPosition(ctx, suite.Client, filePath, tc.line, tc.column)
			if err != nil {
				t.Fatalf("Failed to find references at position: %v", err)
			}

			if !strings.Contains(result, tc.expectedText) {
				t.Errorf("References do not contain expected text %q\nGot:\n%s", tc.expectedText, result)
			}

			fileCount := countFilesInResult(result)
			if fileCount < tc.expectedFiles {
				t.Errorf("Expected references in at least %d files, but found in %d files",
					tc.expectedFiles, fileCount)
			}

			common.SnapshotTest(t, "go", "references_at_position", tc.snapshotName, result)
		})
	}
}
