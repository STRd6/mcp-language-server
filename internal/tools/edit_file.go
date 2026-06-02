package tools

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/STRd6/mcp-language-server/internal/lsp"
	"github.com/STRd6/mcp-language-server/internal/protocol"
	"github.com/STRd6/mcp-language-server/internal/utilities"
)

type TextEdit struct {
	StartLine int    `json:"startLine" jsonschema:"required,description=Start line to replace, inclusive"`
	EndLine   int    `json:"endLine" jsonschema:"required,description=End line to replace, inclusive"`
	NewText   string `json:"newText" jsonschema:"description=Replacement text. Replace with the new text. Leave blank to remove lines."`
}

func ApplyTextEdits(ctx context.Context, client *lsp.Client, filePath string, edits []TextEdit) (string, error) {
	err := client.OpenFile(ctx, filePath)
	if err != nil {
		return "", fmt.Errorf("could not open file: %v", err)
	}

	// Read the file once for edit classification (insert-as-replace vs real
	// replacement). The protocol only has range-replace, so callers wanting
	// to insert pass startLine == endLine and include the original line in
	// newText. Reporting that as "1 removed, 2 added" misleads the caller
	// into thinking the original was clobbered; detect the pattern here
	// and report "Inserted N lines" when applicable.
	content, err := os.ReadFile(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to read file: %v", err)
	}
	lineEnding := "\n"
	if bytes.Contains(content, []byte("\r\n")) {
		lineEnding = "\r\n"
	}
	fileLines := strings.Split(string(content), lineEnding)

	// Sort a copy ascending for reporting; we leave the input order alone
	// (it gets re-sorted descending below for application).
	sortedEdits := make([]TextEdit, len(edits))
	copy(sortedEdits, edits)
	sort.Slice(sortedEdits, func(i, j int) bool {
		return sortedEdits[i].StartLine < sortedEdits[j].StartLine
	})

	// Old-style tallies (kept for the fallback message when not every edit
	// is a pure insertion).
	linesRemovedSorted := 0
	linesAddedSorted := 0
	for _, edit := range sortedEdits {
		linesRemovedSorted += edit.EndLine - edit.StartLine + 1
		if edit.NewText != "" {
			linesAddedSorted += strings.Count(edit.NewText, "\n") + 1
		}
	}

	// Insert-as-replace detection: newText preserves the original lines
	// verbatim as either a prefix (insert-after) or suffix (insert-before),
	// with extra lines tacked on. Only report the "Inserted" message when
	// EVERY edit in the batch fits this pattern, so the existing
	// removed/added accounting remains correct for mixed cases.
	allInserts := len(sortedEdits) > 0
	insertedLines := 0
	for _, edit := range sortedEdits {
		startIdx := edit.StartLine - 1
		endIdx := edit.EndLine - 1
		if startIdx < 0 || endIdx < startIdx || endIdx >= len(fileLines) {
			allInserts = false
			break
		}
		existing := strings.Join(fileLines[startIdx:endIdx+1], lineEnding)
		switch {
		case existing != "" && strings.HasPrefix(edit.NewText, existing+lineEnding):
			extra := edit.NewText[len(existing)+len(lineEnding):]
			insertedLines += strings.Count(extra, lineEnding) + 1
		case existing != "" && strings.HasSuffix(edit.NewText, lineEnding+existing):
			extra := edit.NewText[:len(edit.NewText)-len(existing)-len(lineEnding)]
			insertedLines += strings.Count(extra, lineEnding) + 1
		default:
			allInserts = false
		}
		if !allInserts {
			break
		}
	}

	// Sort edits by line number in descending order to process from bottom to top
	// This way line numbers don't shift under us as we make edits
	sort.Slice(edits, func(i, j int) bool {
		return edits[i].StartLine > edits[j].StartLine
	})

	// Convert from input format to protocol.TextEdit
	var textEdits []protocol.TextEdit
	for _, edit := range edits {
		// Get the range covering the requested lines
		rng, err := getRange(edit.StartLine, edit.EndLine, filePath)
		if err != nil {
			return "", fmt.Errorf("invalid position: %v", err)
		}

		// Always do a replacement
		textEdits = append(textEdits, protocol.TextEdit{
			Range:   rng,
			NewText: edit.NewText,
		})
	}

	edit := protocol.WorkspaceEdit{
		Changes: map[protocol.DocumentUri][]protocol.TextEdit{
			protocol.URIFromPath(filePath): textEdits,
		},
	}

	if err := utilities.ApplyWorkspaceEdit(edit); err != nil {
		return "", fmt.Errorf("failed to apply text edits: %v", err)
	}

	// ApplyWorkspaceEdit only writes disk; the server owns the open doc's
	// content via didChange and won't see the write otherwise. Sync the
	// overlay so the next diagnostics query analyzes the edited content.
	// OpenFile above guarantees the file is tracked, so NotifyChange is valid.
	if err := client.NotifyChange(ctx, filePath); err != nil {
		// Non-fatal: the disk write succeeded; the server will catch up via the
		// watcher. Log and continue so the tool still reports success.
		toolsLogger.Debug("post-edit NotifyChange failed for %s: %v", filePath, err)
	}

	if allInserts && insertedLines > 0 {
		noun := "lines"
		if insertedLines == 1 {
			noun = "line"
		}
		return fmt.Sprintf("Successfully applied text edits. Inserted %d %s.", insertedLines, noun), nil
	}
	return fmt.Sprintf("Successfully applied text edits. %d lines removed, %d lines added.", linesRemovedSorted, linesAddedSorted), nil
}

// getRange creates a protocol.Range that covers the specified start and end lines
func getRange(startLine, endLine int, filePath string) (protocol.Range, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return protocol.Range{}, fmt.Errorf("failed to read file: %w", err)
	}

	// Detect line ending style
	var lineEnding string
	if bytes.Contains(content, []byte("\r\n")) {
		lineEnding = "\r\n"
	} else {
		lineEnding = "\n"
	}

	// Split lines without the line endings
	lines := strings.Split(string(content), lineEnding)

	// Handle start line positioning
	if startLine < 1 {
		return protocol.Range{}, fmt.Errorf("start line must be >= 1, got %d", startLine)
	}

	// Convert to 0-based line numbers
	startIdx := startLine - 1
	endIdx := endLine - 1

	// Handle EOF positioning
	if startIdx >= len(lines) {
		// For EOF, we want to point to the end of the last content-bearing line
		lastContentLineIdx := len(lines) - 1
		if lastContentLineIdx >= 0 && lines[lastContentLineIdx] == "" {
			lastContentLineIdx--
		}

		if lastContentLineIdx < 0 {
			lastContentLineIdx = 0
		}

		pos := protocol.Position{
			Line:      uint32(lastContentLineIdx),
			Character: uint32(len(lines[lastContentLineIdx])),
		}

		return protocol.Range{
			Start: pos,
			End:   pos,
		}, nil
	}

	// Normal range handling
	if endIdx >= len(lines) {
		endIdx = len(lines) - 1
	}

	// Always use the full line range for consistency
	return protocol.Range{
		Start: protocol.Position{
			Line:      uint32(startIdx),
			Character: 0, // Always start at beginning of line
		},
		End: protocol.Position{
			Line:      uint32(endIdx),
			Character: uint32(len(lines[endIdx])), // Go to end of last line
		},
	}, nil
}
