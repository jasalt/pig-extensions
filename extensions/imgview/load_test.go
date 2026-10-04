// Copyright (c) 2026 Jarkko Saltiola
// SPDX-License-Identifier: MIT

package imgview

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

var (
	pngMagic  = []byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a, 0x00, 0x00, 0x00, 0x0d}
	jpegMagic = []byte{0xff, 0xd8, 0xff, 0xe0}
	gifMagic  = []byte{0x47, 0x49, 0x46, 0x38, 0x39, 0x61}
	webpMagic = []byte{0x52, 0x49, 0x46, 0x46, 0, 0, 0, 0, 0x57, 0x45, 0x42, 0x50}
)

func testLoader(home string) loader {
	return loader{client: &http.Client{}, home: func() (string, error) { return home, nil }}
}

// Upstream utils.test.ts cases.
func TestSniffMimeUpstream(t *testing.T) {
	for _, tc := range []struct {
		data []byte
		hint string
		want string
	}{
		{pngMagic, "", "image/png"},
		{jpegMagic, "", "image/jpeg"},
		{gifMagic, "", "image/gif"},
		{webpMagic, "", "image/webp"},
		{[]byte(`<svg xmlns="x"></svg>`), "", "image/svg+xml"},
		{[]byte(`<?xml version="1.0"?><svg></svg>`), "", "image/svg+xml"},
		{[]byte{1, 2, 3, 4}, "/x/y.png", "image/png"},
		{[]byte{1, 2, 3, 4}, "/x/y.jpeg", "image/jpeg"},
		{[]byte{1, 2, 3, 4}, "", "application/octet-stream"},
	} {
		if got := sniffMime(tc.data, tc.hint); got != tc.want {
			t.Errorf("sniffMime(%q, %q) = %q, want %q", tc.data, tc.hint, got, tc.want)
		}
	}
}

func TestSniffMimeBoundaries(t *testing.T) {
	avif := append([]byte{0, 0, 0, 0x1c}, []byte("ftypavif")...)
	for _, tc := range []struct {
		name string
		data []byte
		hint string
		want string
	}{
		{"bmp", []byte("BM...."), "", "image/bmp"},
		{"avif", avif, "", "image/avif"},
		{"avis", append([]byte{0, 0, 0, 0x1c}, []byte("ftypavis")...), "", "image/avif"},
		{"heic is not avif", append([]byte{0, 0, 0, 0x1c}, []byte("ftypheic")...), "", "application/octet-stream"},
		{"short webp", []byte("RIFF\x00\x00\x00\x00WEB"), "", "application/octet-stream"},
		{"svg after js whitespace and BOM", []byte("\uFEFF \n\t<SVG>"), "", "image/svg+xml"},
		{"svg needs separator", []byte("<svgx>"), "", "application/octet-stream"},
		{"magic beats misleading extension", pngMagic, "/x/y.gif", "image/png"},
		{"upper-case extension", []byte{1}, "/x/Y.WEBP", "image/webp"},
		{"jpg extension", []byte{1}, "a.jpg", "image/jpeg"},
		{"query hides url extension", []byte{1}, "https://x/a.png?x=1", "application/octet-stream"},
		{"empty", nil, "", "application/octet-stream"},
	} {
		if got := sniffMime(tc.data, tc.hint); got != tc.want {
			t.Errorf("%s: got %q, want %q", tc.name, got, tc.want)
		}
	}
}

func TestExtensionAndSupportedMime(t *testing.T) {
	for mime, want := range map[string]string{
		"image/png": "png", "image/jpeg": "jpg", "image/jpg": "jpg", "image/gif": "gif",
		"image/webp": "webp", "image/svg+xml": "svg", "application/zip": "bin", "IMAGE/BMP": "bmp", "image/avif": "avif",
	} {
		if got := extensionForMime(mime); got != want {
			t.Errorf("extensionForMime(%q) = %q, want %q", mime, got, want)
		}
	}
	for _, mime := range []string{"image/png", "image/jpeg", "IMAGE/PNG", "image/svg+xml", "image/bmp", "image/avif", "image/gif", "image/webp"} {
		if !isSupportedImageMime(mime) {
			t.Errorf("%q should be supported", mime)
		}
	}
	for _, mime := range []string{"application/pdf", "text/plain", "", "image/tiff", "image/jpg"} {
		if isSupportedImageMime(mime) {
			t.Errorf("%q should be rejected", mime)
		}
	}
}

