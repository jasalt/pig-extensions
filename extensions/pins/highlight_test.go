// Copyright (c) 2026 Jarkko Saltiola
// SPDX-License-Identifier: MIT

package pins

import (
	"strings"
	"testing"

	sdk "github.com/MichaelKinsy/PiG/extensions/sdk"
	"github.com/alecthomas/chroma/v2"
)

func TestSyntaxTokenMapping(t *testing.T) {
	for tokenType, want := range map[chroma.TokenType]string{
		chroma.Keyword:              "syntaxKeyword",
		chroma.KeywordType:          "syntaxType",
		chroma.LiteralStringDouble:  "syntaxString",
		chroma.LiteralNumberInteger: "syntaxNumber",
		chroma.CommentSingle:        "syntaxComment",
		chroma.Operator:             "syntaxOperator",
		chroma.Punctuation:          "syntaxPunctuation",
		chroma.NameFunction:         "syntaxFunction",
		chroma.NameClass:            "syntaxType",
		chroma.NameVariable:         "syntaxVariable",
		chroma.NameDecorator:        "muted",
		chroma.NameTag:              "syntaxKeyword",
		chroma.GenericInserted:      "toolDiffAdded",
		chroma.GenericDeleted:       "toolDiffRemoved",
		chroma.Name:                 "",
		chroma.Text:                 "",
	} {
		if got := syntaxToken(tokenType); got != want {
			t.Errorf("%v: %q, want %q", tokenType, got, want)
		}
	}
}

func TestHighlightPreservesLinesAndFallsBack(t *testing.T) {
	h := highlighter{theme: sdk.UITheme{}} // no colors: output is the source text
	for _, tc := range []struct{ code, lang string }{
		{"func main() {\n\tprintln(\"x\")\n}", "go"},
		{"a = `multi\nline`", "js"},
		{"plain\n\ntext", ""},
		{"x", "no-such-language"},
		{"echo $HOME", "sh"},
	} {
		lines := h.Highlight(tc.code, tc.lang)
		if strings.Join(lines, "\n") != tc.code {
			t.Errorf("%s: %q", tc.lang, lines)
		}
	}
}
