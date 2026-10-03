// Copyright (c) 2026 Jarkko Saltiola
// SPDX-License-Identifier: MIT
package notifypushover

import (
	"context"
	"fmt"
	"os"
	"sync"

	sdk "github.com/MichaelKinsy/PiG/extensions/sdk"
)

const description = "Send a one-way Pushover notification to the human operator when work is blocked on a human decision, approval, credentials, access, or missing external evidence. This only alerts the human; it does not wait for or collect a reply. Use sparingly and continue independent work when possible."

func Extension() *sdk.Extension {
	ext := sdk.New("notify-pushover")
	client := newClient()
	var mu sync.Mutex
	var cached *config
	var closing bool
	var active sync.WaitGroup
	lifetime, cancel := context.WithCancel(context.Background())
	missing := func() string {
		return "Pushover credentials are not configured. Set PIG_PUSHOVER_USER_KEY + PIG_PUSHOVER_APP_TOKEN or create " + credentialPath() + "."
	}
	load := func(ctx sdk.Context, force bool) *config {
		mu.Lock()
		defer mu.Unlock()
		if force || cached == nil {
			var err error
			cached, err = loadConfig(credentialPath(), os.Getenv)
			if err != nil {
				ctx.Notify(err.Error(), "error")
			}
		}
		return cached
	}
	ext.OnEvent(sdk.EventSessionStart, func(ctx sdk.Context, _ map[string]any) (any, error) {
		cfg := load(ctx, true)
		status := "human notify: ready"
		if cfg == nil {
			status = "human notify: no creds"
			ctx.Notify(missing(), "warning")
		}
		ctx.SetStatus(statusKey, status)
		return nil, nil
	})
	ext.OnEvent(sdk.EventSessionShutdown, func(ctx sdk.Context, _ map[string]any) (any, error) {
		mu.Lock()
		closing = true
		cached = nil
		cancel()
		mu.Unlock()
		active.Wait()
		client.CloseIdleConnections()
		ctx.SetStatus(statusKey, "")
		return nil, nil
	})
	deliver := func(ctx sdk.Context, cfg *config, p params) (int, error) {
		if err := ctx.Err(); err != nil {
			return 0, fmt.Errorf("Pushover send cancelled: %s", redact(err.Error(), cfg))
		}
		mu.Lock()
		if closing {
			mu.Unlock()
			return 0, fmt.Errorf("Pushover extension is shutting down")
		}
		active.Add(1)
		mu.Unlock()
		defer active.Done()
		request, stop := context.WithCancel(lifetime)
		joined := make(chan struct{})
		go func() {
			defer close(joined)
			select {
			case <-ctx.Done():
				stop()
			case <-request.Done():
			}
		}()
		defer func() { stop(); <-joined }()
		return send(request, client, cfg, p)
	}
	ext.RegisterTool(sdk.ToolDefinition{
		Name: "notify_human", Label: "Notify human", Description: description,
		PromptSnippet: "Use notify_human to send a one-way Pushover alert for human decisions/access/approval blockers; do not use it for routine progress.",
		Parameters: sdk.Schema{"type": "object", "required": []string{"message"}, "properties": map[string]any{
			"message": map[string]any{"type": "string", "description": "Concise blocker summary and exact action needed from the human."},
			"title":   map[string]any{"type": "string", "description": "Defaults to PiG needs a human decision."},
			"url":     map[string]any{"type": "string"}, "urlTitle": map[string]any{"type": "string"},
			"priority": map[string]any{"type": "integer", "enum": []int{-2, -1, 0, 1}},
		}},
		Execute: func(ctx sdk.Context, args map[string]any) (any, error) {
			cfg := load(ctx, false)
			if cfg == nil {
				return nil, fmt.Errorf("%s", missing())
			}
			p := params{Message: stringArg(args, "message"), Title: stringArg(args, "title"), URL: stringArg(args, "url"), URLTitle: stringArg(args, "urlTitle")}
			if priority, ok := args["priority"].(float64); ok {
				p.Priority = int(priority)
			}
			status, err := deliver(ctx, cfg, p)
			if err != nil {
				return nil, err
			}
			ctx.SetStatus(statusKey, fmt.Sprintf("human notify: sent (%d)", status))
			device := cfg.Device
			if device == "" {
				device = "default device(s)"
			}
			hostname, _ := os.Hostname()
			var deviceDetail any
			if cfg.Device != "" {
				deviceDetail = redact(cfg.Device, cfg)
			}
			return map[string]any{"content": []map[string]any{{"type": "text", "text": redact(fmt.Sprintf("Pushover notification sent (%d) to %s from %s.", status, device, hostname), cfg)}}, "details": map[string]any{"status": status, "device": deviceDetail}}, nil
		},
	})
	ext.Command("notify-human-test", "Send a test one-way Pushover notification.", func(ctx sdk.Context, args string) error {
		cfg := load(ctx, false)
		if cfg == nil {
			ctx.Notify(missing(), "error")
			return nil
		}
		message := trim(args)
		if message == "" {
			hostname, _ := os.Hostname()
			message = "Test from PiG on " + hostname + "."
		}
		status, err := deliver(ctx, cfg, params{Message: message, Title: "PiG human notification test"})
		if err != nil {
			ctx.Notify("Pushover test failed: "+err.Error(), "error")
		} else {
			ctx.Notify(fmt.Sprintf("Pushover test sent (%d).", status), "info")
		}
		return nil
	})
	return ext
}

func stringArg(args map[string]any, key string) string { value, _ := args[key].(string); return value }
