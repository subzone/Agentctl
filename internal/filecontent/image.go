package filecontent

import (
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const maxImageBytes = 5 * 1024 * 1024 // 5 MB

var imageMIME = map[string]string{
	".png":  "image/png",
	".jpg":  "image/jpeg",
	".jpeg": "image/jpeg",
	".gif":  "image/gif",
	".webp": "image/webp",
	".bmp":  "image/bmp",
}

// Image holds raw image bytes for vision models.
type Image struct {
	MimeType string
	Data     []byte
}

// IsImage reports whether the filename or magic bytes look like a supported image.
func IsImage(name string, data []byte) bool {
	if mime := mimeFromName(name); mime != "" {
		return true
	}
	return detectImageMIME(data) != ""
}

// ReadImage loads a supported image file for vision APIs.
func ReadImage(path string) (*Image, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return ReadImageBytes(filepath.Base(path), data)
}

// ReadImageBytes parses image bytes using the filename extension as a hint.
func ReadImageBytes(name string, data []byte) (*Image, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("%s is empty", name)
	}
	if int64(len(data)) > maxImageBytes {
		return nil, fmt.Errorf("%s exceeds %d MB image limit", name, maxImageBytes/(1024*1024))
	}
	mime := mimeFromName(name)
	if mime == "" {
		mime = detectImageMIME(data)
	}
	if mime == "" {
		return nil, fmt.Errorf("%s is not a supported image (png, jpg, gif, webp, bmp)", name)
	}
	return &Image{MimeType: mime, Data: data}, nil
}

// ImageBase64 returns a data URL suitable for OpenAI-style vision APIs.
func ImageBase64(img *Image) string {
	if img == nil || len(img.Data) == 0 {
		return ""
	}
	return "data:" + img.MimeType + ";base64," + base64.StdEncoding.EncodeToString(img.Data)
}

func mimeFromName(name string) string {
	return imageMIME[strings.ToLower(filepath.Ext(name))]
}

func detectImageMIME(data []byte) string {
	if len(data) >= 8 && data[0] == 0x89 && data[1] == 'P' && data[2] == 'N' && data[3] == 'G' {
		return "image/png"
	}
	if len(data) >= 3 && data[0] == 0xFF && data[1] == 0xD8 && data[2] == 0xFF {
		return "image/jpeg"
	}
	if len(data) >= 6 && (string(data[:6]) == "GIF87a" || string(data[:6]) == "GIF89a") {
		return "image/gif"
	}
	if len(data) >= 12 && string(data[0:4]) == "RIFF" && string(data[8:12]) == "WEBP" {
		return "image/webp"
	}
	if len(data) >= 2 && data[0] == 'B' && data[1] == 'M' {
		return "image/bmp"
	}
	return ""
}
