package type_hierarchy_test

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

// TestTypeHierarchy tests the type_hierarchy tool against gopls.
// SharedStruct (types.go:6:6) implements SharedInterface (types.go:19:6).
func TestTypeHierarchy(t *testing.T) {
	suite := internal.GetTestSuite(t)

	ctx, cancel := context.WithTimeout(suite.Context, 10*time.Second)
	defer cancel()

	t.Run("Supertypes", func(t *testing.T) {
		filePath := filepath.Join(suite.WorkspaceDir, "types.go")
		result, err := tools.GetTypeHierarchy(ctx, suite.Client, filePath, 6, 6, "supertypes", 1)
		if err != nil {
			t.Fatalf("GetTypeHierarchy failed: %v", err)
		}

		for _, want := range []string{"Supertypes", "SharedInterface"} {
			if !strings.Contains(result, want) {
				t.Errorf("expected %q in output; got:\n%s", want, result)
			}
		}
		if strings.Contains(result, "Subtypes") {
			t.Errorf("did not expect subtypes section for direction=supertypes; got:\n%s", result)
		}

		common.SnapshotTest(t, "go", "type_hierarchy", "supertypes-shared-struct", result)
	})

	t.Run("Subtypes", func(t *testing.T) {
		// No snapshot: whether gopls includes function-local implementors
		// (CustomImplementor in another_consumer.go) varies by version.
		filePath := filepath.Join(suite.WorkspaceDir, "types.go")
		result, err := tools.GetTypeHierarchy(ctx, suite.Client, filePath, 19, 6, "subtypes", 1)
		if err != nil {
			t.Fatalf("GetTypeHierarchy failed: %v", err)
		}

		for _, want := range []string{"Subtypes", "SharedStruct"} {
			if !strings.Contains(result, want) {
				t.Errorf("expected %q in output; got:\n%s", want, result)
			}
		}
		if strings.Contains(result, "Supertypes") {
			t.Errorf("did not expect supertypes section for direction=subtypes; got:\n%s", result)
		}
	})

	t.Run("BothDirectionsNoRelatives", func(t *testing.T) {
		// SharedType is a bare named int: no supertypes, no subtypes.
		filePath := filepath.Join(suite.WorkspaceDir, "types.go")
		result, err := tools.GetTypeHierarchy(ctx, suite.Client, filePath, 28, 6, "both", 1)
		if err != nil {
			t.Fatalf("GetTypeHierarchy failed: %v", err)
		}

		if !strings.Contains(result, "Supertypes") || !strings.Contains(result, "Subtypes") {
			t.Errorf("expected both sections; got:\n%s", result)
		}
		if strings.Count(result, "(none)") != 2 {
			t.Errorf("expected '(none)' for both directions; got:\n%s", result)
		}
	})

	t.Run("InvalidDirection", func(t *testing.T) {
		filePath := filepath.Join(suite.WorkspaceDir, "types.go")
		_, err := tools.GetTypeHierarchy(ctx, suite.Client, filePath, 6, 6, "diagonal", 1)
		if err == nil || !strings.Contains(err.Error(), "invalid direction") {
			t.Errorf("expected invalid direction error, got: %v", err)
		}
	})
}
