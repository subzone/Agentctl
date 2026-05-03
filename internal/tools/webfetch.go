package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// WebFetchTool fetches a URL and returns the readable text content.
// HTML is stripped to plain text; non-HTML responses are returned as-is.
// Output is capped at MaxBytes to avoid blowing the context window.
type WebFetchTool struct {
	MaxBytes int
	client   *http.Client
}

// NewWebFetch returns a WebFetchTool with reasonable defaults.
func NewWebFetch() *WebFetchTool {
	return &WebFetchTool{
		MaxBytes: 16 * 1024,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (w *WebFetchTool) Name() string { return "web_fetch" }

func (w *WebFetchTool) Description() string {
	return "Fetch a URL and return its text content. HTML pages are converted to plain text. " +
		"Use this to read documentation, articles, API references, or any web page."
}

func (w *WebFetchTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
  "type": "object",
  "properties": {
    "url": {"type": "string", "description": "The URL to fetch"}
  },
  "required": ["url"]
}`)
}

func (w *WebFetchTool) Run(ctx context.Context, input json.RawMessage) (string, error) {
	var args struct {
		URL string `json:"url"`
	}
	if err := json.Unmarshal(input, &args); err != nil {
		return "", fmt.Errorf("invalid input: %w", err)
	}
	if args.URL == "" {
		return "", errors.New("url is required")
	}
	if !strings.HasPrefix(args.URL, "http://") && !strings.HasPrefix(args.URL, "https://") {
		args.URL = "https://" + args.URL
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, args.URL, nil)
	if err != nil {
		return "", fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("User-Agent", "AgentCTL/1.0 (web_fetch tool)")
	req.Header.Set("Accept", "text/html, text/plain, application/json, */*")

	resp, err := w.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("fetch: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %d %s", resp.StatusCode, resp.Status)
	}

	// Read body with size cap.
	maxRead := int64(w.MaxBytes * 2) // read extra for HTML overhead, trim after extraction
	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxRead))
	if err != nil {
		return "", fmt.Errorf("read body: %w", err)
	}

	ct := resp.Header.Get("Content-Type")
	var text string
	if strings.Contains(ct, "text/html") {
		text = htmlToText(raw)
	} else {
		text = string(raw)
	}

	// Trim to MaxBytes.
	if len(text) > w.MaxBytes {
		text = text[:w.MaxBytes] + "\n[output truncated]"
	}

	return text, nil
}

// htmlToText extracts readable text from HTML by stripping tags, scripts,
// styles, and collapsing whitespace. Stdlib-only, no dependencies.
func htmlToText(raw []byte) string {
	s := string(raw)

	// Remove script and style blocks entirely.
	s = stripBetween(s, "<script", "</script>")
	s = stripBetween(s, "<style", "</style>")
	s = stripBetween(s, "<noscript", "</noscript>")
	s = stripBetween(s, "<!--", "-->")

	// Replace block-level tags with newlines.
	for _, tag := range []string{"<br", "<p", "<div", "<li", "<tr", "<h1", "<h2", "<h3", "<h4", "<h5", "<h6", "<blockquote", "<pre", "<hr"} {
		s = replaceTagWithNewline(s, tag)
	}

	// Strip all remaining tags.
	s = stripTags(s)

	// Decode common HTML entities.
	s = decodeEntities(s)

	// Collapse whitespace: multiple spaces → one, multiple newlines → two.
	s = collapseWhitespace(s)

	return strings.TrimSpace(s)
}

// stripBetween removes everything between open and close (case-insensitive).
func stripBetween(s, open, close string) string {
	lower := strings.ToLower(s)
	openLower := strings.ToLower(open)
	closeLower := strings.ToLower(close)
	var buf bytes.Buffer
	for {
		idx := strings.Index(lower, openLower)
		if idx < 0 {
			buf.WriteString(s)
			break
		}
		buf.WriteString(s[:idx])
		end := strings.Index(lower[idx:], closeLower)
		if end < 0 {
			break // unclosed tag, drop the rest
		}
		skip := idx + end + len(close)
		s = s[skip:]
		lower = lower[skip:]
	}
	return buf.String()
}

// replaceTagWithNewline replaces opening tags (e.g. "<p", "<div") with newlines.
func replaceTagWithNewline(s, tagPrefix string) string {
	lower := strings.ToLower(s)
	prefix := strings.ToLower(tagPrefix)
	var buf bytes.Buffer
	for {
		idx := strings.Index(lower, prefix)
		if idx < 0 {
			buf.WriteString(s)
			break
		}
		buf.WriteString(s[:idx])
		buf.WriteByte('\n')
		// Skip past the closing >.
		end := strings.IndexByte(s[idx:], '>')
		if end < 0 {
			break
		}
		skip := idx + end + 1
		s = s[skip:]
		lower = lower[skip:]
	}
	return buf.String()
}

// stripTags removes all HTML tags.
func stripTags(s string) string {
	var buf bytes.Buffer
	inTag := false
	for _, r := range s {
		if r == '<' {
			inTag = true
			continue
		}
		if r == '>' {
			inTag = false
			continue
		}
		if !inTag {
			buf.WriteRune(r)
		}
	}
	return buf.String()
}

// decodeEntities handles common HTML entities.
func decodeEntities(s string) string {
	r := strings.NewReplacer(
		"&amp;", "&",
		"&lt;", "<",
		"&gt;", ">",
		"&quot;", "\"",
		"&#39;", "'",
		"&apos;", "'",
		"&nbsp;", " ",
		"&mdash;", "—",
		"&ndash;", "–",
		"&hellip;", "…",
		"&copy;", "©",
		"&reg;", "®",
		"&trade;", "™",
	)
	return r.Replace(s)
}

// collapseWhitespace normalizes runs of spaces and newlines.
func collapseWhitespace(s string) string {
	var buf bytes.Buffer
	prevNewline := false
	prevSpace := false
	newlineCount := 0
	for _, r := range s {
		if r == '\n' || r == '\r' {
			if !prevNewline {
				newlineCount = 1
				buf.WriteByte('\n')
			} else if newlineCount < 2 {
				newlineCount++
				buf.WriteByte('\n')
			}
			prevNewline = true
			prevSpace = false
			continue
		}
		if r == ' ' || r == '\t' {
			if !prevSpace && !prevNewline {
				buf.WriteByte(' ')
			}
			prevSpace = true
			continue
		}
		prevNewline = false
		prevSpace = false
		newlineCount = 0
		buf.WriteRune(r)
	}
	return buf.String()
}
