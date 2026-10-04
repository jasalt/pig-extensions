// Repro: ctx.ExecWithOptions with a 1 s timeout on a command that leaves a
// background child holding stdout.
package execprobe

import (
	"fmt"
	"time"

	sdk "github.com/MichaelKinsy/PiG/extensions/sdk"
)

func Extension() *sdk.Extension {
	ext := sdk.New("pig-exec-timeout-process-tree")
	ext.Command("exec-probe", "run a timed exec", func(ctx sdk.Context, pidfile string) error {
		start := time.Now()
		res, err := ctx.ExecWithOptions("bash", []string{"-c", "sleep 8 & echo $! > " + pidfile + "; wait"},
			sdk.ExecOptions{Timeout: 1000})
		if err != nil {
			ctx.Notify("exec error: "+err.Error(), "error")
			return nil
		}
		ctx.Notify(fmt.Sprintf("elapsed=%.1fs killed=%t code=%d", time.Since(start).Seconds(), res.Killed, res.ExitCode), "info")
		return nil
	})
	return ext
}
