// Copyright (c) 2026 Gregory Johnson
// Copyright (c) 2026 Jarkko Saltiola
// SPDX-License-Identifier: MIT

package imgview

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

// resolvedImage is one loaded image, mirroring upstream ResolvedImage.
type resolvedImage struct {
	Bytes       []byte
	MimeType    string
	SourceLabel string
	Extension   string
}

var supportedMimes = map[string]bool{
	"image/png":     true,
	"image/jpeg":    true,
	"image/gif":     true,
	"image/webp":    true,
	"image/bmp":     true,
	"image/avif":    true,
	"image/svg+xml": true,
}

func extensionForMime(mimeType string) string {
	m := strings.ToLower(mimeType)
	switch {
	case strings.Contains(m, "jpeg") || strings.Contains(m, "jpg"):
		return "jpg"
	case strings.Contains(m, "png"):
		return "png"
	case strings.Contains(m, "gif"):
		return "gif"
	case strings.Contains(m, "webp"):
		return "webp"
	case strings.Contains(m, "bmp"):
		return "bmp"
	case strings.Contains(m, "avif"):
		return "avif"
	case strings.Contains(m, "svg"):
		return "svg"
	}
	return "bin"
}

var (
	xmlHead = regexp.MustCompile(`(?i)^<\?xml`)
	svgHead = regexp.MustCompile(`(?i)^<svg[\s>]`)
)

// sniffMime detects magic bytes, then falls back to the hint's extension.
func sniffMime(buf []byte, hintPath string) string {
	n := len(buf)
	switch {
	case n >= 4 && buf[0] == 0x89 && buf[1] == 'P' && buf[2] == 'N' && buf[3] == 'G':
		return "image/png"
	case n >= 3 && buf[0] == 0xff && buf[1] == 0xd8 && buf[2] == 0xff:
		return "image/jpeg"
	case n >= 4 && buf[0] == 'G' && buf[1] == 'I' && buf[2] == 'F' && buf[3] == '8':
		return "image/gif"
	case n >= 12 && string(buf[0:4]) == "RIFF" && string(buf[8:12]) == "WEBP":
		return "image/webp"
	case n >= 2 && buf[0] == 'B' && buf[1] == 'M':
		return "image/bmp"
	}
	if n >= 12 && string(buf[4:8]) == "ftyp" {
		if brand := string(buf[8:12]); brand == "avif" || brand == "avis" {
			return "image/avif"
		}
	}
	if n > 0 {
		head := strings.TrimLeftFunc(string(buf[:min(n, 256)]), isJSSpace)
		if xmlHead.MatchString(head) || svgHead.MatchString(head) {
			return "image/svg+xml"
		}
	}
	if hintPath != "" {
		switch strings.TrimPrefix(strings.ToLower(filepath.Ext(hintPath)), ".") {
		case "png":
			return "image/png"
		case "jpg", "jpeg":
			return "image/jpeg"
		case "gif":
			return "image/gif"
		case "webp":
			return "image/webp"
		case "bmp":
			return "image/bmp"
		case "avif":
			return "image/avif"
		case "svg":
			return "image/svg+xml"
		}
	}
	return "application/octet-stream"
}

func isSupportedImageMime(mimeType string) bool {
	return supportedMimes[strings.ToLower(mimeType)]
}

// isJSSpace matches ECMAScript \s, including BOM but not NEL.
func isJSSpace(r rune) bool {
	return unicode.Is(unicode.Zs, r) || strings.ContainsRune("\t\n\v\f\r\u2028\u2029\uFEFF", r)
}

func jsTrim(text string) string { return strings.TrimFunc(text, isJSSpace) }

func expandHome(path, home string) string {
	if path == "~" {
		return home
	}
	if strings.HasPrefix(path, "~/") {
		return filepath.Join(home, path[2:])
	}
	return path
}

var (
	dataURIPattern = regexp.MustCompile(`(?s)^data:([^;,]+)?(;base64)?,(.*)$`)
	httpPattern    = regexp.MustCompile(`(?i)^https?://`)
)

// loader carries the per-extension seams used to load image sources.
type loader struct {
	client *http.Client
	home   func() (string, error)
}

