package main

import (
	"context"
	"fmt"

	"github.com/STRd6/mcp-language-server/internal/lsp"
	"github.com/STRd6/mcp-language-server/internal/protocol"
	"github.com/STRd6/mcp-language-server/internal/tools"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// addTool registers a tool whose handler blocks on waitForLSP. ServeStdio
// starts before the LSP handshake completes (see start()), so tool calls
// that arrive during LSP startup wait here instead of erroring or stalling
// the whole MCP connection.
func (s *mcpServer) addTool(tool mcp.Tool, handler server.ToolHandlerFunc) {
	s.mcpServer.AddTool(tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if err := s.waitForLSP(ctx); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("LSP not ready: %v", err)), nil
		}
		return handler(ctx, request)
	})
}

// registerAlwaysOnTools registers edit_file and diagnostics, which don't
// depend on the LSP advertising any specific capability. Called before
// ServeStdio so tools/list responses include them immediately.
func (s *mcpServer) registerAlwaysOnTools() {
	coreLogger.Debug("Registering always-on MCP tools")

	// edit_file and diagnostics are always registered: edit_file requires only
	// TextDocumentSync (every LSP), and diagnostics rides on push notifications
	// rather than a capability.
	applyTextEditTool := mcp.NewTool("edit_file",
		mcp.WithDescription("Apply multiple text edits to a file."),
		mcp.WithTitleAnnotation("Edit File"),
		mcp.WithDestructiveHintAnnotation(true),
		mcp.WithArray("edits",
			mcp.Required(),
			mcp.Description("List of edits to apply"),
			mcp.Items(map[string]any{
				"type": "object",
				"properties": map[string]any{
					"startLine": map[string]any{
						"type":        "number",
						"description": "Start line to replace, inclusive, one-indexed",
					},
					"endLine": map[string]any{
						"type":        "number",
						"description": "End line to replace, inclusive, one-indexed",
					},
					"newText": map[string]any{
						"type":        "string",
						"description": "Replacement text. Replace with the new text. Leave blank to remove lines.",
					},
				},
				"required": []string{"startLine", "endLine"},
			}),
		),
		mcp.WithString("filePath",
			mcp.Required(),
			mcp.Description("Path to the file to edit"),
		),
	)

	s.addTool(applyTextEditTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		filePath, err := request.RequireString("filePath")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		// `edits` is an array of objects, no typed accessor available — read
		// from the raw arguments map.
		args := request.GetArguments()
		editsArg, ok := args["edits"]
		if !ok {
			return mcp.NewToolResultError("edits is required"), nil
		}
		editsArray, ok := editsArg.([]any)
		if !ok {
			return mcp.NewToolResultError("edits must be an array"), nil
		}

		var edits []tools.TextEdit
		for _, editItem := range editsArray {
			editMap, ok := editItem.(map[string]any)
			if !ok {
				return mcp.NewToolResultError("each edit must be an object"), nil
			}

			startLine, ok := editMap["startLine"].(float64)
			if !ok {
				return mcp.NewToolResultError("startLine must be a number"), nil
			}

			endLine, ok := editMap["endLine"].(float64)
			if !ok {
				return mcp.NewToolResultError("endLine must be a number"), nil
			}

			newText, _ := editMap["newText"].(string) // newText can be empty

			edits = append(edits, tools.TextEdit{
				StartLine: int(startLine),
				EndLine:   int(endLine),
				NewText:   newText,
			})
		}

		coreLogger.Debug("Executing edit_file for file: %s", filePath)
		response, err := tools.ApplyTextEdits(s.ctx, s.lspClient, filePath, edits)
		if err != nil {
			coreLogger.Error("Failed to apply edits: %v", err)
			return mcp.NewToolResultError(fmt.Sprintf("failed to apply edits: %v", err)), nil
		}
		return mcp.NewToolResultText(response), nil
	})

	getDiagnosticsTool := mcp.NewTool("diagnostics",
		mcp.WithDescription("Get diagnostic information for a specific file from the language server."),
		mcp.WithTitleAnnotation("Get Diagnostics"),
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithString("filePath",
			mcp.Required(),
			mcp.Description("The path to the file to get diagnostics for"),
		),
		mcp.WithNumber("contextLines",
			mcp.Description("Number of lines to include around each diagnostic. Defaults to 5; set to 0 to disable. Overridden by LSP_CONTEXT_LINES env var."),
			mcp.DefaultNumber(5),
		),
		mcp.WithBoolean("showLineNumbers",
			mcp.Description("If true, adds line numbers to the output"),
			mcp.DefaultBool(true),
		),
	)

	s.addTool(getDiagnosticsTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		filePath, err := request.RequireString("filePath")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		// contextLines is declared as a number, but accept a bool too for
		// back-compat with the old schema: true → default count (5), false → 0.
		// GetInt handles int/float64/string but not bool; check the raw value
		// for that case first.
		contextLines := 5
		if raw, ok := request.GetArguments()["contextLines"]; ok {
			if b, ok := raw.(bool); ok {
				if !b {
					contextLines = 0
				}
			} else {
				contextLines = request.GetInt("contextLines", 5)
			}
		}

		showLineNumbers := request.GetBool("showLineNumbers", true)

		coreLogger.Debug("Executing diagnostics for file: %s", filePath)
		text, err := tools.GetDiagnosticsForFile(s.ctx, s.lspClient, filePath, contextLines, showLineNumbers)
		if err != nil {
			coreLogger.Error("Failed to get diagnostics: %v", err)
			return mcp.NewToolResultError(fmt.Sprintf("failed to get diagnostics: %v", err)), nil
		}
		return mcp.NewToolResultText(text), nil
	})

	coreLogger.Debug("Always-on MCP tools registered")
}

