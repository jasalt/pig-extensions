package codexusage

import (
	"encoding/json"
	"testing"
)

func TestTruthyMalformedResetResultCannotBecomeSuccess(t *testing.T) {
	for _, value := range []any{float64(42), json.Number("1e999"), map[string]any{}, []any{}} {
		if _, err := parseResetResult(map[string]any{"result": value, "status": "reset"}); err == nil {
			t.Fatalf("accepted non-string result: %v", value)
		}
	}
	for _, value := range []any{nil, "", false, float64(0), json.Number("0")} {
		r, err := parseResetResult(map[string]any{"result": value, "status": "no_credit"})
		if err != nil || r.Result != "no_credit" {
			t.Fatal(r, err)
		}
	}
}
