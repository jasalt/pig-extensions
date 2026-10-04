// Repro: in RPC (and print) mode, ctx.Compact() from an extension reports
// success but never compacts.
package rpccompact

import (
	"sync"
	"time"

	sdk "github.com/MichaelKinsy/PiG/extensions/sdk"
)

func Extension() *sdk.Extension {
	ext := sdk.New("pig-rpc-extension-compact-noop")
	var once sync.Once
	started := make(chan struct{})
	ext.OnEvent(sdk.EventSessionBeforeCompact, func(sdk.Context, map[string]any) (any, error) {
		once.Do(func() { close(started) })
		return nil, nil
	})
	ext.Command("compact-now", "compact from the extension", func(ctx sdk.Context, _ string) error {
		ctx.Compact(nil)
		select {
		case <-started:
			ctx.Notify("compaction started", "info")
		case <-time.After(5 * time.Second):
			ctx.Notify("no compaction within 5s", "warning")
		}
		ctx.CompactWithOptions(sdk.CompactOptions{OnError: func(err error) { ctx.Notify("with callbacks: "+err.Error(), "error") }})
		time.Sleep(time.Second)
		return nil
	})
	return ext
}
