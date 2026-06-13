package call_hierarchy_test

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

// TestCallHierarchy tests the call_hierarchy tool against gopls.
// HelperFunction (helper.go:4:6) is called by ConsumerFunction and
// AnotherConsumer; ConsumerFunction (consumer.go:6:6) calls HelperFunction
// plus several SharedStruct methods.
func TestCallHierarchy(t *testing.T) {
	suite := internal.GetTestSuite(t)

	ctx, cancel := context.WithTimeout(suite.Context, 10*time.Second)
	defer cancel()

	t.Run("IncomingCalls", func(t *testing.T) {
		filePath := filepath.Join(suite.WorkspaceDir, "helper.go")
		result, err := tools.GetCallHierarchy(ctx, suite.Client, filePath, 4, 6, "incoming", 1)
		if err != nil {
			t.Fatalf("GetCallHierarchy failed: %v", err)
		}

		for _, want := range []string{"Incoming calls", "ConsumerFunction", "AnotherConsumer"} {
			if !strings.Contains(result, want) {
				t.Errorf("expected %q in output; got:\n%s", want, result)
			}
		}
		if strings.Contains(result, "Outgoing calls") {
			t.Errorf("did not expect outgoing section for direction=incoming; got:\n%s", result)
		}

		// Both callers are in-workspace, so paths are stable under snapshot
		// normalization.
		common.SnapshotTest(t, "go", "call_hierarchy", "incoming-helper-function", result)
	})

	t.Run("OutgoingCalls", func(t *testing.T) {
		// No snapshot: outgoing calls include fmt.Println, whose GOROOT path
		// varies by machine and isn't normalized by SnapshotTest.
		filePath := filepath.Join(suite.WorkspaceDir, "consumer.go")
		result, err := tools.GetCallHierarchy(ctx, suite.Client, filePath, 6, 6, "outgoing", 1)
		if err != nil {
			t.Fatalf("GetCallHierarchy failed: %v", err)
		}

		for _, want := range []string{"Outgoing calls", "HelperFunction", "Process"} {
			if !strings.Contains(result, want) {
				t.Errorf("expected %q in output; got:\n%s", want, result)
			}
		}
		if strings.Contains(result, "Incoming calls") {
			t.Errorf("did not expect incoming section for direction=outgoing; got:\n%s", result)
		}
	})

	t.Run("BothDirections", func(t *testing.T) {
		filePath := filepath.Join(suite.WorkspaceDir, "helper.go")
		result, err := tools.GetCallHierarchy(ctx, suite.Client, filePath, 4, 6, "both", 1)
		if err != nil {
			t.Fatalf("GetCallHierarchy failed: %v", err)
		}

		if !strings.Contains(result, "Incoming calls") || !strings.Contains(result, "Outgoing calls") {
			t.Errorf("expected both sections; got:\n%s", result)
		}
		// HelperFunction calls nothing.
		if !strings.Contains(result, "(none)") {
			t.Errorf("expected '(none)' for HelperFunction's outgoing calls; got:\n%s", result)
		}
	})

	t.Run("NotCallable", func(t *testing.T) {
		// Line 1 column 1 is the package clause. gopls rejects the request
		// ("identifier not found"); other servers return an empty item list,
		// which the tool reports as "No callable symbol". Both are acceptable.
		filePath := filepath.Join(suite.WorkspaceDir, "helper.go")
		result, err := tools.GetCallHierarchy(ctx, suite.Client, filePath, 1, 1, "both", 1)
		if err == nil && !strings.Contains(result, "No callable symbol") {
			t.Errorf("expected an error or 'No callable symbol'; got:\n%s", result)
		}
	})

	t.Run("InvalidDirection", func(t *testing.T) {
		filePath := filepath.Join(suite.WorkspaceDir, "helper.go")
		_, err := tools.GetCallHierarchy(ctx, suite.Client, filePath, 4, 6, "sideways", 1)
		if err == nil || !strings.Contains(err.Error(), "invalid direction") {
			t.Errorf("expected invalid direction error, got: %v", err)
		}
	})
}
