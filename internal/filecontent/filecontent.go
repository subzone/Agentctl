package filecontent

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	pdf "github.com/ledongthuc/pdf"
)

const defaultMaxBytes = 256 * 1024

// Read extracts human-readable text from a file for agent context.
// Supports UTF-8 text files and PDF documents (text layer only).
func Read(path string, maxBytes int64) (string, error) {
	if maxBytes <= 0 {
		maxBytes = defaultMaxBytes
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return ReadBytes(filepath.Base(path), data, maxBytes)
}

// ReadBytes extracts text from raw file bytes using the filename hint for format detection.
func ReadBytes(name string, data []byte, maxBytes int64) (string, error) {
	if maxBytes <= 0 {
		maxBytes = defaultMaxBytes
	}
	if len(data) == 0 {
		return "", fmt.Errorf("%s is empty", name)
	}

	ext := strings.ToLower(filepath.Ext(name))
	if ext == ".pdf" || isPDF(data) {
		text, err := extractPDF(data)
		if err != nil {
			return "", err
		}
		return truncate(text, maxBytes, name)
	}
	if IsImage(name, data) {
		return "", fmt.Errorf("%s is an image — attach it in chat for vision analysis (text/PDF only for inline read)", name)
	}
	if looksBinary(data) {
		return "", fmt.Errorf("%s is a binary file — supported inline: UTF-8 text and PDF; attach images in chat", name)
	}
	text := strings.TrimRight(string(data), "\n")
	if !utf8.ValidString(text) {
		return "", fmt.Errorf("%s is not valid UTF-8 text", name)
	}
	return truncate(text, maxBytes, name)
}

func isPDF(data []byte) bool {
	return len(data) >= 5 && string(data[:5]) == "%PDF-"
}

func looksBinary(data []byte) bool {
	n := len(data)
	if n > 512 {
		n = 512
	}
	for _, b := range data[:n] {
		if b == 0 {
			return true
		}
	}
	return false
}

func extractPDF(data []byte) (string, error) {
	r, err := pdf.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return "", fmt.Errorf("parse PDF: %w", err)
	}
	reader, err := r.GetPlainText()
	if err != nil {
		return "", fmt.Errorf("extract PDF text: %w", err)
	}
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, reader); err != nil {
		return "", fmt.Errorf("read PDF text: %w", err)
	}
	text := strings.TrimSpace(buf.String())
	if text == "" {
		return "", fmt.Errorf("PDF has no extractable text (it may be a scanned image-only document)")
	}
	return text, nil
}

func truncate(text string, maxBytes int64, name string) (string, error) {
	if int64(len(text)) <= maxBytes {
		return text, nil
	}
	cut := text[:maxBytes]
	// Avoid splitting a UTF-8 rune.
	for len(cut) > 0 && !utf8.ValidString(cut) {
		cut = cut[:len(cut)-1]
	}
	return cut + fmt.Sprintf("\n[truncated — %s exceeded %d bytes]", name, maxBytes), nil
}
