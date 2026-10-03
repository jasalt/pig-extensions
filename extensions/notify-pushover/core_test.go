package notifypushover

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestConfigWholeEnvironmentPrecedenceAndLateFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "notify-pushover.json")
	env := map[string]string{}
	get := func(key string) string { return env[key] }
	cfg, err := loadConfig(path, get)
	if cfg != nil || err != nil {
		t.Fatalf("missing: %v %v", cfg, err)
	}
	if err := os.WriteFile(path, []byte(`{"pushover":{"userKey":" file-user ","appToken":"file-token","device":" file-device "}}`), 0600); err != nil {
		t.Fatal(err)
	}
	env["PIG_PUSHOVER_USER_KEY"] = "partial"
	env["PIG_PUSHOVER_DEVICE"] = "env-device"
	cfg, err = loadConfig(path, get)
	if err != nil || cfg == nil || *cfg != (config{"file-user", "file-token", "file-device"}) {
		t.Fatalf("partial mixing: %v %v", cfg, err)
	}
	env["PIG_PUSHOVER_APP_TOKEN"] = " env-token "
	cfg, err = loadConfig(path, get)
	if err != nil || cfg == nil || *cfg != (config{"partial", "env-token", "env-device"}) {
		t.Fatalf("env precedence: %v %v", cfg, err)
	}
	// Whole environment credentials avoid even parsing a malformed file.
	if err := os.WriteFile(path, []byte(`secret-invalid-json`), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := loadConfig(path, get); err != nil {
		t.Fatal(err)
	}
	delete(env, "PIG_PUSHOVER_APP_TOKEN")
	if _, err := loadConfig(path, get); err == nil || strings.Contains(err.Error(), "secret-invalid-json") {
		t.Fatalf("redacted config diagnostic: %v", err)
	}
	for _, raw := range []string{`null`, `[]`, `{"userKey":"","appToken":"token"}`, `{"pushover":false}`} {
		if err := os.WriteFile(path, []byte(raw), 0600); err != nil {
			t.Fatal(err)
		}
		if cfg, err := loadConfig(path, get); cfg != nil || err != nil {
			t.Fatalf("%s: %v %v", raw, cfg, err)
		}
	}
}

