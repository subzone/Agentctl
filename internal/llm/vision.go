package llm

import "strings"

// UserBlocks builds a user message from optional text and image blocks.
func UserBlocks(text string, images ...ContentBlock) Message {
	var blocks []ContentBlock
	if strings.TrimSpace(text) != "" {
		blocks = append(blocks, ContentBlock{Type: BlockText, Text: text})
	}
	blocks = append(blocks, images...)
	return Message{Role: RoleUser, Content: blocks}
}

// ImageBlock returns a vision content block.
func ImageBlock(mimeType string, data []byte) ContentBlock {
	return ContentBlock{Type: BlockImage, MimeType: mimeType, ImageData: data}
}

// SupportsVision reports whether a model id is likely to accept image inputs.
func SupportsVision(model string) bool {
	m := strings.ToLower(model)
	if i := strings.LastIndex(m, "/"); i >= 0 {
		m = m[i+1:]
	}
	visionHints := []string{
		"gpt-4o", "gpt-4.1", "gpt-4-turbo", "gpt-4-vision", "o1", "o3", "o4",
		"claude-3", "claude-sonnet-4", "claude-opus-4", "claude-haiku-4",
		"gemini", "llava", "bakllava", "moondream", "vision", "vl",
	}
	for _, hint := range visionHints {
		if strings.Contains(m, hint) {
			return true
		}
	}
	return false
}

// MessageHasImages reports whether any user message contains image blocks.
func MessageHasImages(msgs []Message) bool {
	for _, m := range msgs {
		for _, b := range m.Content {
			if b.Type == BlockImage && len(b.ImageData) > 0 {
				return true
			}
		}
	}
	return false
}
