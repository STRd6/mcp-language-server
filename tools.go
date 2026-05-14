package main

import (
	"context"
	"fmt"

	"github.com/isaacphi/mcp-language-server/internal/lsp"
	"github.com/isaacphi/mcp-language-server/internal/protocol"
	"github.com/isaacphi/mcp-language-server/internal/tools"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// addTool registers a tool whose handler blocks on waitForLSP. ServeStdio
// starts before the LSP handshake completes (see start()), so tool calls
// that arrive during LSP startup wait here instead of erroring or stalling
// the whole MCP connection.
func (s *mcpServer) addTool(tool mcp.Tool, handler server.ToolHandlerFunc) {
	s.addTool(tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
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
		// Extract arguments
		filePath, ok := request.Params.Arguments["filePath"].(string)
		if !ok {
			return mcp.NewToolResultError("filePath must be a string"), nil
		}

		// Extract edits array
		editsArg, ok := request.Params.Arguments["edits"]
		if !ok {
			return mcp.NewToolResultError("edits is required"), nil
		}

		// Type assert and convert the edits
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
		mcp.WithBoolean("contextLines",
			mcp.Description("Lines to include around each diagnostic."),
			mcp.DefaultBool(false),
		),
		mcp.WithBoolean("showLineNumbers",
			mcp.Description("If true, adds line numbers to the output"),
			mcp.DefaultBool(true),
		),
	)

	s.addTool(getDiagnosticsTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		// Extract arguments
		filePath, ok := request.Params.Arguments["filePath"].(string)
		if !ok {
			return mcp.NewToolResultError("filePath must be a string"), nil
		}

		contextLines := 5 // default value
		if contextLinesArg, ok := request.Params.Arguments["contextLines"].(int); ok {
			contextLines = contextLinesArg
		}

		showLineNumbers := true // default value
		if showLineNumbersArg, ok := request.Params.Arguments["showLineNumbers"].(bool); ok {
			showLineNumbers = showLineNumbersArg
		}

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
			symbolName, ok := request.Params.Arguments["symbolName"].(string)
			if !ok {
				return mcp.NewToolResultError("symbolName must be a string"), nil
			}

			coreLogger.Debug("Executing definition for symbol: %s", symbolName)
			text, err := tools.ReadDefinition(s.ctx, s.lspClient, symbolName)
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
			symbolName, ok := request.Params.Arguments["symbolName"].(string)
			if !ok {
				return mcp.NewToolResultError("symbolName must be a string"), nil
			}

			coreLogger.Debug("Executing references for symbol: %s", symbolName)
			text, err := tools.FindReferences(s.ctx, s.lspClient, symbolName)
			if err != nil {
				coreLogger.Error("Failed to find references: %v", err)
				return mcp.NewToolResultError(fmt.Sprintf("failed to find references: %v", err)), nil
			}
			return mcp.NewToolResultText(text), nil
		})
	} else {
		coreLogger.Info("Skipping 'references' tool — LSP lacks references capability")
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
			filePath, ok := request.Params.Arguments["filePath"].(string)
			if !ok {
				return mcp.NewToolResultError("filePath must be a string"), nil
			}

			var line, column int
			switch v := request.Params.Arguments["line"].(type) {
			case float64:
				line = int(v)
			case int:
				line = v
			default:
				return mcp.NewToolResultError("line must be a number"), nil
			}

			switch v := request.Params.Arguments["column"].(type) {
			case float64:
				column = int(v)
			case int:
				column = v
			default:
				return mcp.NewToolResultError("column must be a number"), nil
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
			filePath, ok := request.Params.Arguments["filePath"].(string)
			if !ok {
				return mcp.NewToolResultError("filePath must be a string"), nil
			}

			newName, ok := request.Params.Arguments["newName"].(string)
			if !ok {
				return mcp.NewToolResultError("newName must be a string"), nil
			}

			var line, column int
			switch v := request.Params.Arguments["line"].(type) {
			case float64:
				line = int(v)
			case int:
				line = v
			default:
				return mcp.NewToolResultError("line must be a number"), nil
			}

			switch v := request.Params.Arguments["column"].(type) {
			case float64:
				column = int(v)
			case int:
				column = v
			default:
				return mcp.NewToolResultError("column must be a number"), nil
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
			filePath, ok := request.Params.Arguments["filePath"].(string)
			if !ok {
				return mcp.NewToolResultError("filePath must be a string"), nil
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
			filePath, ok := request.Params.Arguments["filePath"].(string)
			if !ok {
				return mcp.NewToolResultError("filePath must be a string"), nil
			}

			coord := func(name string) (int, *mcp.CallToolResult) {
				switch v := request.Params.Arguments[name].(type) {
				case float64:
					return int(v), nil
				case int:
					return v, nil
				default:
					return 0, mcp.NewToolResultError(fmt.Sprintf("%s must be a number", name))
				}
			}

			startLine, errRes := coord("startLine")
			if errRes != nil {
				return errRes, nil
			}
			startColumn, errRes := coord("startColumn")
			if errRes != nil {
				return errRes, nil
			}
			endLine, errRes := coord("endLine")
			if errRes != nil {
				return errRes, nil
			}
			endColumn, errRes := coord("endColumn")
			if errRes != nil {
				return errRes, nil
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
			filePath, ok := request.Params.Arguments["filePath"].(string)
			if !ok {
				return mcp.NewToolResultError("filePath must be a string"), nil
			}

			mode, _ := request.Params.Arguments["mode"].(string)
			if mode == "" {
				mode = "full"
			}
			triggerChar, _ := request.Params.Arguments["triggerChar"].(string)

			optInt := func(name string) int {
				switch v := request.Params.Arguments[name].(type) {
				case float64:
					return int(v)
				case int:
					return v
				default:
					return 0
				}
			}

			coreLogger.Debug("Executing format_document for %s (mode=%s)", filePath, mode)
			text, err := tools.FormatDocument(s.ctx, s.lspClient, filePath, mode,
				optInt("startLine"), optInt("startColumn"),
				optInt("endLine"), optInt("endColumn"),
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
			filePath, ok := request.Params.Arguments["filePath"].(string)
			if !ok {
				return mcp.NewToolResultError("filePath must be a string"), nil
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
