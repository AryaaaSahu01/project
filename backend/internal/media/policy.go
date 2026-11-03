package media

import (
	"bytes"
	"encoding/binary"
	"errors"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"mime"
	"net/http"
	"os"
	"strings"
	"unicode"
	"unicode/utf8"
)

const (
	maxWorkspaceBytes int64 = 1 << 30
	maxWorkspaceItems       = 200
	maxMemoryItems          = 10
)

type mediaRule struct {
	Kind     string `json:"kind"`
	MaxBytes int64  `json:"max_bytes"`
}

var rules = map[string]mediaRule{
	"image/jpeg": {"image", 10 << 20}, "image/png": {"image", 10 << 20}, "image/webp": {"image", 10 << 20},
	"audio/mpeg": {"audio", 25 << 20}, "audio/wav": {"audio", 25 << 20}, "audio/ogg": {"audio", 25 << 20},
	"audio/webm": {"audio", 25 << 20}, "audio/mp4": {"audio", 25 << 20},
	"video/mp4": {"video", 100 << 20}, "video/webm": {"video", 100 << 20},
}

func canonicalType(value string) string {
	t, _, err := mime.ParseMediaType(value)
	if err != nil {
		return ""
	}
	switch t {
	case "audio/x-wav", "audio/wave":
		return "audio/wav"
	case "audio/mp3":
		return "audio/mpeg"
	case "audio/x-m4a":
		return "audio/mp4"
	default:
		return t
	}
}

func safeName(name string) string {
	name = strings.ReplaceAll(name, "\\", "/")
	parts := strings.Split(name, "/")
	name = parts[len(parts)-1]
	name = strings.Map(func(c rune) rune {
		if unicode.IsControl(c) {
			return -1
		}
		return c
	}, name)
	name = strings.TrimSpace(name)
	if !utf8.ValidString(name) || name == "" || name == "." || name == ".." {
		return "Attachment"
	}
	if len([]rune(name)) > 180 {
		name = string([]rune(name)[:180])
	}
	return name
}

func inspectFile(f *os.File, contentType string) error {
	prefix := make([]byte, 4096)
	n, err := f.ReadAt(prefix, 0)
	if err != nil && !errors.Is(err, io.EOF) {
		return err
	}
	prefix = prefix[:n]
	ok := false
	switch contentType {
	case "image/jpeg", "image/png":
		if http.DetectContentType(prefix) == contentType {
			if _, err := f.Seek(0, 0); err != nil {
				return err
			}
			cfg, _, err := image.DecodeConfig(f)
			ok = err == nil && cfg.Width > 0 && cfg.Height > 0 && int64(cfg.Width)*int64(cfg.Height) <= 40000000
		}
	case "image/webp":
		ok = len(prefix) >= 12 && string(prefix[:4]) == "RIFF" && string(prefix[8:12]) == "WEBP"
	case "audio/wav":
		ok = len(prefix) >= 12 && string(prefix[:4]) == "RIFF" && string(prefix[8:12]) == "WAVE"
	case "audio/ogg":
		ok = bytes.HasPrefix(prefix, []byte("OggS\x00"))
	case "audio/mpeg":
		ok = bytes.HasPrefix(prefix, []byte("ID3")) || len(prefix) > 2 && prefix[0] == 0xff && prefix[1]&0xe0 == 0xe0 && prefix[1]&0x06 != 0
	case "audio/mp4", "video/mp4":
		ok = len(prefix) >= 16 && string(prefix[4:8]) == "ftyp" && binary.BigEndian.Uint32(prefix[:4]) >= 16
	case "audio/webm", "video/webm":
		ok = bytes.HasPrefix(prefix, []byte{0x1a, 0x45, 0xdf, 0xa3}) && bytes.Contains(prefix, []byte("webm"))
	}
	if !ok {
		return errors.New("file contents do not match a supported media format")
	}
	_, err = f.Seek(0, 0)
	return err
}
