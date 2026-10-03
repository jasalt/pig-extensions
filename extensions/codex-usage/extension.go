// Copyright (c) 2026 Jarkko Saltiola
// SPDX-License-Identifier: MIT
package codexusage

import (
	"encoding/json"
	"errors"

	sdk "github.com/MichaelKinsy/PiG/extensions/sdk"
)

func handlerView(ctx sdk.Context) view {
	return view{
		key: func() string {
			session, err := ctx.GetSessionID()
			if err != nil {
				return "unavailable"
			}
			return session + "/" + ctx.ModelQualified()
		},
		done: ctx.Done, err: ctx.Err,
		connection: func() (connection, error) {
			model, err := ctx.GetModelInfo()
			if err != nil {
				return connection{}, errors.New("Could not resolve selected model")
			}
			if model == nil {
				return connection{}, errors.New("No model selected")
			}
			raw, err := ctx.GetModelAuth(model.Provider, model.ID)
			if err != nil {
				return connection{}, errors.New("Could not resolve Codex authentication")
			}
			var auth map[string]any
			if err := json.Unmarshal(raw, &auth); err != nil {
				return connection{}, errors.New("Could not decode Codex authentication")
			}
			metadata := ctx.ModelRegistry().Find(model.Provider, model.ID)
			return resolveConnection(model.Provider, metadata, auth)
		},
		status: func(text string) {
			if text != "" {
				text = ctx.UITheme().Fg("dim", text)
			}
			ctx.SetStatus(statusKey, text)
		},
		notify: ctx.Notify,
	}
}

// Extension constructs an effect-free factory. Only runtime lifecycle events
// start polling. Each factory owns its transport, ticker and cancellation tree.
func Extension() *sdk.Extension {
	ext := sdk.New("codex-usage")
	o := newOwner(newClient(), refreshInterval)
	ext.Command("codex-usage", "Show ChatGPT Codex account and rate-limit usage", func(ctx sdk.Context, _ string) error { o.refresh(handlerView(ctx), true); return nil })
	ext.Command("codex-reset", "List banked Codex resets or activate an exact reset ID", func(ctx sdk.Context, args string) error { o.reset(handlerView(ctx), args); return nil })
	ext.OnEvent(sdk.EventSessionStart, func(ctx sdk.Context, _ map[string]any) (any, error) {
		v := handlerView(ctx)
		o.attach(v, true)
		o.start(v)
		o.refresh(v, false)
		return nil, nil
	})
	ext.OnEvent(sdk.EventModelSelect, func(ctx sdk.Context, _ map[string]any) (any, error) {
		v := handlerView(ctx)
		o.attach(v, true)
		v.status("")
		o.refresh(v, false)
		return nil, nil
	})
	ext.OnEvent(sdk.EventAgentSettled, func(ctx sdk.Context, _ map[string]any) (any, error) {
		v := handlerView(ctx)
		o.attach(v, false)
		o.refresh(v, false)
		return nil, nil
	})
	ext.OnEvent(sdk.EventSessionShutdown, func(ctx sdk.Context, _ map[string]any) (any, error) {
		o.shutdown()
		ctx.SetStatus(statusKey, "")
		return nil, nil
	})
	return ext
}
