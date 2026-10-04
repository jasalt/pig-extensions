// Copyright (c) 2026 Jarkko Saltiola
// SPDX-License-Identifier: MIT

// Package btwprobe is a test-only feasibility probe, not a BTW port: a Go
// extension command runs one child coding session through PiG's public SDK.
package btwprobe

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"github.com/MichaelKinsy/PiG/ai"
	"github.com/MichaelKinsy/PiG/coding"
	sdk "github.com/MichaelKinsy/PiG/extensions/sdk"
)

func agentDir(ctx sdk.Context) string {
	if dir := os.Getenv("PIG_CODING_AGENT_DIR"); dir != "" {
		return dir
	}
	return filepath.Join(ctx.ConfigHome(), "agent")
}

func Extension() *sdk.Extension {
	ext := sdk.New("btw-child-session-probe")
	ext.Command("btw-probe", "child session probe", func(ctx sdk.Context, args string) error {
		reply, err := ask(ctx, args)
		if err != nil {
			ctx.Notify("btw-probe error: "+err.Error(), "error")
			return nil
		}
		ctx.Notify("btw-probe reply: "+reply, "info")
		return nil
	})
	return ext
}

func ask(ctx sdk.Context, question string) (string, error) {
	services, err := coding.NewServices(coding.ServicesOptions{CWD: ctx.Cwd(), AgentDir: agentDir(ctx)})
	if err != nil {
		return "", err
	}
	runtime, err := coding.NewRuntime(coding.RuntimeOptions{Services: services})
	if err != nil {
		return "", err
	}
	defer runtime.Close()
	model, err := coding.BuildModel(ctx.ModelQualified(), services)
	if err != nil {
		return "", err
	}
	manager, err := coding.NewInMemorySessionManager(ctx.Cwd())
	if err != nil {
		return "", err
	}
	session, err := runtime.New(coding.SessionStartOptions{
		SessionManager:     manager,
		Model:              model,
		SystemPrompt:       "You answer side questions briefly.",
		ActiveBuiltinTools: map[string]struct{}{"read": {}, "grep": {}, "find": {}, "ls": {}},
	})
	if err != nil {
		return "", err
	}
	defer session.Close()
	request, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		select {
		case <-ctx.Done():
			cancel()
		case <-request.Done():
		}
	}()
	messages, err := session.Send(request, question)
	if err != nil {
		return "", err
	}
	var parts []string
	for _, message := range messages {
		if message.Assistant == nil {
			continue
		}
		for _, block := range message.ContentBlocks() {
			if text, ok := block.(ai.TextContent); ok {
				parts = append(parts, text.Text)
			}
		}
	}
	return strings.Join(parts, ""), nil
}
