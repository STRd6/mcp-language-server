package lsp

import (
	"testing"

	"github.com/STRd6/mcp-language-server/internal/protocol"
)

func TestDetectLanguageID(t *testing.T) {
	tests := []struct {
		uri  string
		want protocol.LanguageKind
	}{
		// Languages the integration tests exercise.
		{"foo.go", protocol.LangGo},
		{"foo.py", protocol.LangPython},
		{"foo.ts", protocol.LangTypeScript},
		{"foo.tsx", protocol.LangTypeScriptReact},
		{"foo.rs", protocol.LangRust},
		{"foo.c", protocol.LangC},
		{"foo.cpp", protocol.LangCPP},
		{"foo.civet", protocol.LanguageKind("civet")},

		// Multi-extension aliases — verify each variant maps the same way.
		{"foo.cxx", protocol.LangCPP},
		{"foo.cc", protocol.LangCPP},
		{"foo.c++", protocol.LangCPP},
		{"foo.bib", protocol.LangBibTeX},
		{"foo.bibtex", protocol.LangBibTeX},
		{"foo.ex", protocol.LangElixir},
		{"foo.exs", protocol.LangElixir},
		{"foo.erl", protocol.LangErlang},
		{"foo.hrl", protocol.LangErlang},
		{"foo.fs", protocol.LangFSharp},
		{"foo.fsi", protocol.LangFSharp},
		{"foo.fsx", protocol.LangFSharp},
		{"foo.fsscript", protocol.LangFSharp},
		{"foo.hbs", protocol.LangHandlebars},
		{"foo.handlebars", protocol.LangHandlebars},
		{"foo.html", protocol.LangHTML},
		{"foo.htm", protocol.LangHTML},
		{"foo.tex", protocol.LangLaTeX},
		{"foo.latex", protocol.LangLaTeX},
		{"foo.md", protocol.LangMarkdown},
		{"foo.markdown", protocol.LangMarkdown},
		{"foo.pas", protocol.LangDelphi},
		{"foo.pascal", protocol.LangDelphi},
		{"foo.diff", protocol.LangDiff},
		{"foo.patch", protocol.LangDiff},
		{"foo.ps1", protocol.LangPowershell},
		{"foo.psm1", protocol.LangPowershell},
		{"foo.pug", protocol.LangPug},
		{"foo.jade", protocol.LangPug},
		{"foo.cshtml", protocol.LangRazor},
		{"foo.razor", protocol.LangRazor},
		{"foo.sh", protocol.LangShellScript},
		{"foo.bash", protocol.LangShellScript},
		{"foo.zsh", protocol.LangShellScript},
		{"foo.ksh", protocol.LangShellScript},
		{"foo.yaml", protocol.LangYAML},
		{"foo.yml", protocol.LangYAML},

		// Single-extension languages — one entry each.
		{"foo.abap", protocol.LangABAP},
		{"foo.bat", protocol.LangWindowsBat},
		{"foo.hera", protocol.LanguageKind("hera")},
		{"foo.clj", protocol.LangClojure},
		{"foo.coffee", protocol.LangCoffeescript},
		{"foo.cs", protocol.LangCSharp},
		{"foo.css", protocol.LangCSS},
		{"foo.d", protocol.LangD},
		{"foo.dart", protocol.LangDart},
		{"foo.dockerfile", protocol.LangDockerfile},
		{"foo.gitcommit", protocol.LangGitCommit},
		{"foo.gitrebase", protocol.LangGitRebase},
		{"foo.groovy", protocol.LangGroovy},
		{"foo.hs", protocol.LangHaskell},
		{"foo.ini", protocol.LangIni},
		{"foo.java", protocol.LangJava},
		{"foo.js", protocol.LangJavaScript},
		{"foo.jsx", protocol.LangJavaScriptReact},
		{"foo.json", protocol.LangJSON},
		{"foo.less", protocol.LangLess},
		{"foo.lua", protocol.LangLua},
		{"foo.makefile", protocol.LangMakefile},
		{"foo.m", protocol.LangObjectiveC},
		{"foo.mm", protocol.LangObjectiveCPP},
		{"foo.pl", protocol.LangPerl},
		{"foo.pm", protocol.LangPerl6},
		{"foo.php", protocol.LangPHP},
		{"foo.r", protocol.LangR},
		{"foo.rb", protocol.LangRuby},
		{"foo.scss", protocol.LangSCSS},
		{"foo.sass", protocol.LangSASS},
		{"foo.scala", protocol.LangScala},
		{"foo.shader", protocol.LangShaderLab},
		{"foo.sql", protocol.LangSQL},
		{"foo.swift", protocol.LangSwift},
		{"foo.v", protocol.LanguageKind("coq")},
		{"foo.xml", protocol.LangXML},
		{"foo.xsl", protocol.LangXSL},

		// Case-insensitive extension matching.
		{"foo.GO", protocol.LangGo},
		{"foo.Py", protocol.LangPython},

		// Path stripped down to extension only.
		{"file:///abs/path/to/foo.rs", protocol.LangRust},

		// Unknown / no extension returns empty.
		{"foo.unknownext", protocol.LanguageKind("")},
		{"foo", protocol.LanguageKind("")},
		{"", protocol.LanguageKind("")},
	}

	for _, tc := range tests {
		if got := DetectLanguageID(tc.uri); got != tc.want {
			t.Errorf("DetectLanguageID(%q) = %q, want %q", tc.uri, got, tc.want)
		}
	}
}
