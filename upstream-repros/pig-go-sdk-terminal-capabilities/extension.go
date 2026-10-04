// Repro: a Go extension cannot learn the host's resolved terminal
// capabilities. sdk.Context has no accessor, and the only public source,
// tui.GetCapabilities, is this process's own environment detection.
package capsprobe

import (
	"fmt"

	sdk "github.com/MichaelKinsy/PiG/extensions/sdk"
	"github.com/MichaelKinsy/PiG/tui"
)

func Extension() *sdk.Extension {
	ext := sdk.New("pig-go-sdk-terminal-capabilities")
	ext.Command("caps-probe", "report terminal capabilities", func(ctx sdk.Context, _ string) error {
		caps := tui.GetCapabilities()
		ctx.Notify(fmt.Sprintf("extension sees hyperlinks=%t images=%q", caps.Hyperlinks, caps.Images), "info")
		return nil
	})
	return ext
}
