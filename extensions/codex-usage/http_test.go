package codexusage

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func adapterConnection(t *testing.T, base string) connection {
	t.Helper()
	c, err := resolveConnection("adapter", map[string]any{"baseUrl": base}, map[string]any{"ok": true, "apiKey": "dummy-secret-token", "headers": map[string]any{"X-Test": "fixture-header-secret", "authorization": "obsolete-secret"}})
	if err != nil {
		t.Fatal(err)
	}
	return c
}
func nativeConnection(t *testing.T) connection {
	t.Helper()
	claims := []byte(`{"email":"native@example.org","https://api.openai.com/auth":{"chatgpt_account_id":"dummy-account-id","chatgpt_plan_type":"pro"}}`)
	token := "header." + base64.RawURLEncoding.EncodeToString(claims) + ".sig"
	c, err := resolveConnection("openai-codex", nil, map[string]any{"apiKey": token})
	if err != nil {
		t.Fatal(err)
	}
	return c
}
func TestConnectionNativeAdapterAndAuthFailures(t *testing.T) {
	c := nativeConnection(t)
	if !c.Native || c.Headers.Get("ChatGPT-Account-Id") != "dummy-account-id" || c.Headers.Get("originator") != "pig" || c.Account != (account{"native@example.org", "pro"}) {
		t.Fatal(c)
	}
	c = adapterConnection(t, "https://adapter.invalid/v1/")
	if c.Base != "https://adapter.invalid/v1" || c.Headers.Get("X-Test") != "fixture-header-secret" || c.Headers.Get("Authorization") != "Bearer dummy-secret-token" {
		t.Fatal(c)
	}
	c, err := resolveConnection("adapter", map[string]any{"baseUrl": "https://wrong.invalid"}, map[string]any{"apiKey": "dummy", "baseUrl": "https://override.invalid/v1/"})
	if err != nil || c.Base != "https://override.invalid/v1" {
		t.Fatal(c, err)
	}
	for _, tc := range []struct {
		provider    string
		model, auth map[string]any
	}{
		{"adapter", nil, nil},
		{"adapter", nil, map[string]any{"ok": false, "apiKey": "dummy"}},
		{"adapter", nil, map[string]any{"apiKey": "dummy"}},
		{"adapter", map[string]any{"baseUrl": "file:///tmp"}, map[string]any{"apiKey": "dummy"}},
		{"openai-codex", nil, map[string]any{"apiKey": "dummy"}},
		{"openai-codex", nil, map[string]any{"apiKey": "head.bad-token.sig"}},
		{"openai-codex", nil, map[string]any{"apiKey": "head.e30.sig"}},
	} {
		if _, err := resolveConnection(tc.provider, tc.model, tc.auth); err == nil {
			t.Fatal("bad connection accepted")
		}
	}
}
func TestAdapterRequestsAndAcceptedResetBodies(t *testing.T) {
	var count atomic.Int32
	var body map[string]any
	result := "reset"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count.Add(1)
		if r.Header.Get("Authorization") != "Bearer dummy-secret-token" || r.Header.Get("X-Test") != "fixture-header-secret" {
			t.Error("resolved headers missing")
		}
		switch r.URL.Path {
		case "/v1/codex/usage":
			if r.Method != "GET" {
				t.Error(r.Method)
			}
			if err := json.NewEncoder(w).Encode(testPayload()); err != nil {
				t.Error(err)
			}
		case "/v1/codex/resets":
			if r.Method != "GET" {
				t.Error(r.Method)
			}
			_, err := io.WriteString(w, `{"credits":[]}`)
			if err != nil {
				t.Error(err)
			}
		case "/v1/codex/reset":
			if r.Method != "POST" || r.Header.Get("Content-Type") != "application/json" {
				t.Error(r.Method, r.Header)
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Error(err)
			}
			payload := map[string]any{}
			if result != "" {
				payload["status"] = result
				payload["rate_limit_windows_reset"] = 2
			}
			if err := json.NewEncoder(w).Encode(payload); err != nil {
				t.Error(err)
			}
		default:
			t.Error("unexpected path", r.URL.Path)
			w.WriteHeader(404)
		}
	}))
	defer server.Close()
	client := newClient()
	defer client.CloseIdleConnections()
	c := adapterConnection(t, server.URL+"/v1/")
	if s, err := fetchStatus(context.Background(), client, c); err != nil || s.Primary == nil {
		t.Fatal(s, err)
	}
	if rows, err := fetchCredits(context.Background(), client, c); err != nil || len(rows) != 0 {
		t.Fatal(rows, err)
	}
	for _, value := range []string{"", "reset", "already_redeemed", "nothing_to_reset", "no_credit"} {
		result = value
		r, err := activateReset(context.Background(), client, c, "exact-id")
		if err != nil {
			t.Fatal(err)
		}
		want := value
		if want == "" {
			want = "reset"
		}
		if r.Result != want {
			t.Fatal(r)
		}
		if len(body) != 1 || body["credit_id"] != "exact-id" {
			t.Fatal(body)
		}
	}
	result = "unexpected"
	before := count.Load()
	if _, err := activateReset(context.Background(), client, c, "exact-id"); err == nil {
		t.Fatal("unexpected result accepted")
	}
	if count.Load() != before+1 {
		t.Fatal("retried redemption")
	}
	if client.Timeout != 15*time.Second {
		t.Fatal(client.Timeout)
	}
}
func TestNativeEndpointsClaimsAndUniqueRedemptionIDs(t *testing.T) {
	c := nativeConnection(t)
	urls := []string{}
	ids := map[string]bool{}
	client := newClient()
	defer client.CloseIdleConnections()
	client.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		urls = append(urls, r.URL.String())
		if r.Header.Get("ChatGPT-Account-Id") != "dummy-account-id" || r.Header.Get("originator") != "pig" {
			t.Error(r.Header)
		}
		payload := map[string]any{}
		switch r.URL.String() {
		case usageURL:
			payload = testPayload()
		case resetsURL:
			payload = map[string]any{"credits": []any{}}
		case resetsURL + "/consume":
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			id := stringValue(body["redeem_request_id"])
			if body["credit_id"] != "native-id" || len(body) != 2 || !regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`).MatchString(id) || ids[id] {
				t.Fatal(body)
			}
			ids[id] = true
		default:
			t.Error(r.URL)
		}
		raw, err := json.Marshal(payload)
		if err != nil {
			t.Fatal(err)
		}
		return &http.Response{StatusCode: 200, Status: "200 OK", Body: io.NopCloser(strings.NewReader(string(raw))), Header: make(http.Header)}, nil
	})
	s, err := fetchStatus(context.Background(), client, c)
	if err != nil || s.Account != c.Account {
		t.Fatal(s, err)
	}
	if _, err := fetchCredits(context.Background(), client, c); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if _, err := activateReset(context.Background(), client, c, "native-id"); err != nil {
			t.Fatal(err)
		}
	}
	if len(urls) != 4 || len(ids) != 2 {
		t.Fatal(urls, ids)
	}
}
func TestHTTPFailuresAndMalformedResponsesRedacted(t *testing.T) {
	c := adapterConnection(t, "https://adapter.invalid")
	cases := []struct {
		code int
		body string
		want string
	}{
		{403, "denied dummy-secret-token fixture-header-secret obsolete-secret", "HTTP 403"},
		{200, "not-json dummy-secret-token", "malformed JSON"},
		{200, "{} {}", "malformed JSON"},
		{200, "null", "malformed JSON"},
	}
	client := newClient()
	defer client.CloseIdleConnections()
	for _, tc := range cases {
		client.Transport = roundTripFunc(func(*http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: tc.code, Body: io.NopCloser(strings.NewReader(tc.body)), Header: make(http.Header)}, nil
		})
		_, err := requestJSON(context.Background(), client, c, c.Base, "GET", nil)
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Fatal(err)
		}
		for _, secret := range []string{"dummy-secret-token", "fixture-header-secret", "obsolete-secret"} {
			if strings.Contains(err.Error(), secret) {
				t.Fatal("credential leak", err)
			}
		}
	}
	client.Transport = roundTripFunc(func(*http.Request) (*http.Response, error) {
		return nil, errors.New("failed dummy-secret-token fixture-header-secret")
	})
	if _, err := requestJSON(context.Background(), client, c, c.Base, "GET", nil); err == nil || strings.Contains(err.Error(), "dummy-secret-token") || strings.Contains(err.Error(), "fixture-header-secret") {
		t.Fatal(err)
	}
}
func TestRedirectsDoNotForwardCredentialsAcrossOrigins(t *testing.T) {
	var leaked bool
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		leaked = r.Header.Get("Authorization") != "" || r.Header.Get("X-Test") != ""
		if err := json.NewEncoder(w).Encode(testPayload()); err != nil {
			t.Error(err)
		}
	}))
	defer target.Close()
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, target.URL, 302) }))
	defer origin.Close()
	client := newClient()
	defer client.CloseIdleConnections()
	if _, err := fetchStatus(context.Background(), client, adapterConnection(t, origin.URL)); err != nil {
		t.Fatal(err)
	}
	if leaked {
		t.Fatal("redirect leaked credentials")
	}
}
func TestHTTPBeforeDuringCancellationAndNoRetry(t *testing.T) {
	var count atomic.Int32
	started := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count.Add(1)
		_, _ = io.Copy(io.Discard, r.Body)
		close(started)
		<-r.Context().Done()
	}))
	defer server.Close()
	c := adapterConnection(t, server.URL)
	client := newClient()
	defer client.CloseIdleConnections()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := fetchStatus(ctx, client, c); err == nil {
		t.Fatal("cancelled request accepted")
	}
	if count.Load() != 0 {
		t.Fatal("sent after cancellation")
	}
	ctx, cancel = context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { _, err := activateReset(ctx, client, c, "dummy-id"); done <- err }()
	select {
	case <-started:
	case <-time.After(3 * time.Second):
		t.Fatal("request not admitted")
	}
	cancel()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("cancel ignored")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("HTTP leaked")
	}
	if count.Load() != 1 {
		t.Fatal("retried potentially accepted reset")
	}
}
func TestEncodedSecretsAreRedacted(t *testing.T) {
	c, err := resolveConnection("adapter", map[string]any{"baseUrl": "https://adapter.invalid"}, map[string]any{"apiKey": "long/+ token"})
	if err != nil {
		t.Fatal(err)
	}
	for _, value := range []string{"long/+ token", url.QueryEscape("long/+ token"), url.PathEscape("long/+ token")} {
		if got := c.scrub(value); strings.Contains(got, value) {
			t.Fatal(got)
		}
	}
}
