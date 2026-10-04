// Copyright (c) 2025 Alessandro Pungitore
// Copyright (c) 2026 Jarkko Saltiola
// SPDX-License-Identifier: MIT

package schedule

import "regexp"

// Conservative redaction for shell output persisted to disk or the session.
// The transient agent follow-up keeps full output; the command stays
// verbatim because it must re-run.

var (
	knownTokens = regexp.MustCompile(`\b(?:ghp_[A-Za-z0-9]{36,}|gho_[A-Za-z0-9]{36,}|ghu_[A-Za-z0-9]{36,}|ghs_[A-Za-z0-9]{36,}|github_pat_[A-Za-z0-9_]{22,}|npm_[A-Za-z0-9]{36,}|sk-[A-Za-z0-9_-]{20,}|sk_live_[A-Za-z0-9]{10,}|rk_live_[A-Za-z0-9]{10,}|xox[baprs]-[A-Za-z0-9-]{10,}|AKIA[0-9A-Z]{16}|AIza[0-9A-Za-z_-]{35}|eyJhbGciOi[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{5,})\b`)
	authScheme  = regexp.MustCompile(`(?i)\b((?:Bearer|Basic|Digest)\s+)[A-Za-z0-9._~+/=-]{16,}`)
	// The original uses a quote backreference; RE2 has none, so the quoted
	// forms are explicit alternatives tried before the unquoted one, which is
	// the same leftmost-first result.
	assignment = regexp.MustCompile(`(?i)\b([A-Za-z0-9_-]*(?:api[_-]?key|apikey|secret|token|password|passwd|credential|authorization)[A-Za-z0-9_-]*["']?\s*[:=]\s*)(?:"([^\s"',;\\]{8,})"|'([^\s"',;\\]{8,})'|([^\s"',;\\]{8,}))`)
)

func redactSecrets(text string) string {
	if text == "" {
		return text
	}
	text = authScheme.ReplaceAllString(text, "${1}[REDACTED]")
	var out []byte
	last := 0
	for _, m := range assignment.FindAllStringSubmatchIndex(text, -1) {
		out = append(out, text[last:m[3]]...)
		switch {
		case m[4] >= 0:
			out = append(out, `"[REDACTED]"`...)
		case m[6] >= 0:
			out = append(out, `'[REDACTED]'`...)
		default:
			out = append(out, "[REDACTED]"...)
		}
		last = m[1]
	}
	text = string(append(out, text[last:]...))
	return knownTokens.ReplaceAllString(text, "[REDACTED]")
}
