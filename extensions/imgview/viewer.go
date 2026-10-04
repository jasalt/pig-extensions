// Copyright (c) 2026 Gregory Johnson
// Copyright (c) 2026 Jarkko Saltiola
// SPDX-License-Identifier: MIT

package imgview

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"
)

// viewerRoot is the private directory for browser viewer files. Viewers stay
// on disk after unload so an opened browser tab can keep reading them.
func viewerRoot() string { return filepath.Join(os.TempDir(), "pig-imgview") }

// writeViewer writes a private, self-contained HTML page embedding img.
func writeViewer(img *resolvedImage, root string, now time.Time) (string, error) {
	if err := os.MkdirAll(root, 0o700); err != nil {
		return "", err
	}
	if err := os.Chmod(root, 0o700); err != nil {
		return "", err
	}
	var random [4]byte
	if _, err := rand.Read(random[:]); err != nil {
		return "", err
	}
	path := filepath.Join(root, fmt.Sprintf("imgview-%d-%s.html", now.UnixMilli(), hex.EncodeToString(random[:])))
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return "", err
	}
	_, writeErr := file.WriteString(viewerHTML(img))
	closeErr := file.Close()
	if writeErr != nil || closeErr != nil {
		_ = os.Remove(path)
		if writeErr != nil {
			return "", writeErr
		}
		return "", closeErr
	}
	return path, nil
}

func viewerHTML(img *resolvedImage) string {
	label := escapeHTML(img.SourceLabel)
	dataURI := "data:" + img.MimeType + ";base64," + base64.StdEncoding.EncodeToString(img.Bytes)
	return `<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>imgview: ` + label + `</title>
<style>
  html, body { margin: 0; padding: 0; height: 100%; background: #111; color: #ddd; font: 13px -apple-system, system-ui, sans-serif; }
  .wrap { display: flex; flex-direction: column; height: 100%; }
  header { padding: 8px 12px; background: #1a1a1a; border-bottom: 1px solid #2a2a2a; user-select: text; }
  header code { color: #9cf; }
  main { flex: 1; display: flex; align-items: center; justify-content: center; overflow: auto; padding: 12px; }
  img { max-width: 100%; max-height: 100%; box-shadow: 0 4px 20px rgba(0,0,0,0.5); image-rendering: -webkit-optimize-contrast; }
</style>
</head>
<body>
  <div class="wrap">
    <header>imgview · <code>` + label + `</code> · ` + groupThousands(len(img.Bytes)) + ` bytes · ` + escapeHTML(img.MimeType) + `</header>
    <main><img src="` + escapeHTML(dataURI) + `" alt="` + label + `"></main>
  </div>
</body>
</html>
`
}

var htmlEscaper = strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;", "'", "&#39;")

func escapeHTML(s string) string { return htmlEscaper.Replace(s) }

// groupThousands formats n as en-US Number.toLocaleString does.
func groupThousands(n int) string {
	digits := fmt.Sprint(n)
	if n < 0 {
		return "-" + groupThousands(-n)
	}
	var out strings.Builder
	for i, c := range digits {
		if i > 0 && (len(digits)-i)%3 == 0 {
			out.WriteByte(',')
		}
		out.WriteRune(c)
	}
	return out.String()
}

// opener launches the platform default handler for one viewer path.
type opener interface {
	Open(target string) (command string, args []string, err error)
}

// processOpener starts xdg-open with argv (no shell) in its own session and
// reaps it in the background. Wait joins outstanding reapers.
type processOpener struct {
	command string
	reapers sync.WaitGroup
}

func (o *processOpener) Open(target string) (string, []string, error) {
	args := []string{target}
	cmd := exec.Command(o.command, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		return o.command, args, err
	}
	o.reapers.Add(1)
	go func() {
		defer o.reapers.Done()
		_ = cmd.Wait()
	}()
	return o.command, args, nil
}

// Wait joins finished openers for at most timeout. An opener that keeps
// running (some handlers block until the browser exits) is left detached.
func (o *processOpener) Wait(timeout time.Duration) {
	done := make(chan struct{})
	go func() {
		o.reapers.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(timeout):
	}
}