// registerCapabilityTools registers tools that are gated on what the LSP
// advertised in its initialize response. Called from the background LSP
// init goroutine after the handshake completes; mcp-go emits
// tools/list_changed so connected clients pick the new tools up live.
func (s *mcpServer) registerCapabilityTools(caps *protocol.ServerCapabilities) {
	// Nil capabilities means the LSP didn't return anything we could parse.
	if caps == nil {
		coreLogger.Warn("No server capabilities — skipping capability-gated tool registration")
		return
	}

	coreLogger.Info("LSP capabilities: definition=%v references=%v hover=%v rename=%v documentSymbol=%v codeAction=%v formatting=%v semanticTokens=%v",
		lsp.HasDefinitionSupport(caps),
		lsp.HasReferencesSupport(caps),
		lsp.HasHoverSupport(caps),
		lsp.HasRenameSupport(caps),
		lsp.HasDocumentSymbolSupport(caps),
		lsp.HasCodeActionSupport(caps),
		lsp.HasFormattingSupport(caps),
		lsp.HasSemanticTokensSupport(caps),
	)

	if lsp.HasDefinitionSupport(caps) {
		readDefinitionTool := mcp.NewTool("definition",
			mcp.WithDescription("Read the source code definition of a symbol (function, type, constant, etc.) from the codebase. Returns the complete implementation code where the symbol is defined."),
			mcp.WithTitleAnnotation("Go to Definition"),
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithString("symbolName",
				mcp.Required(),
				mcp.Description("The name of the symbol whose definition you want to find (e.g. 'mypackage.MyFunction', 'MyType.MyMethod')"),
			),
		)

		s.addTool(readDefinitionTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			symbolName, err := request.RequireString("symbolName")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			coreLogger.Debug("Executing definition for symbol: %s", symbolName)
			text, err := tools.ReadDefinition(s.ctx, s.lspClient, symbolName)
			if err != nil {
				coreLogger.Error("Failed to get definition: %v", err)
				return mcp.NewToolResultError(fmt.Sprintf("failed to get definition: %v", err)), nil
			}
			return mcp.NewToolResultText(text), nil
		})

		definitionAtPositionTool := mcp.NewTool("definition_at_position",
			mcp.WithDescription("Read the source code definition of the symbol at the given file/line/column. Uses textDocument/definition directly (no workspace/symbol fan-out), so it disambiguates same-named symbols by call site and avoids build-output duplicates."),
			mcp.WithTitleAnnotation("Go to Definition (Positional)"),
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithString("filePath",
				mcp.Required(),
				mcp.Description("The path to the file containing the symbol reference"),
			),
			mcp.WithNumber("line",
				mcp.Required(),
				mcp.Description("The line number of the symbol reference (1-indexed)"),
			),
			mcp.WithNumber("column",
				mcp.Required(),
				mcp.Description("The column number of the symbol reference (1-indexed)"),
			),
		)

		s.addTool(definitionAtPositionTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			filePath, err := request.RequireString("filePath")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			line, err := request.RequireInt("line")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			column, err := request.RequireInt("column")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			coreLogger.Debug("Executing definition_at_position for %s:%d:%d", filePath, line, column)
			text, err := tools.ReadDefinitionAtPosition(s.ctx, s.lspClient, filePath, line, column)
			if err != nil {
				coreLogger.Error("Failed to get definition: %v", err)
				return mcp.NewToolResultError(fmt.Sprintf("failed to get definition: %v", err)), nil
			}
			return mcp.NewToolResultText(text), nil
		})
	} else {
		coreLogger.Info("Skipping 'definition' tool — LSP lacks definition or workspace/symbol")
	}

	if lsp.HasReferencesSupport(caps) {
		findReferencesTool := mcp.NewTool("references",
			mcp.WithDescription("Find all usages and references of a symbol throughout the codebase. Returns a list of all files and locations where the symbol appears."),
			mcp.WithTitleAnnotation("Find References"),
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithString("symbolName",
				mcp.Required(),
				mcp.Description("The name of the symbol to search for (e.g. 'mypackage.MyFunction', 'MyType')"),
			),
		)

		s.addTool(findReferencesTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			symbolName, err := request.RequireString("symbolName")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			coreLogger.Debug("Executing references for symbol: %s", symbolName)
			text, err := tools.FindReferences(s.ctx, s.lspClient, symbolName)
			if err != nil {
				coreLogger.Error("Failed to find references: %v", err)
				return mcp.NewToolResultError(fmt.Sprintf("failed to find references: %v", err)), nil
			}
			return mcp.NewToolResultText(text), nil
		})

		referencesAtPositionTool := mcp.NewTool("references_at_position",
			mcp.WithDescription("Find all references to the symbol at the given file/line/column. Uses textDocument/references directly (no workspace/symbol fan-out), so it disambiguates same-named symbols and avoids duplicated reference sets when multiple workspace symbols share a name."),
			mcp.WithTitleAnnotation("Find References (Positional)"),
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithString("filePath",
				mcp.Required(),
				mcp.Description("The path to the file containing the symbol"),
			),
			mcp.WithNumber("line",
				mcp.Required(),
				mcp.Description("The line number of the symbol (1-indexed)"),
			),
			mcp.WithNumber("column",
				mcp.Required(),
				mcp.Description("The column number of the symbol (1-indexed)"),
			),
		)

		s.addTool(referencesAtPositionTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			filePath, err := request.RequireString("filePath")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			line, err := request.RequireInt("line")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			column, err := request.RequireInt("column")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			coreLogger.Debug("Executing references_at_position for %s:%d:%d", filePath, line, column)
			text, err := tools.FindReferencesAtPosition(s.ctx, s.lspClient, filePath, line, column)
			if err != nil {
				coreLogger.Error("Failed to find references: %v", err)
				return mcp.NewToolResultError(fmt.Sprintf("failed to find references: %v", err)), nil
			}
			return mcp.NewToolResultText(text), nil
		})
	} else {
		coreLogger.Info("Skipping 'references' tool — LSP lacks references or workspace/symbol")
	}

	if lsp.HasHoverSupport(caps) {
		hoverTool := mcp.NewTool("hover",
			mcp.WithDescription("Get hover information (type, documentation) for a symbol at the specified position."),
			mcp.WithTitleAnnotation("Hover Information"),
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithString("filePath",
				mcp.Required(),
				mcp.Description("The path to the file to get hover information for"),
			),
			mcp.WithNumber("line",
				mcp.Required(),
				mcp.Description("The line number where the hover is requested (1-indexed)"),
			),
			mcp.WithNumber("column",
				mcp.Required(),
				mcp.Description("The column number where the hover is requested (1-indexed)"),
			),
		)

		s.addTool(hoverTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			filePath, err := request.RequireString("filePath")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			line, err := request.RequireInt("line")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			column, err := request.RequireInt("column")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			coreLogger.Debug("Executing hover for file: %s line: %d column: %d", filePath, line, column)
			text, err := tools.GetHoverInfo(s.ctx, s.lspClient, filePath, line, column)
			if err != nil {
				coreLogger.Error("Failed to get hover information: %v", err)
				return mcp.NewToolResultError(fmt.Sprintf("failed to get hover information: %v", err)), nil
			}
			return mcp.NewToolResultText(text), nil
		})
	} else {
		coreLogger.Info("Skipping 'hover' tool — LSP lacks hover capability")
	}

	if lsp.HasRenameSupport(caps) {
		renameSymbolTool := mcp.NewTool("rename_symbol",
			mcp.WithDescription("Rename a symbol (variable, function, class, etc.) at the specified position and update all references throughout the codebase."),
			mcp.WithTitleAnnotation("Rename Symbol"),
			mcp.WithDestructiveHintAnnotation(true),
			mcp.WithString("filePath",
				mcp.Required(),
				mcp.Description("The path to the file containing the symbol to rename"),
			),
			mcp.WithNumber("line",
				mcp.Required(),
				mcp.Description("The line number where the symbol is located (1-indexed)"),
			),
			mcp.WithNumber("column",
				mcp.Required(),
				mcp.Description("The column number where the symbol is located (1-indexed)"),
			),
			mcp.WithString("newName",
				mcp.Required(),
				mcp.Description("The new name for the symbol"),
			),
		)

		s.addTool(renameSymbolTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			filePath, err := request.RequireString("filePath")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			newName, err := request.RequireString("newName")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			line, err := request.RequireInt("line")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			column, err := request.RequireInt("column")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			coreLogger.Debug("Executing rename_symbol for file: %s line: %d column: %d newName: %s", filePath, line, column, newName)
			text, err := tools.RenameSymbol(s.ctx, s.lspClient, filePath, line, column, newName)
			if err != nil {
				coreLogger.Error("Failed to rename symbol: %v", err)
				return mcp.NewToolResultError(fmt.Sprintf("failed to rename symbol: %v", err)), nil
			}
			return mcp.NewToolResultText(text), nil
		})
	} else {
		coreLogger.Info("Skipping 'rename_symbol' tool — LSP lacks rename capability")
	}

	if lsp.HasDocumentSymbolSupport(caps) {
		documentSymbolsTool := mcp.NewTool("document_symbols",
			mcp.WithDescription("Get the hierarchical symbol outline of a file (classes, functions, methods, etc.)"),
			mcp.WithTitleAnnotation("Document Symbols"),
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithString("filePath",
				mcp.Required(),
				mcp.Description("Path to the file to get symbols for"),
			),
		)

		s.addTool(documentSymbolsTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			filePath, err := request.RequireString("filePath")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			coreLogger.Debug("Executing document_symbols for file: %s", filePath)
			text, err := tools.GetDocumentSymbols(s.ctx, s.lspClient, filePath)
			if err != nil {
				coreLogger.Error("Failed to get document symbols: %v", err)
				return mcp.NewToolResultError(fmt.Sprintf("failed to get document symbols: %v", err)), nil
			}
			return mcp.NewToolResultText(text), nil
		})
	} else {
		coreLogger.Info("Skipping 'document_symbols' tool — LSP lacks documentSymbol capability")
	}

	if lsp.HasCodeActionSupport(caps) {
		codeActionsTool := mcp.NewTool("code_actions",
			mcp.WithDescription("Get available code actions (quick fixes, refactorings, source actions) for a range in a file."),
			mcp.WithTitleAnnotation("Code Actions"),
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithString("filePath",
				mcp.Required(),
				mcp.Description("Path to the file"),
			),
			mcp.WithNumber("startLine",
				mcp.Required(),
				mcp.Description("Start line (1-indexed)"),
			),
			mcp.WithNumber("startColumn",
				mcp.Required(),
				mcp.Description("Start column (1-indexed)"),
			),
			mcp.WithNumber("endLine",
				mcp.Required(),
				mcp.Description("End line (1-indexed)"),
			),
			mcp.WithNumber("endColumn",
				mcp.Required(),
				mcp.Description("End column (1-indexed)"),
			),
		)

		s.addTool(codeActionsTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			filePath, err := request.RequireString("filePath")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			startLine, err := request.RequireInt("startLine")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			startColumn, err := request.RequireInt("startColumn")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			endLine, err := request.RequireInt("endLine")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			endColumn, err := request.RequireInt("endColumn")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			coreLogger.Debug("Executing code_actions for %s [%d:%d-%d:%d]", filePath, startLine, startColumn, endLine, endColumn)
			text, err := tools.GetCodeActions(s.ctx, s.lspClient, filePath, startLine, startColumn, endLine, endColumn)
			if err != nil {
				coreLogger.Error("Failed to get code actions: %v", err)
				return mcp.NewToolResultError(fmt.Sprintf("failed to get code actions: %v", err)), nil
			}
			return mcp.NewToolResultText(text), nil
		})
	} else {
		coreLogger.Info("Skipping 'code_actions' tool — LSP lacks codeAction capability")
	}

	if lsp.HasFormattingSupport(caps) {
		formatDocumentTool := mcp.NewTool("format_document",
			mcp.WithDescription("Format a document (or a range within it) using the LSP server's formatter. Applies the resulting edits to disk."),
			mcp.WithTitleAnnotation("Format Document"),
			mcp.WithDestructiveHintAnnotation(true),
			mcp.WithString("filePath",
				mcp.Required(),
				mcp.Description("Path to the file to format"),
			),
			mcp.WithString("mode",
				mcp.Description("'full' (default), 'range', or 'ontype'"),
				mcp.DefaultString("full"),
			),
			mcp.WithNumber("startLine",
				mcp.Description("Start line for range/ontype mode (1-indexed)"),
			),
			mcp.WithNumber("startColumn",
				mcp.Description("Start column for range/ontype mode (1-indexed)"),
			),
			mcp.WithNumber("endLine",
				mcp.Description("End line for range mode (1-indexed)"),
			),
			mcp.WithNumber("endColumn",
				mcp.Description("End column for range mode (1-indexed)"),
			),
			mcp.WithString("triggerChar",
				mcp.Description("Trigger character for ontype mode"),
			),
		)

		s.addTool(formatDocumentTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			filePath, err := request.RequireString("filePath")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			mode := request.GetString("mode", "full")
			triggerChar := request.GetString("triggerChar", "")

			coreLogger.Debug("Executing format_document for %s (mode=%s)", filePath, mode)
			text, err := tools.FormatDocument(s.ctx, s.lspClient, filePath, mode,
				request.GetInt("startLine", 0), request.GetInt("startColumn", 0),
				request.GetInt("endLine", 0), request.GetInt("endColumn", 0),
				triggerChar)
			if err != nil {
				coreLogger.Error("Failed to format document: %v", err)
				return mcp.NewToolResultError(fmt.Sprintf("failed to format document: %v", err)), nil
			}
			return mcp.NewToolResultText(text), nil
		})
	} else {
		coreLogger.Info("Skipping 'format_document' tool — LSP lacks documentFormatting capability")
	}

	if lsp.HasSemanticTokensSupport(caps) {
		semanticTokensTool := mcp.NewTool("semantic_tokens",
			mcp.WithDescription("Dump the full semantic-tokens response from the LSP server, decoded with the server's token-type / token-modifier legend. Intended for debugging LSP semantic token providers."),
			mcp.WithTitleAnnotation("Semantic Tokens"),
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithString("filePath",
				mcp.Required(),
				mcp.Description("Path to the file to inspect"),
			),
		)

		s.addTool(semanticTokensTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			filePath, err := request.RequireString("filePath")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			coreLogger.Debug("Executing semantic_tokens for %s", filePath)
			text, err := tools.GetSemanticTokens(s.ctx, s.lspClient, caps, filePath)
			if err != nil {
				coreLogger.Error("Failed to get semantic tokens: %v", err)
				return mcp.NewToolResultError(fmt.Sprintf("failed to get semantic tokens: %v", err)), nil
			}
			return mcp.NewToolResultText(text), nil
		})
	} else {
		coreLogger.Info("Skipping 'semantic_tokens' tool — LSP lacks semanticTokens capability")
	}

	if len(s.config.disabledTools) > 0 {
		s.mcpServer.DeleteTools(s.config.disabledTools...)
		coreLogger.Info("Disabled tools: %v", s.config.disabledTools)
	}

	coreLogger.Info("Capability-gated MCP tools registered")
}
