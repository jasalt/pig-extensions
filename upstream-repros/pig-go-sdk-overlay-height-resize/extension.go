// Repro: an open overlay re-renders after a width change but not after a
// height-only change, although its layout depends on the terminal height.
package overlayprobe

import (
	"fmt"
	"sync"

	sdk "github.com/MichaelKinsy/PiG/extensions/sdk"
)

type view struct {
	mu     sync.Mutex
	ctx    sdk.Context
	frames int
}

func (v *view) Render(width int) []string {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.frames++
	return []string{fmt.Sprintf("PROBE frame=%d width=%d height=%d", v.frames, width, v.ctx.Height())}
}

func (v *view) HandleInput(data string) (sdk.RemoteComponentResult, error) {
	return sdk.RemoteComponentResult{Done: data == "q"}, nil
}

func Extension() *sdk.Extension {
	ext := sdk.New("pig-go-sdk-overlay-height-resize")
	ext.Command("overlay-probe", "open a height-reporting overlay", func(ctx sdk.Context, _ string) error {
		_, err := ctx.Custom(&view{ctx: ctx}, sdk.RemoteOverlayOptions{Overlay: true})
		return err
	})
	return ext
}
