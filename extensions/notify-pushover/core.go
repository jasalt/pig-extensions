// Copyright (c) 2026 Jarkko Saltiola
// SPDX-License-Identifier: MIT
// Adapted with user authorization from local Pi pushover-human/index.ts.
package notifypushover

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf16"
	"unicode/utf8"
)

const endpoint = "https://api.pushover.net/1/messages.json"
const statusKey = "notify-pushover"

type config struct{ UserKey, AppToken, Device string }
type params struct {
	Message, Title, URL, URLTitle string
	Priority                      int
}

func expandHome(path string) string {
	home, _ := os.UserHomeDir()
	if path == "~" {
		return home
	}
	if strings.HasPrefix(path, "~/") {
		return filepath.Join(home, path[2:])
	}
	return path
}

// Mirror PiG's selected-agent-directory rules, not SDK ConfigHome's narrower
// PIG_HOME-only fallback. There is no public agent-dir getter in SDK v0.3.1.
func agentDir() string {
	envName := "PIG_CODING_AGENT_DIR"
	if os.Getenv("PIG_USE_PI_DIRS") == "1" {
		envName = "PI_CODING_AGENT_DIR"
	}
	if path := os.Getenv(envName); path != "" {
		return expandHome(path)
	}
	home, _ := os.UserHomeDir()
	if os.Getenv("PIG_USE_PI_DIRS") == "1" {
		return filepath.Join(home, ".pi", "agent")
	}
	root := os.Getenv("PIG_HOME")
	if root == "" {
		if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
			root = filepath.Join(expandHome(xdg), "pig")
		} else {
			root = filepath.Join(home, ".pig")
		}
	}
	return filepath.Join(expandHome(root), "agent")
}

func trim(text string) string {
	return strings.TrimFunc(text, func(r rune) bool {
		return unicode.Is(unicode.Zs, r) || strings.ContainsRune("\t\n\v\f\r\u2028\u2029\uFEFF", r)
	})
}

func credentialPath() string { return filepath.Join(agentDir(), "notify-pushover.json") }

// JavaScript String(value ?? "") behavior for the original JSON fields.
func jsString(value any) string {
	switch value := value.(type) {
	case nil:
		return ""
	case string:
		return value
	case bool:
		return strconv.FormatBool(value)
	case float64:
		if value == 0 {
			return "0"
		}
		if abs := math.Abs(value); abs < 1e-6 || abs >= 1e21 {
			return strings.NewReplacer("e-0", "e-", "e+0", "e+").Replace(strconv.FormatFloat(value, 'e', -1, 64))
		}
		return strconv.FormatFloat(value, 'f', -1, 64)
	case []any:
		parts := make([]string, len(value))
		for i, v := range value {
			parts[i] = jsString(v)
		}
		return strings.Join(parts, ",")
	default:
		return "[object Object]"
	}
}

func loadConfig(path string, getenv func(string) string) (*config, error) {
	user, token := trim(getenv("PIG_PUSHOVER_USER_KEY")), trim(getenv("PIG_PUSHOVER_APP_TOKEN"))
	if user != "" && token != "" {
		return &config{user, token, trim(getenv("PIG_PUSHOVER_DEVICE"))}, nil
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, errors.New("notify-pushover: failed to read credential file")
	}
	var parsed any
	if err := json.Unmarshal(data, &parsed); err != nil {
		return nil, errors.New("notify-pushover: malformed credential JSON")
	}
	po, ok := parsed.(map[string]any)
	if !ok {
		return nil, nil
	}
	if nested, exists := po["pushover"]; exists && nested != nil {
		po, ok = nested.(map[string]any)
		if !ok {
			return nil, nil
		}
	}
	user, token = trim(jsString(po["userKey"])), trim(jsString(po["appToken"]))
	if user == "" || token == "" {
		return nil, nil
	}
	device := po["device"]
	deviceString := ""
	if device != nil && device != false && device != float64(0) && device != "" {
		deviceString = trim(jsString(device))
	}
	return &config{user, token, deviceString}, nil
}

