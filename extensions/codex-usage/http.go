// Copyright (c) 2026 Jarkko Saltiola
// SPDX-License-Identifier: MIT
package codexusage

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
)

const usageURL = "https://chatgpt.com/backend-api/wham/usage"
const resetsURL = "https://chatgpt.com/backend-api/wham/rate-limit-reset-credits"

type connection struct {
	Native  bool
	Base    string
	Headers http.Header
	Account account
	secrets []string
}

func resolveConnection(provider string, model, auth map[string]any) (connection, error) {
	token := stringValue(auth["apiKey"])
	if token == "" || auth["ok"] == false {
		return connection{}, fmt.Errorf("No authentication available for %s", provider)
	}
	c := connection{Native: provider == "openai-codex", Headers: make(http.Header), secrets: []string{token, "Bearer " + token}}
	if c.Native {
		claims, err := decodeJWT(token)
		if err != nil {
			return connection{}, err
		}
		a := object(claims["https://api.openai.com/auth"])
		id := stringValue(a["chatgpt_account_id"])
		if id == "" {
			return connection{}, errors.New("Codex access token has no ChatGPT account ID")
		}
		c.Account = account{stringValue(claims["email"]), stringValue(a["chatgpt_plan_type"])}
		c.Headers.Set("ChatGPT-Account-Id", id)
		// PiG's pinned D26 identity, not the original Pi originator.
		c.Headers.Set("originator", "pig")
		c.secrets = append(c.secrets, id)
	} else {
		c.Base = stringValue(auth["baseUrl"])
		if c.Base == "" {
			c.Base = stringValue(model["baseUrl"])
		}
		if c.Base == "" {
			return connection{}, errors.New("Selected provider has no base URL")
		}
		u, err := url.Parse(c.Base)
		if err != nil || u.Host == "" || u.User != nil || (u.Scheme != "https" && u.Scheme != "http") {
			return connection{}, errors.New("Selected provider has an invalid base URL")
		}
		c.Base = strings.TrimSuffix(c.Base, "/")
		for name, value := range object(auth["headers"]) {
			s, ok := value.(string)
			if !ok {
				continue
			}
			c.Headers.Set(name, s)
			if s != "" {
				c.secrets = append(c.secrets, s)
			}
		}
	}
	c.Headers.Set("Authorization", "Bearer "+token)
	sort.Slice(c.secrets, func(i, j int) bool { return len(c.secrets[i]) > len(c.secrets[j]) })
	return c, nil
}
func decodeJWT(token string) (map[string]any, error) {
	parts := strings.Split(token, ".")
	if len(parts) < 2 || parts[1] == "" {
		return nil, errors.New("Invalid Codex access token")
	}
	encoded := strings.NewReplacer("-", "+", "_", "/").Replace(parts[1])
	encoded += strings.Repeat("=", (4-len(encoded)%4)%4)
	raw, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, errors.New("Could not decode Codex access token")
	}
	var claims map[string]any
	if err := json.Unmarshal(raw, &claims); err != nil || claims == nil {
		return nil, errors.New("Could not decode Codex access token")
	}
	return claims, nil
}
func (c connection) scrub(text string) string {
	for _, secret := range c.secrets {
		if secret == "" {
			continue
		}
		for _, value := range []string{secret, url.QueryEscape(secret), url.PathEscape(secret), strings.ReplaceAll(url.QueryEscape(secret), "+", "%20")} {
			text = strings.ReplaceAll(text, value, "<redacted>")
		}
	}
	return text
}
func newClient() *http.Client {
	return &http.Client{Timeout: 15 * time.Second, Transport: http.DefaultTransport.(*http.Transport).Clone(), CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) >= 10 {
			return errors.New("too many Codex redirects")
		}
		if len(via) > 0 && (req.URL.Scheme != via[0].URL.Scheme || req.URL.Host != via[0].URL.Host) {
			req.Header = make(http.Header)
		}
		return nil
	}}
}
func requestJSON(ctx context.Context, client *http.Client, c connection, target, method string, body any) (map[string]any, error) {
	var data []byte
	if body != nil {
		var err error
		data, err = json.Marshal(body)
		if err != nil {
			return nil, errors.New("Could not encode Codex request")
		}
	}
	req, err := http.NewRequestWithContext(ctx, method, target, bytes.NewReader(data))
	if err != nil {
		return nil, errors.New("Could not create Codex request")
	}
	req.Header = c.Headers.Clone()
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	response, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Codex request failed: %s", c.scrub(err.Error()))
	}
	defer response.Body.Close()
	raw, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, fmt.Errorf("Codex response failed: %s", c.scrub(err.Error()))
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		detail := trim(string(raw))
		if detail != "" {
			detail = ": " + c.scrub(detail)
		}
		return nil, fmt.Errorf("Codex request failed (HTTP %d)%s", response.StatusCode, detail)
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var result map[string]any
	if err := decoder.Decode(&result); err != nil || result == nil {
		return nil, errors.New("Codex returned malformed JSON")
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return nil, errors.New("Codex returned malformed JSON")
	}
	return result, nil
}
func fetchStatus(ctx context.Context, client *http.Client, c connection) (status, error) {
	target := usageURL
	if !c.Native {
		target = c.Base + "/codex/usage"
	}
	payload, err := requestJSON(ctx, client, c, target, http.MethodGet, nil)
	if err != nil {
		return status{}, err
	}
	var acct *account
	if c.Native {
		acct = &c.Account
	}
	s, err := parseStatus(payload, acct)
	s.Account.Email = c.scrub(s.Account.Email)
	s.Account.Plan = c.scrub(s.Account.Plan)
	return s, err
}
func fetchCredits(ctx context.Context, client *http.Client, c connection) ([]credit, error) {
	target := resetsURL
	if !c.Native {
		target = c.Base + "/codex/resets"
	}
	payload, err := requestJSON(ctx, client, c, target, http.MethodGet, nil)
	if err != nil {
		return nil, err
	}
	credits, err := parseCredits(payload)
	for i := range credits {
		v := &credits[i]
		v.ID = c.scrub(v.ID)
		v.Status = c.scrub(v.Status)
		v.Granted = c.scrub(v.Granted)
		v.Expires = c.scrub(v.Expires)
	}
	return credits, err
}
func requestID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", errors.New("Could not create Codex redemption ID")
	}
	b[6] = b[6]&15 | 64
	b[8] = b[8]&63 | 128
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[:4], b[4:6], b[6:8], b[8:10], b[10:]), nil
}
func activateReset(ctx context.Context, client *http.Client, c connection, id string) (resetResult, error) {
	target := c.Base + "/codex/reset"
	body := map[string]any{"credit_id": id}
	if c.Native {
		target = resetsURL + "/consume"
		nonce, err := requestID()
		if err != nil {
			return resetResult{}, err
		}
		body["redeem_request_id"] = nonce
	}
	payload, err := requestJSON(ctx, client, c, target, http.MethodPost, body)
	if err != nil {
		return resetResult{}, err
	}
	result, err := parseResetResult(payload)
	if err != nil {
		return resetResult{}, errors.New(c.scrub(err.Error()))
	}
	return result, nil
}
