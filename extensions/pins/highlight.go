// Copyright Hewlett Packard Enterprise Development LP
// Copyright (c) 2026 Jarkko Saltiola
// SPDX-License-Identifier: MIT

package pins

// The lexer-to-theme mapping below is adapted from PiG v0.3.1
// tui/highlight.go (MIT, Hewlett Packard Enterprise Development LP). PiG's
// public tui.HighlightCode reads the process-global active theme, which in an
// extension process is not the host theme, and mutating that global would
// affect other packed or fused extensions. This copy takes colors from the
// host theme snapshot instead.

import (
	"strings"

	sdk "github.com/MichaelKinsy/PiG/extensions/sdk"
	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"
)

// highlighter highlights code with one host theme snapshot.
type highlighter struct{ theme sdk.UITheme }

// Highlight returns one styled line per source line; an empty or unknown
// language uses the code-block color, as PiG's markdown default does.
func (h highlighter) Highlight(code, lang string) []string {
	if lang != "" {
		if lines, ok := h.highlightLexed(code, lang); ok && len(lines) == strings.Count(code, "\n")+1 {
			return lines
		}
	}
	return h.fallback(code)
}

func (h highlighter) fallback(code string) []string {
	lines := strings.Split(code, "\n")
	for i, line := range lines {
		if line != "" {
			lines[i] = h.theme.Fg("mdCodeBlock", line)
		}
	}
	return lines
}

func (h highlighter) highlightLexed(code, lang string) ([]string, bool) {
	lexer := lexers.Get(lang)
	if lexer == nil {
		lexer = lexers.Match("file." + lang)
	}
	if lexer == nil {
		return nil, false
	}
	lexer = chroma.Coalesce(lexer)
	tokens, err := lexer.Tokenise(nil, code)
	if err != nil {
		return nil, false
	}
	var buf strings.Builder
	emit := func(token, value string) {
		if token == "" {
			buf.WriteString(value)
			return
		}
		segments := strings.Split(value, "\n")
		for i, segment := range segments {
			if segment != "" {
				buf.WriteString(h.theme.Fg(token, segment))
			}
			if i < len(segments)-1 {
				buf.WriteByte('\n')
			}
		}
	}
	depth := 0
	var substitution strings.Builder
	flush := func() {
		if substitution.Len() > 0 {
			emit("syntaxString", substitution.String())
			substitution.Reset()
		}
	}
	for token := tokens(); token != chroma.EOF; token = tokens() {
		color := syntaxToken(token.Type)
		value := token.Value
		if token.Type == chroma.LiteralStringInterpol && (depth > 0 || strings.HasSuffix(value, "{") || strings.HasSuffix(value, `\(`)) {
			for _, r := range value {
				switch r {
				case '{', '(':
					depth++
				case '}', ')':
					if depth > 0 {
						depth--
					}
				}
			}
			substitution.WriteString(value)
			if depth == 0 {
				flush()
			}
			continue
		}
		if depth > 0 {
			if color == "" {
				substitution.WriteString(value)
				continue
			}
			flush()
		}
		emit(color, value)
	}
	flush()
	highlighted := buf.String()
	if lexer.Config().EnsureNL && !strings.HasSuffix(code, "\n") {
		highlighted = strings.TrimSuffix(highlighted, "\n")
	}
	return strings.Split(highlighted, "\n"), true
}

// syntaxToken maps lexer categories to Pi theme color tokens.
func syntaxToken(t chroma.TokenType) string {
	switch t {
	case chroma.GenericInserted:
		return "toolDiffAdded"
	case chroma.GenericDeleted:
		return "toolDiffRemoved"
	case chroma.NameDecorator:
		return "muted"
	case chroma.NameTag:
		return "syntaxKeyword"
	}
	switch t.SubCategory() {
	case chroma.LiteralString:
		return "syntaxString"
	case chroma.LiteralNumber:
		return "syntaxNumber"
	}
	switch t.Category() {
	case chroma.Comment:
		return "syntaxComment"
	case chroma.Keyword:
		if t == chroma.KeywordType {
			return "syntaxType"
		}
		return "syntaxKeyword"
	case chroma.Operator:
		return "syntaxOperator"
	case chroma.Punctuation:
		return "syntaxPunctuation"
	case chroma.Name:
		switch t {
		case chroma.NameFunction, chroma.NameFunctionMagic, chroma.NameBuiltin:
			return "syntaxFunction"
		case chroma.NameClass, chroma.NameNamespace:
			return "syntaxType"
		case chroma.NameVariable, chroma.NameVariableClass, chroma.NameVariableGlobal,
			chroma.NameVariableInstance, chroma.NameAttribute:
			return "syntaxVariable"
		}
	}
	return ""
}
