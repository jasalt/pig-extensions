// Copyright (c) 2026 Jarkko Saltiola
// SPDX-License-Identifier: MIT
// Adapted from jasalt/chatgpt-openai-api-adapter, commit
// 94f45568b4bd7842b1aef362cc3ba883b1312951, contrib/pi-codex-usage.ts.
package codexusage

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"math/big"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"
)

const statusKey = "codex-usage"
const weekSeconds = 7 * 24 * 60 * 60
const refreshInterval = 5 * time.Minute

type window struct{ Used, Seconds, Reset float64 }
type account struct{ Email, Plan string }
type status struct {
	Account         account
	Primary, Weekly *window
}
type credit struct{ ID, Status, Granted, Expires string }
type resetResult struct {
	Result  string
	Windows float64
}

func stringValue(v any) string    { s, _ := v.(string); return s }
func object(v any) map[string]any { m, _ := v.(map[string]any); return m }
func finite(v any) (float64, bool) {
	var n float64
	switch v := v.(type) {
	case float64:
		n = v
	case json.Number:
		var err error
		n, err = strconv.ParseFloat(string(v), 64)
		if err != nil {
			return 0, false
		}
	default:
		return 0, false
	}
	return n, !math.IsNaN(n) && !math.IsInf(n, 0)
}
func validWindow(v any) *window {
	m := object(v)
	used, a := finite(m["used_percent"])
	seconds, b := finite(m["limit_window_seconds"])
	reset, c := finite(m["reset_at"])
	if !a || !b || !c {
		return nil
	}
	return &window{used, seconds, reset}
}
func parseStatus(payload map[string]any, nativeAccount *account) (status, error) {
	rate := object(payload["rate_limit"])
	primary, secondary := validWindow(rate["primary_window"]), validWindow(rate["secondary_window"])
	var weekly *window
	for _, w := range []*window{primary, secondary} {
		if w != nil && math.Abs(w.Seconds-weekSeconds) <= weekSeconds*.05 {
			weekly = w
			break
		}
	}
	if primary == nil && weekly == nil {
		return status{}, errors.New("ChatGPT did not return any Codex usage windows")
	}
	a := object(payload["account"])
	acct := account{stringValue(a["email"]), stringValue(a["plan"])}
	if nativeAccount != nil {
		acct = *nativeAccount
	}
	return status{acct, primary, weekly}, nil
}
func percentLeft(w *window) float64 { return 100 - math.Max(0, math.Min(100, w.Used)) }

// Match JS toFixed(1), including exact binary ties (Go printf rounds ties even).
func formatPercent(n float64) string {
	if math.Trunc(n) == n {
		return strconv.FormatFloat(n, 'f', 0, 64) + "%"
	}
	r := new(big.Rat).SetFloat64(n)
	numerator := new(big.Int).Mul(r.Num(), big.NewInt(10))
	q, rem := new(big.Int), new(big.Int)
	q.QuoRem(numerator, r.Denom(), rem)
	if new(big.Int).Lsh(rem, 1).Cmp(r.Denom()) >= 0 {
		q.Add(q, big.NewInt(1))
	}
	v := q.Int64()
	return fmt.Sprintf("%d.%d%%", v/10, v%10)
}
func durationLabel(w *window) string {
	hours := w.Seconds / 3600
	if math.Trunc(hours) == hours {
		return strconv.FormatFloat(hours, 'f', -1, 64) + "h"
	}
	return "rate"
}
func resetTime(w *window, loc *time.Location) string {
	// ECMAScript Date TimeClip accepts only +/- 8.64e15 milliseconds.
	if math.Abs(w.Reset) > 8.64e12 {
		if w.Seconds >= 86400 {
			return "Invalid Date on Invalid Date"
		}
		return "Invalid Date"
	}
	ms := int64(math.Trunc(w.Reset * 1000))
	date := time.UnixMilli(ms).In(loc)
	result := date.Format("15:04")
	if w.Seconds >= 86400 {
		locale := os.Getenv("LC_ALL")
		if locale == "" {
			locale = os.Getenv("LC_TIME")
		}
		if locale == "" {
			locale = os.Getenv("LANG")
		}
		day := date.Format("Jan 2")
		if strings.HasPrefix(locale, "en_GB") || strings.HasPrefix(locale, "en_AU") {
			day = date.Format("2 Jan")
		}
		result += " on " + day
	}
	return result
}
func compactWindow(w *window, label string, loc *time.Location) string {
	return formatPercent(percentLeft(w)) + "/" + label + " (resets " + resetTime(w, loc) + ")"
}
func formatStatusLine(s status, loc *time.Location) string {
	parts := []string{}
	if s.Primary != nil {
		parts = append(parts, compactWindow(s.Primary, durationLabel(s.Primary), loc))
	}
	if s.Weekly != nil && s.Weekly != s.Primary {
		parts = append(parts, compactWindow(s.Weekly, "w", loc))
	}
	return strings.Join(parts, "  ")
}
func fullWindow(w *window, loc *time.Location) string {
	left := percentLeft(w)
	filled := int(math.Floor(left*20/100 + .5))
	return "[" + strings.Repeat("█", filled) + strings.Repeat("░", 20-filled) + "] " + formatPercent(left) + " left (resets " + resetTime(w, loc) + ")"
}
func titleWord(value string) string {
	runes := []rune(value)
	if len(runes) == 0 {
		return ""
	}
	return strings.ToUpper(string(runes[0])) + strings.ToLower(string(runes[1:]))
}
func formatStatusCard(s status, loc *time.Location) string {
	lines := []string{"ChatGPT Codex status", "Visit https://chatgpt.com/codex/settings/usage for up-to-date information on rate limits and credits."}
	if s.Account.Email != "" {
		plan := ""
		if s.Account.Plan != "" {
			plan = " (" + titleWord(s.Account.Plan) + ")"
		}
		lines = append(lines, "Account:       "+s.Account.Email+plan)
	}
	if s.Primary != nil {
		lines = append(lines, durationLabel(s.Primary)+" limit:      "+fullWindow(s.Primary, loc))
	}
	if s.Weekly != nil && s.Weekly != s.Primary {
		lines = append(lines, "Weekly limit: "+fullWindow(s.Weekly, loc))
	}
	return strings.Join(lines, "\n")
}
func parseDate(s string) (time.Time, bool) {
	for _, layout := range []string{time.RFC3339Nano, time.RFC1123, time.RFC1123Z, "2006-01-02"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t, true
		}
	}
	for _, layout := range []string{"2006-01-02T15:04:05.999999999", "2006-01-02T15:04", "January 2, 2006"} {
		if t, err := time.ParseInLocation(layout, s, time.Local); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}
