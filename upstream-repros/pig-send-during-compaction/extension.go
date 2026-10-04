// Repro: what an extension observes when it submits a user message while
// compaction is in progress.
package compactprobe

import (
	"fmt"
	"sync"
	"time"

	sdk "github.com/MichaelKinsy/PiG/extensions/sdk"
)

func Extension() *sdk.Extension {
	ext := sdk.New("pig-send-during-compaction")
	var once sync.Once
	started := make(chan struct{})
	ext.OnEvent(sdk.EventSessionBeforeCompact, func(sdk.Context, map[string]any) (any, error) {
		once.Do(func() { close(started) })
		return nil, nil
	})
	ext.Command("compact-probe", "send a message during compaction", func(ctx sdk.Context, _ string) error {
		ctx.CompactWithOptions(sdk.CompactOptions{
			OnComplete: func(map[string]any) { ctx.Notify("compaction completed", "info") },
			OnError:    func(err error) { ctx.Notify("compaction failed: "+err.Error(), "error") },
		})
		select {
		case <-started:
		case <-time.After(10 * time.Second):
			ctx.Notify("compaction did not start", "error")
			return nil
		}
		err := ctx.SendUserMessage("MESSAGE-SENT-DURING-COMPACTION", "")
		ctx.Notify(fmt.Sprintf("sendUserMessage returned err=%v", err), "info")
		return nil
	})
	return ext
}