func redact(text string, cfg *config) string {
	secrets := []string{cfg.UserKey, cfg.AppToken}
	sort.Slice(secrets, func(i, j int) bool { return len(secrets[i]) > len(secrets[j]) })
	for _, secret := range secrets {
		if secret == "" {
			continue
		}
		// Include encodeURIComponent and form encoding, then raw spelling.
		escaped := url.QueryEscape(secret)
		component := strings.ReplaceAll(escaped, "+", "%20")
		component = strings.NewReplacer("%21", "!", "%27", "'", "%28", "(", "%29", ")", "%2A", "*").Replace(component)
		for _, pair := range [][2]string{{component, "%3Credacted%3E"}, {escaped, "%3Credacted%3E"}, {secret, "<redacted>"}} {
			text = strings.ReplaceAll(text, pair[0], pair[1])
		}
	}
	return text
}

// JS .slice() counts UTF-16 units; URLSearchParams replaces a cut surrogate.
func limit(text string, n int) string {
	units := make([]uint16, 0, min(n, len(text)))
	for len(text) > 0 && len(units) < n {
		// The PiG SDK preserves unmatched UTF-16 surrogates as WTF-8.
		if len(text) >= 3 && text[0] == 0xed && text[1] >= 0xa0 && text[1] <= 0xbf && text[2] >= 0x80 && text[2] <= 0xbf {
			units = append(units, uint16(text[0]&15)<<12|uint16(text[1]&63)<<6|uint16(text[2]&63))
			text = text[3:]
			continue
		}
		r, size := utf8.DecodeRuneInString(text)
		units = append(units, utf16.Encode([]rune{r})...)
		text = text[size:]
	}
	if len(units) > n {
		units = units[:n]
	}
	return string(utf16.Decode(units))
}

func form(cfg *config, p params) url.Values {
	title := p.Title
	if title == "" {
		title = "PiG needs a human decision"
	}
	priority := p.Priority
	if priority < -2 || priority > 1 {
		priority = 0
	}
	v := url.Values{"token": {cfg.AppToken}, "user": {cfg.UserKey}, "title": {limit(title, 250)}, "message": {limit(p.Message, 1024)}, "priority": {strconv.Itoa(priority)}}
	if cfg.Device != "" {
		v.Set("device", cfg.Device)
	}
	if p.URL != "" {
		v.Set("url", limit(p.URL, 512))
	}
	if p.URLTitle != "" {
		v.Set("url_title", limit(p.URLTitle, 100))
	}
	return v
}

func newClient() *http.Client {
	return &http.Client{Timeout: 15 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
}

// client seam is private to tests. Production always uses the fixed endpoint.
func send(ctx context.Context, client *http.Client, cfg *config, p params) (int, error) {
	if trim(p.Message) == "" {
		return 0, errors.New("Notification message cannot be empty.")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form(cfg, p).Encode()))
	if err != nil {
		return 0, errors.New("Pushover request could not be created")
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	res, err := client.Do(req)
	if err != nil {
		return 0, fmt.Errorf("Pushover send failed: %s", redact(err.Error(), cfg))
	}
	defer res.Body.Close()
	body, err := io.ReadAll(res.Body)
	if err != nil {
		return res.StatusCode, fmt.Errorf("Pushover response failed: %s", redact(err.Error(), cfg))
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		suffix := trim(string(body))
		if suffix != "" {
			suffix = " - " + redact(suffix, cfg)
		}
		statusText := trim(strings.TrimPrefix(res.Status, strconv.Itoa(res.StatusCode)))
		return res.StatusCode, fmt.Errorf("Pushover failed: %d %s%s", res.StatusCode, redact(statusText, cfg), suffix)
	}
	return res.StatusCode, nil
}
