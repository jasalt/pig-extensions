package codexusage

import (
	"encoding/json"
	"math"
	"strings"
	"testing"
	"time"
)

func windowMap(used, seconds, reset float64) map[string]any {
	return map[string]any{"used_percent": used, "limit_window_seconds": seconds, "reset_at": reset}
}
func testPayload() map[string]any {
	return map[string]any{"account": map[string]any{"email": "test@example.org", "plan": "plus"}, "rate_limit": map[string]any{"primary_window": windowMap(23.5, 18000, 1700000000), "secondary_window": windowMap(110, 604800, 1700500000)}}
}

func TestOriginalParsingAndExactPresentation(t *testing.T) {
	t.Setenv("LC_ALL", "en_US.UTF-8")
	s, err := parseStatus(testPayload(), nil)
	if err != nil {
		t.Fatal(err)
	}
	want := "76.5%/5h (resets 22:13)  0%/w (resets 17:06 on Nov 20)"
	if got := formatStatusLine(s, time.UTC); got != want {
		t.Fatalf("line = %q, want %q", got, want)
	}
	want = "ChatGPT Codex status\nVisit https://chatgpt.com/codex/settings/usage for up-to-date information on rate limits and credits.\nAccount:       test@example.org (Plus)\n5h limit:      [███████████████░░░░░] 76.5% left (resets 22:13)\nWeekly limit: [░░░░░░░░░░░░░░░░░░░░] 0% left (resets 17:06 on Nov 20)"
	if got := formatStatusCard(s, time.UTC); got != want {
		t.Fatalf("card = %q, want %q", got, want)
	}
}
func TestFiniteNumbersWeeklyToleranceAndPrimaryIdentity(t *testing.T) {
	for _, bad := range []any{nil, math.NaN(), math.Inf(1), "10", true, json.Number("1e999")} {
		w := windowMap(0, 3600, 1)
		w["used_percent"] = bad
		if _, err := parseStatus(map[string]any{"rate_limit": map[string]any{"primary_window": w}}, nil); err == nil {
			t.Fatalf("accepted %v", bad)
		}
	}
	for _, seconds := range []float64{604800, 600000, 604800 * .95, 604800 * 1.05} {
		s, err := parseStatus(map[string]any{"rate_limit": map[string]any{"secondary_window": windowMap(30, seconds, 1)}}, nil)
		if err != nil || s.Weekly == nil || s.Primary != nil {
			t.Fatal(s, err)
		}
	}
	if _, err := parseStatus(map[string]any{"rate_limit": map[string]any{"secondary_window": windowMap(30, 604800*1.051, 1)}}, nil); err == nil {
		t.Fatal("outside tolerance")
	}
	s, err := parseStatus(map[string]any{"rate_limit": map[string]any{"primary_window": windowMap(110, 604800, 1)}}, nil)
	if err != nil || s.Primary != s.Weekly || strings.Count(formatStatusLine(s, time.UTC), "resets") != 1 {
		t.Fatal(s, err)
	}
	if percentLeft(&window{Used: -10}) != 100 || percentLeft(&window{Used: 110}) != 0 {
		t.Fatal("clamp")
	}
	if durationLabel(&window{Seconds: 3700}) != "rate" {
		t.Fatal("fractional hours")
	}
	override := account{"native@example.org", "pro"}
	s, err = parseStatus(testPayload(), &override)
	if err != nil || s.Account != override {
		t.Fatal(s, err)
	}
}
func TestPercentMatchesJavaScriptTiesAndBarRounding(t *testing.T) {
	for _, tc := range []struct {
		value float64
		want  string
	}{{100, "100%"}, {0, "0%"}, {1.25, "1.3%"}, {2.55, "2.5%"}, {3.15, "3.1%"}, {76.5, "76.5%"}} {
		if got := formatPercent(tc.value); got != tc.want {
			t.Fatalf("%v: %s != %s", tc.value, got, tc.want)
		}
	}
	got := fullWindow(&window{Used: 97.5, Seconds: 3600, Reset: 0}, time.UTC)
	if !strings.HasPrefix(got, "[█░░░░░░░░░░░░░░░░░░░]") {
		t.Fatal(got)
	}
}
func TestLocalDatesDSTAndTimeClip(t *testing.T) {
	t.Setenv("LC_ALL", "en_US.UTF-8")
	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ iso, want string }{{"2026-03-08T06:59:00Z", "01:59"}, {"2026-03-08T07:00:00Z", "03:00"}, {"2026-11-01T05:59:00Z", "01:59"}, {"2026-11-01T06:00:00Z", "01:00"}} {
		d, err := time.Parse(time.RFC3339, tc.iso)
		if err != nil {
			t.Fatal(err)
		}
		if got := resetTime(&window{Seconds: 18000, Reset: float64(d.Unix())}, loc); got != tc.want {
			t.Fatal(got, tc.want)
		}
	}
	d, _ := time.Parse(time.RFC3339, "2026-01-01T00:01:00Z")
	if got := resetTime(&window{Seconds: 604800, Reset: float64(d.Unix())}, loc); got != "19:01 on Dec 31" {
		t.Fatal(got)
	}
	t.Setenv("LC_ALL", "en_GB.UTF-8")
	if got := resetTime(&window{Seconds: 604800, Reset: float64(d.Unix())}, loc); got != "19:01 on 31 Dec" {
		t.Fatal(got)
	}
	if got := resetTime(&window{Seconds: 3600, Reset: 1e15}, time.UTC); got != "Invalid Date" {
		t.Fatal(got)
	}
	if got := resetTime(&window{Seconds: 604800, Reset: 1e15}, time.UTC); got != "Invalid Date on Invalid Date" {
		t.Fatal(got)
	}
}
func TestCreditsExpiryOrderingAndUTCFormatting(t *testing.T) {
	raw := map[string]any{"credits": []any{
		map[string]any{"id": "never", "status": "banked", "granted_at": "invalid"},
		map[string]any{"id": "later", "status": "banked", "granted_at": "2026-01-01T12:30:00+02:00", "expires_at": "2026-03-01T00:00:00Z"},
		map[string]any{"id": "early", "status": "banked", "granted_at": "2026-01-01T12:30:00Z", "expires_at": "2026-02-01T00:00:00Z"},
	}}
	credits, err := parseCredits(raw)
	if err != nil {
		t.Fatal(err)
	}
	if credits[0].ID != "early" || credits[1].ID != "later" || credits[2].ID != "never" {
		t.Fatal(credits)
	}
	want := "Banked Codex rate-limit resets:\nearly  banked  granted 2026-01-01 12:30 UTC  expires 2026-02-01 00:00 UTC\nlater  banked  granted 2026-01-01 10:30 UTC  expires 2026-03-01 00:00 UTC\nnever  banked  granted invalid  expires never/unknown\nActivate one with /codex-reset <reset-id>."
	if got := formatCredits(credits); got != want {
		t.Fatalf("%q != %q", got, want)
	}
	if got := formatCredits(nil); got != "No banked rate-limit reset credits." {
		t.Fatal(got)
	}
	for _, s := range []string{"2026-01-01", "2026-01-01T00:00:00.999Z"} {
		if got := formatCreditTime(s); got != "2026-01-01 00:00 UTC" {
			t.Fatal(got)
		}
	}
	if _, err := parseCredits(map[string]any{"credits": "bad"}); err == nil {
		t.Fatal("malformed credits")
	}
}
func TestResetResultsAndExactIDValidation(t *testing.T) {
	for _, result := range []string{"", "reset", "already_redeemed", "nothing_to_reset", "no_credit"} {
		r, err := parseResetResult(map[string]any{"status": result, "rate_limit_windows_reset": float64(2)})
		if err != nil {
			t.Fatal(err)
		}
		expected := result
		if expected == "" {
			expected = "reset"
		}
		if r.Result != expected {
			t.Fatal(r)
		}
	}
	if _, err := parseResetResult(map[string]any{"result": "unexpected"}); err == nil {
		t.Fatal("unexpected accepted")
	}
	if got := formatResetResult("exact-id", resetResult{"reset", 0}); got != "Reset exact-id activated (0 rate-limit windows reset)." {
		t.Fatal(got)
	}
	if got := formatResetResult("exact-id", resetResult{"no_credit", 2}); got != "Reset exact-id result: no_credit (2 rate-limit windows reset)." {
		t.Fatal(got)
	}
	if trim("\ufeff exact-id \ufeff") != "exact-id" || validCreditID("not an id") || validCreditID("bad\u2028id") || !validCreditID("exact-id") {
		t.Fatal("ID parsing")
	}
}