func (l loader) resolve(ctx context.Context, source, cwd string) (*resolvedImage, error) {
	trimmed := jsTrim(source)
	if trimmed == "" {
		return nil, errors.New("image source is empty")
	}
	if strings.HasPrefix(trimmed, "data:") {
		return decodeDataURI(trimmed)
	}
	if httpPattern.MatchString(trimmed) {
		return l.fetch(ctx, trimmed)
	}
	home, err := l.home()
	if err != nil && (trimmed == "~" || strings.HasPrefix(trimmed, "~/")) {
		return nil, fmt.Errorf("cannot expand %s: %v", trimmed, err)
	}
	abs := trimmed
	if !filepath.IsAbs(abs) {
		abs = expandHome(abs, home)
		if !filepath.IsAbs(abs) {
			abs = filepath.Join(cwd, abs)
		}
	}
	abs = filepath.Clean(abs)
	info, err := os.Stat(abs)
	if err != nil {
		return nil, fmt.Errorf("cannot read %s: %s", abs, describeFSError(err))
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("%s is not a regular file", abs)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	bytes, err := os.ReadFile(abs)
	if err != nil {
		return nil, fmt.Errorf("cannot read %s: %s", abs, describeFSError(err))
	}
	mimeType := sniffMime(bytes, abs)
	return &resolvedImage{Bytes: bytes, MimeType: mimeType, SourceLabel: abs, Extension: extensionForMime(mimeType)}, nil
}

func describeFSError(err error) string {
	var pathErr *os.PathError
	if errors.As(err, &pathErr) {
		return pathErr.Err.Error()
	}
	return err.Error()
}

func decodeDataURI(uri string) (*resolvedImage, error) {
	match := dataURIPattern.FindStringSubmatch(uri)
	if match == nil {
		return nil, errors.New("malformed data: URI")
	}
	declared := match[1]
	if declared == "" {
		declared = "application/octet-stream"
	}
	var bytes []byte
	if match[2] != "" {
		bytes = decodeForgivingBase64(match[3])
	} else {
		// decodeURIComponent semantics: "+" stays literal, bad escapes and
		// invalid UTF-8 are errors.
		decoded, err := url.PathUnescape(match[3])
		if err != nil || !utf8.ValidString(decoded) {
			return nil, errors.New("malformed data: URI: URI malformed")
		}
		bytes = []byte(decoded)
	}
	mimeType := declared
	if sniffed := sniffMime(bytes, ""); sniffed != "application/octet-stream" {
		mimeType = sniffed
	}
	return &resolvedImage{Bytes: bytes, MimeType: mimeType, SourceLabel: "<data uri>", Extension: extensionForMime(mimeType)}, nil
}

// decodeForgivingBase64 follows Node's Buffer.from(text, "base64"): standard
// and URL-safe alphabets, optional padding, ignored non-alphabet characters,
// stopping at the first padding character.
func decodeForgivingBase64(text string) []byte {
	var clean strings.Builder
	clean.Grow(len(text))
	for i := 0; i < len(text); i++ {
		c := text[i]
		switch {
		case c == '=':
			i = len(text)
		case c == '-':
			clean.WriteByte('+')
		case c == '_':
			clean.WriteByte('/')
		case c >= 'A' && c <= 'Z', c >= 'a' && c <= 'z', c >= '0' && c <= '9', c == '+', c == '/':
			clean.WriteByte(c)
		}
	}
	data := clean.String()
	if len(data)%4 == 1 {
		data = data[:len(data)-1]
	}
	decoded, err := base64.RawStdEncoding.DecodeString(data)
	if err != nil {
		return nil
	}
	return decoded
}

func (l loader) fetch(ctx context.Context, rawURL string) (*resolvedImage, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch %s: %v", rawURL, err)
	}
	response, err := l.client.Do(request)
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, ctxErr
		}
		return nil, fmt.Errorf("failed to fetch %s: %v", rawURL, err)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode > 299 {
		return nil, fmt.Errorf("failed to fetch %s: HTTP %s", rawURL, response.Status)
	}
	bytes, err := io.ReadAll(response.Body)
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, ctxErr
		}
		return nil, fmt.Errorf("failed to fetch %s: %v", rawURL, err)
	}
	mimeType := sniffMime(bytes, rawURL)
	if mimeType == "application/octet-stream" {
		declared, _, _ := strings.Cut(response.Header.Get("Content-Type"), ";")
		if declared = strings.TrimSpace(declared); declared != "" {
			mimeType = declared
		}
	}
	return &resolvedImage{Bytes: bytes, MimeType: mimeType, SourceLabel: rawURL, Extension: extensionForMime(mimeType)}, nil
}