func TestExpandHome(t *testing.T) {
	for in, want := range map[string]string{"~": "/home/u", "~/foo": "/home/u/foo", "/abs": "/abs", "rel/path": "rel/path", "~weird": "~weird"} {
		if got := expandHome(in, "/home/u"); got != want {
			t.Errorf("expandHome(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestDataURI(t *testing.T) {
	ctx := context.Background()
	l := testLoader("/home/u")
	img, err := l.resolve(ctx, "data:image/png;base64,"+base64.StdEncoding.EncodeToString(pngMagic), "/tmp")
	if err != nil || img.MimeType != "image/png" || img.SourceLabel != "<data uri>" || img.Extension != "png" || !bytes.Equal(img.Bytes, pngMagic) {
		t.Fatalf("base64 PNG: %+v, %v", img, err)
	}
	if _, err := l.resolve(ctx, "data:not-a-uri", "/tmp"); err == nil || !strings.Contains(err.Error(), "malformed data:") {
		t.Fatalf("malformed: %v", err)
	}
	// Literal "+" survives a percent-encoded (non-base64) payload.
	svg := `<svg xmlns="http://www.w3.org/2000/svg"><text>1+1%3D2</text></svg>`
	img, err = l.resolve(ctx, "data:image/svg+xml,"+svg, "/tmp")
	if err != nil || img.MimeType != "image/svg+xml" || string(img.Bytes) != strings.Replace(svg, "%3D", "=", 1) {
		t.Fatalf("percent payload: %+v, %v", img, err)
	}
	if _, err := l.resolve(ctx, "data:image/svg+xml,%E0%A4%A", "/tmp"); err == nil {
		t.Fatal("bad escape accepted")
	}
	if _, err := l.resolve(ctx, "data:image/svg+xml,%FF", "/tmp"); err == nil {
		t.Fatal("invalid UTF-8 accepted")
	}
	// Node-compatible forgiving base64: URL-safe, unpadded, whitespace.
	raw := []byte{0xfb, 0xff, 0xbf, 0x89, 'P', 'N', 'G'}
	encoded := base64.RawURLEncoding.EncodeToString(raw)
	img, err = l.resolve(ctx, "data:image/x;base64,"+encoded[:4]+"\n "+encoded[4:], "/tmp")
	if err != nil || !bytes.Equal(img.Bytes, raw) {
		t.Fatalf("forgiving base64: %v %v", img, err)
	}
	// Unrecognized bytes keep the declared type (rejected later if unsupported).
	img, err = l.resolve(ctx, "data:image/png;base64,AAECAw==", "/tmp")
	if err != nil || img.MimeType != "image/png" {
		t.Fatalf("declared fallback: %+v %v", img, err)
	}
	img, err = l.resolve(ctx, "data:;base64,AAECAw==", "/tmp")
	if err != nil || img.MimeType != "application/octet-stream" {
		t.Fatalf("missing declared type: %+v %v", img, err)
	}
	// Magic bytes win over a misleading declared type.
	img, _ = l.resolve(ctx, "data:image/gif;base64,"+base64.StdEncoding.EncodeToString(jpegMagic), "/tmp")
	if img.MimeType != "image/jpeg" {
		t.Fatalf("misleading declared type: %q", img.MimeType)
	}
	if _, err := l.resolve(ctx, " \t ", "/tmp"); err == nil || err.Error() != "image source is empty" {
		t.Fatalf("empty: %v", err)
	}
}

func TestFileSources(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	home := filepath.Join(dir, "home")
	cwd := filepath.Join(dir, "cwd")
	for _, d := range []string{home, cwd, filepath.Join(cwd, "with space")} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	file := filepath.Join(cwd, "with space", "x.png")
	if err := os.WriteFile(file, pngMagic, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, "h.jpg"), jpegMagic, 0o644); err != nil {
		t.Fatal(err)
	}
	l := testLoader(home)
	for source, label := range map[string]string{
		file:                               file,
		"with space/x.png":                 file,
		"  with space/x.png\n":             file,
		"~/h.jpg":                          filepath.Join(home, "h.jpg"),
		"./with space/../with space/x.png": file,
	} {
		img, err := l.resolve(ctx, source, cwd)
		if err != nil || img.SourceLabel != label {
			t.Errorf("%q: %+v %v", source, img, err)
		}
	}
	if _, err := l.resolve(ctx, "/no/such/file/exists.png", cwd); err == nil || !strings.HasPrefix(err.Error(), "cannot read /no/such/file/exists.png: ") {
		t.Fatalf("missing: %v", err)
	}
	if _, err := l.resolve(ctx, "with space", cwd); err == nil || err.Error() != filepath.Join(cwd, "with space")+" is not a regular file" {
		t.Fatalf("directory: %v", err)
	}
	// "~user" is not expanded; it is relative to cwd.
	if _, err := l.resolve(ctx, "~weird.png", cwd); err == nil || !strings.Contains(err.Error(), filepath.Join(cwd, "~weird.png")) {
		t.Fatalf("~weird: %v", err)
	}
	// Extension hint applies to unknown local bytes.
	hinted := filepath.Join(cwd, "unknown.webp")
	_ = os.WriteFile(hinted, []byte{1, 2, 3}, 0o644)
	if img, err := l.resolve(ctx, hinted, cwd); err != nil || img.MimeType != "image/webp" {
		t.Fatalf("hint: %+v %v", img, err)
	}
}

func TestHTTPSources(t *testing.T) {
	ctx := context.Background()
	mux := http.NewServeMux()
	mux.HandleFunc("/ok.png", func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write(pngMagic) })
	mux.HandleFunc("/redirect", func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, "/ok.png", http.StatusFound) })
	mux.HandleFunc("/missing", func(w http.ResponseWriter, _ *http.Request) { http.Error(w, "no", http.StatusNotFound) })
	mux.HandleFunc("/declared", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "image/gif; charset=binary")
		_, _ = w.Write([]byte{1, 2, 3})
	})
	mux.HandleFunc("/lying", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write(gifMagic)
	})
	server := httptest.NewServer(mux)
	defer server.Close()
	l := loader{client: server.Client(), home: os.UserHomeDir}
	img, err := l.resolve(ctx, server.URL+"/ok.png", "/")
	if err != nil || img.MimeType != "image/png" || img.SourceLabel != server.URL+"/ok.png" {
		t.Fatalf("ok: %+v %v", img, err)
	}
	if img, err = l.resolve(ctx, server.URL+"/redirect", "/"); err != nil || !bytes.Equal(img.Bytes, pngMagic) {
		t.Fatalf("redirect: %+v %v", img, err)
	}
	if _, err = l.resolve(ctx, server.URL+"/missing", "/"); err == nil || err.Error() != "failed to fetch "+server.URL+"/missing: HTTP 404 Not Found" {
		t.Fatalf("404: %v", err)
	}
	if img, err = l.resolve(ctx, server.URL+"/declared", "/"); err != nil || img.MimeType != "image/gif" {
		t.Fatalf("declared: %+v %v", img, err)
	}
	if img, err = l.resolve(ctx, server.URL+"/lying", "/"); err != nil || img.MimeType != "image/gif" {
		t.Fatalf("magic over header: %+v %v", img, err)
	}
	if _, err = l.resolve(ctx, "HTTP://127.0.0.1:1/x.png", "/"); err == nil || !strings.HasPrefix(err.Error(), "failed to fetch HTTP://127.0.0.1:1/x.png: ") {
		t.Fatalf("connection refused: %v", err)
	}
}

func TestHTTPCancellationDuringBody(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "1000000")
		_, _ = w.Write(pngMagic)
		w.(http.Flusher).Flush()
		close(started)
		select {
		case <-release:
		case <-r.Context().Done():
		}
	}))
	defer server.Close()
	defer close(release)
	ctx, cancel := context.WithCancel(context.Background())
	l := loader{client: server.Client(), home: os.UserHomeDir}
	result := make(chan error, 1)
	go func() {
		_, err := l.resolve(ctx, server.URL+"/slow.png", "/")
		result <- err
	}()
	<-started
	cancel()
	select {
	case err := <-result:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("got %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("cancellation did not stop the download")
	}
}

func TestCancelledBeforeFileRead(t *testing.T) {
	file := filepath.Join(t.TempDir(), "x.png")
	_ = os.WriteFile(file, pngMagic, 0o644)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := testLoader("/").resolve(ctx, file, "/"); !errors.Is(err, context.Canceled) {
		t.Fatalf("got %v", err)
	}
}