func TestAgentDirResolution(t *testing.T) {
	for _, key := range []string{"PIG_HOME", "XDG_CONFIG_HOME", "PIG_CODING_AGENT_DIR", "PIG_USE_PI_DIRS", "PI_CODING_AGENT_DIR"} {
		t.Setenv(key, "")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	if got := agentDir(); got != filepath.Join(home, ".pig/agent") {
		t.Fatal(got)
	}
	t.Setenv("XDG_CONFIG_HOME", "/tmp/config")
	if got := agentDir(); got != "/tmp/config/pig/agent" {
		t.Fatal(got)
	}
	t.Setenv("PIG_HOME", "~/pig-test")
	if got := agentDir(); got != filepath.Join(home, "pig-test/agent") {
		t.Fatal(got)
	}
	t.Setenv("PIG_CODING_AGENT_DIR", "~/agent-test")
	if got := agentDir(); got != filepath.Join(home, "agent-test") {
		t.Fatal(got)
	}
	t.Setenv("PIG_USE_PI_DIRS", "1")
	t.Setenv("PI_CODING_AGENT_DIR", "/tmp/selected-agent")
	if got := credentialPath(); got != "/tmp/selected-agent/notify-pushover.json" {
		t.Fatal(got)
	}
}

func TestFormLimitsAndEncoding(t *testing.T) {
	cfg := &config{"u +&", "t/+&", "dev"}
	p := params{Message: strings.Repeat("x", 1023) + "🐈", Title: strings.Repeat("é", 251), URL: strings.Repeat("+", 513), URLTitle: strings.Repeat("🐈", 51), Priority: 1}
	v := form(cfg, p)
	if v.Get("message") != strings.Repeat("x", 1023)+"�" || len([]rune(v.Get("title"))) != 250 || len(v.Get("url")) != 512 || len([]rune(v.Get("url_title"))) != 50 {
		t.Fatal(v)
	}
	decoded, err := url.ParseQuery(v.Encode())
	if err != nil || decoded.Get("token") != cfg.AppToken || decoded.Get("user") != cfg.UserKey {
		t.Fatal(decoded, err)
	}
	for _, priority := range []int{-2, -1, 0, 1, 2, -3} {
		got := form(cfg, params{Message: "test", Priority: priority}).Get("priority")
		if (priority > 1 || priority < -2) && got != "0" {
			t.Fatal(got)
		}
	}
	if form(cfg, params{Message: "test"}).Get("title") != "PiG needs a human decision" {
		t.Fatal("default title")
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func localClient(server *httptest.Server) *http.Client {
	client := newClient()
	transport := server.Client().Transport
	client.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		copy := r.Clone(r.Context())
		u := *r.URL
		copy.URL = &u
		local, _ := url.Parse(server.URL)
		copy.URL.Scheme = local.Scheme
		copy.URL.Host = local.Host
		copy.Host = local.Host
		return transport.RoundTrip(copy)
	})
	return client
}

func TestHTTPStatusBodyRedactionAndRedirectRefusal(t *testing.T) {
	cfg := &config{"user &long", "token/+", ""}
	var count atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count.Add(1)
		if r.Method != "POST" || r.URL.Path != "/1/messages.json" {
			t.Errorf("request: %s %s", r.Method, r.URL)
		}
		if err := r.ParseForm(); err != nil {
			t.Error(err)
		}
		if r.Form.Get("user") != cfg.UserKey || r.Form.Get("token") != cfg.AppToken {
			t.Error("credentials not form encoded")
		}
		w.Header().Set("Location", "/must-not-follow")
		w.WriteHeader(http.StatusFound)
		_, _ = io.WriteString(w, cfg.UserKey+" "+cfg.AppToken+" "+url.QueryEscape(cfg.AppToken))
	}))
	defer server.Close()
	client := localClient(server)
	defer client.CloseIdleConnections()
	status, err := send(context.Background(), client, cfg, params{Message: "payload"})
	if status != 302 || err == nil || count.Load() != 1 {
		t.Fatalf("redirect: %d %v %d", status, err, count.Load())
	}
	if strings.Contains(err.Error(), cfg.UserKey) || strings.Contains(err.Error(), cfg.AppToken) || strings.Contains(err.Error(), url.QueryEscape(cfg.AppToken)) {
		t.Fatalf("secret leak: %v", err)
	}
	if client.Timeout != 15*time.Second {
		t.Fatal(client.Timeout)
	}
}

func TestHTTPCancellationBeforeAndDuringSend(t *testing.T) {
	cfg := &config{"user", "token", ""}
	var count atomic.Int32
	started := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count.Add(1)
		_, _ = io.Copy(io.Discard, r.Body)
		close(started)
		<-r.Context().Done()
	}))
	defer server.Close()
	client := localClient(server)
	defer client.CloseIdleConnections()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := send(ctx, client, cfg, params{Message: "test"}); err == nil {
		t.Fatal("cancel ignored")
	}
	if count.Load() != 0 {
		t.Fatal("sent before cancellation check")
	}
	ctx, cancel = context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { _, err := send(ctx, client, cfg, params{Message: "test"}); done <- err }()
	select {
	case <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("not started")
	}
	cancel()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("cancel ignored")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("send leaked")
	}
	if count.Load() != 1 {
		t.Fatal("automatic retry")
	}
}

func TestHTTPAcceptedAndNetworkErrors(t *testing.T) {
	cfg := &config{"user", "token", ""}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		_, _ = io.WriteString(w, `{"status":1}`)
	}))
	defer server.Close()
	client := localClient(server)
	defer client.CloseIdleConnections()
	if status, err := send(context.Background(), client, cfg, params{Message: "test"}); status != 200 || err != nil {
		t.Fatal(status, err)
	}
	var count int
	client.Transport = roundTripFunc(func(*http.Request) (*http.Response, error) { count++; return nil, errors.New("failed token user") })
	if _, err := send(context.Background(), client, cfg, params{Message: "test"}); err == nil || strings.Contains(err.Error(), "token") || strings.Contains(err.Error(), "user") {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatal("retried")
	}
	if _, err := send(context.Background(), client, cfg, params{Message: " \n\t"}); err == nil {
		t.Fatal("blank allowed")
	}
	if count != 1 {
		t.Fatal("blank sent")
	}
}
