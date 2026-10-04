// Observation (not a bug): events an extension sees when it submits one
// prompt and queues a second as a follow-up in the same agent run.
package eventorder

import (
	"strings"
	"sync"

	sdk "github.com/MichaelKinsy/PiG/extensions/sdk"
)

func Extension() *sdk.Extension {
	ext := sdk.New("pig-followup-event-order")
	var mu sync.Mutex
	var seen []string
	record := func(name string) sdk.EventFunc {
		return func(ctx sdk.Context, data map[string]any) (any, error) {
			label := name
			if message, ok := data["message"].(map[string]any); ok {
				if message["role"] != "user" {
					return nil, nil
				}
				label += ":" + userText(message["content"])
			}
			if prompt, ok := data["prompt"].(string); ok {
				label += ":" + prompt
			}
			mu.Lock()
			seen = append(seen, label)
			mu.Unlock()
			return nil, nil
		}
	}
	for _, name := range []string{sdk.EventBeforeAgentStart, sdk.EventMessageStart, sdk.EventAgentSettled} {
		ext.OnEvent(name, record(name))
	}
	ext.Command("send-two", "submit ONE, queue TWO", func(ctx sdk.Context, _ string) error {
		_ = ctx.SendUserMessage("ONE", "")
		_ = ctx.SendUserMessage("TWO", "followUp")
		return nil
	})
	ext.Command("events", "report observed events", func(ctx sdk.Context, _ string) error {
		mu.Lock()
		defer mu.Unlock()
		ctx.Notify("events "+strings.Join(seen, " | "), "info")
		return nil
	})
	return ext
}

func userText(content any) string {
	if blocks, ok := content.([]any); ok {
		for _, block := range blocks {
			if b, ok := block.(map[string]any); ok && b["type"] == "text" {
				text, _ := b["text"].(string)
				return text
			}
		}
	}
	text, _ := content.(string)
	return text
}
