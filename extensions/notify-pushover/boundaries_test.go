package notifypushover

import (
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestOriginalCoercionAndECMAScriptWhitespace(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	for _, tc := range []struct {
		raw  string
		want config
	}{
		{`{"userKey":123,"appToken":true,"device":false}`, config{"123", "true", ""}},
		{`{"userKey":["one",null,2],"appToken":{"a":1},"device":0}`, config{"one,,2", "[object Object]", ""}},
		{`{"userKey":1e-7,"appToken":1e21}`, config{"1e-7", "1e+21", ""}},
		{`{"userKey":"\ufeff user \ufeff","appToken":"\u0085token\u0085"}`, config{"user", "\u0085token\u0085", ""}},
	} {
		if err := os.WriteFile(path, []byte(tc.raw), 0600); err != nil {
			t.Fatal(err)
		}
		cfg, err := loadConfig(path, func(string) string { return "" })
		if err != nil || cfg == nil || *cfg != tc.want {
			t.Fatalf("%s: %v %v", tc.raw, cfg, err)
		}
	}
	if got := limit("\xed\xa0\x80", 250); got != "�" {
		t.Fatalf("unmatched surrogate: %q", got)
	}
	if got := limit("\xed\xa0\xbd\xed\xb0\x88", 250); got != "🐈" {
		t.Fatalf("paired WTF8 surrogate: %q", got)
	}
	if got := redact("token!'()*%20with%20space", &config{UserKey: "token!'()* with space"}); got != "%3Credacted%3E" {
		t.Fatal(got)
	}
}

type failedBody struct{ closed bool }

func (*failedBody) Read([]byte) (int, error) { return 0, errors.New("read secret-token failed") }
func (b *failedBody) Close() error           { b.closed = true; return nil }

func TestResponseReadFailureClosesBodyAndRedacts(t *testing.T) {
	body := &failedBody{}
	client := newClient()
	client.Transport = roundTripFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Status: "200 OK", Body: body, Header: make(http.Header)}, nil
	})
	_, err := send(context.Background(), client, &config{"secret-user", "secret-token", ""}, params{Message: "test"})
	if err == nil || strings.Contains(err.Error(), "secret-token") || !body.closed {
		t.Fatal(err, body.closed)
	}
}

func TestTimeoutDoesNotRetry(t *testing.T) {
	client := newClient()
	client.Timeout = 20 * time.Millisecond
	var count int
	client.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		count++
		<-r.Context().Done()
		return nil, r.Context().Err()
	})
	start := time.Now()
	_, err := send(context.Background(), client, &config{"u", "t", ""}, params{Message: "test"})
	if err == nil || count != 1 || time.Since(start) > time.Second {
		t.Fatal(err, count)
	}
}

func TestLegacyCredentialsAreIgnored(t *testing.T) {
	env := map[string]string{"PUSHOVER_USER_KEY": "legacy", "PUSHOVER_APP_TOKEN": "legacy", "PUSHOVER_DEVICE": "legacy"}
	cfg, err := loadConfig(filepath.Join(t.TempDir(), "missing"), func(key string) string { return env[key] })
	if err != nil || cfg != nil {
		t.Fatal(cfg, err)
	}
}

var _ io.ReadCloser = (*failedBody)(nil)