func parseCredits(payload map[string]any) ([]credit, error) {
	raw := payload["credits"]
	if raw == nil {
		return nil, nil
	}
	rows, ok := raw.([]any)
	if !ok {
		return nil, errors.New("Codex reset credits are not an array")
	}
	credits := make([]credit, 0, len(rows))
	for _, row := range rows {
		m := object(row)
		if m == nil {
			return nil, errors.New("Malformed Codex reset credit")
		}
		credits = append(credits, credit{stringValue(m["id"]), stringValue(m["status"]), stringValue(m["granted_at"]), stringValue(m["expires_at"])})
	}
	sort.SliceStable(credits, func(i, j int) bool {
		l, r := credits[i].Expires, credits[j].Expires
		if l == "" {
			return false
		}
		if r == "" {
			return true
		}
		left, a := parseDate(l)
		right, b := parseDate(r)
		// JS comparator NaN means equal; preserve input order for invalid dates.
		return a && b && left.Before(right)
	})
	return credits, nil
}
func formatCreditTime(value string) string {
	if date, ok := parseDate(value); ok {
		return date.UTC().Format("2006-01-02 15:04") + " UTC"
	}
	return value
}
func formatCredits(credits []credit) string {
	if len(credits) == 0 {
		return "No banked rate-limit reset credits."
	}
	lines := []string{"Banked Codex rate-limit resets:"}
	for _, c := range credits {
		expires := "never/unknown"
		if c.Expires != "" {
			expires = formatCreditTime(c.Expires)
		}
		lines = append(lines, c.ID+"  "+c.Status+"  granted "+formatCreditTime(c.Granted)+"  expires "+expires)
	}
	return strings.Join(append(lines, "Activate one with /codex-reset <reset-id>."), "\n")
}
func resultTruthy(value any) bool {
	switch value := value.(type) {
	case nil:
		return false
	case string:
		return value != ""
	case bool:
		return value
	case float64:
		return value != 0 && !math.IsNaN(value)
	case json.Number:
		n, err := strconv.ParseFloat(string(value), 64)
		if err != nil {
			// Overflow is still truthy in JSON.parse (Infinity). A malformed
			// number must not accidentally turn into an accepted empty result.
			return true
		}
		return n != 0 && !math.IsNaN(n)
	default:
		return true
	}
}

func parseResetResult(payload map[string]any) (resetResult, error) {
	value := payload["result"]
	if !resultTruthy(value) {
		value = payload["status"]
	}
	if !resultTruthy(value) {
		value = "reset"
	}
	result, ok := value.(string)
	if !ok {
		return resetResult{}, errors.New("Activate Codex reset returned unexpected result type")
	}
	switch result {
	case "reset", "already_redeemed", "nothing_to_reset", "no_credit":
	default:
		return resetResult{}, fmt.Errorf("Activate Codex reset returned unexpected result: %s", result)
	}
	windows, _ := finite(payload["rate_limit_windows_reset"])
	return resetResult{result, windows}, nil
}
func formatResetResult(id string, r resetResult) string {
	verb := "result: " + r.Result
	if r.Result == "reset" {
		verb = "activated"
	}
	return fmt.Sprintf("Reset %s %s (%s rate-limit windows reset).", id, verb, strconv.FormatFloat(r.Windows, 'f', -1, 64))
}
func trim(text string) string {
	return strings.TrimFunc(text, func(r rune) bool {
		return unicode.Is(unicode.Zs, r) || strings.ContainsRune("\t\n\v\f\r\u2028\u2029\uFEFF", r)
	})
}
func validCreditID(s string) bool {
	return !strings.ContainsFunc(s, func(r rune) bool {
		return unicode.Is(unicode.Zs, r) || strings.ContainsRune("\t\n\v\f\r\u2028\u2029\uFEFF", r)
	})
}
