package tools

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/STRd6/mcp-language-server/internal/lsp"
	"github.com/STRd6/mcp-language-server/internal/protocol"
)

// GetDiagnosticsForFile retrieves diagnostics for a specific file from the language server
func GetDiagnosticsForFile(ctx context.Context, client *lsp.Client, filePath string, contextLines int, showLineNumbers bool) (string, error) {
	// Override with environment variable if specified
	if envLines := os.Getenv("LSP_CONTEXT_LINES"); envLines != "" {
		if val, err := strconv.Atoi(envLines); err == nil && val >= 0 {
			contextLines = val
		}
	}

	// If the doc is already open, the server owns its content via didChange and
	// ignores on-disk edits (didChangeWatchedFiles doesn't touch open docs).
	// Push the current disk content so pull/push diagnostics analyze the latest
	// version instead of a stale overlay — but only when it actually changed,
	// since a no-op didChange is wasted churn that some servers mishandle.
	notified := false
	if client.IsFileOpen(filePath) {
		changed, err := client.NotifyChangeIfChanged(ctx, filePath)
		if err != nil {
			return "", fmt.Errorf("could not sync file: %v", err)
		}
		notified = changed
	} else {
		if err := client.OpenFile(ctx, filePath); err != nil {
			return "", fmt.Errorf("could not open file: %v", err)
		}
	}

	// Convert the file path to URI format
	uri := protocol.URIFromPath(filePath)

	// Wait for the LSP to publish diagnostics for this URI (push mode).
	// 30s upper bound covers cold-start typecheck on real projects (Civet
	// on the Civet repo takes ~10s for its first publish). Returns as soon
	// as the first publish lands; settle window (150ms) absorbs follow-up
	// republishes some servers send after a project-wide rescan.
	hasPull := lsp.HasPullDiagnosticsSupport(client.ServerCapabilities())
	gotPublish := true
	if notified && !hasPull {
		// We just sent a didChange and the server can't be pulled, so any
		// cached publish is stale. WaitForDiagnostics would return immediately
		// off that stale cache (hasExisting), so wait for the NEXT publish.
		client.WaitForNextDiagnostics(ctx, uri, 30*time.Second, 150*time.Millisecond)
	} else {
		gotPublish = client.WaitForDiagnostics(ctx, uri, 30*time.Second, 150*time.Millisecond)
	}

	// Only attempt pull-mode (textDocument/diagnostic) for servers that
	// actually advertised a diagnosticProvider. Push-only servers (e.g.
	// gopls, ts-server) reject with -32601, but the request still rides their
	// request queue — on a busy LSP (cold TS typecheck on a real project)
	// that response can be blocked for many seconds behind the work
	// producing the publishDiagnostics we already got.
	if hasPull {
		diagParams := protocol.DocumentDiagnosticParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: uri},
		}
		if report, derr := client.Diagnostic(ctx, diagParams); derr != nil {
			toolsLogger.Debug("Pull-mode diagnostic request failed: %v", derr)
		} else if full, ok := report.Value.(protocol.RelatedFullDocumentDiagnosticReport); ok {
			// Pull results are computed at request time — authoritative and
			// race-free — so prefer them over the lagging push cache.
			client.SetFileDiagnostics(uri, full.Items)
		}
	}

	// Get diagnostics from the cache
	diagnostics := client.GetFileDiagnostics(uri)

	if len(diagnostics) == 0 {
		if !gotPublish {
			return "No diagnostics published yet for " + filePath +
				" — the language server may still be performing its initial analysis. Try again in a few seconds.", nil
		}
		return "No diagnostics found for " + filePath, nil
	}

	// Format file header
	fileInfo := fmt.Sprintf("%s\nDiagnostics in File: %d\n",
		filePath,
		len(diagnostics),
	)

	// Create a summary of all the diagnostics
	var diagSummaries []string
	var diagLocations []protocol.Location

	for _, diag := range diagnostics {
		severity := getSeverityString(diag.Severity)
		location := fmt.Sprintf("L%d:C%d",
			diag.Range.Start.Line+1,
			diag.Range.Start.Character+1)

		summary := fmt.Sprintf("%s at %s: %s",
			severity,
			location,
			diag.Message)

		// Add source and code if available
		if diag.Source != "" {
			summary += fmt.Sprintf(" (Source: %s", diag.Source)
			if diag.Code != nil {
				summary += fmt.Sprintf(", Code: %v", diag.Code)
			}
			summary += ")"
		} else if diag.Code != nil {
			summary += fmt.Sprintf(" (Code: %v)", diag.Code)
		}

		diagSummaries = append(diagSummaries, summary)

		// Create a location for this diagnostic to use with line ranges
		diagLocations = append(diagLocations, protocol.Location{
			URI:   uri,
			Range: diag.Range,
		})
	}

	// Format content with context
	fileContent, err := os.ReadFile(filePath)
	if err != nil {
		return fileInfo + "\nError reading file: " + err.Error(), nil
	}

	lines := strings.Split(string(fileContent), "\n")

	// Collect lines to display
	var linesToShow map[int]bool
	if contextLines > 0 {
		// Use GetLineRangesToDisplay for context
		linesToShow, err = GetLineRangesToDisplay(ctx, client, diagLocations, len(lines), contextLines)
		if err != nil {
			// If error, just show the diagnostic lines
			linesToShow = make(map[int]bool)
			for _, diag := range diagnostics {
				linesToShow[int(diag.Range.Start.Line)] = true
			}
		}
	} else {
		// Just show the diagnostic lines
		linesToShow = make(map[int]bool)
		for _, diag := range diagnostics {
			linesToShow[int(diag.Range.Start.Line)] = true
		}
	}

	// Convert to line ranges
	lineRanges := ConvertLinesToRanges(linesToShow, len(lines))

	// Format with diagnostics summary in header
	result := fileInfo
	if len(diagSummaries) > 0 {
		result += strings.Join(diagSummaries, "\n") + "\n"
	}

	// Format the content with ranges
	if showLineNumbers {
		result += "\n" + FormatLinesWithRanges(lines, lineRanges)
	}

	return result, nil
}

func getSeverityString(severity protocol.DiagnosticSeverity) string {
	switch severity {
	case protocol.SeverityError:
		return "ERROR"
	case protocol.SeverityWarning:
		return "WARNING"
	case protocol.SeverityInformation:
		return "INFO"
	case protocol.SeverityHint:
		return "HINT"
	default:
		return "UNKNOWN"
	}
}
